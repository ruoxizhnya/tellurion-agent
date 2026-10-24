package pipeline

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/ruoxizhnya/tellurion-contracts/domain"
	"github.com/ruoxizhnya/tellurion-contracts/engine"
	"tellurion-agent/pkg/ai"
	"tellurion-agent/pkg/ai/intent"
	yamlgen "tellurion-agent/pkg/ai/yaml"
	"tellurion-agent/pkg/aistore"
)

// Stage represents a pipeline stage
type Stage string

const (
	StageParse    Stage = "parse"
	StageGenerate Stage = "generate"
	StageValidate Stage = "validate"
	// StageRegister 是 P0-5 新增的阶段：把 YAML 配置构建成可执行策略
	// 并注册进全局 registry。此前没有这一阶段 —— 回测拿一个从未注册
	// 过的名字去找策略，必然 strategy not found。
	StageRegister Stage = "register"
	StageCompile  Stage = "compile"
	StageBacktest Stage = "backtest"
	StageComplete Stage = "complete"
	StageFailed   Stage = "failed"
)

// Result holds the outcome of a pipeline execution
type Result struct {
	ID             string                 `json:"id"`
	Status         Stage                  `json:"status"`
	Intent         *intent.Intent         `json:"intent,omitempty"`
	YAMLConfig     string                 `json:"yaml_config,omitempty"`
	GeneratedCode  string                 `json:"generated_code,omitempty"`
	BuildError     string                 `json:"build_error,omitempty"`
	BacktestResult *domain.BacktestResult `json:"backtest_result,omitempty"`
	BacktestError  string                 `json:"backtest_error,omitempty"`
	// ExperimentID 是这次尝试在实验日志中的行 ID（P1-2）。调用方要靠它把
	// 尝试串成父子链 —— 控制器知道「第 n 次」，但只有 pipeline 知道那一行
	// 落在库里的哪个 ID。0 表示这一路没记日志（无 sink 或写入失败）。
	ExperimentID int64      `json:"experiment_id,omitempty"`
	StartedAt    time.Time  `json:"started_at"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
	DurationMs   int64      `json:"duration_ms"`
	Logs         []string   `json:"logs,omitempty"`
	// done is closed when the ExecuteAsync goroutine finishes, giving
	// callers (especially tests) a safe way to wait for all writes to
	// the Result fields to complete before reading them. Without this
	// channel, tests had to use time.Sleep — which is both flaky and a
	// real data race (the async goroutine writes Status/Logs/etc.
	// concurrently with the test reading them).
	//
	// done is only closed by ExecuteAsync; synchronous Execute and
	// direct StartJob callers never close it. It is excluded from
	// JSON serialisation.
	done chan struct{} `json:"-"`
}

// BacktestRunner is the canonical contract for running backtests.
// S7-P1-3 (ODR-043): previously a local duplicate of the same
// interface in pkg/strategy/copilot.go. Now a zero-cost type alias to
// the single source of truth in pkg/backtest/contracts. All existing code
// (adapters, mocks, compile-time assertions) continues to work
// unchanged because Go type aliases are transparent.
type BacktestRunner = contracts.BacktestRunner

// StrategyParamsRunner 是**可选**扩展：能在一次回测里携带 run 级策略参数
// （QT-20 `strategy_params`）的实现。
//
// 为什么不做成 contracts.BacktestRunner 的必选方法：那份契约在
// tellurion-contracts 仓，改它要发版且会波及所有实现；而「表达式策略」
// 只有 pipeline 这一条链路需要参数。runner 不实现本接口时退回普通的
// RunBacktest（只能按名字跑 core 已注册的策略），行为与 S3-c-3 之前一致。
type StrategyParamsRunner interface {
	RunBacktestWithParams(
		ctx context.Context,
		strategyName string,
		strategyParams map[string]any,
		stockPool []string,
		startDate, endDate string,
	) (*domain.BacktestResult, error)
}

// ExperimentSink 是实验日志的落点（P1-1b）。
//
// 用接口而非 *aistore.Store，有两个理由：pipeline 不该绑死在具体
// 存储上；更要紧的是，实验日志最有价值的契约是「失败也落行」，而这条契约
// 不该被「DB 没起来」挡在测试门外。
//
// 三个动作对应一次尝试的三个时刻：开始（落 running 行）→ 参数确定（补写
// 试了什么）→ 结束（收尾成 completed / failed）。
type ExperimentSink interface {
	InsertExperiment(ctx context.Context, e *aistore.Experiment) (int64, error)
	UpdateExperiment(ctx context.Context, id int64, u aistore.ExperimentUpdate) error
	CompleteExperiment(ctx context.Context, id int64, m *aistore.ExperimentMetrics, errMsg string) error
}

// WithExperimentSink 注入实验日志落点。不注入即不记日志 —— 日志是观测
// 设施，不能反过来成为链路的硬依赖。
func WithExperimentSink(s ExperimentSink) PipelineOption {
	return func(p *Pipeline) {
		p.expSink = s
	}
}

// ExperimentContext 描述一次尝试在一轮探索中的位置。
//
// 走 context 而不是构造 pipeline 时传入，是因为 run_id / seq 每次执行都不同：
// P1-2 的循环控制器要在同一个 run 下连跑 100 次尝试（seq 从 0 递增），
// 而 pipeline 实例是复用的。
//
// 注意 seq 必须由调用方分配 —— experiments 表上有 UNIQUE(run_id, seq)，
// 并发下两边各自 +1 会撞车。
type ExperimentContext struct {
	RunID        string // 一轮探索的标识；空则用 job ID
	Seq          int    // 本轮第几次尝试
	ParentID     *int64 // 由哪次尝试衍生而来（首轮为 nil）
	Hypothesis   string // 假设来源：为什么试这个
	DatasetSplit string // train / hold / test；空则按 train 记
}

type experimentCtxKey struct{}

// WithExperimentContext 把探索位置放进 ctx，供本次执行读取。
func WithExperimentContext(ctx context.Context, ec ExperimentContext) context.Context {
	return context.WithValue(ctx, experimentCtxKey{}, ec)
}

// ExperimentContextFrom 读出 ctx 里的探索位置。导出是因为调用方（P1-2 的
// 循环控制器及其测试）要能验证位置确实被传下去了 —— seq 传错是那种静默出错、
// 事后才发现路径对不上的问题。
func ExperimentContextFrom(ctx context.Context) ExperimentContext {
	if ec, ok := ctx.Value(experimentCtxKey{}).(ExperimentContext); ok {
		return ec
	}
	return ExperimentContext{}
}

type paramOverrideKey struct{}

// WithParameterOverrides 让调用方覆盖本次执行的意图参数（P1-2b）。
//
// 搜索控制器需要这条通道：它采出一组参数，得让底座真的照这组参数跑。没有通道的
// 话，控制器搜它的、底座跑自己的 —— 一轮下来是同一个策略重复 N 遍。
//
// 走 context 而不是给 Execute 加形参，理由同 ExperimentContext：参数每次执行
// 都不同，而 pipeline 实例是复用的。
func WithParameterOverrides(ctx context.Context, params map[string]any) context.Context {
	return context.WithValue(ctx, paramOverrideKey{}, params)
}

// applyParameterOverrides 把覆盖并进已解析的意图：同名的替换，意图里没有的新增。
//
// 时机在解析之后、YAML 生成之前 —— 这样解析得出的参数不会丢，调用方又能
// 精确控制它关心的那几个。
func applyParameterOverrides(i *intent.Intent, ctx context.Context) {
	params, ok := ctx.Value(paramOverrideKey{}).(map[string]any)
	if !ok || len(params) == 0 || i == nil {
		return
	}
	for name, v := range params {
		replaced := false
		for j := range i.Parameters {
			if i.Parameters[j].Name == name {
				i.Parameters[j].Value = v
				replaced = true
				break
			}
		}
		if !replaced {
			i.Parameters = append(i.Parameters, intent.Parameter{
				Name: name, Type: paramTypeOf(v), Value: v,
			})
		}
	}
}

// paramTypeOf 给新增参数标类型，与 intent 包里的约定一致。
func paramTypeOf(v any) string {
	switch v.(type) {
	case int, int64:
		return "int"
	case float64, float32:
		return "float"
	case bool:
		return "bool"
	default:
		return "string"
	}
}

// Pipeline orchestrates the full strategy generation and validation flow
type Pipeline struct {
	intentParser *intent.Parser
	yamlGen      *yamlgen.Generator
	// aiClient 用接口而非具体类型，这样测试可以注入 ai.MockClient
	// 跑完整条链（意图 → YAML → 注册 → 回测）而不碰真实 LLM。
	aiClient ai.LLMClient
	// expSink 是实验日志的落点（P1-1b）。nil 表示这一路不记日志，
	// 链路照跑 —— 观测设施不该拖垮被观测的东西。
	expSink ExperimentSink
	jobs    sync.Map // jobID -> *Result
	// buildDir is the working directory passed to `go build` when
	// validating AI-generated strategy code. It must point at the
	// project root (the directory containing go.mod) so that imports
	// of pkg/domain, pkg/strategy, etc. resolve correctly.
	//
	// S7-P0-2 (ODR-043-2): previously hardcoded to a developer-machine
	// absolute path (see ODR-043-2 for the exact string), which made
	// the pipeline non-portable. It is now dynamically detected by
	// findProjectRoot at construction time and can be overridden via
	// the WithBuildDir option for testing or containerised deployment.
	buildDir string
}

// PipelineOption configures a Pipeline at construction time using the
// functional-options pattern.
type PipelineOption func(*Pipeline)

// WithBuildDir overrides the default project-root detection and sets
// the working directory used by `go build` during compilation
// validation. Useful in tests (point at a temp dir) or when the
// service binary runs from a location where go.mod is not reachable
// by walking upward from the working directory.
func WithBuildDir(dir string) PipelineOption {
	return func(p *Pipeline) {
		if dir != "" {
			p.buildDir = dir
		}
	}
}

// findProjectRoot walks upward from the current working directory
// until it finds a directory containing a go.mod file, which it
// returns as the project root. This makes the pipeline portable
// across developer machines and CI/deployment environments.
//
// Returns an error if go.mod cannot be found before reaching the
// filesystem root — in that case the caller should fall back to the
// current working directory or fail explicitly.
func findProjectRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}

	dir := cwd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root without finding go.mod.
			return "", fmt.Errorf("go.mod not found walking upward from %s", cwd)
		}
		dir = parent
	}
}

// defaultBuildDir returns the dynamically detected project root, or
// falls back to the current working directory if detection fails so
// the pipeline remains usable (with degraded compilation validation)
// rather than refusing to construct.
func defaultBuildDir() string {
	if root, err := findProjectRoot(); err == nil {
		return root
	}
	if cwd, err := os.Getwd(); err == nil {
		return cwd
	}
	return "."
}

// NewPipeline creates a new pipeline instance. The build directory
// used for compilation validation defaults to the detected project
// root; override it with the WithBuildDir option.
func NewPipeline(opts ...PipelineOption) *Pipeline {
	p := &Pipeline{
		intentParser: intent.NewParser(),
		yamlGen:      yamlgen.NewGenerator(),
		aiClient:     ai.NewClient(),
		buildDir:     defaultBuildDir(),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(p)
		}
	}
	return p
}

// NewPipelineWithDeps creates a pipeline with specific dependencies.
// The build directory defaults to the detected project root; override
// it with the WithBuildDir option.
func NewPipelineWithDeps(parser *intent.Parser, gen *yamlgen.Generator, client ai.LLMClient, opts ...PipelineOption) *Pipeline {
	p := &Pipeline{
		intentParser: parser,
		yamlGen:      gen,
		aiClient:     client,
		buildDir:     defaultBuildDir(),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(p)
		}
	}
	return p
}

// IsConfigured returns true if the pipeline can execute (AI client configured)
func (p *Pipeline) IsConfigured() bool {
	return p.aiClient != nil && p.aiClient.IsConfigured()
}

// Execute runs the full pipeline synchronously and returns the result
func (p *Pipeline) Execute(ctx context.Context, description string, runner BacktestRunner) (*Result, error) {
	result := p.StartJob(description)
	return result, p.run(ctx, result, description, runner)
}

// run 是 Execute / ExecuteAsync 共享的实现。
//
// 此前这两个方法各有一份复制粘贴的五段逻辑 —— 正是这种重复让 P0-5
// 只修一边就会漏掉另一边。现在只有一份。
func (p *Pipeline) run(ctx context.Context, result *Result, description string, runner BacktestRunner) (err error) {
	// 实验日志（P1-1b）：先落一行 running，再用 defer 收尾。
	//
	// 「先落行」是刻意的 —— 回测可能跑很久，进程崩在半路时那行 running
	// 是「跑到第几步断的」的唯一证据。等跑完再写，崩了就什么都没留下。
	expID := p.openExperiment(ctx, result, description)
	result.ExperimentID = expID
	defer func() {
		p.closeExperiment(ctx, expID, result, err)
	}()

	// Stage 1: Parse intent
	p.log(result, "Stage 1/5: Parsing intent...")
	parsedIntent, err := p.intentParser.Parse(ctx, description)
	if err != nil {
		p.fail(result, StageParse, fmt.Sprintf("Intent parsing failed: %v", err))
		return err
	}
	result.Intent = parsedIntent
	// P1-2b：把调用方覆盖的参数并进意图 —— 搜索控制器靠这条通道让底座真的
	// 照它采出来的参数跑，而不是每次都跑同一个默认配置。
	applyParameterOverrides(parsedIntent, ctx)
	p.log(result, fmt.Sprintf("Parsed intent: type=%s, name=%s", parsedIntent.StrategyType, parsedIntent.StrategyName))

	// Stage 2: Generate YAML
	p.log(result, "Stage 2/5: Generating YAML configuration...")
	yamlConfig := p.yamlGen.Generate(parsedIntent)
	if yamlConfig == "" {
		p.fail(result, StageGenerate, "YAML generation failed: empty output")
		return fmt.Errorf("yaml generation failed")
	}
	result.YAMLConfig = yamlConfig
	p.log(result, "YAML configuration generated successfully")

	// Stage 3: Resolve the strategy that will actually execute.
	//
	// P0-5 的原始缺陷是「回测拿一个从未存在的名字去查策略」。QT-17 S3-c-3
	// 之后本仓不再有注册表：YAML 的表达式段被映射成 core 内置
	// `expression_template` 的 strategy_params，由 core 按 QT-20 的
	// 「清单默认 ⊕ 覆写」生效。YAML 里的 strategy.name 只作为日志标签。
	//
	// 执行载体仍是表达式（ADR-024），不是 LLM 写的那段 Go 代码。
	p.log(result, "Stage 3/5: Resolving strategy for core-side execution...")
	resolved, cfg, err := p.resolveStrategy(yamlConfig)
	if err != nil {
		p.fail(result, StageRegister, fmt.Sprintf("Strategy resolution failed: %v", err))
		return err
	}
	p.log(result, fmt.Sprintf("Strategy resolved: label=%s run_as=%s", resolved.Label, resolved.RunName))

	// 回测区间/universe 既是回测输入，也是参数向量的一部分 —— 回放时要知道
	// 「在哪个区间、哪批票上试的」。提到 Stage 5 之外算，日志和回测共用同一份。
	startDate, endDate := cfg.Backtest.StartDate, cfg.Backtest.EndDate
	if startDate == "" {
		startDate = "2022-01-01"
	}
	if endDate == "" {
		endDate = "2024-01-01"
	}
	universe := parseUniverse(cfg.Data.Universe)
	if len(universe) == 0 {
		universe = parseUniverse(parsedIntent.Universe)
	}
	p.recordWhatWasTried(ctx, expID, result, parsedIntent, resolved.Label, cfg, startDate, endDate, universe)

	// Stage 4: Optional artifact —— LLM 写一段 Go 代码并编译校验。
	// 产物不加载、不执行，只留在结果里供人审阅，所以失败不阻断。
	p.log(result, "Stage 4/5: Generating code artifact (optional)...")
	p.generateCodeArtifact(ctx, parsedIntent, result)

	// Stage 5: Backtest (if runner provided)
	if runner != nil {
		p.log(result, "Stage 5/5: Running backtest...")
		btResult, err := p.runBacktest(ctx, resolved, universe, startDate, endDate, runner, result)
		if err != nil {
			p.fail(result, StageBacktest, fmt.Sprintf("Backtest failed: %v", err))
			return err
		}
		if err := p.recordBacktestOutcome(result, btResult); err != nil {
			return err
		}
	} else {
		p.log(result, "Stage 5/5: Skipping backtest (no runner provided)")
	}

	p.complete(result)
	return nil
}

// resolveStrategy 把 YAML 解析成「提交给 core 的策略引用 + 参数」。
//
// QT-17 S3-c-3：本仓不再注册策略（那是 core 进程内的事）。执行载体仍是
// 表达式 —— YAML 的 expression 段成为 core 内置 expression_template 的
// strategy_params，生效值由 core 按「清单默认 ⊕ 覆写」解析（QT-20）。
func (p *Pipeline) resolveStrategy(yamlStr string) (*yamlgen.ResolvedStrategy, *yamlgen.Config, error) {
	resolved, err := yamlgen.ResolveExpression(yamlStr)
	if err != nil {
		return nil, nil, err
	}
	return resolved, resolved.Config, nil
}

// generateCodeArtifact 让 LLM 生成一段 Go 代码，编译校验，把结果留在
// result.GeneratedCode / result.BuildError 里供人类审阅。
//
// 产物**不加载也不执行** —— 执行载体是 Stage 3 注册的表达式策略。因此
// 这里的任何失败都只记录、不阻断：LLM 写不出能编译的代码，不该导致
// 整个实验跑不了，何况回测压根不用这段代码。
func (p *Pipeline) generateCodeArtifact(ctx context.Context, i *intent.Intent, result *Result) {
	if p.aiClient == nil || !p.aiClient.IsConfigured() {
		p.log(result, "Code artifact skipped: AI client not configured")
		return
	}
	code, err := p.generateStrategyCode(ctx, i)
	if err != nil {
		p.log(result, fmt.Sprintf("Code artifact skipped: %v", err))
		return
	}
	result.GeneratedCode = code
	if buildErr := p.validateCompilation(code, result); buildErr != nil {
		p.log(result, fmt.Sprintf("Code artifact does not compile (not used for execution): %v", buildErr))
		return
	}
	p.log(result, "Code artifact generated and compiles")
}

// ExecuteAsync starts the pipeline asynchronously and returns the job ID
func (p *Pipeline) ExecuteAsync(ctx context.Context, description string, runner BacktestRunner) string {
	result := p.StartJob(description)

	go func() {
		// Close done when the goroutine exits (on any path) so that
		// callers waiting on <-result.done can safely read the Result
		// fields without racing with our writes. The channel close
		// establishes a happens-before edge per the Go memory model.
		defer close(result.done)
		// 与 Execute 共用 p.run —— 此前这里是复制粘贴的第二份逻辑。
		p.run(ctx, result, description, runner)
	}()

	return result.ID
}

// ExecuteFromYAML runs a backtest directly from a YAML strategy config,
// bypassing the LLM code-generation + compile path. The YAML must
// describe an expression-type strategy (either via an 'expression:'
// section or 'strategy.type: expression').
//
// S7-P3-2 (ODR-043): This closes the loop from natural-language intent
// to executable backtest without generating Go code (per ADR-015 §3
// "AI as quant researcher"). The flow is:
//
//  1. Parse YAML → Config + ResolvedStrategy（展开成 core 的
//     expression_template + strategy_params）
//  2. runner.RunBacktestWithParams（runner 支持时）提交一次粗粒度回测；
//     不支持的 runner 退回 RunBacktest(ctx, name, ...)
//
// Result.YAMLConfig is populated; GeneratedCode/BuildError are left
// empty (no codegen occurred). If runner is nil, backtest is skipped
// and the result is marked complete after the YAML resolves — useful for
// "resolve without running" callers.
func (p *Pipeline) ExecuteFromYAML(ctx context.Context, yamlStr string, runner BacktestRunner) (*Result, error) {
	result := p.StartJob("yaml-direct-execution")

	// Stage 1: Parse YAML.
	p.log(result, "Stage 1/3: Parsing YAML...")
	if _, err := yamlgen.ParseConfig(yamlStr); err != nil {
		p.fail(result, StageParse, fmt.Sprintf("YAML parse failed: %v", err))
		return result, err
	}
	result.YAMLConfig = yamlStr

	// Stage 2: Resolve into the core-side strategy reference.
	// 复用 resolveStrategy —— 与 Execute 同一份逻辑（P0-5）。
	p.log(result, "Stage 2/3: Resolving strategy for core-side execution...")
	resolved, config, err := p.resolveStrategy(yamlStr)
	if err != nil {
		p.fail(result, StageRegister, fmt.Sprintf("Strategy resolution failed: %v", err))
		return result, err
	}

	// Stage 3: Backtest (if runner provided).
	if runner != nil {
		p.log(result, "Stage 3/3: Running backtest...")
		universe := parseUniverse(config.Data.Universe)
		startDate := config.Backtest.StartDate
		endDate := config.Backtest.EndDate
		if startDate == "" {
			startDate = "2022-01-01"
		}
		if endDate == "" {
			endDate = "2024-01-01"
		}
		btResult, err := p.runBacktest(ctx, resolved, universe, startDate, endDate, runner, result)
		if err != nil {
			p.fail(result, StageBacktest, fmt.Sprintf("Backtest failed: %v", err))
			return result, err
		}
		if err := p.recordBacktestOutcome(result, btResult); err != nil {
			return result, err
		}
	} else {
		p.log(result, "Stage 3/3: Skipping backtest (no runner provided)")
	}

	p.complete(result)
	return result, nil
}

// StartJob creates a new pipeline job
func (p *Pipeline) StartJob(description string) *Result {
	jobID := uuid.New().String()
	now := time.Now()
	result := &Result{
		ID:        jobID,
		Status:    StageParse,
		StartedAt: now,
		Logs:      []string{fmt.Sprintf("Pipeline started for: %s", description)},
		done:      make(chan struct{}),
	}
	p.jobs.Store(jobID, result)
	return result
}

// GetJob retrieves a job result by ID
func (p *Pipeline) GetJob(jobID string) *Result {
	val, ok := p.jobs.Load(jobID)
	if !ok {
		return nil
	}
	return val.(*Result)
}

// WaitDone blocks until the ExecuteAsync goroutine has finished writing
// to this Result, then returns. It establishes a happens-before edge so
// callers can safely read Status/Logs/BacktestResult/etc. without racing
// with the async goroutine.
//
// WaitDone is a no-op for Results that were not created via ExecuteAsync
// (e.g. synchronous Execute or direct StartJob in tests) — in those cases
// r.done is nil and the method returns immediately. It is safe to call
// multiple times.
func (r *Result) WaitDone() {
	if r.done != nil {
		<-r.done
	}
}

// generateStrategyCode generates Go strategy code from intent
func (p *Pipeline) generateStrategyCode(ctx context.Context, i *intent.Intent) (string, error) {
	if p.aiClient == nil || !p.aiClient.IsConfigured() {
		return "", fmt.Errorf("AI client not configured")
	}

	prompt := fmt.Sprintf(`Generate a complete Go trading strategy implementing the strategy.Strategy interface.

Strategy Type: %s
Strategy Name: %s
Description: %s
Indicators: %v
Parameters: %v
Universe: %s
Timeframe: %s

Requirements:
1. Package must be "plugins"
2. Implement all Strategy interface methods
3. Use domain.OHLCV for price data
4. Return []strategy.Signal with Action "buy" or "sell"
5. Include init() function that calls strategy.GlobalRegister()
6. No fmt.Print, log.Print, or side effects
7. Keep under 100 lines

Output ONLY the Go source code, no explanations.`, i.StrategyType, i.StrategyName, i.Description, i.Indicators, i.Parameters, i.Universe, i.Timeframe)

	messages := []ai.ChatMessage{
		{Role: "system", Content: "You are an expert quantitative trading strategy developer for A-share market."},
		{Role: "user", Content: prompt},
	}

	resp, err := p.aiClient.Chat(ctx, messages)
	if err != nil {
		return "", err
	}

	// Clean up markdown fences
	resp = strings.TrimSpace(resp)
	resp = strings.TrimPrefix(resp, "```go")
	resp = strings.TrimPrefix(resp, "```")
	resp = strings.TrimSuffix(resp, "```")
	return strings.TrimSpace(resp), nil
}

// buildTimeout bounds the `go build` subprocess.
//
// 没有上界时，一个**需要联网解析**的 import 会让子进程一直等：测试侧挂到
// `go test` 超时（AUD-56 实测该包 601s 被杀），生产侧则表现为「实验卡住」
// 而不是「编译失败」—— 后者才是要写进 result.BuildError 给人看的 artifact
// （ADR-024：代码是 artifact，失败不阻断，但必须**给出结论**）。
// 上界取得宽，是为了不误杀「模块缓存冷、依赖树大」的首次编译。
const buildTimeout = 120 * time.Second

// validateCompilation compiles the generated code in a temp directory
func (p *Pipeline) validateCompilation(code string, result *Result) error {
	tmpDir, err := os.MkdirTemp("", "pipeline-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	outFile := filepath.Join(tmpDir, "strategy.go")
	if err := os.WriteFile(outFile, []byte(code), 0600); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	var stderr bytes.Buffer
	buildCtx, cancel := context.WithTimeout(context.Background(), buildTimeout)
	defer cancel()
	buildCmd := exec.CommandContext(buildCtx, "go", "build", "-o", filepath.Join(tmpDir, "strategy"), outFile)
	// S7-P0-2 (ODR-043-2): use the dynamically detected (or injected)
	// project root instead of a hardcoded developer-machine path so
	// the pipeline is portable. p.buildDir is set by NewPipeline /
	// NewPipelineWithDeps and can be overridden via WithBuildDir.
	buildCmd.Dir = p.buildDir
	buildCmd.Stderr = &stderr
	if err := buildCmd.Run(); err != nil {
		buildErr := stderr.String()
		result.BuildError = buildErr
		if buildCtx.Err() != nil {
			return fmt.Errorf("compilation timed out after %s (the build may have needed network access): %s", buildTimeout, buildErr)
		}
		return fmt.Errorf("compilation failed: %s", buildErr)
	}

	return nil
}

// runBacktest 提交一次回测。
//
// P0-5 的教训保持：名字必须是**能被执行的一方认识的**那个（现在是 core 的
// expression_template），不是意图里那个只在本仓日志里有意义的标签。
//
// runner 实现 StrategyParamsRunner 时带上 strategy_params（表达式策略必须）；
// 否则退回按名字回测 —— 那只能跑 core 已注册的具名策略。
func (p *Pipeline) runBacktest(
	ctx context.Context,
	resolved *yamlgen.ResolvedStrategy,
	universe []string,
	startDate, endDate string,
	runner BacktestRunner,
	result *Result,
) (*domain.BacktestResult, error) {
	var (
		btResult *domain.BacktestResult
		err      error
	)
	if pr, ok := runner.(StrategyParamsRunner); ok {
		btResult, err = pr.RunBacktestWithParams(ctx, resolved.RunName, resolved.Params, universe, startDate, endDate)
	} else {
		btResult, err = runner.RunBacktest(ctx, resolved.RunName, universe, startDate, endDate)
	}
	if err != nil {
		result.BacktestError = err.Error()
		return nil, err
	}

	return btResult, nil
}

// recordBacktestOutcome 把一次回测结果收进 pipeline（OBS-01）。
//
// 有效运行：照旧记日志 "Backtest completed successfully"。
// 无效运行（空票池 / 0 成交 / 零值日期 / 垃圾指标）：**不得**再报成功 ——
// 走失败路径，把引擎给出的 InvalidReasons Code 显式带进 BacktestError，
// 并返回 error，让调用方（loop / 前端 / 作业层）能判出「这次不算成功」。
//
// 结果本身仍挂到 result.BacktestResult 上：失败不等于信息丢失，调用方
// 仍可读到无效原因。
func (p *Pipeline) recordBacktestOutcome(result *Result, btResult *domain.BacktestResult) error {
	result.BacktestResult = btResult
	if btResult == nil {
		return nil
	}
	if len(btResult.InvalidReasons) > 0 {
		msg := fmt.Sprintf("backtest produced an invalid run: %s",
			strings.Join(btResult.InvalidReasons, ", "))
		result.BacktestError = msg
		p.fail(result, StageBacktest, msg)
		return fmt.Errorf("%s", msg)
	}
	p.log(result, "Backtest completed successfully")
	return nil
}

// fail marks a pipeline job as failed
// openExperiment 落一行 running，返回它的 ID。返回 0 表示「这一轮不记日志」，
// 后续补写与收尾都会静默跳过（没配 sink，或者写失败了）。
//
// 此刻还不知道参数与表达式 —— 那些要等 YAML 生成并注册之后才存在。先记下
// 原始意图，万一后面几步崩了，至少知道「本来想试什么」。
func (p *Pipeline) openExperiment(ctx context.Context, result *Result, description string) int64 {
	if p.expSink == nil {
		return 0
	}
	ec := ExperimentContextFrom(ctx)
	if ec.RunID == "" {
		ec.RunID = result.ID
	}
	if ec.DatasetSplit == "" {
		ec.DatasetSplit = aistore.DatasetSplitTrain
	}

	id, err := p.expSink.InsertExperiment(ctx, &aistore.Experiment{
		RunID:        ec.RunID,
		Seq:          ec.Seq,
		ParentID:     ec.ParentID,
		Hypothesis:   ec.Hypothesis,
		DatasetSplit: ec.DatasetSplit,
		Params:       map[string]any{"intent": description},
		Status:       aistore.ExperimentStatusRunning,
	})
	if err != nil {
		// 写不进去不能把实验搞挂，但必须留下可见痕迹：「库里没行」若被读成
		// 「没试过」，方向就完全反了 —— 实际是「试过但没记上」。
		p.log(result, fmt.Sprintf("实验日志写入失败，本次尝试不会被记录: %v", err))
		return 0
	}
	return id
}

// recordWhatWasTried 把「试了什么」补进已落的那一行。
//
// 分两步写不是啰嗦，是顺序不能反：先落行（崩溃时有证据），参数确定后再补。
// 到这一刻 YAML 已生成、策略已注册，表达式与回测区间才真正存在。
func (p *Pipeline) recordWhatWasTried(ctx context.Context, id int64, result *Result, i *intent.Intent, strategyLabel string, cfg *yamlgen.Config, startDate, endDate string, universe []string) {
	if id == 0 {
		return
	}
	params := map[string]any{
		"intent":     i.RawText,
		"start_date": startDate,
		"end_date":   endDate,
		"universe":   universe,
	}
	for _, pm := range i.Parameters {
		if pm.Name != "" {
			params[pm.Name] = pm.Value
		}
	}
	if cfg.Expression.Signal.Lookback > 0 {
		params["lookback"] = cfg.Expression.Signal.Lookback
	}
	if cfg.Expression.Signal.MinStrength > 0 {
		params["min_strength"] = cfg.Expression.Signal.MinStrength
	}

	if err := p.expSink.UpdateExperiment(ctx, id, aistore.ExperimentUpdate{
		Params:       params,
		StrategyName: strategyLabel,
		Expression:   cfg.Expression.Signal.Expression,
	}); err != nil {
		// 补写失败同样不阻断：主行已经在库里，只是参数粗一点。
		p.log(result, fmt.Sprintf("实验日志补写参数失败: %v", err))
	}
}

// closeExperiment 收尾一次尝试。err 是 run 的最终错误，决定落成 completed
// 还是 failed —— **两者都要落**，失败尤其要落。
func (p *Pipeline) closeExperiment(ctx context.Context, id int64, result *Result, err error) {
	if id == 0 {
		return
	}
	if err != nil {
		if e := p.expSink.CompleteExperiment(ctx, id, nil, err.Error()); e != nil {
			p.log(result, fmt.Sprintf("实验日志收尾失败（失败态）: %v", e))
		}
		return
	}

	// 没跑回测（runner 为 nil）时 BacktestResult 是 nil，指标留空 ——
	// 那也是一次真实的尝试，只是没有产出指标。
	var m *aistore.ExperimentMetrics
	if bt := result.BacktestResult; bt != nil {
		m = &aistore.ExperimentMetrics{
			SharpeRatio: bt.SharpeRatio,
			TotalReturn: bt.TotalReturn,
			TotalTrades: bt.TotalTrades,
		}
	}
	if e := p.expSink.CompleteExperiment(ctx, id, m, ""); e != nil {
		p.log(result, fmt.Sprintf("实验日志收尾失败: %v", e))
	}
}

func (p *Pipeline) fail(result *Result, stage Stage, message string) {
	result.Status = StageFailed
	result.BuildError = message
	now := time.Now()
	result.CompletedAt = &now
	result.DurationMs = now.Sub(result.StartedAt).Milliseconds()
	p.log(result, fmt.Sprintf("FAILED at stage %s: %s", stage, message))
}

// complete marks a pipeline job as completed
func (p *Pipeline) complete(result *Result) {
	result.Status = StageComplete
	now := time.Now()
	result.CompletedAt = &now
	result.DurationMs = now.Sub(result.StartedAt).Milliseconds()
	p.log(result, "Pipeline completed successfully")
}

// log adds a log entry to the result
func (p *Pipeline) log(result *Result, message string) {
	result.Logs = append(result.Logs, fmt.Sprintf("[%s] %s", time.Now().Format("15:04:05"), message))
}

func parseUniverse(universe string) []string {
	if universe == "" || universe == "all" {
		return nil
	}
	if strings.HasPrefix(universe, "universe:") {
		universe = strings.TrimPrefix(universe, "universe:")
	}
	parts := strings.Split(universe, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		result = append(result, strings.TrimSpace(p))
	}
	return result
}
