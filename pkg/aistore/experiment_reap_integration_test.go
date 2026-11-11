package aistore

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStore_ReapRunningExperimentsBefore 是真库腿：模拟崩溃遗留的 running 行
// 在「重启」（启动期收尸）后被标记为 failed/orphan，而启动之后新建的行、
// 已收尾的行都不受影响；重复收尸幂等（第二次 0 行）。
func TestStore_ReapRunningExperimentsBefore(t *testing.T) {
	store, closePool := connectTestStore(t)
	defer closePool()
	ctx := context.Background()

	const runID = "itest_reap_running"
	defer func() {
		_, _ = store.pool.Exec(ctx, "DELETE FROM ai.experiments WHERE run_id=$1", runID)
	}()
	_, _ = store.pool.Exec(ctx, "DELETE FROM ai.experiments WHERE run_id=$1", runID)

	insert := func(seq int, hypothesis string) int64 {
		id, err := store.InsertExperiment(ctx, &Experiment{RunID: runID, Seq: seq, Hypothesis: hypothesis})
		require.NoError(t, err)
		return id
	}
	orphan := insert(0, "崩溃前正在跑的那次")
	alive := insert(1, "启动之后才建的行")
	finished := insert(2, "已经正常收尾")

	// 把「遗留」与「新行」的创建时间拉开：收尸判据是 created_at < 启动时刻。
	_, err := store.pool.Exec(ctx, `UPDATE ai.experiments SET created_at=$2 WHERE id=$1`,
		orphan, time.Date(2001, 1, 1, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	_, err = store.pool.Exec(ctx, `UPDATE ai.experiments SET created_at=$2 WHERE id=$1`,
		alive, time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	require.NoError(t, store.CompleteExperiment(ctx, finished, &ExperimentMetrics{SharpeRatio: 1.2}, ""))

	cutoff := time.Date(2026, 11, 12, 8, 0, 0, 0, time.UTC)
	n, err := store.ReapRunningExperimentsBefore(ctx, cutoff, ExperimentOrphanReason)
	require.NoError(t, err)
	assert.Equal(t, int64(1), n, "只应收掉启动前遗留的 running 行")

	var status string
	var errMsg *string
	var finishedAt *time.Time
	require.NoError(t, store.pool.QueryRow(ctx,
		`SELECT status, error_message, finished_at FROM ai.experiments WHERE id=$1`, orphan).
		Scan(&status, &errMsg, &finishedAt))
	assert.Equal(t, ExperimentStatusFailed, status, "遗留 running 必须标成 failed")
	require.NotNil(t, errMsg)
	assert.Equal(t, ExperimentOrphanReason, *errMsg, "收尸原因必须可判定")
	assert.NotNil(t, finishedAt, "收尸必须落完成时间")

	require.NoError(t, store.pool.QueryRow(ctx,
		`SELECT status FROM ai.experiments WHERE id=$1`, alive).Scan(&status))
	assert.Equal(t, ExperimentStatusRunning, status, "启动之后的行不属于遗留，不能被误收")

	require.NoError(t, store.pool.QueryRow(ctx,
		`SELECT status FROM ai.experiments WHERE id=$1`, finished).Scan(&status))
	assert.Equal(t, ExperimentStatusCompleted, status, "已收尾的行不得被改写")

	// 幂等：同一批行收过一次就不再是 running。
	n, err = store.ReapRunningExperimentsBefore(ctx, cutoff, ExperimentOrphanReason)
	require.NoError(t, err)
	assert.Zero(t, n, "重复收尸不得再改任何行")
}
