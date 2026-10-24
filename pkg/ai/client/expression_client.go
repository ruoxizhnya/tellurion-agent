package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ExpressionClient 调 core analysis-service 的表达式静态校验与目录
// （QT-69 的两条只读端点）：
//
//	POST /api/expression/validate —— Parse + Expression.Validate
//	GET  /api/expression/catalog  —— 算子声明 + 字段注册表 + 可用性三态
//
// QT-17 S3-c-4：本仓不再 import `tellurion/pkg/expression`。解析、闸门与
// 「字段现在有没有数据」全部由 core 一处实现；本仓只消费判定与目录，
// 不复制规则、不缓存真相。[readonly-post]，已认证即可（viewer）。
type ExpressionClient struct {
	baseURL    string
	httpClient *http.Client
	auth       authSettings
}

// NewExpressionClient creates a client for core's expression endpoints.
//
// 30s 与其它 analysis-service 客户端同档：静态校验是纯计算，超时即失败，
// 不重试。
func NewExpressionClient(baseURL string, opts ...Option) *ExpressionClient {
	c := &ExpressionClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
	for _, opt := range opts {
		opt(&c.auth)
	}
	return c
}

// ExpressionValidation 是 `POST /api/expression/validate` 的 200 形状。
//
// `Valid=false` 是一个**判定**（parse 失败也算 invalid），不是 HTTP 错误；
// 传输层 / 4xx-5xx 才由方法返回 error（fail-closed）。
type ExpressionValidation struct {
	Valid  bool     `json:"valid"`
	Inputs []string `json:"inputs"`
	AST    string   `json:"ast"`
	Error  string   `json:"error,omitempty"`
	// Stage 是失败阶段：parse（语法）/ gate（算子、字段、参数个数）。
	Stage string `json:"stage,omitempty"`
}

// ExpressionOperatorSpec 对应 ADR-028 §4 的声明七项。参数化算子的
// Lookback / Warmup 是 0 哨兵（真实值由实参推导）；State=true 时
// Lookback=0 表示 ∞ —— 见 Catalog.Thresholds 的机器可读说明。
type ExpressionOperatorSpec struct {
	Name      string `json:"name"`
	Signature string `json:"signature"`
	Lookback  int    `json:"lookback"`
	Causal    bool   `json:"causal"`
	State     bool   `json:"state"`
	Warmup    int    `json:"warmup"`
	Init      string `json:"init"`
	NaNPolicy string `json:"nan_policy"`
}

// ExpressionOperator 是目录里的一个算子。Category 取值与 core 一致：
// "ts" / "cs" / "unary" / "binary"（未知类别原样保留，不猜测）。
type ExpressionOperator struct {
	Name         string                 `json:"name"`
	Category     string                 `json:"category"`
	Arity        int                    `json:"arity"`
	ScalarParams []int                  `json:"scalar_params"`
	Spec         ExpressionOperatorSpec `json:"spec"`
}

// ExpressionField 是目录里的一个字段，Status 三态：
//   - available：已登记，且（探测时）源表有数据 / 无物理表依赖；
//   - unavailable：已登记但源表为空（reason=source_table_empty），**或**
//     请求的名字根本未登记（reason=field_not_registered）；
//   - unknown：已登记，但服务端没装可用性探测（不冒充 available）。
type ExpressionField struct {
	Name      string `json:"name"`
	Source    string `json:"source"`
	DependsOn string `json:"depends_on,omitempty"`
	Status    string `json:"status"`
	Reason    string `json:"reason,omitempty"`
}

// ExpressionThresholds 把注册表里 0 哨兵的语义写成机器可读的约定。
type ExpressionThresholds struct {
	LookbackUnit      string `json:"lookback_unit"`
	WarmupUnit        string `json:"warmup_unit"`
	ZeroLookbackMeans string `json:"zero_lookback_means"`
	ZeroWarmupMeans   string `json:"zero_warmup_means"`
}

// ExpressionCatalog 是 `GET /api/expression/catalog` 的 200 形状。
//
// Availability 只有两个值：probed（真探测过）/ unknown（未装探测）。
// 探测失败在 core 侧是 503 fail-closed —— 不会降级成「字段都不可用」。
type ExpressionCatalog struct {
	Availability string               `json:"availability"`
	Operators    []ExpressionOperator `json:"operators"`
	Fields       []ExpressionField    `json:"fields"`
	Thresholds   ExpressionThresholds `json:"thresholds"`
}

// Validate 提交一次静态校验。
//
// 错误语义（core 侧原样透传，不在本仓改写）：
//   - 400：body 非法 / expression 为空
//   - 非 200：一律 error（服务不可用 / 鉴权失败），不降级成 valid=false ——
//     「服务没答」与「表达式不合法」是两件事。
func (c *ExpressionClient) Validate(ctx context.Context, expression string) (*ExpressionValidation, error) {
	body, err := json.Marshal(map[string]string{"expression": expression})
	if err != nil {
		return nil, fmt.Errorf("failed to encode expression validate request: %w", err)
	}

	req, err := c.newAPIRequest(ctx, http.MethodPost, "/api/expression/validate", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("expression validate request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("expression validate returned status %d: %s",
			resp.StatusCode, strings.TrimSpace(string(msg)))
	}

	var out ExpressionValidation
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("failed to decode expression validate response: %w", err)
	}
	if out.Inputs == nil {
		out.Inputs = []string{}
	}
	return &out, nil
}

// Catalog 取算子 / 字段目录。fields 非空时只查点名的字段 —— core 对未登记的
// 名字返回一条结构化的 unavailable 记录，**不会把它从数组里悄悄删掉**。
//
// 错误语义：非 200（含 400 空字段名 / 503 探测失败）一律 error，不返回半份目录。
func (c *ExpressionClient) Catalog(ctx context.Context, fields []string) (*ExpressionCatalog, error) {
	endpoint := c.baseURL + "/api/expression/catalog"
	if len(fields) > 0 {
		endpoint += "?fields=" + url.QueryEscape(strings.Join(fields, ","))
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create expression catalog request: %w", err)
	}
	if err := c.auth.authorize(req, c.baseURL); err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("expression catalog request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("expression catalog returned status %d: %s",
			resp.StatusCode, strings.TrimSpace(string(msg)))
	}

	var out ExpressionCatalog
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("failed to decode expression catalog response: %w", err)
	}
	switch out.Availability {
	case "probed", "unknown":
	default:
		// 空值或未来新增的状态都 fail-closed：把未知当成「已探测」会伪造可用性。
		return nil, fmt.Errorf("expression catalog 的 availability=%q 不可识别（core 契约只有 probed / unknown）", out.Availability)
	}
	return &out, nil
}

func (c *ExpressionClient) newAPIRequest(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if err := c.auth.authorize(req, c.baseURL); err != nil {
		return nil, err
	}
	return req, nil
}
