package ai

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"tellurion-agent/pkg/ai/budget"
)

// testCostLedger 是跨 Gate 实例共享的账本：第二次 NewGate 读同一份数据，
// 等价于「进程重启后从库里重读」。
type testCostLedger struct {
	spend   map[string]float64
	addErr  error
	readErr error
}

func newTestCostLedger() *testCostLedger {
	return &testCostLedger{spend: map[string]float64{}}
}

func (l *testCostLedger) AddLLMCost(_ context.Context, day time.Time, _ string, _, _ int, cost float64) error {
	if l.addErr != nil {
		return l.addErr
	}
	l.spend[day.UTC().Format("2006-01-02")] += cost
	return nil
}

func (l *testCostLedger) DailyLLMCost(_ context.Context, day time.Time) (float64, error) {
	if l.readErr != nil {
		return 0, l.readErr
	}
	return l.spend[day.UTC().Format("2006-01-02")], nil
}

// budgetTestServer 记录真实 HTTP 命中次数 —— 「拒调用」的判据必须包含
// 「一次请求都没发出去」，而不是只看返回了错误。
func budgetTestServer(t *testing.T, hits *int32) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(hits, 1)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"ok"}}],"usage":{"prompt_tokens":1000000,"completion_tokens":1000000,"total_tokens":2000000}}`)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func budgetTestClient(t *testing.T, url string, gate *budget.Gate) *Client {
	t.Helper()
	c, err := NewClientWithOptions(
		WithAPIKey("test-key"),
		WithAPIURL(url),
		WithModel("budget-test-model"),
		WithLimiter(nil),
		WithBudgetGate(gate),
	)
	require.NoError(t, err)
	return c
}

func fixedGate(ledger budget.Ledger, limit float64, day time.Time) *budget.Gate {
	g := budget.NewGate(ledger, limit)
	g.Now = func() time.Time { return day }
	return g
}

var budgetDay = time.Date(2026, 11, 12, 9, 0, 0, 0, time.UTC)

func TestClient_BudgetGate_RejectsBeforeHTTP(t *testing.T) {
	var hits int32
	srv := budgetTestServer(t, &hits)

	ledger := newTestCostLedger()
	ledger.spend[budgetDay.Format("2006-01-02")] = 5.0
	client := budgetTestClient(t, srv.URL, fixedGate(ledger, 5.0, budgetDay))

	_, err := client.Chat(context.Background(), []ChatMessage{{Role: "user", Content: "hi"}})
	require.Error(t, err)

	var be *budget.Error
	require.ErrorAs(t, err, &be)
	assert.Equal(t, budget.CodeExhausted, be.Code)
	assert.Zero(t, atomic.LoadInt32(&hits), "超预算必须连一次 HTTP 都不发，而不是发了再丢结果")
}

func TestClient_BudgetGate_RecordsSuccessfulCall(t *testing.T) {
	var hits int32
	srv := budgetTestServer(t, &hits)

	ledger := newTestCostLedger()
	gate := fixedGate(ledger, 5.0, budgetDay)
	client := budgetTestClient(t, srv.URL, gate)

	text, err := client.Chat(context.Background(), []ChatMessage{{Role: "user", Content: "hi"}})
	require.NoError(t, err)
	assert.Equal(t, "ok", text)
	assert.Equal(t, int32(1), atomic.LoadInt32(&hits))

	spent, err := gate.Spent(context.Background())
	require.NoError(t, err)
	assert.Positive(t, spent, "Chat 走的是带 usage 的路径，花费必须入账")
}

// TestClient_BudgetGate_SurvivesRestart 是「删掉持久化、靠重启绕过闸应红」的
// 单元腿：同一份账本重建 Gate 与 client（等价于重启），必须仍被拒。
func TestClient_BudgetGate_SurvivesRestart(t *testing.T) {
	var hits int32
	srv := budgetTestServer(t, &hits)

	ledger := newTestCostLedger()
	ledger.spend[budgetDay.Format("2006-01-02")] = 5.0

	first := budgetTestClient(t, srv.URL, fixedGate(ledger, 5.0, budgetDay))
	_, err := first.Chat(context.Background(), []ChatMessage{{Role: "user", Content: "hi"}})
	require.Error(t, err)

	second := budgetTestClient(t, srv.URL, fixedGate(ledger, 5.0, budgetDay))
	_, err = second.Chat(context.Background(), []ChatMessage{{Role: "user", Content: "hi"}})
	require.Error(t, err, "重启后闸必须从账本重读，不能归零放行")

	var be *budget.Error
	require.ErrorAs(t, err, &be)
	assert.Equal(t, budget.CodeExhausted, be.Code)
	assert.Zero(t, atomic.LoadInt32(&hits))
}

// TestClient_BudgetGate_CrossesLimitOnRecordedSpend 走完整因果链：
// 第一次调用放行并记账 → 第二次同一天被同一道闸拒绝 → 重启后仍被拒。
func TestClient_BudgetGate_CrossesLimitOnRecordedSpend(t *testing.T) {
	var hits int32
	srv := budgetTestServer(t, &hits)

	ledger := newTestCostLedger()
	gate := fixedGate(ledger, 0.01, budgetDay)
	client := budgetTestClient(t, srv.URL, gate)

	_, err := client.Chat(context.Background(), []ChatMessage{{Role: "user", Content: "hi"}})
	require.NoError(t, err, "首次调用未到上限，应放行")
	spent, err := gate.Spent(context.Background())
	require.NoError(t, err)
	require.GreaterOrEqual(t, spent, 0.01, "百万 token 的调用必须把花费推过 0.01 上限")

	_, err = client.Chat(context.Background(), []ChatMessage{{Role: "user", Content: "hi"}})
	require.Error(t, err, "记账后第二道闸必须拒绝")
	var be *budget.Error
	require.ErrorAs(t, err, &be)
	assert.Equal(t, budget.CodeExhausted, be.Code)
	assert.Equal(t, int32(1), atomic.LoadInt32(&hits), "被拒的第二次不应发 HTTP")

	restarted := budgetTestClient(t, srv.URL, fixedGate(ledger, 0.01, budgetDay))
	_, err = restarted.Chat(context.Background(), []ChatMessage{{Role: "user", Content: "hi"}})
	require.Error(t, err, "重启后仍应拒绝")
	assert.Equal(t, int32(1), atomic.LoadInt32(&hits))
}

func TestClient_BudgetGate_LedgerReadErrorFailsClosed(t *testing.T) {
	var hits int32
	srv := budgetTestServer(t, &hits)

	ledger := newTestCostLedger()
	ledger.readErr = errors.New("db down")
	client := budgetTestClient(t, srv.URL, fixedGate(ledger, 5.0, budgetDay))

	_, err := client.Chat(context.Background(), []ChatMessage{{Role: "user", Content: "hi"}})
	require.Error(t, err)

	var be *budget.Error
	require.ErrorAs(t, err, &be)
	assert.Equal(t, budget.CodeLedgerError, be.Code)
	assert.Zero(t, atomic.LoadInt32(&hits), "账读不出来就不能花钱")
}

func TestClient_BudgetGate_RecordErrorFailsCall(t *testing.T) {
	var hits int32
	srv := budgetTestServer(t, &hits)

	ledger := newTestCostLedger()
	ledger.addErr = errors.New("db down")
	client := budgetTestClient(t, srv.URL, fixedGate(ledger, 5.0, budgetDay))

	_, err := client.Chat(context.Background(), []ChatMessage{{Role: "user", Content: "hi"}})
	require.Error(t, err, "花费没入账必须让本次调用失败")

	var be *budget.Error
	require.ErrorAs(t, err, &be)
	assert.Equal(t, budget.CodeLedgerError, be.Code)
	assert.Equal(t, int32(1), atomic.LoadInt32(&hits))
}
