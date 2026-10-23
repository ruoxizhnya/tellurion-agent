package aistore

import (
	"context"
	"fmt"

	"github.com/ruoxizhnya/tellurion-kit/db"
)

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
	if err := db.Migrate(ctx, exec, statements); err != nil {
		return fmt.Errorf("aistore: migrate ai schema: %w", err)
	}
	return nil
}
