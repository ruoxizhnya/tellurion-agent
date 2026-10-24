package main

// lifecycle.go —— ai-service 自持的进程骨架（QT-17 S3-d）。
//
// 此前这些构件来自 core 的 `pkg/bootstrap`（InitLogger / LoadConfig /
// StartHTTPServer / GracefulShutdown / RateLimiter / …），那是删
// `replace tellurion => ../tellurion` 的最后一类编译依赖。S3-d 把它们
// 收进本仓：能换 kit 的换 kit（logging / observability / httpserver），
// 剩下的服务生命周期由本仓自己写 —— 进程边界不该由被调服务的代码定义。
//
// 与 core 版本的差异都收在这里，如实登记：
//   - HTTP client 的 outbound service 标签仍是 "data"（历史面板口径不变）；
//   - rate limiter 的豁免表与 core 逐字一致（/health、/api/health、
//     /api/auth/status），原因见下方注释；
//   - LoopbackOnly 未上收 kit（kit v0.1.0 没有），这里自带一份同样语义的实现。

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/rs/zerolog"
	kithttpserver "github.com/ruoxizhnya/tellurion-kit/httpserver"
	kitobservability "github.com/ruoxizhnya/tellurion-kit/observability"
	"github.com/spf13/viper"
)

// outboundHTTPClient 是各服务的出站 HTTP client（统一超时 + 可观测 transport）。
// main 在 metrics 就绪后替换它；builders 只读。
//
// Service 标签沿用 "data" —— outbound 指标按 {service="data"} 聚合的既有
// 口径不变（改口径会让历史面板断线）。
var outboundHTTPClient = &http.Client{
	Timeout:   30 * time.Second,
	Transport: &kitobservability.HTTPTransport{Service: "data"},
}

// loadConfig 读取 YAML 配置。路径来自 CONFIG_PATH env，缺省用 defaultPath。
// 密钥只允许从 env 注入（YAML 是入库的）；JWT_SECRET 是历史用法，
// AUTH_JWT_SECRET 与 AutomaticEnv 的键名一致。
func loadConfig(defaultPath string) (*viper.Viper, error) {
	v := viper.New()
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = defaultPath
	}
	v.SetConfigFile(configPath)
	v.SetConfigType("yaml")
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	_ = v.BindEnv("auth.jwt_secret", "JWT_SECRET", "AUTH_JWT_SECRET")
	_ = v.BindEnv("auth.allow_insecure", "AUTH_INSECURE")
	_ = v.BindEnv("auth.insecure_exposure", "AUTH_INSECURE_EXPOSURE")

	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}
	return v, nil
}

// initMetrics 构造四条 ADR-017 §1 核心指标并附加 Go runtime collectors。
func initMetrics(logger zerolog.Logger) *kitobservability.Metrics {
	m := kitobservability.NewMetrics()
	m.Register()
	m.RegisterCollectors(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
	outboundHTTPClient = &http.Client{
		Timeout:   30 * time.Second,
		Transport: &kitobservability.HTTPTransport{Service: "data", Metrics: m},
	}
	logger.Info().Msg("observability: 4 core metrics registered (ADR-017 §1)")
	return m
}

// applyGinMode 按 server.gin_mode 设置 gin 进程级运行模式（启动期恰一次）。
func applyGinMode(v *viper.Viper, logger zerolog.Logger) {
	raw := v.GetString(kithttpserver.ConfigKeyGinMode)
	applied, recognized := kithttpserver.ApplyGinMode(raw)
	if !recognized {
		logger.Warn().
			Str(kithttpserver.ConfigKeyGinMode, raw).
			Str("applied", applied).
			Msg("unrecognized gin mode; falling back to release")
	}
}

// ─── 限流 ────────────────────────────────────────────────────────────

type rateLimiter struct {
	mu       sync.Mutex
	visitors map[string]*visitorInfo
	rate     int
	window   time.Duration
}

type visitorInfo struct {
	count    int
	lastSeen time.Time
}

// newRateLimiter 构造 per-ClientIP 滑动窗口限流器。
func newRateLimiter(rate int, window time.Duration) *rateLimiter {
	rl := &rateLimiter{
		visitors: make(map[string]*visitorInfo),
		rate:     rate,
		window:   window,
	}
	go rl.cleanup()
	return rl
}

// rateLimitExemptPaths 是**永远不许限流**的请求路径。
//
// 判据是「限流它会不会让客户端分不清 429 与 401」：探针拿到 429 会被前端
// 当作未登录，整站弹登录页；`/api/auth/status` 必须豁免。`/api/auth/login`
// 与 `/api/auth/refresh` 不豁免 —— 它们是口令爆破的第一道闸。逐字相等匹配，
// 不做前缀匹配（前缀匹配会被 `..` 之类的写法绕过）。
var rateLimitExemptPaths = []string{
	"/health",
	"/api/health",
	"/api/auth/status",
}

func isRateLimitExempt(path string) bool {
	for _, p := range rateLimitExemptPaths {
		if p == path {
			return true
		}
	}
	return false
}

func (rl *rateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if isRateLimitExempt(c.Request.URL.Path) {
			c.Next()
			return
		}
		if !rl.allow(c.ClientIP()) {
			kithttpserver.Fail(c, http.StatusTooManyRequests, "rate limit exceeded")
			c.Abort()
			return
		}
		c.Next()
	}
}

func (rl *rateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	v, exists := rl.visitors[ip]
	now := time.Now()
	if !exists || now.Sub(v.lastSeen) > rl.window {
		rl.visitors[ip] = &visitorInfo{count: 1, lastSeen: now}
		return true
	}
	v.count++
	v.lastSeen = now
	return v.count <= rl.rate
}

func (rl *rateLimiter) cleanup() {
	for {
		time.Sleep(time.Minute)
		rl.mu.Lock()
		now := time.Now()
		for ip, v := range rl.visitors {
			if now.Sub(v.lastSeen) > rl.window {
				delete(rl.visitors, ip)
			}
		}
		rl.mu.Unlock()
	}
}

// rateLimitPerMinute 返回每 ClientIP 每分钟的限额（rate_limit.per_minute，
// 缺省 100）。
func rateLimitPerMinute(v *viper.Viper) int {
	if n := v.GetInt("rate_limit.per_minute"); n > 0 {
		return n
	}
	return 100
}

// requestLogger 返回逐请求摘要日志中间件（method/path/status/latency）。
func requestLogger(logger zerolog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		logger.Info().
			Str("method", c.Request.Method).
			Str("path", c.Request.URL.Path).
			Int("status", c.Writer.Status()).
			Dur("latency", time.Since(start)).
			Msg("request")
	}
}

// ─── 回环门 ──────────────────────────────────────────────────────────

// loopbackOnly 在 insecure 模式下逐请求拒绝非回环来源（fail-closed）。
func loopbackOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		if isLoopbackRemoteAddr(c.Request.RemoteAddr) {
			c.Next()
			return
		}
		kithttpserver.Fail(c, http.StatusForbidden,
			"insecure mode: non-loopback request source rejected — open-access is loopback-only")
		c.Abort()
	}
}

// isLoopbackRemoteAddr 报告 `host:port` 形式的来源地址是否为本机回环。
// 空串、解析失败、非回环 IP 一律 false（fail-closed；不猜）。
func isLoopbackRemoteAddr(remoteAddr string) bool {
	host := strings.TrimSpace(remoteAddr)
	if host == "" {
		return false
	}
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.Trim(host, "[]")
	if i := strings.IndexByte(host, '%'); i >= 0 {
		host = host[:i]
	}
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// isLoopbackHost 报告 host 是否只监听本机。空 host 视为非 loopback ——
// gin 绑 ":port" 等价于 0.0.0.0，fail closed 更安全。
func isLoopbackHost(host string) bool {
	h := strings.TrimSpace(host)
	if h == "" {
		return false
	}
	if h == "localhost" {
		return true
	}
	ip := net.ParseIP(strings.Trim(h, "[]"))
	return ip != nil && ip.IsLoopback()
}

// ─── 鉴权启动门 ──────────────────────────────────────────────────────

// insecureExposureLoopbackPublished 是 auth.allow_insecure_exposure 唯一承认的
// 取值：容器必须绑 0.0.0.0，但发布层限定为 127.0.0.1。声明本身不做安全
// 保证；真实暴露面由部署静态护栏 + 运行时 netstat 断言保证。
const insecureExposureLoopbackPublished = "loopback-published"

type authStartup struct {
	Secret   []byte // 空 = open-access
	Insecure bool   // 明确处于「无鉴权」模式
	Refuse   bool   // 拒绝启动
	Reason   string // 拒绝原因
}

// decideAuthStartup 裁决启动期的鉴权配置：**没有密钥就拒绝启动**。
// 唯一豁免是 auth.allow_insecure=true 且（绑定回环 或 声明 loopback-published）。
func decideAuthStartup(secret string, allowInsecure bool, bindHost, insecureExposure string) authStartup {
	if s := strings.TrimSpace(secret); s != "" {
		return authStartup{Secret: []byte(s)}
	}
	if !allowInsecure {
		return authStartup{
			Refuse: true,
			Reason: "auth: JWT secret missing — refusing to start in open-access mode. " +
				"Set JWT_SECRET (or auth.jwt_secret) to a strong random value. " +
				"Local dev only: set AUTH_INSECURE=true AND server.host=127.0.0.1",
		}
	}
	if isLoopbackHost(bindHost) {
		return authStartup{Insecure: true}
	}
	if strings.TrimSpace(insecureExposure) == insecureExposureLoopbackPublished {
		return authStartup{Insecure: true}
	}
	return authStartup{
		Refuse: true,
		Reason: fmt.Sprintf("auth: AUTH_INSECURE=true but server.host=%q is not loopback — "+
			"open-access would be reachable from other hosts. "+
			"Set server.host=127.0.0.1, or configure JWT_SECRET instead. "+
			"Containers (which must bind 0.0.0.0) must instead publish on loopback and declare "+
			"AUTH_INSECURE_EXPOSURE=%s", bindHost, insecureExposureLoopbackPublished),
	}
}

// ─── HTTP 生命周期 ───────────────────────────────────────────────────

// startHTTPServer 在 goroutine 中启动 HTTP server，返回 *http.Server 供优雅关停。
func startHTTPServer(router *gin.Engine, v *viper.Viper, logger zerolog.Logger, serviceName string) *http.Server {
	addr := fmt.Sprintf("%s:%d", v.GetString("server.host"), v.GetInt("server.port"))
	srv := &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
	go func() {
		logger.Info().Str("address", addr).Msgf("%s starting", serviceName)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal().Err(err).Msg("Server failed")
		}
	}()
	return srv
}

// waitForShutdown 阻塞直到收到 SIGINT 或 SIGTERM。
func waitForShutdown() os.Signal {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	return <-quit
}

// gracefulShutdown 只负责 HTTP 优雅退出：停止接受新请求，等在途 handler
// 返回（总预算 30s）。本进程不持有 job service / alert manager / store，
// aiStore 由 main 的 defer 自行收尾。
func gracefulShutdown(srv *http.Server, logger zerolog.Logger) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Error().Err(err).Msg("HTTP server forced to shutdown")
		return
	}
	logger.Info().Msg("HTTP server stopped accepting new requests")
}
