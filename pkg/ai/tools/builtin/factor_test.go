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

// countingServer 统计被调用次数。ADR-041 的「不可用」路径必须在**不发请求**的
// 前提下返回终态错误；用它证明这一点（而不是只看错误文案）。
func countingServer(t *testing.T) (*httptest.Server, *int32) {
	t.Helper()
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		http.Error(w, "factor client must not perform HTTP calls (ADR-041)", http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

// ─── FactorComputeTool tests ──────────────────────────────────────────

func TestFactorComputeTool_Name(t *testing.T) {
	tt := NewFactorComputeTool(client.NewFactorClient("http://x"))
	assert.Equal(t, "factor.compute", tt.Name())
}

func TestFactorComputeTool_Description_MarksUnavailable(t *testing.T) {
	tt := NewFactorComputeTool(client.NewFactorClient("http://x"))
	desc := tt.Description()
	assert.Contains(t, desc, "[unavailable]", "描述必须让 LLM 看到「不可用」，避免反复调用必失败工具")
	assert.Contains(t, desc, "ADR-041")
}

func TestFactorComputeTool_Parameters(t *testing.T) {
	tt := NewFactorComputeTool(client.NewFactorClient("http://x"))
	params := tt.Parameters()
	require.Len(t, params, 4)
	for _, p := range params {
		assert.True(t, p.Required)
	}
	assert.Equal(t, "formula", params[0].Name)
	assert.Equal(t, "symbols", params[1].Name)
}

func TestFactorComputeTool_OutputSchema(t *testing.T) {
	tt := NewFactorComputeTool(client.NewFactorClient("http://x"))
	schema := tt.OutputSchema()
	assert.Equal(t, "object", schema.Type)
}

func TestFactorComputeTool_Execute_Unavailable_NoHTTP(t *testing.T) {
	srv, hits := countingServer(t)
	tt := NewFactorComputeTool(client.NewFactorClient(srv.URL))

	result, err := tt.Execute(context.Background(), map[string]interface{}{
		"formula":    "ts_rank(close, 20)",
		"symbols":    []string{"000001.SZ"},
		"start_date": "2022-01-01",
		"end_date":   "2024-01-01",
	})

	require.Error(t, err)
	assert.Nil(t, result)
	assert.True(t, errors.Is(err, client.ErrFactorCapabilityUnavailable), "必须是可判定的「能力未提供」哨兵，got %v", err)
	assert.Contains(t, err.Error(), "factor.compute")
	assert.Equal(t, int32(0), atomic.LoadInt32(hits), "不可用路径不得发起 HTTP 请求")
}

func TestFactorComputeTool_Execute_MissingFormula(t *testing.T) {
	tt := NewFactorComputeTool(client.NewFactorClient("http://x"))

	_, err := tt.Execute(context.Background(), map[string]interface{}{
		"symbols":    []string{"000001.SZ"},
		"start_date": "2022-01-01",
		"end_date":   "2024-01-01",
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, tools.ErrInvalidArgs))
	assert.Contains(t, err.Error(), "formula")
}

func TestFactorComputeTool_Execute_MissingSymbols(t *testing.T) {
	tt := NewFactorComputeTool(client.NewFactorClient("http://x"))

	_, err := tt.Execute(context.Background(), map[string]interface{}{
		"formula":    "close",
		"start_date": "2022-01-01",
		"end_date":   "2024-01-01",
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, tools.ErrInvalidArgs))
	assert.Contains(t, err.Error(), "symbols")
}

func TestNewFactorComputeTool_NilClientPanics(t *testing.T) {
	assert.Panics(t, func() { NewFactorComputeTool(nil) })
}

// ─── FactorEvaluateTool tests ─────────────────────────────────────────

func TestFactorEvaluateTool_Name(t *testing.T) {
	tt := NewFactorEvaluateTool(client.NewFactorClient("http://x"))
	assert.Equal(t, "factor.evaluate", tt.Name())
}

func TestFactorEvaluateTool_Description_MarksUnavailable(t *testing.T) {
	tt := NewFactorEvaluateTool(client.NewFactorClient("http://x"))
	assert.Contains(t, tt.Description(), "[unavailable]")
}

func TestFactorEvaluateTool_Parameters(t *testing.T) {
	tt := NewFactorEvaluateTool(client.NewFactorClient("http://x"))
	params := tt.Parameters()
	require.Len(t, params, 4)
	for _, p := range params {
		assert.True(t, p.Required)
	}
}

func TestFactorEvaluateTool_OutputSchema(t *testing.T) {
	tt := NewFactorEvaluateTool(client.NewFactorClient("http://x"))
	schema := tt.OutputSchema()
	assert.Equal(t, "object", schema.Type)
	found := map[string]bool{}
	for _, f := range schema.Fields {
		found[f.Name] = true
	}
	assert.True(t, found["ic"])
	assert.True(t, found["ir"])
}

func TestFactorEvaluateTool_Execute_Unavailable_NoHTTP(t *testing.T) {
	srv, hits := countingServer(t)
	tt := NewFactorEvaluateTool(client.NewFactorClient(srv.URL))

	result, err := tt.Execute(context.Background(), map[string]interface{}{
		"formula":    "ts_rank(close, 20)",
		"symbols":    []string{"000001.SZ"},
		"start_date": "2022-01-01",
		"end_date":   "2024-01-01",
	})

	require.Error(t, err)
	assert.Nil(t, result)
	assert.True(t, errors.Is(err, client.ErrFactorCapabilityUnavailable), "got %v", err)
	assert.Contains(t, err.Error(), "factor.evaluate")
	assert.Equal(t, int32(0), atomic.LoadInt32(hits))
}

func TestFactorEvaluateTool_Execute_MissingFormula(t *testing.T) {
	tt := NewFactorEvaluateTool(client.NewFactorClient("http://x"))

	_, err := tt.Execute(context.Background(), map[string]interface{}{
		"symbols":    []string{"000001.SZ"},
		"start_date": "2022-01-01",
		"end_date":   "2024-01-01",
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, tools.ErrInvalidArgs))
}

func TestNewFactorEvaluateTool_NilClientPanics(t *testing.T) {
	assert.Panics(t, func() { NewFactorEvaluateTool(nil) })
}

// ─── Registry integration ─────────────────────────────────────────────

func TestFactorTools_RegisterInRegistry(t *testing.T) {
	c := client.NewFactorClient("http://x")
	compute := NewFactorComputeTool(c)
	evaluate := NewFactorEvaluateTool(c)

	reg := tools.NewRegistry()
	require.NoError(t, reg.Register(compute))
	require.NoError(t, reg.Register(evaluate))

	list := reg.List()
	require.Len(t, list, 2)
	assert.Equal(t, "factor.compute", list[0].Name)
	assert.Equal(t, "factor.evaluate", list[1].Name)
}
