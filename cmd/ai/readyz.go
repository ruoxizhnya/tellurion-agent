package main

// readyz.go —— ai-service 的就绪探针（/readyz、/api/readyz）。
//
// 与 /health 的分工与 core analysis-service 逐字一致：
//
//	/health   存活：进程与 HTTP 栈活着，**不碰数据库**（库抖不该重启进程）。
//	/readyz   就绪：池 ping 通 + ai.* 启动迁移的关键表在位；否则 503。
//
// 就绪探针必须免鉴权且免限流（见 aiPublicPaths / rateLimitExemptPaths）：
// 编排器不持有用户 token，且 429 会被读成「不 ready」而反复摘流量。

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"tellurion-agent/pkg/aistore"
)

// readinessTimeout 是单次就绪判定的预算（与 core 探针同一量级）。
const readinessTimeout = 2 * time.Second

// registerReadiness 注册就绪探针。store 允许为 nil（未接库的运行形态）：
// 按「未就绪」返回 503，而不是 panic。
func registerReadiness(router *gin.Engine, store *aistore.Store, logger zerolog.Logger) {
	handler := func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), readinessTimeout)
		defer cancel()

		if err := store.Ready(ctx); err != nil {
			logger.Warn().Err(err).Msg("readiness check failed")
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status":    "not_ready",
				"service":   "ai-service",
				"checks":    gin.H{"database": err.Error()},
				"timestamp": time.Now().Format(time.RFC3339),
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"status":    "ready",
			"service":   "ai-service",
			"checks":    gin.H{"database": "ok", "schema": "ok"},
			"timestamp": time.Now().Format(time.RFC3339),
		})
	}
	router.GET("/readyz", handler)
	router.GET("/api/readyz", handler)
}
