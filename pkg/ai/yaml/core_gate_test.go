package yaml

// core_gate_test.go —— 生成器产出的表达式必须过 core 的 DSL 闸门。
//
// QT-17 S3-c-4 之前这里是 `expression.NewParser().Parse() + Validate()`，
// 直接 import core 的实现包。现在闸门只在 core 一处实现，本包改为打
// `POST /api/expression/validate`。用例是 env-gated 的：没设 ANALYSIS_URL
// 时跳过（并在日志里明说），设了就真对拍 —— 不把「没跑」伪装成「通过」。

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"tellurion-agent/pkg/ai/client"
)

// liveExpressionClient 在未配置 ANALYSIS_URL 时跳过调用方用例。
func liveExpressionClient(t *testing.T) *client.ExpressionClient {
	t.Helper()
	base := os.Getenv("ANALYSIS_URL")
	if base == "" {
		t.Skip("set ANALYSIS_URL to run gate parity against a live analysis-service")
	}
	opts := []client.Option{client.WithInsecureLoopback()}
	if token := os.Getenv("ANALYSIS_BEARER_TOKEN"); token != "" {
		opts = append(opts, client.WithBearerToken(token))
	}
	return client.NewExpressionClient(base, opts...)
}

// requireCoreGate 断言 formula 过 core 的 Parse + 算子/字段闸门。
func requireCoreGate(t *testing.T, c *client.ExpressionClient, contextMsg, formula string) {
	t.Helper()
	verdict, err := c.Validate(context.Background(), formula)
	require.NoErrorf(t, err, "%s: validate request failed", contextMsg)
	if !verdict.Valid {
		t.Errorf("%s：产出的表达式过不了 core 闸门 %q（stage=%s, error=%s）",
			contextMsg, formula, verdict.Stage, verdict.Error)
	}
}
