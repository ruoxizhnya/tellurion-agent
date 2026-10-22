package client

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"
)

// 三份夹具报文照着 core 的**线上形状**写（REV-01-A ③-1）：
// POST /api/backtest 回 contracts.BacktestResponse 平铺体 —— status / error /
// invalid_reasons 与指标同层，**没有** `result` 键。
// invalid 带 invalid_reasons 且 error 必空，failed 带 error 且 invalid_reasons 必空（K9）。
const (
	completedFlatBody = `{
		"id":"bt-1","status":"completed","strategy":"momentum",
		"start_date":"2024-01-01","end_date":"2024-06-01",
		"total_return":0.2,"annual_return":0.4,"sharpe_ratio":1.5,"sortino_ratio":2.1,
		"max_drawdown":-0.08,"max_drawdown_date":"2024-03-15","win_rate":0.55,
		"total_trades":120,"win_trades":66,"lose_trades":54,"avg_holding_days":7.5,"calmar_ratio":5,
		"portfolio_values":[{"date":"2024-01-02T00:00:00Z","total_value":1000000,"cash":400000,"positions":600000}],
		"trades":[{"id":"t-1","symbol":"600000.SH","direction":"long","quantity":100,"price":10.5,"timestamp":"2024-01-02T00:00:00Z"}],
		"universe_max_size":300
	}`

	invalidFlatBody = `{
		"id":"bt-2","status":"invalid","strategy":"momentum",
		"start_date":"2024-01-01","end_date":"2024-06-01",
		"sharpe_ratio":0.1,"total_return":0,
		"invalid_reasons":["data_gap_skipped","session_terminated"]
	}`

	failedFlatBody = `{"id":"bt-3","status":"failed","error":"calendar unavailable for 2023-01-01"}`
)

// stubRequest 是假 core 收到的一次请求原文（判 DTO 漂移要看它，不只看解出来的值）。
type stubRequest struct {
	Method string
	Path   string
	Auth   string
	Body   []byte
}

type stubServer struct {
	mu       sync.Mutex
	requests []stubRequest
	body     string
	status   int
}

func (s *stubServer) hits() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.requests)
}

func (s *stubServer) setStatus(status int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status = status
}

func (s *stubServer) lastRequest(t *testing.T) stubRequest {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.requests) == 0 {
		t.Fatal("假 core 一次请求都没收到")
	}
	return s.requests[len(s.requests)-1]
}

func newStub(t *testing.T, body string) (*stubServer, *BacktestClient) {
	t.Helper()
	stub, rawURL := newStubServer(t, body)
	return stub, NewBacktestClient(rawURL, WithInsecureLoopback())
}

func newStubServer(t *testing.T, body string) (*stubServer, string) {
	t.Helper()
	stub := &stubServer{body: body, status: http.StatusOK}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		stub.mu.Lock()
		stub.requests = append(stub.requests, stubRequest{
			Method: r.Method,
			Path:   r.URL.Path,
			Auth:   r.Header.Get("Authorization"),
			Body:   raw,
		})
		body, status := stub.body, stub.status
		stub.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return stub, srv.URL
}

// TestRunBacktest_ThreeStatusesAreDistinguishable 是本包的主判据：
// 同一套代码、同一份调用形状，三个取值必须给出**三种可区分**的判读。
func TestRunBacktest_ThreeStatusesAreDistinguishable(t *testing.T) {
	cases := []struct {
		name          string
		body          string
		wantStatus    string
		wantSucceeded bool
		wantErr       bool
		wantReasons   int
	}{
		{"completed 是唯一成功", completedFlatBody, StatusCompleted, true, false, 0},
		{"invalid 不是成功也不是故障", invalidFlatBody, StatusInvalid, false, false, 2},
		{"failed 是故障", failedFlatBody, StatusFailed, false, true, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, c := newStub(t, tc.body)
			outcome, err := c.RunBacktest(context.Background(), BacktestRequest{Strategy: "momentum"})

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

// TestRunBacktest_InvalidReasonCodesAreStable 钉住 C1 的两个新 Code（③-3）：
// 它们是 invalid 语义的**说明**，不是第四种状态 —— 不可当成功、也不当故障重试。
func TestRunBacktest_InvalidReasonCodesAreStable(t *testing.T) {
	_, c := newStub(t, invalidFlatBody)
	outcome, err := c.RunBacktest(context.Background(), BacktestRequest{Strategy: "momentum"})
	if err != nil {
		t.Fatalf("invalid 不是故障，不该有 error，got %v", err)
	}
	if outcome.Succeeded() {
		t.Fatal("invalid 不能被读成成功")
	}
	if outcome.Status != StatusInvalid {
		t.Errorf("status want invalid got %q", outcome.Status)
	}
	for _, code := range []string{InvalidReasonDataGapSkipped, InvalidReasonSessionTerminated} {
		if !outcome.HasInvalidReason(code) {
			t.Errorf("应当判读到 invalid_reasons 里的 %q，got %v", code, outcome.InvalidReasons())
		}
	}
	// 顺序与内容都原样：客户端不重算、不翻译、不补默认值。
	want := "data_gap_skipped,session_terminated"
	if got := strings.Join(outcome.InvalidReasons(), ","); got != want {
		t.Errorf("Code 列表必须逐条原样：want %q got %q", want, got)
	}
	if outcome.Result == nil || outcome.Result.SharpeRatio != 0.1 {
		t.Errorf("无效 ≠ 没跑：引擎算出的指标要保留，got %+v", outcome.Result)
	}
}

// TestRunBacktest_UnknownInvalidReasonIsPreserved：未登记的新 Code 原样透传，
// 不被猜成已知语义（猜 = 把「没记的说成记了」）。
func TestRunBacktest_UnknownInvalidReasonIsPreserved(t *testing.T) {
	_, c := newStub(t, `{"id":"bt-4","status":"invalid","invalid_reasons":["future_code"]}`)
	outcome, err := c.RunBacktest(context.Background(), BacktestRequest{Strategy: "momentum"})
	if err != nil {
		t.Fatalf("invalid 不该是故障，got %v", err)
	}
	if outcome.Succeeded() {
		t.Fatal("带未知 code 的 invalid 同样不许读成成功")
	}
	if !outcome.HasInvalidReason("future_code") {
		t.Errorf("未知 code 要原样保留，got %v", outcome.InvalidReasons())
	}
	if outcome.HasInvalidReason(InvalidReasonDataGapSkipped) {
		t.Error("未知 code 不许被猜成 data_gap_skipped")
	}
}

// TestRunBacktest_FailedIsSemanticFailure：failed 要能被调用方与传输错误区分开
// （errors.Is(ErrBacktestFailed)），这样「不把 failed 误当可重试」有机械判据。
func TestRunBacktest_FailedIsSemanticFailure(t *testing.T) {
	_, c := newStub(t, failedFlatBody)
	outcome, err := c.RunBacktest(context.Background(), BacktestRequest{Strategy: "momentum"})
	if err == nil {
		t.Fatal("failed 必须带出 error（既有调用点靠它中止）")
	}
	if !errors.Is(err, ErrBacktestFailed) {
		t.Errorf("failed 要包 ErrBacktestFailed，got %v", err)
	}
	if !strings.Contains(err.Error(), "calendar unavailable") {
		t.Errorf("error 要带上 core 给的原因原话，got %v", err)
	}
	if outcome == nil || outcome.Status != StatusFailed || outcome.Error == "" {
		t.Fatalf("failed 也要交回 outcome 与 Error，got %+v", outcome)
	}
	if outcome.Succeeded() {
		t.Error("failed 绝不能被读成成功")
	}
}

// TestRunBacktest_EmptyStatusFailsClosed 钉住「空 status 不当 completed」：
// 历史落盘结果没记状态，把「没记」写成「记了 completed」就是伪造判据（K9 同一条理由）。
func TestRunBacktest_EmptyStatusFailsClosed(t *testing.T) {
	_, c := newStub(t, `{"status":"","sharpe_ratio":2.0}`)
	outcome, err := c.RunBacktest(context.Background(), BacktestRequest{Strategy: "momentum"})
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
	_, c := newStub(t, `{"status":"done","sharpe_ratio":2.0}`)
	if _, err := c.RunBacktest(context.Background(), BacktestRequest{Strategy: "momentum"}); err == nil {
		t.Fatal("done 不是 K9 的取值，必须报错而不是当成 completed")
	}
}

// TestRunBacktest_PendingJobIsNotSuccess：POST 的异步分支回 202 + {"job_id","status":"pending"}，
// 旧代码会 (nil, nil) 返回，调用方要么 deref nil 要么把「还没跑」当「跑完了」。
func TestRunBacktest_PendingJobIsNotSuccess(t *testing.T) {
	_, c := newStub(t, `{"status":"pending","job_id":"job-77"}`)
	outcome, err := c.RunBacktest(context.Background(), BacktestRequest{Strategy: "momentum"})
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

// TestRunBacktest_RequestMatchesCoreContract 钉住请求侧契约（REV-01-A ①）：
// 发出去的 JSON 键集必须就是 contracts.BacktestRequest 的 `strategy` / `stock_pool` /
// `start_date` / `end_date`，**不许**再出现 `strategy_name`（core 只认 strategy，
// 写错就是恒 400）。这条断言看的是线上的原始 body，不是结构体。
func TestRunBacktest_RequestMatchesCoreContract(t *testing.T) {
	stub, c := newStub(t, completedFlatBody)
	_, err := c.RunBacktest(context.Background(), BacktestRequest{
		Strategy:  "momentum",
		StockPool: []string{"600000.SH"},
		StartDate: "2024-01-01",
		EndDate:   "2024-06-01",
	})
	if err != nil {
		t.Fatal(err)
	}

	req := stub.lastRequest(t)
	if req.Method != http.MethodPost || req.Path != "/api/backtest" {
		t.Errorf("请求落点变了：%s %s", req.Method, req.Path)
	}
	var body map[string]json.RawMessage
	if err := json.Unmarshal(req.Body, &body); err != nil {
		t.Fatalf("请求体不是 JSON：%v（body=%s）", err, req.Body)
	}
	want := []string{"strategy", "stock_pool", "start_date", "end_date"}
	if len(body) != len(want) {
		t.Errorf("请求键集应与契约一致：want %v got %v", want, keysOf(body))
	}
	for _, k := range want {
		if _, ok := body[k]; !ok {
			t.Errorf("请求缺契约字段 %q，got %v", k, keysOf(body))
		}
	}
	if _, ok := body["strategy_name"]; ok {
		t.Error("请求里不许出现 strategy_name：core 的 BacktestRequest 没有这个字段")
	}
	var got strategyEcho
	if err := json.Unmarshal(req.Body, &got); err != nil {
		t.Fatal(err)
	}
	if got.Strategy != "momentum" || got.StartDate != "2024-01-01" || got.EndDate != "2024-06-01" {
		t.Errorf("请求值不对：%+v", got)
	}
}

// strategyEcho 只用于从线上 body 里回读请求值（避免拿本地结构体自证本地结构体）。
type strategyEcho struct {
	Strategy  string   `json:"strategy"`
	StockPool []string `json:"stock_pool"`
	StartDate string   `json:"start_date"`
	EndDate   string   `json:"end_date"`
}

func keysOf(m map[string]json.RawMessage) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// TestRunBacktest_FlatResponseDecodesMetrics 钉住响应侧契约（REV-01-A ①）：
// 平铺体里的指标 / 日期 / 票池上界必须真的落进 Result —— 旧实现只认 `result` 嵌套，
// 这条腿就是那次「Result 恒 nil」的回归。
func TestRunBacktest_FlatResponseDecodesMetrics(t *testing.T) {
	_, c := newStub(t, completedFlatBody)
	outcome, err := c.RunBacktest(context.Background(), BacktestRequest{Strategy: "momentum"})
	if err != nil {
		t.Fatal(err)
	}
	res := outcome.Result
	if res == nil {
		t.Fatal("平铺响应的指标必须落进 Result（旧实现这里恒 nil）")
	}
	if res.StartDate.Format("2006-01-02") != "2024-01-01" || res.EndDate.Format("2006-01-02") != "2024-06-01" {
		t.Errorf("起止日期要按 core 的 2006-01-02 解析，got %v / %v", res.StartDate, res.EndDate)
	}
	if res.MaxDrawdownDate.Format("2006-01-02") != "2024-03-15" {
		t.Errorf("max_drawdown_date 要解析，got %v", res.MaxDrawdownDate)
	}
	if res.SharpeRatio != 1.5 || res.TotalReturn != 0.2 || res.TotalTrades != 120 {
		t.Errorf("指标错位：%+v", res)
	}
	if res.UniverseMaxSize == nil || *res.UniverseMaxSize != 300 {
		t.Errorf("universe_max_size 要落到指针上，got %v", res.UniverseMaxSize)
	}
	if len(res.PortfolioValues) != 1 || len(res.Trades) != 1 {
		t.Errorf("净值序列 / 成交要透传，got %d / %d", len(res.PortfolioValues), len(res.Trades))
	}
	if outcome.JobID != "bt-1" {
		t.Errorf("响应 id 要交回（可用它查结果），got %q", outcome.JobID)
	}
}

// TestRunBacktest_UnparseableDateFailsClosed：读不懂的日期不许落零值（零值 = 1970 年，伪造数据）。
func TestRunBacktest_UnparseableDateFailsClosed(t *testing.T) {
	_, c := newStub(t, `{"id":"bt-5","status":"completed","start_date":"01/02/2024","sharpe_ratio":1}`)
	if _, err := c.RunBacktest(context.Background(), BacktestRequest{Strategy: "momentum"}); err == nil {
		t.Fatal("日期形状不对必须报错，不许静默落零值")
	}
}

func TestRunBacktest_HTTPErrorHasNoOutcome(t *testing.T) {
	stub, c := newStub(t, `{"error":"invalid request body"}`)
	stub.setStatus(http.StatusBadRequest)

	outcome, err := c.RunBacktest(context.Background(), BacktestRequest{Strategy: "m"})
	if err == nil {
		t.Fatal("4xx 必须报错")
	}
	if outcome != nil {
		t.Errorf("4xx 没有可解的三值响应，outcome 应为 nil，got %+v", outcome)
	}
}

// TestGetBacktestResult_JobResultStatusIsAuthoritative 是 C1 在 GET 通道上的主判据：
// job 行 status 是**生命周期**（同步通道对 invalid 也写 completed），
// 真正的结论在 job.result.status 上。只信 job 行就会把同步 invalid 读成成功。
func TestGetBacktestResult_JobResultStatusIsAuthoritative(t *testing.T) {
	jobBody := `{
		"id":"job-9","status":"completed",
		"result":{"id":"job-9","status":"invalid","invalid_reasons":["data_gap_skipped"],"sharpe_ratio":0.1}
	}`
	_, c := newStub(t, jobBody)
	outcome, err := c.GetBacktestResult(context.Background(), "job-9")
	if err != nil {
		t.Fatalf("invalid 不是故障，got %v", err)
	}
	if outcome.Status != StatusInvalid {
		t.Errorf("结果自带的 status 必须优先于 job 行状态：want invalid got %q", outcome.Status)
	}
	if outcome.Succeeded() {
		t.Fatal("job 行 completed + 结果 invalid 绝不能读成成功")
	}
	if !outcome.HasInvalidReason(InvalidReasonDataGapSkipped) {
		t.Errorf("嵌套结果的 invalid_reasons 要透传，got %v", outcome.InvalidReasons())
	}
	if outcome.JobID != "job-9" {
		t.Errorf("JobID 要回填，got %q", outcome.JobID)
	}
}

// TestGetBacktestResult_JobCompletedWithoutResultStatusFailsClosed：
// job 说 completed、落盘结果却没记 status（旧数据 / 残缺响应）—— 间接信号不许当判据。
func TestGetBacktestResult_JobCompletedWithoutResultStatusFailsClosed(t *testing.T) {
	_, c := newStub(t, `{"id":"job-9","status":"completed","result":{"sharpe_ratio":0.8}}`)
	outcome, err := c.GetBacktestResult(context.Background(), "job-9")
	if err == nil {
		t.Fatal("结果没记 status 时必须 fail-closed，不许当成功")
	}
	if !strings.Contains(err.Error(), "没记 status") {
		t.Errorf("错误要点名缺 status，got %v", err)
	}
	if outcome == nil || outcome.Succeeded() {
		t.Errorf("不许构造出「成功」的 outcome，got %+v", outcome)
	}
}

// TestGetBacktestResult_CompletedWithoutResultFailsClosed：
// job 行说 completed / invalid，但 `result` 是 null 或缺失 —— 缺结果不是「空结果」，
// 更不许拿 job 生命周期当成功判据。三种残缺形状都要红。
func TestGetBacktestResult_CompletedWithoutResultFailsClosed(t *testing.T) {
	for _, body := range []string{
		`{"id":"job-9","status":"completed"}`,
		`{"id":"job-9","status":"completed","result":null}`,
		`{"id":"job-9","status":"invalid"}`,
	} {
		_, c := newStub(t, body)
		outcome, err := c.GetBacktestResult(context.Background(), "job-9")
		if err == nil {
			t.Errorf("body=%s：缺 result 必须 fail-closed，不许当成功", body)
			continue
		}
		if !strings.Contains(err.Error(), "没有 result") {
			t.Errorf("body=%s：错误要点名缺 result，got %v", body, err)
		}
		if outcome != nil && outcome.Succeeded() {
			t.Errorf("body=%s：不许构造出「成功」的 outcome，got %+v", body, outcome)
		}
	}
}

func TestGetBacktestResult_SameThreeValueReading(t *testing.T) {
	jobBody := `{"id":"job-9","status":"completed","result":` + completedFlatBody + `}`
	_, c := newStub(t, jobBody)
	outcome, err := c.GetBacktestResult(context.Background(), "job-9")
	if err != nil {
		t.Fatal(err)
	}
	if !outcome.Succeeded() || outcome.JobID != "job-9" {
		t.Errorf("want 成功且 job_id 回填，got %+v", outcome)
	}
	if outcome.Result == nil || outcome.Result.SharpeRatio != 1.5 {
		t.Errorf("嵌套结果的指标要透传，got %+v", outcome.Result)
	}
}

func TestGetBacktestResult_RunningNamesTheStatus(t *testing.T) {
	_, c := newStub(t, `{"id":"job-9","status":"running"}`)
	outcome, err := c.GetBacktestResult(context.Background(), "job-9")
	if err == nil {
		t.Fatal("running 必须报错")
	}
	if !strings.Contains(err.Error(), "running") {
		t.Errorf("错误要点名 status，got %v", err)
	}
	if outcome == nil || outcome.Status != JobStatusRunning {
		t.Errorf("作业态也要原样交回，got %+v", outcome)
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
	if o.HasInvalidReason(InvalidReasonDataGapSkipped) {
		t.Error("nil outcome 不该判读出任何 code")
	}
}

// TestFixtureBodiesDecodeIntoDTO 是夹具的自我核对：
// 三份报文必须真能被 DTO 解出（假服务端解不动，就是在测一个不存在的形状）。
func TestFixtureBodiesDecodeIntoDTO(t *testing.T) {
	var done BacktestResponse
	if err := json.Unmarshal([]byte(completedFlatBody), &done); err != nil {
		t.Fatal(err)
	}
	if done.Status != StatusCompleted || done.SharpeRatio != 1.5 || done.Result != nil {
		t.Fatalf("completed 平铺报文与 DTO 对不上：%+v", done)
	}

	var bad BacktestResponse
	if err := json.Unmarshal([]byte(invalidFlatBody), &bad); err != nil {
		t.Fatal(err)
	}
	if bad.Status != StatusInvalid || bad.Error != "" {
		t.Fatalf("invalid 报文必须 status=invalid 且 error 空（K9 不变量）：%+v", bad)
	}
	if len(bad.InvalidReasons) != 2 {
		t.Fatalf("invalid_reasons 必须落在 DTO 上，got %v", bad.InvalidReasons)
	}
}
