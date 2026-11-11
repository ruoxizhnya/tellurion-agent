package aistore

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// 本文件是 QT-17 ① 的「逐表核对」腿；SVC-CORE-AI-RETIRE（2026-11-10）后
// 语义翻转为**反向护栏**：ai.* 的七张表已由 core 删表交权，本仓需钉住
// 「core 不再建有这七张表」，防止双 owner 回归。
//
// 为什么不在本仓复制一份 core DDL：复制的那份一定会漂移，而 QT-17 的纪律是
// 「以 core 现有定义为准逐表搬，不凭类型名臆造表名」。这里直接读 core 源码；
// 单仓 checkout（找不到 core）时显式 Skip，CI 会 checkout core（见
// .github/workflows/ci.yml），那条腿在 CI 里是跑的。
//
// 列对拍（原 TestSchema_ColumnsMatchCore）已随 core DDL 退役同步退场：core
// 侧没有列可对了，再对就是拿一份不存在的定义当真相。ai.* 从此由本仓自持，
// 列形状以本仓 Statements 为准（如实登记在 QY-17 施工记录里）。

// retiredCoreTables 是已退役的 core 表名 → 本仓承接表名。新增表必须同时改这里
// 与 Tables，或者登记进下面的 agentOwnedTables。
var retiredCoreTables = map[string]string{
	"experiments":         "ai.experiments",
	"factor_genes":        "ai.factor_genes",
	"strategy_genes":      "ai.strategy_genes",
	"research.profile":    "ai.research_profile",
	"research.conclusion": "ai.research_conclusion",
	"research.question":   "ai.research_question",
	"factor_hypothesis":   "ai.factor_hypothesis",
}

// agentOwnedTables 是**本仓自持、core 没有对应物**的表。
//
// QT-17 的「逐表搬、不臆造」管的是搬迁：core 有的表，列必须逐一对上。
// 但它管不了「本仓新造一张 core 从未有过的表」——那类表没有 core 侧的列可比，
// 硬要它出现在 coreToAI 里只能靠臆造一个 core 表名，那正是这条护栏要防的动作。
//
// 所以这里是一条**显式豁免**，不是后门：登记了的表必须仍在 Tables 与 DDL 里
// （见 TestSchema_AgentOwnedTablesAreDeclared），且不得同时出现在 coreToAI 里
// （一张表不能既是搬迁又是自持）。键是本仓表名，值是豁免理由。
var agentOwnedTables = map[string]string{
	"ai.generation_chain": "QT-32-A：AI 生成链的留痕。core 从未记录过生成过程（旧表只有实验日志），无 core 列可对拍。",
	"ai.adoption_ledger":  "AGENT-INTERVENTION-ADOPTION-LEDGER：干预/采纳台账与 holdout 对账集。core 从未有过采纳决策表，无 core 列可对拍。",
}

var columnNameRE = regexp.MustCompile(`^[a-z_][a-z0-9_]*$`)

// stripGoLineComments 去掉 Go 的 `//` 行注释：core 侧的退役说明里会提到表名，
// 那是文档不是建表语句，不剥掉会把反向护栏逼成假红。
func stripGoLineComments(src string) string {
	var b strings.Builder
	for _, line := range strings.Split(src, "\n") {
		if i := strings.Index(line, "//"); i >= 0 {
			line = line[:i]
		}
		b.WriteString(line)
		b.WriteString("\n")
	}
	return b.String()
}

func loadCorePostgres(t *testing.T) (string, string) {
	t.Helper()
	candidates := []string{
		os.Getenv("TELLURION_CORE"),
		// 本地：两仓在工作区同级。
		filepath.Join("..", "..", "..", "tellurion", "pkg", "storage", "postgres.go"),
		// CI：core checkout 到 workspace 根的 core/。
		filepath.Join("..", "..", "..", "core", "pkg", "storage", "postgres.go"),
	}
	for _, path := range candidates {
		if path == "" {
			continue
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		return string(raw), path
	}
	t.Skip("找不到 core 的 pkg/storage/postgres.go（单仓 checkout 的正常情况；用 TELLURION_CORE 指定 core 仓路径可在本地跑这条腿）")
	return "", ""
}

// aiTableColumns 从本仓 DDL 里取出某张表的列名集合。
func aiTableColumns(t *testing.T, table string) map[string]bool {
	t.Helper()
	for _, stmt := range Statements() {
		m := createTableRE.FindStringSubmatch(stmt)
		if m != nil && m[1] == table {
			return columnsOf(m[2])
		}
	}
	t.Fatalf("本仓 DDL 里找不到 %s", table)
	return nil
}

// columnsOf 抽取列名：跳过表级约束行（PRIMARY KEY / UNIQUE / CONSTRAINT）与
// 收尾括号，其余行取首个 token。列名按 PostgreSQL 未加引号标识符的形态校验。
func columnsOf(body string) map[string]bool {
	out := map[string]bool{}
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(line), ","))
		if line == "" || strings.HasPrefix(line, ")") {
			continue
		}
		upper := strings.ToUpper(line)
		if strings.HasPrefix(upper, "PRIMARY KEY") || strings.HasPrefix(upper, "UNIQUE") || strings.HasPrefix(upper, "CONSTRAINT") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 || !columnNameRE.MatchString(fields[0]) {
			continue
		}
		out[fields[0]] = true
	}
	return out
}

// TestSchema_RetiredCoreTablesAreGone 是 SVC-CORE-AI-RETIRE 的反向护栏：
// core 的 postgres.go 里不得再出现这七张表的 CREATE TABLE。core 一旦重建，
// 就又是双 owner，本仓的写者身份不再唯一。
func TestSchema_RetiredCoreTablesAreGone(t *testing.T) {
	src, path := loadCorePostgres(t)
	src = stripGoLineComments(src)

	for coreName, aiName := range retiredCoreTables {
		t.Run(coreName, func(t *testing.T) {
			re := regexp.MustCompile(`(?i)CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?` + regexp.QuoteMeta(coreName) + `\b`)
			if re.MatchString(src) {
				t.Errorf("core 仍保留 %s 的 CREATE TABLE（%s）—— 该表已归本仓 %s；"+
					"core 重建即恢复双 owner（SVC-CORE-AI-RETIRE）", coreName, path, aiName)
			}
		})
	}

	// 正向对照：core 的 DDL 必须仍至少有一张表，否则本护栏可能因「文件被删空」而假绿。
	if !regexp.MustCompile(`(?i)CREATE\s+TABLE\s+IF\s+NOT\s+EXISTS\s+`).MatchString(src) {
		t.Fatalf("%s 里一条 CREATE TABLE 都找不到 —— core 源码读取路径或文件形态已变，本护栏假绿", path)
	}
}

// TestSchema_MappingCoversTables 双向核对退役对照表与 Tables 清单：
// 两边任何一侧多/少，都会让上面那条缺表护栏静默漏表。
// 唯一的例外是显式登记过的本仓自持表（agentOwnedTables）——它没有 core 来源，
// 但**必须**在 Tables 与 DDL 里，见 TestSchema_AgentOwnedTablesAreDeclared。
func TestSchema_MappingCoversTables(t *testing.T) {
	declared := map[string]bool{}
	for _, name := range Tables {
		declared[name] = true
	}
	inMapping := map[string]bool{}
	for _, aiName := range retiredCoreTables {
		inMapping[aiName] = true
		if !declared[aiName] {
			t.Errorf("退役对照表里的 %s 不在 Tables 清单里", aiName)
		}
	}
	for name := range declared {
		if inMapping[name] {
			continue
		}
		if _, exempt := agentOwnedTables[name]; !exempt {
			t.Errorf("Tables 里的 %s 既没有 core 来源（退役对照表），也没登记为本仓自持表 —— 表清单无从核对", name)
		}
	}
}

// TestSchema_AgentOwnedTablesAreDeclared 给豁免本身装牙：豁免名单不许变成后门。
// 四条腿各钉一种夹带方式——
//  1. 登记为自持但没进 Tables：清单外凭空多一张表，清单核对永远看不到它；
//  2. 登记为自持但 DDL 里没有对应 CREATE：账上有、库里没有；
//  3. 同时在 retiredCoreTables 里：一张表不能既是 core 退役承接又是本仓自持（两条判据互斥）；
//  4. 豁免理由为空：没写理由的豁免等于没人能复核。
//
// 这条腿**不读 core 源码**，所以单仓 checkout 也跑（不像
// TestSchema_RetiredCoreTablesAreGone 那样 skip）—— 名单里的表若被删净，这里会红。
func TestSchema_AgentOwnedTablesAreDeclared(t *testing.T) {
	declared := map[string]bool{}
	for _, name := range Tables {
		declared[name] = true
	}
	created := map[string]bool{}
	for _, stmt := range Statements() {
		if m := createTableRE.FindStringSubmatch(stmt); m != nil {
			created[m[1]] = true
		}
	}
	mapped := map[string]bool{}
	for _, aiName := range retiredCoreTables {
		mapped[aiName] = true
	}

	for name, reason := range agentOwnedTables {
		if !declared[name] {
			t.Errorf("%s 登记为本仓自持表，但不在 Tables 清单里", name)
		}
		if !created[name] {
			t.Errorf("%s 登记为本仓自持表，但 Statements 里没有对应的建表语句", name)
		}
		if mapped[name] {
			t.Errorf("%s 同时出现在搬迁对照表与自持豁免名单里：core 搬迁与本仓自持是两条互斥判据", name)
		}
		if strings.TrimSpace(reason) == "" {
			t.Errorf("%s 的自持豁免没有理由 —— 没写理由的豁免无法被复核", name)
		}
	}
}
