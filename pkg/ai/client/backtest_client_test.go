package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// newStubServer 返回一个「固定报文」的假 core：每次请求都回同一个 JSON。
// 用 t.Cleanup 关闭，不用 time.Sleep（工单 ⑤-1 明令）。
func newStubServer(t *testing.T, body string) *BacktestClient {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return NewBacktestClient(srv.URL)
}

// completedBody / invalidBody / failedBody 三份响应是 K9 冻结口径的三个取值，
// 逐字段照着 core contracts.BacktestResponse 与 domain.BacktestResult 的 json tag 写：
// invalid 带 invalid_reasons 且 error 必空，failed 带 error 且 invalid_reasons 必空。
const (
	completedBody = `{"status":"completed","error":"","result":{"sharpe_ratio":1.5,"total_return":0.2}}`
	invalidBody   = `{"status":"invalid","error":"","result":{"sharpe_ratio":0.1,"total_return":0,"invalid_reasons":["no_trades","warmup_absorbed"]}}`
	failedBody    = `{"status":"failed","error":"calendar unavailable for 2023-01-01"}`
)

// TestRunBacktest_ThreeStatusesAreDistinguishable 是本单的主判据：
// 同一套代码、同一份调用形状，三个取值必须给出**三种可区分**的判读。
// 修好之前 invalid 与 completed 在这里长得一模一样（status 被丢在边界上）。
func TestRunBacktest_ThreeStatusesAreDistinguishable(t *testing.T) {
	cases := []struct {
		name          string
		body          string
		wantStatus    string
		wantSucceeded bool
		wantErr       bool
		wantReasons   int
	}{
		{"completed 是唯一成功", completedBody, StatusCompleted, true, false, 0},
		{"invalid 不是成功也不是故障", invalidBody, StatusInvalid, false, false, 2},
		{"failed 是故障", failedBody, StatusFailed, false, true, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := newStubServer(t, tc.body)
			outcome, err := c.RunBacktest(context.Background(), BacktestRequest{StrategyName: "momentum"})

			if outcome == nil {
				t.Fatalf("拿到响应就必须有 outcome，got nil（err=%v）", err)
			}
			if outcome.Status != tc.wantStatus {
				t.Errorf("status 必须原样透传：want %q got %q", tc.wantStatus, outcome.Status)
			}
			if outcome.Succeeded() != tc.wantSucceeded {
				t.Errorf("Succeeded() want %v got %v（status=%q）", tc.wantSucceeded, outcome.Succeeded(), outcome.Status)
			}
			if tc.wantErr != (err != nil) {
				t.Errorf("err want=%v got=%v", tc.wantErr, err)
			}
			if got := len(outcome.InvalidReasons()); got != tc.wantReasons {
				t.Errorf("invalid_reasons 条数 want=%d got=%d", tc.wantReasons, got)
			}
		})
	}
}

// TestRunBacktest_InvalidReasonsAreReadNotRecomputed 钉住 ③-3：
// 客户端只把引擎挂上来的 Code 原样交出去，不重算、不翻译、不补默认值。
func TestRunBacktest_InvalidReasonsAreReadNotRecomputed(t *testing.T) {
	c := newStubServer(t, invalidBody)
	outcome, err := c.RunBacktest(context.Background(), BacktestRequest{StrategyName: "momentum"})
	if err != nil {
		t.Fatalf("invalid 不是故障，不该有 error，got %v", err)
	}
	want := []string{"no_trades", "warmup_absorbed"}
	got := outcome.InvalidReasons()
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("Code 列表必须逐条原样：want %v got %v", want, got)
	}
	if outcome.Status != StatusInvalid {
		t.Errorf("status want invalid got %q", outcome.Status)
	}
	// 数值仍然是引擎给的数值（无效 ≠ 没跑，指标存在但不可当成功用）。
	if outcome.Result == nil || outcome.Result.SharpeRatio != 0.1 {
		t.Errorf("Result 应保留引擎算出的指标，got %+v", outcome.Result)
	}
}

func TestRunBacktest_FailedCarriesErrorAndIsNotSuccess(t *testing.T) {
	c := newStubServer(t, failedBody)
	outcome, err := c.RunBacktest(context.Background(), BacktestRequest{StrategyName: "momentum"})
	if err == nil {
		t.Fatal("failed 必须带出 error（既有调用点靠它中止）")
	}
	if !strings.Contains(err.Error(), "calendar unavailable") {
		t.Errorf("error 要带上 core 给的原因原话，got %v", err)
	}
	// failed 时 outcome 也非 nil：调用方要能看见 status 才能分清「故障」与「未知」。
	if outcome == nil {
		t.Fatal("failed 也要交回 outcome，三值不能在报错分支上又丢掉")
	}
	if outcome.Status != StatusFailed || outcome.Error == "" {
		t.Errorf("want status=failed + 非空 Error，got status=%q error=%q", outcome.Status, outcome.Error)
	}
	if outcome.Succeeded() {
		t.Error("failed 绝不能被读成成功")
	}
}

// TestRunBacktest_EmptyStatusFailsClosed 钉住「空 status 不当 completed」：
// 历史落盘结果没记状态，把「没记」写成「记了 completed」就是伪造判据（K9 同一条理由）。
func TestRunBacktest_EmptyStatusFailsClosed(t *testing.T) {
	c := newStubServer(t, `{"status":"","result":{"sharpe_ratio":2.0}}`)
	outcome, err := c.RunBacktest(context.Background(), BacktestRequest{StrategyName: "momentum"})
	if err == nil {
		t.Fatal("空 status 必须 fail-closed")
	}
	if !strings.Contains(err.Error(), "未知的 backtest status") {
		t.Errorf("错误要点名是未知 status，got %v", err)
	}
	if outcome.Succeeded() {
		t.Error("空 status 带着漂亮指标也不许读成成功")
	}
}

// TestRunBacktest_FourthValueIsNotInvented 钉住「不新增取值」：
// 三值之外的字符串（哪怕看着像「跑完了」）一律拒绝，不映射、不猜。
func TestRunBacktest_FourthValueIsNotInvented(t *testing.T) {
	c := newStubServer(t, `{"status":"done","result":{"sharpe_ratio":2.0}}`)
	if _, err := c.RunBacktest(context.Background(), BacktestRequest{StrategyName: "momentum"}); err == nil {
		t.Fatal("done 不是 K9 的取值，必须报错而不是当成 completed")
	}
}

// TestRunBacktest_PendingJobIsNotSuccess 是过去最安静的一条：
// POST 的异步分支回 202 + {"job_id","status":"pending"}，旧代码会 (nil, nil) 返回，
// 调用方要么 deref nil 要么把「还没跑」当「跑完了」。
func TestRunBacktest_PendingJobIsNotSuccess(t *testing.T) {
	c := newStubServer(t, `{"status":"pending","job_id":"job-77"}`)
	outcome, err := c.RunBacktest(context.Background(), BacktestRequest{StrategyName: "momentum"})
	if err == nil {
		t.Fatal("pending 不是结果，必须报错")
	}
	if !strings.Contains(err.Error(), "job-77") || !strings.Contains(err.Error(), "pending") {
		t.Errorf("错误要带出 job_id 与真实 status，got %v", err)
	}
	if outcome == nil || outcome.JobID != "job-77" {
		t.Errorf("job_id 要交回给调用方去轮询，got %+v", outcome)
	}
}

// TestRunBacktest_CompletedWithoutResultIsNotSucceeded 钉住判据的形状：
// status 说 completed 但没带出结果，同样是「不可当成功」—— 否则调用点会拿零值指标去打分
// （core 的同步 POST 今天就是平铺响应、没有嵌套 result 键，这条腿就是那道漂移的兜底；
//
//	漂移本身属 REV-01-A 的 DTO 对表，见 docs/TASKS.md）。
func TestRunBacktest_CompletedWithoutResultIsNotSucceeded(t *testing.T) {
	c := newStubServer(t, `{"status":"completed","sharpe_ratio":1.9}`)
	outcome, err := c.RunBacktest(context.Background(), BacktestRequest{StrategyName: "momentum"})
	if err != nil {
		t.Fatalf("completed 无 result 仍是响应不是故障，got err=%v", err)
	}
	if outcome.Status != StatusCompleted {
		t.Errorf("status 仍要原样透传，got %q", outcome.Status)
	}
	if outcome.Result != nil {
		t.Fatalf("夹具里没有嵌套 result，Result 必须为 nil（本测试的前提）")
	}
	if outcome.Succeeded() {
		t.Error("status=completed 但 Result 为 nil 不许读成成功")
	}
}

func TestRunBacktest_HTTPErrorHasNoOutcome(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
	}))
	t.Cleanup(srv.Close)

	outcome, err := NewBacktestClient(srv.URL).RunBacktest(context.Background(), BacktestRequest{StrategyName: "m"})
	if err == nil {
		t.Fatal("4xx 必须报错")
	}
	if outcome != nil {
		t.Errorf("4xx 没有可解的三值响应，outcome 应为 nil，got %+v", outcome)
	}
}

func TestGetBacktestResult_SameThreeValueReading(t *testing.T) {
	// GET /api/backtest/:id 打的是 job 表：标识字段叫 id 不是 job_id，
	// 所以 outcome.JobID 由请求方问的那个 jobID 回填。
	c := newStubServer(t, `{"id":"job-9","status":"completed","result":{"sharpe_ratio":0.8}}`)
	outcome, err := c.GetBacktestResult(context.Background(), "job-9")
	if err != nil {
		t.Fatal(err)
	}
	if !outcome.Succeeded() || outcome.JobID != "job-9" {
		t.Errorf("want 成功且 job_id 回填，got %+v", outcome)
	}
}

func TestGetBacktestResult_RunningNamesTheStatus(t *testing.T) {
	c := newStubServer(t, `{"id":"job-9","status":"running"}`)
	outcome, err := c.GetBacktestResult(context.Background(), "job-9")
	if err == nil {
		t.Fatal("running 必须报错")
	}
	// 旧报文只说 "backtest still running"，不给出收到的 status；
	// 现在必须点名，否则调用方分不清是 running 还是压根不认识的值。
	if !strings.Contains(err.Error(), "running") {
		t.Errorf("错误要点名 status，got %v", err)
	}
	if outcome == nil || outcome.Status != JobStatusRunning {
		t.Errorf("作业态也要原样交回，got %+v", outcome)
	}
}

func TestGetBacktestResult_InvalidIsNotSuccess(t *testing.T) {
	c := newStubServer(t, invalidBody)
	outcome, err := c.GetBacktestResult(context.Background(), "job-9")
	if err != nil {
		t.Fatalf("invalid 不该是故障，got %v", err)
	}
	if outcome.Succeeded() {
		t.Error("job 表回出的 invalid 同样不许读成成功")
	}
	if len(outcome.InvalidReasons()) != 2 {
		t.Errorf("invalid_reasons 要透传，got %v", outcome.InvalidReasons())
	}
}

// TestOutcomeNilSafeOnNilReceiver 钉住两个 helper 的 nil 语义：
// 调用点在 err != nil 时仍可能拿着 nil outcome 问判据，不许 panic。
func TestOutcomeNilSafeOnNilReceiver(t *testing.T) {
	var o *BacktestOutcome
	if o.Succeeded() {
		t.Error("nil outcome 不是成功")
	}
	if got := o.InvalidReasons(); got != nil {
		t.Errorf("nil outcome 的 reasons 应为空，got %v", got)
	}
}

// TestFixtureBodiesDecodeIntoDTO 是夹具的自我核对：
// 三份报文必须真能被现有 DTO 解出（假服务端解不动，就是在测一个不存在的形状）。
func TestFixtureBodiesDecodeIntoDTO(t *testing.T) {
	var done BacktestResponse
	if err := json.Unmarshal([]byte(completedBody), &done); err != nil {
		t.Fatal(err)
	}
	if done.Status != StatusCompleted || done.Result == nil || done.Result.SharpeRatio != 1.5 {
		t.Fatalf("completed 报文与 DTO 对不上：%+v", done)
	}

	var bad BacktestResponse
	if err := json.Unmarshal([]byte(invalidBody), &bad); err != nil {
		t.Fatal(err)
	}
	if bad.Status != StatusInvalid || bad.Error != "" {
		t.Fatalf("invalid 报文必须 status=invalid 且 error 空（K9 不变量）：%+v", bad)
	}
	if len(bad.Result.InvalidReasons) != 2 {
		t.Fatalf("invalid_reasons 必须落在 domain.BacktestResult 上，got %v", bad.Result.InvalidReasons)
	}
}
