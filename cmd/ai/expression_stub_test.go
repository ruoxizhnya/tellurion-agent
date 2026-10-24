// expression_stub_test.go —— cmd/ai 集成测试用的 core 表达式端点替身。
//
// validate_factor 已改为 HTTP（QT-17 S3-c-4）。本替身只覆盖集成测试用到的
// 两个判定：合法表达式、缺右括号；目录返回一份最小快照。真实一致性见
// builtin 包的 TestExpressionValidate_ParityWithCore（env-gated）。
package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"tellurion-agent/pkg/ai/client"
)

func newExpressionStub(t *testing.T) *client.ExpressionClient {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/expression/validate", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["expression"] == "ts_rank(close, 20" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"valid": false, "inputs": []string{},
				"error": "unexpected end of expression", "stage": "parse",
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"valid": true, "inputs": []string{"close"}, "ast": body["expression"],
		})
	})
	mux.HandleFunc("/api/expression/catalog", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"availability": "unknown",
			"operators":    []any{},
			"fields": []any{
				map[string]any{"name": "close", "source": "market", "status": "unknown"},
			},
			"thresholds": map[string]any{},
		})
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return client.NewExpressionClient(srv.URL, client.WithInsecureLoopback())
}
