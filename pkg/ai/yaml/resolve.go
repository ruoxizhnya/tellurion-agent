package yaml

// YAML → core 内置表达式策略的提交形状（QT-17 S3-c-3）。
//
// 本仓不再有进程内策略注册表：pipeline 生成的表达式策略由 core 执行。
// core 侧可执行的内置表达式策略只有一个名字 —— `expression_template`
// （pkg/strategy/expression 自注册）。因此提交形状是：
//
//	strategy:        expression_template
//	strategy_params: YAML 的 expression 段（signal/sizing/risk）逐键映射
//
// 参数名与 core 的 `ExpressionStrategy.Parameters()` 清单逐字一致，未知键会在
// core 的 QT-20 校验里被 400 点名 —— 本仓不复制那份清单，只做能够离线判定
// 的形状校验（方向、仓位的确定性取值）。

import (
	"fmt"

	"github.com/ruoxizhnya/tellurion-contracts/domain"
)

// RunStrategyName 是 core 侧内置表达式策略的注册名。
//
// 为什么不是 YAML 里的 strategy.name：那个名字在本仓历史实现里是「本地注册
// 表键」，core 的注册表里并不存在它。HTTP 化之后，名字只作为实验日志的标签
// 保留（ResolvedStrategy.Label）。
const RunStrategyName = "expression_template"

// ResolvedStrategy 是一次 pipeline 回测要提交的策略引用。
type ResolvedStrategy struct {
	// Label 是 YAML 里的 strategy.name，只用于日志 / 实验记录。
	Label string
	// RunName 是提交给 core 的 strategy 字段。
	RunName string
	// Params 是提交给 core 的 strategy_params（表达式 + 仓位 / 风控覆写）。
	Params map[string]any
	// Config 是解析后的完整 YAML 配置（区间 / universe 等）。
	Config *Config
}

// knownSizingMethods 与 core pkg/strategy/expression 的 SizingMethod 取值一致。
// 在本地先拒绝拼错的取值，比等 core 在回测入口报错更早、也更容易读懂。
var knownSizingMethods = map[string]bool{
	"equal":         true,
	"strength_prop": true,
	"fixed":         true,
}

// ResolveExpression 把一份表达式策略 YAML 解析成「提交给 core 的请求形状」。
//
// 与旧本地加载路径的差别（如实登记）：
//   - 不再在本地解析 DSL（那是 core 的执行路径），语法错误由 core 的
//     expression 引擎在回测入口返回；本仓只校验 YAML 形状、方向、仓位方法；
//   - 不再因为 strategy.name == "expression_template" 而拒绝 —— 本地注册表
//     已不存在，名字只是日志标签；
//   - type=expression 但没有 expression 段时，**不发送** signal_expr，
//     让 core 用清单默认值（本仓的零值无法表达「保持默认」）。
func ResolveExpression(yamlStr string) (*ResolvedStrategy, error) {
	cfg, err := ParseConfig(yamlStr)
	if err != nil {
		return nil, err
	}

	hasExpressionSection := cfg.Expression.Signal.Expression != ""
	if !hasExpressionSection && cfg.Strategy.Type != "expression" {
		return nil, fmt.Errorf(
			"yaml: ResolveExpression: only expression-type strategies are supported; "+
				"either add an 'expression:' section with signal.expression or set "+
				"strategy.type to \"expression\" (got type=%q)",
			cfg.Strategy.Type)
	}

	if cfg.Expression.Signal.Direction != "" {
		if err := validateDirection(domain.Direction(cfg.Expression.Signal.Direction)); err != nil {
			return nil, fmt.Errorf("yaml: ResolveExpression: %w", err)
		}
	}
	if method := cfg.Expression.Sizing.Method; method != "" && !knownSizingMethods[method] {
		return nil, fmt.Errorf(
			"yaml: ResolveExpression: invalid sizing method %q: must be one of equal, strength_prop, fixed",
			method)
	}

	params := ExpressionParamsFromConfig(cfg)
	if !hasExpressionSection {
		// Configure 对「键存在但值为空串」的语义是**覆盖成空**，不是保持默认；
		// 要拿 core 的清单默认表达式，只能不带这个键。
		delete(params, "signal_expr")
	}

	return &ResolvedStrategy{
		Label:   cfg.Strategy.Name,
		RunName: RunStrategyName,
		Params:  params,
		Config:  cfg,
	}, nil
}
