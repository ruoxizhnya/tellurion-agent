package client

// S3-c-2：strategy.list / strategy.get 改走 core 的 `GET /api/strategies`。
// 本文件钉住三件事：
//  1. 请求形状是 **无 query 的 GET** —— core 只有 type/active 全空时才走
//     ListWithDB 合并分支（带上 ?active=true 会退化成「只查 DB」，内置策略整批消失）；
//  2. 响应逐字段透传（name / description / parameters，含 default/min/max）；
//  3. 端点字面量落在 contract 扫描器的视野里（TestClientEndpoints_AreRegisteredInCoreOpenAPI）。

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStrategyClient_ListStrategies_RequestShape(t *testing.T) {
	var (
		gotMethod string
		gotPath   string
		gotQuery  string
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"strategies":[{"name":"momentum","description":"Momentum strategy","parameters":[]}]}`))
	}))
	defer srv.Close()

	infos, err := NewStrategyClient(srv.URL).ListStrategies(t.Context())
	require.NoError(t, err)

	assert.Equal(t, http.MethodGet, gotMethod)
	assert.Equal(t, "/api/strategies", gotPath)
	// 无 query 是合并分支（注册表 ∪ DB 活跃项）的开关，不能被「顺手加个过滤」改掉。
	assert.Empty(t, gotQuery, "带 query 会切到只查 DB 的分支，内置策略会消失")

	require.Len(t, infos, 1)
	assert.Equal(t, "momentum", infos[0].Name)
}

func TestStrategyClient_ListStrategies_FieldsFlowThrough(t *testing.T) {
	body := `{
		"strategies": [
			{
				"name": "dual_momentum",
				"description": "Dual momentum strategy",
				"parameters": [
					{"name": "lookback", "type": "int", "default": 20, "description": "Lookback window", "min": 5, "max": 120},
					{"name": "enabled", "type": "bool", "default": true, "description": "", "min": 0, "max": 0}
				]
			}
		]
	}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	infos, err := NewStrategyClient(srv.URL).ListStrategies(t.Context())
	require.NoError(t, err)
	require.Len(t, infos, 1)

	info := infos[0]
	assert.Equal(t, "dual_momentum", info.Name)
	assert.Equal(t, "Dual momentum strategy", info.Description)
	require.Len(t, info.Parameters, 2)

	assert.Equal(t, StrategyParameter{
		Name:        "lookback",
		Type:        "int",
		Default:     float64(20), // JSON 数字解成 float64，与 core 的 jsonTypeName 口径一致
		Description: "Lookback window",
		Min:         5,
		Max:         120,
	}, info.Parameters[0])
	assert.Equal(t, true, info.Parameters[1].Default)
}

func TestStrategyClient_ListStrategies_EmptyList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"strategies":[]}`))
	}))
	defer srv.Close()

	infos, err := NewStrategyClient(srv.URL).ListStrategies(t.Context())
	require.NoError(t, err)
	assert.Empty(t, infos)
}

func TestStrategyClient_ListStrategies_StatusErrors(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
		want   string
	}{
		{
			name:   "401 未认证",
			status: http.StatusUnauthorized,
			body:   `{"error":"missing or invalid bearer token"}`,
			want:   "missing or invalid bearer token",
		},
		{
			name:   "500 params blob 损坏",
			status: http.StatusInternalServerError,
			body:   `{"error":"parse strategy \"broken\" params: invalid character"}`,
			want:   "parse strategy",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()

			_, err := NewStrategyClient(srv.URL).ListStrategies(t.Context())
			require.Error(t, err)
			assert.Contains(t, err.Error(), "status")
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestStrategyClient_ListStrategies_DecodeError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`not-json`))
	}))
	defer srv.Close()

	_, err := NewStrategyClient(srv.URL).ListStrategies(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decode list_strategies response")
}

func TestStrategyClient_ListStrategies_TransportError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := srv.URL
	srv.Close() // 先关掉：连接必失败

	_, err := NewStrategyClient(url).ListStrategies(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "request failed")
}

func TestStrategyClient_BaseURLTrailingSlashIsTrimmed(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"strategies":[]}`))
	}))
	defer srv.Close()

	_, err := NewStrategyClient(srv.URL + "/").ListStrategies(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "/api/strategies", gotPath)
}

// TestStrategyClient_ResponseMatchesCoreListWithDBShape 是「与 core 实际输出
// 对拍」的在案版：把 core `pkg/strategy.DB.ListWithDB` 的 JSON 形状（注册表条目
// 用 Parameter 描述符、DB 条目用同名字段投影）整段喂进来，逐字段解出。
// core 改字段名时这里红。
func TestStrategyClient_ResponseMatchesCoreListWithDBShape(t *testing.T) {
	body := `{
		"strategies": [
			{
				"name": "seeded_momentum",
				"description": "Seeded from DB",
				"parameters": [
					{"name": "lookback", "type": "float", "default": 20.0, "description": "", "min": 0, "max": 0}
				]
			},
			{
				"name": "momentum",
				"description": "Registry momentum",
				"parameters": null
			}
		]
	}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	infos, err := NewStrategyClient(srv.URL).ListStrategies(t.Context())
	require.NoError(t, err)
	require.Len(t, infos, 2)

	var names []string
	for _, info := range infos {
		names = append(names, info.Name)
	}
	assert.Equal(t, []string{"seeded_momentum", "momentum"}, names)

	raw, err := json.Marshal(infos[0])
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"lookback"`)
}

// TestStrategyClient_EndpointIsScannedByContractTest 钉住验收里的「端点字面量进
// TestClientEndpoints_AreRegisteredInCoreOpenAPI」：扫描器必须看见本端点，
// 否则那条护栏对 strategy 客户端就是空转。
func TestStrategyClient_EndpointIsScannedByContractTest(t *testing.T) {
	endpoints := scanClientEndpoints(t)
	found := false
	for _, endpoint := range endpoints {
		if endpoint == "/api/strategies" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("扫描器没看到 /api/strategies（现有端点：%s）", strings.Join(endpoints, ", "))
	}
}
