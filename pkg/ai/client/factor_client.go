package client

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"
)

// ErrFactorCapabilityUnavailable 表示「因子求值能力在当前 core 上不存在」。
//
// 这是**事实性不可用**，不是瞬时故障：core 只注册了 POST /api/factor/registrations
// 与反代 GET /api/factors/:factor_name（后者已因 data 撤路由成为死链），
// 从来没有 /api/factor/compute 或 /api/factor/evaluate；docs/SPEC.md 里的
// compute-returns/compute-ic 也从未实现。裁决见跨仓 ADR-041：
// **不新造端点**，能力未提供就显式返回未提供。
//
// 调用方应当把本错误当作终态（不要重试），并向用户/LLM 说明改用
// 「提交回测 + 读结果」的替代路径。
var ErrFactorCapabilityUnavailable = errors.New(
	"factor evaluation capability is not provided: core has no /api/factor/compute|evaluate endpoint (ADR-041); use backtest submission instead")

// FactorClient 是因子能力的**占位客户端**：保留类型与请求/响应形状，
// 但 compute/evaluate 已不再发出任何 HTTP 请求（ADR-041）。今天只有 Health
// 会真正打网络。保留类型是为了在 core 真提供求值面时能重新接线，而不是
// 让三个工具在运行期发一个必然 404 的请求。
type FactorClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewFactorClient creates a new factor client
func NewFactorClient(baseURL string) *FactorClient {
	return &FactorClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// ComputeFactorRequest represents a factor computation request
type ComputeFactorRequest struct {
	Formula   string   `json:"formula"`
	Symbols   []string `json:"symbols"`
	StartDate string   `json:"start_date"`
	EndDate   string   `json:"end_date"`
}

// ComputeFactorResponse represents a factor computation response
type ComputeFactorResponse struct {
	Values map[string][]float64 `json:"values,omitempty"`
	IC     float64              `json:"ic,omitempty"`
	IR     float64              `json:"ir,omitempty"`
	Error  string               `json:"error,omitempty"`
}

// ComputeFactor 在当前栈上**不可用**：直接返回 ErrFactorCapabilityUnavailable，
// 不发起 HTTP 请求。签名保留以便将来重新接线。
func (c *FactorClient) ComputeFactor(ctx context.Context, req ComputeFactorRequest) (map[string][]float64, error) {
	return nil, fmt.Errorf("compute factor %q: %w", req.Formula, ErrFactorCapabilityUnavailable)
}

// EvaluateFactor 在当前栈上**不可用**：直接返回 ErrFactorCapabilityUnavailable，
// 不发起 HTTP 请求。签名保留以便将来重新接线。
func (c *FactorClient) EvaluateFactor(ctx context.Context, formula string, symbols []string, startDate, endDate string) (*FactorMetrics, error) {
	return nil, fmt.Errorf("evaluate factor %q over %d symbols: %w", formula, len(symbols), ErrFactorCapabilityUnavailable)
}

// FactorMetrics represents factor quality metrics
type FactorMetrics struct {
	IC float64 `json:"ic"`
	IR float64 `json:"ir"`
}

// Health checks if the factor service is healthy
func (c *FactorClient) Health(ctx context.Context) error {
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
