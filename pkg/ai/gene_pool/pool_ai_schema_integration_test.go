package gene_pool

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	kitdb "github.com/ruoxizhnya/tellurion-kit/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"tellurion-agent/pkg/aistore"
)

// TestPools_WriteOnlyAiSchema 是 QT-17 ⑤ 切换（基因池读写切 ai.*）的真库取证腿。
//
// 不变量是**单向**的：真把两个 pool 跑一遍 CRUD 后，
//   - ai.factor_genes / ai.strategy_genes 必须有且只有这些行；
//   - legacy public.* 的同 id 行必须为 0（旧表可能存在也可能不存在，两种都算通过）。
//
// TEST_DATABASE_DSN 未设置就 skip（与 pkg/aistore 的真库腿同一套约定）。
func TestPools_WriteOnlyAiSchema(t *testing.T) {
	ctx := context.Background()
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if strings.TrimSpace(dsn) == "" {
		t.Skip("TEST_DATABASE_DSN 未设置：跳过真库腿（本地 docker compose 起库后指定 DSN 可跑）")
	}

	pool, err := kitdb.Connect(ctx, dsn, kitdb.DefaultPoolOptions())
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	require.NoError(t, aistore.Migrate(ctx, pool))

	factorPool := NewFactorPool(pool)
	strategyPool := NewStrategyPool(pool)

	const factorID = "itest_factor_gene_ai_scope"
	const strategyID = "itest_strategy_gene_ai_scope"
	now := time.Now().UTC().Truncate(time.Microsecond)
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM ai.factor_genes WHERE id=$1", factorID)
		_, _ = pool.Exec(ctx, "DELETE FROM ai.strategy_genes WHERE id=$1", strategyID)
	})

	// factor：Save → Get → UpdateMetrics → UpdateStatus → Delete。
	require.NoError(t, factorPool.Save(ctx, &FactorGene{
		ID: factorID, Name: "itest_factor", Category: "itest", Formula: "rank(close)",
		IC: 0.01, ParentIDs: []string{"itest_parent"}, Status: "pending",
		CreatedAt: now, UpdatedAt: now,
	}))
	require.Equal(t, 1, countRows(t, ctx, pool, "ai.factor_genes", factorID),
		"Save 必须落在 ai.factor_genes")

	got, err := factorPool.Get(ctx, factorID)
	require.NoError(t, err)
	assert.Equal(t, "rank(close)", got.Formula, "Get 必须读回 ai.factor_genes 的同一行")
	assert.Equal(t, []string{"itest_parent"}, got.ParentIDs)

	require.NoError(t, factorPool.UpdateMetrics(ctx, factorID, 0.05, 0.9, 0.3, 1.2, 0.8))
	require.NoError(t, factorPool.UpdateStatus(ctx, factorID, "active"))
	require.Equal(t, 1, countRowsByStatus(t, ctx, pool, "ai.factor_genes", "active", factorID))

	listed, err := factorPool.ListByGeneration(ctx, 0)
	require.NoError(t, err)
	listedIDs := make([]string, 0, len(listed))
	for _, g := range listed {
		listedIDs = append(listedIDs, g.ID)
	}
	assert.Contains(t, listedIDs, factorID, "ListByGeneration 必须从 ai.factor_genes 读到刚写的行")

	// strategy：Save → Get → List → Delete。
	require.NoError(t, strategyPool.Save(ctx, &StrategyGene{
		ID: strategyID, Name: "itest_strategy", StrategyType: "momentum",
		Params:    map[string]interface{}{"period": float64(20)},
		FactorIDs: []string{factorID}, Status: "pending", CreatedAt: now, UpdatedAt: now,
	}))
	require.Equal(t, 1, countRows(t, ctx, pool, "ai.strategy_genes", strategyID),
		"Save 必须落在 ai.strategy_genes")

	sgot, err := strategyPool.Get(ctx, strategyID)
	require.NoError(t, err)
	assert.Equal(t, "momentum", sgot.StrategyType)
	assert.Equal(t, []string{factorID}, sgot.FactorIDs)

	slist, err := strategyPool.List(ctx, "momentum", "", -1.0, 50)
	require.NoError(t, err)
	slistedIDs := make([]string, 0, len(slist))
	for _, g := range slist {
		slistedIDs = append(slistedIDs, g.ID)
	}
	assert.Contains(t, slistedIDs, strategyID, "List 必须从 ai.strategy_genes 读到刚写的行")

	// 停写：legacy public.* 不得出现这两个 id（表不存在也算通过——没有可写的旧表）。
	assert.Zero(t, legacyRows(t, ctx, pool, "public.factor_genes", factorID),
		"停写后 public.factor_genes 不得出现该行")
	assert.Zero(t, legacyRows(t, ctx, pool, "public.strategy_genes", strategyID),
		"停写后 public.strategy_genes 不得出现该行")

	require.NoError(t, factorPool.Delete(ctx, factorID))
	require.NoError(t, strategyPool.Delete(ctx, strategyID))
	assert.Zero(t, countRows(t, ctx, pool, "ai.factor_genes", factorID))
	assert.Zero(t, countRows(t, ctx, pool, "ai.strategy_genes", strategyID))
}

func countRows(t *testing.T, ctx context.Context, pool *pgxpool.Pool, table, id string) int {
	t.Helper()
	var n int
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM "+table+" WHERE id=$1", id).Scan(&n))
	return n
}

func countRowsByStatus(t *testing.T, ctx context.Context, pool *pgxpool.Pool, table, status, id string) int {
	t.Helper()
	var n int
	require.NoError(t, pool.QueryRow(ctx,
		"SELECT count(*) FROM "+table+" WHERE status=$1 AND id=$2", status, id).Scan(&n))
	return n
}

// legacyRows 在旧表存在时数该 id 的行；表不存在返回 0（没有可写的旧表）。
func legacyRows(t *testing.T, ctx context.Context, pool *pgxpool.Pool, table, id string) int {
	t.Helper()
	var reg *string
	require.NoError(t, pool.QueryRow(ctx, "SELECT to_regclass($1)::text", table).Scan(&reg))
	if reg == nil {
		return 0
	}
	var n int
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM "+table+" WHERE id=$1", id).Scan(&n))
	return n
}
