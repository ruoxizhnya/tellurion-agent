package client

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/ruoxizhnya/tellurion-contracts/domain"
)

// walkForwardReportBody 照 core `domain.WalkForwardReport` 的线上形状。
const walkForwardReportBody = `{
	"strategy_id":"expression_template",
	"universe":"csi300",
	"windows":[{"window_index":0,"train_start":"2021-01-01","train_end":"2022-12-31",
		"test_start":"2023-01-01","test_end":"2023-12-31",
		"train_sharpe":1.4,"test_sharpe":1.1,"test_return":0.08,"test_max_drawdown":-0.05,"oos_vs_train":0.79}],
	"avg_test_sharpe":1.1,"avg_test_return":0.08,"avg_test_max_drawdown":-0.05,
	"avg_degradation":0.79,"pass_rate":1,"overall_pass":true,
	"std_test_sharpe":0,"std_degradation":0,
	"overfit_score":0.2,"prob_no_overfit":0.8,"stability_score":0.9
}`

// TestRunWalkForward_RequestShapeAndDecode 钉住 `POST /api/walkforward` 的
// 提交形状（QT-17 S3-c-3 / D24 第 8 类）：strategy_params 与
// walk_forward_params 都要在请求体里，响应按 domain.WalkForwardReport 解。
func TestRunWalkForward_RequestShapeAndDecode(t *testing.T) {
	stub, rawURL := newStubServer(t, walkForwardReportBody)
	c := NewBacktestClient(rawURL, WithInsecureLoopback())

	report, err := c.RunWalkForward(context.Background(), WalkForwardRequest{
		Strategy:       "expression_template",
		StockPool:      []string{"600000.SH"},
		StartDate:      "2021-01-01",
		EndDate:        "2023-12-31",
		StrategyParams: map[string]any{"signal_expr": "close > 50"},
		WalkForwardParams: domain.WalkForwardParams{
			TrainDays: 250, TestDays: 60, StepDays: 60,
		},
	})
	if err != nil {
		t.Fatalf("RunWalkForward: %v", err)
	}

	req := stub.lastRequest(t)
	if req.Method != http.MethodPost || req.Path != "/api/walkforward" {
		t.Fatalf("期望 POST /api/walkforward，实际 %s %s", req.Method, req.Path)
	}

	var body map[string]any
	if err := json.Unmarshal(req.Body, &body); err != nil {
		t.Fatalf("请求体不是 JSON: %v", err)
	}
	if body["strategy"] != "expression_template" {
		t.Errorf("strategy = %v，want expression_template", body["strategy"])
	}
	params, _ := body["strategy_params"].(map[string]any)
	if params["signal_expr"] != "close > 50" {
		t.Errorf("strategy_params.signal_expr = %v，want close > 50", params["signal_expr"])
	}
	// contracts 的 domain.WalkForwardParams **没有 json tag**，线上键就是
	// 字段名本身。刻意不自定义 snake_case 镜像：core 用同一个类型解码，
	// 拼错键名（train_days）会被静默忽略并回落默认窗口 —— 那是最糟的失败。
	wfp, _ := body["walk_forward_params"].(map[string]any)
	if wfp["TrainDays"] != float64(250) || wfp["TestDays"] != float64(60) || wfp["StepDays"] != float64(60) {
		t.Errorf("walk_forward_params = %v，窗口参数没提交对", wfp)
	}

	if report.StrategyID != "expression_template" || len(report.Windows) != 1 {
		t.Fatalf("报告解码不对: %+v", report)
	}
	if report.AvgTestSharpe != 1.1 || report.Windows[0].TestSharpe != 1.1 {
		t.Errorf("窗口指标没透传: %+v", report.Windows[0])
	}
}

func TestRunWalkForward_Non200IsError(t *testing.T) {
	stub, rawURL := newStubServer(t, `{"error":"invalid strategy_params"}`)
	stub.setStatus(http.StatusBadRequest)
	c := NewBacktestClient(rawURL, WithInsecureLoopback())

	_, err := c.RunWalkForward(context.Background(), WalkForwardRequest{
		Strategy: "expression_template", StartDate: "2021-01-01", EndDate: "2023-12-31",
	})
	if err == nil {
		t.Fatal("400 必须变成 error")
	}
	if !strings.Contains(err.Error(), "400") || !strings.Contains(err.Error(), "invalid strategy_params") {
		t.Errorf("错误必须带上状态码与 core 的原文，got %v", err)
	}
}

// TestBacktestClient_ForwardsInboundToken 钉住 QT-17 S3-c-3 的凭证口径：
// 入站请求的 JWT 放进 ctx 后，客户端以调用者身份转发；没有 token 且未开
// loopback 开发模式时 fail-closed（请求根本不发出）。
func TestBacktestClient_ForwardsInboundToken(t *testing.T) {
	stub, rawURL := newStubServer(t, completedFlatBody)
	c := NewBacktestClient(rawURL, WithTokenFromContext())

	ctx := WithBearerTokenInContext(context.Background(), "jwt-from-inbound")
	_, err := c.RunBacktest(ctx, BacktestRequest{
		Strategy: "momentum", StockPool: []string{"600000.SH"},
		StartDate: "2024-01-01", EndDate: "2024-06-01",
	})
	if err != nil {
		t.Fatalf("RunBacktest: %v", err)
	}
	if got := stub.lastRequest(t).Auth; got != "Bearer jwt-from-inbound" {
		t.Errorf("Authorization = %q，want 入站 token 被转发", got)
	}
}

func TestBacktestClient_NoTokenFailsClosed(t *testing.T) {
	stub, rawURL := newStubServer(t, completedFlatBody)
	c := NewBacktestClient(rawURL, WithTokenFromContext())

	_, err := c.RunBacktest(context.Background(), BacktestRequest{
		Strategy: "momentum", StockPool: []string{"600000.SH"},
		StartDate: "2024-01-01", EndDate: "2024-06-01",
	})
	if err == nil {
		t.Fatal("没有凭证时必须 fail-closed")
	}
	if stub.hits() != 0 {
		t.Errorf("凭证缺失时不应发出请求，实际发了 %d 次", stub.hits())
	}
}

func TestBearerTokenFromContext_EmptyIsNoop(t *testing.T) {
	ctx := context.Background()
	if got := BearerTokenFromContext(ctx); got != "" {
		t.Fatalf("空 ctx 应返回空串，got %q", got)
	}
	if got := BearerTokenFromContext(WithBearerTokenInContext(ctx, "  ")); got != "" {
		t.Fatalf("只有空白字符不应变成凭证，got %q", got)
	}
}
