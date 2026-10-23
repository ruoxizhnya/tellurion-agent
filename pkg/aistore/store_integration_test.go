package aistore

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ruoxizhnya/tellurion-contracts/domain"
	kitdb "github.com/ruoxizhnya/tellurion-kit/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// connectTestStore 是真库腿的公共入口：TEST_DATABASE_DSN 未设置就 skip。
// 迁移在这里跑（幂等），种子**不**跑 —— 种子语义有自己的断言腿。
func connectTestStore(t *testing.T) (*Store, func()) {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if strings.TrimSpace(dsn) == "" {
		t.Skip("TEST_DATABASE_DSN 未设置：跳过真库腿（本地 docker compose 起库后指定 DSN 可跑）")
	}
	pool, err := kitdb.Connect(context.Background(), dsn, kitdb.DefaultPoolOptions())
	require.NoError(t, err)
	require.NoError(t, Migrate(context.Background(), pool))
	return NewStoreWithPool(pool), pool.Close
}

// TestStore_ExperimentLifecycle 走一遍 Insert → Update → Complete 的两步写，
// 再走一遍失败路径，然后 SQL 直读验证：这两条语义逐条对照 core 的
// experiments.go 搬入，对拍它们就是证明「ai.experiments 与旧表同语义」。
func TestStore_ExperimentLifecycle(t *testing.T) {
	store, closePool := connectTestStore(t)
	defer closePool()
	ctx := context.Background()

	const runID = "itest_store_lifecycle"
	defer func() {
		_, _ = store.pool.Exec(ctx, "DELETE FROM ai.experiments WHERE run_id=$1", runID)
	}()

	// completed 路径：Insert（running / split 空回 train）→ Update（补参数）→ Complete（带指标）。
	id, err := store.InsertExperiment(ctx, &Experiment{
		RunID: runID, Seq: 0, Hypothesis: "动量大窗口更好",
	})
	require.NoError(t, err)
	require.NotZero(t, id, "Insert 必须返回自增 id")

	require.NoError(t, store.UpdateExperiment(ctx, id, ExperimentUpdate{
		Params:       map[string]any{"lookback_days": 30, "universe": "hs300"},
		StrategyName: "momentum",
		Expression:   "rank(close / delay(close, 30))",
	}))
	require.NoError(t, store.CompleteExperiment(ctx, id, &ExperimentMetrics{
		SharpeRatio: 1.45, TotalReturn: 0.31, TotalTrades: 9,
	}, ""))

	var status, hypothesis, strategy, expression string
	var params map[string]any
	var metrics []byte
	var errMsg *string
	var finishedAt *time.Time
	err = store.pool.QueryRow(ctx, `
		SELECT status, hypothesis, strategy_name, expression, params,
		       metrics, error_message, finished_at
		FROM ai.experiments WHERE id = $1`, id).Scan(
		&status, &hypothesis, &strategy, &expression, &params,
		&metrics, &errMsg, &finishedAt)
	require.NoError(t, err)
	assert.Equal(t, ExperimentStatusCompleted, status)
	assert.Equal(t, "动量大窗口更好", hypothesis)
	assert.Equal(t, "momentum", strategy)
	assert.Equal(t, "rank(close / delay(close, 30))", expression)
	assert.Equal(t, float64(30), params["lookback_days"])
	assert.NotEmpty(t, params["universe"], "参数向量里要有回测输入")
	assert.NotEmpty(t, metrics, "completed 路径指标必须落库")
	assert.Nil(t, errMsg)
	assert.NotNil(t, finishedAt)

	// failed 路径：失败也落行，且 metrics 置空 —— core 同款语义。
	failedID, err := store.InsertExperiment(ctx, &Experiment{
		RunID: runID, Seq: 1, Hypothesis: "会失败的尝试",
		DatasetSplit: DatasetSplitHold,
	})
	require.NoError(t, err)
	require.NoError(t, store.CompleteExperiment(ctx, failedID, &ExperimentMetrics{
		SharpeRatio: 9.9, TotalReturn: 9.9, TotalTrades: 9,
	}, "engine exploded"))
	require.NoError(t, store.pool.QueryRow(ctx, `
		SELECT status, error_message, metrics FROM ai.experiments WHERE id = $1`, failedID).
		Scan(&status, &errMsg, &metrics))
	assert.Equal(t, ExperimentStatusFailed, status)
	require.NotNil(t, errMsg)
	assert.Equal(t, "engine exploded", *errMsg)
	assert.Empty(t, metrics, "failed 路径 metrics 必须置空 —— 失败的指标不是指标")

	// id==0 的补写/收尾是静默跳过（sink 写入失败时 pipeline 的约定路径）。
	assert.NoError(t, store.UpdateExperiment(ctx, 0, ExperimentUpdate{StrategyName: "x"}))
	assert.NoError(t, store.CompleteExperiment(ctx, 0, nil, ""))
	assert.NoError(t, store.RecordVerdict(ctx, 0, []byte(`{}`)))
}

// TestStore_GetMissingReturnsNilNil 钉「没记录 ≠ 查询失败」的语义：两个返回
// 都为空时调用方要如实上报，不许拿内置数据悄悄兜底。
func TestStore_GetMissingReturnsNilNil(t *testing.T) {
	store, closePool := connectTestStore(t)
	defer closePool()
	ctx := context.Background()

	h, err := store.GetFactorHypothesis(ctx, domain.FactorType("no_such_factor_itest"))
	require.NoError(t, err)
	assert.Nil(t, h, "没记录过的因子假设必须返回 (nil, nil)")

	_, err = store.GetResearchProfile(ctx, "NO_SUCH_TICKER_ITEST")
	assert.NoError(t, err, "没记录过的画像也是 (nil, nil)，不是错误")

	_, err = store.GetResearchProfile(ctx, "")
	assert.Error(t, err, "空 ticker 是调用方 bug，必须报错，不能当『没记录』")
}

// TestStore_FactorHypothesisUpsert 验证 Save 是覆盖式更新（含 updated_at）、
// SourceKind 空回 ad_hoc，以及种子幂等（第二遍 0 行）。
func TestStore_FactorHypothesisUpsertAndSeedIdempotent(t *testing.T) {
	store, closePool := connectTestStore(t)
	defer closePool()
	ctx := context.Background()

	name := domain.FactorType("itest_upsert_factor")
	defer func() {
		_, _ = store.pool.Exec(ctx, "DELETE FROM ai.factor_hypothesis WHERE factor_name=$1", string(name))
	}()

	require.NoError(t, store.SaveFactorHypothesis(ctx, domain.FactorHypothesis{
		FactorName: name, Hypothesis: "第一版假设",
	}))
	got, err := store.GetFactorHypothesis(ctx, name)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, domain.FactorSourceAdHoc, got.SourceKind, "SourceKind 空缺省 ad_hoc")

	require.NoError(t, store.SaveFactorHypothesis(ctx, domain.FactorHypothesis{
		FactorName: name, SourceKind: domain.FactorSourceAI,
		Hypothesis: "第二版假设", Reference: "ref:itest",
	}))
	got, err = store.GetFactorHypothesis(ctx, name)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "第二版假设", got.Hypothesis, "Save 是覆盖式，不是追加")
	assert.Equal(t, domain.FactorSourceAI, got.SourceKind)
	assert.Equal(t, "ref:itest", got.Reference)

	assert.Error(t, store.SaveFactorHypothesis(ctx, domain.FactorHypothesis{FactorName: name}),
		"空假设必须被拒收")

	// 种子幂等（自洽，不依赖库的既有播种状态）：先播一遍（可能 N 行），
	// 再播一遍必须 0 行 —— 覆盖人写假设的种子是事故。
	_, err = store.SeedBuiltinFactorHypotheses(ctx)
	require.NoError(t, err)
	n, err := store.SeedBuiltinFactorHypotheses(ctx)
	require.NoError(t, err)
	assert.Zero(t, n, "builtin 种子重播不得覆盖任何已有行")
}
