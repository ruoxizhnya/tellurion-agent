package main

// 生成链的装配根适配器（QT-32-A）。
//
// pkg/copilot 定义 port（ChainSink / ChainEntry），本仓的存储层是
// pkg/aistore —— 两边不该互相 import，所以映射放在装配根，与
// StaticCheckAdapter / SandboxRunnerAdapter 同一形状（S7-P1-2 的纪律）。

import (
	"context"
	"fmt"

	"tellurion-agent/pkg/aistore"
	"tellurion-agent/pkg/copilot"
)

// chainSink 把 copilot 的链快照写进 ai.generation_chain。
type chainSink struct {
	store *aistore.Store
}

// NewChainSink 构造留痕落点。store 为 nil 时返回 nil —— 那会让
// CopilotService 在开始生成前 fail-closed（记不下链就不生成），
// 而不是静默生成一份无法复查的产物。
func NewChainSink(store *aistore.Store) copilot.ChainSink {
	if store == nil {
		return nil
	}
	return &chainSink{store: store}
}

func (c *chainSink) SaveChain(ctx context.Context, e copilot.ChainEntry) error {
	buildState, err := mapBuildState(e.BuildState)
	if err != nil {
		return err
	}
	blocked, err := mapBlockedReason(e.BlockedReason)
	if err != nil {
		return err
	}
	return c.store.UpsertGenerationChain(ctx, &aistore.GenerationChain{
		JobID:         e.JobID,
		Attempt:       e.Attempt,
		Prompt:        e.Prompt,
		Universe:      e.Universe,
		StartDate:     e.StartDate,
		EndDate:       e.EndDate,
		Source:        e.Source,
		SourceSha256:  e.SourceSha256,
		BuildState:    buildState,
		BuildError:    e.BuildError,
		BlockedReason: blocked,
	})
}

// mapBuildState / mapBlockedReason 是**认不出取值就报错**的映射，不是 default
// 兜底：两个包各持一套词表，把没见过的值静默传下去，等于往库里写一个 CHECK
// 之外的状态、或一个不存在的拦截原因。报错会让 CopilotService 在写链时
// fail-closed（见 trace()），这正是想要的方向：宁可这次生成不做，
// 也不要留下一条说不清自己在哪一环的链。
func mapBuildState(v string) (string, error) {
	switch v {
	case copilot.BuildStateNone:
		return aistore.GenerationBuildNone, nil
	case copilot.BuildStateFailed:
		return aistore.GenerationBuildFailed, nil
	case copilot.BuildStatePassed:
		return aistore.GenerationBuildPassed, nil
	}
	return "", fmt.Errorf("chainSink: 未登记的编译段状态 %q（同步 pkg/copilot 与 pkg/aistore 的词表，别在这里加兜底）", v)
}

func mapBlockedReason(v string) (string, error) {
	switch v {
	case "":
		return "", nil
	case copilot.BlockedPromoteGate:
		return aistore.GenerationBlockedPromoteGate, nil
	}
	return "", fmt.Errorf("chainSink: 未登记的拦截原因 %q（同步 pkg/copilot 与 pkg/aistore 的词表）", v)
}
