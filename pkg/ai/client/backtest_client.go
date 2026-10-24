package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ruoxizhnya/tellurion-contracts/domain"
)

// ErrBacktestFailed 标记「core 判本次 run 失败」这一**语义**失败，与传输层错误区分开：
// 前者重试同一个请求不会变好（参数 / 数据 / 契约问题），后者才可能是瞬时的。
// 判据用 errors.Is(err, ErrBacktestFailed)，不要去比错误文案。
var ErrBacktestFailed = errors.New("backtest failed")

// BacktestClient is an HTTP client for the backtest API
type BacktestClient struct {
	baseURL    string
	httpClient *http.Client

	bearerToken           string
	tokenFromContext      bool
	allowInsecureLoopback bool
}

// bearerTokenContextKey 是本包在 context 里携带「入站请求的 JWT」的键。
//
// QT-17 S3-c-3：回测从进程内改为 HTTP 后，agent 需要以**调用者本人**的身份
// 打 core（POST /api/backtest 至少 trader）。token 由 HTTP handler 从入站
// 请求取出放进 ctx，客户端只负责转发 —— 不签发、不刷新、不落盘。
type bearerTokenContextKey struct{}

// WithBearerTokenInContext 把入站请求的 JWT 放进 ctx，供 BacktestClient
// （WithTokenFromContext）在发出请求时转发。
//
// 异步链路（copilot 的 goroutine、explore 的循环）必须显式把值带过去：
// context.Background() 会丢掉它，客户端随即 fail-closed，而不是静默降级成
// 匿名调用。
func WithBearerTokenInContext(ctx context.Context, token string) context.Context {
	token = strings.TrimSpace(token)
	if token == "" {
		return ctx
	}
	return context.WithValue(ctx, bearerTokenContextKey{}, token)
}

// BearerTokenFromContext 读出 WithBearerTokenInContext 放进来的 token。
func BearerTokenFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	token, _ := ctx.Value(bearerTokenContextKey{}).(string)
	return token
}

// Option configures a BacktestClient.
type Option func(*BacktestClient)

// WithBearerToken 让客户端以 `Authorization: Bearer <token>` 调 core（QT-50 ④ 的
// JWT 消费面）。token 由装配方持有 / 转发，本包不签发、不刷新。
func WithBearerToken(token string) Option {
	return func(c *BacktestClient) { c.bearerToken = strings.TrimSpace(token) }
}

// WithInsecureLoopback 显式开启「无凭证的本地开发」模式（REV-01-A ③-2）。
// 只对 loopback 地址生效：非 loopback 的 baseURL 即使开了这个开关也会被拒绝 ——
// 「本地开发」不该变成「对远端悄悄降级成匿名调用」。
func WithInsecureLoopback() Option {
	return func(c *BacktestClient) { c.allowInsecureLoopback = true }
}

// WithTokenFromContext 开启「按请求转发入站 JWT」（QT-17 S3-c-3）。
//
// 优先级：静态 token（WithBearerToken）> ctx 里的 token > loopback 开发模式。
// 三者都没有时仍然 fail-closed —— 不降级成匿名调用。
func WithTokenFromContext() Option {
	return func(c *BacktestClient) { c.tokenFromContext = true }
}

// NewBacktestClient creates a new backtest client.
//
// 默认 **fail-closed**：没有凭证（WithBearerToken）且没有显式开启 loopback 开发模式时，
// 请求在发出去之前就被拒绝，不静默降级成匿名调用。
func NewBacktestClient(baseURL string, opts ...Option) *BacktestClient {
	c := &BacktestClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			// 同步回测在 core 侧的预算是 10 分钟；walk-forward 是「N 个窗口
			// 串行 × 每窗一次回测」，30 分钟是它的上界而不是目标。调用方的
			// ctx 若更早结束，会先取消（这里只是兜底，防止挂死）。
			Timeout: 30 * time.Minute,
		},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// BacktestRequest 是 POST /api/backtest 的请求体。
//
// 字段名逐条对齐 core 的 `contracts.BacktestRequest`（`tellurion:docs/openapi.yaml`
// 的 BacktestRequest）：`strategy` 是 `binding:"required"` 的那个字段。
// **不要再出现 `strategy_name`** —— 那个名字不在契约里，core 会落到 400
// 「must provide strategy+stock_pool (old format) or strategy_id+universe」。
type BacktestRequest struct {
	Strategy  string   `json:"strategy"`
	StockPool []string `json:"stock_pool,omitempty"`
	// IndexCode 是指数票池（DATA-01）：`universe: csi300` 不是股票代码，
	// 放进 StockPool 会被当成一只叫 "csi300" 的股票。core 负责把人类写法
	// 翻译成 Tushare 代码并展开成分股。
	IndexCode string `json:"index_code,omitempty"`
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
	// StrategyParams 是 run 级参数覆写（QT-20）。pipeline 用它提交
	// expression_template 的表达式与仓位/风控参数，生效值仍由 core 按
	// 「清单默认 ⊕ 覆写」解析。
	StrategyParams map[string]any `json:"strategy_params,omitempty"`
}

// BacktestResponse 承载 core 的**三种**响应形状（同一组契约的三种包装）：
//
//  1. POST /api/backtest：`contracts.BacktestResponse` **平铺** —— status / error /
//     invalid_reasons 与指标同层，没有 `result` 键。
//  2. POST /api/backtest 的 202 异步分支：`{"job_id","status"}`。
//  3. GET /api/backtest/:id：job 表的一行（core `pkg/backtest/job.Job`），结果嵌在
//     `result` 里，且 job 行的 status 是**生命周期**（pending/running/completed/failed），
//     不是引擎裁定。两条落盘路径对 invalid 的处理并不一致：**同步**通道
//     `SaveSyncResult` 把带 status 的完整响应写进 result、job 行写 completed；
//     **异步**通道的 invalid 走 `UpdateJobFailed`（job 行 failed、结果体丢弃，
//     core `pkg/backtest/job/job.go:493`）。所以能读到嵌套 result 时，结论以
//     `result.status` 为准；读不到 result 时不能拿 job 的 completed 冒充成功。
//
// 旧实现只认 `result` 嵌套形状，于是同步 POST 的 200 响应解出来 Result 恒 nil
// （REV-01-A ① 的漂移），调用方再一读指标就是 nil-deref。
//
// 字段名对齐 `contracts.BacktestResponse`；`error` / `invalid_reasons` /
// `universe_max_size` 三项 core 的 openapi.yaml **今天漏登记**（contracts 里有、
// openapi 里没有），登记在 `openapiKnownGaps`，core 补登记后对拍测试会提醒删掉。
type BacktestResponse struct {
	ID     string          `json:"id"`
	JobID  string          `json:"job_id,omitempty"` // 202 异步分支：{"job_id","status"}（openapi post /api/backtest 的 202 内联 schema）
	Status string          `json:"status"`
	Error  string          `json:"error,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`

	StartDate       string                  `json:"start_date,omitempty"`
	EndDate         string                  `json:"end_date,omitempty"`
	TotalReturn     float64                 `json:"total_return,omitempty"`
	AnnualReturn    float64                 `json:"annual_return,omitempty"`
	SharpeRatio     float64                 `json:"sharpe_ratio,omitempty"`
	SortinoRatio    float64                 `json:"sortino_ratio,omitempty"`
	MaxDrawdown     float64                 `json:"max_drawdown,omitempty"`
	MaxDrawdownDate string                  `json:"max_drawdown_date,omitempty"`
	WinRate         float64                 `json:"win_rate,omitempty"`
	TotalTrades     int                     `json:"total_trades,omitempty"`
	WinTrades       int                     `json:"win_trades,omitempty"`
	LoseTrades      int                     `json:"lose_trades,omitempty"`
	AvgHoldingDays  float64                 `json:"avg_holding_days,omitempty"`
	CalmarRatio     float64                 `json:"calmar_ratio,omitempty"`
	PortfolioValues []domain.PortfolioValue `json:"portfolio_values,omitempty"`
	Trades          []domain.Trade          `json:"trades,omitempty"`
	InvalidReasons  []string                `json:"invalid_reasons,omitempty"`
	UniverseMaxSize *int                    `json:"universe_max_size,omitempty"`
}

// K9 三值（冻结于 core `pkg/backtest/contracts/contracts.go` 的 Status* 常量与
// 「ResultSpec 冻结（K9）」注记：status 是调用方的**唯一**判据，三值属响应层，
// 不下沉进 domain.BacktestResult —— 那条裁决是 OBS-01-R2 的 (b)）。
// 本包只**声明取值**，不重算 status，也不新增第四值。
const (
	StatusCompleted = "completed"
	StatusInvalid   = "invalid"
	StatusFailed    = "failed"
)

// InvalidReasons 的稳定 Code（C1 映射：degraded 语义并入 invalid，不引入第四值）。
// 源码今天还没有生产者（core QT-10 / QT-15 待落），消费侧先按契约把判读收在这里；
// 未登记的 Code 原样透传，不猜、不翻译。
const (
	InvalidReasonDataGapSkipped    = "data_gap_skipped"
	InvalidReasonSessionTerminated = "session_terminated"
)

// 作业态取值（GET /api/backtest/:id 打的是 job 表，见 core pkg/backtest/job/job.go 的 Job.Status）：
// pending / running 不是 K9 三值之一，它是「还没跑完」，与 invalid（跑完了但结果无效）是两件事。
const (
	JobStatusPending = "pending"
	JobStatusRunning = "running"
)

// BacktestOutcome 是三值响应的**原样透传**（OBS-01-R2-A ③-1）。
//
// 三值语义（照 core 冻结口径，不改）：
//   - completed：结果可信，Result 有内容；
//   - invalid：结果可解释、不可当成功；`Result.InvalidReasons` 由引擎挂 Code，客户端只读不重算；
//   - failed：故障；`Error` 必填，同时以 error 返回（errors.Is(err, ErrBacktestFailed)），
//     让既有调用点照常中止，且不把它当成可重试的传输错误。
type BacktestOutcome struct {
	Result *domain.BacktestResult
	Status string
	Error  string
	JobID  string
}

// Succeeded 只认 completed（C1 的下游判据：「只接受 completed 为成功」）。
// invalid / failed / 作业态 / 空值一律 false —— 这不是便利函数，是把判据收在一处，
// 免得三个调用点各自写一遍「err == nil 就当成功」。
func (o *BacktestOutcome) Succeeded() bool {
	return o != nil && o.Status == StatusCompleted && o.Result != nil
}

// InvalidReasons 读引擎挂上去的 Code 列表（③-3：不在客户端重算无效原因）。
// Result 为 nil 时返回空切片而不是 panic —— 作业态与 failed 都可能是这个形状。
func (o *BacktestOutcome) InvalidReasons() []string {
	if o == nil || o.Result == nil {
		return nil
	}
	return o.Result.InvalidReasons
}

// HasInvalidReason 判断引擎是否用某个稳定 Code 判了无效。
// 未登记的新 Code 不会被猜成某个已知语义：这里只做精确匹配，其余由 InvalidReasons 原样交出。
func (o *BacktestOutcome) HasInvalidReason(code string) bool {
	for _, c := range o.InvalidReasons() {
		if c == code {
			return true
		}
	}
	return false
}

// classify 把「拿到了一份响应」这个事实变成 (outcome, error)。
//
// fromJob 区分 GET /api/backtest/:id（job 行，结果必须在嵌套 result 里）与
// POST /api/backtest（平铺体）。两者形状相似，但「completed 却没有 result」
// 在 job 通道是残缺响应，不能当成功；靠调用方传入来源，不靠字段猜。
//
// 错误语义只在这里收口：
//   - Error 非空（K9：failed ⇒ Error 必填）→ 带 outcome 一起返回 ErrBacktestFailed，
//     既有调用点的 `if err != nil` 语义逐字节不变；
//   - status 不是三值也不是作业态 → fail-closed 报错，点名收到的值。
//     空 status 尤其危险：旧落盘结果没记状态，把它当 completed 就是伪造判据；
//   - job 行 completed 但结果没记 status → fail-closed（K9：不许用间接信号推断成功）；
//   - job 行 completed / invalid 却没有 result → fail-closed（缺失不是空结果）；
//   - job.result 解不动 / 日期读不懂 → 报错（契约违约，不落零值）。
//
// invalid / completed 都返回 nil error —— 无效是一个**结果属性**不是故障，
// 调用方读 Succeeded()/Status 自己判。
func (r *BacktestResponse) classify(fromJob bool) (*BacktestOutcome, error) {
	jobID := r.ID
	if jobID == "" {
		jobID = r.JobID // 202 异步分支只有 job_id
	}
	outcome := &BacktestOutcome{Status: r.Status, Error: r.Error, JobID: jobID}
	payload := r

	inner, nested, err := r.nestedResult()
	if err != nil {
		return outcome, err
	}
	if nested {
		payload = inner
		// 结果自带的 status 是引擎裁定，优先于 job 行的生命周期状态。
		if inner.Status != "" {
			outcome.Status = inner.Status
		}
		if inner.Error != "" {
			outcome.Error = inner.Error
		}
		// job 行 completed 只说明「作业跑完了」；结果没记 status 时，
		// 把它当 completed 就是用间接信号伪造判据（K9 明令禁止）。
		if inner.Status == "" && r.Status == StatusCompleted {
			return outcome, fmt.Errorf(
				"job %q 状态是 completed，但落盘结果没记 status（ResultSpec 要求 status 必填）：不能当成功", r.ID)
		}
	} else if fromJob && (r.Status == StatusCompleted || r.Status == StatusInvalid) {
		return outcome, fmt.Errorf(
			"job %q 状态是 %q，但没有 result：作业完成不等于结果存在，不能当成功", r.ID, r.Status)
	}

	switch outcome.Status {
	case JobStatusPending, JobStatusRunning:
		return outcome, fmt.Errorf("backtest 还没跑完（status=%q, job_id=%q），用 GetBacktestResult 取结果", outcome.Status, outcome.JobID)
	}

	// 只有跑完的两种形态才有结果可交；failed / 未知值不构造零值 Result，免得调用方拿它去打分。
	if outcome.Status == StatusCompleted || outcome.Status == StatusInvalid {
		res, err := payload.toDomain()
		if err != nil {
			return outcome, err
		}
		outcome.Result = res
	}

	return outcome, classifyStatus(outcome)
}

// nestedResult 解出 GET /api/backtest/:id 的 job.result；平铺形状（POST）返回 present=false。
func (r *BacktestResponse) nestedResult() (*BacktestResponse, bool, error) {
	raw := bytes.TrimSpace(r.Result)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return nil, false, nil
	}
	var inner BacktestResponse
	if err := json.Unmarshal(raw, &inner); err != nil {
		return nil, true, fmt.Errorf("job.result 不符合 BacktestResponse 形状: %w", err)
	}
	return &inner, true, nil
}

// toDomain 把线上指标映射成 domain.BacktestResult（日期字段 core 发的是 2006-01-02）。
// 读不懂的日期直接报错而不是落零值：零值会被下游读成「1970 年」，那是伪造数据。
func (r *BacktestResponse) toDomain() (*domain.BacktestResult, error) {
	start, err := parseWireDate("start_date", r.StartDate)
	if err != nil {
		return nil, err
	}
	end, err := parseWireDate("end_date", r.EndDate)
	if err != nil {
		return nil, err
	}
	maxDD, err := parseWireDate("max_drawdown_date", r.MaxDrawdownDate)
	if err != nil {
		return nil, err
	}
	return &domain.BacktestResult{
		StartDate:       start,
		EndDate:         end,
		TotalReturn:     r.TotalReturn,
		AnnualReturn:    r.AnnualReturn,
		SharpeRatio:     r.SharpeRatio,
		SortinoRatio:    r.SortinoRatio,
		MaxDrawdown:     r.MaxDrawdown,
		MaxDrawdownDate: maxDD,
		WinRate:         r.WinRate,
		TotalTrades:     r.TotalTrades,
		WinTrades:       r.WinTrades,
		LoseTrades:      r.LoseTrades,
		AvgHoldingDays:  r.AvgHoldingDays,
		CalmarRatio:     r.CalmarRatio,
		PortfolioValues: r.PortfolioValues,
		Trades:          r.Trades,
		UniverseMaxSize: r.UniverseMaxSize,
		InvalidReasons:  r.InvalidReasons,
	}, nil
}

func parseWireDate(field, v string) (time.Time, error) {
	if v == "" {
		return time.Time{}, nil
	}
	for _, layout := range []string{"2006-01-02", time.RFC3339} {
		if t, err := time.Parse(layout, v); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("响应 %s=%q 不是可识别的日期（core 契约是 2006-01-02）", field, v)
}

// classifyStatus 是判读的第二段：错误语义只在这里收口。
func classifyStatus(o *BacktestOutcome) error {
	if o.Error != "" {
		return fmt.Errorf("%w: %s", ErrBacktestFailed, o.Error)
	}
	switch o.Status {
	case StatusCompleted, StatusInvalid:
		return nil
	case StatusFailed:
		// failed 却带不出 Error：core 的不变量是「failed ⇒ Error 必填」，走到这里说明响应残缺。
		return fmt.Errorf("%w: status=failed 但响应没带 error 字段", ErrBacktestFailed)
	default:
		return fmt.Errorf("未知的 backtest status %q（K9 只定义 completed / invalid / failed）", o.Status)
	}
}

// RunBacktest executes a backtest via the API.
//
// 返回的 *BacktestOutcome 在「拿到了响应」的所有分支上都非 nil（包括 failed），
// 传输层 / 解码 / HTTP 4xx-5xx / 缺凭证 才是 (nil, error)。
func (c *BacktestClient) RunBacktest(ctx context.Context, req BacktestRequest) (*BacktestOutcome, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := c.newAPIRequest(ctx, http.MethodPost, "/api/backtest", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("backtest API returned status %d", resp.StatusCode)
	}

	var btResp BacktestResponse
	if err := json.NewDecoder(resp.Body).Decode(&btResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return btResp.classify(false)
}

// GetBacktestResult polls for a backtest result by job ID
func (c *BacktestClient) GetBacktestResult(ctx context.Context, jobID string) (*BacktestOutcome, error) {
	httpReq, err := c.newAPIRequest(ctx, http.MethodGet, "/api/backtest/"+url.PathEscape(jobID), nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return nil, fmt.Errorf("backtest job not found: %s", jobID)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("backtest API returned status %d", resp.StatusCode)
	}

	var btResp BacktestResponse
	if err := json.NewDecoder(resp.Body).Decode(&btResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// job 表的标识字段叫 `id` 不是 `job_id`；id 缺失时用调用方问的那个 jobID 回填。
	if btResp.ID == "" {
		btResp.ID = jobID
	}
	return btResp.classify(true)
}

// WalkForwardRequest 是 `POST /api/walkforward` 的请求体。
//
// 字段逐条对齐 core 的 `walkforward.WalkForwardRequest`：**没有 index_code**
// （该 DTO 是 core 自有类型，只收 stock_pool）；`strategy_params` 由 QT-68
// 补齐，每个窗口的 train/test 两段原样带上它。
type WalkForwardRequest struct {
	Strategy          string                   `json:"strategy"`
	StockPool         []string                 `json:"stock_pool,omitempty"`
	StartDate         string                   `json:"start_date"`
	EndDate           string                   `json:"end_date"`
	StrategyParams    map[string]any           `json:"strategy_params,omitempty"`
	WalkForwardParams domain.WalkForwardParams `json:"walk_forward_params"`
}

// RunWalkForward 提交一次 walk-forward（D24 第 8 类粗粒度动作，裁-12）。
//
// 响应是 core 的 `domain.WalkForwardReport` 原样 JSON（200）。错误语义：
// 400（参数非法，点名键）、500（窗口执行失败）—— 两者都不在本仓改写文案。
func (c *BacktestClient) RunWalkForward(ctx context.Context, reqBody WalkForwardRequest) (*domain.WalkForwardReport, error) {
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal walk-forward request: %w", err)
	}

	httpReq, err := c.newAPIRequest(ctx, http.MethodPost, "/api/walkforward", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("walk-forward request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("walk-forward API returned status %d: %s",
			resp.StatusCode, strings.TrimSpace(string(msg)))
	}

	var report domain.WalkForwardReport
	if err := json.NewDecoder(resp.Body).Decode(&report); err != nil {
		return nil, fmt.Errorf("failed to decode walk-forward response: %w", err)
	}
	return &report, nil
}

// Health checks if the backtest service is healthy.
// /health 是公开运维面，不适用 API 的凭证门槛（不走 newAPIRequest）。
func (c *BacktestClient) Health(ctx context.Context) error {
	endpoint := c.baseURL + "/health"

	httpReq, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("health check returned status %d", resp.StatusCode)
	}

	return nil
}

// newAPIRequest 建一个带凭证的 API 请求；无凭证且未显式允许 loopback 开发时**不发请求**。
func (c *BacktestClient) newAPIRequest(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if err := c.authorize(req); err != nil {
		return nil, err
	}
	return req, nil
}

func (c *BacktestClient) authorize(req *http.Request) error {
	token := c.bearerToken
	if token == "" && c.tokenFromContext {
		token = BearerTokenFromContext(req.Context())
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
		return nil
	}
	if c.allowInsecureLoopback {
		if isLoopbackURL(c.baseURL) {
			return nil
		}
		return fmt.Errorf("WithInsecureLoopback 只允许 loopback 地址：baseURL %q 不是 loopback，拒绝无凭证调用", c.baseURL)
	}
	return fmt.Errorf("对 core 的请求没有凭证（默认 fail-closed）：用 WithBearerToken 配 JWT；本地 loopback 开发用 WithInsecureLoopback 显式开启")
}

func isLoopbackURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	host := u.Hostname()
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
