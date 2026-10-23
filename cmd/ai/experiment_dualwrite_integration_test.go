package main

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	kitdb "github.com/ruoxizhnya/tellurion-kit/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"tellurion-agent/pkg/aistore"
	"tellurion/pkg/storage"
)

// TestDualWrite_ParityAgainstLegacy 是 QT-17 ④ 的对拍腿：双写适配器写完
// 一条完整路径（completed / failed / verdict / 父子链）后，两张表必须
// 逐行对得上 —— 行数、语义字段、父代指向（按 seq 对齐，两张表 id 空间不同）。
//
// 这也是 InsertExperiment 父代翻译的取证腿：loop 记的 ParentID 是 legacy id，
// ai.experiments 的自引用 FK 必须拿到翻译后的 ai id，否则 FK 拒收、
// ai 侧整行缺失 —— 对拍正是设计来抓这种「一边有行一边没有」的。
//
// TEST_DATABASE_DSN 未设置就 skip（全仓真库腿同一约定）。
func TestDualWrite_ParityAgainstLegacy(t *testing.T) {
	ctx := context.Background()
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if strings.TrimSpace(dsn) == "" {
		t.Skip("TEST_DATABASE_DSN 未设置：跳过真库腿（本地 docker compose 起库后指定 DSN 可跑）")
	}

	legacy, err := storage.NewPostgresStore(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(legacy.Close)

	pool, err := kitdb.Connect(ctx, dsn, kitdb.DefaultPoolOptions())
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	require.NoError(t, aistore.Migrate(ctx, pool))
	ai := aistore.NewStoreWithPool(pool)

	sink := newDualWriteSink(legacy, ai)

	const runID = "itest_dualwrite_parity"
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM ai.experiments WHERE run_id=$1", runID)
		_, _ = legacy.DB().Exec(ctx, "DELETE FROM experiments WHERE run_id=$1", runID)
	})

	// attempt 0：completed 带指标 + verdict。
	id0, err := sink.InsertExperiment(ctx, &aistore.Experiment{
		RunID: runID, Seq: 0, Hypothesis: "动量大窗口更好",
	})
	require.NoError(t, err)
	require.NotZero(t, id0)
	require.NoError(t, sink.UpdateExperiment(ctx, id0, aistore.ExperimentUpdate{
		Params:       map[string]any{"lookback_days": 30, "universe": "hs300"},
		StrategyName: "momentum",
		Expression:   "rank(close / delay(close, 30))",
	}))
	require.NoError(t, sink.CompleteExperiment(ctx, id0, &aistore.ExperimentMetrics{
		SharpeRatio: 1.45, TotalReturn: 0.31, TotalTrades: 9,
	}, ""))
	require.NoError(t, sink.RecordVerdict(ctx, id0, json.RawMessage(`{"economic":{"p":0.02}}`)))

	// attempt 1：父代指 attempt 0（loop 记的是 sink 返回的 legacy id），
	// 失败路径。
	id1, err := sink.InsertExperiment(ctx, &aistore.Experiment{
		RunID: runID, Seq: 1, ParentID: &id0,
		Hypothesis:   "换小窗口试试",
		DatasetSplit: aistore.DatasetSplitHold,
	})
	require.NoError(t, err)
	require.NoError(t, sink.CompleteExperiment(ctx, id1, nil, "engine exploded"))

	// 对拍：两张表按 (run_id, seq) 逐行比。
	readLegacy := func(t *testing.T) []parityRow {
		t.Helper()
		rows, err := legacy.DB().Query(ctx, `
			SELECT e.seq, p.seq, e.hypothesis, e.dataset_split,
			       e.strategy_name, e.expression, e.status,
			       e.error_message, e.metrics, e.verdict
			FROM experiments e LEFT JOIN experiments p ON e.parent_id = p.id
			WHERE e.run_id = $1 ORDER BY e.seq`, runID)
		require.NoError(t, err)
		defer rows.Close()
		return scanParityRows(t, rows)
	}
	readAI := func(t *testing.T) []parityRow {
		t.Helper()
		rows, err := pool.Query(ctx, `
			SELECT e.seq, p.seq, e.hypothesis, e.dataset_split,
			       e.strategy_name, e.expression, e.status,
			       e.error_message, e.metrics, e.verdict
			FROM ai.experiments e LEFT JOIN ai.experiments p ON e.parent_id = p.id
			WHERE e.run_id = $1 ORDER BY e.seq`, runID)
		require.NoError(t, err)
		defer rows.Close()
		return scanParityRows(t, rows)
	}

	legacyRows := readLegacy(t)
	aiRows := readAI(t)
	require.Len(t, legacyRows, 2, "旧表该有两行（completed + failed）")
	require.Len(t, aiRows, 2, "ai 表该有两行 —— 少一行就是双写断了，对拍的意义就在这")

	for i := range legacyRows {
		l, a := legacyRows[i], aiRows[i]
		assert.Equal(t, l.seq, a.seq, "第 %d 行 seq 不对齐", i)
		assert.Equal(t, l.hypothesis, a.hypothesis)
		assert.Equal(t, l.datasetSplit, a.datasetSplit)
		assert.Equal(t, l.status, a.status, "第 %d 行状态两边不一致", i)
		assert.Equal(t, l.strategyName, a.strategyName)
		assert.Equal(t, l.expression, a.expression)
		assert.Equal(t, l.errorMessage, a.errorMessage)
		assert.Equal(t, jsonBytes(t, l.metrics), jsonBytes(t, a.metrics),
			"第 %d 行 metrics 两边不一致", i)
		assert.Equal(t, jsonBytes(t, l.verdict), jsonBytes(t, a.verdict))
		assert.Equal(t, l.parentSeq, a.parentSeq,
			"第 %d 行父代指错 —— ai 侧的 parent_id 必须经映射翻进 ai 的 id 空间", i)
	}

	// 语义抽查（不止「两边一样」，还要「一样地对」）：
	assert.Equal(t, "completed", aiRows[0].status)
	assert.Equal(t, "failed", aiRows[1].status)
	require.NotNil(t, aiRows[1].parentSeq)
	assert.Equal(t, 0, *aiRows[1].parentSeq, "attempt 1 的父代必须是 attempt 0")
	assert.NotEmpty(t, aiRows[0].verdict, "verdict 要落库")
	assert.NotEmpty(t, aiRows[1].errorMessage)
}

// parityRow 是对拍读出的一行；id 空间不同，父代统一折成 parent 行的 seq 比。
type parityRow struct {
	seq          int
	parentSeq    *int
	hypothesis   string
	datasetSplit string
	strategyName *string
	expression   *string
	status       string
	errorMessage *string
	metrics      []byte
	verdict      []byte
}

func scanParityRows(t *testing.T, rows pgx.Rows) []parityRow {
	t.Helper()
	var out []parityRow
	for rows.Next() {
		var r parityRow
		require.NoError(t, rows.Scan(&r.seq, &r.parentSeq, &r.hypothesis, &r.datasetSplit,
			&r.strategyName, &r.expression, &r.status,
			&r.errorMessage, &r.metrics, &r.verdict))
		out = append(out, r)
	}
	require.NoError(t, rows.Err())
	return out
}

// jsonBytes 把两边可能形状不同的 JSONB 折成可比的 JSON 值
// （SQL NULL 视作 nil，两边语义都是「没有」）。
func jsonBytes(t *testing.T, b []byte) any {
	t.Helper()
	if len(b) == 0 {
		return nil
	}
	var v any
	require.NoError(t, json.Unmarshal(b, &v), "metrics/verdict 必须是合法 JSON")
	return v
}
