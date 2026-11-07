package aistore

import (
	"context"
	"encoding/json"
	"fmt"
)

// 因子候选（DBT-18）—— mining 生成器的产出一律落 ai.factor_genes。
//
// 这张表是 QT-17 从 core public.factor_genes 逐列搬来的，本文件只补本仓
// 的写入口；不碰 core 的业务表，也不缩表结构（列集仍以 schema.go 为准）。
//
// 写入口刻意做成**整批语义**：候选是「一次生成」的单位，要么全军落地，
// 要么一行都没有。半批落库最坏——它既不能说明这次生成成功，也不能说明
// 失败，事后无法判断那条公式是不是当时的完整结论。

// FactorCandidate 是写 ai.factor_genes 的候选形状。
//
// 只带本仓生成器真正能说清的列：id / name / category / formula 是必填，
// 其余是可空的证据。IC / IR 等指标不在这里填——它们是回测/验证的结论，
// 不是候选生成时的已知量；提前填 0 会被读成「算过且为零」。
type FactorCandidate struct {
	ID          string
	Name        string
	Category    string
	Formula     string
	Description string
	Rationale   string
	ParentIDs   []string
	Generation  int
	Status      string
}

func (c FactorCandidate) validate() error {
	switch {
	case c.ID == "":
		return fmt.Errorf("id is required")
	case c.Name == "":
		return fmt.Errorf("name is required")
	case c.Category == "":
		return fmt.Errorf("category is required")
	case c.Formula == "":
		return fmt.Errorf("formula is required")
	}
	return nil
}

// SaveFactorCandidates 在一个事务里写入整批因子候选。
//
// 两段防线，缺一不可：
//  1. **开事务之前**校验整批 —— 有一条不合格就返回，连库都不连；
//  2. 事务里任何一条失败就回滚 —— 不留下「前三条成功、后两条没写」的半成品。
//
// 重复 ID 是更新而不是插第二条：同一 seed / 同一公式重跑应当收敛到一行。
func (s *Store) SaveFactorCandidates(ctx context.Context, candidates []FactorCandidate) error {
	if len(candidates) == 0 {
		return nil
	}
	for i := range candidates {
		if err := candidates[i].validate(); err != nil {
			return fmt.Errorf("aistore: factor candidate %d: %w", i, err)
		}
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("aistore: begin factor candidate batch: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const query = `
		INSERT INTO ai.factor_genes (
			id, name, category, formula, description, rationale,
			parent_ids, generation, status, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW(), NOW())
		ON CONFLICT (id) DO UPDATE SET
			name        = EXCLUDED.name,
			category    = EXCLUDED.category,
			formula     = EXCLUDED.formula,
			description = EXCLUDED.description,
			rationale   = EXCLUDED.rationale,
			parent_ids  = EXCLUDED.parent_ids,
			generation  = EXCLUDED.generation,
			status      = EXCLUDED.status,
			updated_at  = NOW()
	`

	for i, c := range candidates {
		parentIDs, err := json.Marshal(c.ParentIDs)
		if err != nil {
			return fmt.Errorf("aistore: marshal factor candidate %d parent_ids: %w", i, err)
		}
		status := c.Status
		if status == "" {
			status = "pending"
		}
		if _, err := tx.Exec(ctx, query,
			c.ID, c.Name, c.Category, c.Formula, nullableString(c.Description),
			nullableString(c.Rationale), parentIDs, c.Generation, status,
		); err != nil {
			return fmt.Errorf("aistore: save factor candidate %q: %w", c.ID, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("aistore: commit factor candidate batch: %w", err)
	}
	return nil
}
