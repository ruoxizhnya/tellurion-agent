package builtin

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tellurion-agent/pkg/ai/client"
	"tellurion-agent/pkg/ai/tools"
)

// ═══════════════════════════════════════════════════════════════════════
//  ValidateFactorTool tests
// ═══════════════════════════════════════════════════════════════════════

func TestValidateFactorTool_Name(t *testing.T) {
	tt := newValidateTool(t)
	assert.Equal(t, "validate_factor", tt.Name())
}

func TestValidateFactorTool_Description(t *testing.T) {
	tt := newValidateTool(t)
	desc := tt.Description()
	assert.NotEmpty(t, desc)
	// Description starts with "Validate" (capital V) — case-sensitive match.
	assert.Contains(t, desc, "Validate")
	assert.Contains(t, desc, "L1 gate")
}

func TestValidateFactorTool_Parameters(t *testing.T) {
	tt := newValidateTool(t)
	params := tt.Parameters()
	require.Len(t, params, 1)
	assert.Equal(t, "expression", params[0].Name)
	assert.Equal(t, "string", params[0].Type)
	assert.True(t, params[0].Required, "expression should be required")
}

func TestValidateFactorTool_OutputSchema(t *testing.T) {
	tt := newValidateTool(t)
	schema := tt.OutputSchema()
	assert.Equal(t, "object", schema.Type)
	require.NotEmpty(t, schema.Fields)

	expected := map[string]string{
		"valid":          "bool",
		"inputs":         "array",
		"ast":            "string",
		"error":          "string",
		"level":          "string",
		"passed":         "bool",
		"reason":         "string",
		"recommendation": "string",
	}
	assert.Len(t, schema.Fields, len(expected))
	for _, f := range schema.Fields {
		typ, ok := expected[f.Name]
		require.True(t, ok, "unexpected field %q", f.Name)
		assert.Equal(t, typ, f.Type, "field %q type mismatch", f.Name)
	}
}

func TestValidateFactorTool_Execute_ValidExpression(t *testing.T) {
	tt := newValidateTool(t)
	args := map[string]interface{}{
		"expression": "ts_rank(close, 20)",
	}

	result, err := tt.Execute(context.Background(), args)
	require.NoError(t, err, "valid expression should not return a tool error")
	require.NotNil(t, result)

	m, ok := result.(map[string]interface{})
	require.True(t, ok, "result should be map[string]interface{}, got %T", result)

	valid, ok := m["valid"].(bool)
	require.True(t, ok, "valid should be bool, got %T", m["valid"])
	assert.True(t, valid, "expression 'ts_rank(close, 20)' should be valid")

	inputs, ok := m["inputs"].([]string)
	require.True(t, ok, "inputs should be []string, got %T", m["inputs"])
	assert.Contains(t, inputs, "close", "inputs should contain 'close'")

	ast, ok := m["ast"].(string)
	require.True(t, ok, "ast should be string, got %T", m["ast"])
	assert.NotEmpty(t, ast, "ast should be non-empty for valid expression")

	errStr, ok := m["error"].(string)
	require.True(t, ok, "error should be string, got %T", m["error"])
	assert.Empty(t, errStr, "error should be empty for valid expression")

	// L1 GateDecision fields.
	assert.Equal(t, "L1", m["level"], "level should be L1")
	passed, ok := m["passed"].(bool)
	require.True(t, ok, "passed should be bool, got %T", m["passed"])
	assert.True(t, passed, "passed should mirror valid")
	assert.Equal(t, GateReasonPassed, m["reason"], "reason should be 'passed' for valid expression")
	recommendation, ok := m["recommendation"].(string)
	require.True(t, ok, "recommendation should be string, got %T", m["recommendation"])
	assert.NotEmpty(t, recommendation, "recommendation should be non-empty for L1 result")
}

func TestValidateFactorTool_Execute_ComplexExpression(t *testing.T) {
	tt := newValidateTool(t)
	args := map[string]interface{}{
		"expression": "(close - ts_mean(close, 20)) / ts_std(close, 20)",
	}

	result, err := tt.Execute(context.Background(), args)
	require.NoError(t, err)

	m := result.(map[string]interface{})
	assert.True(t, m["valid"].(bool))
	inputs := m["inputs"].([]string)
	assert.Contains(t, inputs, "close")
}

func TestValidateFactorTool_Execute_InvalidSyntax(t *testing.T) {
	// Missing closing paren — syntax error.
	tt := newValidateTool(t)
	args := map[string]interface{}{
		"expression": "ts_rank(close, 20",
	}

	result, err := tt.Execute(context.Background(), args)
	// Parse failure is a VALIDATION RESULT, not a tool error.
	require.NoError(t, err, "parse failure should NOT be returned as a Go error")
	require.NotNil(t, result)

	m, ok := result.(map[string]interface{})
	require.True(t, ok, "result should be map[string]interface{}, got %T", result)

	valid, ok := m["valid"].(bool)
	require.True(t, ok, "valid should be bool")
	assert.False(t, valid, "malformed expression should have valid=false")

	errStr, ok := m["error"].(string)
	require.True(t, ok, "error should be string")
	assert.NotEmpty(t, errStr, "error message should explain the syntax problem")

	inputs, ok := m["inputs"].([]string)
	require.True(t, ok, "inputs should be []string even when invalid")
	assert.Empty(t, inputs, "inputs should be empty for invalid expression")

	// L1 GateDecision: parse failure → passed=false, reason=syntax_error.
	assert.Equal(t, "L1", m["level"], "level should be L1")
	passed, ok := m["passed"].(bool)
	require.True(t, ok, "passed should be bool, got %T", m["passed"])
	assert.False(t, passed, "passed should be false for syntax error")
	assert.Equal(t, GateReasonSyntaxError, m["reason"], "reason should be 'syntax_error' for parse failure")
	recommendation, ok := m["recommendation"].(string)
	require.True(t, ok, "recommendation should be string, got %T", m["recommendation"])
	assert.NotEmpty(t, recommendation, "recommendation should be non-empty to guide retry")
}

func TestValidateFactorTool_Execute_EmptyFormula(t *testing.T) {
	// Empty string is a parse error (parser rejects empty formulas),
	// not an ErrInvalidArgs — requireString rejects "" but the parser
	// would also reject it. Verify the parser path returns valid=false.
	// Actually requireString rejects empty strings with ErrInvalidArgs,
	// so this exercises the arg-validation path, not the parser path.
	tt := newValidateTool(t)
	args := map[string]interface{}{
		"expression": "",
	}
	_, err := tt.Execute(context.Background(), args)
	require.Error(t, err)
	assert.True(t, errors.Is(err, tools.ErrInvalidArgs))
	assert.Contains(t, err.Error(), "expression")
}

func TestValidateFactorTool_Execute_MissingExpression(t *testing.T) {
	tt := newValidateTool(t)
	args := map[string]interface{}{}
	_, err := tt.Execute(context.Background(), args)
	require.Error(t, err)
	assert.True(t, errors.Is(err, tools.ErrInvalidArgs))
	assert.Contains(t, err.Error(), "expression")
}

func TestValidateFactorTool_Execute_WrongType(t *testing.T) {
	tt := newValidateTool(t)
	args := map[string]interface{}{
		"expression": 42,
	}
	_, err := tt.Execute(context.Background(), args)
	require.Error(t, err)
	assert.True(t, errors.Is(err, tools.ErrInvalidArgs))
	assert.Contains(t, err.Error(), "expression")
}

func TestValidateFactorTool_RegisterInRegistry(t *testing.T) {
	tt := newValidateTool(t)
	reg := tools.NewRegistry()
	require.NoError(t, reg.Register(tt))

	got, err := reg.Get("validate_factor")
	require.NoError(t, err)
	assert.Equal(t, "validate_factor", got.Name())

	info := reg.List()
	require.Len(t, info, 1)
	assert.Equal(t, "validate_factor", info[0].Name)
}

// TestValidateFactorTool_DelegatesVerdictToCore 钉住 QT-17 S3-c-4 的判据：
// 工具不自己解析表达式，core 说 valid=false 就原样映射成 L1 失败（含阶段与
// 原因），绝不「本地再判一次」或把服务故障当成合法。
func TestValidateFactorTool_DelegatesVerdictToCore(t *testing.T) {
	tt, stub := newStubValidateTool(t)

	res, err := tt.Execute(context.Background(), map[string]interface{}{
		// 语法上能 parse，但 core 的算子闸门不认（OBS-06 的现场）。
		"expression": "CROSS(MA(close, 5), MA(close, 20))",
	})
	require.NoError(t, err, "core's invalid verdict is a result, not a tool error")
	m := res.(map[string]interface{})
	assert.False(t, m["valid"].(bool))
	assert.False(t, m["passed"].(bool))
	assert.Equal(t, GateReasonSyntaxError, m["reason"])
	assert.Contains(t, m["error"].(string), "CROSS")
	assert.Equal(t, int32(1), stub.validateCalls, "exactly one coarse validate call")

	// 失败建议必须附上目录里的字段状态：不可用字段被点名，且不消失。
	rec := m["recommendation"].(string)
	assert.Contains(t, rec, "close", "usable fields should be advertised")
	assert.Contains(t, rec, "当前无数据", "unavailable fields must be flagged")
	assert.Contains(t, rec, "pe", "unavailable field `pe` must not disappear from the hint")
	assert.NotContains(t, rec, "sector", "group labels are not data fields")
}

// TestValidateFactorTool_ServiceFailureIsToolError：core 不可用 ≠ 表达式不合法。
func TestValidateFactorTool_ServiceFailureIsToolError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "upstream down", http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	tt := NewValidateFactorTool(client.NewExpressionClient(srv.URL, client.WithInsecureLoopback()))
	res, err := tt.Execute(context.Background(), map[string]interface{}{"expression": "close"})
	require.Error(t, err, "a 503 must not be reported as valid=false")
	assert.Nil(t, res)
	assert.Contains(t, err.Error(), "unavailable")
}

// TestValidateFactorTool_CatalogUnavailableIsStated：目录拉不到时只给基础
// 建议并明确说明清单缺失，不编造字段。
func TestValidateFactorTool_CatalogUnavailableIsStated(t *testing.T) {
	tt, stub := newStubValidateTool(t)
	stub.catalogErr = true

	res, err := tt.Execute(context.Background(), map[string]interface{}{
		"expression": "ts_rank(close, 20",
	})
	require.NoError(t, err)
	rec := res.(map[string]interface{})["recommendation"].(string)
	assert.Contains(t, rec, "语法错误")
	assert.Contains(t, rec, "字段目录暂不可用")
}

// ═══════════════════════════════════════════════════════════════════════
//  ComputeFactorICTool tests
// ═══════════════════════════════════════════════════════════════════════

func TestComputeFactorICTool_Name(t *testing.T) {
	tt := NewComputeFactorICTool(client.NewFactorClient("http://x"))
	assert.Equal(t, "compute_factor_ic", tt.Name())
}

func TestComputeFactorICTool_Description_MarksUnavailable(t *testing.T) {
	tt := NewComputeFactorICTool(client.NewFactorClient("http://x"))
	desc := tt.Description()
	assert.Contains(t, desc, "[unavailable]")
	assert.Contains(t, desc, "ADR-041")
	assert.Contains(t, desc, "IC")
}

func TestComputeFactorICTool_Parameters(t *testing.T) {
	tt := NewComputeFactorICTool(client.NewFactorClient("http://x"))
	params := tt.Parameters()
	require.Len(t, params, 4)

	expectedNames := []string{"expression", "symbols", "start_date", "end_date"}
	gotNames := make([]string, len(params))
	for i, p := range params {
		gotNames[i] = p.Name
	}
	assert.Equal(t, expectedNames, gotNames)

	for _, p := range params {
		assert.True(t, p.Required, "parameter %q should be required", p.Name)
	}
}

func TestComputeFactorICTool_OutputSchema(t *testing.T) {
	tt := NewComputeFactorICTool(client.NewFactorClient("http://x"))
	schema := tt.OutputSchema()
	assert.Equal(t, "object", schema.Type)

	found := map[string]string{}
	for _, f := range schema.Fields {
		found[f.Name] = f.Type
	}
	assert.Equal(t, "float", found["ic"], "ic field should be float")
	assert.Equal(t, "float", found["ir"], "ir field should be float")
	// L2 GateDecision fields.
	assert.Equal(t, "string", found["level"], "level field should be string")
	assert.Equal(t, "bool", found["passed"], "passed field should be bool")
	assert.Equal(t, "string", found["reason"], "reason field should be string")
	assert.Equal(t, "string", found["recommendation"], "recommendation field should be string")
}

func TestComputeFactorICTool_Execute_Unavailable_NoHTTP(t *testing.T) {
	srv, hits := countingServer(t)
	tt := NewComputeFactorICTool(client.NewFactorClient(srv.URL))

	result, err := tt.Execute(context.Background(), map[string]interface{}{
		"expression": "ts_rank(close, 20)",
		"symbols":    []string{"000001.SZ", "600000.SH"},
		"start_date": "2022-01-01",
		"end_date":   "2024-01-01",
	})

	require.Error(t, err)
	assert.Nil(t, result)
	assert.True(t, errors.Is(err, client.ErrFactorCapabilityUnavailable), "必须是可判定的「能力未提供」哨兵，got %v", err)
	assert.Contains(t, err.Error(), "compute_factor_ic")
	assert.Equal(t, int32(0), atomic.LoadInt32(hits), "不可用路径不得发起 HTTP 请求")
}

func TestComputeFactorICTool_Execute_MissingExpression(t *testing.T) {
	tt := NewComputeFactorICTool(client.NewFactorClient("http://x"))
	args := map[string]interface{}{
		"symbols":    []string{"000001.SZ"},
		"start_date": "2022-01-01",
		"end_date":   "2024-01-01",
	}
	_, err := tt.Execute(context.Background(), args)
	require.Error(t, err)
	assert.True(t, errors.Is(err, tools.ErrInvalidArgs))
	assert.Contains(t, err.Error(), "expression")
}

func TestComputeFactorICTool_Execute_MissingSymbols(t *testing.T) {
	tt := NewComputeFactorICTool(client.NewFactorClient("http://x"))
	args := map[string]interface{}{
		"expression": "close",
		"start_date": "2022-01-01",
		"end_date":   "2024-01-01",
	}
	_, err := tt.Execute(context.Background(), args)
	require.Error(t, err)
	assert.True(t, errors.Is(err, tools.ErrInvalidArgs))
	assert.Contains(t, err.Error(), "symbols")
}

func TestComputeFactorICTool_Execute_MissingStartDate(t *testing.T) {
	tt := NewComputeFactorICTool(client.NewFactorClient("http://x"))
	args := map[string]interface{}{
		"expression": "close",
		"symbols":    []string{"000001.SZ"},
		"end_date":   "2024-01-01",
	}
	_, err := tt.Execute(context.Background(), args)
	require.Error(t, err)
	assert.True(t, errors.Is(err, tools.ErrInvalidArgs))
	assert.Contains(t, err.Error(), "start_date")
}

func TestComputeFactorICTool_Execute_MissingEndDate(t *testing.T) {
	tt := NewComputeFactorICTool(client.NewFactorClient("http://x"))
	args := map[string]interface{}{
		"expression": "close",
		"symbols":    []string{"000001.SZ"},
		"start_date": "2022-01-01",
	}
	_, err := tt.Execute(context.Background(), args)
	require.Error(t, err)
	assert.True(t, errors.Is(err, tools.ErrInvalidArgs))
	assert.Contains(t, err.Error(), "end_date")
}

func TestNewComputeFactorICTool_NilClientPanics(t *testing.T) {
	assert.Panics(t, func() {
		NewComputeFactorICTool(nil)
	})
}

func TestComputeFactorICTool_RegisterInRegistry(t *testing.T) {
	c := client.NewFactorClient("http://x")
	tt := NewComputeFactorICTool(c)
	reg := tools.NewRegistry()
	require.NoError(t, reg.Register(tt))

	got, err := reg.Get("compute_factor_ic")
	require.NoError(t, err)
	assert.Equal(t, "compute_factor_ic", got.Name())

	info := reg.List()
	require.Len(t, info, 1)
	assert.Equal(t, "compute_factor_ic", info[0].Name)
	assert.Len(t, info[0].Parameters, 4)
}

// ═══════════════════════════════════════════════════════════════════════
//  Cross-tool: ValidateFactor + ComputeFactorIC end-to-end flow
// ═══════════════════════════════════════════════════════════════════════

// TestPipeline_ValidateThenComputeIC simulates the L1→L2 gate flow that
// Hermes will use: validate syntax first, then (if valid) compute IC.
// This catches integration regressions early (e.g. a parser change that
// breaks both tools simultaneously).
func TestPipeline_ValidateThenComputeIC(t *testing.T) {
	validateTool := newValidateTool(t)
	icTool := NewComputeFactorICTool(client.NewFactorClient("http://x"))

	args := map[string]interface{}{
		"expression": "ts_rank(close, 20)",
		"symbols":    []string{"000001.SZ"},
		"start_date": "2022-01-01",
		"end_date":   "2024-01-01",
	}

	// L1: validate
	vResult, err := validateTool.Execute(context.Background(), args)
	require.NoError(t, err)
	vMap := vResult.(map[string]interface{})
	require.True(t, vMap["valid"].(bool), "L1 gate: expression should validate")

	// L2: 语法合法 ≠ 能力存在。ADR-041：求值面未提供，工具必须显式失败，
	// 且 failure 不能被误判成 ErrInvalidArgs（L1 已通过）。
	icResult, err := icTool.Execute(context.Background(), args)
	require.Error(t, err)
	assert.Nil(t, icResult)
	assert.True(t, errors.Is(err, client.ErrFactorCapabilityUnavailable))
	assert.False(t, errors.Is(err, tools.ErrInvalidArgs))
}

// L2 的 GateDecision 门限逻辑（低 IC / 零 IC）当前不可达：求值面不存在，
// IC 永远算不出来。门限类型与判据保留在 production 代码里，等 core 真提供
// 求值面后（ADR-041 §6 重开条件）再补回对应用例。
