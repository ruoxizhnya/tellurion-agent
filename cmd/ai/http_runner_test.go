package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ruoxizhnya/tellurion-contracts/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"tellurion-agent/pkg/ai/client"
)

// runnerStub 记录假 core 收到的请求体，并按给定报文回包。
type runnerStub struct {
	body   string
	status int
	reqs   []map[string]any
}

func newRunnerStub(t *testing.T, body string) (*runnerStub, *HTTPBacktestRunner) {
	t.Helper()
	stub := &runnerStub{body: body, status: http.StatusOK}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var m map[string]any
		_ = json.Unmarshal(raw, &m)
		stub.reqs = append(stub.reqs, m)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(stub.status)
		_, _ = w.Write([]byte(stub.body))
	}))
	t.Cleanup(srv.Close)
	return stub, NewHTTPBacktestRunner(client.NewBacktestClient(srv.URL, client.WithInsecureLoopback()))
}

const runnerCompletedBody = `{"id":"bt-1","status":"completed","strategy":"expression_template",
	"start_date":"2024-01-01","end_date":"2024-06-01","sharpe_ratio":1.2,"total_return":0.1,"total_trades":9}`

// TestHTTPBacktestRunner_SubmitsIndexAndParams 钉住两件事：
//   - universe: csi300 走 index_code，不冒充股票代码（DATA-01）；
//   - 表达式策略经 strategy_params 提交（QT-17 S3-c-3）。
func TestHTTPBacktestRunner_SubmitsIndexAndParams(t *testing.T) {
	stub, runner := newRunnerStub(t, runnerCompletedBody)

	res, err := runner.RunBacktestWithParams(
		context.Background(), "expression_template",
		map[string]any{"signal_expr": "cs_rank(close) > 0.8"},
		[]string{"csi300"}, "2024-01-01", "2024-06-01",
	)
	require.NoError(t, err)
	require.NotNil(t, res)

	require.Len(t, stub.reqs, 1)
	body := stub.reqs[0]
	assert.Equal(t, "expression_template", body["strategy"])
	assert.Equal(t, "csi300", body["index_code"], "指数必须走 index_code")
	_, hasPool := body["stock_pool"]
	assert.False(t, hasPool, "指数形态下不得再提交 stock_pool")
	params, ok := body["strategy_params"].(map[string]any)
	require.True(t, ok, "strategy_params 必须随请求提交")
	assert.Equal(t, "cs_rank(close) > 0.8", params["signal_expr"])
}

// TestHTTPBacktestRunner_InvalidKeepsResult：invalid 是结果属性不是故障，
// 结果必须原样交给 pipeline（它据此判失败并展示 invalid_reasons）。
func TestHTTPBacktestRunner_InvalidKeepsResult(t *testing.T) {
	_, runner := newRunnerStub(t, `{"id":"bt-2","status":"invalid",
		"start_date":"2024-01-01","end_date":"2024-06-01",
		"invalid_reasons":["zero_trades"]}`)

	res, err := runner.RunBacktest(context.Background(), "momentum", []string{"600000.SH"}, "2024-01-01", "2024-06-01")
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Equal(t, []string{"zero_trades"}, res.InvalidReasons)
}

// TestHTTPBacktestRunner_FailedIsError：failed 必须变成 error，不能拿零值
// 结果去打分。
func TestHTTPBacktestRunner_FailedIsError(t *testing.T) {
	_, runner := newRunnerStub(t, `{"id":"bt-3","status":"failed","error":"calendar unavailable"}`)

	_, err := runner.RunBacktest(context.Background(), "momentum", []string{"600000.SH"}, "2024-01-01", "2024-06-01")
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "calendar unavailable"), "下游原因要保留: %v", err)
}

// TestHTTPBacktestRunner_WalkForwardUsesCoarseEndpoint：walk-forward 只提交
// 一次粗粒度请求，不逐窗口远程调用。
func TestHTTPBacktestRunner_WalkForwardUsesCoarseEndpoint(t *testing.T) {
	stub, runner := newRunnerStub(t, `{"strategy_id":"expression_template","windows":[],"avg_test_sharpe":1.0}`)
	wf := NewHTTPWalkForwardRunner(runner.c)

	report, err := wf.RunWalkForward(
		context.Background(), "expression_template", []string{"csi300"},
		"2021-01-01", "2023-12-31",
		domain.WalkForwardParams{TrainDays: 250, TestDays: 60, StepDays: 60},
	)
	require.NoError(t, err)
	require.NotNil(t, report)
	assert.Equal(t, "expression_template", report.StrategyID)

	require.Len(t, stub.reqs, 1, "walk-forward 必须是一次请求，不是逐窗口调用")
	body := stub.reqs[0]
	// 键名是 contracts 类型字段名（无 json tag）—— 与 core 的解码侧同源。
	wfp, ok := body["walk_forward_params"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(250), wfp["TrainDays"])
	assert.Equal(t, float64(60), wfp["TestDays"])
}
