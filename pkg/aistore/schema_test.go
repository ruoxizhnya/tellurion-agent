package aistore

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

var (
	createTableRE = regexp.MustCompile(`(?s)^CREATE TABLE IF NOT EXISTS (\S+) \((.*)\)$`)
	createIndexRE = regexp.MustCompile(`^CREATE INDEX IF NOT EXISTS (\S+) ON (\S+)\(`)
	alterTableRE  = regexp.MustCompile(`^ALTER TABLE (\S+)`)
)

// TestStatements_AreIdempotent 是启动期迁移的前提：core 式迁移没有版本表，
// 每次启动都会全量重跑，任何一条缺 IF NOT EXISTS 的语句都会在第二次启动时炸。
func TestStatements_AreIdempotent(t *testing.T) {
	for i, stmt := range Statements() {
		upper := strings.ToUpper(stmt)
		if !strings.Contains(upper, "IF NOT EXISTS") {
			t.Errorf("statement %d 不幂等（缺 IF NOT EXISTS）：%.80s", i+1, stmt)
		}
	}
}

// TestStatements_OnlyTouchAISchema 钉住所有权边界：本仓只允许建/改 ai schema，
// 出现 public. / research. / quant. 就说明把别家的表搬进来了。
func TestStatements_OnlyTouchAISchema(t *testing.T) {
	for i, stmt := range Statements() {
		upper := strings.ToUpper(strings.TrimSpace(stmt))
		switch {
		case strings.HasPrefix(upper, "CREATE TABLE"):
			m := createTableRE.FindStringSubmatch(stmt)
			if m == nil {
				t.Fatalf("statement %d 无法解析为建表语句：%.80s", i+1, stmt)
			}
			if !strings.HasPrefix(m[1], Schema+".") {
				t.Errorf("statement %d 建了 %s schema 的表（本仓只拥有 %s.*）：%s", i+1, strings.SplitN(m[1], ".", 2)[0], Schema, m[1])
			}
		case strings.HasPrefix(upper, "CREATE INDEX"):
			m := createIndexRE.FindStringSubmatch(stmt)
			if m == nil {
				t.Fatalf("statement %d 无法解析为建索引语句：%.80s", i+1, stmt)
			}
			if !strings.HasPrefix(m[2], Schema+".") {
				t.Errorf("statement %d 在 %s 上建索引（本仓只拥有 %s.*）：%s", i+1, m[2], Schema, m[1])
			}
		case strings.HasPrefix(upper, "ALTER TABLE"):
			m := alterTableRE.FindStringSubmatch(stmt)
			if m == nil || !strings.HasPrefix(m[1], Schema+".") {
				t.Errorf("statement %d 改了 %s 的表（本仓只拥有 %s.*）", i+1, stmt, Schema)
			}
		case strings.HasPrefix(upper, "CREATE SCHEMA"):
			if !strings.Contains(stmt, Schema) {
				t.Errorf("statement %d 建的不是 %s schema：%s", i+1, Schema, stmt)
			}
		default:
			t.Errorf("statement %d 是未预期的语句类型：%.80s", i+1, stmt)
		}
	}
}

// TestTables_MatchStatements 双向核对清单与 DDL：清单少一张 → 拥有表却没登记；
// DDL 少一张 → 登记了却没建。
func TestTables_MatchStatements(t *testing.T) {
	declared := map[string]bool{}
	for _, name := range Tables {
		declared[name] = true
	}

	created := map[string]bool{}
	for i, stmt := range Statements() {
		m := createTableRE.FindStringSubmatch(stmt)
		if m == nil {
			continue
		}
		if created[m[1]] {
			t.Errorf("statement %d 重复建表 %s", i+1, m[1])
		}
		created[m[1]] = true
	}

	for name := range declared {
		if !created[name] {
			t.Errorf("Tables 登记了 %s，但 Statements 里没有对应的建表语句", name)
		}
	}
	for name := range created {
		if !declared[name] {
			t.Errorf("Statements 建了 %s，但 Tables 清单里没有登记", name)
		}
	}
}

// TestStatements_IndexesFollowTheirTables 钉住执行顺序：索引必须排在被索引的表之后，
// 否则空库首跑会在 CREATE INDEX 处红。
func TestStatements_IndexesFollowTheirTables(t *testing.T) {
	seen := map[string]bool{}
	for i, stmt := range Statements() {
		if m := createTableRE.FindStringSubmatch(stmt); m != nil {
			seen[m[1]] = true
			continue
		}
		if m := createIndexRE.FindStringSubmatch(stmt); m != nil {
			if !seen[m[2]] {
				t.Errorf("statement %d 在 %s 建索引，但该表此时还没被创建（顺序反了）", i+1, m[2])
			}
		}
	}
}

// TestStatements_NoForeignSchemaReferences 钉住外键只指向 ai.*：把 core 的
// public/research 表名带进来，双写期一停旧表就整段报错。
func TestStatements_NoForeignSchemaReferences(t *testing.T) {
	for i, stmt := range Statements() {
		for _, foreign := range []string{"research.", "public.", "quant."} {
			if strings.Contains(stmt, foreign) {
				t.Errorf("statement %d 引用了外 schema %q：%s", i+1, foreign, stmt)
			}
		}
		if strings.Contains(stmt, "REFERENCES experiments(") {
			t.Errorf("statement %d 的自引用还指向未限定表名：%s", i+1, stmt)
		}
	}
}

// TestStatements_ReturnsCopy 钉住 API 语义：调用方改不动包内真相。
func TestStatements_ReturnsCopy(t *testing.T) {
	first := Statements()
	original := first[0]
	first[0] = "CREATE TABLE IF NOT EXISTS ai.tampered (id INT)"
	if got := Statements()[0]; got != original {
		t.Fatalf("Statements() 返回的不是副本：%q", got)
	}
}

// failingExecer 记录调用次数并按位置失败，不需要真库就能验证错位报错。
type failingExecer struct {
	calls  int
	failAt int
}

func (f *failingExecer) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	f.calls++
	if f.calls == f.failAt {
		return pgconn.CommandTag{}, errors.New("boom")
	}
	return pgconn.CommandTag{}, nil
}

// TestMigrate_StopsAtFailingStatement 钉住「遇错即停 + 报出位置」：
// 失败后不再执行后续语句，错误里带 kit/db 的 statement 序号。
func TestMigrate_StopsAtFailingStatement(t *testing.T) {
	fake := &failingExecer{failAt: 2}
	err := Migrate(context.Background(), fake)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "statement 2") {
		t.Errorf("错误应点名失败位置 statement 2，got %v", err)
	}
	if fake.calls != 2 {
		t.Errorf("应在第 2 条停止，实际执行了 %d 条", fake.calls)
	}
}

// TestMigrate_RunsEveryStatement 钉住成功路径：所有语句按序执行一次。
func TestMigrate_RunsEveryStatement(t *testing.T) {
	fake := &failingExecer{failAt: -1}
	if err := Migrate(context.Background(), fake); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := len(Statements()); fake.calls != want {
		t.Errorf("执行了 %d 条语句，want %d", fake.calls, want)
	}
}
