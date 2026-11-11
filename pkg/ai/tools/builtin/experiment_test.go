package builtin

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"tellurion-agent/pkg/aistore"
)

// 回测 / 走查工具写实验行（AGENT-EXPERIMENT-COUNT-SURFACE）。
//
// 这两条腿锁的是：工具不是「只跑不记账」的旁路。少了行，Hermes 直调工具的
// 试错就不进累计计数，统计维的分母会被算小。

type completedExperiment struct {
	id     int64
	metrics *aistore.ExperimentMetrics
	errMsg  string
}

type fakeExperimentRecorder struct {
	inserts   []aistore.Experiment
	completes []completedExperiment
	insertErr error
}

func (f *fakeExperimentRecorder) InsertExperiment(_ context.Context, e *aistore.Experiment) (int64, error) {
	f.inserts = append(f.inserts, *e)
	if f.insertErr != nil {
		return 0, f.insertErr
	}
	return int64(len(f.inserts)), nil
}

func (f *fakeExperimentRecorder) CompleteExperiment(_ context.Context, id int64, m *aistore.ExperimentMetrics, errMsg string) error {
	f.completes = append(f.completes, completedExperiment{id: id, metrics: m, errMsg: errMsg})
	return nil
}

func backtestArgs() map[string]interface{} {
	return map[string]interface{}{
		"strategy_name": "momentum",
		"stock_pool":    []string{"000001.SZ", "600000.SH"},
		"start_date":    "2022-01-01",
		"end_date":      "2023-01-01",
	}
}

func TestBacktestTool_RecordsExperimentRow(t *testing.T) {
	rec := &fakeExperimentRecorder{}
	tool := NewBacktestTool(&mockRunner{}).WithExperimentRecorder(rec)

	_, err := tool.Execute(context.Background(), backtestArgs())
	require.NoError(t, err)

	require.Len(t, rec.inserts, 1, "每次成功回测都必须先落一行 running")
	ins := rec.inserts[0]
	assert.Contains(t, ins.RunID, "tool-", "工具直调要有独立 run 坐标")
	assert.Equal(t, "backtest.run", ins.Hypothesis)
	assert.Equal(t, "momentum", ins.StrategyName)
	assert.Equal(t, aistore.DatasetSplitTest, ins.DatasetSplit)
	assert.Equal(t, "2022-01-01", ins.Params["start_date"])

	require.Len(t, rec.completes, 1, "落行之后必须收尾，否则留下永远 running 的脏行")
	assert.Equal(t, int64(1), rec.completes[0].id)
	require.NotNil(t, rec.completes[0].metrics)
	assert.Equal(t, 5, rec.completes[0].metrics.TotalTrades)
}

func TestBacktestTool_RecordsFailedExperimentRow(t *testing.T) {
	rec := &fakeExperimentRecorder{}
	tool := NewBacktestTool(&mockRunner{err: errors.New("data missing")}).
		WithExperimentRecorder(rec)

	_, err := tool.Execute(context.Background(), backtestArgs())
	require.Error(t, err)

	require.Len(t, rec.inserts, 1, "失败的尝试同样是一次实验，必须留行")
	require.Len(t, rec.completes, 1)
	assert.Nil(t, rec.completes[0].metrics, "失败行不该带 metrics")
	assert.Contains(t, rec.completes[0].errMsg, "data missing")
}

func TestWalkForwardTool_RecordsExperimentRow(t *testing.T) {
	rec := &fakeExperimentRecorder{}
	tool := NewWalkForwardValidateTool(&mockWFRunner{}).WithExperimentRecorder(rec)

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"strategy_name": "momentum",
		"stock_pool":    []string{"000001.SZ"},
		"start_date":    "2022-01-01",
		"end_date":      "2023-01-01",
	})
	require.NoError(t, err)

	require.Len(t, rec.inserts, 1)
	assert.Equal(t, "walk_forward_validate", rec.inserts[0].Hypothesis)
	assert.Equal(t, aistore.DatasetSplitTest, rec.inserts[0].DatasetSplit)
	require.Len(t, rec.completes, 1)
	require.NotNil(t, rec.completes[0].metrics)
	assert.InDelta(t, 0.9, rec.completes[0].metrics.SharpeRatio, 1e-9, "走查记录 OOS Sharpe")
}
