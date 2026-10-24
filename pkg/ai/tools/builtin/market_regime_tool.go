package builtin

import (
	"context"
	"fmt"
	"time"

	"github.com/ruoxizhnya/tellurion-contracts/domain"
	"tellurion-agent/pkg/ai/client"
	"tellurion-agent/pkg/ai/tools"
)

// ═══════════════════════════════════════════════════════════════════════
//  GetMarketRegimeTool (Hermes Phase 2.3)
// ═══════════════════════════════════════════════════════════════════════
//
// GetMarketRegimeTool gives Hermes a single-call view of the prevailing market
// regime (trend + volatility + sentiment) for a symbol and date range. It is
// the market-awareness companion to get_strategy_lineage (which is the
// history-awareness tool): together they let the agent answer "what regime
// was this strategy designed for, and what regime are we entering now?" before
// mutating or deploying a strategy.
//
// QT-17 S3-c-1：regime 判定改为 **一次粗粒度 HTTP 调用** core 的
// `POST /api/risk/detect_regime`（粗粒度形状 `symbol + start_date + end_date`，
// 服务端取前复权日线并判定）。本仓不再持有 `risk.RiskManager`，也不再自己
// 取 OHLCV、自己算 MA / 波动率——同一条纯函数只在 core 一处存在。
//
// 因此本工具的返回值里**没有** `bars_analyzed`：端点不回报参与计算的 bar 数，
// 本仓不能把一个没量过的数字印出来（愿景原则 4）。宁可少一个字段，也不给
// 一个看似可信的 0。
//
// Default symbol: "000300.SH" (CSI 300 index) — the standard A-share market
// benchmark. The agent can override with any stock or index symbol the
// analysis-service has OHLCV for.
//
// Tool name: "get_market_regime"
// Input: start_date (required), end_date (required), symbol (optional, default 000300.SH)
// Output: *marketRegimeResult — flattened regime fields + provenance metadata

const (
	// DefaultMarketRegimeSymbol is the symbol used when the caller doesn't
	// specify one. CSI 300 (000300.SH) is the standard A-share large-cap
	// benchmark and aligns with the default "csi300" universe used by the
	// backtest engine.
	DefaultMarketRegimeSymbol = "000300.SH"
)

// RegimeClient is the narrow interface satisfied by *client.RiskClient.
// Extracted so the tool can be unit-tested with a fake instead of standing up
// an HTTP server (the HTTP shape itself is covered in pkg/ai/client).
type RegimeClient interface {
	// DetectRegime classifies the market regime for symbol over
	// [startDate, endDate] (both YYYY-MM-DD). The service fetches the bars;
	// the caller never passes OHLCV across the boundary.
	DetectRegime(ctx context.Context, symbol, startDate, endDate string) (*domain.MarketRegime, error)
}

// Compile-time assertion that the concrete client satisfies the narrow
// interface. Failing at compile time is strictly better than failing at
// startup.
var _ RegimeClient = (*client.RiskClient)(nil)

// marketRegimeResult is the flattened, LLM-friendly return value of
// GetMarketRegimeTool.Execute. Rather than nesting *domain.MarketRegime inside
// a wrapper, we flatten its fields to the top level — this keeps the JSON one
// level shallower, which is easier for the agent to parse and reason about.
//
// Fields:
//   - Symbol / StartDate / EndDate: provenance metadata so the agent can cite
//     what data the regime was computed from.
//   - Trend / Volatility / Sentiment / AsOf: the regime classification itself,
//     flattened from *domain.MarketRegime.
type marketRegimeResult struct {
	Symbol     string    `json:"symbol"`
	StartDate  string    `json:"start_date"`
	EndDate    string    `json:"end_date"`
	Trend      string    `json:"trend"`      // "bull" | "bear" | "sideways"
	Volatility string    `json:"volatility"` // "low" | "medium" | "high"
	Sentiment  float64   `json:"sentiment"`  // [-1.0, 1.0]
	AsOf       time.Time `json:"as_of"`      // service-side classification timestamp
}

// GetMarketRegimeTool analyzes the market regime for a symbol and date range.
// See the package-level doc comment above for the full design.
type GetMarketRegimeTool struct {
	regime RegimeClient
}

var _ tools.Tool = (*GetMarketRegimeTool)(nil)

// NewGetMarketRegimeTool constructs a GetMarketRegimeTool backed by the given
// regime client. Panics on nil — fail-loud at wiring time (composition root),
// not at first Execute call.
func NewGetMarketRegimeTool(regime RegimeClient) *GetMarketRegimeTool {
	if regime == nil {
		panic("builtin: NewGetMarketRegimeTool called with nil RegimeClient")
	}
	return &GetMarketRegimeTool{regime: regime}
}

func (t *GetMarketRegimeTool) Name() string { return "get_market_regime" }

func (t *GetMarketRegimeTool) Description() string {
	return "Fetch market regime analysis (bull/bear/sideways trend + volatility level + sentiment score) " +
		"for a given symbol and date range. Defaults to CSI 300 index (000300.SH) as the A-share market benchmark. " +
		"Uses moving-average crossover + volatility classification. Requires sufficient OHLCV history (typically 200+ trading days). " +
		"Use this before strategy design to understand the prevailing market regime and pick strategies that fit it."
}

func (t *GetMarketRegimeTool) Parameters() []tools.Parameter {
	return []tools.Parameter{
		{
			Name:        "start_date",
			Type:        "string",
			Description: "Analysis start date in YYYY-MM-DD format. Inclusive.",
			Required:    true,
		},
		{
			Name:        "end_date",
			Type:        "string",
			Description: "Analysis end date in YYYY-MM-DD format. Inclusive.",
			Required:    true,
		},
		{
			Name:        "symbol",
			Type:        "string",
			Description: "Symbol to analyze. Defaults to '000300.SH' (CSI 300 index, the A-share market benchmark). Override with any stock or index symbol.",
			Required:    false,
			Default:     DefaultMarketRegimeSymbol,
		},
	}
}

func (t *GetMarketRegimeTool) OutputSchema() tools.OutputSchema {
	return tools.OutputSchema{
		Type:        "object",
		Description: "Market regime analysis: trend, volatility, sentiment, plus provenance metadata.",
		Fields: []tools.OutputField{
			{Name: "symbol", Type: "string", Description: "Analyzed symbol."},
			{Name: "start_date", Type: "string", Description: "Analysis start date (YYYY-MM-DD)."},
			{Name: "end_date", Type: "string", Description: "Analysis end date (YYYY-MM-DD)."},
			{Name: "trend", Type: "string", Description: "Market trend classification: 'bull', 'bear', or 'sideways'."},
			{Name: "volatility", Type: "string", Description: "Volatility level: 'low', 'medium', or 'high'."},
			{Name: "sentiment", Type: "float", Description: "Sentiment score in [-1.0, 1.0]. Positive = bullish, negative = bearish."},
			{Name: "as_of", Type: "string", Description: "Service-side timestamp when the regime was classified."},
		},
	}
}

// Execute runs the market regime analysis. Errors are returned when:
//   - start_date or end_date is missing/empty/not a string (wrapped ErrInvalidArgs)
//   - symbol has the wrong type (wrapped ErrInvalidArgs)
//   - start_date or end_date is not in YYYY-MM-DD format (wrapped ErrInvalidArgs)
//   - start_date is after end_date (wrapped ErrInvalidArgs)
//   - the risk endpoint fails: 400 (bad request), 404 (no rows in range),
//     503 (server store not wired), transport error
//   - DetectRegime fails (e.g., insufficient bars — typically needs 200+)
func (t *GetMarketRegimeTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	startDate, err := requireString(args, "start_date")
	if err != nil {
		return nil, err
	}
	endDate, err := requireString(args, "end_date")
	if err != nil {
		return nil, err
	}
	symbol, err := optionalString(args, "symbol")
	if err != nil {
		return nil, err
	}
	if symbol == "" {
		symbol = DefaultMarketRegimeSymbol
	}

	// Validate date formats early — a bad format would otherwise produce a
	// confusing downstream 400 from the risk endpoint.
	if err := validateDateFormat(startDate); err != nil {
		return nil, fmt.Errorf("get_market_regime: invalid start_date: %w", err)
	}
	if err := validateDateFormat(endDate); err != nil {
		return nil, fmt.Errorf("get_market_regime: invalid end_date: %w", err)
	}
	if startDate > endDate {
		return nil, fmt.Errorf("get_market_regime: start_date (%s) must not be after end_date (%s)", startDate, endDate)
	}

	regime, err := t.regime.DetectRegime(ctx, symbol, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("get_market_regime: detect regime for %s [%s, %s]: %w", symbol, startDate, endDate, err)
	}

	return &marketRegimeResult{
		Symbol:     symbol,
		StartDate:  startDate,
		EndDate:    endDate,
		Trend:      regime.Trend,
		Volatility: regime.Volatility,
		Sentiment:  regime.Sentiment,
		AsOf:       regime.Timestamp,
	}, nil
}

// ─── Internal helpers ─────────────────────────────────────────────────

// validateDateFormat checks that s is in YYYY-MM-DD format and is a real
// calendar date. Returns nil if valid.
func validateDateFormat(s string) error {
	_, err := time.Parse("2006-01-02", s)
	if err != nil {
		return fmt.Errorf("expected YYYY-MM-DD format, got %q", s)
	}
	return nil
}
