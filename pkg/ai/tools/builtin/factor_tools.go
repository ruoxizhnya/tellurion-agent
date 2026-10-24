package builtin

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"tellurion-agent/pkg/ai/client"
	"tellurion-agent/pkg/ai/tools"
)

// ─── ValidateFactorTool ────────────────────────────────────────────────
//
// L1 validation gate: delegates static validation (Parse + operator/field
// gate) to core's `POST /api/expression/validate` (QT-69 / QT-17 S3-c-4).
// The local parser/evaluator is gone: the DSL grammar and its fail-closed
// gate have exactly one implementation, in core. This method does NOT
// compute values — that is the job of factor.compute / compute_factor_ic.
//
// OBS-06 semantics are preserved by delegation: parsing alone is not
// enough. `CROSS(MA(close,5), MA(close,20))` parses (function-call
// syntax) but both operators are unknown; core's gate returns valid=false.
//
// Tool name: "validate_factor"
// Input: expression (required)
// Output: { valid, inputs, ast, error, level, passed, reason, recommendation }
//
// The four `level/passed/reason/recommendation` fields constitute the
// GateDecision (see gate.go). They let Hermes uniformly check "did this
// gate pass?" without parsing tool-specific fields. For L1:
//   - level    = "L1"
//   - passed   = valid (core's parse + gate verdict)
//   - reason   = "passed" or "syntax_error"
//   - recommendation = LLM-facing actionable hint (Chinese)
//
// Error semantics: core's valid=false is a RESULT, not a Go error. A
// transport failure / non-200 (core unreachable, 401, 503) IS a Go error —
// "the service did not answer" is not "the expression is invalid", and
// silently returning valid=false would let a broken dialect look like a
// broken formula.
type ValidateFactorTool struct {
	c *client.ExpressionClient
}

var _ tools.Tool = (*ValidateFactorTool)(nil)

// NewValidateFactorTool constructs a ValidateFactorTool backed by c.
// Panics if c is nil — wiring bug, fail loud at startup.
func NewValidateFactorTool(c *client.ExpressionClient) *ValidateFactorTool {
	if c == nil {
		panic("builtin: NewValidateFactorTool called with nil ExpressionClient")
	}
	return &ValidateFactorTool{c: c}
}

func (t *ValidateFactorTool) Name() string { return "validate_factor" }

func (t *ValidateFactorTool) Description() string {
	return "Validate a factor expression's syntax (L1 gate). Returns validity, required data inputs (e.g. close, volume), and the parsed AST. Does not compute values — call compute_factor_ic for that."
}

func (t *ValidateFactorTool) Parameters() []tools.Parameter {
	return []tools.Parameter{
		{
			Name:        "expression",
			Type:        "string",
			Description: "Factor DSL expression to validate, e.g. 'ts_rank(close, 20)' or '(close - ts_mean(close, 20)) / ts_std(close, 20)'.",
			Required:    true,
		},
	}
}

func (t *ValidateFactorTool) OutputSchema() tools.OutputSchema {
	return tools.OutputSchema{
		Type:        "object",
		Description: "Validation result. If valid is false, error explains the syntax problem. The level/passed/reason/recommendation fields are the L1 GateDecision — Hermes reads `passed` to decide whether to advance to L2.",
		Fields: []tools.OutputField{
			{Name: "valid", Type: "bool", Description: "true if the expression parses successfully."},
			{Name: "inputs", Type: "array", Description: "Required data fields (e.g. [\"close\", \"volume\"]). Empty if invalid."},
			{Name: "ast", Type: "string", Description: "String representation of the parsed AST. Empty if invalid."},
			{Name: "error", Type: "string", Description: "Parse error message. Empty if valid."},
			{Name: "level", Type: "string", Description: "Gate identifier: always \"L1\"."},
			{Name: "passed", Type: "bool", Description: "true if the gate passed (mirrors `valid`)."},
			{Name: "reason", Type: "string", Description: "Machine-readable reason code: \"passed\" or \"syntax_error\"."},
			{Name: "recommendation", Type: "string", Description: "LLM-facing actionable hint (Chinese)."},
		},
	}
}

func (t *ValidateFactorTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	expr, err := requireString(args, "expression")
	if err != nil {
		return nil, err
	}

	verdict, err := t.c.Validate(ctx, expr)
	if err != nil {
		// 服务没答 ≠ 表达式不合法：fail-closed，不伪造 valid=false。
		return nil, fmt.Errorf("validate_factor: expression validation service unavailable: %w", err)
	}

	if !verdict.Valid {
		passed := false
		return map[string]interface{}{
			"valid":          false,
			"inputs":         []string{},
			"ast":            "",
			"error":          verdict.Error,
			"level":          "L1",
			"passed":         passed,
			"reason":         gateReasonL1(passed),
			"recommendation": t.l1FailureRecommendation(ctx),
		}, nil
	}

	inputs := append([]string(nil), verdict.Inputs...)
	if inputs == nil {
		inputs = []string{}
	}
	passed := true
	return map[string]interface{}{
		"valid":          true,
		"inputs":         inputs,
		"ast":            verdict.AST,
		"error":          "",
		"level":          "L1",
		"passed":         passed,
		"reason":         gateReasonL1(passed),
		"recommendation": gateRecommendationL1(passed),
	}, nil
}

// l1FailureRecommendation builds the L1 failure recommendation text with
// the DSL's legal field set appended (grouped by data source). This lets
// an agent that tripped the gate self-correct: it sees exactly which
// fields exist and which source each belongs to (OBS-08 切片 1, 3.6).
//
// 只改文本内容 —— JSON 输出字段结构（valid/inputs/ast/error/level/
// passed/reason/recommendation）保持不变。
//
// QT-17 S3-c-4：字段清单改为从 core 的 `GET /api/expression/catalog` 取。
// 目录拿不到时只给基础建议并**明确说明**清单缺失，不拿过期/凭空清单糊弄。
func (t *ValidateFactorTool) l1FailureRecommendation(ctx context.Context) string {
	rec := gateRecommendationL1(false)
	catalog, err := t.c.Catalog(ctx, nil)
	if err != nil {
		return rec + " （字段目录暂不可用，无法附上合法字段清单）"
	}
	if hint := fieldsHint(catalog); hint != "" {
		rec += " " + hint
	}
	return rec
}

// 字段状态 / 来源字面量与 core `cmd/analysis/handlers_expression.go` 一致。
const (
	fieldStatusAvailable   = "available"
	fieldStatusUnavailable = "unavailable"
	fieldStatusUnknown     = "unknown"

	fieldSourceGroup = "group"
)

// fieldsHint 把目录渲染成「（可用字段：market: …；fundamentals: …）」。
//
// 语义要点（OBS-08 切片 2）：
//   - `unavailable` 的字段**不能从清单里消失** —— 已登记但源表为空时，
//     必须在提示里点名「当前无数据、别用」，否则 AI 会反复尝试注定失败的公式；
//   - `unknown`（core 没装可用性探测）不等于不可用，列进可用清单；
//   - `source=group` 是 cs_neutralize 的分组标签，不是数据字段，不列。
func fieldsHint(c *client.ExpressionCatalog) string {
	if c == nil {
		return ""
	}

	bySource := map[string][]string{}
	var unusable []string
	for _, f := range c.Fields {
		if f.Source == fieldSourceGroup {
			continue
		}
		switch f.Status {
		case fieldStatusUnavailable:
			unusable = append(unusable, f.Name)
		default: // available / unknown（未探测 ≠ 不可用）
			bySource[f.Source] = append(bySource[f.Source], f.Name)
		}
	}

	// 来源顺序固定：market → fundamentals → activity → 其余按名字排序，
	// 保证同一份目录每次渲染一致（提示词可断言、diff 可读）。
	order := []string{"market", "fundamentals", "activity"}
	seen := map[string]bool{}
	for _, src := range order {
		seen[src] = true
	}
	others := make([]string, 0, len(bySource))
	for src := range bySource {
		if !seen[src] {
			others = append(others, src)
		}
	}
	sort.Strings(others)
	order = append(order, others...)

	parts := make([]string, 0, len(order))
	for _, src := range order {
		fields := bySource[src]
		if len(fields) == 0 {
			continue
		}
		sort.Strings(fields)
		parts = append(parts, src+": "+strings.Join(fields, ", "))
	}

	hint := ""
	if len(parts) > 0 {
		hint = "（可用字段：" + strings.Join(parts, "；") + "）"
	}
	if len(unusable) > 0 {
		sort.Strings(unusable)
		hint += "（当前无数据、不要使用：" + strings.Join(unusable, ", ") + "）"
	}
	return hint
}

// ─── ComputeFactorICTool ───────────────────────────────────────────────
//
// L2 validation gate: computes a factor expression's Information
// Coefficient (IC) and Information Ratio (IR) over a symbol list and
// date range. IC > 0.03 is generally useful; IC > 0.05 is strong.
//
// Tool name: "compute_factor_ic"
// Input: expression, symbols, start_date, end_date (all required)
// Output: *computeFactorICResult (embeds GateDecision + IC/IR metrics)
//
// GateDecision (L2):
//   - level    = "L2"
//   - passed   = (IC >= GateL2MinIC)
//   - reason   = "passed" or "low_ic"
//   - recommendation = LLM-facing actionable hint (Chinese)
//
// Design note (deviation from hermes-agent-integration-system-design.md
// §3.2): the design doc specifies a `universe` parameter (default
// "csi300") instead of `symbols`. A universe resolver would map a named
// pool ("csi300") to a concrete []string. That resolver is Phase 2
// work (needs storage or data-service backing). For Phase 1 we accept
// an explicit `symbols` list, matching the existing factor.evaluate /
// factor.compute tools. When the resolver lands, a future commit can
// add `universe` as an alternative input — both fields will be accepted
// and exactly one will be required.
type ComputeFactorICTool struct {
	c *client.FactorClient
}

var _ tools.Tool = (*ComputeFactorICTool)(nil)

// NewComputeFactorICTool constructs a ComputeFactorICTool backed by c.
// Panics if c is nil — wiring bug, fail loud at startup.
func NewComputeFactorICTool(c *client.FactorClient) *ComputeFactorICTool {
	if c == nil {
		panic("builtin: NewComputeFactorICTool called with nil FactorClient")
	}
	return &ComputeFactorICTool{c: c}
}

func (t *ComputeFactorICTool) Name() string { return "compute_factor_ic" }

func (t *ComputeFactorICTool) Description() string {
	return "Compute a factor expression's Information Coefficient (IC) and Information Ratio (IR) over a symbol list and date range (L2 gate). IC > 0.03 is useful; IC > 0.05 is strong."
}

func (t *ComputeFactorICTool) Parameters() []tools.Parameter {
	return []tools.Parameter{
		{
			Name:        "expression",
			Type:        "string",
			Description: "Factor DSL expression, e.g. 'ts_rank(close, 20)'.",
			Required:    true,
		},
		{
			Name:        "symbols",
			Type:        "[]string",
			Description: "List of stock symbols to evaluate the factor over, e.g. [\"000001.SZ\", \"600000.SH\"].",
			Required:    true,
		},
		{
			Name:        "start_date",
			Type:        "string",
			Description: "Evaluation window start date in YYYY-MM-DD format.",
			Required:    true,
		},
		{
			Name:        "end_date",
			Type:        "string",
			Description: "Evaluation window end date in YYYY-MM-DD format.",
			Required:    true,
		},
	}
}

func (t *ComputeFactorICTool) OutputSchema() tools.OutputSchema {
	return tools.OutputSchema{
		Type:        "object",
		Description: "Factor quality metrics + L2 GateDecision. Hermes reads `passed` to decide whether the factor is worth a full backtest (L3).",
		Fields: []tools.OutputField{
			{Name: "ic", Type: "float", Description: "Information Coefficient: rank correlation between factor value and forward return. |IC| > 0.03 is useful."},
			{Name: "ir", Type: "float", Description: "Information Ratio: IC mean / IC std. IR > 0.5 is strong."},
			{Name: "level", Type: "string", Description: "Gate identifier: always \"L2\"."},
			{Name: "passed", Type: "bool", Description: "true if IC >= 0.02 (the L2 threshold)."},
			{Name: "reason", Type: "string", Description: "Machine-readable reason code: \"passed\" or \"low_ic\"."},
			{Name: "recommendation", Type: "string", Description: "LLM-facing actionable hint (Chinese)."},
		},
	}
}

func (t *ComputeFactorICTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	expr, err := requireString(args, "expression")
	if err != nil {
		return nil, err
	}
	symbols, err := requireStringSlice(args, "symbols")
	if err != nil {
		return nil, err
	}
	startDate, err := requireString(args, "start_date")
	if err != nil {
		return nil, err
	}
	endDate, err := requireString(args, "end_date")
	if err != nil {
		return nil, err
	}

	m, err := t.c.EvaluateFactor(ctx, expr, symbols, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("compute_factor_ic: %w", err)
	}

	passed := m.IC >= GateL2MinIC
	return &computeFactorICResult{
		GateDecision: GateDecision{
			Level:          "L2",
			Passed:         passed,
			Reason:         gateReasonL2(m.IC),
			Recommendation: gateRecommendationL2(m.IC),
		},
		IC: m.IC,
		IR: m.IR,
	}, nil
}

// computeFactorICResult wraps *client.FactorMetrics with the L2
// GateDecision. We define a local struct (rather than embedding
// GateDecision in client.FactorMetrics) to keep the gate decision
// logic inside the builtin package — the upstream client should not
// depend on gate concepts.
//
// JSON shape: { ic, ir, level, passed, reason, recommendation }
type computeFactorICResult struct {
	GateDecision
	IC float64 `json:"ic"`
	IR float64 `json:"ir"`
}
