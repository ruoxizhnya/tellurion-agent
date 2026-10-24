package agents

// OBS-08 切片 2（prompts 清理）：策略生成的领域知识段护栏。
//
// 这段知识原先躺在零引用的 `pkg/ai/prompts/strategy_generate.txt` 里，清理时
// 判定它**不可重建**（负值 P/E 是亏损不是便宜、故排名用 neg(pe) —— 这是自写
// 的领域洞察，注册表与代码都生成不出来），于是合并进 generate.go 的活提示词。
//
// 本测试守住「合并后知识没丢」：后人改提示词若删掉这些点，测试红。
import (
	"strings"
	"testing"
)

func TestStrategyDomainGuidanceKeepsKeyKnowledge(t *testing.T) {
	got := strategyDomainGuidance()

	// 每条都是 generate.go 原提示词没有、而 txt 独有的领域知识。
	must := map[string]string{
		"T+1":            "T+1",
		"涨跌停 ±10%":       "±10%",
		"ST 股 ±5%":       "±5%",
		"禁止卖空":           "Short selling is not available",
		"止损/仓位/回撤":       "stop-loss / position sizing / max-drawdown",
		"PIT 防前视":        "PIT",
		"无财报时 fail-loud": "fails loudly",
		"非正值为 NaN":       "NaN when non-positive",
		"负 PE 是亏损不是便宜":   "losing money, NOT that it is cheap",
		"排名用 neg(pe)":    "neg(pe)",
	}
	for label, needle := range must {
		if !strings.Contains(got, needle) {
			t.Errorf("领域知识段丢失 %q（期望含 %q）—— 这是自写的洞察，不是可重建的资产", label, needle)
		}
	}
}

// TestStrategyPromptIncludesDSLSyntax 策略生成提示词必须带上从 core 目录派生的
// DSL 语法段（QT-17 S3-c-4：字段/算子清单来自 GET /api/expression/catalog，
// 不再是本仓硬编码或 import 注册表）。
func TestStrategyPromptIncludesDSLSyntax(t *testing.T) {
	catalog := allAvailableCatalog()
	got := factorDSLSyntax(catalog) // 生成侧与研究侧共用同一段派生语法

	for _, op := range catalog.Operators {
		if !strings.Contains(got, op.Name) {
			t.Errorf("策略提示词的 DSL 段缺少算子 %q", op.Name)
		}
	}
	for _, f := range catalog.Fields {
		if f.Source == "group" {
			continue
		}
		if !strings.Contains(got, f.Name) {
			t.Errorf("策略提示词的 DSL 段缺少字段 %q", f.Name)
		}
	}
	// 幻影字段不得出现（OBS-08：广告不存在的字段 = AI 必然产出过不了闸门的表达式）。
	// 清单单一来源 + 过时自检见 research_prompt_test.go 的 assertNoPhantomAdvertised。
	assertNoPhantomAdvertised(t, got, catalog)
}
