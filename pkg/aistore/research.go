package aistore

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// 研究域（EQD-P2-1）——core research.profile / conclusion / question 的
// ai.* 自持版，列与 JSONB 形状逐字对照 tellurion:pkg/storage/research.go。
// Citations 原样存 JSONB（{source, dataset, key, as_of, content_hash} + JSON
// Pointer），这一层不重写它。

// ResearchProfile 是一个 ticker 的归档研究画像；Conclusions / Questions 是
// 同一 ticker 的子行。Ticker 是裸 6 位代码（contracts/profile.schema.json
// ^[0-9]{6}$），带交易所后缀的 ts_code 形态只存在于行情侧表。
type ResearchProfile struct {
	Ticker         string     `json:"ticker"`
	Name           string     `json:"name"`
	SchemaVersion  int        `json:"schema_version"`
	SourceFile     *string    `json:"source_file,omitempty"`
	SourceMtime    *time.Time `json:"source_mtime,omitempty"`
	LastResearched *time.Time `json:"last_researched,omitempty"`
	NeedsReview    bool       `json:"needs_review"`
	ReviewReason   *string    `json:"review_reason,omitempty"`
	UpdatedAt      time.Time  `json:"updated_at"`

	Conclusions []ResearchConclusion `json:"conclusions"`
	Questions   []ResearchQuestion   `json:"questions"`
}

// ResearchConclusion 是 ai.research_conclusion 的一行。
type ResearchConclusion struct {
	Ticker          string          `json:"ticker"`
	ID              string          `json:"id"`
	Title           string          `json:"title"`
	Body            *string         `json:"body,omitempty"`
	AsOf            *string         `json:"as_of,omitempty"`
	Confidence      *string         `json:"confidence,omitempty"`
	Status          string          `json:"status"`
	Citations       json.RawMessage `json:"citations"`
	EvidencePointer *string         `json:"evidence_pointer,omitempty"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

// ResearchQuestion 是 ai.research_question 的一行。
type ResearchQuestion struct {
	Ticker   string  `json:"ticker"`
	ID       string  `json:"id"`
	Text     string  `json:"text"`
	Status   string  `json:"status"`
	RaisedAt *string `json:"raised_at,omitempty"`
}

// GetResearchProfile 取一个 ticker 的画像与其结论/问题。ticker 从未被投影时
// 返回 (nil, nil) —— 与 core 语义一致，「没找到」不是错误。子行缺失不是
// 错误：新画像合法地没有结论/问题，两个切片返回空（非 nil）。
func (s *Store) GetResearchProfile(ctx context.Context, ticker string) (*ResearchProfile, error) {
	if ticker == "" {
		return nil, fmt.Errorf("research profile: ticker is required")
	}

	profile := &ResearchProfile{}
	err := s.pool.QueryRow(ctx, `
		SELECT ticker, name, schema_version, source_file, source_mtime,
		       last_researched, needs_review, review_reason, updated_at
		FROM ai.research_profile WHERE ticker = $1
	`, ticker).Scan(
		&profile.Ticker, &profile.Name, &profile.SchemaVersion,
		&profile.SourceFile, &profile.SourceMtime, &profile.LastResearched,
		&profile.NeedsReview, &profile.ReviewReason, &profile.UpdatedAt,
	)
	if err != nil {
		if err == notFoundErr {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get research profile: %w", err)
	}

	conclusions, err := s.listResearchConclusions(ctx, ticker)
	if err != nil {
		return nil, err
	}
	questions, err := s.listResearchQuestions(ctx, ticker)
	if err != nil {
		return nil, err
	}
	profile.Conclusions = conclusions
	profile.Questions = questions
	return profile, nil
}

// listResearchConclusions 返回 ticker 的全部结论（按 conclusion_id 排序）。
// 被取代的行原样返回（带 status）—— 归档是事实源，消费方决定怎么处置。
func (s *Store) listResearchConclusions(ctx context.Context, ticker string) ([]ResearchConclusion, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT ticker, conclusion_id, title, body, as_of, confidence,
		       status, citations, evidence_pointer, updated_at
		FROM ai.research_conclusion WHERE ticker = $1
		ORDER BY conclusion_id
	`, ticker)
	if err != nil {
		return nil, fmt.Errorf("failed to list research conclusions: %w", err)
	}
	defer rows.Close()

	out := make([]ResearchConclusion, 0)
	for rows.Next() {
		var c ResearchConclusion
		var citations []byte
		if err := rows.Scan(
			&c.Ticker, &c.ID, &c.Title, &c.Body, &c.AsOf, &c.Confidence,
			&c.Status, &citations, &c.EvidencePointer, &c.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan research conclusion: %w", err)
		}
		c.Citations = json.RawMessage(citations)
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to list research conclusions: %w", err)
	}
	return out, nil
}

func (s *Store) listResearchQuestions(ctx context.Context, ticker string) ([]ResearchQuestion, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT ticker, question_id, text, status, raised_at
		FROM ai.research_question WHERE ticker = $1
		ORDER BY question_id
	`, ticker)
	if err != nil {
		return nil, fmt.Errorf("failed to list research questions: %w", err)
	}
	defer rows.Close()

	out := make([]ResearchQuestion, 0)
	for rows.Next() {
		var q ResearchQuestion
		if err := rows.Scan(
			&q.Ticker, &q.ID, &q.Text, &q.Status, &q.RaisedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan research question: %w", err)
		}
		out = append(out, q)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to list research questions: %w", err)
	}
	return out, nil
}
