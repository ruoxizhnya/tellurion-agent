package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// REV-01-A ③-2：对 core 的鉴权消费面默认 fail-closed。
// 本文件钉住四件事：无凭证不发请求、loopback 开发模式必须显式开启且只对 loopback 生效、
// 有 token 时带上 Bearer 头、公开的 /health 不适用凭证门槛。

// TestAuth_NoCredentialsFailsClosed：没有 token 也没有显式 loopback 模式 →
// 请求**在本地就被拒绝**（假 core 一次都不该收到）。
func TestAuth_NoCredentialsFailsClosed(t *testing.T) {
	stub, rawURL := newStubServer(t, completedFlatBody)
	c := NewBacktestClient(rawURL)

	ctx := context.Background()
	if outcome, err := c.RunBacktest(ctx, BacktestRequest{Strategy: "momentum"}); err == nil {
		t.Errorf("无凭证时必须拒绝，got outcome=%+v", outcome)
	} else if !strings.Contains(err.Error(), "fail-closed") {
		t.Errorf("错误要说明默认 fail-closed，got %v", err)
	}
	if _, err := c.GetBacktestResult(ctx, "job-9"); err == nil {
		t.Error("GET 通道同样要 fail-closed")
	}
	if got := stub.hits(); got != 0 {
		t.Errorf("拒绝必须发生在发请求之前，假 core 却收到了 %d 次", got)
	}
}

// TestAuth_InsecureLoopbackOptIn：显式开启后，loopback 开发模式可用（这是本地唯一允许的匿名路径）。
func TestAuth_InsecureLoopbackOptIn(t *testing.T) {
	stub, c := newStub(t, completedFlatBody)
	if _, err := c.RunBacktest(context.Background(), BacktestRequest{Strategy: "momentum"}); err != nil {
		t.Fatalf("显式 loopback 开发模式应可用，got %v", err)
	}
	if got := stub.hits(); got != 1 {
		t.Errorf("应当真的发出一次请求，got %d", got)
	}
	if got := stub.lastRequest(t).Auth; got != "" {
		t.Errorf("insecure 模式下不该凭空造一个 Authorization 头，got %q", got)
	}
}

// TestAuth_InsecureOnlyForLoopback：开了开关也不能对远端匿名调用 ——
// 「本地开发」不许悄悄变成「远端降级」。
func TestAuth_InsecureOnlyForLoopback(t *testing.T) {
	c := NewBacktestClient("http://192.0.2.10:8085", WithInsecureLoopback())
	_, err := c.RunBacktest(context.Background(), BacktestRequest{Strategy: "momentum"})
	if err == nil {
		t.Fatal("非 loopback 地址 + insecure 开关必须被拒绝")
	}
	if !strings.Contains(err.Error(), "loopback") {
		t.Errorf("错误要点名只允许 loopback，got %v", err)
	}
}

// TestAuth_BearerTokenIsSent：配了 token 就按 QT-50 ④ 的 JWT 面发 Bearer 头。
func TestAuth_BearerTokenIsSent(t *testing.T) {
	stub, rawURL := newStubServer(t, completedFlatBody)
	c := NewBacktestClient(rawURL, WithBearerToken("jwt-abc"))

	if _, err := c.RunBacktest(context.Background(), BacktestRequest{Strategy: "momentum"}); err != nil {
		t.Fatal(err)
	}
	if got, want := stub.lastRequest(t).Auth, "Bearer jwt-abc"; got != want {
		t.Errorf("Authorization want %q got %q", want, got)
	}
}

// TestAuth_TokenPrecedenceOverInsecure：两者都给时按 token 走（不因为开了开发模式就丢掉凭证）。
func TestAuth_TokenPrecedenceOverInsecure(t *testing.T) {
	stub, rawURL := newStubServer(t, completedFlatBody)
	c := NewBacktestClient(rawURL, WithInsecureLoopback(), WithBearerToken("jwt-abc"))

	if _, err := c.RunBacktest(context.Background(), BacktestRequest{Strategy: "momentum"}); err != nil {
		t.Fatal(err)
	}
	if got, want := stub.lastRequest(t).Auth, "Bearer jwt-abc"; got != want {
		t.Errorf("Authorization want %q got %q", want, got)
	}
}

// TestAuth_HealthDoesNotRequireCredentials：/health 是公开运维面，不该被 API 的凭证门槛拦住。
func TestAuth_HealthDoesNotRequireCredentials(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			t.Errorf("Health 打错路径：%s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	if err := NewBacktestClient(srv.URL).Health(context.Background()); err != nil {
		t.Errorf("无凭证的 /health 应放行，got %v", err)
	}
}
