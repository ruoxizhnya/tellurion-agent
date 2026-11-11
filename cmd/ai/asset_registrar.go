package main

// 资产登记端口的装配根适配器（QT-32-B）。
//
// pkg/copilot 定义 port（AssetRegistrar / AssetRef），HTTP 实现在
// pkg/ai/client —— 两边不互相 import，所以映射放在这里，与 chainSink /
// StaticCheckAdapter 同一形状（S7-P1-2 的纪律）。

import (
	"context"
	"strings"

	"github.com/rs/zerolog"
	"github.com/spf13/viper"
	"tellurion-agent/pkg/ai/client"
	"tellurion-agent/pkg/copilot"
)

// strategyAssetRegistrar 把 copilot 的登记请求转给资产服务客户端。
type strategyAssetRegistrar struct {
	c *client.StrategyAssetClient
}

// NewStrategyAssetRegistrar 构造登记端口。
//
// `strategy_asset.url` 为空、或载体/参数不合法时返回 nil —— 配了 schemeless
// 地址之类的错误不该在第一次生成时才炸，但也绝不能静默降级成「不登记」：
// CopilotService 见到 nil 会在编译通过后 fail-closed（见 run() 的登记段）。
func NewStrategyAssetRegistrar(v *viper.Viper, logger zerolog.Logger) copilot.AssetRegistrar {
	baseURL := strings.TrimSpace(v.GetString("strategy_asset.url"))
	if baseURL == "" {
		logger.Warn().
			Msg("strategy_asset.url is empty: copilot generation will fail closed after the build passes (QT-32-B)")
		return nil
	}

	c, err := client.NewStrategyAssetClient(client.StrategyAssetConfig{
		BaseURL:      baseURL,
		Token:        v.GetString("strategy_asset.token"),
		Carrier:      v.GetString("strategy_asset.carrier"),
		ABIVersion:   v.GetString("strategy_asset.abi_version"),
		Scope:        v.GetString("strategy_asset.scope"),
		Entry:        v.GetString("strategy_asset.entry"),
		Toolchain:    v.GetString("strategy_asset.toolchain"),
		PollInterval: v.GetDuration("strategy_asset.poll_interval"),
		PollTimeout:  v.GetDuration("strategy_asset.poll_timeout"),
	})
	if err != nil {
		logger.Error().
			Err(err).
			Msg("strategy asset registrar disabled: copilot generation will fail closed after the build passes (QT-32-B)")
		return nil
	}

	tokenConfigured := strings.TrimSpace(v.GetString("strategy_asset.token")) != ""
	if !tokenConfigured {
		// 不拒启动（公开期链路照旧），但也不静默：strategy 收紧后这条链路会 401。
		logger.Warn().
			Msg("strategy_asset.token is empty: asset-service writes will be rejected (401) once the strategy write endpoints are gated; set STRATEGY_ASSET_TOKEN")
	}

	logger.Info().
		Str("url", baseURL).
		Str("carrier", c.Carrier()).
		Bool("token_configured", tokenConfigured).
		Msg("strategy asset registrar initialized")
	return &strategyAssetRegistrar{c: c}
}

func (r *strategyAssetRegistrar) Register(ctx context.Context, source string) (copilot.AssetRef, error) {
	ref, err := r.c.Register(ctx, source)
	return copilot.AssetRef{
		StrategyID:   ref.StrategyID,
		Carrier:      ref.Carrier,
		VersionHash:  ref.VersionHash,
		ArtifactHash: ref.ArtifactHash,
	}, err
}
