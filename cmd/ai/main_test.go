package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestMain sets gin to test mode exactly once for the whole package.
//
// 与 cmd/analysis/main_test.go 同源（AUD-12）：gin 的 mode 是两个包级
// 全局，并行测试各自 SetMode 会数据竞争 —— TestMain 在任何测试 goroutine
// 之前设一次。
func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	os.Exit(m.Run())
}

// TestHealthProbesCanonicalAndAlias 钉住 /healthz（规范名）与 /health（兼容
// 别名）同 handler、同响应体：删掉任一注册都会红（X-PROBE-CANONICAL-HEALTHZ）。
func TestHealthProbesCanonicalAndAlias(t *testing.T) {
	router := gin.New()
	registerHealthProbes(router)

	var bodies []string
	for _, path := range []string{"/healthz", "/health"} {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: status = %d, want 200", path, rec.Code)
		}
		bodies = append(bodies, rec.Body.String())
	}
	if bodies[0] != bodies[1] {
		t.Fatalf("/healthz body %q != /health body %q", bodies[0], bodies[1])
	}
}

// TestHealthProbesArePublicAndUnthrottled 钉住两张豁免表都收了规范名与别名：
// 漏加 /healthz 会让未鉴权探针 401、限流打到探针上（X-PROBE-CANONICAL-HEALTHZ）。
func TestHealthProbesArePublicAndUnthrottled(t *testing.T) {
	for _, path := range []string{"/healthz", "/health"} {
		if !slices.Contains(aiPublicPaths, path) {
			t.Errorf("%s missing from aiPublicPaths", path)
		}
		if !isRateLimitExempt(path) {
			t.Errorf("%s missing from rateLimitExemptPaths", path)
		}
	}
}
