package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/ruoxizhnya/tellurion-contracts/domain"
	"github.com/ruoxizhnya/tellurion-kit/httpserver"
	aicausal "tellurion-agent/pkg/ai/causal"
	"tellurion-agent/pkg/ai/client"
	"tellurion-agent/pkg/ai/loop"
	"tellurion-agent/pkg/ai/pipeline"
	"tellurion-agent/pkg/ai/search"
	"tellurion-agent/pkg/ai/validation"
	"tellurion-agent/pkg/aistore"
)

// experimentReaper 是启动期收尸的能力面。*aistore.Store 满足它。
type experimentReaper interface {
	ReapRunningExperimentsBefore(ctx context.Context, before time.Time, reason string) (int64, error)
}

// reapExploreLeavings 在启动期收尸：把本进程启动前遗留的 running 实验行标记为
// failed/orphan（AGENT-EXPLORE-STATE-REAP / D10=b）。
//
// 收尸失败只记日志、不阻断启动 —— 它修的是观测面，不该反过来让服务起不来。
// 返回回收行数，供日志与测试口径。
func reapExploreLeavings(ctx context.Context, store experimentReaper, before time.Time, logger zerolog.Logger) int64 {
	if store == nil {
		return 0
	}
	n, err := store.ReapRunningExperimentsBefore(ctx, before, aistore.ExperimentOrphanReason)
	if err != nil {
		logger.Warn().Err(err).Msg("explore 启动收尸失败：遗留 running 实验保持原状")
		return 0
	}
	if n > 0 {
		logger.Warn().Int64("reaped", n).Msg("explore 启动收尸：遗留 running 实验已标记 failed/orphan")
	}
	return n
}

// verdictSink 把验证器裁决写回实验日志那一行（P2-9 接线）。
//
// 单开一个接口而不塞进 pipeline.ExperimentSink：裁决不是 pipeline 产出的，
// 它是循环控制器在每次尝试**之后**跑的。让 pipeline 去认识验证器，等于把
// 编排层的活塞进能力层 —— 而 pipeline 连尝试失败都要老实记下来，不该再
// 多背一个「什么时候算裁决」的判断。
type verdictSink interface {
	RecordVerdict(ctx context.Context, experimentID int64, verdict json.RawMessage) error
}

// exploreBias 是偏差维（P2-9d）的输入，来自**当前底座的实测状态**而不是
// 猜的。改这两行之前先看 pkg/ai/validation/bias.go 顶部的接线状态注释。
//
// 留空的字段是**真的没查过**，不是"没问题" —— 未评估的子维度不进概率，
// 只给 note。谎称干净比承认没查更危险。
var exploreBias = validation.BiasInput{
	// 行情落在 ohlcv_daily_qfq：前复权。它带一点前视成分（最新价会
	// 反推历史价），所以这一子维度拿 0.85 而不是满分。
	PriceAdjustment: validation.AdjustPre,
	// 池子来源：同步股票列表默认 list_status="L"（当前上市，见
	// cmd/data/sync_handlers.go:90），默认回测池也是手挑的一批**现存**
	// 蓝筹（pkg/ai/agents/validate.go 的 defaultAShareStockPool）。
	// 两者都没有退市票 —— 幸存者偏差是真的存在，不是理论风险。
	// 填了它，每一条裁决都会带上这条 blocking 质疑：这正是 P2-4 那笔债
	// 该有的样子（看得见的债才是债，藏起来的债会变成错误的自信）。
	PoolSource: validation.PoolSourceCurrent,
	// PITVerified / DataLagKnown 留空：只有 equitydeep 链路强制 ann_date，
	// 探索这条链路没查过。于是前视子维度是「未评估」并给 note。
}

// ExploreHandler 是 P1-2 循环控制器的 HTTP 入口。
//
// 控制器本身只是个库，没有这一层它就只是躺在包里的能力 —— 跟 P1-1b 的 sink
// 同一个教训：**加能力不等于接上了线**。
//
// 它同时是 P1-3 观察页的数据源：进度靠 OnAttempt 回调累积，叫停走 Stop。
type ExploreHandler struct {
	// pipeline 用接口而非具体类型：控制器只要求它能 Execute 一次尝试，
	// 这样测试才能注入一个不解析意图、不跑编译校验的替身。
	tryRunner loop.TryRunner
	runner    pipeline.BacktestRunner
	// verdicts 是裁决的落点（P2-9 接线）。nil = 只跑裁决不落库 ——
	// 观测设施不能反过来成为链路的硬依赖。
	verdicts verdictSink
	// narrator 是因果维（P2-9f）的叙述者。nil = 因果维未评估 —— 没讲就是
	// 没讲，不拿剩下五维凑一个"看起来完整"的结论。
	narrator validation.Narrator
	// counter 是一轮已落库尝试计数的来源（实验计数持久化）。nil = 只用内存
	// seq；生产装配从 sink 上探得 *aistore.Store 后必须接上。
	counter loop.TrialCounter
	// marks 是 G4 标记的落点。nil = 标记写入返回 503，而不是假装记下了。
	marks exploreMarkSink
	// pending 是顶栏「待审阅 N」的取数面。nil = 读面 503，由 core 聚合层
	// 编码成 unavailable（UI 显示「—」），不给一个看起来正常的 0。
	pending reviewPendingSource

	mu   sync.Mutex
	runs map[string]*exploreRun
}

// reviewPendingSource 是「待审阅 N」的取数面。*aistore.Store 满足它。
type reviewPendingSource interface {
	CountPendingReviews(ctx context.Context) (int, error)
}

// exploreMarkSink 是 G4 标记的写入面。*aistore.Store 满足它。
type exploreMarkSink interface {
	InsertExploreMark(ctx context.Context, m aistore.ExploreMark) (aistore.ExploreMark, error)
}

// biasInput 组装这一轮的偏差维输入（P2-4）。
//
// 上市窗口今天没有任何服务提供：core 的 QT-67 已随 SVC-CORE-DATA-RETIRE 退役，
// data 也还没有 point-in-time 端点。旧实现会去调一条永远 404 的端点再退回同一个
// 常量 —— 那只是把「没有」伪装成「查过了」。所以这里照实报 current：债还在，
// 藏起来它会变成错误的自信。重开条件见 tellurion-data 仓 TASKS.md 的 ⏸ 条目。
func (h *ExploreHandler) biasInput() validation.BiasInput {
	return exploreBias
}

// bearerTokenFromHeader 取 `Authorization: Bearer <jwt>` 里的 token，
// 供转发给 core 的 D24 只读动作使用。缺失/形状不对一律返回空串，
// 由客户端 fail-closed 拒绝，不降级成匿名调用。
func bearerTokenFromHeader(header string) string {
	const prefix = "Bearer "
	if len(header) <= len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
		return ""
	}
	return strings.TrimSpace(header[len(prefix):])
}

// exploreRun 是一轮进行中（或已结束）的探索。
type exploreRun struct {
	mu       sync.Mutex
	ctrl     *loop.Controller
	cancel   context.CancelFunc
	started  time.Time
	attempts []loop.Attempt
	result   *loop.RunResult
	err      error
	done     bool
	// seed / proposer 是这一轮的可复现坐标（DBT-18）：带 seed 重放能得到
	// 同一串建议与同一串 ai.experiments 行。它们同时写进每次尝试的 Hypothesis，
	// 所以「这一轮用的哪个 seed、哪个提议器」有库里的行可查。
	seed     int64
	proposer string
}

// StartExploreRequest 启动一轮探索的请求。
type StartExploreRequest struct {
	Description string `json:"description" binding:"required" example:"做一个动量策略"`
	MaxTries    int    `json:"max_tries" example:"50"`
	// LookbackMin / LookbackMax 限定回看窗口的搜索范围；不传则用 5..250。
	LookbackMin int `json:"lookback_min"`
	LookbackMax int `json:"lookback_max"`
	// Seed 是提议器的随机种子（DBT-18）。不传时退回墙上时钟，
	// 但实际使用的值会出现在启动响应、状态响应和日志里 —— 不静默。
	Seed *int64 `json:"seed"`
	// Proposer 选择候选提议器：tpe（默认）或 mcts。两者实现同一个
	// loop.Proposer 接口，切换不改编排层。
	Proposer string `json:"proposer" example:"tpe"`
}

// AttemptView 是一次尝试的对外视图。
type AttemptView struct {
	Seq          int            `json:"seq"`
	Hypothesis   string         `json:"hypothesis"`
	Params       map[string]any `json:"params"`
	ExperimentID int64          `json:"experiment_id"`
	// NumTrials 是统计维实际使用的累计尝试数（含持久化基数）。重启后 Seq
	// 会归零，这个数字不会 —— 观察页靠它判断多重检验校正的分母。
	NumTrials int     `json:"num_trials"`
	Sharpe    float64 `json:"sharpe"`
	OK        bool    `json:"ok"`
	Error     string  `json:"error,omitempty"`

	// Verdict 是验证器链对这次尝试的裁决（P2-9）。失败的尝试为 null ——
	// 没有回测结果就没有可被证伪的东西，不给一个看起来正常的裁决。
	Verdict *validation.Verdict `json:"verdict,omitempty"`
}

// ExploreStatusResponse 是一轮探索的实时状态。
type ExploreStatusResponse struct {
	RunID    string        `json:"run_id"`
	Running  bool          `json:"running"`
	Stopped  string        `json:"stopped,omitempty"`
	Attempts []AttemptView `json:"attempts"`
	BestSeq  *int          `json:"best_seq,omitempty"`
	// Seed / Proposer 是这一轮的可复现坐标（DBT-18）。
	Seed     int64  `json:"seed"`
	Proposer string `json:"proposer"`
}

// NewExploreHandler 构造探索处理器。
func NewExploreHandler(p loop.TryRunner, runner pipeline.BacktestRunner) *ExploreHandler {
	return &ExploreHandler{
		tryRunner: p,
		runner:    runner,
		runs:      make(map[string]*exploreRun),
	}
}

// RegisterExploreRoutes 注册探索路由。
func (h *ExploreHandler) RegisterExploreRoutes(router *gin.RouterGroup) {
	g := router.Group("/explore")
	{
		g.POST("/runs", h.Start)
		g.GET("/runs/:runID", h.Status)
		g.GET("/runs/:runID/stream", h.Stream)
		g.POST("/runs/:runID/stop", h.Stop)
		// 三条人工干预写面（B-06 G2/G3/G4）：指方向、追问、标记。
		// 三处上下文入口（候选卡 / 被放弃的尝试 / 结果流）共用同一组坐标：
		// run_id + attempt_seq。
		g.POST("/runs/:runID/steer", h.Steer)
		g.POST("/runs/:runID/attempts/:seq/ask", h.AskAttempt)
		g.POST("/runs/:runID/attempts/:seq/mark", h.MarkAttempt)
		// 顶栏「待审阅 N」的上游读面（core /api/review/pending 的 ai_explore 源）。
		g.GET("/review/pending", h.ReviewPending)
	}
}

// registerExploreRoutes 注册探索路由。
//
// 探索用的 pipeline 实例**必须带上实验日志落点** —— 否则这一路的尝试一行
// 都不会记，而「试了多少次」正是过拟合检测唯一依赖的数字。
func registerExploreRoutes(router *gin.Engine, runner pipeline.BacktestRunner, sink pipeline.ExperimentSink) {
	var opts []pipeline.PipelineOption
	if sink != nil {
		opts = append(opts, pipeline.WithExperimentSink(sink))
	}
	handler := NewExploreHandler(buildPipeline(sink, opts...), runner)
	// 实验日志落点若同时能写裁决（*aistore.Store 满足），就把裁决
	// 也接上；不满足就只记日志不记裁决。main.go 里 aiStore 在初始化失败时
	// 进程已退出，sink 只可能是非 nil 的 *aistore.Store。
	if vs, ok := sink.(verdictSink); ok {
		handler.verdicts = vs
	}
	// 实验计数同样从 sink 上探：*aistore.Store 既能写日志也能按 run 计数。
	if tc, ok := sink.(loop.TrialCounter); ok {
		handler.counter = tc
	}
	// G4 标记同样同源：接不上就 503，不假装记下了。
	if ms, ok := sink.(exploreMarkSink); ok {
		handler.marks = ms
	}
	// 待审阅计数同样从 sink 上探；接不上就让读面 503，由 core 报 unavailable。
	if ps, ok := sink.(reviewPendingSource); ok {
		handler.pending = ps
	}
	// 因果维要真调一次模型：没配 key 就不配叙述者，那一维老老实实记成
	// 未评估。配了却调不通也一样 —— ValidateCausal 返回的错误只记日志，
	// 不会把这一轮探索搞挂。
	if c := budgetedLLMClient(sink); c.IsConfigured() {
		handler.narrator = aicausal.New(c)
	}
	api := router.Group("/api")
	handler.RegisterExploreRoutes(api)
}

// Start 启动一轮探索，立即返回 run_id —— 一轮可能跑几十上百次尝试，
// 不能让 HTTP 请求挂在那儿等。
func (h *ExploreHandler) Start(c *gin.Context) {
	var req StartExploreRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpserver.Error(c, http.StatusBadRequest, err)
		return
	}
	if req.MaxTries <= 0 {
		req.MaxTries = 20
	}
	lbMin, lbMax := req.LookbackMin, req.LookbackMax
	if lbMin <= 0 {
		lbMin = 5
	}
	if lbMax <= lbMin {
		lbMax = 250
	}

	space := &search.SearchSpace{Params: []search.ParamDef{
		// 参数名必须与意图参数同名，否则覆盖静默失效（见 P1-2b）。
		{Name: "lookback_days", Type: "int", Min: float64(lbMin), Max: float64(lbMax)},
	}}

	// 显式 seed 优先；没给才退回墙上时钟。无论走哪条，实际值都会写进
	// 启动响应、状态响应、日志，以及每次尝试的 Hypothesis（随
	// ai.experiments 落库）—— 可复现的前提是「用的哪个 seed」可查。
	seed := time.Now().UnixNano()
	if req.Seed != nil {
		seed = *req.Seed
	}
	proposerName := strings.ToLower(strings.TrimSpace(req.Proposer))
	if proposerName == "" {
		proposerName = "tpe"
	}
	var proposer loop.Proposer
	switch proposerName {
	case "tpe":
		proposer = loop.NewTPEProposer(space, seed)
	case "mcts":
		proposer = loop.NewMCTSProposer(space, seed)
	default:
		httpserver.Error(c, http.StatusBadRequest,
			fmt.Errorf("unknown proposer %q (want \"tpe\" or \"mcts\")", req.Proposer))
		return
	}

	// ⚠️ 刻意不用 c.Request.Context()：响应一返回 gin 就会取消它，异步跑的
	// 这一轮会在第一步就被掐断。这里要的是独立于请求的生命周期。
	// 凭证在返回前取出：gin 的 context 在 handler 返回后会被复用，不能带进 goroutine。
	bearerToken := bearerTokenFromHeader(c.GetHeader("Authorization"))
	ctx, cancel := context.WithCancel(context.Background())
	// 回测改走 HTTP 后，每一轮尝试都要带上调用者的凭证（core 的
	// /api/backtest 至少 trader）。值显式放回独立 ctx —— 不能带 gin 的
	// 请求 ctx（响应一返回就被取消）。
	ctx = client.WithBearerTokenInContext(ctx, bearerToken)

	runID := uuid.New().String()
	ctrl := loop.NewController(h.tryRunner, proposer, h.runner)
	if h.counter != nil {
		ctrl = ctrl.WithTrialCounter(h.counter)
	}
	run := &exploreRun{
		ctrl:     ctrl,
		cancel:   cancel,
		started:  time.Now(),
		seed:     seed,
		proposer: proposerName,
	}
	log.Printf("explore run %s: proposer=%s seed=%d", runID, proposerName, seed)
	h.mu.Lock()
	h.runs[runID] = run
	h.mu.Unlock()

	go func() {
		defer func() {
			run.mu.Lock()
			run.done = true
			run.mu.Unlock()
		}()
		out, err := run.ctrl.Run(ctx, loop.Config{
			RunID:       runID,
			Description: req.Description,
			MaxTries:    req.MaxTries,
			Bias:        h.biasInput(),
			OnAttempt: func(a loop.Attempt) {
				run.mu.Lock()
				run.attempts = append(run.attempts, a)
				run.mu.Unlock()
				// 锁外写库：裁决落盘不该挡住下一次尝试。
				h.recordVerdict(runID, a)
			},
		})
		run.mu.Lock()
		run.result, run.err = out, err
		run.mu.Unlock()

		// 因果维只对最终候选做一次（P2-9f）。
		//
		// 它是六维里唯一要花一次模型调用的一维：每次尝试都做，一轮几十次
		// 探索就是几十次调用，换来的却是几十份"讲给已被淘汰的候选听"的
		// 故事。真正需要人判断的只有最后那个候选 —— 所以放在这里。
		h.reviewBestCausally(ctx, run, runID, req.Description, out)
	}()

	c.JSON(http.StatusOK, gin.H{
		"run_id": runID, "max_tries": req.MaxTries,
		"seed": seed, "proposer": proposerName,
	})
}

// recordVerdict 把一次尝试的裁决写进实验日志那一行。
//
// 写失败只打日志，不中断探索：裁决是对已有结果的评论，评论丢了实验还在，
// 反过来（为了写评论把探索搞挂）才不可接受。
func (h *ExploreHandler) recordVerdict(runID string, a loop.Attempt) {
	if h.verdicts == nil || a.Verdict == nil || a.ExperimentID == 0 {
		return
	}
	b, err := json.Marshal(a.Verdict)
	if err != nil {
		log.Printf("explore run %s seq %d: 裁决序列化失败，未写入日志: %v", runID, a.Seq, err)
		return
	}
	// 刻意不用请求 ctx：它随响应一起取消，而这一轮是异步跑的。
	// 裁决是事后评论，给个短超时比让它无限挂住更合适。
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := h.verdicts.RecordVerdict(ctx, a.ExperimentID, b); err != nil {
		log.Printf("explore run %s seq %d: 裁决未写入实验日志: %v", runID, a.Seq, err)
	}
}

// reviewBestCausally 给本轮最优的候选补上因果维。
//
// 叙述失败（模型没配、调用超时、返回不是 JSON）时保持原样 —— 那一维仍旧是
// 「未评估」，而不是悄悄记成通过。审查是附加价值，不该有能力毁掉一轮探索。
func (h *ExploreHandler) reviewBestCausally(
	ctx context.Context, run *exploreRun, runID, description string, out *loop.RunResult,
) {
	if h.narrator == nil || out == nil || out.Best == nil || out.Best.Verdict == nil {
		return
	}
	var br *domain.BacktestResult
	if out.Best.Result != nil {
		br = out.Best.Result.BacktestResult
	}
	periods := 0
	if br != nil {
		periods = len(br.PortfolioValues)
	}

	c, err := validation.ValidateCausal(ctx, validation.CausalRequest{
		Name:       description,
		Hypothesis: out.Best.Hypothesis,
		Params:     out.Best.Params,
		Periods:    periods,
		Result:     br,
	}, h.narrator)
	if err != nil {
		log.Printf("explore run %s: 因果维未评估（%v）", runID, err)
		return
	}
	if c == nil {
		return
	}

	// 裁决是通过指针共享给 run.attempts 里那一份的，所以改之前必须持锁 ——
	// 观察页正在另一边 poll 同一个对象。
	run.mu.Lock()
	validation.AttachCausal(out.Best.Verdict, c)
	run.mu.Unlock()

	// 库里那份也要跟着更新，否则日志里的裁决永远缺因果维。
	h.recordVerdict(runID, *out.Best)
}

// Status 查一轮探索的实时进度（轮询端点）。
func (h *ExploreHandler) Status(c *gin.Context) {
	runID := c.Param("runID")
	h.mu.Lock()
	run, ok := h.runs[runID]
	h.mu.Unlock()
	if !ok {
		httpserver.Fail(c, http.StatusNotFound, "run not found")
		return
	}
	c.JSON(http.StatusOK, h.statusOf(runID, run))
}

// statusOf 持 run.mu 取一份当前快照。轮询端点与 SSE 共用它，保证两条路推出去
// 的 JSON 一字不差 —— 前端 EventSource 和 poll 能复用同一套类型。
func (h *ExploreHandler) statusOf(runID string, run *exploreRun) ExploreStatusResponse {
	run.mu.Lock()
	defer run.mu.Unlock()

	views := make([]AttemptView, 0, len(run.attempts))
	for _, a := range run.attempts {
		v := AttemptView{
			Seq:          a.Seq,
			Hypothesis:   a.Hypothesis,
			Params:       a.Params,
			ExperimentID: a.ExperimentID,
			NumTrials:    a.NumTrials,
			OK:           a.OK,
			Verdict:      a.Verdict,
		}
		if a.Result != nil && a.Result.BacktestResult != nil {
			v.Sharpe = a.Result.BacktestResult.SharpeRatio
		}
		if a.Err != nil {
			v.Error = a.Err.Error()
		}
		views = append(views, v)
	}

	resp := ExploreStatusResponse{
		RunID:    runID,
		Running:  !run.done,
		Attempts: views,
		Seed:     run.seed,
		Proposer: run.proposer,
	}
	if run.result != nil {
		resp.Stopped = string(run.result.Stopped)
		if run.result.Best != nil {
			seq := run.result.Best.Seq
			resp.BestSeq = &seq
		}
	}
	return resp
}

// Stream 用 SSE 把一轮探索的实时状态推给观察页，替代每 2.5s 一次的轮询：
// 一条长连接只占限流器 1 次请求，从根上把 429 与「真失败」分开（OPS-01）。
//
// 形状复用 core 先例 data-service 的 sseProgressHandler —— 轮询内存快照转推，
// 不为省这几秒引入多订阅 fan-out。
//
// ⚠️ 客户端断线只是**停推**：这一轮用的是 Start 里 context.Background() 派生的
// 独立 cancel（见 handlers_explore.go Start 的注释），绝不因为连接断了就取消
// 探索 —— 叫停只走 POST /runs/:runID/stop。
func (h *ExploreHandler) Stream(c *gin.Context) {
	runID := c.Param("runID")
	h.mu.Lock()
	run, ok := h.runs[runID]
	h.mu.Unlock()
	if !ok {
		httpserver.Fail(c, http.StatusNotFound, "run not found")
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	// 节奏逻辑抽进 pumpStream，让「终止 / 断线」语义脱离网络计时可被确定性测试；
	// 这里只负责把 gin 的 SSE 写入接成 emit。
	pumpStream(c.Request.Context(), 2*time.Second,
		func() ExploreStatusResponse { return h.statusOf(runID, run) },
		func(event string, snap ExploreStatusResponse) {
			c.SSEvent(event, snap)
			c.Writer.Flush()
		})
}

// pumpStream 推「progress →（跑完时）complete」直到本轮结束或 ctx 取消。
//
// snapshot 每次取当下状态，emit 写一帧。⚠️ ctx 取消只是**停推**——本函数拿不到
// run 的 cancel，也就无从取消探索，这正是「连接生命周期 ≠ run 生命周期」的结构
// 保证（叫停只走 POST /runs/:runID/stop）。断线由 ctx 感知，无需对写失败另设分支。
//
// 首帧若已跑完就立刻 progress + complete 返回，不留一个空转到超时的连接。
func pumpStream(ctx context.Context, interval time.Duration,
	snapshot func() ExploreStatusResponse,
	emit func(event string, snap ExploreStatusResponse),
) {
	snap := snapshot()
	emit("progress", snap)
	if !snap.Running {
		emit("complete", snap)
		return
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return // 断线只停推，不 cancel run
		case <-ticker.C:
			snap := snapshot()
			emit("progress", snap)
			if !snap.Running {
				emit("complete", snap)
				return
			}
		}
	}
}

// Stop 叫停一轮探索。已跑的不丢 —— 这正是「被叫停的探索」与「跑满的探索」
// 可信度不同的地方，日志里都留着。
func (h *ExploreHandler) Stop(c *gin.Context) {
	h.mu.Lock()
	run, ok := h.runs[c.Param("runID")]
	h.mu.Unlock()
	if !ok {
		httpserver.Fail(c, http.StatusNotFound, "run not found")
		return
	}

	run.ctrl.Stop()
	if run.cancel != nil {
		run.cancel()
	}
	c.JSON(http.StatusOK, gin.H{"run_id": c.Param("runID"), "stopping": true})
}

// ---------------------------------------------------------------------------
// B-06 人工干预三端点（G2 指方向 / G3 追问 / G4 标记）
//
// 三处 UI 上下文入口（候选卡、被放弃的尝试、结果流）共用同一组坐标：
// run_id（+ attempt_seq）。语义分开，形状一致，前端不必为每个入口记一套参数。
// ---------------------------------------------------------------------------

// SteerRequest 是 G2 指方向的请求体。
type SteerRequest struct {
	// Direction 是人给的约束文本（「先别扩参数空间，只试反转类」）。
	Direction string `json:"direction" binding:"required" example:"先别扩大参数空间，只试反转类"`
	// AttemptSeq 记录这次方向是从哪张候选卡指出来的；省略 = 卡外指方向。
	AttemptSeq *int `json:"attempt_seq"`
}

// Steer 是一次人类指方向（G2）。
//
// 只影响**后续**尝试：正在跑的那次不改。已结束的 run 返回 409 —— 方向没有
// 可以作用的下一次尝试，接受它就是制造一个永远不会生效的承诺。
func (h *ExploreHandler) Steer(c *gin.Context) {
	runID := c.Param("runID")
	run, ok := h.lookupRun(runID)
	if !ok {
		httpserver.Fail(c, http.StatusNotFound, "run not found")
		return
	}

	var req SteerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpserver.Error(c, http.StatusBadRequest, err)
		return
	}
	req.Direction = strings.TrimSpace(req.Direction)
	if req.Direction == "" {
		httpserver.Fail(c, http.StatusBadRequest, "direction is required")
		return
	}
	if len([]rune(req.Direction)) > 1000 {
		httpserver.Fail(c, http.StatusBadRequest, "direction is too long (max 1000 characters)")
		return
	}

	run.mu.Lock()
	done := run.done
	run.mu.Unlock()
	if done {
		httpserver.Fail(c, http.StatusConflict, "run already finished")
		return
	}

	n := run.ctrl.Steer(req.Direction, req.AttemptSeq)
	c.JSON(http.StatusOK, gin.H{
		"run_id":     runID,
		"accepted":   true,
		"count":      n,
		"directions": run.ctrl.Directions(),
		"applied_to": "next_attempt",
	})
}

// AttemptQuestionRequest 是 G3 追问的请求体。
type AttemptQuestionRequest struct {
	Question string `json:"question" binding:"required" example:"为什么放弃这个方向？"`
}

// AttemptEvidence 是回答所依据的证据快照 —— 追问不能是又一段无出处的故事。
type AttemptEvidence struct {
	Hypothesis   string         `json:"hypothesis"`
	Params       map[string]any `json:"params"`
	ExperimentID int64          `json:"experiment_id"`
	NumTrials    int            `json:"num_trials"`
	OK           bool           `json:"ok"`
	Error        string         `json:"error,omitempty"`
	Sharpe       float64        `json:"sharpe"`
	// VerdictProbability / Blocking / Challenges 是验证器裁决的摘录；
	// 没有裁决时整体缺席，而不是给一个 0 分裁决。
	VerdictProbability *float64 `json:"verdict_probability,omitempty"`
	Blocking           int      `json:"blocking"`
	Challenges         []string `json:"challenges"`
}

// AttemptAnswerResponse 是 G3 的回答。
type AttemptAnswerResponse struct {
	RunID    string `json:"run_id"`
	Seq      int    `json:"seq"`
	Question string `json:"question"`
	Answer   string `json:"answer"`
	// Source 说明答案从哪来。当前只有 evidence —— 用这次尝试的落库坐标、
	// 参数、错误与裁决拼出来，不额外调模型，因此可复现。
	Source   string          `json:"source"`
	Evidence AttemptEvidence `json:"evidence"`
}

// AskAttempt 回答「关于某次尝试的一个问题」（G3）。
//
// 追问挂在**被放弃的尝试**上：答案只从这次尝试自身的证据生成（错误、参数、
// 验证器质疑），不编造那一次从未跑过的结果。没有 LLM 也能给出确定答案。
func (h *ExploreHandler) AskAttempt(c *gin.Context) {
	runID := c.Param("runID")
	run, ok := h.lookupRun(runID)
	if !ok {
		httpserver.Fail(c, http.StatusNotFound, "run not found")
		return
	}
	seq, ok := parseAttemptSeq(c)
	if !ok {
		return
	}
	a, ok := findAttempt(run, seq)
	if !ok {
		httpserver.Fail(c, http.StatusNotFound, "attempt not found")
		return
	}

	var req AttemptQuestionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpserver.Error(c, http.StatusBadRequest, err)
		return
	}
	req.Question = strings.TrimSpace(req.Question)
	if req.Question == "" {
		httpserver.Fail(c, http.StatusBadRequest, "question is required")
		return
	}
	if len([]rune(req.Question)) > 500 {
		httpserver.Fail(c, http.StatusBadRequest, "question is too long (max 500 characters)")
		return
	}

	answer, evidence := explainAttempt(a)
	c.JSON(http.StatusOK, AttemptAnswerResponse{
		RunID:    runID,
		Seq:      seq,
		Question: req.Question,
		Answer:   answer,
		Source:   "evidence",
		Evidence: evidence,
	})
}

// explainAttempt 从一次尝试的证据拼出确定答案。
//
// 顺序刻意固定：先讲做了什么，再讲结果/失败，再讲裁决，最后讲当时搜的参数。
// 参数按键排序，保证同一输入永远给同一段文字（契约测试要能逐字断言）。
func explainAttempt(a loop.Attempt) (string, AttemptEvidence) {
	ev := AttemptEvidence{
		Hypothesis:   a.Hypothesis,
		Params:       a.Params,
		ExperimentID: a.ExperimentID,
		NumTrials:    a.NumTrials,
		OK:           a.OK,
		Challenges:   []string{},
	}
	if a.Result != nil && a.Result.BacktestResult != nil {
		ev.Sharpe = a.Result.BacktestResult.SharpeRatio
	}
	if a.Err != nil {
		ev.Error = a.Err.Error()
	}
	if a.Verdict != nil {
		p := a.Verdict.Probability
		ev.VerdictProbability = &p
		ev.Blocking = a.Verdict.Blocking
		for _, ch := range a.Verdict.Challenges {
			if ch.Severity == validation.SeverityBlocking {
				ev.Challenges = append(ev.Challenges, ch.Dimension+"："+ch.Message)
			}
		}
	}

	var b strings.Builder
	if ev.Hypothesis != "" {
		b.WriteString("这次尝试是在「" + ev.Hypothesis + "」上做的。")
	} else {
		b.WriteString("这次尝试没有记录假设。")
	}
	switch {
	case ev.Error != "":
		b.WriteString("它没有跑完：" + ev.Error + "。")
	case a.Result == nil || a.Result.BacktestResult == nil:
		b.WriteString("它没有产出可用的回测结果。")
	default:
		b.WriteString(fmt.Sprintf("它跑出了结果，Sharpe=%.2f。", ev.Sharpe))
	}
	if ev.VerdictProbability != nil {
		b.WriteString(fmt.Sprintf("验证器综合概率 %.0f%%，%d 条阻断质疑。",
			*ev.VerdictProbability*100, ev.Blocking))
		if len(ev.Challenges) > 0 {
			b.WriteString("阻断点：" + strings.Join(ev.Challenges, "；") + "。")
		}
	} else if ev.Error == "" {
		b.WriteString("这次尝试没有验证器裁决。")
	}
	if len(a.Params) > 0 {
		keys := make([]string, 0, len(a.Params))
		for k := range a.Params {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		parts := make([]string, 0, len(keys))
		for _, k := range keys {
			parts = append(parts, fmt.Sprintf("%s=%v", k, a.Params[k]))
		}
		b.WriteString("当时搜的参数：" + strings.Join(parts, "，") + "。")
	}
	if ev.NumTrials > 0 {
		b.WriteString(fmt.Sprintf("统计校正用的累计尝试数：%d。", ev.NumTrials))
	}
	return b.String(), ev
}

// MarkAttemptRequest 是 G4 标记的请求体。
type MarkAttemptRequest struct {
	// Kind 只有 derive（派生子 run）与 promote（提升权重）两种。
	Kind  string `json:"kind" binding:"required" enums:"derive,promote" example:"derive"`
	Note  string `json:"note" example:"这一路值得围绕它多试几组"`
	Actor string `json:"actor" example:"ruoxi"`
}

// MarkAttempt 给结果流上的一次尝试打轻量标注（G4）。
//
// 它不是裁决：不参与校准、不覆盖历史，同一坐标可以重复标记。写面要求 run
// 与 seq 都能在内存运行态里对上 —— 宁可 404，也不给一个不存在的坐标留标注。
func (h *ExploreHandler) MarkAttempt(c *gin.Context) {
	runID := c.Param("runID")
	if h.marks == nil {
		httpserver.Fail(c, http.StatusServiceUnavailable, "explore mark store not configured")
		return
	}
	run, ok := h.lookupRun(runID)
	if !ok {
		httpserver.Fail(c, http.StatusNotFound, "run not found")
		return
	}
	seq, ok := parseAttemptSeq(c)
	if !ok {
		return
	}
	a, ok := findAttempt(run, seq)
	if !ok {
		httpserver.Fail(c, http.StatusNotFound, "attempt not found")
		return
	}

	var req MarkAttemptRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpserver.Error(c, http.StatusBadRequest, err)
		return
	}
	kind := strings.ToLower(strings.TrimSpace(req.Kind))
	if kind != aistore.ExploreMarkDerive && kind != aistore.ExploreMarkPromote {
		httpserver.Fail(c, http.StatusBadRequest, "kind must be derive or promote")
		return
	}
	note := strings.TrimSpace(req.Note)
	if len([]rune(note)) > 2000 {
		httpserver.Fail(c, http.StatusBadRequest, "note is too long (max 2000 characters)")
		return
	}

	m, err := h.marks.InsertExploreMark(c.Request.Context(), aistore.ExploreMark{
		RunID:        runID,
		AttemptSeq:   seq,
		Kind:         kind,
		Note:         note,
		Actor:        strings.TrimSpace(req.Actor),
		ExperimentID: a.ExperimentID,
	})
	if err != nil {
		httpserver.Error(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusCreated, m)
}

// lookupRun 取内存运行态。
func (h *ExploreHandler) lookupRun(runID string) (*exploreRun, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	run, ok := h.runs[runID]
	return run, ok
}

// parseAttemptSeq 解析路径里的 attempt seq；非法时已写好 400，调用方直接返回。
func parseAttemptSeq(c *gin.Context) (int, bool) {
	seq, err := strconv.Atoi(c.Param("seq"))
	if err != nil || seq < 0 {
		httpserver.Fail(c, http.StatusBadRequest, "invalid attempt seq")
		return 0, false
	}
	return seq, true
}

// findAttempt 持 run.mu 复制一份尝试快照，避免与探索 goroutine 并发读写。
func findAttempt(run *exploreRun, seq int) (loop.Attempt, bool) {
	run.mu.Lock()
	defer run.mu.Unlock()
	for _, a := range run.attempts {
		if a.Seq == seq {
			return a, true
		}
	}
	return loop.Attempt{}, false
}

// ReviewPendingResponse 是顶栏「待审阅 N」的上游契约（core 的 ai_explore 源）。
type ReviewPendingResponse struct {
	Pending int    `json:"pending" example:"3"`
	AsOf    string `json:"as_of" example:"2026-11-12T09:00:00Z"`
}

// ReviewPending 返回待审阅条数。
//
// 口径见 aistore.CountPendingReviews：台账里 outcome_hit 为空的行 = 结果尚未
// 回填、等待人工复盘。0 是可信答案（真的没有待审）；取不到数时返回 5xx，
// 让 core 聚合层如实标成 unavailable —— 不在这里把「不知道」写成 0。
func (h *ExploreHandler) ReviewPending(c *gin.Context) {
	if h.pending == nil {
		httpserver.Fail(c, http.StatusServiceUnavailable, "review pending source not configured")
		return
	}
	n, err := h.pending.CountPendingReviews(c.Request.Context())
	if err != nil {
		httpserver.Error(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, ReviewPendingResponse{
		Pending: n,
		AsOf:    time.Now().UTC().Format(time.RFC3339),
	})
}
