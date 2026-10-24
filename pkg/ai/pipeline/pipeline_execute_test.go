package pipeline

import (
	"context"
	"testing"

	"github.com/ruoxizhnya/tellurion-contracts/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"tellurion-agent/pkg/ai"
	"tellurion-agent/pkg/ai/intent"
	yamlgen "tellurion-agent/pkg/ai/yaml"
)

// P0-5：Execute 此前有两条平行的路 —— Stage 2 产出可执行的 YAML，
// Stage 3/4 让 LLM 写一段 Go 代码编译完就删掉。而 Stage 5 的回测拿
// `parsedIntent.StrategyName` 去查策略，这个名字**从未被注册过**，
// 于是必然 strategy not found。
//
// QT-17 S3-c-3：本仓不再有 registry —— 回测提交给 core 的内置
// `expression_template`，表达式经 strategy_params 生效。下面这组测试锁的
// 契约变成：回测提交的**必须是 core 能执行的策略名 + 表达式参数**，而
// LLM 生成的 Go 代码只是可审阅的 artifact（编译不过也不该拖累回测）。

// registryBacktestRunner 记录提交给执行方的策略引用与参数。
//
// 旧实现会去全局 registry 里查名字（进程内引擎的行为）；HTTP 化之后
// 「能不能跑」由 core 决定，本仓能验证的是**提交形状**：名字必须是
// core 注册的 expression_template，表达式必须随 strategy_params 一起走。
type registryBacktestRunner struct {
	RequestedName string
	Params        map[string]any
	Calls         int
}

func (r *registryBacktestRunner) RunBacktest(ctx context.Context, strategyName string, stockPool []string, startDate, endDate string) (*domain.BacktestResult, error) {
	r.RequestedName = strategyName
	r.Calls++
	return &domain.BacktestResult{TotalTrades: 3, TotalReturn: 0.05}, nil
}

func (r *registryBacktestRunner) RunBacktestWithParams(
	ctx context.Context,
	strategyName string,
	strategyParams map[string]any,
	stockPool []string,
	startDate, endDate string,
) (*domain.BacktestResult, error) {
	r.Params = strategyParams
	return r.RunBacktest(ctx, strategyName, stockPool, startDate, endDate)
}

// assertCoreExecutableStrategy 断言请求里的策略名是 core 注册的表达式策略，
// 而不是意图里那个只在本仓日志里有意义的标签。
func assertCoreExecutableStrategy(t *testing.T, runner *registryBacktestRunner) {
	t.Helper()
	assert.Equal(t, yamlgen.RunStrategyName, runner.RequestedName,
		"提交的策略名必须是 core 注册的 %q，否则必然 strategy not found", yamlgen.RunStrategyName)
	require.NotNil(t, runner.Params, "表达式策略必须经 strategy_params 提交")
	assert.NotEmpty(t, runner.Params["signal_expr"], "strategy_params 必须带信号表达式")
}

func newTestPipeline(mock *ai.MockClient) *Pipeline {
	return NewPipelineWithDeps(intent.NewParser(), yamlgen.NewGenerator(), mock)
}

// TestExecute_SubmitsCoreExecutableStrategy 是 P0-5 在新边界上的回归：
// 回测提交的必须是 core 能执行的策略（expression_template + 表达式参数），
// 不是意图里的标签。
func TestExecute_SubmitsCoreExecutableStrategy(t *testing.T) {
	mock := &ai.MockClient{Configured: true, ChatResponse: "package broken\nthis is not go code"}
	runner := &registryBacktestRunner{}

	res, err := newTestPipeline(mock).Execute(context.Background(), "做一个动量策略", runner)

	require.NoError(t, err)
	assert.Equal(t, StageComplete, res.Status)
	assert.Equal(t, 1, runner.Calls, "回测应该真的被跑了一次")
	assertCoreExecutableStrategy(t, runner)
	require.NotNil(t, res.BacktestResult, "回测必须出结果，而不是 strategy not found")
	assert.Empty(t, res.BacktestError)
}

// TestExecute_BrokenGeneratedCodeIsArtifactOnly：LLM 写的代码编译不过时，
// 只记录不阻断 —— 真正执行的是 YAML/表达式策略，两者不是同一个东西。
func TestExecute_BrokenGeneratedCodeIsArtifactOnly(t *testing.T) {
	mock := &ai.MockClient{Configured: true, ChatResponse: "package broken\nthis is not go code"}
	runner := &registryBacktestRunner{}

	res, err := newTestPipeline(mock).Execute(context.Background(), "做一个均值回归策略", runner)

	require.NoError(t, err)
	assert.Equal(t, StageComplete, res.Status)
	assert.NotEmpty(t, res.GeneratedCode, "生成的代码要留在结果里供审阅")
	assert.NotEmpty(t, res.BuildError, "编译失败要如实记录")
	require.NotNil(t, res.BacktestResult, "artifact 编译失败不该拖累回测")
}

// TestExecute_WithoutConfiguredLLM_StillBacktests：没配 LLM 时，
// 规则解析 + YAML 生成这条路本来就够跑，不该被代码生成环节卡住。
// （LLM 只影响 artifact 的有无，不影响能不能回测。）
func TestExecute_WithoutConfiguredLLM_StillBacktests(t *testing.T) {
	mock := &ai.MockClient{Configured: false}
	runner := &registryBacktestRunner{}

	res, err := newTestPipeline(mock).Execute(context.Background(), "做一个动量策略", runner)

	require.NoError(t, err)
	assert.Equal(t, StageComplete, res.Status)
	require.NotNil(t, res.BacktestResult)
	assert.Empty(t, res.GeneratedCode, "没有 LLM 就不该有生成的代码")
}

// TestExecuteAsync_SubmitsCoreExecutableStrategy：异步路径与主路径
// 是复制粘贴的两份逻辑，别只修一边。
func TestExecuteAsync_SubmitsCoreExecutableStrategy(t *testing.T) {
	mock := &ai.MockClient{Configured: false}
	runner := &registryBacktestRunner{}

	p := newTestPipeline(mock)
	jobID := p.ExecuteAsync(context.Background(), "做一个突破策略", runner)
	res := p.GetJob(jobID)
	res.WaitDone()

	assert.Equal(t, StageComplete, res.Status)
	assertCoreExecutableStrategy(t, runner)
	require.NotNil(t, res.BacktestResult, "异步路径同样必须提交可执行策略后再回测")
}
