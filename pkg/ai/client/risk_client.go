package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/ruoxizhnya/tellurion-contracts/domain"
)

// RiskClient 调 core analysis-service 的 `POST /api/risk/detect_regime`
// **粗粒度形状**（QT-70）：只发 symbol + 日期区间，由服务端取前复权日线并在
// 进程内判定 regime。
//
// 为什么不用内联形状（`{"data":[OHLCV...]}`）：那要把 200+ 行行情塞进 agent
// 上下文再回传 —— get_market_regime 工具存在的意义就是避免这件事。服务端
// 取数与判定是同一条纯函数（readonly-post，viewer 即可），所以「同参数同数据
// ⇒ 同结果」由 core 的那一份实现保证，本仓不再自己算。
//
// 本仓之所以需要它（QT-17 S3-c-1）：`tellurion/pkg/risk` 是 core 实现包，
// 本仓的编译期依赖随 S3 退场；工具改为一次粗粒度 HTTP 快照。
type RiskClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewRiskClient creates a client for core's risk API.
//
// 30s 与 core 该端点的服务端预算一致（`cmd/analysis/handlers_risk.go` 的
// `context.WithTimeout(c.Request.Context(), 30*time.Second)`）：取数 + 判定是
// 一次纯计算，超时就是失败，不重试（重试只会把超时叠成更长的超时）。
func NewRiskClient(baseURL string) *RiskClient {
	return &RiskClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// detectRegimeCoarseRequest 是 `POST /api/risk/detect_regime` 的粗粒度请求体。
// 键名逐字对齐 core `docs/openapi.yaml`（symbol / start_date / end_date）；
// **不发 `data`** —— 内联形状在 core 侧优先级更高，一旦带上就会把服务端取数
// 分支挤掉，等于又回到「客户端算」。
type detectRegimeCoarseRequest struct {
	Symbol    string `json:"symbol"`
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
}

// DetectRegime 用粗粒度形状取一次 regime 快照。
//
// 错误语义（core 侧原样透传，不在本仓改写）：
//   - 400：字段缺失 / 日期格式坏 / start_date 晚于 end_date
//   - 404：区间内没有 OHLCV 行（core 不会拿空序列编一份看似合理的 regime）
//   - 503：服务端取数面未接线（fail-closed）
func (c *RiskClient) DetectRegime(ctx context.Context, symbol, startDate, endDate string) (*domain.MarketRegime, error) {
	body, err := json.Marshal(detectRegimeCoarseRequest{
		Symbol:    symbol,
		StartDate: startDate,
		EndDate:   endDate,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to encode detect_regime request: %w", err)
	}

	// 路径字面量与 core openapi 的 path 键逐字一致（endpoint_contract_test 的
	// 扫描器靠它找出处）。
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/risk/detect_regime", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create detect_regime request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("detect_regime request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("detect_regime %s [%s, %s] returned status %d: %s",
			symbol, startDate, endDate, resp.StatusCode, strings.TrimSpace(string(msg)))
	}

	var regime domain.MarketRegime
	if err := json.NewDecoder(resp.Body).Decode(&regime); err != nil {
		return nil, fmt.Errorf("failed to decode detect_regime response: %w", err)
	}
	return &regime, nil
}
