// expression_parity_test.go —— 与真 core 的判定对拍（env-gated）。
//
// 本仓不再持有 DSL parser，所以「agent 的结论 == core 的 Validate」不能靠
// 单测里的替身自证。这条腿在设了 ANALYSIS_URL 时打真 analysis-service：
// 同一批表达式逐例对拍 valid，并确认 `?fields=` 点名的未登记字段不会被
// 从目录里删掉（OBS-08）。
package builtin

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tellurion-agent/pkg/ai/client"
)

func newLiveExpressionClient(t *testing.T) *client.ExpressionClient {
	t.Helper()
	base := os.Getenv("ANALYSIS_URL")
	if base == "" {
		t.Skip("set ANALYSIS_URL to run against a live analysis-service")
	}
	opts := []client.Option{client.WithInsecureLoopback()}
	if token := os.Getenv("ANALYSIS_BEARER_TOKEN"); token != "" {
		opts = append(opts, client.WithBearerToken(token))
	}
	return client.NewExpressionClient(base, opts...)
}

// TestExpressionValidate_ParityWithCore 逐例对拍 core 的 Parse + Validate 结论。
func TestExpressionValidate_ParityWithCore(t *testing.T) {
	c := newLiveExpressionClient(t)
	cases := []struct {
		expr  string
		valid bool
	}{
		{"ts_rank(close, 20)", true},
		{"(close - ts_mean(close, 20)) / ts_std(close, 20)", true},
		{"cs_rank(ts_pct_change(close, 20)) > 0.8", true},
		{"ts_ewma(close, 0.3)", true},
		{"ts_kalman(close, 0.1, 0.5)", true},
		{"CROSS(MA(close, 5), MA(close, 20))", false}, // OBS-06：parse 成功但算子未登记
		{"ts_mean(close)", false},                     // 参数个数不符
		{"nosuchfield + 1", false},                    // 未知字段
		{"ts_rank(close, 20", false},                  // 语法错误
	}
	for _, tc := range cases {
		got, err := c.Validate(context.Background(), tc.expr)
		require.NoError(t, err, "%q: transport/HTTP failure", tc.expr)
		assert.Equalf(t, tc.valid, got.Valid, "%q: verdict drift vs core (stage=%s, error=%s)", tc.expr, got.Stage, got.Error)
		if !tc.valid {
			assert.NotEqualf(t, "", got.Error, "%q: invalid verdict must explain why", tc.expr)
			assert.NotEqualf(t, "", got.Stage, "%q: invalid verdict must name the stage", tc.expr)
		}
	}
}

// TestExpressionCatalog_UnavailableFieldsDoNotDisappear 对拍目录的 OBS-08 语义：
// 点名查询里「未登记」与「已登记但源表为空」都保留为结构化条目。
func TestExpressionCatalog_UnavailableFieldsDoNotDisappear(t *testing.T) {
	c := newLiveExpressionClient(t)
	cat, err := c.Catalog(context.Background(), []string{"close", "pe", "definitely_not_a_field"})
	require.NoError(t, err)
	require.Len(t, cat.Fields, 3, "named fields must be preserved, including unavailable ones")

	byName := map[string]client.ExpressionField{}
	for _, f := range cat.Fields {
		byName[f.Name] = f
	}
	require.Contains(t, byName, "close")
	require.Contains(t, byName, "pe")
	require.Contains(t, byName, "definitely_not_a_field")
	assert.Equal(t, "field_not_registered", byName["definitely_not_a_field"].Reason)
	assert.Contains(t, []string{"available", "unavailable", "unknown"}, byName["close"].Status)
}
