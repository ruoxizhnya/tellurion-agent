package gene_pool

import (
	"go/ast"
	"go/parser"
	"go/token"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// TestSQL_TableRefsAreSchemaQualified 钉住 QT-17 ⑤ 的停写边界：基因池的每一条
// SQL 都必须显式写 ai.<table>。未限定表名会按 search_path 落到 core 的
// public.*，那正是切换前的事故面（见 docs/db-schema.md §5 待定 1）。
//
// 只解析字符串字面量（注释里 "from the … table" 这类英文不参与），不需要库；
// 改动 SQL 时它会先红，而不是等真库腿写脏别人的表。
func TestSQL_TableRefsAreSchemaQualified(t *testing.T) {
	// 裸表名：前面不是 '.' 也不是标识符字符，就算未限定。
	bareTable := regexp.MustCompile(`(?i)(?:^|[^.\w])(factor_genes|strategy_genes)\b`)
	// SQL 关键字大写是本文件的既有写法；小写的 "update status: %w" 那类
	// 错误文案因此不会进来。
	sqlRef := regexp.MustCompile(`\b(?:FROM|INTO|UPDATE)\s+([A-Za-z_][\w.]*)`)
	qualified := regexp.MustCompile(`(?i)ai\.(?:factor_genes|strategy_genes)\b`)

	for _, file := range []string{"factor_pool.go", "strategy_pool.go"} {
		fset := token.NewFileSet()
		parsed, err := parser.ParseFile(fset, file, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", file, err)
		}

		nQualified := 0
		ast.Inspect(parsed, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			src, err := strconv.Unquote(lit.Value)
			if err != nil {
				return true
			}

			if m := bareTable.FindStringSubmatch(src); m != nil {
				t.Errorf("%s: 未限定 schema 的表引用 %q（必须写 ai.<table>）", file, m[1])
			}
			for _, m := range sqlRef.FindAllStringSubmatch(src, -1) {
				table := m[1]
				if strings.EqualFold(table, "SET") {
					continue // ON CONFLICT … DO UPDATE SET
				}
				if !strings.HasPrefix(strings.ToLower(table), "ai.") {
					t.Errorf("%s: 未限定 schema 的表引用 %q（必须写 ai.<table>）", file, table)
				}
			}
			nQualified += len(qualified.FindAllString(src, -1))
			return true
		})

		if nQualified == 0 {
			t.Fatalf("%s: 没有解析到任何 ai.* 表引用 —— 护栏本身失效", file)
		}
	}
}
