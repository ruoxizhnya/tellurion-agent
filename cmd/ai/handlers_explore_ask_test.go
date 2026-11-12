package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/ruoxizhnya/tellurion-contracts/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"tellurion-agent/pkg/ai/loop"
	"tellurion-agent/pkg/ai/pipeline"
	"tellurion-agent/pkg/ai/validation"
)

// B-06 G3 追问的契约测试：追问挂在**被放弃的尝试**上，答案只从该次尝试
// 自身的证据生成 —— 不编造那一次从未跑过的结果，也不依赖 LLM。

func askFixture() (*ExploreHandler, *gin.Engine) {
	h := NewExploreHandler(nil, nil)
	seedRun(h, "run-ask", []loop.Attempt{
		{
			Seq: 0, Hypothesis: "动量：20 日回看",
			Params: map[string]any{"topk": 10, "lookback": 20},
			OK:     false, Err: errors.New("回测炸了"), NumTrials: 3,
		},
		{
			Seq: 1, Hypothesis: "反转：5 日回看", OK: true,
			ExperimentID: 42,
			Result:       &pipeline.Result{BacktestResult: &domain.BacktestResult{SharpeRatio: 0.8}},
			Verdict: &validation.Verdict{
				Probability: 0.3, Blocking: 1,
				Challenges: []validation.Challenge{{
					Dimension: "稳健", Severity: validation.SeverityBlocking,
					Message: "只有 2 段行情",
				}},
			},
		},
	}, true)
	return h, newExploreRouter(h)
}

func TestExploreInterventions_AskAbandonedAttempt(t *testing.T) {
	_, r := askFixture()

	w := postJSON(r, "/api/explore/runs/run-ask/attempts/0/ask",
		`{"question":"为什么放弃这个方向？"}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var resp AttemptAnswerResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "run-ask", resp.RunID)
	assert.Equal(t, 0, resp.Seq)
	assert.Equal(t, "evidence", resp.Source)
	assert.Contains(t, resp.Answer, "动量：20 日回看")
	assert.Contains(t, resp.Answer, "回测炸了")
	assert.Contains(t, resp.Answer, "lookback=20，topk=10", "参数按键排序，答案可复现")
	assert.Contains(t, resp.Answer, "累计尝试数：3")
	assert.Equal(t, "回测炸了", resp.Evidence.Error)
	assert.False(t, resp.Evidence.OK)
	assert.Nil(t, resp.Evidence.VerdictProbability, "没有裁决就不要造一个 0 分裁决")
}

func TestExploreInterventions_AskCarriesBlockingEvidence(t *testing.T) {
	_, r := askFixture()

	w := postJSON(r, "/api/explore/runs/run-ask/attempts/1/ask",
		`{"question":"验证器为什么不认？"}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var resp AttemptAnswerResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Contains(t, resp.Answer, "Sharpe=0.80")
	assert.Contains(t, resp.Answer, "验证器综合概率 30%")
	assert.Contains(t, resp.Answer, "阻断点：稳健：只有 2 段行情")
	require.NotNil(t, resp.Evidence.VerdictProbability)
	assert.InDelta(t, 0.3, *resp.Evidence.VerdictProbability, 1e-9)
	assert.Equal(t, int64(42), resp.Evidence.ExperimentID)
}

func TestExploreInterventions_AskValidation(t *testing.T) {
	_, r := askFixture()

	assert.Equal(t, http.StatusNotFound,
		postJSON(r, "/api/explore/runs/nope/attempts/0/ask", `{"question":"x"}`).Code)
	assert.Equal(t, http.StatusNotFound,
		postJSON(r, "/api/explore/runs/run-ask/attempts/9/ask", `{"question":"x"}`).Code)
	assert.Equal(t, http.StatusBadRequest,
		postJSON(r, "/api/explore/runs/run-ask/attempts/abc/ask", `{"question":"x"}`).Code)
	assert.Equal(t, http.StatusBadRequest,
		postJSON(r, "/api/explore/runs/run-ask/attempts/0/ask", `{"question":"  "}`).Code)
}
