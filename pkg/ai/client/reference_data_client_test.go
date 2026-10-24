package client

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// QT-17 ③ / QT-67：两条只读动作的 wire 形状与 fail-closed 凭证闸。

func TestReferenceDataClient_ListingWindows(t *testing.T) {
	var gotMethod, gotPath, gotAuth string
	var gotReq listingWindowsRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath, gotAuth = r.Method, r.URL.Path, r.Header.Get("Authorization")
		require.NoError(t, json.NewDecoder(r.Body).Decode(&gotReq))
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{
			"count": 2,
			"windows": [
				{"symbol": "600000.SH", "list_date": "1999-11-10", "delist_date": null},
				{"symbol": "000001.SZ", "list_date": null, "delist_date": "2020-01-02"}
			]
		}`)
	}))
	defer srv.Close()

	c := NewReferenceDataClient(srv.URL)
	windows, err := c.ListingWindows(context.Background(), "tok", nil)
	require.NoError(t, err)

	assert.Equal(t, http.MethodPost, gotMethod)
	assert.Equal(t, "/api/v1/listing-windows", gotPath)
	assert.Equal(t, "Bearer tok", gotAuth, "必须转发调用方的 JWT")
	assert.Empty(t, gotReq.Symbols, "nil symbols = 全部标的；不能臆造成某个列表")

	require.Len(t, windows, 2)
	assert.Equal(t, "600000.SH", windows[0].Symbol)
	require.NotNil(t, windows[0].ListDate)
	assert.Equal(t, "1999-11-10", *windows[0].ListDate)
	assert.Nil(t, windows[0].DelistDate, "NULL 必须是 nil，不能伪造一个日期")
	assert.Nil(t, windows[1].ListDate)
	require.NotNil(t, windows[1].DelistDate)
	assert.Equal(t, "2020-01-02", *windows[1].DelistDate)
}

func TestReferenceDataClient_ListingWindowsPassesRequestedSymbols(t *testing.T) {
	var gotReq listingWindowsRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&gotReq))
		_, _ = io.WriteString(w, `{"count":0,"windows":[]}`)
	}))
	defer srv.Close()

	_, err := NewReferenceDataClient(srv.URL).ListingWindows(
		context.Background(), "tok", []string{"600000.SH", "000001.SZ"})
	require.NoError(t, err)
	assert.Equal(t, []string{"600000.SH", "000001.SZ"}, gotReq.Symbols)
}

func TestReferenceDataClient_TradingCalendar(t *testing.T) {
	var gotMethod, gotPath, gotStart, gotEnd, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		gotStart, gotEnd = r.URL.Query().Get("start"), r.URL.Query().Get("end")
		gotAuth = r.Header.Get("Authorization")
		_, _ = io.WriteString(w, `{
			"start_date": "2026-01-01",
			"end_date": "2026-01-03",
			"count": 2,
			"entries": [
				{"trade_date": "2026-01-02", "exchange": "SSE", "is_trading_day": true},
				{"trade_date": "2026-01-03", "exchange": "SSE", "is_trading_day": false}
			]
		}`)
	}))
	defer srv.Close()

	entries, err := NewReferenceDataClient(srv.URL).TradingCalendar(
		context.Background(), "tok", "2026-01-01", "2026-01-03")
	require.NoError(t, err)

	assert.Equal(t, http.MethodGet, gotMethod)
	assert.Equal(t, "/api/v1/trading-calendar", gotPath)
	assert.Equal(t, "2026-01-01", gotStart)
	assert.Equal(t, "2026-01-03", gotEnd)
	assert.Equal(t, "Bearer tok", gotAuth)
	require.Len(t, entries, 2)
	assert.True(t, entries[0].IsTradingDay)
	assert.False(t, entries[1].IsTradingDay, "休市日也要原样返回，不能过滤")
}

// 没有可转发的凭证时不发请求 —— fail-closed，不悄悄降级成匿名调用。
func TestReferenceDataClient_MissingTokenFailsClosed(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	defer srv.Close()

	c := NewReferenceDataClient(srv.URL)
	_, err := c.ListingWindows(context.Background(), "", nil)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrReferenceDataNoCredentials), "错误要可判等，got %v", err)

	_, err = c.TradingCalendar(context.Background(), "", "2026-01-01", "2026-01-03")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrReferenceDataNoCredentials))
	assert.False(t, called, "没有凭证时请求根本不该发出")
}

// 503 是 core fail-closed 的确定性信号（存储未装配），不能被解成空结果。
func TestReferenceDataClient_NonOKStatusIsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = io.WriteString(w, `{"error":"store unavailable"}`)
	}))
	defer srv.Close()

	_, err := NewReferenceDataClient(srv.URL).ListingWindows(context.Background(), "tok", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "503")
}
