package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/ruoxizhnya/tellurion-contracts/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"tellurion-agent/pkg/ai/adoption"
	"tellurion-agent/pkg/ai/pipeline"
	"tellurion-agent/pkg/aistore"
)

// stubBacktestRunner is a test double implementing pipeline.BacktestRunner.
// It records that RunBacktest was invoked, proving the handler wires the
// runner through to the pipeline instead of passing nil.
type stubBacktestRunner struct {
	called       bool
	strategyName string
}

func (s *stubBacktestRunner) RunBacktest(ctx context.Context, strategyName string, stockPool []string, startDate, endDate string) (*domain.BacktestResult, error) {
	s.called = true
	s.strategyName = strategyName
	return &domain.BacktestResult{}, nil
}

// TestNewPipelineHandler_DefaultHasNoRunner verifies backward compatibility:
// when no runner option is supplied, handler.runner stays nil and the pipeline
// will skip the backtest stage (the legacy behaviour).
func TestNewPipelineHandler_DefaultHasNoRunner(t *testing.T) {
	handler := NewPipelineHandler()
	assert.Nil(t, handler.runner, "default handler should have nil runner for backward compat")
}

// TestNewPipelineHandler_WithRunner verifies the S7-P0-1 fix: the handler
// must accept a BacktestRunner dependency via functional option so that
// main.go can inject copilotRunner instead of leaving it nil.
//
// Before the fix, NewPipelineHandler accepted no arguments and RunPipeline
// hardcoded nil as the runner, causing Stage 5 (backtest) to be silently
// skipped on every request — the pipeline was never end-to-end runnable.
func TestNewPipelineHandler_WithRunner(t *testing.T) {
	runner := &stubBacktestRunner{}
	handler := NewPipelineHandler(WithBacktestRunner(runner))

	require.NotNil(t, handler.runner, "handler must hold the injected runner")
	assert.Same(t, runner, handler.runner, "handler.runner must be the exact instance injected")
}

// countingSink 记录实验日志被写入了几次。
type countingSink struct {
	inserts   int
	completed int
}

func (c *countingSink) InsertExperiment(ctx context.Context, e *aistore.Experiment) (int64, error) {
	c.inserts++
	return int64(c.inserts), nil
}

func (c *countingSink) UpdateExperiment(ctx context.Context, id int64, u aistore.ExperimentUpdate) error {
	return nil
}

func (c *countingSink) CompleteExperiment(ctx context.Context, id int64, m *aistore.ExperimentMetrics, errMsg string) error {
	c.completed++
	return nil
}

// TestNewPipelineHandler_WiresExperimentSink 锁的是 P1-1b 的最后一环：
//
// pipeline 有能力记日志还不够 —— **服务启动时必须真的把 sink 接上**。
// 少了这一步，线上跑一百次实验，库里依然一行都没有，而过拟合检测完全
// 建立在「试了多少次」这个数字上。
func TestNewPipelineHandler_WiresExperimentSink(t *testing.T) {
	sink := &countingSink{}
	h := NewPipelineHandler(WithExperimentSink(sink))

	_, err := h.pipeline.Execute(context.Background(), "做一个动量策略", nil)
	require.NoError(t, err)

	assert.Equal(t, 1, sink.inserts, "注入 sink 后，跑一次必须落一行日志")
	assert.Equal(t, 1, sink.completed, "跑完要收尾，不能永远 running")
}

// TestNewPipelineHandler_NoSinkStillRuns：不注入 sink 时链路照跑 ——
// 日志是观测设施，不是硬依赖。
func TestNewPipelineHandler_NoSinkStillRuns(t *testing.T) {
	h := NewPipelineHandler()

	_, err := h.pipeline.Execute(context.Background(), "做一个动量策略", nil)
	require.NoError(t, err)
}

// TestNewPipelineHandler_WithRunner_NilStaysNil verifies that explicitly
// passing a nil runner via the option is equivalent to not passing it.
func TestNewPipelineHandler_WithRunner_NilStaysNil(t *testing.T) {
	handler := NewPipelineHandler(WithBacktestRunner(nil))
	assert.Nil(t, handler.runner)
}

// TestPipelineHandler_ImplementsRunnerContract is a compile-time check that
// stubBacktestRunner satisfies the pipeline.BacktestRunner interface.
func TestPipelineHandler_ImplementsRunnerContract(t *testing.T) {
	var _ pipeline.BacktestRunner = (*stubBacktestRunner)(nil)
}

// fakeExperimentCounter 是读面的固定返回替身。
type fakeExperimentCounter struct {
	count int64
	err   error
}

func (f fakeExperimentCounter) CountExperiments(context.Context) (int64, error) {
	return f.count, f.err
}

func pipelineCountRouter(h *PipelineHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h.RegisterPipelineRoutes(r.Group("/api"))
	return r
}

// TestGetExperimentCount_Wired 锁读面：计数原样返回，不从进程内状态猜。
func TestGetExperimentCount_Wired(t *testing.T) {
	r := pipelineCountRouter(NewPipelineHandler(WithExperimentCounter(fakeExperimentCounter{count: 42})))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/pipeline/experiments/count", nil))

	assert.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"count":42}`, w.Body.String())
}

// TestGetExperimentCount_NotWired503：没接计数来源时必须显式失败，
// 而不是返回一个看起来正常的 0（那是把「没查」伪装成「没跑过」）。
func TestGetExperimentCount_NotWired503(t *testing.T) {
	r := pipelineCountRouter(NewPipelineHandler())

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/pipeline/experiments/count", nil))

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

// ── intervention / adoption ledger ───────────────────────────────────

type fakeAdoptionLedger struct {
	entries   []aistore.AdoptionDecision
	inserted  []aistore.AdoptionDecision
	insertErr error
}

func (f *fakeAdoptionLedger) InsertAdoptionDecision(_ context.Context, d aistore.AdoptionDecision) (int64, error) {
	if f.insertErr != nil {
		return 0, f.insertErr
	}
	f.inserted = append(f.inserted, d)
	return int64(len(f.inserted)), nil
}

func (f *fakeAdoptionLedger) ListAdoptionLedger(context.Context) ([]aistore.AdoptionDecision, error) {
	return f.entries, nil
}

func postAdoption(t *testing.T, r *gin.Engine, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/pipeline/adoption", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestRecordAdoptionDecision_Created(t *testing.T) {
	ledger := &fakeAdoptionLedger{}
	r := pipelineCountRouter(NewPipelineHandler(WithAdoptionLedger(ledger)))

	w := postAdoption(t, r, `{"proposal_id":"p1","actor":"alice","intervention":"lookback 20->60","adopted":true,"claimed_confidence":0.6,"outcome_hit":true,"result_ref":"exp:12"}`)

	require.Equal(t, http.StatusCreated, w.Code, "body=%s", w.Body.String())
	require.Len(t, ledger.inserted, 1)
	d := ledger.inserted[0]
	assert.Equal(t, "p1", d.ProposalID)
	assert.Equal(t, "alice", d.Actor)
	assert.Equal(t, "lookback 20->60", d.Intervention)
	assert.True(t, d.Adopted)
	assert.InDelta(t, 0.6, d.ClaimedConfidence, 1e-9)
	require.NotNil(t, d.OutcomeHit)
	assert.True(t, *d.OutcomeHit)
}

func TestRecordAdoptionDecision_RejectsMissingDecision(t *testing.T) {
	ledger := &fakeAdoptionLedger{}
	r := pipelineCountRouter(NewPipelineHandler(WithAdoptionLedger(ledger)))

	// 没写 adopted：不能被当成「默认否决」写进台账。
	w := postAdoption(t, r, `{"proposal_id":"p1","actor":"alice","claimed_confidence":0.6}`)
	assert.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())
	assert.Empty(t, ledger.inserted)

	// 置信度越界同样拒收 —— 台账的基准不能是 1.2 这种数。
	w = postAdoption(t, r, `{"proposal_id":"p2","actor":"alice","adopted":true,"claimed_confidence":1.2}`)
	assert.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())
	assert.Empty(t, ledger.inserted)
}

func TestRecordAdoptionDecision_Duplicate409(t *testing.T) {
	ledger := &fakeAdoptionLedger{insertErr: aistore.ErrDuplicateAdoption}
	r := pipelineCountRouter(NewPipelineHandler(WithAdoptionLedger(ledger)))

	w := postAdoption(t, r, `{"proposal_id":"p1","actor":"alice","adopted":true,"claimed_confidence":0.6}`)
	assert.Equal(t, http.StatusConflict, w.Code, "body=%s", w.Body.String())
}

func TestGetCalibration_ReportsHoldoutSeparately(t *testing.T) {
	no, yes := false, true
	ledger := &fakeAdoptionLedger{entries: []aistore.AdoptionDecision{
		{ProposalID: "p1", ClaimedConfidence: 0.6, OutcomeHit: &yes},
		{ProposalID: "p2", ClaimedConfidence: 0.6, OutcomeHit: &no},
		{ProposalID: "h1", ClaimedConfidence: 0.9, IsHoldout: true, OutcomeHit: &yes},
	}}
	r := pipelineCountRouter(NewPipelineHandler(WithAdoptionLedger(ledger)))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/pipeline/calibration", nil))

	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	var report adoption.Report
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &report))
	assert.Equal(t, 2, report.SampleSize)
	assert.InDelta(t, 0.5, report.HitRate, 1e-9)
	assert.InDelta(t, 0.6, report.ClaimedRate, 1e-9)
	assert.InDelta(t, 0.1, report.CalibrationError, 1e-9)
	assert.InDelta(t, 0.26, report.BrierScore, 1e-9)
	assert.Equal(t, 0, report.Pending)
	assert.Equal(t, 1, report.Holdout.SampleSize)
	assert.InDelta(t, 1.0, report.Holdout.HitRate, 1e-9)
}

func TestGetCalibration_NotWired503(t *testing.T) {
	r := pipelineCountRouter(NewPipelineHandler())

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/pipeline/calibration", nil))

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}
