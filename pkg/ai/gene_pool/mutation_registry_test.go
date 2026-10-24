// K3b：基因池变异算子候选名单与 DSL 目录的一致性护栏。
//
// 病根（K3 切片 2 审查发现 K3b）：算子名合法集合的单一事实源是 core 的
// 表达式注册表，但 AI 侧多处**写死算子名**：
//   - gene_pool/mutation.go 的 wrapCandidates / unwrapCandidates（本文件守）
//   - ai/yaml/generator.go 的表达式模板（由 generator_gate_test.go 守）
//   - ai/agents 提示词里的示例表达式（示例性质，危害较小，登记未覆盖）
//
// 危害：注册表改名 / 删名后，这些地方会静默产出「过不了 DSL 闸门」的表达式
// ——闸门是 fail-closed 的，于是 AI 白跑一轮、报错还指向不相关的位置。
//
// QT-17 S3-c-4：本包不再 import core 的 expression 包。两条腿改为 env-gated
// 的真 core 对拍（设 ANALYSIS_URL 才跑）：目录腿比对算子名，行为腿跑闸门。
package gene_pool

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"tellurion-agent/pkg/ai/client"
)

func liveExpressionClient(t *testing.T) *client.ExpressionClient {
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

// TestMutationCandidatesAreRegisteredOperators 是 K3b 的名单对齐护栏：
// 变异候选算子名必须都在 core 目录里，否则变异会产出过不了闸门的表达式。
func TestMutationCandidatesAreRegisteredOperators(t *testing.T) {
	c := liveExpressionClient(t)
	catalog, err := c.Catalog(context.Background(), nil)
	require.NoError(t, err, "fetch expression catalog from core")

	registered := make(map[string]bool, len(catalog.Operators))
	for _, op := range catalog.Operators {
		registered[op.Name] = true
	}

	check := func(label string, names []string) {
		if len(names) == 0 {
			t.Errorf("%s 名单为空（测试夹具过期或被误删）", label)
			return
		}
		for _, name := range names {
			if !registered[name] {
				t.Errorf("%s 的候选算子 %q 不在 core DSL 目录——K3b 漂移：它会产出过不了闸门的表达式。"+
					"修法：改成目录里现有的算子名（GET /api/expression/catalog 可查）",
					label, name)
			}
		}
	}
	check("wrapCandidates", wrapCandidates)
	check("unwrapCandidates", unwrapCandidates)
}

// TestMutationCandidatesProduceGatedExpressions 是行为腿：用候选算子包裹一条
// 合法表达式后，结果仍必须能过 core 闸门（光名单在目录里还不够——包裹后的
// 语法也要成立）。
func TestMutationCandidatesProduceGatedExpressions(t *testing.T) {
	c := liveExpressionClient(t)

	base := "cs_rank(ts_pct_change(close, 20)) > 0.8"
	for _, fn := range wrapCandidates {
		wrapped := fn + "(" + base + ")"
		verdict, err := c.Validate(context.Background(), wrapped)
		require.NoError(t, err, "候选算子 %q：validate request failed", fn)
		if !verdict.Valid {
			t.Errorf("候选算子 %q 包裹后过不了闸门 %q（stage=%s, error=%s）",
				fn, wrapped, verdict.Stage, verdict.Error)
		}
	}
}
