package client

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestExpressionClient_Validate_RequestShapeAndVerdict verifies that the client
// posts {"expression": ...} and passes core's verdict through unchanged.
func TestExpressionClient_Validate_RequestShapeAndVerdict(t *testing.T) {
	var gotPath, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		raw, _ := io.ReadAll(r.Body)
		gotBody = string(raw)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"valid":false,"inputs":[],"error":"unknown operator CROSS","stage":"gate"}`))
	}))
	defer srv.Close()

	c := NewExpressionClient(srv.URL, WithInsecureLoopback())
	verdict, err := c.Validate(context.Background(), "CROSS(MA(close, 5))")
	require.NoError(t, err, "core's valid=false is a result, not a transport error")

	assert.Equal(t, "/api/expression/validate", gotPath)
	assert.JSONEq(t, `{"expression":"CROSS(MA(close, 5))"}`, gotBody)

	assert.False(t, verdict.Valid)
	assert.Empty(t, verdict.Inputs, "inputs must be an empty slice, not null")
	assert.Equal(t, "unknown operator CROSS", verdict.Error)
	assert.Equal(t, "gate", verdict.Stage)
}

// TestExpressionClient_Validate_ValidVerdict keeps valid inputs/AST intact.
func TestExpressionClient_Validate_ValidVerdict(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"valid":true,"inputs":["close"],"ast":"ts_rank(close, 20)"}`))
	}))
	defer srv.Close()

	c := NewExpressionClient(srv.URL, WithInsecureLoopback())
	verdict, err := c.Validate(context.Background(), "ts_rank(close, 20)")
	require.NoError(t, err)
	assert.True(t, verdict.Valid)
	assert.Equal(t, []string{"close"}, verdict.Inputs)
	assert.Equal(t, "ts_rank(close, 20)", verdict.AST)
}

// TestExpressionClient_Validate_Non200IsError：服务没答不是「公式不合法」。
func TestExpressionClient_Validate_Non200IsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "expression is required", http.StatusBadRequest)
	}))
	defer srv.Close()

	c := NewExpressionClient(srv.URL, WithInsecureLoopback())
	verdict, err := c.Validate(context.Background(), "")
	require.Error(t, err)
	assert.Nil(t, verdict)
	assert.Contains(t, err.Error(), "400")
}

// TestExpressionClient_Catalog_PreservesUnavailableFields is the OBS-08 core
// requirement: a registered-but-empty field must arrive as a structured
// unavailable entry, and a field named via ?fields= but not registered must
// NOT disappear from the array.
func TestExpressionClient_Catalog_PreservesUnavailableFields(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query().Get("fields")
		_, _ = w.Write([]byte(`{
			"availability":"probed",
			"operators":[{"name":"ts_mean","category":"ts","arity":2,"scalar_params":[1],"spec":{"name":"ts_mean","signature":"ts_mean(x,n)","lookback":0,"causal":true,"state":false,"warmup":0,"init":"","nan_policy":"propagate"}}],
			"fields":[
				{"name":"close","source":"market","depends_on":"ohlcv","status":"available"},
				{"name":"pe","source":"fundamentals","depends_on":"stock_fundamentals","status":"unavailable","reason":"source_table_empty"},
				{"name":"nosuchfield","status":"unavailable","reason":"field_not_registered"}
			],
			"thresholds":{"lookback_unit":"bars","warmup_unit":"bars","zero_lookback_means":"parameterized","zero_warmup_means":"parameterized"}
		}`))
	}))
	defer srv.Close()

	c := NewExpressionClient(srv.URL, WithInsecureLoopback())
	cat, err := c.Catalog(context.Background(), []string{"close", "pe", "nosuchfield"})
	require.NoError(t, err)

	assert.Equal(t, "close,pe,nosuchfield", gotQuery)
	require.Len(t, cat.Fields, 3, "unavailable/unregistered fields must not disappear")
	assert.Equal(t, "unavailable", cat.Fields[1].Status)
	assert.Equal(t, "source_table_empty", cat.Fields[1].Reason)
	assert.Equal(t, "field_not_registered", cat.Fields[2].Reason)
	require.Len(t, cat.Operators, 1)
	assert.Equal(t, "ts", cat.Operators[0].Category)
}

// TestExpressionClient_Catalog_UnrecognizedAvailabilityIsError：未知状态不能
// 被当成「已探测」——那会伪造字段可用性。
func TestExpressionClient_Catalog_UnrecognizedAvailabilityIsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"availability":"maybe","operators":[],"fields":[],"thresholds":{}}`))
	}))
	defer srv.Close()

	c := NewExpressionClient(srv.URL, WithInsecureLoopback())
	cat, err := c.Catalog(context.Background(), nil)
	require.Error(t, err)
	assert.Nil(t, cat)
	assert.Contains(t, err.Error(), "availability")
}

// TestExpressionClient_TokenForwarding：入站 JWT 通过 ctx 转发；没有凭证时
// fail-closed，一个请求都不发。
func TestExpressionClient_TokenForwarding(t *testing.T) {
	var gotAuth string
	var requests int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requests, 1)
		gotAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"availability":"unknown","operators":[],"fields":[],"thresholds":{}}`))
	}))
	defer srv.Close()

	c := NewExpressionClient(srv.URL, WithTokenFromContext())
	ctx := WithBearerTokenInContext(context.Background(), "jwt-inbound")
	_, err := c.Catalog(ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, "Bearer jwt-inbound", gotAuth)

	// No token, no loopback allowance: fail-closed before dialing.
	atomic.StoreInt32(&requests, 0)
	strict := NewExpressionClient(srv.URL)
	_, err = strict.Catalog(context.Background(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "没有凭证")
	assert.Zero(t, atomic.LoadInt32(&requests), "fail-closed must not send the request")
}

// TestExpressionClient_ResponseShapes asserts the decode structs cover the
// exact catalog keys core emits (guards against typo drift in JSON tags).
func TestExpressionClient_ResponseShapes(t *testing.T) {
	cat := ExpressionCatalog{
		Availability: "probed",
		Operators: []ExpressionOperator{{
			Name: "ts_mean", Category: "ts", Arity: 2, ScalarParams: []int{1},
			Spec: ExpressionOperatorSpec{Name: "ts_mean", Signature: "ts_mean(x,n)", Causal: true},
		}},
		Fields: []ExpressionField{{Name: "close", Source: "market", Status: "available"}},
	}
	raw, err := json.Marshal(cat)
	require.NoError(t, err)
	var round ExpressionCatalog
	require.NoError(t, json.Unmarshal(raw, &round))
	assert.Equal(t, cat, round)
}
