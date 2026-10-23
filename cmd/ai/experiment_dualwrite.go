package main

import (
	"context"
	"encoding/json"
	"sync"

	"tellurion-agent/pkg/ai/pipeline"
	"tellurion-agent/pkg/aistore"
	"tellurion/pkg/storage"
)

// 双写期适配器（QT-17 ④）：实验日志同时写 core 旧表（public.experiments）
// 与 ai.experiments，两边行数/内容可对拍；对拍一致后才切读、停写。
//
// 这是**过渡态**，与 replace tellurion 同寿：停写（core 旧表不再增长）之后
// 本文件整体删除，写路径只剩 aistore。登记在 internal/repoguard 的 storage
// import 白名单里，删除时白名单同步收口。

// dualWriteSink 把一次实验日志写进两张表。语义：
//   - 两边都写，任何一边失败都不阻断另一边（pipeline 会把错误记进日志，
//     「库里没行」不能静默）；
//   - 返回第一条错误，旧表优先 —— 它是今天的既定行为，读方还在 core 侧。
//
// id 对齐：两张表各自 BIGSERIAL 发号，id 不同。pipeline 全程只持有**一个**
// int64（openExperiment 返回值 → Update/Complete/RecordVerdict），所以适配器
// 记住 legacyID → aiID 的映射，后续补写/收尾/裁决两张表都能跟上。映射在进程
// 内存里，重启即失：重启前没跑完的尝试，ai 侧那行会停在 running 只带 intent
// 参数 —— 对拍测试正是抓这种差异的地方，不是静默接受它。
type dualWriteSink struct {
	legacy *storage.PostgresStore
	ai     *aistore.Store

	mu    sync.Mutex
	aiIDs map[int64]int64
}

var _ pipeline.ExperimentSink = (*dualWriteSink)(nil)

// verdictSink 定义在 handlers_explore.go（未导出）；这条断言钉住双写落点
// 也能接裁决，否则 explore 路由的 sink.(verdictSink) 断言会静默失败。
var _ interface {
	RecordVerdict(ctx context.Context, experimentID int64, verdict json.RawMessage) error
} = (*dualWriteSink)(nil)

func newDualWriteSink(legacy *storage.PostgresStore, ai *aistore.Store) *dualWriteSink {
	return &dualWriteSink{legacy: legacy, ai: ai, aiIDs: map[int64]int64{}}
}

func (d *dualWriteSink) InsertExperiment(ctx context.Context, e *aistore.Experiment) (int64, error) {
	legacyID, legacyErr := d.legacy.InsertExperiment(ctx, toLegacyExperiment(e))
	// ai 侧的 parent_id 有自引用 FK（REFERENCES ai.experiments(id)），而
	// e.ParentID 是 loop 记的 legacy id 空间（sink 返回什么它记什么）——
	// 原样插入必撞 FK。先翻映射；映射丢失（重启前遗留的父代）丢边不断行，
	// 缺的父子关系由对拍暴露，不许拿错空间的 id 硬塞。
	aiExp := *e
	if e.ParentID != nil {
		if aiParent := d.aiID(*e.ParentID); aiParent != 0 {
			aiExp.ParentID = &aiParent
		} else {
			aiExp.ParentID = nil
		}
	}
	aiID, aiErr := d.ai.InsertExperiment(ctx, &aiExp)
	if legacyID != 0 && aiID != 0 && aiErr == nil {
		d.mu.Lock()
		d.aiIDs[legacyID] = aiID
		d.mu.Unlock()
	}
	if legacyErr != nil {
		return legacyID, legacyErr
	}
	if aiErr != nil {
		return aiID, aiErr
	}
	return legacyID, nil
}

func (d *dualWriteSink) aiID(legacyID int64) int64 {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.aiIDs[legacyID]
}

func (d *dualWriteSink) UpdateExperiment(ctx context.Context, id int64, u aistore.ExperimentUpdate) error {
	legacyErr := d.legacy.UpdateExperiment(ctx, id, storage.ExperimentUpdate{
		Params:       u.Params,
		StrategyName: u.StrategyName,
		Expression:   u.Expression,
	})
	aiErr := d.ai.UpdateExperiment(ctx, d.aiID(id), u)
	if legacyErr != nil {
		return legacyErr
	}
	return aiErr
}

func (d *dualWriteSink) CompleteExperiment(ctx context.Context, id int64, m *aistore.ExperimentMetrics, errMsg string) error {
	var legacyMetrics *storage.ExperimentMetrics
	if m != nil {
		legacyMetrics = &storage.ExperimentMetrics{
			SharpeRatio: m.SharpeRatio,
			TotalReturn: m.TotalReturn,
			TotalTrades: m.TotalTrades,
		}
	}
	legacyErr := d.legacy.CompleteExperiment(ctx, id, legacyMetrics, errMsg)
	aiErr := d.ai.CompleteExperiment(ctx, d.aiID(id), m, errMsg)
	if legacyErr != nil {
		return legacyErr
	}
	return aiErr
}

func (d *dualWriteSink) RecordVerdict(ctx context.Context, id int64, verdict json.RawMessage) error {
	legacyErr := d.legacy.RecordVerdict(ctx, id, verdict)
	aiErr := d.ai.RecordVerdict(ctx, d.aiID(id), verdict)
	if legacyErr != nil {
		return legacyErr
	}
	return aiErr
}

// toLegacyExperiment 把 aistore 的 DTO 折算成 core 旧表的形状。两边字段集
// 由 aistore 的 drift 测试（ai 列 ↔ core 列）与 core 的 storage 类型共同钉住；
// 这里只做逐字段搬运，不解释、不补默认值 —— 补默认值是两边各自 Insert 的职责。
func toLegacyExperiment(e *aistore.Experiment) *storage.Experiment {
	var m *storage.ExperimentMetrics
	if e.Metrics != nil {
		m = &storage.ExperimentMetrics{
			SharpeRatio: e.Metrics.SharpeRatio,
			TotalReturn: e.Metrics.TotalReturn,
			TotalTrades: e.Metrics.TotalTrades,
		}
	}
	return &storage.Experiment{
		ID:           e.ID,
		RunID:        e.RunID,
		Seq:          e.Seq,
		ParentID:     e.ParentID,
		Hypothesis:   e.Hypothesis,
		Params:       e.Params,
		DatasetSplit: e.DatasetSplit,
		StrategyName: e.StrategyName,
		Expression:   e.Expression,
		Status:       e.Status,
		Metrics:      m,
		Verdict:      e.Verdict,
		Error:        e.Error,
		CreatedAt:    e.CreatedAt,
		FinishedAt:   e.FinishedAt,
	}
}
