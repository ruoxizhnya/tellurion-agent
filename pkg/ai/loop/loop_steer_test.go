package loop

import (
	"context"
	"testing"

	"github.com/ruoxizhnya/tellurion-contracts/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"tellurion-agent/pkg/ai/pipeline"
)

// G2 指方向：运行中追加的方向只作用于**后续**尝试。
//
// 用 runner 自己在第一次执行里调 Steer，模拟「跑的过程中人点了候选卡」——
// 这比在测试里猜时序更稳，也把「已经跑完的那次不重算」钉成了绝对判据。
type steerAwareRunner struct {
	ctrl         *Controller
	descriptions []string
	hypotheses   []string
}

func (r *steerAwareRunner) Execute(ctx context.Context, description string, _ pipeline.BacktestRunner) (*pipeline.Result, error) {
	ec := pipeline.ExperimentContextFrom(ctx)
	r.descriptions = append(r.descriptions, description)
	r.hypotheses = append(r.hypotheses, ec.Hypothesis)
	if ec.Seq == 0 {
		r.ctrl.Steer("只试反转类参数", ptr(0))
	}
	return &pipeline.Result{
		Status:         pipeline.StageComplete,
		ExperimentID:   int64(ec.Seq + 1),
		BacktestResult: &domain.BacktestResult{SharpeRatio: float64(ec.Seq)},
	}, nil
}

func TestController_Steer_AppliesToSubsequentAttemptsOnly(t *testing.T) {
	r := &steerAwareRunner{}
	c := NewController(r, &scriptedProposer{}, nil)
	r.ctrl = c

	out, err := c.Run(context.Background(),
		Config{RunID: "run-steer", Description: "做一个动量策略", MaxTries: 3})
	require.NoError(t, err)
	require.Len(t, out.Tries, 3)

	assert.NotContains(t, r.descriptions[0], "人类指方向", "第一次尝试在指方向之前就已开始")
	assert.NotContains(t, r.hypotheses[0], "人指方向")
	for i := 1; i < 3; i++ {
		assert.Contains(t, r.descriptions[i], "人类指方向")
		assert.Contains(t, r.descriptions[i], "只试反转类参数")
		assert.Contains(t, r.hypotheses[i], "人指方向: 只试反转类参数")
		assert.Contains(t, out.Tries[i].Hypothesis, "人指方向: 只试反转类参数",
			"方向必须落进尝试的假设，才有库里的行可查")
	}
}

func TestController_Steer_EmptyIsIgnored(t *testing.T) {
	c := NewController(&fakeTryRunner{}, &scriptedProposer{}, nil)
	assert.Zero(t, c.Steer("   ", nil))
	assert.Empty(t, c.Directions())
}

func TestController_Steer_DirectionsAreCopied(t *testing.T) {
	c := NewController(&fakeTryRunner{}, &scriptedProposer{}, nil)
	c.Steer("a", nil)

	dirs := c.Directions()
	dirs[0].Text = "被外部改了"
	assert.Equal(t, "a", c.Directions()[0].Text, "快照不能被调用方改回控制器")
}
