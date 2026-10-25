package copilot

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/ruoxizhnya/tellurion-contracts/domain"
)

// LLMClient is the local contract for an LLM backend that generates
// strategy code. S7-P1-2 (ODR-043): this interface is intentionally
// defined HERE in pkg/strategy (lower layer) rather than imported from
// pkg/ai (higher layer) — defining it locally breaks the
// strategy → ai reverse dependency while preserving Go's structural
// typing: any concrete type with these three methods (e.g. *ai.Client,
// *ai.MockClient) satisfies this interface without an explicit adapter.
//
// cmd/analysis/main.go wires the real *ai.Client at the composition
// root via WithLLMClient; tests inject *ai.MockClient or a local stub.
type LLMClient interface {
	IsConfigured() bool
	GenerateStrategyCode(ctx context.Context, description string) (string, error)
	FixStrategyCode(ctx context.Context, code string, buildErrors string) (string, error)
}

// CodeChecker is the local contract for the regex-based staticcheck
// sandbox gate (Sprint 6 P0-4 / ADR-007 Phase 1). S7-P1-2: defined
// locally to break the strategy → internal/sandbox/staticcheck reverse
// dependency. The real adapter lives in cmd/analysis/main.go.
type CodeChecker interface {
	CheckOrError(code string) error
}

// BuildExecutor is the local contract for the process-isolation
// sandbox runner that compiles LLM-generated strategies (Sprint 6
// P1-11 / ODR-020). S7-P1-2: defined locally to break the
// strategy → internal/sandbox/runner reverse dependency. The real
// adapter lives in cmd/analysis/main.go.
//
// IsTimeout reports whether err is a timeout from the underlying runner
// (mirrors errors.Is(err, sandboxrunner.ErrTimeout) without leaking
// that sentinel across layers).
type BuildExecutor interface {
	Run(ctx context.Context, name string, args []string, workingDir string) (stdout, stderr *bytes.Buffer, err error)
	IsTimeout(err error) bool
}

// ChainSink is the local contract for the durable generation trace
// (QT-32-A). Defined here, like LLMClient / CodeChecker / BuildExecutor,
// so this package does not reach into the store layer; the adapter lives
// at the composition root (cmd/ai/builders.go).
//
// A generation that cannot be traced is rejected. Before this port existed,
// a copilot job lived only in the in-memory job map and its source only in
// a temp directory that `defer os.RemoveAll` deleted — so a restarted
// process left no evidence the generation ever happened, which is exactly
// what QT-32 ① is about.
type ChainSink interface {
	SaveChain(ctx context.Context, entry ChainEntry) error
}

// ChainEntry 是一次生成尝试在留痕上的当前状态（QT-32-A）。
//
// 链的每段推进都带**全量**状态写一次，所以它是快照不是补丁：读到哪一行，
// 就是那次推进结束时的事实，不需要再拼别的表。
type ChainEntry struct {
	JobID   string
	Attempt int
	Prompt  string
	// Universe / StartDate / EndDate 是人要求的回测区间。闸门今天把它拦住了，
	// 但「当时要求跑什么」也是链上的一环，拦下照样留痕。
	Universe  string
	StartDate string
	EndDate   string
	// Source 是**真正交给编译器**的那份字节。重试修过就是另一份、另一行、
	// 另一个指纹 —— 链要能回答「最后编进去的到底是哪一版」。
	Source       string
	SourceSha256 string

	BuildState    string
	BuildError    string
	BlockedReason string
}

// 编译段的状态取值。它与 aistore 的 GenerationBuild* 一一对应，对应关系由装配根
// 的护栏钉住（cmd/ai/chain_sink_test.go）：两处各持一套词表而没人核对，
// 就是漂移的开始。
const (
	// BuildStateNone：还没走到编译 —— 含「出码了但被静态闸当场拒掉，一份都没
	// 送进沙箱」。拒掉的源码与原因照样写在同一行里。
	BuildStateNone = "none"
	// BuildStateFailed：送进沙箱了，`go build` 没过。
	BuildStateFailed = "failed"
	// BuildStatePassed：沙箱编译过了。过了不等于能拿去跑 —— 那一段是闸门的事。
	BuildStatePassed = "passed"
)

// BlockedPromoteGate 是链被拦下的唯一一种原因（与 aistore 的
// GenerationBlockedPromoteGate 同一取值）：AI 产物没有可核验的资产坐标，
// 未过 promote 人工闸门，不进回测（QT-32 ③-②）。
//
// 装配根没注入留痕落点也是 fail-closed，但那不是「闸门拦下」而是「没开工」——
// 它连一行链都写不出去，所以只在 job 的错误信息里点名，不占这个枚举。
const BlockedPromoteGate = "promote_gate"

// CopilotService generates Go strategy code from natural-language
// descriptions and traces every generation into ai.generation_chain.
//
// It deliberately does **not** run a backtest with the generated code.
// An AI-produced strategy has to become a registered asset and pass the
// human `promote` gate before anything executes it (QT-32 ③-②, and ADR-024's
// 「LLM 生成的 Go 代码只是可审阅 artifact，不加载、不执行」). See the gate in run().
type CopilotService struct {
	// aiClient is the LLM backend. S7-P1-2: typed as the LOCAL LLMClient
	// interface (defined in this file) so pkg/strategy no longer imports
	// pkg/ai. *ai.Client / *ai.MockClient satisfy this interface via Go's
	// structural typing — tests can still inject *ai.MockClient.
	aiClient LLMClient

	// codeChecker is the regex-based staticcheck sandbox gate. S7-P1-2:
	// typed as the LOCAL CodeChecker interface so pkg/strategy no longer
	// imports internal/sandbox/staticcheck. nil → run() fails closed.
	codeChecker CodeChecker

	// buildExecutor is the process-isolation sandbox runner. S7-P1-2:
	// typed as the LOCAL BuildExecutor interface so pkg/strategy no
	// longer imports internal/sandbox/runner. nil → run() fails closed
	// at the build step.
	buildExecutor BuildExecutor

	// chain is the durable generation trace (QT-32-A). Injected via
	// WithChainSink; nil → run() fails closed **before** the LLM call,
	// because an untraceable generation is the thing this ticket removes.
	chain ChainSink

	generated  int64 // total generated (LLM called)
	buildable  int64 // build succeeded
	backtested int64 // backtest produced valid result (≥1 trade).
	// 结构上恒 0 —— 闸门不放行任何未 promote 的产物（见 run() 里的闸门段）。
	// 字段保留是因为 /api/copilot/stats 的 backtest_valid / acceptance_rate 是
	// 对外契约（ui 侧 tellurion-ui:web/src/types/api.ts 消费），删字段是跨仓
	// breaking；它此前也没非 0 过：旧实现拿硬编码策略名去 core 跑回测，
	// 从未注册过那个名字，每次都断在 core。

	jobs sync.Map // jobID string -> *JobResult

	// Sprint 6 P0-4 (ODR-013): WorkingDir replaces the previously
	// hard-coded `buildCmd.Dir = "/Users/ruoxi/longshaosWorld/tellurion"`.
	//
	// WorkingDir is the directory `go build` runs in when compiling
	// the LLM-generated strategy. It must contain a `go.mod` so that
	// the generated file can resolve its imports
	// (github.com/ruoxizhnya/tellurion-contracts/domain, etc.). The
	// service does NOT auto-detect this — callers MUST set it
	// explicitly via WithWorkingDir so the configuration is
	// environment-agnostic and reviewable in code review.
	//
	// Empty WorkingDir causes the service to fail-closed at build
	// time (it returns "sandbox_rejected" with a clear error). We do
	// NOT silently fall back to os.Getwd() because the typical
	// caller is a long-running server whose working directory may
	// change between restarts.
	workingDir string

	// logger is the structured logger used by the sandbox gate and
	// retry loop. Defaults to zerolog.Nop() if the caller did not
	// set one (so unit tests don't have to thread a logger through).
	logger zerolog.Logger
}

// JobResult holds the outcome of a copilot generation job.
//
// In-memory, per-process, and **not** the trace: durable evidence goes to
// ai.generation_chain (QT-32-A). This struct is the poll-this-job view; the
// chain is the answer to "which generation was that, and what did you do
// with it" after the process is gone.
//
// StrategyName / BacktestResult / BacktestErr are currently unsettable —
// the gate refuses to hand un-promoted AI output to a backtest. They stay
// because the JSON keys are a published contract
// (tellurion-ui:web/src/types/api.ts 的 CopilotJobStatus); filling them is
// what the asset-service leg (QT-32-A ⑦-1) buys back.
type JobResult struct {
	sync.Mutex
	JobID          string                 `json:"job_id"`
	Status         string                 `json:"status"`
	Code           string                 `json:"generated_code,omitempty"`
	BuildErr       string                 `json:"build_error,omitempty"`
	StrategyName   string                 `json:"strategy_name,omitempty"`
	BacktestResult *domain.BacktestResult `json:"backtest_result,omitempty"`
	BacktestErr    string                 `json:"backtest_error,omitempty"`
}

// GenerateParams are the input parameters for strategy generation.
type GenerateParams struct {
	Description string `json:"description"`
	Universe    string `json:"universe"`   // csi300, csi500, csi800, all
	StartDate   string `json:"start_date"` // YYYY-MM-DD
	EndDate     string `json:"end_date"`   // YYYY-MM-DD
}

// NewCopilotService creates a new CopilotService with NO dependencies
// wired. S7-P1-2 (ODR-043): the caller MUST inject the LLM client,
// code checker, and build executor via WithLLMClient / WithCodeChecker
// / WithBuildExecutor. This fail-closed DI pattern replaces the old
// behavior of calling ai.NewClient() internally, which created a
// strategy → ai reverse dependency.
//
// Backward-compat note: cmd/analysis/main.go previously relied on
// NewCopilotService() to read AI_API_KEY/AI_API_URL from env. That
// wiring now lives in main.go explicitly (composition root).
func NewCopilotService() *CopilotService {
	return &CopilotService{
		logger: zerolog.Nop(),
	}
}

// NewCopilotServiceWithLLM creates a CopilotService with an explicit
// LLMClient. S7-P1-2: accepts the LOCAL LLMClient interface (not
// ai.LLMClient) so this file doesn't import pkg/ai. *ai.MockClient
// and *ai.Client satisfy this interface via structural typing.
//
// If client is nil, aiClient stays nil — the service is "not configured"
// (IsConfigured returns false) and run() will fail at the first LLM
// call. This is the fail-closed behavior; the caller must inject a
// real client via WithLLMClient if needed.
func NewCopilotServiceWithLLM(client LLMClient) *CopilotService {
	return &CopilotService{aiClient: client, logger: zerolog.Nop()}
}

// WithWorkingDir returns the receiver with WorkingDir set to dir.
// This is the post-construction setter used by cmd/analysis/main.go
// once it has read the value from analysis-service.yaml. The method
// returns the service so it can be chained immediately after
// construction:
//
//	svc := copilot.NewCopilotService().WithWorkingDir(v.GetString("copilot.working_dir"))
//
// dir is NOT validated at this point — the validation is deferred to
// the first Generate() call so a misconfigured server can still start
// (and report the issue via /health or /api/copilot/generate) instead
// of crashing on boot.
func (s *CopilotService) WithWorkingDir(dir string) *CopilotService {
	s.workingDir = dir
	return s
}

// WorkingDir returns the currently configured working directory.
// Exported so config-bootstrap code and tests can assert that the
// service received a working dir at all.
func (s *CopilotService) WorkingDir() string {
	return s.workingDir
}

// WithLogger attaches a structured logger to the service. Production
// callers in cmd/analysis use the root logger with a `component=copilot`
// field. Tests may leave the default zerolog.Nop() in place.
func (s *CopilotService) WithLogger(l zerolog.Logger) *CopilotService {
	if l.GetLevel() != zerolog.Disabled {
		s.logger = l
	}
	return s
}

// WithLLMClient injects the LLM client. S7-P1-2: this is the
// composition-root wiring point that replaces the old ai.NewClient()
// call inside NewCopilotService(). Pass *ai.Client (production) or
// *ai.MockClient / a local stub (tests).
func (s *CopilotService) WithLLMClient(c LLMClient) *CopilotService {
	s.aiClient = c
	return s
}

// WithCodeChecker injects the regex-based staticcheck sandbox gate.
// S7-P1-2: this is the composition-root wiring point that replaces
// the old direct staticcheck.CheckOrError() call. Pass an adapter
// wrapping internal/sandbox/staticcheck (see cmd/analysis/main.go).
// If never called, run() fails closed with "code checker not configured".
func (s *CopilotService) WithCodeChecker(c CodeChecker) *CopilotService {
	s.codeChecker = c
	return s
}

// WithBuildExecutor injects the process-isolation sandbox runner.
// S7-P1-2: this is the composition-root wiring point that replaces
// the old direct sandboxrunner.New() call. Pass an adapter wrapping
// internal/sandbox/runner (see cmd/analysis/main.go). If never called,
// run() fails closed at the build step with "build executor not configured".
func (s *CopilotService) WithBuildExecutor(e BuildExecutor) *CopilotService {
	s.buildExecutor = e
	return s
}

// WithChainSink injects the durable generation trace (QT-32-A). Like the
// checker and the executor, this is a composition-root wiring point:
// never called → run() fails closed before the LLM call, because a
// generation that leaves no row is unauditable after the process dies.
func (s *CopilotService) WithChainSink(c ChainSink) *CopilotService {
	s.chain = c
	return s
}

// IsConfigured returns true if AI client is configured.
func (s *CopilotService) IsConfigured() bool {
	return s.aiClient != nil && s.aiClient.IsConfigured()
}

// Generate starts a strategy generation job and returns immediately.
// Callers poll via GetJob.
func (s *CopilotService) Generate(ctx context.Context, params GenerateParams) *JobResult {
	jobID := uuid.New().String()
	result := &JobResult{JobID: jobID, Status: "pending"}
	s.jobs.Store(jobID, result)
	atomic.AddInt64(&s.generated, 1)

	go func() {
		// WithoutCancel 保留 ctx 里的值（QT-17 S3-c-3：入站请求的 JWT 由
		// handler 放进 ctx），同时保持旧的生命周期语义 —— 作业不随响应返回
		// 而被取消。换成 Background() 会把凭证一起丢掉。
		s.run(context.WithoutCancel(ctx), jobID, params)
	}()

	return result
}

// GetJob retrieves a job result by ID.
func (s *CopilotService) GetJob(jobID string) *JobResult {
	val, ok := s.jobs.Load(jobID)
	if !ok {
		return nil
	}
	return val.(*JobResult)
}

// Stats returns acceptance statistics.
func (s *CopilotService) Stats() (generated, buildable, backtested int64) {
	return atomic.LoadInt64(&s.generated),
		atomic.LoadInt64(&s.buildable),
		atomic.LoadInt64(&s.backtested)
}

// AcceptanceRate returns the fraction of generated strategies that produced valid backtests.
func (s *CopilotService) AcceptanceRate() float64 {
	g := atomic.LoadInt64(&s.generated)
	if g == 0 {
		return 0
	}
	return float64(atomic.LoadInt64(&s.backtested)) / float64(g)
}

func (s *CopilotService) run(ctx context.Context, jobID string, params GenerateParams) {
	result := s.GetJob(jobID)
	if result == nil {
		return
	}

	// QT-32-A ③-3：留痕是生成的**前置**，不是附加项。没有落点就不开始 ——
	// 与 nil codeChecker / nil buildExecutor 两条既有腿同形。
	if s.chain == nil {
		s.logger.Warn().
			Str("job_id", jobID).
			Msg("chain sink not configured; rejecting generated strategy (fail-closed)")
		result.Lock()
		result.Status = "sandbox_rejected"
		result.BuildErr = "chain sink not configured; call WithChainSink() at the composition root"
		result.Unlock()
		return
	}

	entry := ChainEntry{
		JobID:      jobID,
		Prompt:     params.Description,
		Universe:   params.Universe,
		StartDate:  params.StartDate,
		EndDate:    params.EndDate,
		BuildState: BuildStateNone,
	}
	// 链的第一环在人给的意图上，不在 LLM 的产出上：后面任何一步失败，这一行
	// 都已经在了。「跑过但事后查不到」从此不存在。
	if !s.trace(ctx, result, entry) {
		return
	}

	code, err := s.aiClient.GenerateStrategyCode(ctx, params.Description)
	if err != nil {
		result.Lock()
		result.Status = "llm_failed"
		result.BuildErr = err.Error()
		result.Unlock()
		return
	}

	result.Lock()
	result.Code = code
	result.Unlock()

	// Sprint 6 P0-4 (ODR-013): Stage-1 sandbox gate.
	//
	// Run the regex-based staticcheck pass BEFORE creating any temp
	// directory or invoking `go build`. If the LLM produced code
	// containing known-dangerous patterns (os.RemoveAll, exec.Command,
	// net.Dial, panic, …), reject the job with status
	// "sandbox_rejected" and a finding-bearing error message.
	//
	// This is the cheap, fail-closed filter from ADR-007 Phase 1; the
	// process-isolation sandbox (Phase 2) is tracked under Sprint 6
	// P1-11.
	//
	// S7-P1-2 (ODR-043): the check is delegated to s.codeChecker
	// (local CodeChecker interface) instead of calling
	// staticcheck.CheckOrError directly, breaking the strategy →
	// internal/sandbox reverse dependency. If s.codeChecker is nil
	// (caller forgot to inject), we fail closed — rejecting the code
	// rather than silently allowing it through the gate.
	if s.codeChecker == nil {
		s.stop(ctx, result, entry, "sandbox_rejected",
			"code checker not configured; call WithCodeChecker() at the composition root")
		return
	}
	if err := s.codeChecker.CheckOrError(code); err != nil {
		s.logger.Warn().
			Err(err).
			Str("job_id", jobID).
			Msg("Generated strategy rejected by staticcheck sandbox gate (P0-4)")
		// 被静态闸拒掉的那一份也要留：它同样是一条「LLM 产出了什么」的证据，
		// 而且是人复查生成质量时想看的那一种。
		entry.Source = code
		entry.SourceSha256 = sha256Hex(code)
		s.stop(ctx, result, entry, "sandbox_rejected", err.Error())
		return
	}

	tmpDir, err := os.MkdirTemp("", "copilot-*")
	if err != nil {
		s.stop(ctx, result, entry, "build_failed", "failed to create temp dir: "+err.Error())
		return
	}
	defer os.RemoveAll(tmpDir)

	code = strings.TrimPrefix(code, "```go")
	code = strings.TrimPrefix(code, "```")
	code = strings.TrimSuffix(code, "```")
	code = strings.TrimSpace(code)

	// Sprint 6 P0-4 (ODR-013): fail-closed if WorkingDir is not set.
	// The previous hard-coded path
	// "/Users/ruoxi/longshaosWorld/tellurion" was the only reason
	// this code worked on a single developer's machine; in any other
	// environment (CI, Docker, a different developer's laptop) the
	// `go build` would have failed silently with a non-obvious
	// "no go.mod" error. Configuration must be explicit.
	if s.workingDir == "" {
		s.stop(ctx, result, entry, "sandbox_rejected",
			"copilot.working_dir is not configured; set it in analysis-service.yaml under the `copilot:` key")
		return
	}

	const maxRetries = 2

	for attempt := 0; attempt <= maxRetries; attempt++ {
		// 一次尝试一行：源码 + 指纹 + 编译结论。LLM 修过的那一版是另一份
		// 字节、另一个指纹、另一行 —— 链要能回答「最后编进去的到底是哪一版」。
		entry.Attempt = attempt
		entry.Source = code
		entry.SourceSha256 = sha256Hex(code)
		entry.BuildState = BuildStateNone
		entry.BuildError = ""
		entry.BlockedReason = ""

		outFile := filepath.Join(tmpDir, fmt.Sprintf("strategy_v%d.go", attempt))
		if err := os.WriteFile(outFile, []byte(code), 0600); err != nil {
			s.stop(ctx, result, entry, "build_failed", "failed to write file: "+err.Error())
			return
		}

		// 先把「这一版要编」写进链，再动手编：进程死在 `go build` 中间，
		// 库里也留着这一 attempt，而不是什么都没有。
		if !s.trace(ctx, result, entry) {
			return
		}

		// S7-P1-2 (ODR-043): fail-closed if build executor is not
		// injected. The old code called sandboxrunner.New() inline,
		// creating a strategy → internal/sandbox/runner reverse
		// dependency. The executor is now injected via
		// WithBuildExecutor at the composition root; nil means the
		// caller forgot to wire it, and we reject rather than
		// silently skipping the build.
		if s.buildExecutor == nil {
			s.logger.Warn().
				Str("job_id", jobID).
				Msg("build executor not configured; rejecting build step (fail-closed)")
			s.stop(ctx, result, entry, "build_failed",
				"build executor not configured; call WithBuildExecutor() at the composition root")
			return
		}

		// Sprint 6 P1-11 (ODR-020): process-isolation sandbox. The
		// `go build` command is executed via the injected BuildExecutor
		// (S7-P1-2: was sandboxrunner.New() inline) which enforces a
		// 30s wall-clock timeout, a 1GB virtual memory cap, and runs
		// the child in its own session / process group. If the LLM
		// produces a runaway-loop / fork-bomb / mem-leak build, the
		// executor kills it before it can DoS the analysis service.
		var stderr bytes.Buffer
		buildOut := filepath.Join(tmpDir, fmt.Sprintf("strategy_v%d", attempt))
		_, buildStderr, err := s.buildExecutor.Run(ctx, "go", []string{"build", "-o", buildOut, outFile}, s.workingDir)
		// Copy the executor's stderr capture into our local buffer so the
		// rest of the loop (LLM retry logic) keeps working unchanged.
		if buildStderr != nil {
			stderr.Write(buildStderr.Bytes())
		}
		if err != nil {
			buildErr := stderr.String()
			if s.buildExecutor.IsTimeout(err) {
				s.logger.Warn().
					Err(err).
					Str("job_id", jobID).
					Int("attempt", attempt).
					Msg("Sandbox runner killed `go build` after timeout (P1-11)")
			}
			if attempt < maxRetries && s.aiClient != nil && s.aiClient.IsConfigured() {
				fixedCode, fixErr := s.aiClient.FixStrategyCode(ctx, code, buildErr)
				if fixErr == nil && fixedCode != "" {
					// Re-run the sandbox gate on the LLM's fix. A
					// self-correcting LLM that, in a retry,
					// accidentally introduces an os.RemoveAll
					// (because it copied a snippet from training
					// data) would otherwise sneak past us.
					if recheckErr := s.codeChecker.CheckOrError(fixedCode); recheckErr == nil {
						// 换下一版之前，先把这一 attempt 的失败结论落进链。
						// 不落的话，链上那一行停在「要编」（none），把「第一版
						// 确实编不过」这个事实抹掉了 —— 而那正是生成质量的证据。
						entry.BuildState = BuildStateFailed
						entry.BuildError = buildErr
						if !s.trace(ctx, result, entry) {
							return
						}
						code = fixedCode
						result.Lock()
						result.Code = code
						result.Unlock()
						continue
					} else {
						s.logger.Warn().
							Err(recheckErr).
							Str("job_id", jobID).
							Int("attempt", attempt).
							Msg("LLM's retry code rejected by staticcheck; aborting retry loop")
					}
				}
			}
			entry.BuildState = BuildStateFailed
			s.stop(ctx, result, entry, "build_failed", buildErr)
			return
		}

		entry.BuildState = BuildStatePassed
		atomic.AddInt64(&s.buildable, 1)
		break
	}

	// ── QT-32-A ③-5：promote 人工闸门（生产者侧）─────────────────────
	//
	// 编译过了 ≠ 可以拿去跑。AI 产物要先进资产服务、过人的 promote，回测才
	// 有「这次跑的就是这一份源码」的可核验坐标：
	//   - 闸门结论的真相在 strategy.strategy_artifacts.status
	//     （tellurion-strategy:internal/asset/ddl.go 的状态机 +
	//     internal/httpapi/api.go 的 POST .../versions/{hash}/promote，
	//     那里明写「promote 是人的闸门」）。本仓不存第二份结论。
	//   - 本仓今天也**没有**那个坐标可交：go build 的临时可执行文件不是资产
	//     服务认可的载体，version_hash 又需要资产服务签发的 strategy_id
	//     （tellurion-contracts:engine/provenance.go:65 的六项输入）。
	// 所以只能拦，不能猜。旧实现在这里做的是另一件事：plugin.Open 打开一个
	// 非 plugin 产物必然失败，然后回落成硬编码字面量 "generated-strategy"
	// 去让 core 跑回测 —— core 从未注册过那个名字，于是这条链一路断在远端，
	// 本仓既没说谎也没留痕，只是没人知道它断了。
	entry.BlockedReason = BlockedPromoteGate
	s.stopAtGate(ctx, result, entry,
		"编译通过；AI 产物未过 promote 人工闸门，不进回测（QT-32-A）。需要资产服务登记的 artifact 坐标后，这一环由 tellurion-strategy 的 resolve/promote 判定。")
}

// trace 把链上的一段推进写下去。写不进去 = 这一轮不许继续（QT-32-A ③-3）：
// 一条记不下来的生成，等价于一次无法复查的生成。
// 返回 true 表示调用方可以继续。
func (s *CopilotService) trace(ctx context.Context, result *JobResult, entry ChainEntry) bool {
	if err := s.chain.SaveChain(ctx, entry); err != nil {
		s.logger.Error().
			Err(err).
			Str("job_id", entry.JobID).
			Int("attempt", entry.Attempt).
			Msg("generation trace write failed; aborting job (fail-closed)")
		s.rejectJob(result, "sandbox_rejected",
			fmt.Sprintf("生成链写不进去（ai.generation_chain）：%v", err))
		return false
	}
	return true
}

// stop 收一个**环节错误**造成的终态：先更新内存里的 job（reason 是人唯一的
// 解释槽），再把同一句写进链的 build_error。链写失败只记日志 —— 此刻终态已经
// 落定，不能因为审计口失灵就把人看到的 job 改回「还在跑」。
func (s *CopilotService) stop(ctx context.Context, result *JobResult, entry ChainEntry, status, reason string) {
	s.rejectJob(result, status, reason)
	entry.BuildError = reason
	if err := s.chain.SaveChain(ctx, entry); err != nil {
		s.logger.Error().
			Err(err).
			Str("job_id", entry.JobID).
			Int("attempt", entry.Attempt).
			Msg("job terminated but its generation trace row was not written")
	}
}

// stopAtGate 收一个**闸门拦下**的终态。它与 stop 分开是有意的：被拦不是编译
// 错误，把「未过 promote 闸门」那句塞进链的 build_error，就是把两条轴混成一条
// —— 读库的人从此分不出「编不过」与「编过了但不许跑」，而那恰恰是要看的差别。
// 链上说话的是枚举 blocked_reason；给人看的解释只在 job 的 build_error 里。
func (s *CopilotService) stopAtGate(ctx context.Context, result *JobResult, entry ChainEntry, reason string) {
	s.rejectJob(result, "sandbox_rejected", reason)
	if err := s.chain.SaveChain(ctx, entry); err != nil {
		s.logger.Error().
			Err(err).
			Str("job_id", entry.JobID).
			Int("attempt", entry.Attempt).
			Msg("job blocked at gate but its generation trace row was not written")
	}
}

// rejectJob 是终态在内存侧的收口（链的写法由调用方决定）。
func (s *CopilotService) rejectJob(result *JobResult, status, reason string) {
	result.Lock()
	result.Status = status
	result.BuildErr = reason
	result.Unlock()
}

// sha256Hex 是这一行源码的指纹：对它自己存的字节算，用途是重复检测与篡改
// 检测。它**不是** version_hash（那需要资产服务签发的 strategy_id，见
// tellurion-contracts:engine/provenance.go:65），也不是 artifact_hash
// （同文件 :39，只覆盖编译产物字节）。名字直说它是什么，免得将来有人拿它
// 当执行真相。
func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
