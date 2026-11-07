package aistore

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ruoxizhnya/tellurion-kit/db"
)

// migrationLockKey 是本包迁移的互斥键：任意常量，跨进程一致即可。
// 多个测试包（或未来多个进程）同时对同一个库跑建表语句时，PostgreSQL 的
// IF NOT EXISTS 在索引/约束上不是并发安全的（会撞 pg_class 唯一键），
// 因此在事务里先拿 advisory lock，让并发迁移串行。
const migrationLockKey int64 = 7362_0011

// Migrate applies the `ai` schema DDL to exec.
//
// The pool belongs to the caller: this package owns the statements, not the
// connection (ADR-D10 — "agent 自建 store，只用 kit 的连接能力"). exec may be a
// pool, a connection or a transaction, which is also how the integration test
// runs the whole migration inside a transaction it rolls back.
//
// There is no migration table: like core, the DDL is a sequence of idempotent
// statements, so re-running it on an up-to-date database is a no-op. A failure
// stops at the first statement and names its position.
func Migrate(ctx context.Context, exec db.Execer) error {
	// 池入口：整批 DDL 放进一个事务，并用事务级 advisory lock 串行化并发迁移。
	// 池上的每条 Exec 可能落到不同连接，事务同时保证语句落在同一连接上。
	if pool, ok := exec.(*pgxpool.Pool); ok {
		tx, err := pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("aistore: begin migration tx: %w", err)
		}
		defer func() { _ = tx.Rollback(ctx) }()
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, migrationLockKey); err != nil {
			return fmt.Errorf("aistore: acquire migration lock: %w", err)
		}
		if err := db.Migrate(ctx, tx, statements); err != nil {
			return fmt.Errorf("aistore: migrate ai schema: %w", err)
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("aistore: commit ai schema migration: %w", err)
		}
		return nil
	}

	if err := db.Migrate(ctx, exec, statements); err != nil {
		return fmt.Errorf("aistore: migrate ai schema: %w", err)
	}
	return nil
}
