package agents

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"tellurion-agent/pkg/ai/client"
	"tellurion-agent/pkg/ai/gene_pool"
)

// stubValidator 是 core `POST /api/expression/validate` 的测试替身。
// 判定表只覆盖本文件用到的表达式；真实一致性见 builtin 包的
// TestExpressionValidate_ParityWithCore（env-gated，打真 core）。
type stubValidator struct {
	invalid map[string]string // formula -> gate/parse 失败原因
	inputs  map[string][]string
	err     error
	calls   int
}

func (s *stubValidator) Validate(_ context.Context, expression string) (*client.ExpressionValidation, error) {
	s.calls++
	if s.err != nil {
		return nil, s.err
	}
	if msg, bad := s.invalid[expression]; bad {
		return &client.ExpressionValidation{Valid: false, Inputs: []string{}, Error: msg, Stage: "gate"}, nil
	}
	inputs := append([]string(nil), s.inputs[expression]...)
	if inputs == nil && !strings.Contains(expression, "+") {
		for _, f := range []string{"close", "open", "high", "low", "volume"} {
			if strings.Contains(expression, f) {
				inputs = append(inputs, f)
			}
		}
	}
	return &client.ExpressionValidation{Valid: true, Inputs: inputs, AST: expression}, nil
}

// stubSampler 是 core `POST /api/factor/compute` 的测试替身。
type stubSampler struct {
	values  map[string][]float64
	err     error
	lastReq client.ComputeFactorRequest
	calls   int
}

func (s *stubSampler) ComputeFactor(_ context.Context, req client.ComputeFactorRequest) (map[string][]float64, error) {
	s.calls++
	s.lastReq = req
	if s.err != nil {
		return nil, s.err
	}
	return s.values, nil
}

func newL1Agent(t *testing.T) (*ValidateAgent, *stubValidator) {
	t.Helper()
	v := &stubValidator{
		invalid: map[string]string{"ts_mean(": "unexpected end of expression"},
		inputs:  map[string][]string{"ts_mean(close, 3)": {"close"}},
	}
	return NewValidateAgent(v), v
}

// ---- L1 Validation Tests ----

func TestValidateAgent_NewValidateAgent(t *testing.T) {
	agent, _ := newL1Agent(t)
	require.NotNil(t, agent)
	assert.NotNil(t, agent.icCalc, "icCalc should not be nil")
	assert.NotNil(t, agent.validator, "validator should be wired")
}

func TestValidateAgent_NewValidateAgentWithBacktest(t *testing.T) {
	agent := NewValidateAgentWithBacktest(nil, nil)
	require.NotNil(t, agent)
	assert.Nil(t, agent.btClient, "btClient should be nil when not provided")
	assert.Nil(t, agent.validator, "nil validator is kept nil (L1 fails closed)")
}

func TestValidate_L1Syntax_ValidFormula(t *testing.T) {
	agent, v := newL1Agent(t)
	result, err := agent.Validate(context.Background(), "ts_mean(close, 3)", L1Syntax)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Empty(t, result.Errors)
	assert.Equal(t, L1Syntax, result.Level)
	assert.Equal(t, 1.0, result.Score)
	assert.Equal(t, []string{"close"}, result.Inputs, "inputs come from core's verdict")
	assert.Equal(t, 1, v.calls, "L1 must delegate to the HTTP validator")
}

func TestValidate_L1Syntax_InvalidFormula(t *testing.T) {
	agent, _ := newL1Agent(t)
	result, err := agent.Validate(context.Background(), "ts_mean(", L1Syntax)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotEmpty(t, result.Errors)
	assert.Contains(t, result.Errors[0], "unexpected end of expression")
}

// TestValidate_L1_NoValidatorFailsClosed：没有校验器时不许本地猜一个结论。
func TestValidate_L1_NoValidatorFailsClosed(t *testing.T) {
	agent := NewValidateAgent(nil)
	result, err := agent.Validate(context.Background(), "close", L1Syntax)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotEmpty(t, result.Errors)
	assert.Contains(t, result.Errors[0], "No expression validator")
	assert.Equal(t, 0.0, result.Score)
}

// TestValidate_L1_ServiceErrorIsNotInvalid：传输失败必须在 errors 里点名
// 服务错误，而不是伪装成「公式不合法」。
func TestValidate_L1_ServiceErrorIsNotInvalid(t *testing.T) {
	v := &stubValidator{err: fmt.Errorf("expression validate returned status 503")}
	agent := NewValidateAgent(v)
	result, err := agent.Validate(context.Background(), "close", L1Syntax)
	require.NoError(t, err)
	require.NotEmpty(t, result.Errors)
	assert.Contains(t, result.Errors[0], "Validation service error")
	assert.Contains(t, result.Errors[0], "503")
}

func TestValidate_L1Syntax_ZeroInputs(t *testing.T) {
	agent, _ := newL1Agent(t)
	// A formula with no market data inputs (pure literals) — core returns [].
	result, err := agent.Validate(context.Background(), "5 + 3", L1Syntax)
	require.NoError(t, err)
	require.NotEmpty(t, result.Warnings)
	assert.Contains(t, result.Warnings[0], "no data inputs")
}

// ---- L2 Validation Tests ----

func TestValidate_L2QuickEval_Success(t *testing.T) {
	agent, _ := newL1Agent(t)
	sampler := &stubSampler{values: map[string][]float64{
		"000001.SZ": {1, 2, 3, 4},
		"600000.SH": {2, 4, 6, 8},
	}}
	agent.SetFactorSampler(sampler)

	result, err := agent.Validate(context.Background(), "ts_mean(close, 3)", L2QuickEval)
	require.NoError(t, err)
	assert.Empty(t, result.Errors)
	assert.GreaterOrEqual(t, result.Score, 2.0)
	assert.Equal(t, 1, sampler.calls)
	assert.Equal(t, agent.defaultStockPool, sampler.lastReq.Symbols,
		"L2 samples the configured default pool through core")
	assert.Equal(t, l2StartDate, sampler.lastReq.StartDate)
	assert.Equal(t, l2EndDate, sampler.lastReq.EndDate)
}

func TestValidate_L2QuickEval_NoSampler(t *testing.T) {
	agent, _ := newL1Agent(t)
	result, err := agent.Validate(context.Background(), "ts_mean(close, 3)", L2QuickEval)
	require.NoError(t, err)
	require.NotEmpty(t, result.Warnings)
	assert.Contains(t, result.Warnings[0], "No factor sampler")
}

func TestValidate_L2_ZeroVariance(t *testing.T) {
	agent, _ := newL1Agent(t)
	agent.SetFactorSampler(&stubSampler{values: map[string][]float64{
		"000001.SZ": {100, 100, 100, 100},
	}})
	result, err := agent.Validate(context.Background(), "100.0", L2QuickEval)
	require.NoError(t, err)
	require.NotEmpty(t, result.Errors, "constant values must fail the variance check")
	assert.Contains(t, result.Errors[0], "Zero variance")
}

func TestValidate_L2_SamplerError(t *testing.T) {
	agent, _ := newL1Agent(t)
	agent.SetFactorSampler(&stubSampler{err: fmt.Errorf("factor API returned status 500")})
	result, err := agent.Validate(context.Background(), "ts_mean(close, 3)", L2QuickEval)
	require.NoError(t, err)
	require.NotEmpty(t, result.Errors)
	assert.Contains(t, result.Errors[0], "Evaluation error")
}

func TestValidate_L2_InvalidFormula(t *testing.T) {
	agent, _ := newL1Agent(t)
	agent.SetFactorSampler(&stubSampler{values: map[string][]float64{"000001.SZ": {1, 2}}})
	result, err := agent.Validate(context.Background(), "ts_mean(", L2QuickEval)
	require.NoError(t, err)
	require.NotEmpty(t, result.Errors, "L1 failure must stop the chain before L2")
}

// ---- ValidateGene Tests ----

func TestValidateGene_Success(t *testing.T) {
	agent, _ := newL1Agent(t)
	gene := &gene_pool.FactorGene{ID: "gene-001", Formula: "ts_mean(close, 3)", Status: "pending"}
	result, err := agent.ValidateGene(context.Background(), gene, L1Syntax)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "validated", gene.Status)
	assert.Equal(t, result.IC, gene.IC)
}

func TestValidateGene_Rejected(t *testing.T) {
	agent, _ := newL1Agent(t)
	gene := &gene_pool.FactorGene{ID: "gene-002", Formula: "ts_mean(", Status: "pending"}
	result, err := agent.ValidateGene(context.Background(), gene, L1Syntax)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "rejected", gene.Status)
}

// ---- ComputeFitness / BatchValidate Tests ----

func TestValidate_ComputeFitness_NoClient(t *testing.T) {
	agent, _ := newL1Agent(t)
	agent.SetFactorSampler(&stubSampler{values: map[string][]float64{"000001.SZ": {1, 2, 3}}})
	gene := &gene_pool.FactorGene{ID: "gene-003", Formula: "ts_mean(close, 5)"}
	result, err := agent.ValidateGene(context.Background(), gene, L3StandardBacktest)
	require.NoError(t, err)
	require.NotEmpty(t, result.Warnings)
	assert.Contains(t, result.Warnings, "No backtest client available for L3 validation")
}

func TestValidate_BatchValidate_NoClient(t *testing.T) {
	agent, _ := newL1Agent(t)
	genes := []*gene_pool.FactorGene{
		{ID: "gene-1", Formula: "ts_mean(close, 5)"},
		{ID: "gene-2", Formula: "ts_std(volume, 10)"},
	}
	results, err := agent.BatchValidate(context.Background(), genes, L1Syntax)
	require.NoError(t, err)
	assert.Len(t, results, len(genes))
	for _, g := range genes {
		assert.Contains(t, []string{"validated", "rejected"}, g.Status)
	}
}

// ---- L3/L4 Validation Tests ----

func TestValidate_L3NoClient(t *testing.T) {
	agent, _ := newL1Agent(t)
	agent.SetFactorSampler(&stubSampler{values: map[string][]float64{"000001.SZ": {1, 2, 3}}})
	result, err := agent.Validate(context.Background(), "ts_mean(close, 5)", L3StandardBacktest)
	require.NoError(t, err)
	require.NotEmpty(t, result.Warnings)
	assert.Contains(t, result.Warnings, "No backtest client available for L3 validation")
}

func TestValidate_L4NoClient(t *testing.T) {
	agent, _ := newL1Agent(t)
	agent.SetFactorSampler(&stubSampler{values: map[string][]float64{"000001.SZ": {1, 2, 3}}})
	result, err := agent.Validate(context.Background(), "ts_mean(close, 5)", L4WalkForward)
	require.NoError(t, err)
	require.NotEmpty(t, result.Warnings)
	assert.Contains(t, strings.Join(result.Warnings, "\n"), "L4 validation requires BacktestRunner")
}

// ---- S7-P0-3 (ODR-043-3): ValidateAgent L3 default stock pool ----

// hasAShareSuffix returns true if the symbol looks like an A-share
// stock code (e.g. "000001.SZ", "600519.SH"). Used by the S7-P0-3
// tests to confirm the default pool is no longer US tickers.
func hasAShareSuffix(sym string) bool {
	return strings.HasSuffix(sym, ".SZ") || strings.HasSuffix(sym, ".SH")
}

// TestValidateAgent_DefaultStockPoolIsAShare verifies that the default
// L3 backtest stock pool contains A-share stock codes (suffixed .SZ or
// .SH), not US tickers. Before S7-P0-3 the pool was hardcoded to
// ["AAPL","GOOGL","MSFT"] which cannot resolve in an A-share backtest
// engine and produced misleading L3 validation failures.
func TestValidateAgent_DefaultStockPoolIsAShare(t *testing.T) {
	agent, _ := newL1Agent(t)
	require.NotEmpty(t, agent.defaultStockPool, "default stock pool must not be empty")

	for _, sym := range agent.defaultStockPool {
		assert.True(t, hasAShareSuffix(sym),
			"default stock pool symbol %q must be an A-share code (.SZ/.SH), not a US ticker", sym)
	}
}

// TestValidateAgent_DefaultStockPoolHasNoUSTickers is a direct check
// that the three US tickers removed by S7-P0-3 are absent from the
// default pool.
func TestValidateAgent_DefaultStockPoolHasNoUSTickers(t *testing.T) {
	agent, _ := newL1Agent(t)
	for _, sym := range agent.defaultStockPool {
		assert.NotContains(t, []string{"AAPL", "GOOGL", "MSFT"}, sym,
			"default stock pool must not contain US tickers after S7-P0-3")
	}
}

// TestValidateAgent_SetStockPool_Override verifies that the stock pool
// can be overridden (e.g. for testing or when a caller wants a
// different validation universe). Consistent with the existing
// SetBacktestRunner / SetL4Config override pattern.
func TestValidateAgent_SetStockPool_Override(t *testing.T) {
	agent, _ := newL1Agent(t)
	custom := []string{"000333.SZ", "601318.SH"}
	agent.SetStockPool(custom)

	require.Equal(t, custom, agent.defaultStockPool,
		"SetStockPool must override the default pool")
}

// TestValidateAgent_SourceHasNoUSStockDefaults is a regression guard
// scanning validate.go source: the US tickers must not appear as
// string literals in the source (they were removed by S7-P0-3). If
// this test fails, someone re-introduced the US-stock default.
func TestValidateAgent_SourceHasNoUSStockDefaults(t *testing.T) {
	source, err := os.ReadFile("validate.go")
	require.NoError(t, err, "must be able to read validate.go from the test working dir")

	src := string(source)
	for _, ticker := range []string{`"AAPL"`, `"GOOGL"`, `"MSFT"`} {
		assert.NotContains(t, src, ticker,
			"validate.go must not contain US ticker %s as a string literal; use a.defaultStockPool instead", ticker)
	}
}
