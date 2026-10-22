package client

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// 本文件是 REV-01-A ③-1 的「与契约对拍」腿：客户端 DTO 的 json 键必须能在
// core 的 openapi.yaml 里找到出处，禁止手写一个漂移的私有形状。
//
// 为什么不在本仓复制一份 openapi：复制的那份一定漂移（ui 仓的同一条规则）。
// 测试在 core 源码可见时直接读它的 openapi.yaml；单仓 checkout（R1）时显式 Skip ——
// agent CI 会 checkout core（见 .github/workflows/ci.yml），CI 里这条腿是跑的。

// openapiKnownGaps：core 的 **contracts 里有、openapi.yaml 里漏登记**的字段。
// 只登记实测确认的缺口；每个键同时断言「openapi 里确实还没有」——
// core 补登记后本测试会红，逼着把这一行删掉，allowlist 不会静默腐烂。
var openapiKnownGaps = map[string]string{
	"error":             "contracts.BacktestResponse.Error（K9: failed ⇒ error 必填）；openapi.yaml 未登记",
	"invalid_reasons":   "contracts.BacktestResponse.InvalidReasons（OBS-01）；openapi.yaml 未登记",
	"universe_max_size": "contracts.BacktestResponse.UniverseMaxSize（OBS-01）；openapi.yaml 未登记",
}

type openAPIDoc struct {
	Components struct {
		Schemas map[string]openAPISchema `yaml:"schemas"`
	} `yaml:"components"`
	// 只取 POST /api/backtest 的 202 内联 schema（异步分支的 {"job_id","status"}）。
	Paths map[string]struct {
		Post struct {
			Responses map[string]struct {
				Content map[string]struct {
					Schema openAPISchema `yaml:"schema"`
				} `yaml:"content"`
			} `yaml:"responses"`
		} `yaml:"post"`
	} `yaml:"paths"`
}

type openAPISchema struct {
	Required   []string       `yaml:"required"`
	Properties map[string]any `yaml:"properties"`
}

func loadCoreOpenAPI(t *testing.T) (*openAPIDoc, string) {
	t.Helper()
	candidates := []string{
		os.Getenv("TELLURION_OPENAPI"),
		// CI：agent 仓在 workspace 根，core checkout 到 <根>/core/。
		filepath.Join("..", "..", "..", "core", "docs", "openapi.yaml"),
		// 本地：两仓在工作区同级。
		filepath.Join("..", "..", "..", "..", "tellurion", "docs", "openapi.yaml"),
	}
	for _, path := range candidates {
		if path == "" {
			continue
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var doc openAPIDoc
		if err := yaml.Unmarshal(raw, &doc); err != nil {
			t.Fatalf("解析 %s 失败: %v", path, err)
		}
		return &doc, path
	}
	t.Skip("找不到 core 的 openapi.yaml（单仓 checkout 的正常情况；用 TELLURION_OPENAPI 指定路径可在本地跑这条腿）")
	return nil, ""
}

// TestOpenAPI_RequestKeysMatchContract 钉住请求侧：发出去的每个键都必须是契约属性，
// 契约 required 的每个键都必须发。`strategy_name` 这种旧名字会在这里当场红。
func TestOpenAPI_RequestKeysMatchContract(t *testing.T) {
	doc, path := loadCoreOpenAPI(t)
	if err := checkRequestKeys(doc, jsonKeys(BacktestRequest{})); err != nil {
		t.Fatalf("%s：请求 DTO 与 core BacktestRequest 不一致：%v", path, err)
	}
}

// TestOpenAPI_ResponseKeysMatchContract 钉住响应侧：解出来的每个键都要有出处
// （BacktestResponse 或 job 行的 BacktestJob），缺口只认 openapiKnownGaps 里登记过的。
func TestOpenAPI_ResponseKeysMatchContract(t *testing.T) {
	doc, path := loadCoreOpenAPI(t)
	if err := checkResponseKeys(doc, jsonKeys(BacktestResponse{}), openapiKnownGaps); err != nil {
		t.Fatalf("%s：响应 DTO 与 core 契约不一致：%v", path, err)
	}
}

// TestOpenAPI_GapsAreStillGaps 是 allowlist 的防腐腿：登记为缺口的字段，
// openapi 里今天必须真的没有。core 补登记后这里红 —— 删掉 allowlist 那行即可。
func TestOpenAPI_GapsAreStillGaps(t *testing.T) {
	doc, _ := loadCoreOpenAPI(t)
	for key, why := range openapiKnownGaps {
		if _, ok := doc.Components.Schemas["BacktestResponse"].Properties[key]; ok {
			t.Errorf("openapi 已登记 %q，把它从 openapiKnownGaps 删掉（登记理由：%s）", key, why)
		}
	}
}

// TestOpenAPI_CheckerRejectsLegacyShapes 是破坏验证的长期在案版：
// 旧的两处漂移（请求发 strategy_name、响应只认嵌套 result）必须被同一个检查判红。
// 这两条形状就是 REV-01-A 开工前线上的真实形状，不是假想。
func TestOpenAPI_CheckerRejectsLegacyShapes(t *testing.T) {
	doc, _ := loadCoreOpenAPI(t)

	legacyRequest := []string{"strategy_name", "stock_pool", "start_date", "end_date"}
	if err := checkRequestKeys(doc, legacyRequest); err == nil {
		t.Error("旧请求键集（strategy_name）必须红")
	} else if !strings.Contains(err.Error(), "strategy_name") {
		t.Errorf("红因要点名 strategy_name，got %v", err)
	}

	// 旧响应 DTO：{result, error, job_id, status} —— 每个键今天都能找到出处
	// （result 在 job 行、job_id 在 202、error 在缺口表），真正的漂移是**它不解平铺体的指标**：
	// 旧代码只认嵌套 result，同步 POST 的指标全被丢掉（Result 恒 nil 的根因）。
	// 所以这条腿靠「契约声明的数值指标必须都被解码」判红，而不是靠未知键。
	legacyResponse := []string{"result", "error", "job_id", "status"}
	if err := checkResponseKeys(doc, legacyResponse, openapiKnownGaps); err == nil {
		t.Error("旧响应键集（只认嵌套 result、不解平铺指标）必须红")
	} else if !strings.Contains(err.Error(), "sharpe_ratio") {
		t.Errorf("红因要点名缺掉的指标（如 sharpe_ratio），got %v", err)
	}

	// 未知键那条腿单独钉：编造的键不能被放过。
	bogus := append(jsonKeys(BacktestResponse{}), "metrics_typo")
	if err := checkResponseKeys(doc, bogus, openapiKnownGaps); err == nil {
		t.Error("编造的响应键必须红")
	} else if !strings.Contains(err.Error(), "metrics_typo") {
		t.Errorf("红因要点名编造的键，got %v", err)
	}
}

func checkRequestKeys(doc *openAPIDoc, sent []string) error {
	schema, ok := doc.Components.Schemas["BacktestRequest"]
	if !ok {
		return fmt.Errorf("openapi 里没有 BacktestRequest schema")
	}
	have := map[string]bool{}
	for k := range schema.Properties {
		have[k] = true
	}
	var unknown []string
	for _, k := range sent {
		if !have[k] {
			unknown = append(unknown, k)
		}
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		return fmt.Errorf("发送的键不在 BacktestRequest.properties 里：%s（契约属性：%s）",
			strings.Join(unknown, ", "), strings.Join(sortedKeys(schema.Properties), ", "))
	}
	sentSet := map[string]bool{}
	for _, k := range sent {
		sentSet[k] = true
	}
	var missing []string
	for _, k := range schema.Required {
		if !sentSet[k] {
			missing = append(missing, k)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return fmt.Errorf("契约必填但客户端没发：%s", strings.Join(missing, ", "))
	}
	return nil
}

func checkResponseKeys(doc *openAPIDoc, decoded []string, gaps map[string]string) error {
	resp, ok := doc.Components.Schemas["BacktestResponse"]
	if !ok {
		return fmt.Errorf("openapi 里没有 BacktestResponse schema")
	}
	job := doc.Components.Schemas["BacktestJob"]

	// 202 异步分支是内联 schema（{"job_id","status"}），不在 components 里。
	async := map[string]bool{}
	if path, ok := doc.Paths["/api/backtest"]; ok {
		for _, media := range path.Post.Responses["202"].Content {
			for k := range media.Schema.Properties {
				async[k] = true
			}
		}
	}

	var unknown []string
	for _, k := range decoded {
		if _, inResp := resp.Properties[k]; inResp {
			continue
		}
		if _, inJob := job.Properties[k]; inJob {
			continue
		}
		if async[k] {
			continue
		}
		if _, allowed := gaps[k]; allowed {
			continue
		}
		unknown = append(unknown, k)
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		return fmt.Errorf("解码的键没有契约出处：%s（既不在 BacktestResponse / BacktestJob / 202 内联 schema，也不在缺口登记表里）",
			strings.Join(unknown, ", "))
	}

	// 完整性：契约里的每个数值指标（number / integer）都必须被解码。
	// 漏一个就是静默丢一个评分输入 —— 旧 DTO 只认嵌套 result 时，整批指标都是漏的。
	if missing := missingNumericMetrics(resp, decoded); len(missing) > 0 {
		return fmt.Errorf("缺少契约指标字段（number/integer）：%s", strings.Join(missing, ", "))
	}
	return nil
}

// notPerformanceMetrics 是契约里的数值字段、但**不是绩效指标**的例外：
// initial_capital 是请求入参的回显（provenance），本仓不参与评分，不需要解。
var notPerformanceMetrics = map[string]bool{
	"initial_capital": true,
}

// missingNumericMetrics 返回契约里声明为 number / integer、但 DTO 没接的字段。
// 只查数值指标：字符串型 provenance（strategy / fill_timing）与数组型明细不由本仓消费。
func missingNumericMetrics(schema openAPISchema, decoded []string) []string {
	have := map[string]bool{}
	for _, k := range decoded {
		have[k] = true
	}
	var missing []string
	for name, raw := range schema.Properties {
		if notPerformanceMetrics[name] {
			continue
		}
		prop, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		typ, _ := prop["type"].(string)
		if (typ == "number" || typ == "integer") && !have[name] {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)
	return missing
}

// jsonKeys 反射出 DTO 的 json 键。改错一个 tag，上面两条对拍腿就会红 ——
// 这就是「与 openapi 对拍」对 DTO 侧的要求。
func jsonKeys(v any) []string {
	t := reflect.TypeOf(v)
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	var keys []string
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		tag := f.Tag.Get("json")
		name := strings.Split(tag, ",")[0]
		if name == "" || name == "-" {
			continue
		}
		keys = append(keys, name)
	}
	return keys
}

func sortedKeys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
