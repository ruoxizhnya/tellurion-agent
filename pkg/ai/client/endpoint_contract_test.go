package client

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync/atomic"
	"testing"
)

// phantomFactorEndpoints：core 从未实现、因此**禁止**再出现在客户端源码里的端点。
//
// ADR-041：core 只注册 POST /api/factor/registrations 与反代 GET /api/factors/:name；
// /api/factor/compute 与 /api/factor/evaluate 是旧客户端凭空假设的，现已改为
// 显式「能力未提供」（ErrFactorCapabilityUnavailable）。这两条路径若重新出现，
// 说明有人又把它当成现役能力——测试必须红。
var phantomFactorEndpoints = []string{
	"/api/factor/compute",
	"/api/factor/evaluate",
}

// nonCoreClientEndpoints：由 core 以外的服务提供、因此不要求出现在 core openapi 的端点。
// 与 phantomFactorEndpoints 的区别：这里不是「客户端凭空假设」，而是归属别的服务。
// TestNonCoreClientEndpoints_AreStillExternal 会断言它们仍被源码引用、且 core openapi
// 仍然没有 —— 任一侧变化都会红，逼着重新判断归属。
var nonCoreClientEndpoints = map[string]string{
	"/api/v1/trading/calendar": "由 tellurion-data 提供（GET /api/v1/trading/calendar，契约见 tellurion-data:docs/api-schema.md）；agent CI 不 checkout data，故不纳入 core openapi 对拍",
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
		if _, external := nonCoreClientEndpoints[endpoint]; external {
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

// TestFactorClient_UnavailableWithoutHTTP 证明「不可用」是在客户端层判定的：
// 即便 baseURL 指向一个能应答的服务器，也不得发出任何请求（ADR-041）。
func TestFactorClient_UnavailableWithoutHTTP(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewFactorClient(srv.URL)

	if _, err := c.ComputeFactor(context.Background(), ComputeFactorRequest{
		Formula:   "ts_rank(close, 20)",
		Symbols:   []string{"000001.SZ"},
		StartDate: "2022-01-01",
		EndDate:   "2024-01-01",
	}); !errors.Is(err, ErrFactorCapabilityUnavailable) {
		t.Fatalf("ComputeFactor err = %v，期望 ErrFactorCapabilityUnavailable", err)
	}

	if _, err := c.EvaluateFactor(context.Background(), "ts_rank(close, 20)", []string{"000001.SZ"}, "2022-01-01", "2024-01-01"); !errors.Is(err, ErrFactorCapabilityUnavailable) {
		t.Fatalf("EvaluateFactor err = %v，期望 ErrFactorCapabilityUnavailable", err)
	}

	if n := atomic.LoadInt32(&hits); n != 0 {
		t.Fatalf("不可用路径发起了 %d 次 HTTP 请求，期望 0", n)
	}
}

// TestNoPhantomFactorEndpoints 是 ADR-041 的防腐腿：幻影端点不得回到源码。
func TestNoPhantomFactorEndpoints(t *testing.T) {
	scanned := map[string]bool{}
	for _, endpoint := range scanClientEndpoints(t) {
		scanned[endpoint] = true
	}
	for _, endpoint := range phantomFactorEndpoints {
		if scanned[endpoint] {
			t.Errorf("%q 重新出现在客户端源码里；core 没有该端点（ADR-041），应保持「能力未提供」，不得发请求", endpoint)
		}
	}
}

// TestNonCoreClientEndpoints_AreStillExternal 是归属表两侧的防腐腿：
// 端点必须仍出现在客户端源码里，且 core openapi 必须仍然没有它。
func TestNonCoreClientEndpoints_AreStillExternal(t *testing.T) {
	doc, _ := loadCoreOpenAPI(t)
	scanned := map[string]bool{}
	for _, endpoint := range scanClientEndpoints(t) {
		scanned[endpoint] = true
	}
	for endpoint, why := range nonCoreClientEndpoints {
		if !scanned[endpoint] {
			t.Errorf("%q 已不在客户端源码里，删掉这条登记（%s）", endpoint, why)
		}
		if endpointRegistered(doc, endpoint) {
			t.Errorf("core openapi 已登记 %q，把它从 nonCoreClientEndpoints 删掉（%s）", endpoint, why)
		}
	}
}

// TestEndpointScanner_SeesKnownLiveEndpoint 是破坏验证的在案版：扫描器必须看见
// 真实存在的端点字面量，否则上面的防腐腿就是空转（扫不到 = 永远绿）。
func TestEndpointScanner_SeesKnownLiveEndpoint(t *testing.T) {
	endpoints := scanClientEndpoints(t)
	found := false
	for _, endpoint := range endpoints {
		if strings.HasPrefix(endpoint, "/api/backtest") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("扫描器没扫到任何 /api/backtest 端点（扫到 %v），扫描器大概率失效", endpoints)
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
