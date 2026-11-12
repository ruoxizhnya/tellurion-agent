package main

// 鉴权三态回归护栏（安全缺陷：鉴权未启用时默认放行）。
//
// 三态口径（六仓统一）：
//  1. 有 JWT_SECRET                    → 全闸；
//  2. 无 secret + 回环绑定 + 无显式声明 → 进程能起，写面/工具执行 503
//     auth_unavailable，读面公开；
//  3. 无 secret + AUTH_INSECURE_EXPOSURE=loopback-published
//     （且 AUTH_INSECURE=true）        → 明确放开，启动 WARN。
//
// 非回环绑定 + 无 secret + 无声明 → 直接拒绝启动（本轮修复的正是旧实现
// 「0.0.0.0 反而什么门都不装」）。

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	kitauth "github.com/ruoxizhnya/tellurion-kit/auth"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecideAuthStartup_ThreeStates(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name          string
		secret        string
		allowInsecure bool
		host          string
		exposure      string
		wantMode      authMode
		wantRefuse    bool
	}{
		{
			name:     "secret set → enforced",
			secret:   "s3cr3t",
			wantMode: authModeEnforced,
		},
		{
			name:          "no secret, no opt-in → refuse",
			allowInsecure: false,
			host:          "127.0.0.1",
			wantRefuse:    true,
		},
		{
			name:          "no secret, loopback bind, no declaration → unavailable",
			allowInsecure: true,
			host:          "127.0.0.1",
			wantMode:      authModeUnavailable,
		},
		{
			// 本轮缺陷：容器绑 0.0.0.0 时旧实现不装任何门。现在必须拒绝启动。
			name:          "no secret, non-loopback bind, no declaration → refuse",
			allowInsecure: true,
			host:          "0.0.0.0",
			wantRefuse:    true,
		},
		{
			name:          "no secret, declared loopback-published → open",
			allowInsecure: true,
			host:          "0.0.0.0",
			exposure:      insecureExposureLoopbackPublished,
			wantMode:      authModeOpen,
		},
		{
			name:          "no secret, loopback bind + declaration → open",
			allowInsecure: true,
			host:          "127.0.0.1",
			exposure:      insecureExposureLoopbackPublished,
			wantMode:      authModeOpen,
		},
		{
			name:          "misspelled exposure → not a declaration",
			allowInsecure: true,
			host:          "0.0.0.0",
			exposure:      "loopback",
			wantRefuse:    true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			d := decideAuthStartup(tc.secret, tc.allowInsecure, tc.host, tc.exposure)
			assert.Equal(t, tc.wantRefuse, d.Refuse, "reason=%s", d.Reason)
			if tc.wantRefuse {
				assert.NotEmpty(t, d.Reason, "refusal must explain how to fix it")
				return
			}
			assert.Equal(t, tc.wantMode, d.Mode)
			if tc.wantMode == authModeEnforced {
				assert.NotEmpty(t, d.Secret)
			} else {
				assert.Empty(t, d.Secret)
			}
		})
	}
}

// TestOpenAccessWarning_BindAddresses 钉住第 3 态启动 WARN 的文案口径：
// 必须同时说出「写面无鉴权」与实际绑定地址；非回环（含空 host = 全部接口）
// 追加「已对外暴露」，回环只写地址；声明值标明是「声明」，且回环时不得再
// 出现 loopback-only 之类与实际绑定矛盾的表述。
func TestOpenAccessWarning_BindAddresses(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		host       string
		wantAddr   string
		wantExpose bool
	}{
		{name: "loopback", host: "127.0.0.1", wantAddr: `"127.0.0.1:8086"`, wantExpose: false},
		{name: "wildcard", host: "0.0.0.0", wantAddr: `"0.0.0.0:8086"`, wantExpose: true},
		{name: "empty-host", host: "", wantAddr: `":8086"`, wantExpose: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			msg := openAccessWarning(tc.host, 8086)

			assert.Contains(t, msg, "写面无鉴权", "WARN 必须点名写面无鉴权")
			assert.Contains(t, msg, tc.wantAddr, "WARN 必须插值实际绑定地址")
			assert.Contains(t, msg, "declaration", "声明值必须标明是声明")
			assert.Contains(t, msg, "AUTH_INSECURE_EXPOSURE=loopback-published")
			assert.NotContains(t, msg, "loopback-only", "不得再用与实际绑定矛盾的 loopback-only 表述")

			if tc.wantExpose {
				assert.Contains(t, msg, "已对外暴露")
				assert.Contains(t, msg, "non-loopback")
				assert.NotContains(t, msg, "is loopback")
			} else {
				assert.NotContains(t, msg, "已对外暴露")
				assert.Contains(t, msg, "is loopback")
				assert.NotContains(t, msg, "non-loopback")
			}
		})
	}
}

func authModeViper(host string) *viper.Viper {
	v := viper.New()
	v.Set("server.host", host)
	v.Set("rate_limit.per_minute", 10000)
	return v
}

// authModeRouter mounts one read and one write stub on the real middleware
// chain so the assertions exercise buildRouter's state selection.
func authModeRouter(svc *kitauth.TokenService, mode authMode, host string) *gin.Engine {
	r := buildRouter(svc, mode, authModeViper(host), zerolog.Nop())
	r.POST("/api/copilot/generate", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	r.POST("/api/tools/save_factor", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	r.GET("/api/pipeline/jobs", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	return r
}

func authModeRequest(t *testing.T, r *gin.Engine, method, path, token string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	// httptest 默认 RemoteAddr 是 192.0.2.1（非回环）；显式钉成回环，
	// 让第 3 态的来源门与本地直跑同语义。
	req.RemoteAddr = "127.0.0.1:54321"
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// TestBuildRouter_State1_Enforced：有 secret 时写面无 token 必须 401。
func TestBuildRouter_State1_Enforced(t *testing.T) {
	t.Parallel()
	svc := kitauth.NewTokenService(kitauth.Config{JWTSecret: []byte("test")})
	r := authModeRouter(svc, authModeEnforced, "127.0.0.1")

	w := authModeRequest(t, r, http.MethodPost, "/api/copilot/generate", "")
	assert.Equal(t, http.StatusUnauthorized, w.Code,
		"enforced mode must reject anonymous writes; body=%s", w.Body.String())

	w = authModeRequest(t, r, http.MethodPost, "/api/copilot/generate",
		tokenFor(t, svc, RoleAdmin))
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestBuildRouter_State2_Unavailable：无 secret + 回环 + 无声明时，
// 写端点（含 POST /api/copilot/generate 与工具执行）503 auth_unavailable，
// 读端点仍公开。
func TestBuildRouter_State2_Unavailable(t *testing.T) {
	t.Parallel()
	svc := kitauth.NewTokenService(kitauth.Config{})
	require.False(t, svc.Enabled())
	r := authModeRouter(svc, authModeUnavailable, "127.0.0.1")

	for _, path := range []string{"/api/copilot/generate", "/api/tools/save_factor"} {
		w := authModeRequest(t, r, http.MethodPost, path, "")
		require.Equal(t, http.StatusServiceUnavailable, w.Code,
			"%s must be 503 when auth is unavailable; body=%s", path, w.Body.String())

		var body map[string]interface{}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
		assert.Equal(t, authUnavailableCode, body["code"], "%s code", path)
	}

	w := authModeRequest(t, r, http.MethodGet, "/api/pipeline/jobs", "")
	assert.Equal(t, http.StatusOK, w.Code,
		"read endpoints stay public in degraded mode; body=%s", w.Body.String())
}

// TestBuildRouter_State3_Open：显式声明放开后写面可执行（回环来源）。
func TestBuildRouter_State3_Open(t *testing.T) {
	t.Parallel()
	svc := kitauth.NewTokenService(kitauth.Config{})
	r := authModeRouter(svc, authModeOpen, "127.0.0.1")

	w := authModeRequest(t, r, http.MethodPost, "/api/copilot/generate", "")
	assert.Equal(t, http.StatusOK, w.Code,
		"declared open-access mode must allow writes; body=%s", w.Body.String())

	// 回环绑定时运行期来源门仍在：非回环来源要拒绝。
	req := httptest.NewRequest(http.MethodPost, "/api/copilot/generate", strings.NewReader(`{}`))
	req.RemoteAddr = "10.0.0.7:12345"
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code,
		"open mode bound to loopback must still reject off-host sources")
}
