package main

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	kitdb "github.com/ruoxizhnya/tellurion-kit/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"tellurion-agent/pkg/aistore"
)

// TestStopWrite_LegacyExperimentsFrozen 是 QT-17 ④ 停写切换的取证腿。
//
// 停写前这条腿的对拍版本（旧表 + ai 表同写后逐行比对）已随双写适配器一起
// 删除；停写后的不变量变成**单向**的：
//
//   - 生产落点（*aistore.Store，即 main.go 现在直接注入的 sink）写完整一轮
//     尝试后，ai.experiments 必须有对应行；
//   - legacy public.experiments 的**行数与最大 id 都不得变化** —— 那是
//     「旧表不再增长」的直接证据，不是靠口头声明。
//
// 旧表遗留的 30 行（早于双写期，见 ODR-001）已一次性回填归档进
// ai.experiments；本腿只验证停写后的增量行为，不重放历史。
func TestStopWrite_LegacyExperimentsFrozen(t *testing.T) {
	ctx := context.Background()
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if strings.TrimSpace(dsn) == "" {
		t.Skip("TEST_DATABASE_DSN 未设置：跳过真库腿（本地 docker compose 起库后指定 DSN 可跑）")
	}

	pool, err := kitdb.Connect(ctx, dsn, kitdb.DefaultPoolOptions())
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	require.NoError(t, aistore.Migrate(ctx, pool))
	ai := aistore.NewStoreWithPool(pool)

	var legacyCount, legacyMaxID int64
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*), COALESCE(max(id), 0) FROM public.experiments`).
		Scan(&legacyCount, &legacyMaxID))

	const runID = "itest_stopwrite_frozen"
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM ai.experiments WHERE run_id=$1", runID)
	})

	// 走一遍 Insert → Update → Complete(completed) → RecordVerdict。
	id0, err := ai.InsertExperiment(ctx, &aistore.Experiment{
		RunID: runID, Seq: 0, Hypothesis: "停写后只写 ai 表",
	})
	require.NoError(t, err)
	require.NotZero(t, id0)
	require.NoError(t, ai.UpdateExperiment(ctx, id0, aistore.ExperimentUpdate{
		Params:       map[string]any{"lookback_days": 20},
		StrategyName: "momentum",
		Expression:   "rank(close / delay(close, 20))",
	}))
	require.NoError(t, ai.CompleteExperiment(ctx, id0, &aistore.ExperimentMetrics{
		SharpeRatio: 1.1, TotalReturn: 0.2, TotalTrades: 7,
	}, ""))
	require.NoError(t, ai.RecordVerdict(ctx, id0, json.RawMessage(`{"economic":{"p":0.03}}`)))

	// 父子链：id0 是 ai id 空间，直接作为 parent_id 合法（不再需要 id 映射）。
	id1, err := ai.InsertExperiment(ctx, &aistore.Experiment{
		RunID: runID, Seq: 1, ParentID: &id0, Hypothesis: "子代",
		DatasetSplit: aistore.DatasetSplitHold,
	})
	require.NoError(t, err)
	require.NoError(t, ai.CompleteExperiment(ctx, id1, nil, "engine exploded"))

	// ai 侧：两行、状态正确、父代指向 ai id 空间、裁决落库。
	var gotCount int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM ai.experiments WHERE run_id=$1`, runID).Scan(&gotCount))
	assert.Equal(t, 2, gotCount)

	var status string
	var verdict []byte
	var parentID *int64
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT status, verdict, parent_id FROM ai.experiments WHERE id=$1`, id0).
		Scan(&status, &verdict, &parentID))
	assert.Equal(t, aistore.ExperimentStatusCompleted, status)
	assert.NotEmpty(t, verdict, "裁决必须落在 ai.experiments")
	assert.Nil(t, parentID)

	require.NoError(t, pool.QueryRow(ctx,
		`SELECT status, parent_id FROM ai.experiments WHERE id=$1`, id1).
		Scan(&status, &parentID))
	assert.Equal(t, aistore.ExperimentStatusFailed, status)
	require.NotNil(t, parentID)
	assert.Equal(t, id0, *parentID, "父代必须是 ai id 空间里的那一行")

	// legacy 侧：行数与最大 id 一动不动 —— 这就是「停写」。
	var afterCount, afterMaxID int64
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*), COALESCE(max(id), 0) FROM public.experiments`).
		Scan(&afterCount, &afterMaxID))
	assert.Equal(t, legacyCount, afterCount, "停写后 legacy public.experiments 不得新增行")
	assert.Equal(t, legacyMaxID, afterMaxID, "停写后 legacy 序列不得前进")
}
