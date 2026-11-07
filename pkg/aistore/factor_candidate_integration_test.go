package aistore

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSaveFactorCandidates_ValidationBeforeDB：整批校验在开事务之前完成。
// 第二行缺少 formula ⇒ 整批拒绝，第一行也不许落库。
func TestSaveFactorCandidates_ValidationBeforeDB(t *testing.T) {
	// 不连库：s.pool 为 nil，一旦代码走到 Begin 就会 panic。
	// 本用例证明「校验失败在连库之前返回」。
	s := &Store{}
	err := s.SaveFactorCandidates(context.Background(), []FactorCandidate{
		{ID: "dbt18-valid", Name: "n", Category: "mined", Formula: "close"},
		{ID: "dbt18-invalid", Name: "n", Category: "mined"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "formula is required")
}

// TestSaveFactorCandidates_Integration 是真库腿：整批写入 ai.factor_genes，
// 再验证「含非法候选的批次一行都不落」。没有 TEST_DATABASE_DSN 时如实 skip。
func TestSaveFactorCandidates_Integration(t *testing.T) {
	store, closePool := connectTestStore(t)
	defer closePool()
	ctx := context.Background()

	const prefix = "dbt18-itest-"
	defer func() {
		_, _ = store.pool.Exec(ctx, "DELETE FROM ai.factor_genes WHERE id LIKE $1", prefix+"%")
	}()

	batch := []FactorCandidate{
		{ID: prefix + "a", Name: "mined-a", Category: "mined", Formula: "abs(close)",
			Description: "seed=1", ParentIDs: []string{"p1"}, Generation: 2, Status: "pending"},
		{ID: prefix + "b", Name: "mined-b", Category: "mined", Formula: "log(volume)",
			Description: "seed=1", Generation: 2},
	}
	require.NoError(t, store.SaveFactorCandidates(ctx, batch))

	var n int
	require.NoError(t, store.pool.QueryRow(ctx,
		"SELECT COUNT(*) FROM ai.factor_genes WHERE id LIKE $1", prefix+"%").Scan(&n))
	assert.Equal(t, 2, n)

	// 非法批次：第二个候选没有 formula ⇒ 整批回滚，计数仍是 2（不是 3）。
	bad := []FactorCandidate{
		{ID: prefix + "c", Name: "mined-c", Category: "mined", Formula: "sqrt(close)"},
		{ID: prefix + "d", Name: "mined-d", Category: "mined"},
	}
	require.Error(t, store.SaveFactorCandidates(ctx, bad))
	require.NoError(t, store.pool.QueryRow(ctx,
		"SELECT COUNT(*) FROM ai.factor_genes WHERE id LIKE $1", prefix+"%").Scan(&n))
	assert.Equal(t, 2, n, "非法批次不许落下前一行")

	// 同 ID 重写是更新，不是第二条。
	require.NoError(t, store.SaveFactorCandidates(ctx, []FactorCandidate{
		{ID: prefix + "a", Name: "mined-a2", Category: "mined", Formula: "abs(close)",
			Description: "seed=1", Generation: 3},
	}))
	require.NoError(t, store.pool.QueryRow(ctx,
		"SELECT COUNT(*) FROM ai.factor_genes WHERE id = $1", prefix+"a").Scan(&n))
	assert.Equal(t, 1, n)
}
