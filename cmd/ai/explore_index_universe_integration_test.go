package main

// DATA-01 端到端集成测试：`universe: csi300` → IndexCode → 真票池 → 回测跑通。
//
// 这条测试串起两仓的改动：
//
//	agent 侧 HTTP 回测适配器：universe 里的 "csi300" 识别为指数 → 填 IndexCode
//	core 侧 index_alias + resolveStockPool：csi300 → 000300.SH → 展开成分股
//
// QT-17 S3-c-3 之后本仓不再装配进程内引擎，因此这条腿改成**打活的
// analysis-service**：证据从「同一个进程里跑了一遍」变成「同一条 HTTP
// 请求在 core 里跑通了」。前置（缺任一项即 skip，跳过不是通过）：
//   - ANALYSIS_URL 指向活的 analysis-service（如 http://localhost:8085）
//   - analysis 侧鉴权关闭（loopback 开发）或 ANALYSIS_BEARER_TOKEN 已设
//   - index_constituents 已灌数（POST /sync/index-constituents/000300.SH）
//   - core 的 pkg/strategy/expression 已注册（expression_template 可执行）

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"tellurion-agent/pkg/ai/client"
)

func TestData01_IndexUniverse_EndToEndBacktest(t *testing.T) {
	baseURL := os.Getenv("ANALYSIS_URL")
	if baseURL == "" {
		t.Skip("需要 ANALYSIS_URL（活的 analysis-service）")
	}

	opts := []client.Option{client.WithInsecureLoopback()}
	if token := os.Getenv("ANALYSIS_BEARER_TOKEN"); token != "" {
		opts = append(opts, client.WithBearerToken(token))
	}
	runner := NewHTTPBacktestRunner(client.NewBacktestClient(baseURL, opts...))

	// 区间必须落在 index_constituents 的快照覆盖内（Tushare index_weight
	// 实测覆盖 2024-10-31 ~ 2026-09-30）。区间外会展开为空并 fail-loud ——
	// 那是正确行为，另有测试专测（见 core 的 universe_expand_integration_test.go）。
	//
	// 区间刻意取短（一个季度）：要证的是「票池能展开 + 回测能跑出有效运行」，
	// 不是回测性能。
	res, err := runner.RunBacktest(context.Background(), "momentum", []string{"csi300"}, "2026-01-01", "2026-03-31")
	require.NoError(t, err, "universe:csi300 展开后不应 fail-loud（IndexCode 链路已通）")
	require.NotNil(t, res)

	// 断言只压在**票池展开**上（DATA-01 的目标），不压在「回测有效」上：
	// core 是否注册 momentum 策略与票池展开无关。
	require.NotNil(t, res.UniverseMaxSize, "UniverseMaxSize 必须是测量值而不是 nil（未评估）")
	require.Greater(t, *res.UniverseMaxSize, 0, "票池规模必须 > 0 —— universe:csi300 真的展开成了成分股")

	t.Logf("端到端（HTTP）：universe=[csi300] → 票池规模 %d，trades=%d，invalid_reasons=%v",
		*res.UniverseMaxSize, res.TotalTrades, res.InvalidReasons)
}
