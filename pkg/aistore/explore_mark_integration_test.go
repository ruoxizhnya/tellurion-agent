package aistore

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStore_ExploreMark 是真库腿：G4 标记必须比内存运行态（D10=b）活得久。
// 走一遍写入 → 列出 → 同一坐标重复标记 → 非法 kind 被拒，并验证索引列
// 能按 run 取回；重复执行迁移保持幂等。
func TestStore_ExploreMark(t *testing.T) {
	store, closePool := connectTestStore(t)
	defer closePool()
	ctx := context.Background()

	const runID = "itest_explore_mark"
	cleanup := func() {
		_, _ = store.pool.Exec(ctx, "DELETE FROM ai.explore_marks WHERE run_id=$1", runID)
	}
	defer cleanup()
	cleanup()

	first, err := store.InsertExploreMark(ctx, ExploreMark{
		RunID: runID, AttemptSeq: 3, Kind: ExploreMarkDerive,
		Note: "围绕它多试几组", Actor: "ruoxi", ExperimentID: 42,
	})
	require.NoError(t, err)
	assert.NotZero(t, first.ID)
	assert.False(t, first.CreatedAt.IsZero())

	// 同一坐标允许重复标记：标注是流水，不是唯一决策。
	second, err := store.InsertExploreMark(ctx, ExploreMark{
		RunID: runID, AttemptSeq: 3, Kind: ExploreMarkPromote,
		Note: "提升权重",
	})
	require.NoError(t, err)
	assert.Greater(t, second.ID, first.ID)

	marks, err := store.ListExploreMarks(ctx, runID)
	require.NoError(t, err)
	require.Len(t, marks, 2)
	assert.Equal(t, ExploreMarkDerive, marks[0].Kind)
	assert.Equal(t, "围绕它多试几组", marks[0].Note, "自由文本必须原样保留")
	assert.Equal(t, int64(42), marks[0].ExperimentID, "要能 join 回那次尝试")
	assert.Equal(t, ExploreMarkPromote, marks[1].Kind)

	// 只列本轮：别的 run 不该混进来。
	other, err := store.InsertExploreMark(ctx, ExploreMark{
		RunID: runID + "_other", AttemptSeq: 0, Kind: ExploreMarkPromote,
	})
	require.NoError(t, err)
	defer func() {
		_, _ = store.pool.Exec(ctx, "DELETE FROM ai.explore_marks WHERE run_id=$1", runID+"_other")
	}()
	assert.NotZero(t, other.ID)
	marks, err = store.ListExploreMarks(ctx, runID)
	require.NoError(t, err)
	assert.Len(t, marks, 2, "按 run 隔离")

	// 拒绝面：kind 漂移与负 seq 在落库前就被挡下。
	_, err = store.InsertExploreMark(ctx, ExploreMark{RunID: runID, AttemptSeq: 0, Kind: "archive"})
	require.Error(t, err)
	_, err = store.InsertExploreMark(ctx, ExploreMark{RunID: runID, AttemptSeq: -1, Kind: ExploreMarkDerive})
	require.Error(t, err)
	_, err = store.InsertExploreMark(ctx, ExploreMark{AttemptSeq: 0, Kind: ExploreMarkDerive})
	require.Error(t, err)

	// 迁移幂等：再跑一次 Migrate 不报错，且不重复建表。
	require.NoError(t, Migrate(ctx, store.pool))
}
