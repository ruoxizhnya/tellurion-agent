package aistore

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// 实验日志（P1-1）——core public.experiments 的 ai.* 自持版。
//
// 语义逐条对照 tellurion:pkg/storage/experiments.go 搬入（QT-17 ②）：失败也
// 落行、running 是「被叫停」的合法终态、两步写（先落行再补参数）。这一层
// 只做存取，不做判断：不评最优、不排序、不打分。

// 实验状态。running 既是初始态，也是被叫停时的终态 —— 中途叫停的实验永远
// 停在 running，那正是回路断在哪一步的证据，不能被当成脏数据清掉。
const (
	ExperimentStatusRunning   = "running"
	ExperimentStatusCompleted = "completed"
	ExperimentStatusFailed    = "failed"
)

// 数据集划分。一次尝试必须说清自己用的是哪份数据，否则保留期验证无从谈起。
const (
	DatasetSplitTrain = "train"
	DatasetSplitHold  = "hold"
	DatasetSplitTest  = "test"
)

// Experiment 是一次尝试的完整记录。
type Experiment struct {
	ID           int64
	RunID        string
	Seq          int
	ParentID     *int64
	Hypothesis   string
	Params       map[string]any
	DatasetSplit string
	StrategyName string
	Expression   string
	Status       string
	Metrics      *ExperimentMetrics
	// Verdict 是验证器链的裁决（P2-9），原样存 JSONB。类型是 json.RawMessage
	// 而不是具体校验类型：库里要的是一份能读的证据，反序列化由读的那侧决定。
	Verdict    json.RawMessage
	Error      string
	CreatedAt  time.Time
	FinishedAt *time.Time
}

// ExperimentMetrics 是一次尝试的产出指标。
type ExperimentMetrics struct {
	SharpeRatio float64 `json:"sharpe_ratio"`
	TotalReturn float64 `json:"total_return"`
	TotalTrades int     `json:"total_trades"`
}

// ExperimentUpdate 补写「试了什么」。零值字段表示「不改」（SQL 侧 COALESCE）。
type ExperimentUpdate struct {
	Params       map[string]any
	StrategyName string
	Expression   string
}

// InsertExperiment 开一次尝试，返回自增 ID。初始状态恒为 running：「开始了
// 但没结果」必须先落库，否则进程一崩，这次尝试就从未存在过。
func (s *Store) InsertExperiment(ctx context.Context, e *Experiment) (int64, error) {
	params, err := marshalJSONBObject(e.Params)
	if err != nil {
		return 0, fmt.Errorf("failed to encode experiment params: %w", err)
	}
	split := e.DatasetSplit
	if split == "" {
		split = DatasetSplitTrain
	}

	var id int64
	err = s.pool.QueryRow(ctx, `
		INSERT INTO ai.experiments
			(run_id, seq, parent_id, hypothesis, params, dataset_split,
			 strategy_name, expression, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW())
		RETURNING id
	`, e.RunID, e.Seq, e.ParentID, e.Hypothesis, params, split,
		e.StrategyName, e.Expression, ExperimentStatusRunning,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("failed to insert experiment: %w", err)
	}
	return id, nil
}

// UpdateExperiment 把「试了什么」补进已存在的那一行。id 为 0 表示这一轮没有
// 日志落点（例如 sink 写入失败），静默跳过。
func (s *Store) UpdateExperiment(ctx context.Context, id int64, u ExperimentUpdate) error {
	if id == 0 {
		return nil
	}
	var params []byte
	if u.Params != nil {
		b, err := marshalJSONBObject(u.Params)
		if err != nil {
			return fmt.Errorf("failed to encode experiment params: %w", err)
		}
		params = b
	}
	_, err := s.pool.Exec(ctx, `
		UPDATE ai.experiments SET
			params        = COALESCE($2, params),
			strategy_name = COALESCE($3, strategy_name),
			expression    = COALESCE($4, expression)
		WHERE id = $1
	`, id, params, nullableString(u.StrategyName), nullableString(u.Expression))
	if err != nil {
		return fmt.Errorf("failed to update experiment: %w", err)
	}
	return nil
}

// CompleteExperiment 收尾一次尝试：errMsg 非空落 failed（metrics 置空），
// 否则 completed。两者都要落 —— 失败尤其要落。
func (s *Store) CompleteExperiment(ctx context.Context, id int64, m *ExperimentMetrics, errMsg string) error {
	status := ExperimentStatusCompleted
	if errMsg != "" {
		status = ExperimentStatusFailed
		m = nil
	}

	var metrics []byte
	if m != nil {
		b, err := json.Marshal(m)
		if err != nil {
			return fmt.Errorf("failed to encode experiment metrics: %w", err)
		}
		metrics = b
	}

	var errVal *string
	if errMsg != "" {
		errVal = &errMsg
	}

	if _, err := s.pool.Exec(ctx, `
		UPDATE ai.experiments
		SET status = $2, metrics = $3, error_message = $4, finished_at = NOW()
		WHERE id = $1
	`, id, status, metrics, errVal); err != nil {
		return fmt.Errorf("failed to complete experiment: %w", err)
	}
	return nil
}

// RecordVerdict 把验证器链的裁决写回那一行实验（P2-9 接线）。与
// CompleteExperiment 分开：裁决不在 pipeline 里产生，时间上晚于 metrics 落库。
// 裁决写失败不回滚实验本身 —— 它是对已有结果的评论。
func (s *Store) RecordVerdict(ctx context.Context, id int64, verdict json.RawMessage) error {
	if id == 0 || len(verdict) == 0 {
		return nil
	}
	if _, err := s.pool.Exec(ctx,
		`UPDATE ai.experiments SET verdict = $2 WHERE id = $1`, id, verdict); err != nil {
		return fmt.Errorf("failed to record experiment verdict: %w", err)
	}
	return nil
}

func marshalJSONBObject(m map[string]any) ([]byte, error) {
	if m == nil {
		return []byte("{}"), nil
	}
	return json.Marshal(m)
}

func nullableString(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}

// notFoundErr 供调用方区分「没记录」与「查询失败」；两个返回都为空时表示
// 没记录，调用方要如实上报，不要拿内置数据悄悄兜底。
var notFoundErr = pgx.ErrNoRows
