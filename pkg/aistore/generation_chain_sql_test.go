package aistore

// QT-32-A 的 SQL 形状护栏（静态，无库依赖）。
//
// ai.generation_chain 是本仓自持的新表，core 侧没有对拍对象（见
// schema_core_drift_test.go 的 agentOwnedTables）。少了 core DDL 那把尺子，
// 「加了一列但 store 没写/没读」就变成只有真跑一次库才暴露的洞。这里用
// DDL 当尺子量 SQL：三处列表（INSERT / DO UPDATE SET / SELECT）必须各自
// 覆盖整张表，少一列当场红。
//
// 形状同 pkg/ai/gene_pool/sql_scope_test.go：只解析源码里的字符串字面量，
// 注释不参与。

import (
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const chainSourceFile = "generation_chain.go"

var (
	chainInsertColsRE = regexp.MustCompile(`(?s)INSERT INTO ai\.generation_chain\s*\((.*?)\)\s*\n\s*VALUES`)
	chainSelectColsRE = regexp.MustCompile(`(?s)SELECT\s+(.*?)\s*\n\s+FROM ai\.generation_chain`)
	chainSetRowsRE    = regexp.MustCompile(`(?m)^\s*([a-z_][a-z0-9_]*)\s*=\s*[^,]+,?`)
	chainConflictRE   = regexp.MustCompile(`ON CONFLICT \(([^)]*)\) DO UPDATE SET`)
)

// chainDDLColumns 取 DDL 里的列名集合（含表级约束行的剔除，复用 drift 测试的解析）。
func chainDDLColumns(t *testing.T) []string {
	t.Helper()
	cols := aiTableColumns(t, "ai.generation_chain")
	var out []string
	for c := range cols {
		out = append(out, c)
	}
	sort.Strings(out)
	return out
}

func readChainSource(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(chainSourceFile)
	require.NoError(t, err, "读 %s 失败：这条护栏就是空转", chainSourceFile)
	return string(raw)
}

func splitColumns(s string) []string {
	var out []string
	for _, f := range strings.Fields(strings.NewReplacer("\n", " ", "\t", " ").Replace(s)) {
		f = strings.Trim(f, ",")
		if f == "" {
			continue
		}
		out = append(out, f)
	}
	sort.Strings(out)
	return out
}

// TestGenerationChain_InsertCoversEveryColumn：INSERT 的列必须等于 DDL 的列。
// 加一列不写进 INSERT，行就会带着库默认值落库，读回来是空的 —— 而所有用例
// 看着都「过」。
func TestGenerationChain_InsertCoversEveryColumn(t *testing.T) {
	src := readChainSource(t)
	m := chainInsertColsRE.FindStringSubmatch(src)
	require.NotNil(t, m, "在 %s 里找不到 ai.generation_chain 的 INSERT 列清单（SQL 形态变了，同步本测试）", chainSourceFile)

	want := chainDDLColumns(t)
	got := splitColumns(m[1])
	assert.Equal(t, want, got, "INSERT 列清单与 DDL 的列不一致（新增/漏写/拼错都要在同一笔提交里对齐）")
}

// TestGenerationChain_SelectCoversEveryColumn：读回必须把每一列都取出来，
// 否则那一行「存了但没人读得到」，留痕等于半条。
func TestGenerationChain_SelectCoversEveryColumn(t *testing.T) {
	src := readChainSource(t)
	m := chainSelectColsRE.FindStringSubmatch(src)
	require.NotNil(t, m, "在 %s 里找不到 ai.generation_chain 的 SELECT 列清单", chainSourceFile)

	want := chainDDLColumns(t)
	got := splitColumns(m[1])
	assert.Equal(t, want, got, "SELECT 列清单与 DDL 的列不一致")
}

// TestGenerationChain_UpsertRewritesEveryMutableColumn：ON CONFLICT 的 DO UPDATE
// 必须覆盖除身份列与 created_at 之外的每一列。漏掉的那一列会停在第一次写的值上 ——
// 整行覆盖的语义（见 UpsertGenerationChain 的注释）就名不副实了。
func TestGenerationChain_UpsertRewritesEveryMutableColumn(t *testing.T) {
	src := readChainSource(t)
	conflict := chainConflictRE.FindStringSubmatch(src)
	require.NotNil(t, conflict, "找不到 ON CONFLICT ... DO UPDATE SET：upsert 语义已变")
	// splitColumns 会排序，所以这里比的是「哪两列」而不是书写顺序。
	assert.Equal(t, []string{"attempt", "job_id"}, splitColumns(conflict[1]),
		"冲突键必须是 job_id 与 attempt 两列，与 PRIMARY KEY 一致")

	setBlockStart := strings.Index(src, "DO UPDATE SET")
	require.Greater(t, setBlockStart, 0)
	setBlock := src[setBlockStart+len("DO UPDATE SET"):]
	if end := strings.Index(setBlock, "`"); end >= 0 {
		setBlock = setBlock[:end]
	}

	var assigned []string
	for _, row := range chainSetRowsRE.FindAllStringSubmatch(setBlock, -1) {
		assigned = append(assigned, row[1])
	}
	sort.Strings(assigned)

	var want []string
	for _, c := range chainDDLColumns(t) {
		switch c {
		case "job_id", "attempt", "created_at":
			continue // 身份列与开链时间不参与覆盖
		}
		want = append(want, c)
	}
	assert.Equal(t, want, assigned, "DO UPDATE SET 必须覆盖每一个可变列")
}

// TestGenerationChain_BuildStateVocabularyMatchesDDL 把「常量词表」钉在 DDL 的
// CHECK 上。加一个 GenerationBuild* 常量而没同步 CHECK，真库会在写入时炸；
// 反过来 CHECK 放宽而常量没加，就是库里能出现没人解释得了的状态。
func TestGenerationChain_BuildStateVocabularyMatchesDDL(t *testing.T) {
	var ddl string
	for _, stmt := range Statements() {
		if strings.Contains(stmt, "CREATE TABLE IF NOT EXISTS ai.generation_chain") {
			ddl = stmt
			break
		}
	}
	require.NotEmpty(t, ddl, "DDL 里没有 ai.generation_chain")

	m := regexp.MustCompile(`CHECK \(build_state IN \(([^)]*)\)\)`).FindStringSubmatch(ddl)
	require.NotNil(t, m, "build_state 的 CHECK 找不到或形状变了")

	var allowed []string
	for _, v := range strings.Split(m[1], ",") {
		allowed = append(allowed, strings.Trim(strings.TrimSpace(v), "'"))
	}
	sort.Strings(allowed)

	constants := []string{GenerationBuildNone, GenerationBuildFailed, GenerationBuildPassed}
	sort.Strings(constants)
	assert.Equal(t, allowed, constants,
		"Go 侧的编译状态常量与 DDL CHECK 的取值必须一一对应（两边都要改才是加一种状态）")
}
