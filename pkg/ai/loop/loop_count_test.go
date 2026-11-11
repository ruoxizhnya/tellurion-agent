package loop

import (
	"context"
	"errors"
	"testing"

	"github.com/ruoxizhnya/tellurion-contracts/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"tellurion-agent/pkg/ai/pipeline"
	"tellurion-agent/pkg/aistore"
)

// 实验计数持久化（AGENT-EXPERIMENT-COUNT-SURFACE）。
//
// 控制器自己只有内存 seq：进程重启后从 0 重数，多重检验校正的分母就小了，
// 同一个 Sharpe 会被算得比实际更显著。这里锁住两条判据：
//   - 接上 TrialCounter 后，NumTrials 从库里的基数继续数；
//   - 计数读不出来时整轮拒绝开跑，而不是悄悄退回 seq+1。

// fixedCounter 返回预先写死的已落库计数。
type fixedCounter struct {
	stats aistore.TrialStats
	err   error
}

func (f fixedCounter) ExperimentTrialStats(context.Context, string) (aistore.TrialStats, error) {
	return f.stats, f.err
}

// equityRunner 产出带净值曲线的结果 —— 统计维要求 periods > 0 才评估，
// 没有曲线的结果会让 NumTrials 根本不被消费。
type equityRunner struct {
	calls int
}

func (r *equityRunner) Execute(context.Context, string, pipeline.BacktestRunner) (*pipeline.Result, error) {
	r.calls++
	return &pipeline.Result{
		ExperimentID: int64(r.calls),
		BacktestResult: &domain.BacktestResult{
			SharpeRatio:     1.2,
			PortfolioValues: []domain.PortfolioValue{{}, {}},
		},
	}, nil
}

func TestRun_NumTrialsIncludesPersistedBase(t *testing.T) {
	r := &equityRunner{}
	out, err := NewController(r, &scriptedProposer{}, nil).
		WithTrialCounter(fixedCounter{stats: aistore.TrialStats{Total: 7, Failed: 2}}).
		Run(context.Background(), Config{RunID: "run-restarted", MaxTries: 2})
	require.NoError(t, err)
	require.Len(t, out.Tries, 2)

	assert.Equal(t, 8, out.Tries[0].NumTrials, "第一次尝试的累计次数 = 库里的 7 + 本次 1")
	assert.Equal(t, 9, out.Tries[1].NumTrials, "第二次尝试继续累加")
}

func TestRun_CountFailureRefusesToStart(t *testing.T) {
	r := &equityRunner{}
	out, err := NewController(r, &scriptedProposer{}, nil).
		WithTrialCounter(fixedCounter{err: errors.New("db down")}).
		Run(context.Background(), Config{RunID: "run-1", MaxTries: 2})

	require.Error(t, err, "读不出已试次数就不能开跑：分母不明时给出的显著性没有意义")
	assert.Nil(t, out)
	assert.Zero(t, r.calls, "拒绝开跑时一次尝试都不该发生")
}
