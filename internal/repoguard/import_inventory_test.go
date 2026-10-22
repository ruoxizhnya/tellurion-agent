package repoguard

import (
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
)

const inventoryDoc = "docs/contracts-import-replacement.md"

var inventoryMapping = map[string]string{
	"tellurion/pkg/domain":             "github.com/ruoxizhnya/tellurion-contracts/domain",
	"tellurion/pkg/backtest/contracts": "github.com/ruoxizhnya/tellurion-contracts/engine",
	"tellurion/pkg/fees":               "github.com/ruoxizhnya/tellurion-contracts/fees",
}

type inventoryRow struct {
	file string
	line int
	old  string
}

func (r inventoryRow) key() string {
	return fmt.Sprintf("%s:%d\t%s", r.file, r.line, r.old)
}

var (
	inventoryRowRE = regexp.MustCompile("^\\|\\s*\\d+\\s*\\|\\s*`([^`]+\\.go)`\\s*\\|\\s*(\\d+)\\s*\\|\\s*`([^`]+)`\\s*\\|\\s*`([^`]+)`\\s*\\|\\s*$")
	summaryRowRE   = regexp.MustCompile("^\\|\\s*([ABC])\\s*\\|\\s*`([^`]+)`\\s*\\|\\s*`([^`]+)`\\s*\\|\\s*(\\d+)\\s*\\|\\s*(\\d+)\\s*\\|\\s*$")
	summaryTotalRE = regexp.MustCompile("^\\|\\s*合计\\s*\\|\\s*—\\s*\\|\\s*—\\s*\\|\\s*(\\d+)\\s*\\|\\s*(\\d+)\\s*\\|\\s*$")
	nonImportRowRE = regexp.MustCompile("^\\|\\s*`([^`]+\\.go)`\\s*\\|\\s*(\\d+)\\s*\\|\\s*`([^`]+)`\\s*\\|")
)

func TestImportInventory(t *testing.T) {
	root := repoRoot(t)
	docPath := filepath.Join(root, inventoryDoc)
	doc, err := os.ReadFile(docPath)
	if err != nil {
		t.Fatalf("读清单失败（B4 执行后应删除本护栏并归档清单）：%v", err)
	}

	rows, nonImports := parseInventory(t, string(doc))
	actualImports, domainMarketImports := scanImports(t, root)
	actualText := scanTextOccurrences(t, root)

	t.Run("import 行与源码树逐条一致", func(t *testing.T) {
		assertSameSites(t, rows, actualImports)
	})
	t.Run("汇总计数与清单行一致", func(t *testing.T) {
		assertSummary(t, string(doc), rows)
	})
	t.Run("旧路径的每一处文本都被登记", func(t *testing.T) {
		assertTextCoverage(t, root, actualImports, actualText, nonImports)
	})
	t.Run("agent 无 domain/market 直接 import", func(t *testing.T) {
		for key := range domainMarketImports {
			t.Errorf("%s 直接 import 了 domain/market；清单声称 0 处", key)
		}
	})
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller 失败，无法定位仓库根")
	}
	return filepath.Dir(filepath.Dir(filepath.Dir(thisFile)))
}

func parseInventory(t *testing.T, doc string) ([]inventoryRow, []inventoryRow) {
	t.Helper()
	var rows, nonImports []inventoryRow
	for _, line := range strings.Split(doc, "\n") {
		if m := inventoryRowRE.FindStringSubmatch(line); m != nil {
			lineNo, err := strconv.Atoi(m[2])
			if err != nil {
				t.Fatalf("清单行号不可解析：%q", line)
			}
			want, ok := inventoryMapping[m[3]]
			if !ok {
				t.Errorf("清单出现未登记旧路径 %q：%s", m[3], line)
				continue
			}
			if m[4] != want {
				t.Errorf("%s 的替换目标是 %q，应为 %q", m[1], m[4], want)
				continue
			}
			rows = append(rows, inventoryRow{file: m[1], line: lineNo, old: m[3]})
			continue
		}
		if m := nonImportRowRE.FindStringSubmatch(line); m != nil {
			lineNo, err := strconv.Atoi(m[2])
			if err != nil {
				t.Fatalf("非 import 行号不可解析：%q", line)
			}
			if _, ok := inventoryMapping[m[3]]; !ok {
				t.Errorf("非 import 表出现未登记旧路径 %q：%s", m[3], line)
				continue
			}
			nonImports = append(nonImports, inventoryRow{file: m[1], line: lineNo, old: m[3]})
		}
	}
	if len(rows) == 0 {
		t.Fatal("清单里没有解析到任何 import 行；表头或行格式被改坏了")
	}
	return rows, nonImports
}

func assertSummary(t *testing.T, doc string, rows []inventoryRow) {
	t.Helper()
	seenGroups := map[string]bool{}
	var totalSites, totalFiles *int
	for _, line := range strings.Split(doc, "\n") {
		if m := summaryRowRE.FindStringSubmatch(line); m != nil {
			group, oldPath, newPath := m[1], m[2], m[3]
			if seenGroups[group] {
				t.Errorf("汇总表重复出现 %s 组", group)
			}
			sites, _ := strconv.Atoi(m[4])
			files, _ := strconv.Atoi(m[5])
			if want, ok := inventoryMapping[oldPath]; !ok || newPath != want {
				t.Errorf("汇总 %s 的映射是 %q → %q，与清单规则不符", group, oldPath, newPath)
				continue
			}
			var wantSites int
			fileSet := map[string]bool{}
			for _, r := range rows {
				if r.old == oldPath {
					wantSites++
					fileSet[r.file] = true
				}
			}
			if sites != wantSites {
				t.Errorf("汇总 %s 写 %d 个位点，明细表实有 %d 个", group, sites, wantSites)
			}
			if files != len(fileSet) {
				t.Errorf("汇总 %s 写 %d 个文件，明细表实有 %d 个", group, files, len(fileSet))
			}
			seenGroups[group] = true
			continue
		}
		if m := summaryTotalRE.FindStringSubmatch(line); m != nil {
			s, _ := strconv.Atoi(m[1])
			f, _ := strconv.Atoi(m[2])
			totalSites, totalFiles = &s, &f
		}
	}
	for _, g := range []string{"A", "B", "C"} {
		if !seenGroups[g] {
			t.Errorf("汇总表缺少 %s 组", g)
		}
	}
	if totalSites == nil || totalFiles == nil {
		t.Fatal("汇总表缺少合计行")
	}
	fileSet := map[string]bool{}
	for _, r := range rows {
		fileSet[r.file] = true
	}
	if *totalSites != len(rows) {
		t.Errorf("合计写 %d 个位点，明细表实有 %d 个", *totalSites, len(rows))
	}
	if *totalFiles != len(fileSet) {
		t.Errorf("合计写 %d 个文件，明细表实有 %d 个", *totalFiles, len(fileSet))
	}
}

func assertSameSites(t *testing.T, rows []inventoryRow, actual map[string]inventoryRow) {
	t.Helper()
	if len(actual) == 0 {
		t.Fatal("旧 import 已清零：B4 应已执行——删除本护栏，并把 docs/contracts-import-replacement.md 标注完成后归档")
	}
	documented := map[string]inventoryRow{}
	for _, r := range rows {
		if _, dup := documented[r.key()]; dup {
			t.Errorf("清单重复登记同一 import：%s", r.key())
		}
		documented[r.key()] = r
	}
	var missing, extra []string
	for k := range actual {
		if _, ok := documented[k]; !ok {
			missing = append(missing, k)
		}
	}
	for k := range documented {
		if _, ok := actual[k]; !ok {
			extra = append(extra, k)
		}
	}
	sort.Strings(missing)
	sort.Strings(extra)
	if len(missing) > 0 {
		t.Errorf("源码里存在清单未登记的旧 import（%d 处）：%s", len(missing), strings.Join(missing, "； "))
	}
	if len(extra) > 0 {
		t.Errorf("清单登记了源码中不存在的旧 import（%d 处，行号漂移或已替换）：%s", len(extra), strings.Join(extra, "； "))
	}
}

func assertTextCoverage(t *testing.T, root string, imports map[string]inventoryRow, text map[string]inventoryRow, nonImports []inventoryRow) {
	t.Helper()
	documented := map[string]bool{}
	for _, r := range nonImports {
		if documented[r.key()] {
			t.Errorf("清单重复登记同一非 import 落点：%s", r.key())
		}
		documented[r.key()] = true
		if _, isImport := imports[r.key()]; isImport {
			t.Errorf("%s 被登记为非 import，但源码里它是真实 import", r.key())
		}
		line, err := readLine(filepath.Join(root, r.file), r.line)
		if err != nil {
			t.Errorf("非 import 落点读不到：%s：%v", r.key(), err)
			continue
		}
		if !strings.Contains(line, r.old) {
			t.Errorf("非 import 落点已漂移：%s 第 %d 行不再含 %q（现为 %q）", r.file, r.line, r.old, strings.TrimSpace(line))
		}
	}
	var uncovered []string
	for k := range text {
		if _, isImport := imports[k]; isImport {
			continue
		}
		if !documented[k] {
			uncovered = append(uncovered, k)
		}
	}
	sort.Strings(uncovered)
	if len(uncovered) > 0 {
		t.Errorf("旧路径出现在未登记的文本里（%d 处；换掉或登记进 §6）：%s", len(uncovered), strings.Join(uncovered, "； "))
	}
}

func scanImports(t *testing.T, root string) (map[string]inventoryRow, map[string]bool) {
	t.Helper()
	fset := token.NewFileSet()
	sites := map[string]inventoryRow{}
	forbidden := map[string]bool{}
	walkModuleGoFiles(t, root, func(rel, path string) error {
		file, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
		for _, imp := range file.Imports {
			importPath, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				continue
			}
			line := fset.Position(imp.Pos()).Line
			if importPath == "tellurion/pkg/domain/market" {
				forbidden[fmt.Sprintf("%s:%d", rel, line)] = true
				continue
			}
			if _, ok := inventoryMapping[importPath]; !ok {
				continue
			}
			row := inventoryRow{file: rel, line: line, old: importPath}
			sites[row.key()] = row
		}
		return nil
	})
	return sites, forbidden
}

func scanTextOccurrences(t *testing.T, root string) map[string]inventoryRow {
	t.Helper()
	sites := map[string]inventoryRow{}
	walkModuleGoFiles(t, root, func(rel, path string) error {
		// 护栏自己就在这个测试包里，常量里必然出现旧路径；test-only 包整包跳过。
		if strings.HasPrefix(rel, "internal/repoguard/") {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for i, line := range strings.Split(string(content), "\n") {
			for old := range inventoryMapping {
				if strings.Contains(line, old) {
					row := inventoryRow{file: rel, line: i + 1, old: old}
					sites[row.key()] = row
				}
			}
		}
		return nil
	})
	return sites
}

// walkModuleGoFiles 只遍历本模块的 .go 文件。CI 会把 core 仓 checkout 到
// 仓内的 core/（见 .github/workflows/ci.yml），那是一个独立 module，不属于
// 本仓清单，必须按 go.mod 边界跳过。
func walkModuleGoFiles(t *testing.T, root string, fn func(rel, path string) error) {
	t.Helper()
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "vendor", "node_modules":
				return fs.SkipDir
			}
			if path != root {
				if _, statErr := os.Stat(filepath.Join(path, "go.mod")); statErr == nil {
					return fs.SkipDir
				}
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".go") {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		return fn(filepath.ToSlash(rel), path)
	})
	if err != nil {
		t.Fatalf("扫描本模块 .go 文件失败：%v", err)
	}
}

func TestWalkModuleGoFiles_SkipsNestedModules(t *testing.T) {
	root := t.TempDir()
	writeFile := func(rel, content string) {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeFile("main.go", "package main\n")
	writeFile("pkg/deep.go", "package pkg\n")
	writeFile("core/go.mod", "module tellurion\n")
	writeFile("core/handlers.go", "package main\n")
	writeFile("core/pkg/nested.go", "package pkg\n")

	var got []string
	walkModuleGoFiles(t, root, func(rel, _ string) error {
		got = append(got, rel)
		return nil
	})
	sort.Strings(got)
	want := []string{"main.go", "pkg/deep.go"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("嵌套 module 未被跳过：got %v want %v", got, want)
	}
}

func readLine(file string, line int) (string, error) {
	content, err := os.ReadFile(file)
	if err != nil {
		return "", err
	}
	lines := strings.Split(string(content), "\n")
	if line < 1 || line > len(lines) {
		return "", fmt.Errorf("行号 %d 越界（文件共 %d 行）", line, len(lines))
	}
	return lines[line-1], nil
}
