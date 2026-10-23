package aistore

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ruoxizhnya/tellurion-kit/db"
)

// TestMigrate_Integration 是真库腿：在空库上首跑、再跑一遍验证幂等，
// 然后逐表核对 information_schema 的列与 DDL 声明一致。
//
// 默认跳过不是"掩盖问题"：本仓 CI 没有 PG service（core 同样约定）。指定
// TEST_DATABASE_DSN 即可在本地 docker compose 上跑——这条腿的价值在于
// 「DDL 语法 + 顺序 + 自引用外键」只有真 PG 才能证明。
func TestMigrate_Integration(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if strings.TrimSpace(dsn) == "" {
		t.Skip("TEST_DATABASE_DSN 未设置：跳过真库腿（本地 docker compose 起库后指定 DSN 可跑）")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	pool, err := db.Connect(ctx, dsn, db.DefaultPoolOptions())
	if err != nil {
		t.Fatalf("连接测试库失败：%v", err)
	}
	defer pool.Close()

	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("首跑迁移失败：%v", err)
	}
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("重跑迁移失败（幂等被破坏）：%v", err)
	}

	for _, qualified := range Tables {
		parts := strings.SplitN(qualified, ".", 2)
		schema, table := parts[0], parts[1]

		var exists bool
		if err := pool.QueryRow(ctx,
			`SELECT EXISTS (
			   SELECT 1 FROM information_schema.tables
			   WHERE table_schema = $1 AND table_name = $2
			 )`, schema, table).Scan(&exists); err != nil {
			t.Fatalf("查询 %s 是否存在失败：%v", qualified, err)
		}
		if !exists {
			t.Errorf("迁移后 %s 仍不存在", qualified)
			continue
		}

		want := map[string]bool{}
		for _, stmt := range Statements() {
			if m := createTableRE.FindStringSubmatch(stmt); m != nil && m[1] == qualified {
				want = columnsOf(m[2])
			}
		}

		rows, err := pool.Query(ctx,
			`SELECT column_name FROM information_schema.columns
			 WHERE table_schema = $1 AND table_name = $2`, schema, table)
		if err != nil {
			t.Fatalf("查询 %s 的列失败：%v", qualified, err)
		}
		got := map[string]bool{}
		for rows.Next() {
			var name string
			if err := rows.Scan(&name); err != nil {
				t.Fatalf("扫描列名失败：%v", err)
			}
			got[name] = true
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			t.Fatalf("遍历列名失败：%v", err)
		}

		for col := range want {
			if !got[col] {
				t.Errorf("%s 缺列 %s", qualified, col)
			}
		}
		for col := range got {
			if !want[col] {
				t.Errorf("%s 多出 DDL 未声明的列 %s", qualified, col)
			}
		}
	}
}
