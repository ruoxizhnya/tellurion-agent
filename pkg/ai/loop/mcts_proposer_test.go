package loop

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"tellurion-agent/pkg/ai/search"
)

// TPE 与 MCTS 必须能互相顶替：控制器只认 Proposer 接口，不认算法。
var (
	_ Proposer = (*TPEProposer)(nil)
	_ Proposer = (*MCTSProposer)(nil)
)

// TestMCTSProposer_SameSeedReplays：同 seed + 同一串观测 ⇒ 同一串建议。
//
// 它守的是「回放」这件事：seed 一旦写进库里（Hypothesis），就必须真能
// 复现同一轮搜索，否则留痕只是讲故事。
func TestMCTSProposer_SameSeedReplays(t *testing.T) {
	const seed = int64(20261107)
	p1 := NewMCTSProposer(testSpace(), seed)
	p2 := NewMCTSProposer(testSpace(), seed)

	var observed []Observation
	for i := 0; i < 8; i++ {
		s1 := p1.Suggest(observed)
		s2 := p2.Suggest(observed)
		require.Equalf(t, s1, s2, "第 %d 次建议不一致（seed=%d）", i, seed)

		observed = append(observed, Observation{
			Seq:    i,
			Params: s1.Params,
			Value:  float64(i%4) * 0.5,
			OK:     true,
		})
	}
}

// TestMCTSProposer_ReplacesTPE：同一个控制器，换提议器即可跑满一轮。
func TestMCTSProposer_ReplacesTPE(t *testing.T) {
	cases := map[string]func(*search.SearchSpace, int64) Proposer{
		"tpe":  func(s *search.SearchSpace, seed int64) Proposer { return NewTPEProposer(s, seed) },
		"mcts": func(s *search.SearchSpace, seed int64) Proposer { return NewMCTSProposer(s, seed) },
	}

	for name, mk := range cases {
		t.Run(name, func(t *testing.T) {
			r := &fakeTryRunner{sharpe: []float64{0.2, 0.6, 0.9, 1.1, 0.7}}
			out, err := NewController(r, mk(testSpace(), 7), nil).Run(context.Background(), Config{
				RunID:       "run-" + name,
				Description: "做一个动量策略",
				MaxTries:    5,
			})
			require.NoError(t, err)
			require.Len(t, out.Tries, 5)
			require.NotNil(t, out.Best)
			for _, a := range out.Tries {
				require.NotEmpty(t, a.Params, "提议器必须每组参数都给出值")
				assert.Contains(t, a.Hypothesis, "seed=7", "每次尝试都要留下 seed 痕迹")
			}
		})
	}
}

// TestMCTSProposer_ParentIsBestSuccessful：父代指向当前最优，失败尝试不当父代。
func TestMCTSProposer_ParentIsBestSuccessful(t *testing.T) {
	p := NewMCTSProposer(testSpace(), 1)
	observed := []Observation{
		{Seq: 0, Params: map[string]any{"lookback": 20, "top_pct": 0.2, "universe": "csi300"}, Value: 2.0, OK: true},
		{Seq: 1, Params: map[string]any{"lookback": 30, "top_pct": 0.2, "universe": "csi300"}, Value: 0.5, OK: true},
		{Seq: 2, Params: map[string]any{"lookback": 40, "top_pct": 0.2, "universe": "csi300"}, Value: 3.0, OK: true},
		{Seq: 3, Params: map[string]any{"lookback": 50, "top_pct": 0.2, "universe": "csi300"}, Value: 0.0, OK: false},
	}

	s := p.Suggest(observed)
	require.NotNil(t, s.ParentSeq)
	assert.Equal(t, 1, *s.ParentSeq, "父代应是当前最优 seq 1，不是失败的 seq 3")
	assert.Contains(t, s.Hypothesis, "MCTS")
	assert.Contains(t, s.Hypothesis, "seed=1")
	assert.NotEmpty(t, s.Params)
}

// TestMCTSProposer_ParamsWithinSpace：树搜索采样同样不能越界。
func TestMCTSProposer_ParamsWithinSpace(t *testing.T) {
	p := NewMCTSProposer(testSpace(), 11)
	observed := make([]Observation, 12)
	for i := range observed {
		observed[i] = Observation{
			Seq:    i,
			Params: map[string]any{"lookback": 10 + i, "top_pct": 0.1 + float64(i)/100, "universe": "csi300"},
			Value:  float64(i),
			OK:     true,
		}
	}

	for i := 0; i < 20; i++ {
		s := p.Suggest(observed)
		lb := s.Params["lookback"].(int)
		assert.GreaterOrEqual(t, lb, 5)
		assert.LessOrEqual(t, lb, 60)

		tp := s.Params["top_pct"].(float64)
		assert.GreaterOrEqual(t, tp, 0.05)
		assert.LessOrEqual(t, tp, 0.5)
		assert.Contains(t, []string{"csi300", "csi500"}, s.Params["universe"])
	}
}
