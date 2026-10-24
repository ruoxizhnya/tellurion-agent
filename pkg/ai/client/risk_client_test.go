package client

// S3-c-1：get_market_regime 改走 core 的 `POST /api/risk/detect_regime`
// 粗粒度形状。本文件钉住三件事：
//  1. 请求形状只有 symbol / start_date / end_date —— **不带内联 data**
//     （内联在 core 侧优先级更高，带上就把服务端取数分支挤掉了）；
//  2. 响应逐字段透传，错误状态码与 core 的 400/404/503 语义对齐；
//  3. 端点字面量落在 contract 扫描器的视野里（TestClientEndpoints_AreRegisteredInCoreOpenAPI）。

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ruoxizhnya/tellurion-contracts/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRiskClient_DetectRegime_CoarseShape(t *testing.T) {
	var (
		gotMethod string
		gotPath   string
		gotBody   map[string]any
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotBody)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"trend":"bull","volatility":"medium","sentiment":0.42,"timestamp":"2024-06-01T00:00:00Z"}`))
	}))
	defer srv.Close()

	regime, err := NewRiskClient(srv.URL).DetectRegime(
		t.Context(), "600000.SH", "2024-01-01", "2024-06-01",
	)
	require.NoError(t, err)

	assert.Equal(t, http.MethodPost, gotMethod)
	assert.Equal(t, "/api/risk/detect_regime", gotPath)

	// 粗粒度形状：只有三个键，且没有内联 data。
	assert.Equal(t, map[string]any{
		"symbol":     "600000.SH",
		"start_date": "2024-01-01",
		"end_date":   "2024-06-01",
	}, gotBody)
	_, hasInline := gotBody["data"]
	assert.False(t, hasInline, "内联 data 会挤掉 core 的服务端取数分支")

	require.NotNil(t, regime)
	assert.Equal(t, "bull", regime.Trend)
	assert.Equal(t, "medium", regime.Volatility)
	assert.Equal(t, 0.42, regime.Sentiment)
	assert.Equal(t, time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC), regime.Timestamp)
}

func TestRiskClient_DetectRegime_StatusErrors(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
		want   string
	}{
		{
			name:   "400 参数坏",
			status: http.StatusBadRequest,
			body:   `{"error":"symbol, start_date and end_date are all required"}`,
			want:   "symbol, start_date and end_date are all required",
		},
		{
			name:   "404 无行",
			status: http.StatusNotFound,
			body:   `{"error":"no OHLCV rows for symbol \"999999.SH\" between 2024-01-01 and 2024-06-01"}`,
			want:   "no OHLCV rows",
		},
		{
			name:   "503 取数面未接线",
			status: http.StatusServiceUnavailable,
			body:   `{"error":"OHLCV store unavailable for detect_regime range shape"}`,
			want:   "unavailable",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()

			_, err := NewRiskClient(srv.URL).DetectRegime(t.Context(), "999999.SH", "2024-01-01", "2024-06-01")
			require.Error(t, err)
			assert.Contains(t, err.Error(), "status")
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestRiskClient_DetectRegime_DecodeError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`not-json`))
	}))
	defer srv.Close()

	_, err := NewRiskClient(srv.URL).DetectRegime(t.Context(), "600000.SH", "2024-01-01", "2024-06-01")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decode detect_regime response")
}

func TestRiskClient_DetectRegime_TransportError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := srv.URL
	srv.Close() // 先关掉：连接必失败

	_, err := NewRiskClient(url).DetectRegime(t.Context(), "600000.SH", "2024-01-01", "2024-06-01")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "request failed")
}

func TestRiskClient_BaseURLTrailingSlashIsTrimmed(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	_, err := NewRiskClient(srv.URL+"/").DetectRegime(t.Context(), "600000.SH", "2024-01-01", "2024-06-01")
	require.NoError(t, err)
	assert.Equal(t, "/api/risk/detect_regime", gotPath)
}

// TestRiskClient_ResponseValuesFlowThrough 与 builtin 的同名对拍腿配对：
// 服务端返回什么，客户端就交出什么；不在本层换算/兜底。
func TestRiskClient_ResponseValuesFlowThrough(t *testing.T) {
	want := domain.MarketRegime{
		Trend:      "sideways",
		Volatility: "low",
		Sentiment:  0.3220913107511046,
		Timestamp:  time.Date(2024, 9, 7, 3, 4, 5, 0, time.UTC),
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := json.Marshal(want)
		require.NoError(t, err)
		_, _ = w.Write(raw)
	}))
	defer srv.Close()

	got, err := NewRiskClient(srv.URL).DetectRegime(t.Context(), "000300.SH", "2024-01-01", "2024-09-06")
	require.NoError(t, err)
	assert.Equal(t, want, *got)
}

// TestRiskClient_EndpointIsScannedByContractTest 钉住验收里的「端点字面量进
// TestClientEndpoints_AreRegisteredInCoreOpenAPI」：扫描器必须看见本端点，
// 否则那条护栏对 risk 客户端就是空转。
func TestRiskClient_EndpointIsScannedByContractTest(t *testing.T) {
	endpoints := scanClientEndpoints(t)
	found := false
	for _, endpoint := range endpoints {
		if endpoint == "/api/risk/detect_regime" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("扫描器没看到 /api/risk/detect_regime（现有端点：%s）", strings.Join(endpoints, ", "))
	}
}
