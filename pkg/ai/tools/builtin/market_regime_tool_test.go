package builtin

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/ruoxizhnya/tellurion-contracts/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"tellurion-agent/pkg/ai/tools"
)

// ═══════════════════════════════════════════════════════════════════════
//  Mock implementation
// ═══════════════════════════════════════════════════════════════════════

// mockRegimeClient satisfies RegimeClient for testing. It records the
// (symbol, startDate, endDate) triple it received and returns a preset
// regime/error. The HTTP shape itself is covered in pkg/ai/client.
type mockRegimeClient struct {
	regime   *domain.MarketRegime
	err      error
	gotCtx   context.Context
	gotSym   string
	gotStart string
	gotEnd   string
	calls    int
}

func (m *mockRegimeClient) DetectRegime(ctx context.Context, symbol, startDate, endDate string) (*domain.MarketRegime, error) {
	m.calls++
	m.gotCtx = ctx
	m.gotSym = symbol
	m.gotStart = startDate
	m.gotEnd = endDate
	if m.err != nil {
		return nil, m.err
	}
	return m.regime, nil
}

func defaultRegime() *domain.MarketRegime {
	return &domain.MarketRegime{
		Trend:      "bull",
		Volatility: "medium",
		Sentiment:  0.5,
		Timestamp:  time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC),
	}
}

func newMarketRegimeTool(regime *mockRegimeClient) *GetMarketRegimeTool {
	if regime == nil {
		regime = &mockRegimeClient{regime: defaultRegime()}
	}
	return NewGetMarketRegimeTool(regime)
}

// ═══════════════════════════════════════════════════════════════════════
//  Identity tests (Name / Description / Parameters / OutputSchema)
// ═══════════════════════════════════════════════════════════════════════

func TestGetMarketRegimeTool_Name(t *testing.T) {
	tool := newMarketRegimeTool(nil)
	assert.Equal(t, "get_market_regime", tool.Name())
}

func TestGetMarketRegimeTool_Description(t *testing.T) {
	tool := newMarketRegimeTool(nil)
	desc := tool.Description()
	assert.Contains(t, desc, "regime")
	assert.Contains(t, desc, "bull")
	assert.Contains(t, desc, "sideways")
}

func TestGetMarketRegimeTool_Parameters(t *testing.T) {
	tool := newMarketRegimeTool(nil)
	params := tool.Parameters()
	require.Len(t, params, 3)

	// start_date — required
	assert.Equal(t, "start_date", params[0].Name)
	assert.Equal(t, "string", params[0].Type)
	assert.True(t, params[0].Required)

	// end_date — required
	assert.Equal(t, "end_date", params[1].Name)
	assert.Equal(t, "string", params[1].Type)
	assert.True(t, params[1].Required)

	// symbol — optional with default
	assert.Equal(t, "symbol", params[2].Name)
	assert.Equal(t, "string", params[2].Type)
	assert.False(t, params[2].Required)
	assert.Equal(t, DefaultMarketRegimeSymbol, params[2].Default)
}

func TestGetMarketRegimeTool_OutputSchema(t *testing.T) {
	tool := newMarketRegimeTool(nil)
	schema := tool.OutputSchema()
	assert.Equal(t, "object", schema.Type)
	// QT-17 S3-c-1: bars_analyzed 已移除（端点不回报 bar 数，不印没量过的数字）。
	assert.Len(t, schema.Fields, 7)

	fieldNames := make(map[string]bool, len(schema.Fields))
	for _, f := range schema.Fields {
		fieldNames[f.Name] = true
	}
	expectedFields := []string{
		"symbol", "start_date", "end_date",
		"trend", "volatility", "sentiment", "as_of",
	}
	for _, name := range expectedFields {
		assert.True(t, fieldNames[name], "schema should include field %q", name)
	}
	assert.False(t, fieldNames["bars_analyzed"], "bars_analyzed 不再存在（未量过的数字不进输出）")
}

// ═══════════════════════════════════════════════════════════════════════
//  Constructor nil-panic tests
// ═══════════════════════════════════════════════════════════════════════

func TestNewGetMarketRegimeTool_NilClientPanics(t *testing.T) {
	assert.Panics(t, func() {
		NewGetMarketRegimeTool(nil)
	})
}

// ═══════════════════════════════════════════════════════════════════════
//  Execute — happy paths
// ═══════════════════════════════════════════════════════════════════════

func TestGetMarketRegimeTool_Execute_DefaultSymbol(t *testing.T) {
	regime := &mockRegimeClient{regime: &domain.MarketRegime{
		Trend:      "bull",
		Volatility: "low",
		Sentiment:  0.7,
		Timestamp:  time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC),
	}}
	tool := NewGetMarketRegimeTool(regime)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"start_date": "2023-01-01",
		"end_date":   "2024-06-01",
	})
	require.NoError(t, err)
	require.NotNil(t, result)

	// The client is called with the default symbol and the caller's range.
	assert.Equal(t, DefaultMarketRegimeSymbol, regime.gotSym)
	assert.Equal(t, "2023-01-01", regime.gotStart)
	assert.Equal(t, "2024-06-01", regime.gotEnd)

	res, ok := result.(*marketRegimeResult)
	require.True(t, ok)
	assert.Equal(t, DefaultMarketRegimeSymbol, res.Symbol)
	assert.Equal(t, "2023-01-01", res.StartDate)
	assert.Equal(t, "2024-06-01", res.EndDate)
	assert.Equal(t, "bull", res.Trend)
	assert.Equal(t, "low", res.Volatility)
	assert.Equal(t, 0.7, res.Sentiment)
	assert.Equal(t, time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC), res.AsOf)
}

func TestGetMarketRegimeTool_Execute_CustomSymbol(t *testing.T) {
	regime := &mockRegimeClient{regime: &domain.MarketRegime{Trend: "bear"}}
	tool := NewGetMarketRegimeTool(regime)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"start_date": "2023-01-01",
		"end_date":   "2024-01-01",
		"symbol":     "600000.SH",
	})
	require.NoError(t, err)

	res := result.(*marketRegimeResult)
	assert.Equal(t, "600000.SH", res.Symbol)
	assert.Equal(t, "600000.SH", regime.gotSym)
	assert.Equal(t, "bear", res.Trend)
}

func TestGetMarketRegimeTool_Execute_TrendValues(t *testing.T) {
	for _, trend := range []string{"bull", "bear", "sideways"} {
		t.Run(trend, func(t *testing.T) {
			regime := &mockRegimeClient{regime: &domain.MarketRegime{Trend: trend}}
			tool := NewGetMarketRegimeTool(regime)

			result, err := tool.Execute(context.Background(), map[string]interface{}{
				"start_date": "2023-01-01",
				"end_date":   "2024-01-01",
			})
			require.NoError(t, err)
			assert.Equal(t, trend, result.(*marketRegimeResult).Trend)
		})
	}
}

func TestGetMarketRegimeTool_Execute_VolatilityLevels(t *testing.T) {
	for _, vol := range []string{"low", "medium", "high"} {
		t.Run(vol, func(t *testing.T) {
			regime := &mockRegimeClient{regime: &domain.MarketRegime{Volatility: vol}}
			tool := NewGetMarketRegimeTool(regime)

			result, err := tool.Execute(context.Background(), map[string]interface{}{
				"start_date": "2023-01-01",
				"end_date":   "2024-01-01",
			})
			require.NoError(t, err)
			assert.Equal(t, vol, result.(*marketRegimeResult).Volatility)
		})
	}
}

func TestGetMarketRegimeTool_Execute_SentimentRange(t *testing.T) {
	// Sentiment is [-1.0, 1.0] — verify extreme values pass through.
	for _, s := range []float64{-1.0, 0.0, 0.5, 1.0} {
		regime := &mockRegimeClient{regime: &domain.MarketRegime{Sentiment: s}}
		tool := NewGetMarketRegimeTool(regime)

		result, err := tool.Execute(context.Background(), map[string]interface{}{
			"start_date": "2023-01-01",
			"end_date":   "2024-01-01",
		})
		require.NoError(t, err)
		assert.Equal(t, s, result.(*marketRegimeResult).Sentiment)
	}
}

// TestGetMarketRegimeTool_SameParamsSameDataSameRegime 是 S3-c-1 的对拍腿：
// 工具已不参与 regime 计算，只把服务端的结果逐字段透传。
//
// 基线取自 core 的同一份纯函数（`risk.RegimeDetector`，config 为
// fast_ma=10 / slow_ma=20 / vol_lookback=60）对固定序列的判定：
//
//	250 根、日期 2024-01-01 起逐日、close = 100+i
//	=> trend="sideways" volatility="low" sentiment=0.3220913107511046
//
// 服务端（core）对同一 symbol + 区间取到同一批行时，返回的就是这一份；
// 本测试钉住「除 provenance 外，工具不添加/不修改任何 regime 字段」。
func TestGetMarketRegimeTool_SameParamsSameDataSameRegime(t *testing.T) {
	golden := &domain.MarketRegime{
		Trend:      "sideways",
		Volatility: "low",
		Sentiment:  0.3220913107511046,
		Timestamp:  time.Date(2024, 9, 7, 0, 0, 0, 0, time.UTC),
	}
	regime := &mockRegimeClient{regime: golden}
	tool := NewGetMarketRegimeTool(regime)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"start_date": "2024-01-01",
		"end_date":   "2024-09-06",
		"symbol":     "000300.SH",
	})
	require.NoError(t, err)

	res := result.(*marketRegimeResult)
	assert.Equal(t, "sideways", res.Trend)
	assert.Equal(t, "low", res.Volatility)
	assert.Equal(t, 0.3220913107511046, res.Sentiment)
	assert.Equal(t, golden.Timestamp, res.AsOf)
	assert.Equal(t, "000300.SH", res.Symbol)
	assert.Equal(t, "2024-01-01", res.StartDate)
	assert.Equal(t, "2024-09-06", res.EndDate)
}

// ═══════════════════════════════════════════════════════════════════════
//  Execute — argument validation errors
// ═══════════════════════════════════════════════════════════════════════

func TestGetMarketRegimeTool_Execute_MissingStartDate(t *testing.T) {
	tool := newMarketRegimeTool(nil)
	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"end_date": "2024-01-01",
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, tools.ErrInvalidArgs))
}

func TestGetMarketRegimeTool_Execute_MissingEndDate(t *testing.T) {
	tool := newMarketRegimeTool(nil)
	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"start_date": "2023-01-01",
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, tools.ErrInvalidArgs))
}

func TestGetMarketRegimeTool_Execute_EmptyStartDate(t *testing.T) {
	tool := newMarketRegimeTool(nil)
	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"start_date": "",
		"end_date":   "2024-01-01",
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, tools.ErrInvalidArgs))
}

func TestGetMarketRegimeTool_Execute_WrongTypeStartDate(t *testing.T) {
	tool := newMarketRegimeTool(nil)
	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"start_date": 20230101,
		"end_date":   "2024-01-01",
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, tools.ErrInvalidArgs))
}

func TestGetMarketRegimeTool_Execute_WrongTypeSymbol(t *testing.T) {
	tool := newMarketRegimeTool(nil)
	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"start_date": "2023-01-01",
		"end_date":   "2024-01-01",
		"symbol":     123,
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, tools.ErrInvalidArgs))
}

func TestGetMarketRegimeTool_Execute_InvalidDateFormat(t *testing.T) {
	tool := newMarketRegimeTool(nil)
	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"start_date": "not-a-date",
		"end_date":   "2024-01-01",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "start_date")
	assert.Contains(t, err.Error(), "YYYY-MM-DD")
}

func TestGetMarketRegimeTool_Execute_InvalidDateFormatEnd(t *testing.T) {
	tool := newMarketRegimeTool(nil)
	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"start_date": "2023-01-01",
		"end_date":   "2024/01/01",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "end_date")
	assert.Contains(t, err.Error(), "YYYY-MM-DD")
}

func TestGetMarketRegimeTool_Execute_StartAfterEnd(t *testing.T) {
	tool := newMarketRegimeTool(nil)
	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"start_date": "2024-06-01",
		"end_date":   "2023-01-01",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must not be after")
}

func TestGetMarketRegimeTool_Execute_EmptyStringSymbol(t *testing.T) {
	// Empty string symbol should fall back to default.
	regime := &mockRegimeClient{regime: &domain.MarketRegime{}}
	tool := NewGetMarketRegimeTool(regime)

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"start_date": "2023-01-01",
		"end_date":   "2024-01-01",
		"symbol":     "",
	})
	require.NoError(t, err)
	assert.Equal(t, DefaultMarketRegimeSymbol, regime.gotSym)
}

// ═══════════════════════════════════════════════════════════════════════
//  Execute — downstream errors
// ═══════════════════════════════════════════════════════════════════════

func TestGetMarketRegimeTool_Execute_DetectError(t *testing.T) {
	// Simulate core's "no OHLCV rows" (404) / "insufficient bars" errors —
	// both must surface through the tool unchanged.
	detectErr := errors.New("no OHLCV rows for symbol \"999999.SH\" between 2023-01-01 and 2024-01-01")
	regime := &mockRegimeClient{err: detectErr}
	tool := NewGetMarketRegimeTool(regime)

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"start_date": "2023-01-01",
		"end_date":   "2024-01-01",
		"symbol":     "999999.SH",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "detect regime")
	assert.Contains(t, err.Error(), "999999.SH")
	assert.Contains(t, err.Error(), "no OHLCV rows")
}

func TestGetMarketRegimeTool_Execute_ClientNotCalledOnValidationError(t *testing.T) {
	regime := &mockRegimeClient{regime: &domain.MarketRegime{}}
	tool := NewGetMarketRegimeTool(regime)

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"start_date": "2024-06-01",
		"end_date":   "2023-01-01",
	})
	require.Error(t, err)
	assert.Zero(t, regime.calls, "参数不合法时不该发请求")
}

// ═══════════════════════════════════════════════════════════════════════
//  Helper function tests
// ═══════════════════════════════════════════════════════════════════════

func TestValidateDateFormat_Valid(t *testing.T) {
	assert.NoError(t, validateDateFormat("2024-01-01"))
	assert.NoError(t, validateDateFormat("2024-12-31"))
	assert.NoError(t, validateDateFormat("2023-06-15"))
}

func TestValidateDateFormat_Invalid(t *testing.T) {
	assert.Error(t, validateDateFormat("not-a-date"))
	assert.Error(t, validateDateFormat("2024/01/01"))
	assert.Error(t, validateDateFormat("20240101"))
	assert.Error(t, validateDateFormat("2024-13-01")) // month 13
	assert.Error(t, validateDateFormat("2024-01-32")) // day 32
	assert.Error(t, validateDateFormat(""))
}

// ═══════════════════════════════════════════════════════════════════════
//  Integration tests
// ═══════════════════════════════════════════════════════════════════════

func TestGetMarketRegimeTool_RegisterInRegistry(t *testing.T) {
	reg := tools.NewRegistry()
	tool := newMarketRegimeTool(nil)
	require.NoError(t, reg.Register(tool))

	// Verify it's discoverable.
	listed := reg.List()
	found := false
	for _, t := range listed {
		if t.Name == "get_market_regime" {
			found = true
			break
		}
	}
	assert.True(t, found, "get_market_regime should be in registry")
}

func TestGetMarketRegimeTool_JSONMarshalable(t *testing.T) {
	regime := &mockRegimeClient{regime: &domain.MarketRegime{
		Trend:      "bull",
		Volatility: "high",
		Sentiment:  0.85,
		Timestamp:  time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC),
	}}
	tool := NewGetMarketRegimeTool(regime)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"start_date": "2023-01-01",
		"end_date":   "2024-06-01",
	})
	require.NoError(t, err)

	data, err := json.Marshal(result)
	require.NoError(t, err)
	assert.NotEmpty(t, data)

	// Verify key JSON fields are present.
	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &m))
	assert.Equal(t, "bull", m["trend"])
	assert.Equal(t, "high", m["volatility"])
	assert.Equal(t, "000300.SH", m["symbol"])
	_, hasBars := m["bars_analyzed"]
	assert.False(t, hasBars, "输出里不该有 bars_analyzed")
}

func TestGetMarketRegimeTool_ContextPropagatedToClient(t *testing.T) {
	regime := &mockRegimeClient{regime: &domain.MarketRegime{}}
	tool := NewGetMarketRegimeTool(regime)

	ctx := context.WithValue(context.Background(), "testkey", "testval")
	_, err := tool.Execute(ctx, map[string]interface{}{
		"start_date": "2023-01-01",
		"end_date":   "2024-01-01",
	})
	require.NoError(t, err)
	assert.Equal(t, ctx, regime.gotCtx)
}
