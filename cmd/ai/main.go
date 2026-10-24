package main

// ai-service（cmd/ai）—— AI 实验员侧的进程边界（ADR-027 §5 第 8 步的
// 前置切片，AI 拆仓阶段 1）。
//
// 职责：承载 Copilot / pipeline / explore / tools 四族 handler 与 Tools
// Registry 的装配，让「试什么」（AI 领域）住进自己的进程；analysis-service
// 对这四族路由做纯反代（cmd/analysis/handlers_ai_proxy.go），前端与
// openapi 契约零改动。
//
// 依赖方向（拆仓后的不变量，repoguard/ai_boundary 盯）：本进程 import
// core 的 pkg/*（引擎/存储/风控/表达式）—— 单向；core 侧对 pkg/ai 的
// 引用在阶段 1 仍存在（BuildCopilot / BuildToolsRegistry），
// 阶段 2 本文件与 pkg/ai 一起搬进 tellurion-agent 仓，core 归零。
//
// 回测行为与 analysis-service 的对等性：engine 装配（RiskManager /
// MockTrader / Store / WF 工厂）与 analysis 的 main 逐项一致 ——
// 同一个实验在两个进程跑出不同结果是不可接受的（可复现性）。

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	kitauth "github.com/ruoxizhnya/tellurion-kit/auth"
	kitdb "github.com/ruoxizhnya/tellurion-kit/db"
	kithttpserver "github.com/ruoxizhnya/tellurion-kit/httpserver"
	"github.com/spf13/viper"
	"tellurion-agent/pkg/ai/client"
	"tellurion-agent/pkg/ai/gene_pool"
	"tellurion-agent/pkg/ai/pipeline"
	"tellurion-agent/pkg/aistore"
	"tellurion/pkg/backtest"
	"tellurion/pkg/bootstrap"
	corehttpserver "tellurion/pkg/httpserver"
	"tellurion/pkg/marketdata"
	"tellurion/pkg/observability"
)

func main() {
	logger := bootstrap.InitLogger()
	m := bootstrap.InitMetrics(logger)
	v := bootstrap.LoadConfig(logger, "config/ai-service.yaml")

	// ── 引擎装配（与 analysis-service main 逐项对等）────────────────
	engine, httpProvider := bootstrap.BuildBacktestEngine(v, logger)
	riskManager := bootstrap.BuildRiskManager(v, logger)
	engine.SetRiskManager(riskManager)
	executionTrader := bootstrap.BuildExecutionTrader(v, logger)
	engine.SetLiveTrader(executionTrader)

	store := bootstrap.InitStore(v, logger)
	engine.SetStore(store)

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

	// walk-forward 的每个窗口必须拿到**独立**的 Engine 实例（引擎级缓存
	// 是 per-instance 的，共享单例会让并发窗口互相污染 —— 与 analysis
	// main 同一条注释、同一个坑）。
	newEngine := func() (*backtest.Engine, error) {
		eng, err := backtest.NewEngine(v, httpProvider, logger)
		if err != nil {
			return nil, err
		}
		eng.SetRiskManager(riskManager)
		eng.SetLiveTrader(executionTrader)
		eng.SetStore(store)
		return eng, nil
	}

	pgProvider := marketdata.NewPostgresProvider(store, logger)
	dataAdapter := marketdata.NewDataAdapter(nil, pgProvider, httpProvider, logger)
	wfEngine := backtest.NewWalkForwardEngine(func() (*backtest.Engine, error) {
		eng, err := newEngine()
		if err != nil {
			return nil, err
		}
		// 新引擎必须挂上 dataAdapter，否则多源回退路径不生效（与
		// analysis 的 buildDataServices 同一条注释）。
		eng.SetDataAdapter(dataAdapter)
		eng.SetStore(store)
		return eng, nil
	}, store, logger)
	logger.Info().Msg("Walk-forward engine initialized (tools-side)")

	copilotService, copilotRunner := BuildCopilot(v, engine, logger)

	factorPool := gene_pool.NewFactorPool(aiStore.Pool())
	strategyPool := gene_pool.NewStrategyPool(aiStore.Pool())
	wfRunner := NewWalkForwardEngineAdapter(wfEngine)

	// factor.compute / factor.evaluate / compute_factor_ic 经 HTTP 打
	// analysis-service 的 /api/factor/*（只在那里注册）。
	analysisURL := analysisAPIURL(v)
	toolsRegistry := BuildToolsRegistry(
		v,
		analysisURL,
		copilotRunner,
		wfRunner,
		factorPool,
		strategyPool,
		riskManager,
		aiStore,
		aiStore,
		logger,
	)

	bootstrap.ApplyGinMode(v, logger)
	router := buildRouter(authSvc, v, logger)

	// 健康探针（local-stack netstat 断言 + compose healthcheck 消费）。
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"service": "ai-service",
			"status":  "ok",
		})
	})
	router.GET("/metrics", observability.Handler(m))

	// ── 四族 AI 路由（自 analysis 迁入，路径不变）────────────────────
	registerCopilotRoutes(router, copilotService, copilotRunner)

	// P1-1b：实验日志落点。QT-17 ④ 已**停写** legacy public.experiments，
	// 写路径只剩本仓自持的 ai.experiments —— aistore.Store 同时满足
	// pipeline.ExperimentSink 与 explore 的 verdictSink，不再需要双写适配器。
	// 显式判空而非直接传 —— 具体指针的 nil 塞进接口会变成非 nil 接口值
	// （P0-5 同款坑）。
	var expSink pipeline.ExperimentSink
	if aiStore != nil {
		expSink = aiStore
	}
	registerPipelineRoutes(router, copilotRunner, expSink)
	// QT-17 ③：上市窗口改走 core 的 QT-67 只读端点（D24 第 7 类），
	// 不再 import core 的 pkg/storage。
	registerExploreRoutes(router, copilotRunner, expSink, client.NewReferenceDataClient(analysisURL))

	// AUD-02 (ODR-065 H5)：WithToolsAuth 按工具的审计副作用类做 per-tool
	// RBAC —— save_factor / save_strategy 要 trader-or-admin，其余未分类
	// 一律 admin（fail-closed）。
	NewToolsHandler(toolsRegistry, logger, WithToolsAuth(authSvc)).RegisterRoutes(router)

	srv := bootstrap.StartHTTPServer(router, v, logger, "AI Service (ai-service)")

	sig := bootstrap.WaitForShutdown()
	logger.Info().Str("signal", sig.String()).Msg("Shutdown signal received; beginning graceful drain")
	bootstrap.GracefulShutdown(srv, nil, nil, store, logger)
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

// initTokenService 构造 kit 的 token 服务，并执行 core 的启动门
// （`bootstrap.DecideAuthStartup`：没有密钥就拒绝启动，唯一豁免是显式声明的
// 本地/容器回环形态）。
//
// 与旧 `bootstrap.InitAuth` 的差别（S3-b 有意为之）：不持有连接池、不读写
// users、不写 audit_logs —— 本进程只验证 token 签名与 claims。
func initTokenService(v *viper.Viper, logger zerolog.Logger) *kitauth.TokenService {
	d := bootstrap.DecideAuthStartup(
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
// `*auth.Service`（带连接池、audit 写库），S3-b 换 kit 后类型不再兼容。
// 其余构件仍复用 core bootstrap 的实现（限流 / 请求日志 / 启动门），
// 到 S3-d 随 router 自持一起换 kit。
func buildRouter(authSvc *kitauth.TokenService, v *viper.Viper, logger zerolog.Logger) *gin.Engine {
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(kithttpserver.CORS(kithttpserver.AllowedOrigins(v)))
	router.Use(bootstrap.NewRateLimiter(bootstrap.RateLimitPerMinute(v), time.Minute).Middleware())
	router.Use(bootstrap.RequestLogger(logger))
	if authSvc.Enabled() {
		router.Use(authSvc.Middleware())
	} else if bootstrap.IsLoopbackHost(v.GetString("server.host")) {
		// QT-50 ③-④：insecure 模式逐请求回环门。kit v0.1.0 的 httpserver
		// 还没有 LoopbackOnly，这里直接复用 core 的同一份实现（单一真相；
		// S3-d 把 router 生命周期收进本仓时再决定是否上收 kit）。
		router.Use(corehttpserver.LoopbackOnly())
	}
	return router
}
