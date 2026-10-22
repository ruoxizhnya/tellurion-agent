package agents

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"tellurion-agent/pkg/ai/client"
)

// OBS-01-R2-A 的调用点侧证据：httpBacktestRunner 把三值适配成 (BacktestMetrics, error) 时，
// invalid 既不能读成「成功且零指标」（walk-forward 会把它当成夏普恰好为 0 的正常窗口），
// 也不能读成「和 completed 一样」。

func newBacktestClientStub(t *testing.T, body string) *client.BacktestClient {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	// REV-01-A ③-2：默认 fail-closed；这里的假 core 是本机 loopback，显式开开发模式。
	return client.NewBacktestClient(srv.URL, client.WithInsecureLoopback())
}

// 报文形状是 core 的**线上形状**（REV-01-A ③-1）：POST /api/backtest 回平铺体，
// 没有嵌套 `result` 键。

func TestHTTPBacktestRunner_CompletedYieldsMetrics(t *testing.T) {
	r := NewHTTPBacktestRunner(newBacktestClientStub(t,
		`{"id":"bt-1","status":"completed","sharpe_ratio":1.5,"total_return":0.2}`))

	m, err := r.RunBacktest(context.Background(), "momentum", []string{"600000.SH"}, "2023-01-01", "2024-01-01")
	if err != nil {
		t.Fatalf("completed 是成功路径，got err=%v", err)
	}
	if m.Sharpe != 1.5 || m.Return != 0.2 {
		t.Errorf("指标要照实交出去，got %+v", m)
	}
}

func TestHTTPBacktestRunner_InvalidIsNotSilentZeroMetrics(t *testing.T) {
	r := NewHTTPBacktestRunner(newBacktestClientStub(t,
		`{"id":"bt-2","status":"invalid","sharpe_ratio":0.1,"invalid_reasons":["no_trades"]}`))

	m, err := r.RunBacktest(context.Background(), "momentum", []string{"600000.SH"}, "2023-01-01", "2024-01-01")
	if err == nil {
		t.Fatalf("invalid 必须报错，不能读成成功（got metrics=%+v）", m)
	}
	if m != (BacktestMetrics{}) {
		t.Errorf("invalid 的指标不得参与评分，got %+v", m)
	}
	// 原因必须可见：调用方（和后面看日志的人）要能分清是故障还是被判无效。
	for _, want := range []string{"invalid", "invalid_reasons", "no_trades"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("错误信息要带出 %q，got %v", want, err)
		}
	}
}

func TestHTTPBacktestRunner_FailedStillErrors(t *testing.T) {
	r := NewHTTPBacktestRunner(newBacktestClientStub(t,
		`{"id":"bt-3","status":"failed","error":"calendar unavailable"}`))

	if _, err := r.RunBacktest(context.Background(), "momentum", []string{"600000.SH"}, "2023-01-01", "2024-01-01"); err == nil {
		t.Fatal("failed 必须报错")
	}
}

func TestHTTPBacktestRunner_NilClientIsNilRunner(t *testing.T) {
	// 既有装配语义：nil 客户端 = 不走 HTTP（进程内 / 无网络场景），不能变成「什么都算成功」。
	if r := NewHTTPBacktestRunner(nil); r != nil {
		t.Errorf("nil client 必须返回 nil runner，got %#v", r)
	}
}
