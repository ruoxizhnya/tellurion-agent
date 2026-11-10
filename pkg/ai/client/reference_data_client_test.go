package client

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// SVC-CORE-DATA-RETIRE 后交易日历由 tellurion-data 提供：
// `GET /api/v1/trading/calendar?start=&end=` → {"trading_days":["YYYY-MM-DD"]}。
// 契约来源：tellurion-data/docs/api-schema.md。

func TestReferenceDataClient_TradingCalendar(t *testing.T) {
	var gotMethod, gotPath, gotStart, gotEnd, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		gotStart, gotEnd = r.URL.Query().Get("start"), r.URL.Query().Get("end")
		gotAuth = r.Header.Get("Authorization")
		_, _ = io.WriteString(w, `{"trading_days":["2026-01-02","2026-01-05"]}`)
	}))
	defer srv.Close()

	days, err := NewReferenceDataClient(srv.URL).TradingCalendar(
		context.Background(), "tok", "2026-01-01", "2026-01-05")
	require.NoError(t, err)

	assert.Equal(t, http.MethodGet, gotMethod)
	assert.Equal(t, "/api/v1/trading/calendar", gotPath)
	assert.Equal(t, "2026-01-01", gotStart)
	assert.Equal(t, "2026-01-05", gotEnd)
	assert.Equal(t, "Bearer tok", gotAuth, "必须转发调用方的 JWT")
	assert.Equal(t, []string{"2026-01-02", "2026-01-05"}, days)
}

// 没有可转发的凭证时不发请求 —— fail-closed，不悄悄降级成匿名调用。
func TestReferenceDataClient_MissingTokenFailsClosed(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	defer srv.Close()

	_, err := NewReferenceDataClient(srv.URL).TradingCalendar(
		context.Background(), "", "2026-01-01", "2026-01-05")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrReferenceDataNoCredentials), "错误要可判等，got %v", err)
	assert.False(t, called, "没有凭证时请求根本不该发出")
}

// 非 200 是确定性信号，不能被解成空结果。
func TestReferenceDataClient_NonOKStatusIsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = io.WriteString(w, `{"error":"store unavailable"}`)
	}))
	defer srv.Close()

	_, err := NewReferenceDataClient(srv.URL).TradingCalendar(
		context.Background(), "tok", "2026-01-01", "2026-01-05")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "503")
}
