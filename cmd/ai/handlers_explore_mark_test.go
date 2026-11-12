package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"tellurion-agent/pkg/ai/loop"
	"tellurion-agent/pkg/aistore"
)

// B-06 G4 标记的契约测试：结果流上的轻量标注 —— 不是裁决，不参与校准，
// 同一坐标允许重复标记。

type fakeMarkStore struct {
	mu    sync.Mutex
	marks []aistore.ExploreMark
	next  int64
}

func (f *fakeMarkStore) InsertExploreMark(_ context.Context, m aistore.ExploreMark) (aistore.ExploreMark, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.next++
	m.ID = f.next
	m.CreatedAt = time.Date(2026, 11, 12, 9, 0, 0, 0, time.UTC)
	f.marks = append(f.marks, m)
	return m, nil
}

func (f *fakeMarkStore) all() []aistore.ExploreMark {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]aistore.ExploreMark, len(f.marks))
	copy(out, f.marks)
	return out
}

func TestExploreInterventions_MarkAttempt(t *testing.T) {
	h := NewExploreHandler(nil, nil)
	store := &fakeMarkStore{}
	h.marks = store
	seedRun(h, "run-mark", []loop.Attempt{
		{Seq: 2, Hypothesis: "动量", ExperimentID: 77},
	}, true)
	r := newExploreRouter(h)

	w := postJSON(r, "/api/explore/runs/run-mark/attempts/2/mark",
		`{"kind":"derive","note":"  值得围绕它多试  ","actor":"ruoxi"}`)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	var got aistore.ExploreMark
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Equal(t, int64(1), got.ID)
	assert.Equal(t, "run-mark", got.RunID)
	assert.Equal(t, 2, got.AttemptSeq)
	assert.Equal(t, aistore.ExploreMarkDerive, got.Kind)
	assert.Equal(t, "值得围绕它多试", got.Note, "note 必须去首尾空白")
	assert.Equal(t, "ruoxi", got.Actor)
	assert.Equal(t, int64(77), got.ExperimentID, "标记要能 join 回那次尝试的证据")
	assert.False(t, got.CreatedAt.IsZero())

	// 同一坐标允许重复标记：标注是流水，不是唯一决策。
	w = postJSON(r, "/api/explore/runs/run-mark/attempts/2/mark",
		`{"kind":"promote","note":"再看一眼"}`)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	require.Len(t, store.all(), 2)

	assert.Equal(t, http.StatusBadRequest,
		postJSON(r, "/api/explore/runs/run-mark/attempts/2/mark", `{"kind":"archive"}`).Code)
	assert.Equal(t, http.StatusNotFound,
		postJSON(r, "/api/explore/runs/run-mark/attempts/9/mark", `{"kind":"derive"}`).Code)
	assert.Equal(t, http.StatusNotFound,
		postJSON(r, "/api/explore/runs/nope/attempts/2/mark", `{"kind":"derive"}`).Code)
}

func TestExploreInterventions_MarkWithoutStoreIs503(t *testing.T) {
	h := NewExploreHandler(nil, nil)
	seedRun(h, "run-nostore", []loop.Attempt{{Seq: 0}}, true)
	r := newExploreRouter(h)

	w := postJSON(r, "/api/explore/runs/run-nostore/attempts/0/mark", `{"kind":"derive"}`)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code, w.Body.String())
}

// 三处入口共用同一坐标：同一 run 上问与标都能落到同一个 seq；这条测试防的是
// 将来有人给某条端点单独换一套路径参数。
func TestExploreInterventions_SharedCoordinates(t *testing.T) {
	h, r := askFixture()
	h.marks = &fakeMarkStore{}

	for _, path := range []string{
		"/api/explore/runs/run-ask/attempts/1/ask",
		"/api/explore/runs/run-ask/attempts/1/mark",
	} {
		if strings.HasSuffix(path, "/ask") {
			assert.Equal(t, http.StatusOK, postJSON(r, path, `{"question":"还成立吗？"}`).Code)
			continue
		}
		assert.Equal(t, http.StatusCreated, postJSON(r, path, `{"kind":"promote"}`).Code)
	}
}
