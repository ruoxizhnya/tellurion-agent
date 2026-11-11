package builtin

import (
	"context"
	"log"

	"github.com/google/uuid"
	"tellurion-agent/pkg/aistore"
)

// ─── Tool-invoked experiment rows ─────────────────────────────────────
//
// backtest.run 与 walk_forward_validate 也是一次实验，必须留下 ai.experiments
// 行。少了它们，累计实验计数只看得见 explore/pipeline 的尝试，而 Hermes 这类
// 直接调工具的外部 agent 用掉的多次试错完全不可见 —— 多重检验校正分母就小了。
//
// 这里只依赖两个最小写方法，不把整个 *aistore.Store 拖进工具层：工具不需要
// 读面，也不该能改裁决。

// ExperimentRecorder persists one tool-invoked experiment row.
// *aistore.Store satisfies it.
type ExperimentRecorder interface {
	InsertExperiment(ctx context.Context, e *aistore.Experiment) (int64, error)
	CompleteExperiment(ctx context.Context, id int64, m *aistore.ExperimentMetrics, errMsg string) error
}

// toolRunIDPrefix 区分工具直调产生的实验行：每调用一次就是一个独立 run，
// seq 恒为 0，靠全局 count 汇总。
const toolRunIDPrefix = "tool-"

// startToolExperiment 落一行 running 实验，返回行 ID（0 = 没落点或写失败）。
//
// 写失败只记日志、不打断工具调用：审计落库是旁路，工具本身的结果不能因为
// 记账失败被丢掉。但日志必须显式，不能静默 —— 静默丢行会让计数看起来正常。
func startToolExperiment(
	ctx context.Context,
	rec ExperimentRecorder,
	tool, strategyName string,
	params map[string]any,
) int64 {
	if rec == nil {
		return 0
	}
	id, err := rec.InsertExperiment(ctx, &aistore.Experiment{
		RunID:        toolRunIDPrefix + uuid.NewString(),
		Seq:          0,
		Hypothesis:   tool,
		Params:       params,
		DatasetSplit: aistore.DatasetSplitTest,
		StrategyName: strategyName,
	})
	if err != nil {
		log.Printf("%s: 实验行未写入（计数将少一次）: %v", tool, err)
		return 0
	}
	return id
}

// finishToolExperiment 收尾那次实验。这是与 startToolExperiment 配对的第二步：
// 只记「开始了」而不记结束，留下的是一排永远 running 的行。
func finishToolExperiment(
	ctx context.Context,
	rec ExperimentRecorder,
	id int64,
	m *aistore.ExperimentMetrics,
	errMsg string,
) {
	if rec == nil || id == 0 {
		return
	}
	if err := rec.CompleteExperiment(ctx, id, m, errMsg); err != nil {
		log.Printf("tool experiment %d: 收尾未写入: %v", id, err)
	}
}
