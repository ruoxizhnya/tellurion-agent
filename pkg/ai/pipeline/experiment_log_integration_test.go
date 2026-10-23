package pipeline

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/ruoxizhnya/tellurion-contracts/domain"
	kitdb "github.com/ruoxizhnya/tellurion-kit/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"tellurion-agent/pkg/ai"
	"tellurion-agent/pkg/ai/intent"
	yamlgen "tellurion-agent/pkg/ai/yaml"
	"tellurion-agent/pkg/aistore"
)

// 真库取证：1a 的存储层与 1b 的生产者必须真的能串起来。
//
// 内存 sink 只能证明 pipeline 的行为契约，证明不了「库里真的有行」——
// 而 P1-1 的验收标准恰恰是后者。QT-17 ② 起这条腿打的是本仓自有的
// aistore（ai.experiments）：pipeline 生产者与 agent 自持存储的真实闭环。
//
// TEST_DATABASE_DSN 未设置就 skip（与 pkg/aistore 的真库腿同一套约定）：
// 连不上库是环境问题，不是被测代码的问题，失败信号没有意义。
func TestExperimentLog_RealDB_RoundTrip(t *testing.T) {
	ctx := context.Background()
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("TEST_DATABASE_DSN 未设置：跳过真库腿（本地 docker compose 起库后指定 DSN 可跑）")
	}
	pool, err := kitdb.Connect(ctx, dsn, kitdb.DefaultPoolOptions())
	require.NoError(t, err)
	defer pool.Close()
	require.NoError(t, aistore.Migrate(ctx, pool))
	store := aistore.NewStoreWithPool(pool)

	runID := "itest_run_momentum"
	defer pool.Exec(ctx, "DELETE FROM ai.experiments WHERE run_id=$1", runID)

	p := NewPipelineWithDeps(
		intent.NewParser(), yamlgen.NewGenerator(), &ai.MockClient{},
		WithExperimentSink(store),
	)
	runner := &mockBacktestRunner{result: &domain.BacktestResult{
		TotalTrades: 9, TotalReturn: 0.31, SharpeRatio: 1.45,
	}}

	execCtx := WithExperimentContext(ctx, ExperimentContext{
		RunID:      runID,
		Seq:        0,
		Hypothesis: "动量大窗口更好",
	})
	res, err := p.Execute(execCtx, "做一个动量策略", runner)
	require.NoError(t, err)
	require.Equal(t, StageComplete, res.Status)

	// 从库里读回来 —— 这是「能回放一条完整探索路径」的最小证明。
	// aistore 没有实验读方法（agent 只写不读），所以直接 SQL 取行。
	rows, err := pool.Query(ctx,
		`SELECT status, hypothesis, expression, strategy_name, metrics, finished_at, params
		 FROM ai.experiments WHERE run_id = $1`, runID)
	require.NoError(t, err)
	defer rows.Close()

	type row struct {
		status, hypothesis, expression, strategyName string
		metrics                                      []byte
		finishedAt                                   *time.Time
		params                                       map[string]any
	}
	list := []row{}
	for rows.Next() {
		var r row
		require.NoError(t, rows.Scan(&r.status, &r.hypothesis, &r.expression,
			&r.strategyName, &r.metrics, &r.finishedAt, &r.params))
		list = append(list, r)
	}
	require.NoError(t, rows.Err())
	require.Len(t, list, 1, "跑完一次尝试，库里必须有一行")

	got := list[0]
	assert.Equal(t, aistore.ExperimentStatusCompleted, got.status)
	assert.Equal(t, "动量大窗口更好", got.hypothesis)
	assert.NotEmpty(t, got.expression, "回放时要能看出这次跑的是什么信号")
	assert.NotEmpty(t, got.strategyName)
	assert.NotEmpty(t, got.metrics, "收尾时指标要落库")
	assert.NotNil(t, got.finishedAt)
	assert.NotEmpty(t, got.params["universe"], "参数向量里要有回测输入")
}
