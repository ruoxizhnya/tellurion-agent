package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// ErrReferenceDataNoCredentials 是 fail-closed 的凭证闸：没有可转发的 JWT
// 时请求不发出，也不降级成匿名调用。
var ErrReferenceDataNoCredentials = errors.New("reference-data request has no bearer token (fail-closed)")

// ReferenceDataClient 调 tellurion-data 的只读交易日历。
//
// 数据源是 data 仓的 `GET /api/v1/trading/calendar`：SVC-CORE-DATA-RETIRE 后
// core 不再持有该表，也不反代这条路径。它只回答「哪些日子是交易日」——
// 没有交易所维度，也没有 is_trading_day 字段。
//
// 客户端不签发、不刷新凭证：token 由调用方（HTTP handler）从入站请求转发，
// 每次调用显式传入，不在客户端里长期持有。
type ReferenceDataClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewReferenceDataClient creates a client for tellurion-data's read-only reference-data API.
//
// 10s 超时：这是探索启动前的一次小读，data 不可达时不该让偏差维把整轮
// 探索拖住；超时/失败由调用方降级成最保守口径。
func NewReferenceDataClient(baseURL string) *ReferenceDataClient {
	return &ReferenceDataClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

type tradingCalendarResponse struct {
	TradingDays []string `json:"trading_days"`
}

// TradingCalendar 取 [startDate, endDate] 区间内的交易日（`YYYY-MM-DD`）。
// 日期只认 `YYYY-MM-DD`，由 data 校验并给出 400。
func (c *ReferenceDataClient) TradingCalendar(ctx context.Context, bearerToken, startDate, endDate string) ([]string, error) {
	query := url.Values{}
	query.Set("start", startDate)
	query.Set("end", endDate)

	// 路径字面量与 data 的路由逐字一致（endpoint_contract_test 的扫描器
	// 靠它找出处），查询串单独拼。
	path := "/api/v1/trading/calendar"
	if encoded := query.Encode(); encoded != "" {
		path += "?" + encoded
	}
	var out tradingCalendarResponse
	if err := c.get(ctx, bearerToken, path, &out); err != nil {
		return nil, err
	}
	return out.TradingDays, nil
}

func (c *ReferenceDataClient) get(ctx context.Context, bearerToken, path string, out any) error {
	if bearerToken == "" {
		return ErrReferenceDataNoCredentials
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("failed to create reference-data request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+bearerToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("reference-data request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("reference-data API GET %s returned status %d", path, resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("failed to decode reference-data response: %w", err)
	}
	return nil
}
