package copilot

// QT-32-A：生成链留痕 + promote 人工闸门（生产者侧）。
//
// 这组用例钉的是四件事，每件都对应一次「今天做不到、以后也不能悄悄做到」：
//  1. 没有留痕落点就不开始生成 —— 连 LLM 都不该调（花钱买一份查不到的东西）；
//  2. 链上每行的指纹覆盖它自己存的源码，改一个字节就是另一个指纹；
//  3. 编译成功**不等于**可以拿去跑：闸门把链停在 promote_gate；
//  4. 那条「拿硬编码策略名去 core 跑回测」的假路径不许复活（源码级守卫）。

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── 假件 ─────────────────────────────────────────────────────────────

// recordingSink 记下每一次写链调用（含失败注入）。
//
// 副本化存储：ChainEntry 是值类型，但里面的字符串不可变，所以直接 append 就够；
// 失败注入用 errAt（第 N 次写失败）而不是布尔开关 —— 让「第一环就写不进」与
// 「编译之后才写不进」能分开测。
type recordingSink struct {
	mu      sync.Mutex
	entries []ChainEntry
	errAt   int // 0 = 不失败；N>0 = 第 N 次写返回 err
	calls   int
}

var errSinkDown = errors.New("sink: ai.generation_chain unreachable")

func (r *recordingSink) SaveChain(_ context.Context, e ChainEntry) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls++
	if r.errAt > 0 && r.calls == r.errAt {
		return errSinkDown
	}
	r.entries = append(r.entries, e)
	return nil
}

func (r *recordingSink) all() []ChainEntry {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]ChainEntry, len(r.entries))
	copy(out, r.entries)
	return out
}

// last 返回最后一次写链的内容；一次都没写过则 fail。
func (r *recordingSink) last(t *testing.T) ChainEntry {
	t.Helper()
	got := r.all()
	require.NotEmpty(t, got, "生成链一次都没写 —— 留痕没接上")
	return got[len(got)-1]
}

// shaOf 按同一套算法重算源码指纹，用来做「自证一致」断言。
func shaOf(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// buildServiceWithChain 装配一条完整依赖链（LLM / 静态闸 / 编译 / 留痕 / 工作目录）。
func buildServiceWithChain(llm LLMClient, checker CodeChecker, exec BuildExecutor, sink ChainSink) *CopilotService {
	return NewCopilotService().
		WithLLMClient(llm).
		WithCodeChecker(checker).
		WithBuildExecutor(exec).
		WithChainSink(sink).
		WithWorkingDir(os.TempDir())
}

// ── ① 留痕是前置，不是附加项 ────────────────────────────────────────

// TestQT32A_NilChainSink_FailsClosedBeforeCallingLLM：没装留痕落点就不开工，
// 而且**连 LLM 都不调用** —— 否则花一次真生成的钱买一份事后查不到的产物。
func TestQT32A_NilChainSink_FailsClosedBeforeCallingLLM(t *testing.T) {
	llm := &stubLLMClient{configured: true, generateResponse: "package x\nfunc f() {}\n"}
	svc := NewCopilotService().WithLLMClient(llm).WithCodeChecker(&stubCodeChecker{}).WithWorkingDir(t.TempDir())
	require.Nil(t, svc.chain, "本用例测的就是没注入 sink 的装配形态")

	res := svc.Generate(context.Background(), GenerateParams{Description: "momentum"})
	require.NotNil(t, res)
	waitTerminal(t, svc, res.JobID, 2*time.Second)

	job := svc.GetJob(res.JobID)
	job.Lock()
	status, buildErr := job.Status, job.BuildErr
	job.Unlock()

	assert.Equal(t, "sandbox_rejected", status,
		"没有留痕落点必须 fail-closed 拒绝，不能静默生成一份查不到的产物")
	assert.Contains(t, buildErr, "chain sink not configured",
		"错误要点名缺的是哪一个依赖，运维才知道去装配根补：got %q", buildErr)
	assert.Equal(t, int32(0), atomic.LoadInt32(&llm.generateCalls),
		"没装 sink 时 LLM 一次都不该被调用（0 花费，0 无法复查的产物）")
}

// TestQT32A_TraceWriteFailure_AbortsBeforeLLM：sink 装了但第一环就写不进去
// —— 同样不能继续。这条与上一条区分开：「没装配」是配置问题，「写失败」是
// 运行时故障，两者都必须拦住生成，不能因为「至少装配了」就放行。
func TestQT32A_TraceWriteFailure_AbortsBeforeLLM(t *testing.T) {
	llm := &stubLLMClient{configured: true, generateResponse: "package x\nfunc f() {}\n"}
	sink := &recordingSink{errAt: 1}
	svc := buildServiceWithChain(llm, &stubCodeChecker{}, &stubBuildExecutor{}, sink)

	res := svc.Generate(context.Background(), GenerateParams{Description: "momentum"})
	waitTerminal(t, svc, res.JobID, 2*time.Second)

	job := svc.GetJob(res.JobID)
	job.Lock()
	status, buildErr := job.Status, job.BuildErr
	job.Unlock()

	assert.Equal(t, "sandbox_rejected", status)
	assert.Contains(t, buildErr, "ai.generation_chain",
		"要点名是哪张表写不进去，而不是笼统一句「写失败」：got %q", buildErr)
	assert.Equal(t, int32(0), atomic.LoadInt32(&llm.generateCalls),
		"链的第一行没落库就不该调 LLM")
}

// ── ② 指纹覆盖同一行的源码 ──────────────────────────────────────────

// TestQT32A_TraceCarriesPromptSourceAndSelfConsistentFingerprint：一次编译成功的
// 生成，最后一行必须带 prompt / 源码 / 指纹，且指纹就是那一行源码的 sha256。
// 反证腿：把源码改一个字节，指纹必须变 —— 否则「指纹」只是个常量装饰。
func TestQT32A_TraceCarriesPromptSourceAndSelfConsistentFingerprint(t *testing.T) {
	source := "package x\nfunc f() {}\n"
	exec := &stubBuildExecutor{}
	sink := &recordingSink{}
	svc := buildServiceWithChain(
		&stubLLMClient{configured: true, generateResponse: source},
		&stubCodeChecker{}, // 放行
		exec,               // 编译成功
		sink,
	)

	res := svc.Generate(context.Background(), GenerateParams{
		Description: "动量策略，30 日窗口",
		Universe:    "csi300",
		StartDate:   "2025-01-01",
		EndDate:     "2025-12-31",
	})
	waitTerminal(t, svc, res.JobID, 2*time.Second)

	final := sink.last(t)
	assert.Equal(t, res.JobID, final.JobID, "链必须落在自己的 job 上")
	assert.Equal(t, "动量策略，30 日窗口", final.Prompt, "prompt 是链的第一环，不能只留个 job id")
	assert.Equal(t, "csi300", final.Universe, "人要求的回测区间也是链上的一环")
	assert.Equal(t, "2025-01-01", final.StartDate)
	assert.Equal(t, "2025-12-31", final.EndDate)
	assert.Equal(t, BuildStatePassed, final.BuildState, "编译成功要如实记成 passed")

	// 链上那份源码，必须**就是**送进编译器的那份字节 —— 拿执行器记下的实际
	// 文件内容对拍，不在断言里重写一遍去围栏 / TrimSpace 的逻辑（那样测的是
	// 我对实现的理解，不是实现本身）。
	require.NotEmpty(t, exec.compiled(), "执行器没读到源文件，这条对拍就是空转")
	assert.Equal(t, exec.compiled(), final.Source, "链上存的源码要与编译时磁盘上的字节一字不差")
	assert.NotEmpty(t, final.Source, "编译成功的那一行必须带源码")
	assert.Equal(t, shaOf(final.Source), final.SourceSha256,
		"指纹必须覆盖同一行存的源码（自证），got %q want sha256(source)=%s",
		final.SourceSha256, shaOf(final.Source))
	assert.Len(t, final.SourceSha256, 64, "sha256 hex 是 64 字符，短了就是写错了东西")

	// 反证腿：同 prompt 换一份源码 → 另一个指纹。
	// 这条腿防的是「指纹写成常量 / 拿 prompt 算 hash」那类假实现。
	different := source + "\nvar extra = 1\n"
	other := shaOf(different)
	assert.NotEqual(t, final.SourceSha256, other,
		"改一个字节的源码必须得到另一个指纹，否则指纹没有识别力")

	// 第一行在 LLM 之前就得存在，且只有意图、没有源码。
	first := sink.all()[0]
	assert.Equal(t, "动量策略，30 日窗口", first.Prompt)
	assert.Empty(t, first.Source, "第一行写于出码之前，那时还没有源码可存")
	assert.Equal(t, BuildStateNone, first.BuildState)
}

// ── ③ 编译过了也要过闸门 ────────────────────────────────────────────

// TestQT32A_PromoteGateBlocksEvenAfterSuccessfulBuild：沙箱编译成功**不**构成
// 「可以拿去回测」。链必须停在 promote_gate，并留下人能读懂的原因。
//
// 这条用例是 QT-32 ⑤「未过 promote 的 AI 产物被回测路径拒绝」在本仓的那一半。
// 执行侧那一半（strategy_ref 解析 + 生产回测）在 core + 资产服务，本仓替不了它作证。
func TestQT32A_PromoteGateBlocksEvenAfterSuccessfulBuild(t *testing.T) {
	sink := &recordingSink{}
	svc := buildServiceWithChain(
		&stubLLMClient{configured: true, generateResponse: "package x\nfunc f() {}\n"},
		&stubCodeChecker{},
		&stubBuildExecutor{}, // err == nil ⇒ 编译成功
		sink,
	)

	res := svc.Generate(context.Background(), GenerateParams{Description: "momentum"})
	waitTerminal(t, svc, res.JobID, 2*time.Second)

	job := svc.GetJob(res.JobID)
	job.Lock()
	status, buildErr, strategyName := job.Status, job.BuildErr, job.StrategyName
	job.Unlock()

	final := sink.last(t)
	assert.Equal(t, BuildStatePassed, final.BuildState, "闸门是在编译成功之后拦的，编译结论要如实留着")
	assert.Equal(t, BlockedPromoteGate, final.BlockedReason,
		"链必须点名是哪道闸停下的，got %q", final.BlockedReason)
	// 两条轴不许混进一列：被拦不是编译错误。读库的人要能一眼分开
	// 「编不过」与「编过了但不许跑」—— 后者才是需要人去 promote 的那种。
	assert.Empty(t, final.BuildError,
		"闸门拦下不得占用链上的 build_error（那条轴属于编译 / 写盘错误）")

	assert.Equal(t, "sandbox_rejected", status,
		"ui 侧终态白名单里的拒绝态；不能为了好看留一个 generated")
	assert.Contains(t, buildErr, "不进回测", "给调用方的话要点名「没去跑回测」这件事：got %q", buildErr)
	assert.Contains(t, buildErr, "promote", "同一句话也要说清是谁的闸门：got %q", buildErr)
	assert.Empty(t, strategyName,
		"不得再凭空造一个策略名 —— 旧实现拿硬编码字面量去 core 跑回测，等于宣称跑的就是这一份源码")

	// 计数器的账也要对上：编译成功计入 buildable，backtested 结构上为 0。
	_, buildable, backtested := svc.Stats()
	assert.EqualValues(t, 1, buildable, "编译过了就是过了，闸门不改变这一事实")
	assert.EqualValues(t, 0, backtested, "闸门拦下后不可能有回测；这条断言同时钉住「没有偷偷跑」")
}

// TestQT32A_BuildFailureIsNotReportedAsGateBlock：编译失败时的结论是 failed，
// 不该被闸门冒充 —— 两种「没跑成」在人眼里是完全不同的下一步。
func TestQT32A_BuildFailureIsNotReportedAsGateBlock(t *testing.T) {
	sink := &recordingSink{}
	// LLM 标成未配置：出码照样出（run() 无条件调 GenerateStrategyCode），但
	// 编译失败后的修码分支要求 IsConfigured() 为真才走 —— 这里要把「一次
	// 编译失败」钉死，不让 LLM 修码把结论改成第二次尝试。
	//
	// 失败原因走 stderr：真 `go build` 的诊断就在 stderr，run() 记录的也是它，
	// error 对象只表示「非零退出」。
	exec := &stubBuildExecutor{
		err:    errors.New("exit status 2"),
		stderr: bytes.NewBufferString("strategy_v0.go:2:1: syntax error: unexpected EOF\n"),
	}
	svc := buildServiceWithChain(
		&stubLLMClient{configured: false, generateResponse: "package x\nfunc f() {}\n"},
		&stubCodeChecker{},
		exec,
		sink,
	)

	res := svc.Generate(context.Background(), GenerateParams{Description: "momentum"})
	waitTerminal(t, svc, res.JobID, 2*time.Second)

	final := sink.last(t)
	assert.Equal(t, BuildStateFailed, final.BuildState)
	assert.Empty(t, final.BlockedReason,
		"编译失败不是闸门拦的，blocked_reason 必须留空，别把两种原因混成一个：got %q", final.BlockedReason)
	assert.Contains(t, final.BuildError, "syntax error")
}

// ── ④ 假路径不许复活（源码级守卫）────────────────────────────────────

// TestQT32A_NoFabricatedStrategyPathInProductionSource 是本单真正的护栏。
//
// runner 参数已经从签名里删掉，所以「断言 runner 被调用 0 次」在进程内**不可
// 表达**了 —— 那条断言换成读源码：本包的生产代码里不许再出现
//   - plugin.Open（加载产物：违反 ADR-024 与本仓 api-schema §5）
//   - "generated-strategy" / "copilot-generated"（凭空造的策略名）
//   - RunBacktest(（从本包发起回测）
//
// 形状同 pkg/ai/gene_pool/sql_scope_test.go：解析非测试 .go 的源码文本，
// 不看注释。删除式重构最容易「将来顺手加回来」，这条就是加回来当场红。
func TestQT32A_NoFabricatedStrategyPathInProductionSource(t *testing.T) {
	forbidden := []string{
		"plugin.Open",
		`"generated-strategy"`,
		`"copilot-generated"`,
		"RunBacktest(",
	}

	entries, err := os.ReadDir(".")
	require.NoError(t, err, "读本包目录失败")

	var checked []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		checked = append(checked, name)
		raw, err := os.ReadFile(filepath.Join(".", name))
		require.NoError(t, err, "读 %s 失败", name)
		lines := strings.Split(string(raw), "\n")
		for i, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "//") {
				continue // 注释里提到这些词是说明历史，不是实现
			}
			for _, bad := range forbidden {
				assert.NotContains(t, trimmed, bad,
					"%s:%d 出现了被 QT-32-A 移除的形状 %q —— AI 产物要先过 promote 人工闸门，"+
						"不能拿造出来的策略名去跑回测", name, i+1, bad)
			}
		}
	}
	require.NotEmpty(t, checked, "一条源码都没查，这条守卫就是空转")
	sort.Strings(checked)
	t.Logf("已核对 %d 个生产文件：%s", len(checked), strings.Join(checked, ", "))
}

// TestQT32A_StateVocabularyIsClosed 钉住取值本身闭合：三个编译状态 + 一个拦截
// 原因，都不是空串、都不重复。DDL CHECK 与 aistore 常量的对应关系在
// pkg/aistore/generation_chain_sql_test.go 与 cmd/ai/chain_sink_test.go 那两侧核。
func TestQT32A_StateVocabularyIsClosed(t *testing.T) {
	states := []string{BuildStateNone, BuildStateFailed, BuildStatePassed}
	seen := map[string]bool{}
	for _, v := range states {
		assert.NotEmpty(t, v, "状态取值不许是空串 —— 空串在库里是「没写」不是「这一环」")
		require.False(t, seen[v], "取值 %q 重复了：三个常量本该是三个不同的状态", v)
		seen[v] = true
	}
	assert.Equal(t, "promote_gate", BlockedPromoteGate)
}

// sequencedExecutor 按调用次序给结果：第一次编译失败，之后都成功。
// 用来走通「LLM 修码 → 再编一次」这条重试路径。
type sequencedExecutor struct {
	calls       int32
	firstStderr string

	mu          sync.Mutex
	seenSources []string
}

func (e *sequencedExecutor) Run(_ context.Context, _ string, args []string, _ string) (*bytes.Buffer, *bytes.Buffer, error) {
	var src string
	for _, a := range args {
		if strings.HasSuffix(a, ".go") {
			if raw, err := os.ReadFile(a); err == nil {
				src = string(raw)
			}
		}
	}
	e.mu.Lock()
	e.seenSources = append(e.seenSources, src)
	e.mu.Unlock()

	if atomic.AddInt32(&e.calls, 1) == 1 {
		return &bytes.Buffer{}, bytes.NewBufferString(e.firstStderr), errors.New("exit status 1")
	}
	return &bytes.Buffer{}, &bytes.Buffer{}, nil
}

func (e *sequencedExecutor) IsTimeout(error) bool { return false }

func (e *sequencedExecutor) sources() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]string, len(e.seenSources))
	copy(out, e.seenSources)
	return out
}

// TestQT32A_RetryKeepsTheFailedAttemptRow：修码重试**不能**把前一次的失败结论
// 抹掉。链上必须同时留着 attempt 0 的 failed（带当时的诊断）与 attempt 1 的
// passed，两行的源码与指纹各不相同 —— 否则「第一次为什么没成」这条生成质量
// 的证据就只剩最后一次。
func TestQT32A_RetryKeepsTheFailedAttemptRow(t *testing.T) {
	llm := &stubLLMClient{
		configured:       true,
		generateResponse: "package x\nfunc f() { broken() }\n",
		fixResponse:      "package x\nfunc f() {}\n",
	}
	exec := &sequencedExecutor{firstStderr: "strategy_v0.go:2:11: undefined: broken\n"}
	sink := &recordingSink{}
	svc := buildServiceWithChain(llm, &stubCodeChecker{}, exec, sink)

	res := svc.Generate(context.Background(), GenerateParams{Description: "momentum"})
	waitTerminal(t, svc, res.JobID, 2*time.Second)

	entries := sink.all()
	require.NotEmpty(t, entries)

	// 按 (attempt, state) 找：attempt 0 必须留下一行 failed。
	var sawFailed0, sawPassed1 bool
	var failed0 ChainEntry
	for _, e := range entries {
		if e.Attempt == 0 && e.BuildState == BuildStateFailed {
			sawFailed0 = true
			failed0 = e
		}
		if e.Attempt == 1 && e.BuildState == BuildStatePassed {
			sawPassed1 = true
		}
	}
	assert.True(t, sawFailed0, "第 0 次尝试的失败结论必须留在链上，got states=%v", statesOf(entries))
	assert.True(t, sawPassed1, "第 1 次尝试的编译成功必须留在链上，got states=%v", statesOf(entries))
	assert.Contains(t, failed0.BuildError, "undefined: broken",
		"那一行还要带上当时的诊断，否则「为什么失败」又只能靠猜")

	// 两次尝试编的是两份不同的字节（执行器实际读到的文件内容为准）。
	require.Len(t, exec.sources(), 2, "执行器应被调用两次")
	assert.NotEqual(t, exec.sources()[0], exec.sources()[1], "修码后的源码必须真的换了")
	final := entries[len(entries)-1]
	assert.Equal(t, 1, final.Attempt, "链的最后一行属于最后一次尝试")
	assert.Equal(t, BlockedPromoteGate, final.BlockedReason, "重试成功也照样过闸门")
}

func statesOf(entries []ChainEntry) []string {
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, fmt.Sprintf("%d/%s", e.Attempt, e.BuildState))
	}
	return out
}
