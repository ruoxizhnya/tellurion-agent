package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	kitdb "github.com/ruoxizhnya/tellurion-kit/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"tellurion-agent/pkg/ai/tools/builtin"
	"tellurion-agent/pkg/aistore"
)

// TestExperimentCount_PersistsAcrossRestart 是实验计数（AGENT-EXPERIMENT-COUNT-SURFACE）
// 的真库腿：
//
//  1. 用生产写路径（backtest.run 工具 + *aistore.Store）真跑 3 次；
//  2. 计数从 3 次实验行派生；
//  3. 重建连接池（模拟进程重启），计数仍是同一个数；
//  4. 读面 /api/pipeline/experiments/count 返回同一个数。
//
// 破坏方式：撤掉 WithExperimentRecorder（退回「只跑不记」），第 2 步立刻变红。
func TestExperimentCount_PersistsAcrossRestart(t *testing.T) {
	ctx := context.Background()
	dsn := strings.TrimSpace(os.Getenv("TEST_DATABASE_DSN"))
	if dsn == "" {
		t.Skip("TEST_DATABASE_DSN 未设置：跳过真库腿（本地 docker compose 起库后指定 DSN 可跑）")
	}

	pool, err := kitdb.Connect(ctx, dsn, kitdb.DefaultPoolOptions())
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	require.NoError(t, aistore.Migrate(ctx, pool))
	store := aistore.NewStoreWithPool(pool)

	// 只清理本次测试新增的行：记录起点 id，避免动到同库其他数据。
	var maxID int64
	require.NoError(t, pool.QueryRow(ctx, `SELECT COALESCE(max(id), 0) FROM ai.experiments`).Scan(&maxID))
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM ai.experiments WHERE id > $1`, maxID)
	})

	before, err := store.CountExperiments(ctx)
	require.NoError(t, err)

	tool := builtin.NewBacktestTool(&stubBacktestRunner{}).WithExperimentRecorder(store)
	for i := 0; i < 3; i++ {
		_, err := tool.Execute(ctx, map[string]interface{}{
			"strategy_name": "count_itest",
			"stock_pool":    []string{"000001.SZ"},
			"start_date":    "2022-01-01",
			"end_date":      "2022-06-30",
		})
		require.NoError(t, err)
	}

	after, err := store.CountExperiments(ctx)
	require.NoError(t, err)
	assert.Equal(t, before+3, after, "真跑 3 次回测，计数必须正好 +3")

	// ── 模拟重启：新连接池 + 新 Store，没有任何进程内状态可带走 ──
	pool2, err := kitdb.Connect(ctx, dsn, kitdb.DefaultPoolOptions())
	require.NoError(t, err)
	t.Cleanup(pool2.Close)
	restarted := aistore.NewStoreWithPool(pool2)

	afterRestart, err := restarted.CountExperiments(ctx)
	require.NoError(t, err)
	assert.Equal(t, after, afterRestart, "计数由实验行派生，重启后不能归零")

	// ── 读面 ──
	gin.SetMode(gin.TestMode)
	h := NewPipelineHandler(WithExperimentCounter(restarted))
	router := gin.New()
	h.RegisterPipelineRoutes(router.Group("/api"))
	req := httptest.NewRequest(http.MethodGet, "/api/pipeline/experiments/count", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	var body struct {
		Count int64 `json:"count"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, afterRestart, body.Count, "读面必须与库里的累计计数一致")
}
