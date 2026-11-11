package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	kitdb "github.com/ruoxizhnya/tellurion-kit/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"tellurion-agent/pkg/ai/adoption"
	"tellurion-agent/pkg/aistore"
)

// TestAdoptionLedger_EndToEnd 是真库 + 真 HTTP 腿：
//
//  1. 通过 POST /api/pipeline/adoption 真写入 3 条校准声 + 1 条 holdout；
//  2. GET /api/pipeline/calibration 算出的命中率与手算一致；
//  3. holdout 只出现在 holdout 报告里，没有混进校准分母。
func TestAdoptionLedger_EndToEnd(t *testing.T) {
	ctx := context.Background()
	dsn := strings.TrimSpace(os.Getenv("TEST_DATABASE_DSN"))
	if dsn == "" {
		t.Skip("TEST_DATABASE_DSN 未设置：跳过真库腿")
	}

	pool, err := kitdb.Connect(ctx, dsn, kitdb.DefaultPoolOptions())
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	require.NoError(t, aistore.Migrate(ctx, pool))
	store := aistore.NewStoreWithPool(pool)

	const prefix = "itest_e2e_adopt_"
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM ai.adoption_ledger WHERE proposal_id LIKE $1", prefix+"%")
	})
	_, _ = pool.Exec(ctx, "DELETE FROM ai.adoption_ledger WHERE proposal_id LIKE $1", prefix+"%")

	router := pipelineCountRouter(NewPipelineHandler(WithAdoptionLedger(store)))

	bodies := []string{
		`{"proposal_id":"itest_e2e_adopt_1","actor":"alice","intervention":"","adopted":true,"claimed_confidence":0.6,"outcome_hit":true,"result_ref":"exp:1"}`,
		`{"proposal_id":"itest_e2e_adopt_2","actor":"alice","intervention":"","adopted":true,"claimed_confidence":0.6,"outcome_hit":true,"result_ref":"exp:2"}`,
		`{"proposal_id":"itest_e2e_adopt_3","actor":"bob","intervention":"reject weak signal","adopted":false,"claimed_confidence":0.6,"outcome_hit":false,"result_ref":"exp:3"}`,
		`{"proposal_id":"itest_e2e_adopt_h1","actor":"bob","intervention":"","adopted":true,"claimed_confidence":0.9,"is_holdout":true,"outcome_hit":true,"result_ref":"exp:4"}`,
	}
	for _, body := range bodies {
		w := postAdoption(t, router, body)
		require.Equal(t, http.StatusCreated, w.Code, "body=%s resp=%s", body, w.Body.String())
	}

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/pipeline/calibration", nil))
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	var report adoption.Report
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &report))
	assert.Equal(t, 3, report.SampleSize, "holdout 不得混进校准集")
	assert.InDelta(t, 2.0/3.0, report.HitRate, 1e-9)
	assert.InDelta(t, 0.6, report.ClaimedRate, 1e-9)
	assert.InDelta(t, 2.0/3.0-0.6, report.CalibrationError, 1e-9)
	assert.Equal(t, 0, report.Pending)
	assert.Equal(t, 1, report.Holdout.SampleSize)
	assert.InDelta(t, 1.0, report.Holdout.HitRate, 1e-9)
}
