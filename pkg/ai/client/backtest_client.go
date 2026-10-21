package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"tellurion/pkg/domain"
)

// BacktestClient is an HTTP client for the backtest API
type BacktestClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewBacktestClient creates a new backtest client
func NewBacktestClient(baseURL string) *BacktestClient {
	return &BacktestClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// BacktestRequest represents a backtest request
type BacktestRequest struct {
	StrategyName string   `json:"strategy_name"`
	StockPool    []string `json:"stock_pool"`
	StartDate    string   `json:"start_date"`
	EndDate      string   `json:"end_date"`
}

// BacktestResponse represents a backtest response
type BacktestResponse struct {
	Result *domain.BacktestResult `json:"result,omitempty"`
	Error  string                 `json:"error,omitempty"`
	JobID  string                 `json:"job_id,omitempty"`
	Status string                 `json:"status"`
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

// 作业态取值（GET /api/backtest/:id 打的是 job 表，见 core pkg/backtest/job/job.go 的 Job.Status）：
// pending / running 不是 K9 三值之一，它是「还没跑完」，与 invalid（跑完了但结果无效）是两件事。
const (
	JobStatusPending = "pending"
	JobStatusRunning = "running"
)

// BacktestOutcome 是三值响应的**原样透传**（OBS-01-R2-A ③-1）。
//
// 为什么要有这个结构：`RunBacktest` 以前返回 `(*domain.BacktestResult, error)`，
// 而 `status` 在解出来那一刻就被丢掉了 —— 于是 `invalid`（跑完了、但引擎判它无效）
// 与 `completed` 在调用方手里长得一模一样，只能反过来从 `InvalidReasons` 猜。
// 「猜」正是 K9 明令禁止的判据来源。
//
// 三值语义（照 core 冻结口径，不改）：
//   - completed：结果可信，Result 有内容；
//   - invalid：结果可解释、不可当成功；`Result.InvalidReasons` 由引擎挂 Code，客户端只读不重算；
//   - failed：故障；`Error` 必填，同时以 error 返回，让既有调用点照常中止。
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

// outcome 从解出来的响应建三值视图，字段一律原样搬，不做任何推断。
func (r *BacktestResponse) outcome() *BacktestOutcome {
	return &BacktestOutcome{
		Result: r.Result,
		Status: r.Status,
		Error:  r.Error,
		JobID:  r.JobID,
	}
}

// classify 把「拿到了一份响应」这个事实变成 (outcome, error)。
//
// 两条中止线，各有理由：
//   - Error 非空（K9：failed ⇒ Error 必填）→ 带 outcome 一起返回 error，
//     既有调用点的 `if err != nil` 语义逐字节不变（OBS-01-R2-A 之前它们就是这么中止的）；
//   - status 不是三值也不是作业态 → fail-closed 报错，点名收到的值。
//     空 status 尤其危险：旧落盘结果没记状态，把它当 completed 就是伪造判据。
//
// invalid / completed 都返回 nil error —— 无效是一个**结果属性**不是故障，
// 调用方读 Succeeded()/Status 自己判，这正是本单要还给它的能力。
func (r *BacktestResponse) classify() (*BacktestOutcome, error) {
	outcome := r.outcome()
	if r.Error != "" {
		return outcome, fmt.Errorf("backtest failed: %s", r.Error)
	}
	switch r.Status {
	case StatusCompleted, StatusInvalid:
		return outcome, nil
	case StatusFailed:
		// failed 却带不出 Error：core 的不变量是「failed ⇒ Error 必填」，走到这里说明响应残缺。
		return outcome, fmt.Errorf("backtest failed: status=failed 但响应没带 error 字段")
	case JobStatusPending, JobStatusRunning:
		return outcome, fmt.Errorf("backtest 还没跑完（status=%q, job_id=%q），用 GetBacktestResult 取结果", r.Status, r.JobID)
	default:
		return outcome, fmt.Errorf("未知的 backtest status %q（K9 只定义 completed / invalid / failed）", r.Status)
	}
}

// RunBacktest executes a backtest via the API.
//
// 返回的 *BacktestOutcome 在「拿到了响应」的所有分支上都非 nil（包括 failed），
// 传输层 / 解码 / HTTP 4xx-5xx 才是 (nil, error)。
func (c *BacktestClient) RunBacktest(ctx context.Context, req BacktestRequest) (*BacktestOutcome, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/backtest", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

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

	return btResp.classify()
}

// GetBacktestResult polls for a backtest result by job ID
func (c *BacktestClient) GetBacktestResult(ctx context.Context, jobID string) (*BacktestOutcome, error) {
	url := fmt.Sprintf("%s/api/backtest/%s", c.baseURL, jobID)

	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
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

	// GET /api/backtest/:id 打的是 job 表（core pkg/backtest/job/job.go 的 Job），
	// pending / running 是「还没完」，不是 K9 三值之一；classify 里已按作业态单独报错，
	// 这里不再重判一遍（两处判据会漂移）。
	// Job 的标识字段叫 `id` 不是 `job_id`，所以我们问的那个 jobID 才是权威回填。
	if btResp.JobID == "" {
		btResp.JobID = jobID
	}
	return btResp.classify()
}

// Health checks if the backtest service is healthy
func (c *BacktestClient) Health(ctx context.Context) error {
	url := c.baseURL + "/health"

	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
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
