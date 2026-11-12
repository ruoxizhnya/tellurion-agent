package aistore

import (
	"context"
	"fmt"
	"time"
)

// 人工标记的类型（G4）。写面只收这两种，别的一律拒收 —— 类型漂移会让
// 「结果流上的标注」变成另一套没人读得懂的词。
const (
	ExploreMarkDerive  = "derive"  // 派生子 run：围绕这次尝试多试
	ExploreMarkPromote = "promote" // 提升权重：这一路值得优先看
)

// ExploreMark 是 explore 结果流上的一次人工轻量标注（AGENT-EXPLORE-INTERVENTIONS / G4）。
//
// 它不是裁决（采纳台账管的是提案采纳），也不参与校准；只把「人对这次尝试
// 做了什么记号」持久留下来。运行态是内存态（D10=b），标注必须比它活得久。
type ExploreMark struct {
	ID           int64     `json:"id"`
	RunID        string    `json:"run_id"`
	AttemptSeq   int       `json:"attempt_seq"`
	Kind         string    `json:"kind"`
	Note         string    `json:"note"`
	Actor        string    `json:"actor"`
	ExperimentID int64     `json:"experiment_id"`
	CreatedAt    time.Time `json:"created_at"`
}

// InsertExploreMark 追加一条标记。返回落库后的行（含 id 与 created_at）。
//
// 同一坐标允许重复标记：标注是流水，不是唯一决策 —— 与 adoption_ledger 的
// proposal_id 唯一是两种语义，不要混。
func (s *Store) InsertExploreMark(ctx context.Context, m ExploreMark) (ExploreMark, error) {
	if m.RunID == "" {
		return ExploreMark{}, fmt.Errorf("explore mark: run_id 不能为空")
	}
	if m.AttemptSeq < 0 {
		return ExploreMark{}, fmt.Errorf("explore mark: attempt_seq 不能为负（收到 %d）", m.AttemptSeq)
	}
	switch m.Kind {
	case ExploreMarkDerive, ExploreMarkPromote:
	default:
		return ExploreMark{}, fmt.Errorf("explore mark: kind 只能是 derive 或 promote（收到 %q）", m.Kind)
	}

	err := s.pool.QueryRow(ctx, `
		INSERT INTO ai.explore_marks (run_id, attempt_seq, kind, note, actor, experiment_id)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at
	`, m.RunID, m.AttemptSeq, m.Kind, m.Note, m.Actor, m.ExperimentID).Scan(&m.ID, &m.CreatedAt)
	if err != nil {
		return ExploreMark{}, fmt.Errorf("explore mark: 写入失败: %w", err)
	}
	return m, nil
}

// ListExploreMarks 返回某轮探索上的全部标记，按落库顺序。
func (s *Store) ListExploreMarks(ctx context.Context, runID string) ([]ExploreMark, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, run_id, attempt_seq, kind, note, actor, experiment_id, created_at
		FROM ai.explore_marks
		WHERE run_id = $1
		ORDER BY id
	`, runID)
	if err != nil {
		return nil, fmt.Errorf("explore mark: 读取失败: %w", err)
	}
	defer rows.Close()

	var out []ExploreMark
	for rows.Next() {
		var m ExploreMark
		if err := rows.Scan(&m.ID, &m.RunID, &m.AttemptSeq, &m.Kind, &m.Note, &m.Actor, &m.ExperimentID, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("explore mark: 扫描失败: %w", err)
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("explore mark: 读取失败: %w", err)
	}
	return out, nil
}
