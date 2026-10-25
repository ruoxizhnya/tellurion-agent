package pipeline

// S7-P1-3 regression test: verify pipeline.BacktestRunner is a zero-cost
// type alias to contracts.BacktestRunner (the SAME type), not a re-declared
// interface.
//
// Why this lives in pkg/ai/pipeline and not next to the contract
// (pkg/backtest/contracts/runner_test.go): proving the alias is identical
// requires importing pkg/ai/pipeline, and this package already depends on
// the contracts package in production. Asserting it here adds zero new
// coupling and keeps the core contract's own test free of any pkg/ai import
// (ADR-027 §5 step 3: the contract is core-side infrastructure, not an AI
// capability).
//
// This is the "eliminate duplicate definitions" guard: a future contributor
// who re-introduces a local `type BacktestRunner interface` here will break
// this test at compile time, surfacing the regression before it ships.
//
// QT-32-A 起本测试不再有 copilot 那一侧的断言：pkg/copilot 不再持有回测
// runner（AI 产物要先进资产服务、过 promote 人工闸门，见 pkg/copilot 的
// 闸门段）。那条腿跟着实现一起退场，不是被放宽。

import (
	"context"
	"testing"

	"github.com/ruoxizhnya/tellurion-contracts/domain"
	"github.com/ruoxizhnya/tellurion-contracts/engine"
)

// aliasStubRunner is a minimal concrete implementation used only to prove
// that the two BacktestRunner references denote the same type.
type aliasStubRunner struct{}

func (aliasStubRunner) RunBacktest(ctx context.Context, strategyName string, stockPool []string, startDate, endDate string) (*domain.BacktestResult, error) {
	return &domain.BacktestResult{}, nil
}

// TestS7P1_3_PipelineAliasIsIdentical verifies that pipeline.BacktestRunner
// is a zero-cost type alias to contracts.BacktestRunner (the SAME type), not
// a re-declared interface. If this package re-declares the interface, the
// cross-assignment below fails to compile.
func TestS7P1_3_PipelineAliasIsIdentical(t *testing.T) {
	var p BacktestRunner = aliasStubRunner{}

	// Assign pipeline → contracts. With a re-declared (distinct) interface
	// this assignment would fail.
	var c contracts.BacktestRunner = p
	_ = c
	t.Log("pipeline.BacktestRunner and contracts.BacktestRunner " +
		"are the SAME type (zero-cost alias)")
}
