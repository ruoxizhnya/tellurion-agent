package main

// endpoints.go —— 启动物理地址体检（配置键 ↔ env 覆盖名）。
//
// 与 core `cmd/analysis/endpoints.go` 同一判据（agent 对 core 零编译依赖，
// 这段 30 行按仓复制）。目的：yaml 是宿主机视角（127.0.0.1），容器由
// deploy/compose.yaml 的 env 覆盖；宿主机上若还留着容器 DNS
// （data-service / analysis-service），在**启动时**点名 env 报错，
// 而不是在第一次工具调用时远处 404 / no such host。
//
// 只解析主机名，不探测端口：被调服务可能晚于本进程启动，那是「暂时不可用」，
// 由运行时降级/探针负责，不该判成配置错误。

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/spf13/viper"
)

const endpointResolveTimeout = 2 * time.Second

// validateOutboundEndpoints 逐项校验真正会被读取的出站地址。
//
// strategy_asset.url 允许为空（= 登记端口未注入，编译通过后 fail-closed），
// 所以空值不算缺配；非空则必须能解析。
func validateOutboundEndpoints(v *viper.Viper, logger zerolog.Logger) {
	checks := []struct {
		key    string
		envKey string
		value  string
	}{
		{key: "data_service.url", envKey: "DATA_SERVICE_URL", value: v.GetString("data_service.url")},
		{key: "ai_service.factor_api_url", envKey: "AI_SERVICE_FACTOR_API_URL", value: analysisAPIURL(v)},
	}
	if raw := strings.TrimSpace(v.GetString("strategy_asset.url")); raw != "" {
		checks = append(checks, struct {
			key    string
			envKey string
			value  string
		}{"strategy_asset.url", "STRATEGY_ASSET_URL", raw})
	} else {
		logger.Warn().Msg("strategy_asset.url is empty — asset registration will fail closed after compile")
	}

	for _, c := range checks {
		if err := validateServiceEndpoint(c.key, c.envKey, c.value); err != nil {
			logger.Fatal().Err(err).Msg("outbound endpoint configuration invalid")
		}
		logger.Debug().Str("key", c.key).Str("url", c.value).Msg("outbound endpoint ok")
	}
}

// validateServiceEndpoint 见 core 的同名实现：空值/非法 URL 报错；回环与
// IP 字面量直接通过；其它主机名解析一次，失败时给出 env 名与宿主机写法。
func validateServiceEndpoint(key, envKey, rawURL string) error {
	raw := strings.TrimSpace(rawURL)
	if raw == "" {
		return fmt.Errorf("%s is empty — set %s (container: service DNS, host: http://127.0.0.1:<port>)",
			key, envKey)
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("%s=%q is not an absolute URL (want http(s)://host:port); set %s",
			key, raw, envKey)
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("%s=%q has no host; set %s", key, raw, envKey)
	}
	if isLoopbackOrIP(host) {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), endpointResolveTimeout)
	defer cancel()
	if _, err := net.DefaultResolver.LookupHost(ctx, host); err != nil {
		return fmt.Errorf(
			"%s points at %q, whose host %q does not resolve here: %v. "+
				"Compose service names only work inside the container network — "+
				"for a host-side run set %s=http://127.0.0.1:<port>",
			key, raw, host, err, envKey)
	}
	return nil
}

func isLoopbackOrIP(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	return ip != nil
}
