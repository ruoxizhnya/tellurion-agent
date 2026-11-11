// Package budget 把「LLM 日累计 USD」变成一道持久化的闸。
//
// hermes.yaml 里的 `max_cost_per_session: 5.0` 此前只是一行注释：进程内
// 只有 metrics.Metrics 的 map，重启即归零，也没有任何调用点在花钱之前查它。
// 这里的口径（AGENT-LLM-BUDGET-GATE）：
//
//   - 闸看的是**当日累计**（UTC 日），不是单次或单会话 —— agent 没有可靠的
//     会话边界，按日累计才是重启杀不掉的口径；
//   - 累计值由 ai.ai_costs 的行派生，重启后从库里重读；
//   - 花费读不出来时 fail-closed：拒绝调用，而不是当成 0 放行。
package budget

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

// 错误码。调用方（job 状态 / 日志 / 运维）按码判断，不要去解析文案。
const (
	CodeExhausted   = "llm_budget_exhausted"
	CodeLedgerError = "llm_budget_ledger_error"
)

// DefaultDailyLimitUSD 与 docs/hermes/config/hermes.yaml 的
// max_cost_per_session: 5.0 对齐（按日口径执行）。可用 AI_DAILY_BUDGET_USD 覆盖。
const DefaultDailyLimitUSD = 5.0

// Ledger 是日累计成本的持久化读写面。*aistore.Store 满足它。
type Ledger interface {
	AddLLMCost(ctx context.Context, day time.Time, model string, promptTokens, completionTokens int, costUSD float64) error
	DailyLLMCost(ctx context.Context, day time.Time) (float64, error)
}

// Error 是预算闸的显式失败。Code 是机器可读的错误码。
type Error struct {
	Code         string
	Spent        float64
	Limit        float64
	LedgerDetail error
}

func (e *Error) Error() string {
	if e.LedgerDetail != nil {
		return fmt.Sprintf("llm budget: %s: %v", e.Code, e.LedgerDetail)
	}
	return fmt.Sprintf("llm budget: %s (spent $%.4f of daily limit $%.2f)", e.Code, e.Spent, e.Limit)
}

func (e *Error) Unwrap() error { return e.LedgerDetail }

// ErrExhausted 用于 errors.Is 判定「超预算」。
var ErrExhausted = errors.New("llm daily budget exhausted")

func (e *Error) Is(target error) bool {
	return target == ErrExhausted && e.Code == CodeExhausted
}

// Gate 是预算闸本体。零值不可用；用 NewGate 构造。
type Gate struct {
	Ledger        Ledger
	DailyLimitUSD float64
	// Now 是时钟，测试可替换。nil 时用 time.Now。
	Now func() time.Time
}

// NewGate 构造闸。limit <= 0 时回落到 DefaultDailyLimitUSD —— 不把
// 「配置写错」解释成「不要闸」。
func NewGate(ledger Ledger, limitUSD float64) *Gate {
	if limitUSD <= 0 {
		limitUSD = DefaultDailyLimitUSD
	}
	return &Gate{Ledger: ledger, DailyLimitUSD: limitUSD, Now: time.Now}
}

// NewGateFromEnv 用 AI_DAILY_BUDGET_USD 构造；未设置或非法值走默认 5 USD。
func NewGateFromEnv(ledger Ledger) *Gate {
	limit := DefaultDailyLimitUSD
	if raw := os.Getenv("AI_DAILY_BUDGET_USD"); raw != "" {
		if v, err := strconv.ParseFloat(raw, 64); err == nil && v > 0 {
			limit = v
		}
	}
	return NewGate(ledger, limit)
}

// Check 在发起 LLM 调用前查当日累计花费。已到/超过上限即拒绝。
// 读账失败同样拒绝（fail-closed）：先花钱后记账，账都读不出来就别花。
func (g *Gate) Check(ctx context.Context) error {
	if g == nil || g.Ledger == nil {
		return nil
	}
	spent, err := g.Ledger.DailyLLMCost(ctx, g.now())
	if err != nil {
		return &Error{Code: CodeLedgerError, Limit: g.DailyLimitUSD, LedgerDetail: err}
	}
	if spent >= g.DailyLimitUSD {
		return &Error{Code: CodeExhausted, Spent: spent, Limit: g.DailyLimitUSD}
	}
	return nil
}

// Record 记一次调用的花费。调用方必须把返回的错误当成调用失败处理：
// 花费没入账，下一道闸就会低估，等于开了一个越花越多的口子。
func (g *Gate) Record(ctx context.Context, model string, promptTokens, completionTokens int, costUSD float64) error {
	if g == nil || g.Ledger == nil {
		return nil
	}
	if err := g.Ledger.AddLLMCost(ctx, g.now(), model, promptTokens, completionTokens, costUSD); err != nil {
		return &Error{Code: CodeLedgerError, Limit: g.DailyLimitUSD, LedgerDetail: err}
	}
	return nil
}

// Spent 返回当日累计花费（运维读面 / 测试用）。
func (g *Gate) Spent(ctx context.Context) (float64, error) {
	if g == nil || g.Ledger == nil {
		return 0, nil
	}
	return g.Ledger.DailyLLMCost(ctx, g.now())
}

func (g *Gate) now() time.Time {
	if g.Now != nil {
		return g.Now()
	}
	return time.Now()
}
