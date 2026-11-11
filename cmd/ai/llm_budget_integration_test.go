package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	kitdb "github.com/ruoxizhnya/tellurion-kit/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"tellurion-agent/pkg/ai"
	"tellurion-agent/pkg/ai/budget"
	"tellurion-agent/pkg/aistore"
)

// TestLLMBudgetGate_PersistsAcrossRestart 是日累计 LLM 预算闸
// （AGENT-LLM-BUDGET-GATE）的真库腿：
//
//  1. 花费落 ai.ai_costs，当日累计由行派生；
//  2. 到上限后 Chat 拒调用，错误码 llm_budget_exhausted，且一次 HTTP 都没发；
//  3. 重建连接池 + Store + Gate（模拟进程重启），仍然拒绝 —— 删掉持久化、
//     靠重启绕过闸，第 3 步立刻变红。
func TestLLMBudgetGate_PersistsAcrossRestart(t *testing.T) {
	ctx := context.Background()
	dsn := strings.TrimSpace(os.Getenv("TEST_DATABASE_DSN"))
	if dsn == "" {
		t.Skip("TEST_DATABASE_DSN 未设置：跳过真库腿（本地 docker compose 起库后指定 DSN 可跑）")
	}

	pool, err := kitdb.Connect(ctx, dsn, kitdb.DefaultPoolOptions())
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	require.NoError(t, aistore.Migrate(ctx, pool))
	// 迁移幂等：第二次执行不得报错，也不得改表形状。
	require.NoError(t, aistore.Migrate(ctx, pool), "重复迁移必须幂等")
	store := aistore.NewStoreWithPool(pool)

	const model = "itest-budget-model"
	today := time.Now().UTC()
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM ai.ai_costs WHERE model = $1`, model)
	})

	// 预置到上限：1 USD，闸也设 1 USD。
	require.NoError(t, store.AddLLMCost(ctx, today, model, 100, 100, 1.0))
	spent, err := store.DailyLLMCost(ctx, today)
	require.NoError(t, err)
	require.InDelta(t, 1.0, spent, 1e-9, "日累计必须由 ai.ai_costs 的行派生")

	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"ok"}}],"usage":{"prompt_tokens":1000,"completion_tokens":1000,"total_tokens":2000}}`)
	}))
	t.Cleanup(srv.Close)

	t.Setenv("AI_API_KEY", "test-key")
	t.Setenv("AI_API_URL", srv.URL)
	t.Setenv("AI_MODEL", model)
	t.Setenv("AI_DAILY_BUDGET_USD", "1")

	// 走生产装配：newLLMClient(store) + 环境变量额度。
	client := newLLMClient(store)

	_, err = client.Chat(ctx, []ai.ChatMessage{{Role: "user", Content: "hi"}})
	require.Error(t, err, "到达日累计上限后必须拒调用")

	var be *budget.Error
	require.ErrorAs(t, err, &be)
	assert.Equal(t, budget.CodeExhausted, be.Code, "必须给机器可读的错误码")
	assert.Zero(t, atomic.LoadInt32(&hits), "被拒的调用连 HTTP 都不该发")

	// ── 模拟重启：新连接池 + 新 Store + 新 Gate，没有任何进程内状态可带走 ──
	pool2, err := kitdb.Connect(ctx, dsn, kitdb.DefaultPoolOptions())
	require.NoError(t, err)
	t.Cleanup(pool2.Close)
	restarted := aistore.NewStoreWithPool(pool2)

	client2 := newLLMClient(restarted)

	_, err = client2.Chat(ctx, []ai.ChatMessage{{Role: "user", Content: "hi"}})
	require.Error(t, err, "重启后闸必须从 ai.ai_costs 重读累计，不能归零放行")

	require.ErrorAs(t, err, &be)
	assert.Equal(t, budget.CodeExhausted, be.Code)
	assert.Zero(t, atomic.LoadInt32(&hits))

	// 库里确实只有一行、且花费没有被重启抹掉。
	var rows int
	var total float64
	require.NoError(t, pool2.QueryRow(ctx,
		`SELECT count(*), COALESCE(SUM(cost_usd), 0) FROM ai.ai_costs WHERE model = $1`,
		model).Scan(&rows, &total))
	assert.Equal(t, 1, rows)
	assert.InDelta(t, 1.0, total, 1e-9)
}

// TestLLMBudgetGate_RealCallCrossesLimit 验证记账真的推高累计：一次成功调用
// 之后，同一道闸的第二次调用被拒；重启后的新闸同样拒绝。
func TestLLMBudgetGate_RealCallCrossesLimit(t *testing.T) {
	ctx := context.Background()
	dsn := strings.TrimSpace(os.Getenv("TEST_DATABASE_DSN"))
	if dsn == "" {
		t.Skip("TEST_DATABASE_DSN 未设置：跳过真库腿（本地 docker compose 起库后指定 DSN 可跑）")
	}

	pool, err := kitdb.Connect(ctx, dsn, kitdb.DefaultPoolOptions())
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	require.NoError(t, aistore.Migrate(ctx, pool))
	store := aistore.NewStoreWithPool(pool)

	const model = "itest-budget-cross"
	day := time.Date(2001, 2, 4, 0, 0, 0, 0, time.UTC)
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM ai.ai_costs WHERE day = $1`, day.Format("2006-01-02"))
	})

	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"ok"}}],"usage":{"prompt_tokens":1000000,"completion_tokens":1000000,"total_tokens":2000000}}`)
	}))
	t.Cleanup(srv.Close)

	gate := budget.NewGate(store, 0.01)
	gate.Now = func() time.Time { return day }
	client, err := ai.NewClientWithOptions(
		ai.WithAPIKey("test-key"),
		ai.WithAPIURL(srv.URL),
		ai.WithModel(model),
		ai.WithLimiter(nil),
		ai.WithBudgetGate(gate),
	)
	require.NoError(t, err)

	_, err = client.Chat(ctx, []ai.ChatMessage{{Role: "user", Content: "hi"}})
	require.NoError(t, err, "首次调用未到上限，应放行")

	persisted, err := store.DailyLLMCost(ctx, day)
	require.NoError(t, err)
	require.GreaterOrEqual(t, persisted, 0.01, "百万 token 的调用必须把花费推过上限并落库")

	_, err = client.Chat(ctx, []ai.ChatMessage{{Role: "user", Content: "hi"}})
	require.Error(t, err, "记账后第二道闸必须拒绝")
	var be *budget.Error
	require.ErrorAs(t, err, &be)
	assert.Equal(t, budget.CodeExhausted, be.Code)
	assert.Equal(t, int32(1), atomic.LoadInt32(&hits))

	// 重启：新 Gate 读同一份落库累计。
	gate2 := budget.NewGate(store, 0.01)
	gate2.Now = func() time.Time { return day }
	restarted, err := ai.NewClientWithOptions(
		ai.WithAPIKey("test-key"),
		ai.WithAPIURL(srv.URL),
		ai.WithModel(model),
		ai.WithLimiter(nil),
		ai.WithBudgetGate(gate2),
	)
	require.NoError(t, err)
	_, err = restarted.Chat(ctx, []ai.ChatMessage{{Role: "user", Content: "hi"}})
	require.Error(t, err, "重启后仍应拒绝")
	assert.Equal(t, int32(1), atomic.LoadInt32(&hits))
}
