package client

// QT-32-B：资产服务客户端的 wire 用例。
//
// 这些用例钉的是「请求的形状与顺序」，不是服务端行为：假件只回放资产服务的
// 契约（201 / 200 / 202 / BuildJob 状态机），断言本仓发出去的每一步都对得上。
// 其中两条最容易写错、也最贵：
//   - dsl 腿**不许**调 build（服务端会以 unsupported_carrier 拒绝）；
//   - succeeded 而不带 artifact_hash 是服务端违约，必须报错而不是拿空串当坐标。

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testVersionHash = "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"

type recordedCall struct {
	method string
	path   string
	body   string
	auth   string
}

type fakeAssetService struct {
	t     *testing.T
	calls []recordedCall

	mu      sync.Mutex
	jobStep int
	jobs    []buildJobView
}

func (f *fakeAssetService) handle(w http.ResponseWriter, r *http.Request) {
	raw, _ := io.ReadAll(r.Body)
	f.mu.Lock()
	f.calls = append(f.calls, recordedCall{
		method: r.Method,
		path:   r.URL.Path,
		body:   string(raw),
		auth:   r.Header.Get("Authorization"),
	})
	f.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/api/strategies":
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]string{"id": "stg-1"})

	case r.Method == http.MethodPut && r.URL.Path == "/api/strategies/stg-1/source":
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"version_hash": testVersionHash})

	case r.Method == http.MethodPost && r.URL.Path == "/api/strategies/stg-1/build":
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(map[string]string{"job_id": "job-1"})

	case r.Method == http.MethodGet && r.URL.Path == "/api/strategies/stg-1/builds/job-1":
		f.mu.Lock()
		job := buildJobView{JobID: "job-1", State: "running"}
		if f.jobStep < len(f.jobs) {
			job = f.jobs[f.jobStep]
			f.jobStep++
		}
		f.mu.Unlock()
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(job)

	default:
		f.t.Errorf("unexpected asset-service request: %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusTeapot)
	}
}

func (f *fakeAssetService) recorded() []recordedCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]recordedCall, len(f.calls))
	copy(out, f.calls)
	return out
}

func (f *fakeAssetService) routes() []string {
	out := make([]string, 0, len(f.recorded()))
	for _, c := range f.recorded() {
		out = append(out, c.method+" "+c.path)
	}
	return out
}

func newFakeAssetService(t *testing.T, jobs []buildJobView) (*httptest.Server, *fakeAssetService) {
	t.Helper()
	fake := &fakeAssetService{t: t, jobs: jobs}
	srv := httptest.NewServer(http.HandlerFunc(fake.handle))
	t.Cleanup(srv.Close)
	return srv, fake
}

// TestStrategyAssetClient_DSLRegistersSourceWithoutBuild：dsl 的坐标只有
// strategy_id + version_hash，且**绝不调 build**（服务端对 dsl 会 400）。
func TestStrategyAssetClient_DSLRegistersSourceWithoutBuild(t *testing.T) {
	srv, fake := newFakeAssetService(t, nil)
	c, err := NewStrategyAssetClient(StrategyAssetConfig{BaseURL: srv.URL, Carrier: AssetCarrierDSL})
	require.NoError(t, err)

	source := "package x\nfunc f() {}\n"
	ref, err := c.Register(context.Background(), source)
	require.NoError(t, err)

	assert.Equal(t, AssetRef{
		StrategyID:  "stg-1",
		Carrier:     AssetCarrierDSL,
		VersionHash: testVersionHash,
	}, ref, "dsl 不带 artifact_hash，也不许拿 version_hash 冒充它")

	assert.Equal(t, []string{
		"POST /api/strategies",
		"PUT /api/strategies/stg-1/source",
	}, fake.routes(), "dsl 腿恰两步：建定义 → 传源码；多一步 build 就是错的")

	calls := fake.recorded()
	var create map[string]any
	require.NoError(t, json.Unmarshal([]byte(calls[0].body), &create))
	assert.Equal(t, "dsl", create["carrier"])
	assert.Equal(t, "market", create["scope"], "缺省 scope 必须显式发，服务端只校验存在")
	assert.Equal(t, "OnBar", create["entry"])
	_, hasABI := create["abi_version"]
	assert.False(t, hasABI, "dsl 的 abi_version 必须为空（服务端对非空会 invalid_abi_version）")

	var put map[string]string
	require.NoError(t, json.Unmarshal([]byte(calls[1].body), &put))
	assert.Equal(t, source, put["source"], "传的必须是原始字节，不能自行改写")
}

// TestStrategyAssetClient_SendsServiceTokenOnEveryCall：配了 token 就必须在
// **每一次**调用上带 `Authorization: Bearer`（含轮询 GET）。5 条写端点在
// strategy 侧收紧后按 `role=service` 判定，少带一次就是一个 401。
func TestStrategyAssetClient_SendsServiceTokenOnEveryCall(t *testing.T) {
	srv, fake := newFakeAssetService(t, []buildJobView{
		{JobID: "job-1", State: "running"},
		{JobID: "job-1", State: "succeeded", ArtifactHash: "abcdef"},
	})
	c, err := NewStrategyAssetClient(StrategyAssetConfig{
		BaseURL:      srv.URL,
		Carrier:      AssetCarrierWASM,
		ABIVersion:   "wasm32/onbar-v1",
		Token:        "  svc-token-abc  ",
		PollInterval: time.Millisecond,
	})
	require.NoError(t, err)

	_, err = c.Register(context.Background(), "package x\n")
	require.NoError(t, err)

	calls := fake.recorded()
	require.Len(t, calls, 5, "wasm 腿五步：建定义 → 传源码 → build → 两次轮询")
	for _, call := range calls {
		assert.Equal(t, "Bearer svc-token-abc", call.auth,
			"%s %s 必须带 service token（首尾空白应被去掉）", call.method, call.path)
	}
}

// TestStrategyAssetClient_EmptyTokenSendsNoAuthorizationHeader：空（或纯空白）
// token = 公开期行为**逐字节不变**——不发 Authorization、不报错、照常登记。
// 这是「先发布带 token 的能力、后收紧服务端」两步之间不断链的前提。
func TestStrategyAssetClient_EmptyTokenSendsNoAuthorizationHeader(t *testing.T) {
	srv, fake := newFakeAssetService(t, nil)
	c, err := NewStrategyAssetClient(StrategyAssetConfig{
		BaseURL: srv.URL, Carrier: AssetCarrierDSL, Token: "   ",
	})
	require.NoError(t, err)

	_, err = c.Register(context.Background(), "package x\n")
	require.NoError(t, err, "缺 token 不得拒调用：公开期链路必须照早先行为工作")

	calls := fake.recorded()
	require.Len(t, calls, 2)
	for _, call := range calls {
		assert.Empty(t, call.auth,
			"空 token 不发 Authorization（%s %s）", call.method, call.path)
	}
}

// TestStrategyAssetClient_WasmBuildsAndPollsToArtifact：产物载体多两步
// （build + 轮询），artifact_hash 只能来自终态作业。
func TestStrategyAssetClient_WasmBuildsAndPollsToArtifact(t *testing.T) {
	srv, fake := newFakeAssetService(t, []buildJobView{
		{JobID: "job-1", State: "running"},
		{JobID: "job-1", State: "succeeded", ArtifactHash: "abcdef"},
	})
	c, err := NewStrategyAssetClient(StrategyAssetConfig{
		BaseURL:      srv.URL,
		Carrier:      AssetCarrierWASM,
		ABIVersion:   "wasm32/onbar-v1",
		Toolchain:    "tinygo-0.34",
		PollInterval: time.Millisecond,
	})
	require.NoError(t, err)

	ref, err := c.Register(context.Background(), "package x\n")
	require.NoError(t, err)

	assert.Equal(t, "stg-1", ref.StrategyID)
	assert.Equal(t, testVersionHash, ref.VersionHash)
	assert.Equal(t, "abcdef", ref.ArtifactHash)

	assert.Equal(t, []string{
		"POST /api/strategies",
		"PUT /api/strategies/stg-1/source",
		"POST /api/strategies/stg-1/build",
		"GET /api/strategies/stg-1/builds/job-1",
		"GET /api/strategies/stg-1/builds/job-1",
	}, fake.routes())

	buildBody := fake.recorded()[2].body
	var build map[string]string
	require.NoError(t, json.Unmarshal([]byte(buildBody), &build))
	assert.Equal(t, testVersionHash, build["version_hash"], "编译必须锚在刚签发的版本上")
	assert.Equal(t, "tinygo-0.34", build["toolchain"])
}

func TestStrategyAssetClient_BuildFailureCarriesErrorCode(t *testing.T) {
	srv, _ := newFakeAssetService(t, []buildJobView{
		{JobID: "job-1", State: "failed", ErrorCode: "toolchain_unavailable", Error: "tinygo: not found"},
	})
	c, err := NewStrategyAssetClient(StrategyAssetConfig{
		BaseURL: srv.URL, Carrier: AssetCarrierWASM, ABIVersion: "wasm32/onbar-v1",
		PollInterval: time.Millisecond,
	})
	require.NoError(t, err)

	ref, err := c.Register(context.Background(), "package x\n")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "job-1")
	assert.Contains(t, err.Error(), "toolchain_unavailable", "错误码是运维的检索键，不能吞")
	assert.Contains(t, err.Error(), "tinygo: not found")

	// 失败时保留已拿到的部分坐标：链据此说明走到了哪一步。
	assert.Equal(t, "stg-1", ref.StrategyID)
	assert.Equal(t, testVersionHash, ref.VersionHash)
	assert.Empty(t, ref.ArtifactHash)
}

func TestStrategyAssetClient_SucceededWithoutArtifactIsError(t *testing.T) {
	srv, _ := newFakeAssetService(t, []buildJobView{{JobID: "job-1", State: "succeeded"}})
	c, err := NewStrategyAssetClient(StrategyAssetConfig{
		BaseURL: srv.URL, Carrier: AssetCarrierWASM, ABIVersion: "wasm32/onbar-v1",
		PollInterval: time.Millisecond,
	})
	require.NoError(t, err)

	_, err = c.Register(context.Background(), "package x\n")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "succeeded without artifact_hash",
		"空串绝不能当成坐标：那会让下游拿到一个永远解析不到的唯一标识")
}

func TestStrategyAssetClient_PollTimeout(t *testing.T) {
	srv, _ := newFakeAssetService(t, nil) // 永远 running
	c, err := NewStrategyAssetClient(StrategyAssetConfig{
		BaseURL: srv.URL, Carrier: AssetCarrierWASM, ABIVersion: "wasm32/onbar-v1",
		PollInterval: time.Millisecond, PollTimeout: 20 * time.Millisecond,
	})
	require.NoError(t, err)

	_, err = c.Register(context.Background(), "package x\n")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "still \"running\"", "超时错误要说清作业卡在哪个状态")
}

func TestStrategyAssetClient_Non2xxSurfacesStatusAndBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"error":"strategy already exists","code":"conflict"}`))
	}))
	t.Cleanup(srv.Close)

	c, err := NewStrategyAssetClient(StrategyAssetConfig{BaseURL: srv.URL})
	require.NoError(t, err)

	_, err = c.Register(context.Background(), "package x\n")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "status 409")
	assert.Contains(t, err.Error(), "strategy already exists",
		"上游 4xx 文案本来就是给人看的，要透传")
}

func TestNewStrategyAssetClient_RejectsUnknownCarrier(t *testing.T) {
	_, err := NewStrategyAssetClient(StrategyAssetConfig{BaseURL: "http://127.0.0.1:1", Carrier: "so-v2"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown carrier")
}

func TestNewStrategyAssetClient_RequiresBaseURL(t *testing.T) {
	_, err := NewStrategyAssetClient(StrategyAssetConfig{BaseURL: "  "})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "base url is empty")
}

// 缺省值：dsl + market + OnBar + 轮询上界；调用方不传也要有确定的形状。
func TestNewStrategyAssetClient_Defaults(t *testing.T) {
	c, err := NewStrategyAssetClient(StrategyAssetConfig{BaseURL: "http://127.0.0.1:8090/"})
	require.NoError(t, err)
	assert.Equal(t, AssetCarrierDSL, c.Carrier())
	assert.Equal(t, "http://127.0.0.1:8090", c.cfg.BaseURL, "尾部斜杠要去掉，否则拼出 //api")
	assert.Equal(t, "market", c.cfg.Scope)
	assert.Equal(t, "OnBar", c.cfg.Entry)
	assert.Equal(t, time.Second, c.cfg.PollInterval)
	assert.Equal(t, 2*time.Minute, c.cfg.PollTimeout)
}

// 编译失败的错误文案里不能出现空荡荡的一截（error_code/error 都空时也不许留 "  "）。
func TestStrategyAssetClient_BuildFailureWithoutDetailIsStillReadable(t *testing.T) {
	srv, _ := newFakeAssetService(t, []buildJobView{{JobID: "job-1", State: "failed"}})
	c, err := NewStrategyAssetClient(StrategyAssetConfig{
		BaseURL: srv.URL, Carrier: AssetCarrierWASM, ABIVersion: "wasm32/onbar-v1",
		PollInterval: time.Millisecond,
	})
	require.NoError(t, err)

	_, err = c.Register(context.Background(), "package x\n")
	require.Error(t, err)
	assert.True(t, strings.HasPrefix(err.Error(), "build job job-1 failed:"),
		"无细节时也要能定位到作业：got %q", err.Error())
}
