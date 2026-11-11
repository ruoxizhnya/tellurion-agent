package aistore

import (
	"context"
	"fmt"
	"time"
)

// AddLLMCost 累加一次 LLM 调用的 token 与 USD 花费。
//
// 一行 = 一个 UTC 日 × 一个模型；同一模型在同一天反复调用走 upsert 累加。
// 这是 ai.ai_costs 的唯一写入口（AGENT-LLM-BUDGET-GATE）。
func (s *Store) AddLLMCost(
	ctx context.Context,
	day time.Time,
	model string,
	promptTokens, completionTokens int,
	costUSD float64,
) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO ai.ai_costs
			(day, model, prompt_tokens, completion_tokens, cost_usd, calls, updated_at)
		VALUES ($1, $2, $3, $4, $5, 1, NOW())
		ON CONFLICT (day, model) DO UPDATE SET
			prompt_tokens     = ai.ai_costs.prompt_tokens + EXCLUDED.prompt_tokens,
			completion_tokens = ai.ai_costs.completion_tokens + EXCLUDED.completion_tokens,
			cost_usd          = ai.ai_costs.cost_usd + EXCLUDED.cost_usd,
			calls             = ai.ai_costs.calls + 1,
			updated_at        = NOW()
	`, day.UTC().Format("2006-01-02"), model, promptTokens, completionTokens, costUSD)
	if err != nil {
		return fmt.Errorf("failed to record llm cost (%s/%s): %w", day.UTC().Format("2006-01-02"), model, err)
	}
	return nil
}

// DailyLLMCost 返回某个 UTC 日的累计 USD 花费。没有记录时返回 0。
func (s *Store) DailyLLMCost(ctx context.Context, day time.Time) (float64, error) {
	var total float64
	err := s.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(cost_usd), 0)
		FROM ai.ai_costs
		WHERE day = $1
	`, day.UTC().Format("2006-01-02")).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("failed to read daily llm cost: %w", err)
	}
	return total, nil
}
