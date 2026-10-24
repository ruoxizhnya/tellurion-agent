package main

// ai-service（cmd/ai）—— AI 实验员侧的进程边界（ADR-027 §5 第 8 步的
// 前置切片，AI 拆仓阶段 1）。
//
// 职责：承载 Copilot / pipeline / explore / tools 四族 handler 与 Tools
// Registry 的装配，让「试什么」（AI 领域）住进自己的进程；analysis-service
// 对这四族路由做纯反代（cmd/analysis/handlers_ai_proxy.go），前端与
// openapi 契约零改动。
//
// 依赖方向（拆仓后的不变量，repoguard/ai_boundary 盯）：QT-17 S3-d 起本进程
// 对 core **零编译依赖**（bootstrap / observability 已换 kit 或本仓自持），
// 只经 HTTP 调 analysis-service；core 侧对 pkg/ai 的引用早在阶段 2 归零。
//
// QT-17 S3-c-3 起本进程**不再装配回测引擎**：回测 / walk-forward / 表达式
// 校验都走 core analysis-service 的 HTTP 粗粒度端点，"回测行为对等"由
// 「同一个 core 进程执行」直接保证，而不是靠两份引擎装配逐项对齐。

import (
	"context"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	kitauth "github.com/ruoxizhnya/tellurion-kit/auth"
	kitdb "github.com/ruoxizhnya/tellurion-kit/db"
	kithttpserver "github.com/ruoxizhnya/tellurion-kit/httpserver"
	kitlogging "github.com/ruoxizhnya/tellurion-kit/logging"
	kitobservability "github.com/ruoxizhnya/tellurion-kit/observability"
	"github.com/spf13/viper"
	"tellurion-agent/pkg/ai/client"
	"tellurion-agent/pkg/ai/gene_pool"
	"tellurion-agent/pkg/ai/pipeline"
	"tellurion-agent/pkg/aistore"
)

func main() {
	// S3-d：配置 / 日志 / 指标 / 生命周期全部本仓自持（core bootstrap 退场）。
	preLogger := zerolog.New(os.Stdout).With().Timestamp().Logger()
	v, err := loadConfig("config/ai-service.yaml")
	if err != nil {
		preLogger.Fatal().Err(err).Msg("Failed to read config file")
	}
	kitlogging.Init(v.GetString("logging.level"), v.GetString("logging.format"))
	logger := kitlogging.Logger
	m := initMetrics(logger)

	// QT-17 ②：实验/研究/假设域的自有 store（ai schema，幂等迁移 + 内置
	// 假设种子）。DSN 与 core store 同源同键（database.*），只是落自己的表；
	// 连接能力用 kit（D10），不借 core 的 storage.BuildDSN。
	aiDSN, err := kitdb.BuildDSN(kitdb.Config{
		URL:      v.GetString("database.url"),
		Host:     v.GetString("database.host"),
		Port:     v.GetInt("database.port"),
		User:     v.GetString("database.user"),
		Password: v.GetString("database.password"),
		Name:     v.GetString("database.database"),
		SSLMode:  v.GetString("database.sslmode"),
	})
	if err != nil {
		logger.Fatal().Err(err).Msg("invalid ai store database configuration")
	}
	aiStore, err := aistore.NewStore(context.Background(), aiDSN, kitdb.DefaultPoolOptions())
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to initialize ai store")
	}
	defer aiStore.Close()

	// QT-17 S3-b：本进程只消费 token claims（kit/auth 解析 + 本仓角色常量），
	// 不再经 bootstrap.InitAuth 借 core 连接池读 users 表。用户库留在 core。
	authSvc := initTokenService(v, logger)

	// ── 回测 / walk-forward 只走 HTTP（QT-17 S3-c-3）───────────────
	//
	// 本进程不再装配 backtest.Engine / WalkForwardEngine：逐 bar 执行留 core，
	// agent 一次请求覆盖整段区间（D24 粗粒度动作）。凭据按入站请求转发 ——
	// 网关把用户的 Authorization 交给本进程，中间件放进 ctx，客户端只转发、
	// 不签发。本地 loopback + 无凭证时由 WithInsecureLoopback 放行；其余
	// 缺凭证的场景 fail-closed。
	analysisURL := analysisAPIURL(v)
	btClient := client.NewBacktestClient(
		analysisURL,
		client.WithTokenFromContext(),
		client.WithInsecureLoopback(),
	)
	backtestRunner := NewHTTPBacktestRunner(btClient)
	wfRunner := NewHTTPWalkForwardRunner(btClient)

	copilotService := BuildCopilot(v, backtestRunner, logger)

	factorPool := gene_pool.NewFactorPool(aiStore.Pool())
	strategyPool := gene_pool.NewStrategyPool(aiStore.Pool())

	// factor.compute / factor.evaluate / compute_factor_ic 经 HTTP 打
	// analysis-service 的 /api/factor/*（只在那里注册）。
	toolsRegistry := BuildToolsRegistry(
		v,
		analysisURL,
		backtestRunner,
		wfRunner,
		factorPool,
		strategyPool,
		aiStore,
		aiStore,
		logger,
	)

	applyGinMode(v, logger)
	router := buildRouter(authSvc, v, logger)

	// 健康探针（local-stack netstat 断言 + compose healthcheck 消费）。
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"service": "ai-service",
			"status":  "ok",
		})
	})
	router.GET("/metrics", kitobservability.Handler(m))

	// ── 四族 AI 路由（自 analysis 迁入，路径不变）────────────────────
	registerCopilotRoutes(router, copilotService, backtestRunner)

	// P1-1b：实验日志落点。QT-17 ④ 已**停写** legacy public.experiments，
	// 写路径只剩本仓自持的 ai.experiments —— aistore.Store 同时满足
	// pipeline.ExperimentSink 与 explore 的 verdictSink，不再需要双写适配器。
	// 显式判空而非直接传 —— 具体指针的 nil 塞进接口会变成非 nil 接口值
	// （P0-5 同款坑）。
	var expSink pipeline.ExperimentSink
	if aiStore != nil {
		expSink = aiStore
	}
	registerPipelineRoutes(router, backtestRunner, expSink)
	// QT-17 ③：上市窗口改走 core 的 QT-67 只读端点（D24 第 7 类），
	// 不再 import core 的 pkg/storage。
	registerExploreRoutes(router, backtestRunner, expSink, client.NewReferenceDataClient(analysisURL))

	// AUD-02 (ODR-065 H5)：WithToolsAuth 按工具的审计副作用类做 per-tool
	// RBAC —— save_factor / save_strategy 要 trader-or-admin，其余未分类
	// 一律 admin（fail-closed）。
	NewToolsHandler(toolsRegistry, logger, WithToolsAuth(authSvc)).RegisterRoutes(router)

	srv := startHTTPServer(router, v, logger, "AI Service (ai-service)")

	sig := waitForShutdown()
	logger.Info().Str("signal", sig.String()).Msg("Shutdown signal received; beginning graceful drain")
	// 本进程不持有 core 的 job service / alert manager / store：
	// gracefulShutdown 只负责 HTTP 优雅退出，aiStore 由 defer 自行收尾。
	gracefulShutdown(srv, logger)
}

// analysisAPIURL 解析 core analysis-service 的基址
// （ai_service.factor_api_url，缺省 :8085）。因子工具与 QT-67 的
// 日历/上市窗口只读动作共用同一个服务；配置键名是历史沿用，不再只指因子。
func analysisAPIURL(v *viper.Viper) string {
	if u := v.GetString("ai_service.factor_api_url"); u != "" {
		return u
	}
	return "http://localhost:8085"
}

// aiPublicPaths 是免鉴权路径。与 core 的 auth.PublicPaths() 逐字一致：
// 本进程并不注册 /api/auth/* 这些路由，但反代/探针可能把请求打到这里，
// 401 与 404 的区别会影响前端的鉴权姿势判断，故保持同一张表。
var aiPublicPaths = []string{
	"/api/auth/login",
	"/api/auth/refresh",
	"/api/auth/status",
	"/api/auth/bootstrap",
	"/health",
	"/api/health",
	"/metrics",
}

// initTokenService 构造 kit 的 token 服务，并执行本仓的启动门
// （`decideAuthStartup`：没有密钥就拒绝启动，唯一豁免是显式声明的
// 本地/容器回环形态）。
//
// 与旧 `bootstrap.InitAuth` 的差别（S3-b 有意为之）：不持有连接池、不读写
// users、不写 audit_logs —— 本进程只验证 token 签名与 claims。
func initTokenService(v *viper.Viper, logger zerolog.Logger) *kitauth.TokenService {
	d := decideAuthStartup(
		v.GetString("auth.jwt_secret"),
		v.GetBool("auth.allow_insecure"),
		v.GetString("server.host"),
		v.GetString("auth.insecure_exposure"),
	)
	if d.Refuse {
		logger.Fatal().Msg(d.Reason)
	}

	svc := kitauth.NewTokenService(kitauth.Config{
		JWTSecret:       d.Secret,
		AccessTokenTTL:  v.GetDuration("auth.access_token_ttl"),
		RefreshTokenTTL: v.GetDuration("auth.refresh_token_ttl"),
		Issuer:          v.GetString("auth.issuer"),
		PublicPaths:     aiPublicPaths,
	})
	if svc.Enabled() {
		logger.Info().
			Int("access_ttl_sec", int(svc.AccessTTL().Seconds())).
			Msg("auth: JWT enabled (token claims only; users live in core)")
	} else {
		logger.Warn().
			Msg("auth: INSECURE open-access mode — NO authentication, loopback only, do not use outside local dev")
	}
	return svc
}

// buildRouter 组装 HTTP 中间件链：recovery、CORS、限流、请求日志、鉴权。
//
// 为什么本仓自持而不是继续用 `bootstrap.BuildRouter`：后者形参是 core 的
// `*auth.Service`（带连接池、audit 写库）。S3-d 起路由生命周期与限流/回环门
// 全部由本仓持有，kit 只提供 CORS 与错误响应。
func buildRouter(authSvc *kitauth.TokenService, v *viper.Viper, logger zerolog.Logger) *gin.Engine {
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(kithttpserver.CORS(kithttpserver.AllowedOrigins(v)))
	router.Use(newRateLimiter(rateLimitPerMinute(v), time.Minute).Middleware())
	router.Use(requestLogger(logger))
	if authSvc.Enabled() {
		router.Use(authSvc.Middleware())
		// 回测 / walk-forward / 表达式校验要打 core 的变更端点；以调用者
		// 本人身份转发（本进程不签发 token）。显式放进请求 ctx，异步链路
		// 由调用方用 context.WithoutCancel 保留值。
		router.Use(forwardBearerToken())
	} else if isLoopbackHost(v.GetString("server.host")) {
		// insecure 模式逐请求回环门：启动期只判绑定网卡，同机进程 / 端口
		// 转发仍能打到管理面，故运行期还要判来源（fail-closed）。
		router.Use(loopbackOnly())
	}
	return router
}

// forwardBearerToken 把入站 `Authorization: Bearer <jwt>` 放进请求 ctx，
// 供 client.BacktestClient（WithTokenFromContext）转发给 core。
//
// 只放值、不做鉴权判断：鉴权已由 authSvc.Middleware 完成；这里缺失 token
// 也不拒绝请求（工具/只读路径可能根本不打 core），真正需要凭证的调用会在
// 客户端 fail-closed。
func forwardBearerToken() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := bearerTokenFromHeader(c.GetHeader("Authorization"))
		if token != "" {
			c.Request = c.Request.WithContext(
				client.WithBearerTokenInContext(c.Request.Context(), token))
		}
		c.Next()
	}
}
