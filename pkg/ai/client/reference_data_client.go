package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// ErrReferenceDataNoCredentials 是 fail-closed 的凭证闸：没有可转发的 JWT
// 时请求不发出，也不降级成匿名调用。
var ErrReferenceDataNoCredentials = errors.New("reference-data request has no bearer token (fail-closed)")

// ReferenceDataClient 调 core 的 QT-67 只读动作（D24 第 7 类）：
// 交易日历与上市窗口。两条都是**粗粒度**端点（一次请求覆盖整个区间 /
// 整批 symbol），不是 per-symbol / per-bar 远程调用。
//
// 客户端不签发、不刷新凭证：token 由调用方（HTTP handler）从入站请求转发，
// 每次调用显式传入，不在客户端里长期持有。
type ReferenceDataClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewReferenceDataClient creates a client for core's read-only reference-data API.
//
// 10s 超时：这是探索启动前的一次小读，core 不可达时不该让偏差维把整轮
// 探索拖住；超时/失败由调用方降级成最保守口径。
func NewReferenceDataClient(baseURL string) *ReferenceDataClient {
	return &ReferenceDataClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// TradingCalendarEntry 是 `GET /api/v1/trading-calendar` 的一行，
// 字段与 core `docs/openapi.yaml` 的 TradingCalendarEntry 逐字对齐。
type TradingCalendarEntry struct {
	TradeDate    string `json:"trade_date"`
	Exchange     string `json:"exchange"`
	IsTradingDay bool   `json:"is_trading_day"`
}

type tradingCalendarResponse struct {
	StartDate string                 `json:"start_date"`
	EndDate   string                 `json:"end_date"`
	Count     int                    `json:"count"`
	Entries   []TradingCalendarEntry `json:"entries"`
}

// ListingWindow 是 `POST /api/v1/listing-windows` 的一个标的窗口。
// `list_date` / `delist_date` 为 NULL 时是 nil（未知 ≠ 一个看起来合理的日期）。
type ListingWindow struct {
	Symbol     string  `json:"symbol"`
	ListDate   *string `json:"list_date"`
	DelistDate *string `json:"delist_date"`
}

type listingWindowsRequest struct {
	Symbols []string `json:"symbols,omitempty"`
}

type listingWindowsResponse struct {
	Count   int             `json:"count"`
	Windows []ListingWindow `json:"windows"`
}

// TradingCalendar 取 [startDate, endDate] 区间内的全部日历行（含休市日）。
// 日期只认 `YYYY-MM-DD`，由 core 校验并给出 400。
func (c *ReferenceDataClient) TradingCalendar(ctx context.Context, bearerToken, startDate, endDate string) ([]TradingCalendarEntry, error) {
	query := url.Values{}
	query.Set("start", startDate)
	query.Set("end", endDate)

	// 路径字面量与 openapi 的 path 键逐字一致（endpoint_contract_test 的
	// 扫描器靠它找出处），查询串单独拼。
	path := "/api/v1/trading-calendar"
	if encoded := query.Encode(); encoded != "" {
		path += "?" + encoded
	}
	var out tradingCalendarResponse
	if err := c.get(ctx, bearerToken, path, &out); err != nil {
		return nil, err
	}
	return out.Entries, nil
}

// ListingWindows 取一批标的的上市窗口；symbols 为空 = core 侧全部标的。
// 未知 symbol 会被 core 省略，调用方按请求集对差即可。
func (c *ReferenceDataClient) ListingWindows(ctx context.Context, bearerToken string, symbols []string) ([]ListingWindow, error) {
	body, err := json.Marshal(listingWindowsRequest{Symbols: symbols})
	if err != nil {
		return nil, fmt.Errorf("failed to encode listing-windows request: %w", err)
	}

	var out listingWindowsResponse
	if err := c.do(ctx, bearerToken, http.MethodPost, "/api/v1/listing-windows", body, &out); err != nil {
		return nil, err
	}
	return out.Windows, nil
}

func (c *ReferenceDataClient) get(ctx context.Context, bearerToken, path string, out any) error {
	return c.do(ctx, bearerToken, http.MethodGet, path, nil, out)
}

func (c *ReferenceDataClient) do(ctx context.Context, bearerToken, method, path string, body []byte, out any) error {
	if bearerToken == "" {
		return ErrReferenceDataNoCredentials
	}

	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return fmt.Errorf("failed to create reference-data request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+bearerToken)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("reference-data request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("reference-data API %s %s returned status %d", method, path, resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("failed to decode reference-data response: %w", err)
	}
	return nil
}
