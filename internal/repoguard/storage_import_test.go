// Package repoguard 持有本仓的结构性护栏测试。
//
// 本文件钉的不变量（QT-17 ②⑤，④ 停写后收紧）：实验域数据由本仓自有的
// aistore（ai.* schema）持有，生产代码不得再借 core 的 *PostgresStore 读写
// 这块数据 —— 即非测试的 .go 文件禁止 import tellurion/pkg/storage，唯一
// 出口是 storageWhitelist 里剩下的那条仪器域过渡边（带理由与过期条件）。
//
// QT-17 ④ 停写切换后，白名单从三条收到一条：
//   - cmd/ai/experiment_dualwrite.go 随停写整体删除（条目失效即红，所以同时删）；
//   - cmd/ai/main.go 改用 kit/db 的 BuildDSN，不再 import storage；
//   - 只剩 handlers_explore.go 的 ListingWindow（仪器域，等 D24 第 7 类动作）。
//
// 这条护栏因此同时是「legacy 旧表停写」的执法者：想再写 public.experiments /
// research.*，就必须重新 import storage，而那条 import 会在这里当场变红。
//
// 判定哲学沿用 core:internal/repoguard/ai_boundary_test.go 的 fail-closed：
// 名列允许的才放行，其余一律红；白名单不是豁免列表，是登记簿 ——
// 条目对应的文件必须存在且真的还 import storage（条目失效即红，
// 防止白名单变成没人记得为什么要有的化石）。
package repoguard

import (
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const storageImportPath = "tellurion/pkg/storage"

// storageWhitelist 是允许 import tellurion/pkg/storage 的生产文件的登记簿。
// key = 仓内 slash 分隔的文件相对路径；value = 为什么还在 + 什么时候消失。
// 新增条目必须在 value 里写清这两件事，否则条目本身就不该存在。
var storageWhitelist = map[string]string{
	// ListingWindow 系列工具读的是仪器域（行情/宇宙）数据，不是实验域；
	// D24 第 7 类动作（core 只读 port）落地后切 port 走。
	// 停写切换后这是**最后一条**：实验域的三条过渡边已全部收回。
	"cmd/ai/handlers_explore.go": "ListingWindow 仪器域列表读 core 存储；过期条件 = D24 第 7 类动作（只读 port）落地",
}

// scanStorageImports 扫描 root 下所有非测试 .go 文件的 import，返回两组结果：
// violations —— 白名单外的 storage import；staleWhitelist —— 白名单里
// 已失效的条目（文件不存在，或文件已不再 import storage：条目该删了）。
// 解析失败按 error 处理：护栏读不懂的文件不能静默放行。
func scanStorageImports(root string, whitelist map[string]string) (violations, stale []string, err error) {
	importsStorage := map[string]bool{} // whitelist key -> really imports storage

	walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, werr error) error {
		if werr != nil {
			return werr
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "node_modules", "testdata", "vendor":
				return fs.SkipDir
			}
			return nil
		}
		name := d.Name()
		// 测试文件按约定不在护栏范围（live_backtest_integration_test 等合法
		// 用 core store 查仪器域是既定事实，登记在案，不在此处裁决）。
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			return nil
		}

		raw, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		file, perr := parser.ParseFile(token.NewFileSet(), path, raw, parser.ImportsOnly)
		if perr != nil {
			return fmt.Errorf("parse %s: %w", path, perr)
		}
		rel, rerr := filepath.Rel(root, path)
		if rerr != nil {
			return rerr
		}
		rel = filepath.ToSlash(rel)

		has := false
		for _, imp := range file.Imports {
			p, uerr := strconv.Unquote(imp.Path.Value)
			if uerr != nil {
				continue
			}
			if p == storageImportPath {
				has = true
				break
			}
		}
		if _, listed := whitelist[rel]; listed {
			importsStorage[rel] = has
			return nil
		}
		if has {
			violations = append(violations, rel+" -> "+storageImportPath+
				"（实验域数据归本仓 aistore 持有；如确需过渡，请在 internal/repoguard 的 storageWhitelist 登记理由与过期条件）")
		}
		return nil
	})
	if walkErr != nil {
		return nil, nil, walkErr
	}

	for key := range whitelist {
		if !importsStorage[key] {
			stale = append(stale, key)
		}
	}
	sort.Strings(violations)
	sort.Strings(stale)
	return violations, stale, nil
}

// TestStorageImportBoundary 是真仓腿：全仓扫一遍，违规与失效白名单都必须为零。
func TestStorageImportBoundary(t *testing.T) {
	t.Parallel()

	root := filepath.Join("..", "..")
	violations, stale, err := scanStorageImports(root, storageWhitelist)
	require.NoError(t, err)

	assert.Empty(t, violations,
		"生产代码不得 import tellurion/pkg/storage（QT-17 ②：实验域数据由本仓 aistore 持有）。违规点：\n%s",
		strings.Join(violations, "\n"))
	assert.Empty(t, stale,
		"storageWhitelist 有条目已失效（文件不存在或已不再 import storage）——把条目删掉，登记簿只留活的：\n%s",
		strings.Join(stale, "\n"))
}

// TestStorageImportBoundary_Fixtures 是护栏自身的正反取证（破坏验证的常驻
// 形态）：在临时目录里造出四种情形，逐条证明扫描器真的分得清。
// 在真仓上手动改坏再还原的破坏腿会被并发提交与 .gitignore 噪声拖累，
// 夹具腿把「改坏 → 红」固化成每次 CI 都跑的证据。
func TestStorageImportBoundary_Fixtures(t *testing.T) {
	t.Parallel()

	t.Run("违规 import 被抓", func(t *testing.T) {
		root := t.TempDir()
		writeFixture(t, filepath.Join(root, "pkg", "evil.go"),
			"package evil\n\nimport _ \"tellurion/pkg/storage\"\n")
		violations, stale, err := scanStorageImports(root, map[string]string{})
		require.NoError(t, err)
		require.Len(t, violations, 1, "白名单外的 storage import 必须红")
		assert.Contains(t, violations[0], "pkg/evil.go")
		assert.Empty(t, stale)
	})

	t.Run("干净文件不误伤", func(t *testing.T) {
		root := t.TempDir()
		writeFixture(t, filepath.Join(root, "pkg", "clean.go"),
			"package clean\n\nimport \"tellurion-agent/pkg/aistore\"\n")
		violations, stale, err := scanStorageImports(root, map[string]string{})
		require.NoError(t, err)
		assert.Empty(t, violations)
		assert.Empty(t, stale)
	})

	t.Run("白名单条目放行", func(t *testing.T) {
		root := t.TempDir()
		writeFixture(t, filepath.Join(root, "cmd", "dualwrite.go"),
			"package main\n\nimport _ \"tellurion/pkg/storage\"\n")
		violations, stale, err := scanStorageImports(root,
			map[string]string{"cmd/dualwrite.go": "双写期；过期条件 = 停写切换"})
		require.NoError(t, err)
		assert.Empty(t, violations, "登记过的过渡边放行")
		assert.Empty(t, stale)
	})

	t.Run("白名单条目不再 import 即失效", func(t *testing.T) {
		root := t.TempDir()
		// 文件还在，但已切走 —— 条目成了化石，必须红。
		writeFixture(t, filepath.Join(root, "cmd", "switched.go"),
			"package main\n\nimport \"tellurion-agent/pkg/aistore\"\n")
		violations, stale, err := scanStorageImports(root,
			map[string]string{"cmd/switched.go": "已过期的条目"})
		require.NoError(t, err)
		assert.Empty(t, violations)
		require.Len(t, stale, 1, "条目不再 import storage 就是失效，要删")
		assert.Equal(t, "cmd/switched.go", stale[0])
	})

	t.Run("白名单条目文件消失即失效", func(t *testing.T) {
		root := t.TempDir()
		violations, stale, err := scanStorageImports(root,
			map[string]string{"cmd/gone.go": "文件已被删除的条目"})
		require.NoError(t, err)
		assert.Empty(t, violations)
		require.Len(t, stale, 1, "条目指向的文件没了，条目必须跟着删")
		assert.Equal(t, "cmd/gone.go", stale[0])
	})

	t.Run("测试文件不在护栏范围", func(t *testing.T) {
		root := t.TempDir()
		writeFixture(t, filepath.Join(root, "pkg", "probe_test.go"),
			"package probe\n\nimport _ \"tellurion/pkg/storage\"\n")
		violations, stale, err := scanStorageImports(root, map[string]string{})
		require.NoError(t, err)
		assert.Empty(t, violations, "测试文件按约定不在本护栏范围")
		assert.Empty(t, stale)
	})
}

// writeFixture 落一个夹具文件（自动建父目录）。
func writeFixture(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}
