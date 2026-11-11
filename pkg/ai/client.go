package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"tellurion-agent/pkg/ai/budget"
)

// LLMClient is the contract that downstream packages (e.g. pkg/strategy) depend
// on. Extracting an interface lets tests inject a deterministic mock without
// hitting a real LLM endpoint, which is required for CI to pass without
// network access or shared credentials (Sprint 6 P0-1).
//
// Implementations:
//   - *Client  — real OpenAI-compatible HTTP client with timeout/retry/rate/cost
//   - *MockClient (in mock.go) — deterministic fake for unit tests
type LLMClient interface {
	IsConfigured() bool
	Chat(ctx context.Context, messages []ChatMessage) (string, error)
	GenerateStrategyCode(ctx context.Context, description string) (string, error)
	FixStrategyCode(ctx context.Context, code string, buildErrors string) (string, error)
}

// Compile-time assertion: *Client must satisfy LLMClient. If a method signature
// on *Client changes, this line fails to compile and the breakage is caught
// at the boundary rather than at runtime in a consumer.
var _ LLMClient = (*Client)(nil)

// Default HTTP timeout applied to the underlying transport. Without an
// explicit timeout a misbehaving LLM server can hang the caller's goroutine
// indefinitely (Sprint 6 P0-1 / TQ-003).
//
// 2026-11-11：30s → 180s。真栈实测（容器内 ai-service 调 MiniMax
// chat-completions）三次尝试全部 `context deadline exceeded`，而宿主对同一
// URL 的一次 GET 就要 5.4s —— 生成整段策略代码本来就不是 30s 量级的请求。
// 用 AI_HTTP_TIMEOUT 覆盖（Go duration 或裸秒数），重试次数**不动**：
// 超时不够是配置问题，不该靠多试几次掩盖。
const defaultHTTPTimeout = 180 * time.Second

// httpTimeoutEnv 是覆盖出厂超时的环境变量名。
const httpTimeoutEnv = "AI_HTTP_TIMEOUT"

// resolveHTTPTimeout 读 AI_HTTP_TIMEOUT，空值 / 不可解析 / 非正值一律回落到
// 出厂 180s（拼错不该把客户端变成"永不超时"）。
func resolveHTTPTimeout() time.Duration {
	raw := strings.TrimSpace(os.Getenv(httpTimeoutEnv))
	if raw == "" {
		return defaultHTTPTimeout
	}
	if d, err := time.ParseDuration(raw); err == nil && d > 0 {
		return d
	}
	if secs, err := strconv.Atoi(raw); err == nil && secs > 0 {
		return time.Duration(secs) * time.Second
	}
	return defaultHTTPTimeout
}

// Client wraps an HTTP client for LLM chat completions with production-grade
// resilience (timeout, retry, rate-limit, cost tracking, tracing) and
// observability (metrics). All knobs are exposed as functional options.
type Client struct {
	apiKey     string
	apiURL     string
	model      string
	httpClient *http.Client

	// Resilience layer (Sprint 6 P1-14, AR-008).
	limiter   *Limiter    // token-bucket rate limiter
	retry     RetryPolicy // exponential backoff with jitter
	costTable *CostTable  // per-model token cost
	metrics   *Metrics    // per-call observability
	tracer    Tracer      // pluggable tracer (default: NoopTracer)
	// budgetGate 是日累计 USD 闸（nil = 不设闸，仅测试装配会走到）。
	// 读账与记账都必须落库：进程内数字会被重启抹掉（AGENT-LLM-BUDGET-GATE）。
	budgetGate *budget.Gate

	// mu protects limiter/retry/costTable/metrics/tracer swaps.
	// Field assignments during NewClientWithOptions are safe without
	// the lock because no other goroutine can see the client until
	// construction returns. The lock is held only by option setters
	// and the Chat() call site, which is also a single point of truth.
	mu sync.Mutex
}

// DefaultModel is the chat model used when AI_MODEL is unset or blank.
//
// Exported (rather than an inline literal) so callers and tests can assert
// the fallback without duplicating the string — a hardcoded literal in two
// places drifts the moment someone changes the default.
const DefaultModel = "gpt-4o-mini"

// resolveModel picks the chat model from the environment, falling back to
// DefaultModel when AI_MODEL is unset or whitespace-only.
//
// WHY AI_MODEL EXISTS: AI_API_URL already lets you point the client at any
// OpenAI-compatible gateway, but the *model* was hardcoded. A provider that
// speaks /v1/chat/completions still rejects a foreign model id (MiniMax
// answers "model not found" for "gpt-4o-mini"), so URL-only configuration is
// half a feature. Any OpenAI-compatible backend — OpenAI, MiniMax CN,
// DeepSeek, Qwen, OpenRouter, a local Ollama/vLLM shim — needs both knobs.
func resolveModel() string {
	if m := strings.TrimSpace(os.Getenv("AI_MODEL")); m != "" {
		return m
	}
	return DefaultModel
}

// NewClient creates a new AI client reading config from environment.
// A 30s default HTTP timeout is applied to prevent indefinite hangs.
func NewClient() *Client {
	c, _ := NewClientWithOptions()
	return c
}

// NewClientWithBudget 构造接了日累计预算闸的客户端。
//
// 闸的累计值来自 ai.ai_costs，进程重启后从库里重读。WithBudgetGate 是纯赋值
// 选项、不会失败；真失败说明客户端构造本身坏了，启动期直接 panic 比静默
// 放行（无闸）安全。
func NewClientWithBudget(gate *budget.Gate) *Client {
	c, err := NewClientWithOptions(WithBudgetGate(gate))
	if err != nil {
		panic(fmt.Sprintf("ai: construct budgeted client: %v", err))
	}
	return c
}

// NewClientWithOptions builds a Client with functional options. Useful for
// tests that need to inject a custom model, custom HTTP client, or fixed
// API key/URL without touching environment variables.
//
// Precedence for each setting is: explicit option > environment variable >
// built-in default. WithModel therefore beats AI_MODEL, which beats
// DefaultModel.
func NewClientWithOptions(opts ...ClientOption) (*Client, error) {
	c := &Client{
		apiKey:     os.Getenv("AI_API_KEY"),
		apiURL:     os.Getenv("AI_API_URL"),
		model:      resolveModel(),
		httpClient: &http.Client{Timeout: resolveHTTPTimeout()},
		limiter:    NewLimiter(),
		retry:      DefaultRetryPolicy,
		costTable:  NewCostTable(),
		metrics:    NewMetrics(),
		tracer:     NoopTracer{},
	}
	for _, opt := range opts {
		if err := opt(c); err != nil {
			return nil, fmt.Errorf("ai client option: %w", err)
		}
	}
	return c, nil
}

// ClientOption configures a Client at construction time. Follows the
// functional-options pattern recommended by ADR-020 (avoiding Setter soup).
//
// Options may return an error to fail-fast on invalid input (e.g. negative
// rate). They run in order; later options override earlier ones.
type ClientOption func(*Client) error

// WithAPIKey overrides AI_API_KEY for this client instance.
func WithAPIKey(key string) ClientOption {
	return func(c *Client) error { c.apiKey = key; return nil }
}

// WithAPIURL overrides AI_API_URL for this client instance.
func WithAPIURL(url string) ClientOption {
	return func(c *Client) error { c.apiURL = url; return nil }
}

// WithModel overrides the model — it beats both AI_MODEL and DefaultModel.
func WithModel(model string) ClientOption {
	return func(c *Client) error { c.model = model; return nil }
}

// WithHTTPClient replaces the underlying *http.Client (e.g. with a custom
// transport for testing or for an OTel-instrumented round tripper).
func WithHTTPClient(hc *http.Client) ClientOption {
	return func(c *Client) error { c.httpClient = hc; return nil }
}

// WithTimeout overrides the default 30s HTTP timeout. Use 0 to disable.
func WithTimeout(d time.Duration) ClientOption {
	return func(c *Client) error {
		if d <= 0 {
			c.httpClient = &http.Client{}
			return nil
		}
		c.httpClient = &http.Client{Timeout: d}
		return nil
	}
}

// WithLimiter sets a custom rate limiter. Pass nil to disable rate limiting
// (useful for tests that exercise single-call latency in isolation).
func WithLimiter(l *Limiter) ClientOption {
	return func(c *Client) error { c.limiter = l; return nil }
}

// WithRetryPolicy sets a custom retry policy. Pass DefaultRetryPolicy (or
// a copy with adjustments) for production use.
func WithRetryPolicy(p RetryPolicy) ClientOption {
	return func(c *Client) error { c.retry = p; return nil }
}

// WithCostTable sets a custom cost table. Defaults to NewCostTable()
// populated with 2026-06-12 OpenAI/Anthropic/DeepSeek rates.
func WithCostTable(t *CostTable) ClientOption {
	return func(c *Client) error {
		if t == nil {
			return errors.New("WithCostTable: nil table")
		}
		c.costTable = t
		return nil
	}
}

// WithMetrics sets a custom metrics tracker. Pass nil to disable metrics.
func WithMetrics(m *Metrics) ClientOption {
	return func(c *Client) error { c.metrics = m; return nil }
}

// WithBudgetGate 接上日累计 USD 闸。nil 合法（等价于不设闸，仅测试装配用）。
func WithBudgetGate(g *budget.Gate) ClientOption {
	return func(c *Client) error { c.budgetGate = g; return nil }
}

// WithTracer sets a custom tracer. Defaults to NoopTracer. To plug in OTel,
// wrap otel.Tracer into the Tracer interface in your service init code and
// pass it here — this keeps the AI package free of OTel dependencies.
func WithTracer(t Tracer) ClientOption {
	return func(c *Client) error {
		if t == nil {
			c.tracer = NoopTracer{}
			return nil
		}
		c.tracer = t
		return nil
	}
}

// IsConfigured returns true when both AI_API_KEY and AI_API_URL are set.
//
// nil receiver 返回 false：调用方常把 *Client 装进 ai.LLMClient 接口，
// 而「装了类型的 nil 指针」不等于 nil 接口 —— 没有这层防御，
// `p.aiClient != nil && p.aiClient.IsConfigured()` 会在传 nil 时 panic。
func (c *Client) IsConfigured() bool {
	return c != nil && c.apiKey != "" && c.apiURL != ""
}

// Limiter returns the rate limiter (read-only handle). Useful for the
// /metrics endpoint to expose token availability.
func (c *Client) Limiter() *Limiter { return c.limiter }

// CostTable returns the cost table (read-only handle). Useful for the
// /pricing endpoint.
func (c *Client) CostTable() *CostTable { return c.costTable }

// Metrics returns the metrics tracker. Safe to call from any goroutine.
func (c *Client) Metrics() *Metrics { return c.metrics }

// ChatMessage represents a single message in a chat conversation.
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatRequest is the request body for OpenAI-compatible chat completions.
type ChatRequest struct {
	Model    string        `json:"model"`
	Messages []ChatMessage `json:"messages"`
}

// ChatResponse is the top-level response from OpenAI-compatible chat completions.
type ChatResponse struct {
	Choices []Choice `json:"choices"`
	Usage   *Usage   `json:"usage,omitempty"` // OpenAI returns this; other providers may omit
}

// Choice represents one completion choice.
type Choice struct {
	Message ChatMessage `json:"message"`
}

// Chat sends a chat completion request and returns the assistant's reply text.
// The call goes through the full resilience stack: budget gate → rate-limit
// wait → retry-with-backoff → per-call observability (tracer + metrics + cost).
//
// 它现在委托给 ChatWithUsage：此前 Chat 走的是「拿不到 usage」的简化路径，
// 于是 copilot 等 Chat 调用方的花费既不入账也不受预算闸约束。委托后所有
// 生产调用都记 token×单价，闸才有完整的行为面。
func (c *Client) Chat(ctx context.Context, messages []ChatMessage) (string, error) {
	text, _, err := c.ChatWithUsage(ctx, messages)
	return text, err
}

// ChatWithUsage is like Chat but returns the token usage and computes
// the cost. Use this for production paths where cost tracking is
// required; use Chat() for tests / quick paths.
func (c *Client) ChatWithUsage(ctx context.Context, messages []ChatMessage) (string, Usage, error) {
	if !c.IsConfigured() {
		return "", Usage{}, fmt.Errorf("AI_API_KEY or AI_API_URL not configured")
	}
	c.mu.Lock()
	limiter := c.limiter
	retry := c.retry
	costTable := c.costTable
	metrics := c.metrics
	tracer := c.tracer
	gate := c.budgetGate
	c.mu.Unlock()

	// Defensive defaults: a hand-rolled Client (e.g. from tests) may not have
	// these wired. Chat() delegates here, so this path must be as tolerant as
	// the old simple path was.
	if tracer == nil {
		tracer = NoopTracer{}
	}
	if metrics == nil {
		metrics = NewMetrics()
	}
	if costTable == nil {
		costTable = NewCostTable()
	}

	ctx, span := tracer.StartSpan(ctx, SpanName, map[string]any{
		AttrAIModel: c.model,
	})
	defer span.End()

	// 预算闸在限流等待之前：已经超预算就别再排队等一次注定被拒的调用。
	// 读账失败同样拒绝（fail-closed）—— 账都读不出来，就没有依据证明还能花。
	if gate != nil {
		if err := gate.Check(ctx); err != nil {
			span.RecordError(err, 0)
			return "", Usage{}, err
		}
	}

	rateLimited := false
	if limiter != nil {
		if err := limiter.Wait(ctx); err != nil {
			span.RecordError(err, 0)
			return "", Usage{}, fmt.Errorf("rate limit: %w", err)
		}
		rateLimited = true
	}

	start := time.Now()
	attemptCount := 0
	var lastUsage Usage
	type chatResult struct {
		text  string
		usage Usage
	}
	result, status, err := retry.Do(ctx, func(ctx context.Context, attempt int) (string, int, error) {
		attemptCount = attempt
		span.SetAttribute(AttrAIRetryCount, attempt-1)
		text, usage, status, callErr := c.doChatOnceWithUsage(ctx, messages)
		lastUsage = usage
		return text, status, callErr
	})
	duration := time.Since(start)

	cost := 0.0
	if costTable != nil {
		cost = costTable.Calculate(c.model, lastUsage.PromptTokens, lastUsage.CompletionTokens)
	}

	span.SetAttribute(AttrAIStatusCode, status)
	span.SetAttribute(AttrAIPromptTok, lastUsage.PromptTokens)
	span.SetAttribute(AttrAICompletionTok, lastUsage.CompletionTokens)
	span.SetAttribute(AttrAITotalCost, cost)
	span.SetAttribute(AttrAIDurationMS, duration.Milliseconds())
	span.SetAttribute(AttrAIRateLimited, rateLimited)
	if err != nil {
		span.RecordError(err, status)
	}

	if metrics != nil {
		metrics.Record(CallResult{
			Model:       c.model,
			Usage:       lastUsage,
			CostUSD:     cost,
			Duration:    duration,
			StatusCode:  status,
			Err:         err,
			Retried:     attemptCount > 1,
			RateLimited: rateLimited,
		})
	}

	// 花费入账。失败 = 这次调用失败：账没记上，下一道闸就会低估，
	// 等于给「越花越多」开了一个口子（fail-closed）。
	// 记账用脱离取消的 ctx + 短超时：调用方 ctx 被取消时仍然要把已经花掉
	// 的钱记下来，但也不能让一次 DB 卡死拖住调用方。
	if gate != nil && err == nil {
		recordCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		recErr := gate.Record(recordCtx, c.model, lastUsage.PromptTokens, lastUsage.CompletionTokens, cost)
		cancel()
		if recErr != nil {
			span.RecordError(recErr, status)
			return "", lastUsage, recErr
		}
	}

	return result, lastUsage, err
}

// doChatOnce performs a single HTTP request (no retry, no rate limit).
// Used internally by Chat() and ChatWithUsage(); exported via the
// retryable signature.
func (c *Client) doChatOnce(ctx context.Context, messages []ChatMessage) (string, int, error) {
	text, _, status, err := c.doChatOnceFull(ctx, messages)
	return text, status, err
}

// doChatOnceWithUsage is the version that returns usage. Returns
// (text, usage, status, err).
func (c *Client) doChatOnceWithUsage(ctx context.Context, messages []ChatMessage) (string, Usage, int, error) {
	text, usage, status, err := c.doChatOnceFull(ctx, messages)
	return text, usage, status, err
}

// doChatOnceFull does the actual HTTP call. Returns (text, usage, status, err).
func (c *Client) doChatOnceFull(ctx context.Context, messages []ChatMessage) (string, Usage, int, error) {
	reqBody := ChatRequest{Model: c.model, Messages: messages}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", Usage{}, 0, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.apiURL, bytes.NewReader(body))
	if err != nil {
		return "", Usage{}, 0, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", Usage{}, 0, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return "", Usage{}, resp.StatusCode, fmt.Errorf("AI API returned status %d", resp.StatusCode)
	}

	var chatResp ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return "", Usage{}, resp.StatusCode, fmt.Errorf("failed to decode response: %w", err)
	}
	if len(chatResp.Choices) == 0 {
		return "", Usage{}, resp.StatusCode, fmt.Errorf("no response choices from LLM")
	}
	var usage Usage
	if chatResp.Usage != nil {
		usage = *chatResp.Usage
	}
	return chatResp.Choices[0].Message.Content, usage, resp.StatusCode, nil
}

// GenerateStrategyCode calls the LLM with system + user prompts and returns
// the raw Go code, stripping any markdown fences.
func (c *Client) GenerateStrategyCode(ctx context.Context, description string) (string, error) {
	messages := []ChatMessage{
		{Role: "system", Content: SystemPrompt},
		{Role: "user", Content: fmt.Sprintf(UserPromptTemplate, description)},
	}
	resp, err := c.Chat(ctx, messages)
	if err != nil {
		return "", err
	}
	return stripFences(resp), nil
}

// FixStrategyCode asks the LLM to fix compilation errors in the given code.
func (c *Client) FixStrategyCode(ctx context.Context, code string, buildErrors string) (string, error) {
	messages := []ChatMessage{
		{Role: "system", Content: SystemPrompt},
		{Role: "user", Content: fmt.Sprintf(FixPromptTemplate, code, buildErrors)},
	}
	resp, err := c.Chat(ctx, messages)
	if err != nil {
		return "", err
	}
	return stripFences(resp), nil
}

func stripFences(s string) string {
	s = strings.TrimPrefix(s, "```go")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	return strings.TrimSpace(s)
}
