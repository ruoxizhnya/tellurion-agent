// expression_stub_test.go —— validate_factor 的测试替身（core 表达式端点）。
//
// QT-17 S3-c-4 之后，DSL 的 Parse + 闸门只在 core 一处实现。本替身按
// **固定的表达式 → 判定**表回答请求，用途是验证工具的「请求形状 + 判定映射 +
// 失败建议」，不是重新实现闸门。真实判定的一致性由两条腿保证：
//   - core `cmd/analysis/handlers_expression.go` 直接调用 expression.Validate；
//   - env-gated 的 `TestExpressionValidate_ParityWithCore`（设 ANALYSIS_URL 后
//     打真 core，逐例对拍）。
package builtin

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"tellurion-agent/pkg/ai/client"
)

// expressionStubVerdict 是替身为某些表达式给出的固定判定。
type expressionStubVerdict struct {
	valid  bool
	error  string
	stage  string
	inputs []string
	ast    string
}

// fieldsHintSuffix 复刻 core 闸门错误里附带的字段清单（registry.go 的
// `AvailableFieldsHint`），让 OBS-08 的文案断言仍然测到「失败原因带清单」。
const fieldsHintSuffix = "（可用字段：market: close, open, turnover, volume；fundamentals: pe, pb, profit, ps, roa, revenue, roe；group: sector）"

// invalidExpressions 是替身按 core 语义断言的失败用例。
var invalidExpressions = map[string]expressionStubVerdict{
	"ts_rank(close, 20":                  {valid: false, error: "unexpected end of expression", stage: "parse"},
	"CROSS(MA(close, 5), MA(close, 20))": {valid: false, error: "unknown operator CROSS" + fieldsHintSuffix, stage: "gate"},
	"ts_mean(close)":                     {valid: false, error: "operator ts_mean expects 2 arguments" + fieldsHintSuffix, stage: "gate"},
	"nosuchfield + 1":                    {valid: false, error: `unknown data field "nosuchfield"` + fieldsHintSuffix, stage: "gate"},
	"market_cap":                         {valid: false, error: `unknown data field "market_cap"` + fieldsHintSuffix, stage: "gate"},
	"roe_ttm":                            {valid: false, error: `unknown data field "roe_ttm"` + fieldsHintSuffix, stage: "gate"},
	"eps":                                {valid: false, error: `unknown data field "eps"` + fieldsHintSuffix, stage: "gate"},
}

type expressionStub struct {
	server        *httptest.Server
	validateCalls int32
	catalogCalls  int32
	catalogErr    bool
	lastBody      map[string]string
}

func newExpressionStub() *expressionStub {
	s := &expressionStub{}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/expression/validate", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&s.validateCalls, 1)
		raw, _ := io.ReadAll(r.Body)
		var body map[string]string
		_ = json.Unmarshal(raw, &body)
		s.lastBody = body

		expr := body["expression"]
		if verdict, bad := invalidExpressions[expr]; bad {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"valid": false, "inputs": []string{}, "error": verdict.error, "stage": verdict.stage,
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"valid":  true,
			"inputs": stubInputs(expr),
			"ast":    expr,
		})
	})
	mux.HandleFunc("/api/expression/catalog", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&s.catalogCalls, 1)
		if s.catalogErr {
			http.Error(w, "availability probe failed", http.StatusServiceUnavailable)
			return
		}
		_ = json.NewEncoder(w).Encode(stubCatalog())
	})
	s.server = httptest.NewServer(mux)
	return s
}

func (s *expressionStub) Close() { s.server.Close() }

func (s *expressionStub) client() *client.ExpressionClient {
	return client.NewExpressionClient(s.server.URL, client.WithInsecureLoopback())
}

// stubInputs 粗扫表达式里的已知字段名（仅用于断言 inputs 透传）。
func stubInputs(expr string) []string {
	var out []string
	for _, f := range []string{"close", "open", "high", "low", "volume", "turnover", "pe", "pb", "roe", "roa", "revenue", "profit"} {
		if strings.Contains(expr, f) {
			out = append(out, f)
		}
	}
	if out == nil {
		out = []string{}
	}
	return out
}

// stubCatalog 模拟「行情有数据、财报表为空」的真库现状（OBS-08 切片 2）。
func stubCatalog() client.ExpressionCatalog {
	field := func(name, source, status string) client.ExpressionField {
		return client.ExpressionField{Name: name, Source: source, Status: status}
	}
	return client.ExpressionCatalog{
		Availability: "probed",
		Operators: []client.ExpressionOperator{
			{Name: "cs_rank", Category: "cs", Arity: 1},
			{Name: "ts_mean", Category: "ts", Arity: 2},
			{Name: "ts_std", Category: "ts", Arity: 2},
			{Name: "neg", Category: "unary", Arity: 1},
			{Name: "+", Category: "binary", Arity: 2},
		},
		Fields: []client.ExpressionField{
			field("close", "market", "available"),
			field("open", "market", "available"),
			field("volume", "market", "available"),
			field("turnover", "market", "available"),
			field("pe", "fundamentals", "unavailable"),
			field("pb", "fundamentals", "unavailable"),
			field("roe", "fundamentals", "unavailable"),
			field("roa", "fundamentals", "unavailable"),
			field("sector", "group", "unavailable"),
		},
	}
}

// newStubValidateTool 是测试里构造 validate_factor 的唯一入口。
func newStubValidateTool(t *testing.T) (*ValidateFactorTool, *expressionStub) {
	t.Helper()
	stub := newExpressionStub()
	t.Cleanup(stub.Close)
	return NewValidateFactorTool(stub.client()), stub
}

// newValidateTool 给不需要观察替身调用的用例。
func newValidateTool(t *testing.T) *ValidateFactorTool {
	t.Helper()
	tool, _ := newStubValidateTool(t)
	return tool
}
