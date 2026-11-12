package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// core 的 ai_explore 源契约测试：GET /api/explore/review/pending →
// 200 {"pending": <int>, "as_of": "<RFC3339>"}。形状一旦漂移，顶栏角标
// 会被 core 判成 unavailable。

type fakeReviewPending struct {
	n   int
	err error
}

func (f *fakeReviewPending) CountPendingReviews(context.Context) (int, error) {
	return f.n, f.err
}

func TestExploreHandler_ReviewPending(t *testing.T) {
	h := NewExploreHandler(nil, nil)
	h.pending = &fakeReviewPending{n: 3}
	r := newExploreRouter(h)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/explore/review/pending", nil)
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var resp ReviewPendingResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, 3, resp.Pending)

	asOf, err := time.Parse(time.RFC3339, resp.AsOf)
	require.NoError(t, err, "as_of 必须是 RFC3339")
	assert.Equal(t, time.UTC, asOf.Location(), "as_of 统一 UTC，聚合层不再猜时区")
}

// 真的 0 条要返回数字 0，而不是 null / 缺字段 —— null 在 core 那边意味着
// 「上游不知道」，会被 UI 画成「—」。
func TestExploreHandler_ReviewPending_ZeroIsANumber(t *testing.T) {
	h := NewExploreHandler(nil, nil)
	h.pending = &fakeReviewPending{n: 0}
	r := newExploreRouter(h)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/explore/review/pending", nil)
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var raw map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &raw))
	require.Contains(t, raw, "pending")
	assert.Equal(t, float64(0), raw["pending"], "真 0 必须是数字 0")
}

func TestExploreHandler_ReviewPending_NotConfiguredIs503(t *testing.T) {
	r := newExploreRouter(NewExploreHandler(nil, nil))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/explore/review/pending", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code, w.Body.String())
}

// 取数失败不能退化成 0：那会让「库连不上」看起来像「没有待审」。
func TestExploreHandler_ReviewPending_ErrorIs500(t *testing.T) {
	h := NewExploreHandler(nil, nil)
	h.pending = &fakeReviewPending{err: errors.New("db down")}
	r := newExploreRouter(h)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/explore/review/pending", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
}
