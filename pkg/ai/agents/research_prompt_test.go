package agents

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"tellurion-agent/pkg/ai/client"
)

// TestFactorDSLSyntax_RendersCatalog 是 OBS-08 的护栏：研究提示词里广告的
// 字段与算子清单必须来自 core 的表达式目录（`GET /api/expression/catalog`），
// 不得硬编码（QT-17 S3-c-4：本仓不再 import 注册表）。
//
// 这条护栏的由来：2026-10-09 实测该段曾硬编码，广告了 `market_cap`
// （provider 永不可供的幻影字段 ⇒ AI 产出必然过不了闸门的表达式），
// 同时漏掉 `ps`/`roa`/`revenue`/`profit` 与 9 个算子。
func TestFactorDSLSyntax_RendersCatalog(t *testing.T) {
	catalog := allAvailableCatalog()
	got := factorDSLSyntax(catalog)

	for _, f := range catalog.Fields {
		if f.Source == "group" {
			continue // 分组标签不是数据字段
		}
		if !strings.Contains(got, f.Name) {
			t.Errorf("提示词缺少可用字段 %q（目录有、提示词没广告 ⇒ AI 少用能力）", f.Name)
		}
	}
	for _, op := range catalog.Operators {
		if !strings.Contains(got, op.Name) {
			t.Errorf("提示词缺少算子 %q（目录有、提示词没广告 ⇒ AI 少用能力）", op.Name)
		}
	}

	// 反向：目录里没有的幻影字段不得被广告。
	assertNoPhantomAdvertised(t, got, catalog)
}

// TestFactorDSLSyntax_NilCatalogIsHonest：未注入目录时如实说明「不可用」，
// 不凭记忆编字段清单（那是幻影字段的来源）。
func TestFactorDSLSyntax_NilCatalogIsHonest(t *testing.T) {
	got := factorDSLSyntax(nil)
	require.Contains(t, got, "not available")
	for _, phantom := range dslPhantomFields {
		require.NotContains(t, got, phantom)
	}
}

// dslPhantomFields 是「目录里没有、provider 永不可供」的字段名（OBS-08 反向
// 护栏）。刻意做成单一来源并放这里：这条清单原先在生成侧与研究侧两个测试文件里
// 各抄一份，D5 给 `turnover_rate` 建了注册项后没人同步删，就漂移成了恒红 ——
// `turnover_rate` 是合法可求值字段，「能力层全集」必须广告它，把它当幻影正是
// 本次漂移的根因。
//
// 注意 `volatility` 不在此列 —— 它是 factor 的 *category* 名，不是 DSL 字段。
var dslPhantomFields = []string{"market_cap", "roe_ttm", "eps", "vwap"}

// assertNoPhantomAdvertised 检查提示词没广告幻影字段（清单用单一来源 dslPhantomFields）。
func assertNoPhantomAdvertised(t *testing.T, got string, catalog *client.ExpressionCatalog) {
	t.Helper()
	for _, v := range phantomViolations(got, catalog, dslPhantomFields) {
		t.Error(v)
	}
}

// phantomViolations 是纯判定逻辑（不碰 *testing.T，便于破坏验证）：
// 遍历 phantom 清单，两类违规各出一条可读消息——
//   - stale：某幻影其实已在目录里（清单过时）。OBS-08 本次漂移的形态就是这条，
//     若只保留「广告了不存在字段」那一句，目录加字段后护栏会恒红、最后被人整段
//     删掉；点名「清单过时」才让维护者知道该删的是清单那一项。
//   - advertised：提示词广告了目录确实没有的字段（真·幻影，AI 会产出过不了闸门的表达式）。
func phantomViolations(got string, catalog *client.ExpressionCatalog, phantoms []string) []string {
	available := make(map[string]bool, len(catalog.Fields))
	for _, f := range catalog.Fields {
		available[f.Name] = true
	}
	var out []string
	for _, phantom := range phantoms {
		switch {
		case available[phantom]:
			out = append(out, "幻影清单过时："+phantom+" 已在目录里（应被广告，不该列在 phantoms）—— 从清单移除它")
		case strings.Contains(got, phantom):
			out = append(out, "提示词广告了不存在的字段 "+phantom+"（OBS-08：AI 会产出必然过不了闸门的表达式）")
		}
	}
	return out
}

// TestPhantomGuard_DetectsStaleRegistry 是 phantomViolations 的破坏验证（无需改坏
// 生产代码）：喂一个「目录里确实有」的字段 turnover_rate 当幻影，护栏必须走
// stale 分支点名，而不是假装通过、也不是错误地报「广告了不存在字段」。
// 反过来喂真幻影 market_cap 进一份不含它的文本，应无违规。
func TestPhantomGuard_DetectsStaleRegistry(t *testing.T) {
	catalog := allAvailableCatalog()
	clean := "- Data fields: open, close, turnover_rate, volume_ratio\n"

	// 正证据：turnover_rate 已在目录 ⇒ 把它列进 phantoms 必须被判「过时」。
	stale := phantomViolations(clean, catalog, []string{"turnover_rate"})
	require.Len(t, stale, 1, "已登记字段被当幻影，必须产出一条违规")
	require.Contains(t, stale[0], "过时", "违规应点名『清单过时』，而非『广告了不存在字段』：%s", stale[0])

	// 反证腿：真幻影 + 不含它的文本 ⇒ 零违规（护栏不空转、不误报）。
	require.Empty(t, phantomViolations(clean, catalog, []string{"market_cap", "roe_ttm", "eps", "vwap"}),
		"未注册的幻影出现在不含它的文本里，不该有任何违规")

	// 第三条腿：真幻影确实被广告 ⇒ 报「广告了不存在字段」（护栏仍拦得住真幻影）。
	dirty := "- Data fields: close\n- Note: see market_cap for sizing\n"
	got := phantomViolations(dirty, catalog, []string{"market_cap"})
	require.Len(t, got, 1, "被广告的真幻影必须被抓到")
	require.Contains(t, got[0], "广告了不存在", "真幻影被广告应报此因：%s", got[0])
}

// TestFactorDSLSyntax_MarksUnavailableFields 是 OBS-08 切片 2 的落点护栏：
// 源表为空的字段必须 ① 不出现在「可用字段」清单里，② 被明确标注为
// 「当前无数据、别用」，**不能从清单里消失**。
//
// 只做 ① 不够：AI 会因为「记得 DSL 里有 pe/roe」而反复尝试，等于把一轮试验
// 浪费在注定拿不到数据的公式上（危害被 OBS-01 兜住，但纯属白跑）。
func TestFactorDSLSyntax_MarksUnavailableFields(t *testing.T) {
	got := factorDSLSyntax(testCatalog()) // 行情有数据、财报表为空

	// ① 「Data fields:」那一行只含可用字段。
	//
	// 用**精确字段集合**而非 strings.Contains：实测 Contains("pe") 会命中
	// "open"（o-pe-n）这种子串，把正确输出误判成错误。
	dataLine := lineStartingWith(got, "- Data fields:")
	if dataLine == "" {
		t.Fatal("提示词缺少 Data fields 行")
	}
	listed := fieldSet(dataLine)
	for _, f := range []string{"pe", "pb", "ps", "roe", "roa", "revenue", "profit"} {
		if listed[f] {
			t.Errorf("零数据的字段 %q 出现在可用清单里：%s", f, dataLine)
		}
	}
	for _, f := range []string{"close", "volume", "turnover_rate"} {
		if !listed[f] {
			t.Errorf("有数据的字段 %q 未在可用清单里：%s", f, dataLine)
		}
	}
	if listed["sector"] {
		t.Errorf("分组标签 sector 不该出现在数据字段清单里：%s", dataLine)
	}

	// ② 不可用字段必须被明确点名禁用。
	unusableLine := lineStartingWith(got, "- UNUSABLE NOW")
	if unusableLine == "" {
		t.Fatal("提示词未标注「当前不可用」字段段 —— AI 会对着空数据产垃圾")
	}
	marked := fieldSet(unusableLine)
	for _, f := range []string{"pe", "pb", "ps", "roe", "roa", "revenue", "profit"} {
		if !marked[f] {
			t.Errorf("不可用字段 %q 未被点名禁用：%s", f, unusableLine)
		}
	}
}

// lineStartingWith 取出以 prefix 开头的那一行（用于按行断言，避免
// strings.Contains 被「另一段里的同名字段」误命中）。
func lineStartingWith(s, prefix string) string {
	for _, ln := range strings.Split(s, "\n") {
		if strings.HasPrefix(ln, prefix) {
			return ln
		}
	}
	return ""
}

// fieldSet 把 "- Data fields: a, b, c" 解析成字段名集合（精确匹配）。
func fieldSet(line string) map[string]bool {
	idx := strings.Index(line, ":")
	if idx < 0 {
		return nil
	}
	out := make(map[string]bool)
	for _, f := range strings.Split(line[idx+1:], ",") {
		f = strings.TrimSpace(f)
		if f != "" {
			out[f] = true
		}
	}
	return out
}
