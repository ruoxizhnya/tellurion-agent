package budget

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// memLedger 是进程内账本，只用于单元测试。生产路径必须落 ai.ai_costs
// （真库腿见 cmd/ai/llm_budget_integration_test.go）。
type memLedger struct {
	spend map[string]float64
	err   error
}

func newMemLedger() *memLedger {
	return &memLedger{spend: map[string]float64{}}
}

func (m *memLedger) AddLLMCost(_ context.Context, day time.Time, _ string, _, _ int, cost float64) error {
	if m.err != nil {
		return m.err
	}
	m.spend[day.UTC().Format("2006-01-02")] += cost
	return nil
}

func (m *memLedger) DailyLLMCost(_ context.Context, day time.Time) (float64, error) {
	if m.err != nil {
		return 0, m.err
	}
	return m.spend[day.UTC().Format("2006-01-02")], nil
}

var testDay = time.Date(2026, 11, 12, 10, 0, 0, 0, time.UTC)

func fixedGate(ledger Ledger, limit float64) *Gate {
	g := NewGate(ledger, limit)
	g.Now = func() time.Time { return testDay }
	return g
}

func TestGate_AllowsUnderLimit(t *testing.T) {
	ledger := newMemLedger()
	gate := fixedGate(ledger, 5.0)
	require.NoError(t, gate.Record(context.Background(), "m", 100, 100, 4.99))
	assert.NoError(t, gate.Check(context.Background()), "未到上限必须放行")
}

func TestGate_RejectsAtLimitWithExplicitCode(t *testing.T) {
	ledger := newMemLedger()
	gate := fixedGate(ledger, 5.0)
	require.NoError(t, gate.Record(context.Background(), "m", 100, 100, 5.0))

	err := gate.Check(context.Background())
	require.Error(t, err)

	var be *Error
	require.ErrorAs(t, err, &be)
	assert.Equal(t, CodeExhausted, be.Code, "超预算必须是可判定的错误码，不能靠解析文案")
	assert.Equal(t, 5.0, be.Limit)
	assert.Contains(t, err.Error(), CodeExhausted)
}

func TestGate_RejectsAboveLimit(t *testing.T) {
	ledger := newMemLedger()
	gate := fixedGate(ledger, 5.0)
	require.NoError(t, gate.Record(context.Background(), "m", 100, 100, 7.5))

	err := gate.Check(context.Background())
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrExhausted), "超预算应能用 errors.Is 判定")
}

func TestGate_RejectsWhenLedgerReadFails(t *testing.T) {
	ledger := newMemLedger()
	ledger.err = errors.New("db down")
	gate := fixedGate(ledger, 5.0)

	err := gate.Check(context.Background())
	require.Error(t, err)

	var be *Error
	require.ErrorAs(t, err, &be)
	assert.Equal(t, CodeLedgerError, be.Code, "账读不出来必须 fail-closed，不能当 0 放行")
	assert.False(t, errors.Is(err, ErrExhausted), "读账失败不等于超预算")
}

func TestGate_RecordFailureIsExplicit(t *testing.T) {
	ledger := newMemLedger()
	ledger.err = errors.New("db down")
	gate := fixedGate(ledger, 5.0)

	err := gate.Record(context.Background(), "m", 10, 10, 0.1)
	require.Error(t, err)

	var be *Error
	require.ErrorAs(t, err, &be)
	assert.Equal(t, CodeLedgerError, be.Code, "花费没入账必须报错，否则下一道闸会低估")
}

func TestGate_SpentReadsLedger(t *testing.T) {
	ledger := newMemLedger()
	gate := fixedGate(ledger, 5.0)
	require.NoError(t, gate.Record(context.Background(), "m", 10, 10, 1.25))

	spent, err := gate.Spent(context.Background())
	require.NoError(t, err)
	assert.InDelta(t, 1.25, spent, 1e-9)
}

func TestGate_DayBoundaryResetsAccumulation(t *testing.T) {
	ledger := newMemLedger()
	gate := fixedGate(ledger, 5.0)
	require.NoError(t, gate.Record(context.Background(), "m", 10, 10, 5.0))
	require.Error(t, gate.Check(context.Background()))

	next := testDay.AddDate(0, 0, 1)
	gate.Now = func() time.Time { return next }
	assert.NoError(t, gate.Check(context.Background()), "闸是日累计口径，跨日应重新起算")
}

func TestNewGate_NonPositiveLimitFallsBack(t *testing.T) {
	ledger := newMemLedger()
	for _, limit := range []float64{0, -1} {
		gate := NewGate(ledger, limit)
		assert.Equal(t, DefaultDailyLimitUSD, gate.DailyLimitUSD,
			"配置写错不能解释成「不要闸」")
	}
}

func TestNewGateFromEnv(t *testing.T) {
	ledger := newMemLedger()

	t.Setenv("AI_DAILY_BUDGET_USD", "")
	assert.Equal(t, DefaultDailyLimitUSD, NewGateFromEnv(ledger).DailyLimitUSD)

	t.Setenv("AI_DAILY_BUDGET_USD", "12.5")
	assert.Equal(t, 12.5, NewGateFromEnv(ledger).DailyLimitUSD)

	t.Setenv("AI_DAILY_BUDGET_USD", "not-a-number")
	assert.Equal(t, DefaultDailyLimitUSD, NewGateFromEnv(ledger).DailyLimitUSD,
		"非法值回落默认，不能变成 0 = 不设闸")

	t.Setenv("AI_DAILY_BUDGET_USD", "0")
	assert.Equal(t, DefaultDailyLimitUSD, NewGateFromEnv(ledger).DailyLimitUSD)
}

func TestGate_NilIsNoop(t *testing.T) {
	var gate *Gate
	assert.NoError(t, gate.Check(context.Background()))
	assert.NoError(t, gate.Record(context.Background(), "m", 1, 1, 1))
	spent, err := gate.Spent(context.Background())
	assert.NoError(t, err)
	assert.Zero(t, spent)
}

func TestGate_NilLedgerIsNoop(t *testing.T) {
	gate := NewGate(nil, 1.0)
	assert.NoError(t, gate.Check(context.Background()))
	assert.NoError(t, gate.Record(context.Background(), "m", 1, 1, 1))
}
