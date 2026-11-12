package main

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"tellurion-agent/pkg/ai/loop"
)

// B-06 G2 指方向的契约测试：形状与拒绝条件必须稳定 —— 前端三处入口
//（候选卡 / 被放弃的尝试 / 结果流）只认这一套。

// seedRun 直接往运行态里放一轮固定历史，避免契约测试依赖异步时序。
func seedRun(h *ExploreHandler, runID string, attempts []loop.Attempt, done bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.runs[runID] = &exploreRun{
		ctrl:     loop.NewController(nil, nil, nil),
		attempts: attempts,
		done:     done,
	}
}

func TestExploreInterventions_SteerAppliesToNextAttemptOnly(t *testing.T) {
	tr := &fakeExploreRunner{block: make(chan struct{})}
	r := newExploreRouter(NewExploreHandler(tr, nil))

	runID := startRun(t, r, `{"description":"做一个动量策略","max_tries":3}`)
	require.Eventually(t, func() bool { return len(tr.seen()) == 1 }, 2*time.Second, 10*time.Millisecond)

	w := postJSON(r, "/api/explore/runs/"+runID+"/steer",
		`{"direction":"只试反转类参数","attempt_seq":0}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, true, resp["accepted"])
	assert.Equal(t, "next_attempt", resp["applied_to"])
	assert.Equal(t, float64(1), resp["count"])
	dirs, _ := resp["directions"].([]any)
	require.Len(t, dirs, 1)
	assert.Equal(t, "只试反转类参数", dirs[0].(map[string]any)["text"])

	close(tr.block)
	st := waitDone(t, r, runID)
	require.Len(t, st.Attempts, 3)
	assert.NotContains(t, st.Attempts[0].Hypothesis, "人指方向",
		"已经跑完的尝试不能被追溯改写")
	for _, a := range st.Attempts[1:] {
		assert.Contains(t, a.Hypothesis, "人指方向: 只试反转类参数",
			"方向必须落进后续尝试的假设，才有库里的行可查")
	}

	// 已结束的 run 再指方向：409，而不是接受一个永远不会生效的承诺。
	w = postJSON(r, "/api/explore/runs/"+runID+"/steer", `{"direction":"再来一条"}`)
	assert.Equal(t, http.StatusConflict, w.Code, w.Body.String())
}

func TestExploreInterventions_SteerValidation(t *testing.T) {
	h := NewExploreHandler(&fakeExploreRunner{}, nil)
	seedRun(h, "run-idle", nil, false)
	r := newExploreRouter(h)

	assert.Equal(t, http.StatusNotFound,
		postJSON(r, "/api/explore/runs/nope/steer", `{"direction":"x"}`).Code)
	assert.Equal(t, http.StatusBadRequest,
		postJSON(r, "/api/explore/runs/run-idle/steer", `{"direction":"   "}`).Code)
	assert.Equal(t, http.StatusBadRequest,
		postJSON(r, "/api/explore/runs/run-idle/steer", `{}`).Code)
}
