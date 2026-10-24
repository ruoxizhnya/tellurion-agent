package main

// HTTP 回测 / walk-forward 适配器（QT-17 S3-c-3）。
//
// 本进程不再装配 backtest.Engine：回测只有一条路 —— 提交给 core 的
// `POST /api/backtest` / `POST /api/walkforward`（D24 粗粒度动作）。
// 逐 bar 执行仍在 core 进程内，agent 一次请求覆盖整段区间。
//
// 两个适配器都实现**窄接口**（contracts.BacktestRunner / builtin.WalkForwardRunner），
// 让 pipeline / copilot / explore / tools 不需要认识具体引擎。

import (
	"context"
	"fmt"
	"strings"

	"github.com/ruoxizhnya/tellurion-contracts/domain"
	"tellurion-agent/pkg/ai/client"
)

// ── universe → 回测请求的指数识别（DATA-01）────────────────────────────────

// indexUniverseAliases 是「这不是股票代码，而是指数」的白名单。
//
// 用白名单而非「形状匹配」兜底：把一只真股票误判成指数，会让回测悄悄跑在
// 一个完全不相干的票池上（而且看起来一切正常）—— 宁可漏判（退化成原来的
// 行为：查不到、fail-loud），也不误判。
//
// 这里只负责**识别**，不做代码翻译：`csi300` → `000300.SH` 的翻译在 core
// 侧（pkg/backtest/index_alias.go）一处生效，本仓不复制第二张映射表。
var indexUniverseAliases = map[string]bool{
	"csi300": true, "hs300": true, "沪深300": true,
	"csi500": true, "zz500": true, "中证500": true,
	"csi800": true, "zz800": true, "中证800": true,
	"sse50": true, "sz50": true, "上证50": true,
	// 已是 Tushare 指数代码的写法（注意与 600xxx/000xxx 股票代码区分：
	// 指数代码是 000xxx.SH 系列，000001.SZ 这类是股票，不在表内）
	"000300.sh": true, "000905.sh": true, "000906.sh": true, "000016.sh": true,
}

// looksLikeIndexCode 判断单个字符串是否为已知的指数标识。
func looksLikeIndexCode(s string) bool {
	key := strings.ToLower(strings.TrimSpace(s))
	if key == "" {
		return false
	}
	if indexUniverseAliases[key] {
		return true
	}
	// 通用形态兜底：csi/zz + 数字（如 csi1000、zz1000）—— 仍属「像指数的写法」；
	// 即使 core 侧暂未登记该指数，也会走 IndexCode 分支并因此查不到而 fail-loud，
	// 而不是被当成一只股票静默丢掉。
	for _, prefix := range []string{"csi", "zz"} {
		if rest, ok := strings.CutPrefix(key, prefix); ok && rest != "" && allDigits(rest) {
			return true
		}
	}
	return false
}

func allDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return s != ""
}

// splitUniverseIndex 判断 universe 是否表示「指数」。
//
// 返回 (指数代码, 余下的股票代码, 是否指数)。判定条件刻意收紧到「只有一项、
// 且该项是已知指数写法」—— 混合列表（指数 + 个股）语义不明，不做猜测。
func splitUniverseIndex(pool []string) (string, []string, bool) {
	if len(pool) != 1 {
		return "", pool, false
	}
	if !looksLikeIndexCode(pool[0]) {
		return "", pool, false
	}
	return strings.TrimSpace(pool[0]), nil, true
}

// buildBacktestRequest 把 universe 组装成回测请求：指数形态填 IndexCode
// （由 core 展开真实成分股），否则照旧填 StockPool。
//
// 抽成纯函数是为了能被直接单测 —— 「universe: csi300 走 IndexCode」这条
// 验收不能只靠读代码确认（DATA-01 ⑤）。
func buildBacktestRequest(strategyName string, universe []string, startDate, endDate string) client.BacktestRequest {
	req := client.BacktestRequest{
		Strategy:  strategyName,
		StartDate: startDate,
		EndDate:   endDate,
	}
	if idx, rest, ok := splitUniverseIndex(universe); ok {
		req.IndexCode = idx
		req.StockPool = rest // 指数形态下为空
	} else {
		req.StockPool = universe
	}
	return req
}

// ── 回测 ──────────────────────────────────────────────────────────────

// HTTPBacktestRunner 实现 contracts.BacktestRunner。
type HTTPBacktestRunner struct {
	c *client.BacktestClient
}

// NewHTTPBacktestRunner 构造 HTTP 回测适配器。nil client 是装配错误，fail loud。
func NewHTTPBacktestRunner(c *client.BacktestClient) *HTTPBacktestRunner {
	if c == nil {
		panic("cmd/ai: NewHTTPBacktestRunner called with nil BacktestClient")
	}
	return &HTTPBacktestRunner{c: c}
}

// RunBacktest 实现 contracts.BacktestRunner（无策略参数）。
//
// 判据与旧进程内适配器一致：completed 与 invalid 都返回结果（invalid 的
// `InvalidReasons` 由 pipeline 判成失败并原样展示）；failed / 传输错误才返回
// error。这样 `backtest.run` 工具与 pipeline 看到的三值语义逐字节不变。
func (r *HTTPBacktestRunner) RunBacktest(ctx context.Context, strategyName string, stockPool []string, startDate, endDate string) (*domain.BacktestResult, error) {
	return r.run(ctx, strategyName, nil, stockPool, startDate, endDate)
}

// RunBacktestWithParams 实现 pipeline.StrategyParamsRunner（QT-20）。
//
// 表达式策略在本仓已无本地注册表：pipeline 提交 strategy=expression_template
// + strategy_params，由 core 的清单默认 ⊕ 覆写解析出生效参数。
func (r *HTTPBacktestRunner) RunBacktestWithParams(
	ctx context.Context,
	strategyName string,
	strategyParams map[string]any,
	stockPool []string,
	startDate, endDate string,
) (*domain.BacktestResult, error) {
	return r.run(ctx, strategyName, strategyParams, stockPool, startDate, endDate)
}

func (r *HTTPBacktestRunner) run(
	ctx context.Context,
	strategyName string,
	strategyParams map[string]any,
	stockPool []string,
	startDate, endDate string,
) (*domain.BacktestResult, error) {
	req := buildBacktestRequest(strategyName, stockPool, startDate, endDate)
	req.StrategyParams = strategyParams

	outcome, err := r.c.RunBacktest(ctx, req)
	if err != nil {
		return nil, err
	}
	if outcome.Status == client.StatusFailed {
		return nil, fmt.Errorf("backtest failed: %s", outcome.Error)
	}
	if outcome.Result == nil {
		// 作业态（pending/running）或未知状态：没有结果可交，不能编一份零值。
		return nil, fmt.Errorf("backtest 没有可用的结果（status=%q, job_id=%q）", outcome.Status, outcome.JobID)
	}
	return outcome.Result, nil
}

// ── walk-forward ───────────────────────────────────────────────────────

// HTTPWalkForwardRunner 实现 builtin.WalkForwardRunner。
type HTTPWalkForwardRunner struct {
	c *client.BacktestClient
}

// NewHTTPWalkForwardRunner 构造 HTTP walk-forward 适配器。
func NewHTTPWalkForwardRunner(c *client.BacktestClient) *HTTPWalkForwardRunner {
	if c == nil {
		panic("cmd/ai: NewHTTPWalkForwardRunner called with nil BacktestClient")
	}
	return &HTTPWalkForwardRunner{c: c}
}

// RunWalkForward 实现 builtin.WalkForwardRunner。
//
// core 的 DTO 没有 index_code：与旧进程内适配器一致，stockPool 原样提交。
func (r *HTTPWalkForwardRunner) RunWalkForward(
	ctx context.Context,
	strategyName string,
	stockPool []string,
	startDate, endDate string,
	params domain.WalkForwardParams,
) (*domain.WalkForwardReport, error) {
	return r.c.RunWalkForward(ctx, client.WalkForwardRequest{
		Strategy:          strategyName,
		StockPool:         stockPool,
		StartDate:         startDate,
		EndDate:           endDate,
		WalkForwardParams: params,
	})
}
