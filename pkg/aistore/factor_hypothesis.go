package aistore

import (
	"context"
	"fmt"
	"time"

	"github.com/ruoxizhnya/tellurion-contracts/domain"
)

// 因子的假设来源（P2-3）—— core public.factor_hypothesis 的 ai.* 自持版，
// SQL 语义逐字对照 tellurion:pkg/storage/factor_hypothesis.go。类型直接用
// 契约仓的 domain.FactorHypothesis / domain.FactorType（这是跨仓契约，不是
// 本仓 DTO），因此 tool 的窄接口不需要任何转换适配。
//
// factor_cache 存因子的数值，这里存因子的理由。缺了后者，就没法回答
// 「这个因子凭什么有效」——「在数据上试出来的相关性」和「有经济学机制支撑
// 的效应」是两回事。

// SaveFactorHypothesis 写入或更新一个因子的假设来源。factor_name 做主键、
// 覆盖式更新（连同 updated_at），不是追加历史。
func (s *Store) SaveFactorHypothesis(ctx context.Context, h domain.FactorHypothesis) error {
	if h.FactorName == "" {
		return fmt.Errorf("factor hypothesis: factor_name is required")
	}
	if h.Hypothesis == "" {
		return fmt.Errorf("factor hypothesis: hypothesis is required (空假设等于没记录)")
	}
	if h.SourceKind == "" {
		h.SourceKind = domain.FactorSourceAdHoc
	}

	const query = `
		INSERT INTO ai.factor_hypothesis (factor_name, source_kind, hypothesis, reference)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (factor_name) DO UPDATE SET
			source_kind = EXCLUDED.source_kind,
			hypothesis  = EXCLUDED.hypothesis,
			reference   = EXCLUDED.reference,
			updated_at  = NOW()
	`
	if _, err := s.pool.Exec(ctx, query,
		string(h.FactorName), h.SourceKind, h.Hypothesis, nullableString(h.Reference),
	); err != nil {
		return fmt.Errorf("failed to save factor hypothesis: %w", err)
	}
	return nil
}

// GetFactorHypothesis 取一个因子的假设来源。两个返回都为空表示「没记录过」
// —— 调用方要如实上报，不要拿 domain.BuiltinFactorHypotheses 悄悄兜底。
func (s *Store) GetFactorHypothesis(ctx context.Context, name domain.FactorType) (*domain.FactorHypothesis, error) {
	const query = `
		SELECT factor_name, source_kind, hypothesis, COALESCE(reference, ''), updated_at
		FROM ai.factor_hypothesis WHERE factor_name = $1
	`
	h := &domain.FactorHypothesis{}
	var ref string
	var updated time.Time
	err := s.pool.QueryRow(ctx, query, string(name)).Scan(
		&h.FactorName, &h.SourceKind, &h.Hypothesis, &ref, &updated)
	if err != nil {
		if err == notFoundErr {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get factor hypothesis: %w", err)
	}
	h.Reference = ref
	return h, nil
}

// ListFactorHypotheses 列出全部已记录的因子假设（按因子名排序）。
func (s *Store) ListFactorHypotheses(ctx context.Context) ([]domain.FactorHypothesis, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT factor_name, source_kind, hypothesis, COALESCE(reference, '')
		FROM ai.factor_hypothesis ORDER BY factor_name`)
	if err != nil {
		return nil, fmt.Errorf("failed to list factor hypotheses: %w", err)
	}
	defer rows.Close()

	var out []domain.FactorHypothesis
	for rows.Next() {
		var h domain.FactorHypothesis
		var ref string
		if err := rows.Scan(&h.FactorName, &h.SourceKind, &h.Hypothesis, &ref); err != nil {
			return nil, fmt.Errorf("failed to scan factor hypothesis: %w", err)
		}
		h.Reference = ref
		out = append(out, h)
	}
	return out, rows.Err()
}

// SeedBuiltinFactorHypotheses 把内置因子的假设写进库（P2-3）。只在库里还没
// 有该因子记录时写 —— AI 挖掘链路写过的不覆盖，否则每次启动都会把人写的
// 假设冲掉。双写期它让 ai.factor_hypothesis 与 core 启动时的种子收敛。
func (s *Store) SeedBuiltinFactorHypotheses(ctx context.Context) (int, error) {
	n := 0
	for _, h := range domain.BuiltinFactorHypotheses {
		const query = `
			INSERT INTO ai.factor_hypothesis (factor_name, source_kind, hypothesis, reference)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (factor_name) DO NOTHING
		`
		tag, err := s.pool.Exec(ctx, query,
			string(h.FactorName), h.SourceKind, h.Hypothesis, nullableString(h.Reference))
		if err != nil {
			return n, fmt.Errorf("failed to seed factor hypothesis %s: %w", h.FactorName, err)
		}
		if tag.RowsAffected() > 0 {
			n++
		}
	}
	return n, nil
}
