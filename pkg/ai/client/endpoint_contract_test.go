package client

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// clientEndpointGaps：core 的 openapi.yaml 里**没有**、但本仓客户端今天仍在调用的端点。
//
// 只登记实测缺口，并且 TestClientEndpointGaps_AreStillGaps 会断言它们「今天仍然没有」——
// core 补登记后本测试变红，逼着删掉这一行，allowlist 不会静默腐烂。
var clientEndpointGaps = map[string]string{
	"/api/factor/compute":  "公式 × 符号的因子求值在 core 无对应端点（core 只有 /api/factor/compute-returns 的「因子类型 × 交易日」）；QT-50 ③-② 补粗粒度端点后删除本行",
	"/api/factor/evaluate": "IC/IR 求值的入参形状（formula + symbols）与 core /api/factor/compute-ic 不同；QT-50 ③-② 补粗粒度端点后删除本行",
}

var endpointLiteralRE = regexp.MustCompile(`"(/(?:api|health|metrics)[^"\s]*)"`)

// TestClientEndpoints_AreRegisteredInCoreOpenAPI 是 C2 ③ 的「粗粒度对齐」腿：
// 客户端发出的每一个 /api 路径都必须在 core 的 openapi.yaml 里有出处。
//
// 扫源码而不是维护一张手写清单：手写清单会在新增客户端时忘更新，那正是
// 幻影端点的成因（本仓 factor 客户端调的两个路径从未在 core 注册过）。
func TestClientEndpoints_AreRegisteredInCoreOpenAPI(t *testing.T) {
	doc, path := loadCoreOpenAPI(t)

	endpoints := scanClientEndpoints(t)
	if len(endpoints) < 3 {
		t.Fatalf("只扫到 %d 个客户端端点（%v），扫描器大概率失效", len(endpoints), endpoints)
	}

	var unknown []string
	for _, endpoint := range endpoints {
		if endpointRegistered(doc, endpoint) {
			continue
		}
		if _, allowed := clientEndpointGaps[endpoint]; allowed {
			continue
		}
		unknown = append(unknown, endpoint)
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		t.Errorf("%s：客户端调用了 openapi 未登记的端点：%s（新增端点先落 wire 契约，或登记进 clientEndpointGaps 并写明单号）",
			path, strings.Join(unknown, ", "))
	}
}

// TestClientEndpointGaps_AreStillGaps 是 allowlist 的防腐腿。
func TestClientEndpointGaps_AreStillGaps(t *testing.T) {
	doc, _ := loadCoreOpenAPI(t)
	for endpoint, why := range clientEndpointGaps {
		if endpointRegistered(doc, endpoint) {
			t.Errorf("openapi 已登记 %q，把它从 clientEndpointGaps 删掉（登记理由：%s）", endpoint, why)
		}
	}
}

// TestEndpointScanner_SeesKnownPhantom 是破坏验证的在案版：扫描器必须看见
// factor 客户端那两个真实幻影端点，否则上面那条腿就是空转。
func TestEndpointScanner_SeesKnownPhantom(t *testing.T) {
	endpoints := scanClientEndpoints(t)
	want := map[string]bool{"/api/factor/compute": false, "/api/factor/evaluate": false}
	for _, endpoint := range endpoints {
		if _, ok := want[endpoint]; ok {
			want[endpoint] = true
		}
	}
	for endpoint, seen := range want {
		if !seen {
			t.Errorf("扫描器漏掉了已存在的端点字面量 %q", endpoint)
		}
	}
}

// endpointRegistered 判断一个客户端路径是否有 openapi 出处。以 "/" 结尾的字面量
// 是动态段的拼接前缀（如 "/api/backtest/" + id），按前缀匹配。
func endpointRegistered(doc *openAPIDoc, endpoint string) bool {
	if _, ok := doc.Paths[endpoint]; ok {
		return true
	}
	if !strings.HasSuffix(endpoint, "/") {
		return false
	}
	for path := range doc.Paths {
		if strings.HasPrefix(path, endpoint) {
			return true
		}
	}
	return false
}

// scanClientEndpoints 扫本包非测试源码里的端点字面量，去重后排序。
func scanClientEndpoints(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("读取 client 包目录失败：%v", err)
	}
	seen := map[string]bool{}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		raw, err := os.ReadFile(filepath.Clean(name))
		if err != nil {
			t.Fatalf("读取 %s 失败：%v", name, err)
		}
		for _, m := range endpointLiteralRE.FindAllStringSubmatch(string(raw), -1) {
			seen[m[1]] = true
		}
	}
	out := make([]string, 0, len(seen))
	for endpoint := range seen {
		out = append(out, endpoint)
	}
	sort.Strings(out)
	return out
}
