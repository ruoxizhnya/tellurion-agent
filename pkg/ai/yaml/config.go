// Package yaml provides YAML configuration generation and parsing for
// AI-generated strategies. The Generator emits YAML from an intent;
// ResolveExpression turns that YAML into the strategy_params submitted to
// core's expression_template.
//
// QT-17 S3-d：旧的 LoadStrategy / LoadAndRegister（把 YAML 在进程内装配成
// core 的 ExpressionStrategy）已删除 —— 本仓不再执行策略，执行只在 core。
package yaml

import (
	"fmt"

	yamlv3 "gopkg.in/yaml.v3"

	"github.com/ruoxizhnya/tellurion-contracts/domain"
)

// ParseConfig parses a YAML config string into a Config struct.
//
// This is the inverse of Generator.configToYAML: it accepts a YAML
// document matching the Config schema (strategy/backtest/data/risk/
// execution/optimization/expression) and returns the structured form.
// Required fields are validated: the strategy section and its name
// field must be present.
//
// Returns a clear error for:
//   - empty input
//   - malformed YAML (syntax errors)
//   - missing top-level strategy: section
//   - missing strategy.name field
//
// The Expression section is optional; if absent, the returned Config
// has a zero-valued Expression field.
func ParseConfig(yamlStr string) (*Config, error) {
	if yamlStr == "" {
		return nil, fmt.Errorf("yaml: ParseConfig: input is empty")
	}

	var config Config
	if err := yamlv3.Unmarshal([]byte(yamlStr), &config); err != nil {
		return nil, fmt.Errorf("yaml: ParseConfig: %w", err)
	}

	// Validate required top-level section.
	if config.Strategy.Name == "" && config.Strategy.Type == "" && config.Strategy.Description == "" {
		return nil, fmt.Errorf("yaml: ParseConfig: missing required section: strategy:")
	}

	// Validate required strategy.name field. We treat name as the
	// primary identifier (it becomes a log label downstream).
	if config.Strategy.Name == "" {
		return nil, fmt.Errorf("yaml: ParseConfig: missing required strategy field: name:")
	}

	return &config, nil
}

// validateDirection returns an error if d is not one of the recognized
// domain.Direction constants.
func validateDirection(d domain.Direction) error {
	switch d {
	case domain.DirectionLong, domain.DirectionShort, domain.DirectionClose, domain.DirectionHold:
		return nil
	default:
		return fmt.Errorf("invalid direction %q: must be one of long, short, close, hold", string(d))
	}
}

// ExpressionParamsFromConfig builds the flat params map that core's
// expression_template accepts, from a parsed Config's Expression section.
// Only non-zero values are included so partial updates preserve existing
// config.
//
// The param-name strings mirror ExpressionStrategy.Parameters() in
// pkg/strategy/expression/strategy.go — keeping the mapping in one
// package avoids divergent copies.
//
// When the Expression section is empty (the strategy.type == "expression"
// path with no explicit expression: block), the returned map contains
// only signal_expr="" — ResolveExpression drops that key so core applies
// its manifest default.
func ExpressionParamsFromConfig(config *Config) map[string]interface{} {
	if config == nil {
		return nil
	}
	expr := config.Expression
	params := map[string]interface{}{
		"signal_expr": expr.Signal.Expression,
	}
	if expr.Signal.Action != "" {
		params["action"] = expr.Signal.Action
	}
	if expr.Signal.Direction != "" {
		params["direction"] = expr.Signal.Direction
	}
	if expr.Signal.MinStrength != 0 {
		params["min_strength"] = expr.Signal.MinStrength
	}
	if expr.Signal.Lookback != 0 {
		params["lookback"] = expr.Signal.Lookback
	}
	if expr.Sizing.Method != "" {
		params["sizing_method"] = expr.Sizing.Method
	}
	if expr.Sizing.FixedWeight != 0 {
		params["fixed_weight"] = expr.Sizing.FixedWeight
	}
	if expr.Sizing.MaxPerStock != 0 {
		params["max_per_stock"] = expr.Sizing.MaxPerStock
	}
	if expr.Sizing.MaxTotal != 0 {
		params["max_total"] = expr.Sizing.MaxTotal
	}
	if expr.Risk.MaxPositionPct != 0 {
		params["max_position_pct"] = expr.Risk.MaxPositionPct
	}
	if expr.Risk.MaxOpenPositions != 0 {
		params["max_open_positions"] = expr.Risk.MaxOpenPositions
	}
	if expr.Risk.MinCashBuffer != 0 {
		params["min_cash_buffer"] = expr.Risk.MinCashBuffer
	}
	return params
}
