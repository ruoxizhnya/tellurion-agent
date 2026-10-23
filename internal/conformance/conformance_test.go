package conformance

// agent 侧的契约一致性腿（QT-09 ③）：与契约仓、core 读**同一份** cases.json，
// 对 agent 自己的实现实算比对 —— 任一侧语义漂移即红。
//
// 这里钉住的正是历史漂移点：agent 生成的 YAML 曾把 slippage_rate 写成 0.001
// （FixedSlippageRate）/ commission_rate 写成过期的 0.00025。语料把两侧的
// 默认值绑到 fees.DefaultCommissionRate / fees.DefaultSlippageRate 上。
//
// 语料目录定位顺序见 locateCorpus；全都找不到就 t.Fatalf，绝不 t.Skip。

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	contracts "github.com/ruoxizhnya/tellurion-contracts/engine"
	"github.com/ruoxizhnya/tellurion-contracts/fees"
	"tellurion-agent/pkg/ai/intent"
	yamlgen "tellurion-agent/pkg/ai/yaml"
)

const (
	contractsModule = "github.com/ruoxizhnya/tellurion-contracts"
	corpusDir       = "conformance"
	side            = "agent"
)

// knownKinds 是语料的全部 kind（三仓共用），用于区分「未知 kind」与「别的侧的 kind」。
var knownKinds = map[string]bool{
	"stamp_tax_rate_for":    true,
	"fee_defaults":          true,
	"daily_visible_at":      true,
	"intraday_visible_at":   true,
	"visible_by":            true,
	"time_semantics":        true,
	"run_spec_hash":         true,
	"trade_fees":            true,
	"fee_defaults_via_yaml": true,
}

// TestContractConformance 是 agent 侧入口。
func TestContractConformance(t *testing.T) {
	dir := locateCorpus(t)
	cases, err := loadCases(filepath.Join(dir, corpusDir))
	if err != nil {
		t.Fatalf("加载契约语料失败: %v", err)
	}
	t.Logf("语料目录: %s（%d 条）", dir, len(cases))

	handlers := map[string]func(*testing.T, caseSpec){
		"run_spec_hash":         checkRunSpecHash,
		"fee_defaults_via_yaml": checkFeeDefaultsViaYAML,
	}

	ran, skipped := 0, 0
	for _, c := range cases {
		c := c
		if !declaresSide(c.Sides, side) {
			skipped++
			t.Logf("跳过 %s（sides=%v，本侧不跑）", c.ID, c.Sides)
			continue
		}
		handler, ok := handlers[c.Kind]
		if !ok {
			if knownKinds[c.Kind] {
				t.Fatalf("用例 %q：kind %q 不是 agent 侧的实现，但 sides 声明了 agent（覆盖矩阵自相矛盾）", c.ID, c.Kind)
			}
			t.Fatalf("用例 %q：未知 kind %q，但 sides 声明了 agent（未知 kind 不许静默跳过）", c.ID, c.Kind)
		}
		ran++
		t.Run(c.ID, func(t *testing.T) { handler(t, c) })
	}

	t.Logf("agent 侧：执行 %d 条 / 跳过 %d 条 / 语料共 %d 条", ran, skipped, len(cases))
	if ran == 0 {
		t.Fatalf("agent 侧一条用例都没执行 —— 语料与 sides 声明脱节")
	}
}

// locateCorpus 依次尝试：
//  1. `go list -m -f {{.Dir}}` 指到的 module 目录（含 conformance/cases.json 才算数）；
//  2. runtime.GOROOT()/bin/go 再试一次（PATH 没有 go 的环境）；
//  3. 从当前工作目录逐级向上找同级的 tellurion-contracts 工作树。
//
// 全部失败 → t.Fatalf，不用 t.Skip：语料不可达是「本侧没真正执行」，不是「不适用」。
func locateCorpus(t *testing.T) string {
	t.Helper()

	var candidates []string
	tryGo := func(goBin string) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		out, err := exec.CommandContext(ctx, goBin, "list", "-m", "-f", "{{.Dir}}", contractsModule).Output()
		if err != nil {
			t.Logf("go list -m（%s）失败: %v", goBin, err)
			return
		}
		if dir := string(bytes.TrimSpace(out)); dir != "" {
			candidates = append(candidates, dir)
		}
	}

	if bin, err := exec.LookPath("go"); err == nil {
		tryGo(bin)
	}
	if root := runtime.GOROOT(); root != "" {
		for _, name := range []string{"go", "go.exe"} {
			bin := filepath.Join(root, "bin", name)
			if info, err := os.Stat(bin); err == nil && !info.IsDir() {
				tryGo(bin)
				break
			}
		}
	}
	if wd, err := os.Getwd(); err == nil {
		for dir := wd; ; {
			candidates = append(candidates, filepath.Join(dir, "tellurion-contracts"))
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}

	for _, c := range candidates {
		if info, err := os.Stat(filepath.Join(c, corpusDir, "cases.json")); err == nil && !info.IsDir() {
			return c
		}
	}
	t.Fatalf("找不到契约仓语料 %s/%s/cases.json（已尝试: %v）—— 语料不可达不是「本侧不适用」",
		contractsModule, corpusDir, candidates)
	return ""
}

// ─── 各 kind 的断言 ──────────────────────────────────────────

// runSpecCase 是语料里 run_spec_hash 的入参（与 contracts.RunSpecInput 一一对应）。
type runSpecCase struct {
	Start                   string             `json:"start"`
	End                     string             `json:"end"`
	InitialCapital          float64            `json:"initial_capital"`
	RiskFreeRate            float64            `json:"risk_free_rate"`
	StrategyArtifactHash    string             `json:"strategy_artifact_hash"`
	StrategyVersionHash     string             `json:"strategy_version_hash"`
	StrategyParams          map[string]any     `json:"strategy_params"`
	StockPool               []string           `json:"stock_pool"`
	IndexCode               string             `json:"index_code"`
	Universe                []string           `json:"universe"`
	Frequency               string             `json:"frequency"`
	FillTiming              string             `json:"fill_timing"`
	Kernel                  string             `json:"kernel"`
	EngineVersion           string             `json:"engine_version"`
	Fees                    map[string]float64 `json:"fees"`
	Slippage                float64            `json:"slippage"`
	CostModel               string             `json:"cost_model"`
	LotSize                 int64              `json:"lot_size"`
	TPlus                   string             `json:"t_plus"`
	PriceLimitRule          string             `json:"price_limit_rule"`
	RiskRulesHash           string             `json:"risk_rules_hash"`
	DatasetSnapshotHash     string             `json:"dataset_snapshot_hash"`
	AdjustmentAnchorDigests []string           `json:"adjustment_anchor_digests"`
	Seed                    int64              `json:"seed"`
}

func checkRunSpecHash(t *testing.T, c caseSpec) {
	t.Helper()
	var in runSpecCase
	var want struct {
		Hash string `json:"hash"`
	}
	decodePair(t, c, &in, &want)

	spec := contracts.RunSpecInput{
		Start:                   parseDateArg(t, c, "start", in.Start),
		End:                     parseDateArg(t, c, "end", in.End),
		InitialCapital:          in.InitialCapital,
		RiskFreeRate:            in.RiskFreeRate,
		StrategyArtifactHash:    in.StrategyArtifactHash,
		StrategyVersionHash:     in.StrategyVersionHash,
		StrategyParamsEffective: in.StrategyParams,
		StockPool:               in.StockPool,
		IndexCode:               in.IndexCode,
		Universe:                in.Universe,
		Frequency:               in.Frequency,
		FillTiming:              contracts.FillTiming(in.FillTiming),
		Kernel:                  in.Kernel,
		EngineVersion:           in.EngineVersion,
		Fees:                    in.Fees,
		Slippage:                in.Slippage,
		CostModel:               in.CostModel,
		LotSize:                 in.LotSize,
		TPlus:                   in.TPlus,
		PriceLimitRule:          in.PriceLimitRule,
		RiskRulesHash:           in.RiskRulesHash,
		DatasetSnapshotHash:     in.DatasetSnapshotHash,
		AdjustmentAnchorDigests: in.AdjustmentAnchorDigests,
		Seed:                    in.Seed,
	}

	got, err := spec.Hash()
	if err != nil {
		t.Fatalf("用例 %q: RunSpecInput.Hash 报错: %v", c.ID, err)
	}
	if got != want.Hash {
		t.Errorf("用例 %q: run_spec_hash 期望 %s，实际 %s（agent 与契约仓的编码口径漂移）", c.ID, want.Hash, got)
	}
}

// yamlCase 是语料里 fee_defaults_via_yaml 的入参。
type yamlCase struct {
	StrategyType string `json:"strategy_type"`
	StrategyName string `json:"strategy_name"`
	Description  string `json:"description"`
	Options      *struct {
		StartDate      string  `json:"start_date"`
		EndDate        string  `json:"end_date"`
		InitialCapital float64 `json:"initial_capital"`
		RebalanceFreq  string  `json:"rebalance_freq"`
	} `json:"options"`
}

// checkFeeDefaultsViaYAML 走真实链路：Intent → Generator → YAML 文本 → ParseConfig，
// 再从解析回来的 config 读 backtest 的默认费率。
//
// 游标（YAML 字符串）本身也参与断言：只比结构体会漏掉「生成器写死了过期默认值、
// 但加载器又把它覆盖回去」这类双向漂移。
func checkFeeDefaultsViaYAML(t *testing.T, c caseSpec) {
	t.Helper()
	var in yamlCase
	var want struct {
		CommissionRate float64 `json:"commission_rate"`
		SlippageRate   float64 `json:"slippage_rate"`
	}
	decodePair(t, c, &in, &want)

	it := &intent.Intent{
		StrategyType: intent.StrategyType(in.StrategyType),
		StrategyName: in.StrategyName,
		Description:  in.Description,
		Timeframe:    "1d",
		Universe:     "csi300",
	}

	gen := yamlgen.NewGenerator()
	var text string
	if in.Options != nil {
		text = gen.GenerateWithOptions(it, yamlgen.GenerateOptions{
			StartDate:      in.Options.StartDate,
			EndDate:        in.Options.EndDate,
			InitialCapital: in.Options.InitialCapital,
			RebalanceFreq:  in.Options.RebalanceFreq,
		})
	} else {
		text = gen.Generate(it)
	}
	if text == "" {
		t.Fatalf("用例 %q: 生成器返回空 YAML", c.ID)
	}

	cfg, err := yamlgen.ParseConfig(text)
	if err != nil {
		t.Fatalf("用例 %q: 生成器产出的 YAML 无法解析: %v\n---\n%s", c.ID, err, text)
	}

	// 语料期望 + 契约常量，两条同时钉：前者抓 agent 自己的漂移，后者抓
	// 「agent 与 core 各自写死、但一起漂到同一个错值」的共谋漂移。
	if !almostEqual(cfg.Backtest.CommissionRate, want.CommissionRate) {
		t.Errorf("用例 %q: YAML 默认 commission_rate 期望 %v，实际 %v", c.ID, want.CommissionRate, cfg.Backtest.CommissionRate)
	}
	if !almostEqual(cfg.Backtest.CommissionRate, fees.DefaultCommissionRate) {
		t.Errorf("用例 %q: YAML 默认 commission_rate=%v 与 fees.DefaultCommissionRate=%v 漂移",
			c.ID, cfg.Backtest.CommissionRate, fees.DefaultCommissionRate)
	}
	if !almostEqual(cfg.Backtest.SlippageRate, want.SlippageRate) {
		t.Errorf("用例 %q: YAML 默认 slippage_rate 期望 %v，实际 %v", c.ID, want.SlippageRate, cfg.Backtest.SlippageRate)
	}
	if !almostEqual(cfg.Backtest.SlippageRate, fees.DefaultSlippageRate) {
		t.Errorf("用例 %q: YAML 默认 slippage_rate=%v 与 fees.DefaultSlippageRate=%v 漂移",
			c.ID, cfg.Backtest.SlippageRate, fees.DefaultSlippageRate)
	}
}

// ─── 语料读取与公共工具 ──────────────────────────────────────

type caseSpec struct {
	ID     string          `json:"id"`
	Kind   string          `json:"kind"`
	Sides  []string        `json:"sides"`
	Input  json.RawMessage `json:"input"`
	Expect json.RawMessage `json:"expect"`
}

// loadCases 读取并校验语料；任何缺陷都返回错误（空语料/重复 id/未知侧…）。
func loadCases(dir string) ([]caseSpec, error) {
	path := filepath.Join(dir, "cases.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取语料 %s: %w", path, err)
	}
	var cases []caseSpec
	if err := json.Unmarshal(raw, &cases); err != nil {
		return nil, fmt.Errorf("解析语料 %s: %w", path, err)
	}
	if len(cases) == 0 {
		return nil, fmt.Errorf("语料 %s 为空（空语料 = 假绿）", path)
	}
	seen := make(map[string]bool, len(cases))
	for i, c := range cases {
		if c.ID == "" {
			return nil, fmt.Errorf("语料 %s 第 %d 条缺少 id", path, i)
		}
		if seen[c.ID] {
			return nil, fmt.Errorf("语料 %s 里 id %q 重复", path, c.ID)
		}
		seen[c.ID] = true
		if c.Kind == "" {
			return nil, fmt.Errorf("用例 %q 缺少 kind", c.ID)
		}
		if len(c.Sides) == 0 {
			return nil, fmt.Errorf("用例 %q 的 sides 为空（不再明侧的用例永远不会被执行）", c.ID)
		}
		for _, s := range c.Sides {
			if s != "contracts" && s != "core" && s != "agent" {
				return nil, fmt.Errorf("用例 %q 声明了未知侧 %q（已知：agent/contracts/core）", c.ID, s)
			}
		}
		if len(c.Input) == 0 {
			return nil, fmt.Errorf("用例 %q 缺少 input", c.ID)
		}
		if len(c.Expect) == 0 {
			return nil, fmt.Errorf("用例 %q 缺少 expect", c.ID)
		}
	}
	return cases, nil
}

func declaresSide(sides []string, want string) bool {
	for _, s := range sides {
		if s == want {
			return true
		}
	}
	return false
}

// decodePair 严格解析 input/expect：未知字段报错，避免语料键名打错静默变零值。
func decodePair(t *testing.T, c caseSpec, input, expect any) {
	t.Helper()
	if err := decodeStrict(c.Input, input); err != nil {
		t.Fatalf("用例 %q 的 input 解析失败: %v", c.ID, err)
	}
	if err := decodeStrict(c.Expect, expect); err != nil {
		t.Fatalf("用例 %q 的 expect 解析失败: %v", c.ID, err)
	}
}

func decodeStrict(raw json.RawMessage, dst any) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}

func parseTimeArg(t *testing.T, c caseSpec, field, value string) time.Time {
	t.Helper()
	if value == "" {
		return time.Time{}
	}
	ts, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Fatalf("用例 %q 的 %s=%q 不是 RFC3339: %v", c.ID, field, value, err)
	}
	return ts
}

func parseDateArg(t *testing.T, c caseSpec, field, value string) time.Time {
	t.Helper()
	ts, err := time.Parse("2006-01-02", value)
	if err != nil {
		t.Fatalf("用例 %q 的 %s=%q 不是 YYYY-MM-DD: %v", c.ID, field, value, err)
	}
	return ts
}

func almostEqual(got, want float64) bool {
	return math.Abs(got-want) <= 1e-12
}
