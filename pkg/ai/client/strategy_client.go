package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// StrategyClient 调 core analysis-service 的 `GET /api/strategies`。
//
// QT-17 S3-c-2：本仓不再 import `tellurion/pkg/strategy`（core 实现包）来读
// 包级注册表。注册表条目与 DB 活跃项本来就是 core 的真相，本仓只消费一份
// 合并后的快照 —— 「注册表 ∪ DB 活跃项」的合并语义由 `strategyDB.ListWithDB`
// 在服务端一处实现（DB 条目覆盖同名注册表条目）。
//
// **不发任何 query 参数**是有意为之：core 的 handler 只有 `type` / `active`
// 全空时才走 `ListWithDB` 合并分支；带上 `?active=true` 会退化成「只查 DB」，
// 内置策略会整批消失。本包的用例把「无 query」钉死。
type StrategyClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewStrategyClient creates a client for core's strategy listing API.
//
// 30s 与其它 analysis-service 客户端同档：这是一次只读查询，超时即失败，
// 不重试。
func NewStrategyClient(baseURL string) *StrategyClient {
	return &StrategyClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// StrategyInfo mirrors the JSON emitted by core for a strategy catalog entry
// (`pkg/strategy.StrategyInfo`): name / description / parameters. Kept as a
// local DTO rather than importing core's package — the wire shape is the
// contract, not the Go type.
type StrategyInfo struct {
	Name        string              `json:"name"`
	Description string              `json:"description"`
	Parameters  []StrategyParameter `json:"parameters"`
}

// StrategyParameter mirrors core's `pkg/strategy.Parameter` descriptor.
type StrategyParameter struct {
	Name        string  `json:"name"`
	Type        string  `json:"type"` // "int", "float", "string", "bool"
	Default     any     `json:"default"`
	Description string  `json:"description"`
	Min         float64 `json:"min"`
	Max         float64 `json:"max"`
}

// strategyListResponse is the envelope core returns: {"strategies": [...]}.
type strategyListResponse struct {
	Strategies []StrategyInfo `json:"strategies"`
}

// ListStrategies returns the merged catalog (registry ∪ active DB strategies).
//
// 错误语义（core 侧原样透传，不在本仓改写）：401 —— 未认证；500 ——
// DB 查询失败或 params blob 损坏（core 对损坏的 params fail-loud，不返回
// 一份缺参数的策略）。
func (c *StrategyClient) ListStrategies(ctx context.Context) ([]StrategyInfo, error) {
	// 路径字面量与 core openapi 的 path 键逐字一致（endpoint_contract_test 的
	// 扫描器靠它找出处）。刻意不带 query：见类型注释。
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/strategies", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create list_strategies request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("list_strategies request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("list_strategies returned status %d: %s",
			resp.StatusCode, strings.TrimSpace(string(msg)))
	}

	var out strategyListResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("failed to decode list_strategies response: %w", err)
	}
	return out.Strategies, nil
}
