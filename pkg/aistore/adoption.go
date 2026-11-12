package aistore

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

// ErrDuplicateAdoption 表示同一 proposal_id 已有裁决。
//
// 决策不可回写：重复写入不是「更新」，而是两处互相矛盾的裁决在同一个
// proposal 上共存。宁可拒收，也不让后写的覆盖先写的。
var ErrDuplicateAdoption = errors.New("adoption decision already recorded")

// AdoptionDecision 是 ai.adoption_ledger 的一行：一个提案的一次人工裁决。
type AdoptionDecision struct {
	ID                int64
	ProposalID        string
	Actor             string
	Intervention      string
	Adopted           bool
	ClaimedConfidence float64
	IsHoldout         bool
	// OutcomeHit 为空 = 结果尚未回填；非空才算「已对账」。
	OutcomeHit *bool
	ResultRef  string
	CreatedAt  time.Time
	ResolvedAt *time.Time
}

// InsertAdoptionDecision 落一行台账。proposal_id 冲突时返回 ErrDuplicateAdoption。
func (s *Store) InsertAdoptionDecision(ctx context.Context, d AdoptionDecision) (int64, error) {
	var id int64
	err := s.pool.QueryRow(ctx, `
		INSERT INTO ai.adoption_ledger
			(proposal_id, actor, intervention, adopted, claimed_confidence,
			 is_holdout, outcome_hit, result_ref, resolved_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8,
			CASE WHEN $7::boolean IS NULL THEN NULL ELSE NOW() END)
		RETURNING id
	`, d.ProposalID, d.Actor, d.Intervention, d.Adopted, d.ClaimedConfidence,
		d.IsHoldout, d.OutcomeHit, d.ResultRef).Scan(&id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return 0, fmt.Errorf("adoption decision %q: %w", d.ProposalID, ErrDuplicateAdoption)
		}
		return 0, fmt.Errorf("failed to insert adoption decision %q: %w", d.ProposalID, err)
	}
	return id, nil
}

// CountPendingReviews 返回「待审阅」条数。
//
// 口径：台账里 outcome_hit 为空 = 结果尚未回填、等待人工复盘/对账的行。
// 这是本仓唯一持久、可机器判定的待办队列：探索候选的运行态是纯内存
// （D10=b），且没有到台账的稳定关联键，把猜测的 join 当成「待审阅」会
// 让角标时真时假 —— 宁可只数确定的那部分，也不谎报。
func (s *Store) CountPendingReviews(ctx context.Context) (int, error) {
	var n int
	if err := s.pool.QueryRow(ctx, `
		SELECT count(*) FROM ai.adoption_ledger WHERE outcome_hit IS NULL
	`).Scan(&n); err != nil {
		return 0, fmt.Errorf("failed to count pending reviews: %w", err)
	}
	return n, nil
}

// ListAdoptionLedger 按写入顺序返回全部台账行。读面自己决定怎么分组
// （校准集 / holdout / pending），存储层不替它筛。
func (s *Store) ListAdoptionLedger(ctx context.Context) ([]AdoptionDecision, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, proposal_id, actor, intervention, adopted, claimed_confidence,
		       is_holdout, outcome_hit, result_ref, created_at, resolved_at
		FROM ai.adoption_ledger
		ORDER BY id
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to list adoption ledger: %w", err)
	}
	defer rows.Close()

	out := make([]AdoptionDecision, 0)
	for rows.Next() {
		var d AdoptionDecision
		if err := rows.Scan(
			&d.ID, &d.ProposalID, &d.Actor, &d.Intervention, &d.Adopted,
			&d.ClaimedConfidence, &d.IsHoldout, &d.OutcomeHit, &d.ResultRef,
			&d.CreatedAt, &d.ResolvedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan adoption decision: %w", err)
		}
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to read adoption ledger: %w", err)
	}
	return out, nil
}
