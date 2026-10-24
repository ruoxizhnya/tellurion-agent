package yaml

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"tellurion-agent/pkg/ai/intent"
)

// P0-5 的第二层根因：意图 → 可执行配置之间缺一层映射。
//
// Generator 只在 intent 自带 `signal_expr` 参数时才产出 expression 段；
// 规则解析（ruleBasedExtract）出来的 intent 只带语义类型（momentum /
// mean_reversion / ...），没有 signal_expr。修法：给「能用价量表达」的
// 意图类型一个确定性的默认表达式。映射写在生成的 YAML 里，人能审阅、
// 能改参数；AI 后续调的是旋钮，不是每次重新发明一个策略。
//
// QT-17 S3-d：本包不再把 YAML 加载成本进程内的 ExpressionStrategy
// （loader 已删，执行只在 core）。本文件只断言「生成的 YAML 结构正确 +
// 表达式非空」，闸门由 core_gate_test.go 的 env-gated 对拍负责。

func TestGenerate_PriceExpressibleTypesProduceExpressions(t *testing.T) {
	cases := []struct {
		name string
		typ  intent.StrategyType
	}{
		{"momentum", intent.StrategyTypeMomentum},
		{"mean_reversion", intent.StrategyTypeMeanReversion},
		{"trend_following", intent.StrategyTypeTrendFollowing},
		{"breakout", intent.StrategyTypeBreakout},
		{"multi_factor", intent.StrategyTypeMultiFactor},
	}

	g := NewGenerator()
	core := liveExpressionClient(t)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			i := &intent.Intent{
				StrategyName: "test_" + tc.name,
				StrategyType: tc.typ,
				Universe:     "csi300",
				Timeframe:    "1d",
			}
			yamlStr := g.Generate(i)
			require.NotEmpty(t, yamlStr)

			cfg, err := ParseConfig(yamlStr)
			require.NoError(t, err)
			assert.Equal(t, "test_"+tc.name, cfg.Strategy.Name)
			require.NotEmpty(t, cfg.Expression.Signal.Expression,
				"没有表达式就没有可执行内容")

			requireCoreGate(t, core, "默认表达式", cfg.Expression.Signal.Expression)
		})
	}
}

// TestGenerate_FundamentalTypesProduceRealExpressions：P2-12 之后，
// pe / pb / ps / roe / roa 进了表达式引擎，value / quality 终于能给出
// 诚实表达式了 —— 不再需要「明确失败」这条退路。
//
// 「绝不拿价格糊弄」的保证没有消失，只是挪到了 core 的求值侧 —— 没有财报
// 数据时用 pe 会报错，而不是返回 0。
func TestGenerate_FundamentalTypesProduceRealExpressions(t *testing.T) {
	cases := []struct {
		typ      intent.StrategyType
		wantRefs []string // 表达式里必须出现的字段
		mustNot  string   // 绝不能用这个糊弄
	}{
		{intent.StrategyTypeValue, []string{"pe", "pb"}, ""},
		{intent.StrategyTypeQuality, []string{"roe", "roa"}, ""},
	}

	core := liveExpressionClient(t)
	for _, tc := range cases {
		t.Run(string(tc.typ), func(t *testing.T) {
			i := &intent.Intent{
				StrategyName: "test_" + string(tc.typ),
				StrategyType: tc.typ,
				Universe:     "csi300",
				Timeframe:    "1d",
			}
			cfg, err := ParseConfig(NewGenerator().Generate(i))
			require.NoError(t, err)
			expr := cfg.Expression.Signal.Expression
			require.NotEmpty(t, expr, "value/quality 现在必须能产出表达式")

			for _, ref := range tc.wantRefs {
				assert.Contains(t, expr, ref, "表达式要真的用到 %s，不能拿价格糊弄", ref)
			}
			requireCoreGate(t, core, "基本面默认表达式", expr)
		})
	}
}

// TestGenerate_CustomStillFailsExplicitly：custom 依然无解 —— 它没有语义，
// 编不出诚实表达式，宁可报空也不给默认值。
func TestGenerate_CustomStillFailsExplicitly(t *testing.T) {
	i := &intent.Intent{
		StrategyName: "test_custom",
		StrategyType: intent.StrategyTypeCustom,
		Universe:     "csi300",
		Timeframe:    "1d",
	}
	cfg, err := ParseConfig(NewGenerator().Generate(i))
	require.NoError(t, err)
	assert.Empty(t, cfg.Expression.Signal.Expression,
		"custom 没有语义，必须明确不给表达式而不是随便给一个")
}

// TestGenerate_ExplicitSignalExprWins：显式指定的表达式优先于类型默认。
func TestGenerate_ExplicitSignalExprWins(t *testing.T) {
	i := &intent.Intent{
		StrategyName: "explicit",
		StrategyType: intent.StrategyTypeMomentum,
		Parameters:   []intent.Parameter{{Name: "signal_expr", Value: "cs_rank(volume) > 0.9"}},
	}
	cfg, err := ParseConfig(NewGenerator().Generate(i))
	require.NoError(t, err)
	assert.Equal(t, "cs_rank(volume) > 0.9", cfg.Expression.Signal.Expression)
}
