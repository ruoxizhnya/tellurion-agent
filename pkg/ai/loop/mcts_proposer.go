package loop

import (
	"fmt"

	"tellurion-agent/pkg/ai/search"
)

// MCTSProposer 用确定性 UCT 树搜索在离散化的参数桶上提议下一组参数（DBT-18）。
//
// 它与 TPEProposer 实现同一个 Proposer 接口，构造参数也只差算法：调用方
// （cmd/ai 的探索端点）可以在不改编排层的前提下切换两者并对比。
//
// seed 必须由调用方显式给出。它同时出现在 Hypothesis 里，而 Hypothesis 会
// 落进 ai.experiments.hypothesis —— 也就是说「这一轮用的哪个 seed」这件事
// 有库里的行可查，不依赖进程日志。
type MCTSProposer struct {
	opt   *search.MCTSOptimizer
	space *search.SearchSpace
	seed  int64
}

// NewMCTSProposer 构造 MCTS 提议者。seed 相同、观测序列相同 ⇒ 建议序列相同。
func NewMCTSProposer(space *search.SearchSpace, seed int64) *MCTSProposer {
	return &MCTSProposer{opt: search.NewMCTSOptimizer(seed), space: space, seed: seed}
}

// Suggest 的父子语义与 TPE 一致：树搜索在「好参数所在的桶」里继续找，
// 所以说它衍生自当前最优那次尝试。这是近似，不是严格的遗传父代。
func (p *MCTSProposer) Suggest(observed []Observation) Suggestion {
	history := make([]*search.Trial, 0, len(observed))
	for _, o := range observed {
		// 失败尝试不喂给树：Value=0 的语义是「没跑出来」，不是「很差的参数」。
		if !o.OK {
			continue
		}
		history = append(history, &search.Trial{
			ID: o.Seq, Params: o.Params, Value: o.Value, State: "completed",
		})
	}

	parent := bestSeq(observed)
	if parent == nil {
		return Suggestion{
			Params:     p.opt.Suggest(p.space, history),
			Hypothesis: fmt.Sprintf("MCTS(seed=%d)：还没有成功观测，先按均匀采样铺点", p.seed),
		}
	}
	return Suggestion{
		Params:     p.opt.Suggest(p.space, history),
		ParentSeq:  parent,
		Hypothesis: fmt.Sprintf("MCTS(seed=%d)：在第 %d 次（当前最优）附近做树搜索", p.seed, *parent),
	}
}
