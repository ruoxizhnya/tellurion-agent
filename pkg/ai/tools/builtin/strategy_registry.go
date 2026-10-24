package builtin

import (
	"context"
	"fmt"

	"tellurion-agent/pkg/ai/client"
	"tellurion-agent/pkg/ai/tools"
)

// StrategyCatalog is the narrow interface satisfied by *client.StrategyClient.
// Extracted so the tools can be unit-tested with a fake instead of standing up
// an HTTP server (the HTTP shape itself is covered in pkg/ai/client).
//
// QT-17 S3-c-2：策略目录改由 core 的 `GET /api/strategies` 提供（服务端
// 合并「注册表 ∪ DB 活跃项」）。本仓不再 import `tellurion/pkg/strategy`，
// 也不再持有任何策略实例 —— 策略注册与实例构造仍只发生在引擎进程内。
type StrategyCatalog interface {
	// ListStrategies returns the merged strategy catalog.
	ListStrategies(ctx context.Context) ([]client.StrategyInfo, error)
}

// Compile-time assertion that the concrete client satisfies the narrow
// interface. Failing at compile time is strictly better than failing at
// startup.
var _ StrategyCatalog = (*client.StrategyClient)(nil)

// ─── StrategyListTool ─────────────────────────────────────────────────

// StrategyListTool lists all strategies known to core (registry ∪ active DB
// entries). Read-only — strategy registration still goes through the existing
// POST /api/strategies endpoint or the plugin loader on the server side.
//
// Tool name: "strategy.list"
// Input: none
// Output: []client.StrategyInfo (name + description + parameters per strategy)
type StrategyListTool struct {
	catalog StrategyCatalog
}

var _ tools.Tool = (*StrategyListTool)(nil)

// NewStrategyListTool constructs a StrategyListTool backed by the given
// catalog. Panics on nil — fail-loud at wiring time (composition root), not at
// first Execute call.
func NewStrategyListTool(catalog StrategyCatalog) *StrategyListTool {
	if catalog == nil {
		panic("builtin: NewStrategyListTool called with nil StrategyCatalog")
	}
	return &StrategyListTool{catalog: catalog}
}

func (t *StrategyListTool) Name() string { return "strategy.list" }

func (t *StrategyListTool) Description() string {
	return "List all strategies known to the analysis service (built-in registry merged with active database entries). Returns each strategy's name, description, and parameter schema. Use this to discover what strategies are available before running a backtest."
}

func (t *StrategyListTool) Parameters() []tools.Parameter { return nil }

func (t *StrategyListTool) OutputSchema() tools.OutputSchema {
	return tools.OutputSchema{
		Type:        "array",
		Description: "Array of registered strategies with name, description, and parameter schema.",
		Fields: []tools.OutputField{
			{Name: "name", Type: "string", Description: "Strategy name (use as 'strategy_name' in backtest.run)."},
			{Name: "description", Type: "string", Description: "Human-readable strategy description."},
			{Name: "parameters", Type: "array", Description: "Strategy parameter schema."},
		},
	}
}

func (t *StrategyListTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	return t.catalog.ListStrategies(ctx)
}

// ─── StrategyGetTool ──────────────────────────────────────────────────

// StrategyGetTool fetches a single strategy's full metadata by name.
//
// QT-17 S3-c-2：`GET /api/strategies/{id}` 只查 DB（内置策略 404），因此本
// 工具改从 list 结果里按 name 过滤 —— 与旧的 `strategy.GlobalGet` 语义一致
// （原名精确匹配），但覆盖范围是完整的合并目录。
//
// Tool name: "strategy.get"
// Input: name (required)
// Output: client.StrategyInfo (name + description + parameters)
type StrategyGetTool struct {
	catalog StrategyCatalog
}

var _ tools.Tool = (*StrategyGetTool)(nil)

// NewStrategyGetTool constructs a StrategyGetTool backed by the given catalog.
// Panics on nil — fail-loud at wiring time (composition root).
func NewStrategyGetTool(catalog StrategyCatalog) *StrategyGetTool {
	if catalog == nil {
		panic("builtin: NewStrategyGetTool called with nil StrategyCatalog")
	}
	return &StrategyGetTool{catalog: catalog}
}

func (t *StrategyGetTool) Name() string { return "strategy.get" }

func (t *StrategyGetTool) Description() string {
	return "Get detailed metadata for a single registered strategy by name. Returns the strategy's name, description, and full parameter schema. Useful before calling backtest.run to understand what parameters a strategy accepts."
}

func (t *StrategyGetTool) Parameters() []tools.Parameter {
	return []tools.Parameter{
		{
			Name:        "name",
			Type:        "string",
			Description: "Strategy name (e.g. 'momentum', 'dual_momentum', 'expression_template'). Use strategy.list to discover names.",
			Required:    true,
		},
	}
}

func (t *StrategyGetTool) OutputSchema() tools.OutputSchema {
	return tools.OutputSchema{
		Type:        "object",
		Description: "Strategy metadata: name, description, and parameter schema.",
		Fields: []tools.OutputField{
			{Name: "name", Type: "string", Description: "Strategy name."},
			{Name: "description", Type: "string", Description: "Human-readable description."},
			{Name: "parameters", Type: "array", Description: "Parameter schema (name, type, default, required)."},
		},
	}
}

func (t *StrategyGetTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	name, err := requireString(args, "name")
	if err != nil {
		return nil, err
	}

	infos, err := t.catalog.ListStrategies(ctx)
	if err != nil {
		return nil, fmt.Errorf("strategy.get: %w", err)
	}

	// 精确匹配（与旧 GlobalGet 的 map 键语义一致）；不模糊、不大小写折叠。
	for _, info := range infos {
		if info.Name == name {
			return info, nil
		}
	}
	return nil, fmt.Errorf("strategy.get: strategy %q is not in the catalog (GET /api/strategies: registry ∪ active DB entries)", name)
}
