package client

// QT-32-B：把一次编译通过的 AI 生成物登记到 tellurion-strategy 的资产服务，
// 拿回**服务端签发**的坐标（strategy_id / version_hash，产物载体再加
// artifact_hash）。
//
// 三条口径是刻意的：
//   - **坐标只能由资产服务签发**。本仓不自己算 version_hash：它需要服务端生成
//     的 strategy_id 参与（六个输入见
//     tellurion-strategy:internal/asset/service.go:156-163），自己拼一份就是
//     第二个真相。`source_sha256` 留在链上做指纹，不冒充坐标。
//   - **dsl / builtin / external 不调 build**。资产服务的 SubmitBuild 对
//     `!Carrier.NeedsArtifact()` 的载体结构性拒绝（service.go:227-229），
//     调了只会拿到 400；「dsl 没有 artifact_hash」是定义，不是失败。
//   - **失败一律显式**。非 2xx、编译 failed、轮询超时、succeeded 却无
//     artifact_hash，都返回带步骤与 job_id 的错误 —— 调用方（copilot）据此
//     fail-closed，不静默放过一份没有坐标的产物。

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

// 资产服务的载体取值（wire 字面量，与 tellurion-strategy 的 asset.Carrier 一致）。
const (
	AssetCarrierDSL      = "dsl"
	AssetCarrierWASM     = "wasm"
	AssetCarrierSO       = "so"
	AssetCarrierBuiltin  = "builtin"
	AssetCarrierExternal = "external"
)

// 编译作业的终态（wire 字面量，见 asset.JobSucceeded / JobFailed）。
const (
	assetJobSucceeded = "succeeded"
	assetJobFailed    = "failed"
)

// StrategyAssetConfig 是一次登记所需的 wire 参数。
//
// Carrier 决定登记形状：dsl/builtin/external 只登记源码版本；wasm/so 还要
// 触发编译并把产物 hash 取回。ABIVersion 只对产物载体有意义（dsl 必须为空，
// 否则资产服务以 invalid_abi_version 拒绝）。
type StrategyAssetConfig struct {
	BaseURL    string
	Carrier    string
	ABIVersion string
	Scope      string
	Entry      string
	Toolchain  string
	// PollInterval / PollTimeout 只用于产物载体的编译轮询。
	PollInterval time.Duration
	PollTimeout  time.Duration
}

// AssetRef 是资产服务签发的坐标。ArtifactHash 只在产物载体上非空。
type AssetRef struct {
	StrategyID   string
	Carrier      string
	VersionHash  string
	ArtifactHash string
}

// StrategyAssetClient 是资产服务的写侧客户端。
type StrategyAssetClient struct {
	cfg    StrategyAssetConfig
	client *http.Client
}

// NewStrategyAssetClient 构造客户端，并补齐缺省值后**校验载体**。
//
// 缺省：scope=market、entry=OnBar（资产服务对非空只做存在性校验，取值随
// 定义；这两个是内置载体之外最保守的默认）、poll 1s / 2min。
// 未知载体在这里就拒绝：等请求打到服务端再被拒，错误里只剩一个 400。
func NewStrategyAssetClient(cfg StrategyAssetConfig) (*StrategyAssetClient, error) {
	cfg.BaseURL = strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("strategy asset: base url is empty")
	}
	if cfg.Carrier == "" {
		cfg.Carrier = AssetCarrierDSL
	}
	if _, err := carrierNeedsArtifact(cfg.Carrier); err != nil {
		return nil, err
	}
	if cfg.Scope == "" {
		cfg.Scope = "market"
	}
	if cfg.Entry == "" {
		cfg.Entry = "OnBar"
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = time.Second
	}
	if cfg.PollTimeout <= 0 {
		cfg.PollTimeout = 2 * time.Minute
	}
	return &StrategyAssetClient{
		cfg: cfg,
		client: &http.Client{
			// 单次请求上界；编译轮询另有总时长上界（PollTimeout）。
			Timeout: 30 * time.Second,
		},
	}, nil
}

// Carrier 返回实际使用的载体（缺省已补齐）。
func (c *StrategyAssetClient) Carrier() string { return c.cfg.Carrier }

// Register 登记一份生成物并返回资产坐标。
//
// 失败时返回**已经拿到的部分坐标**（例如定义建了、源码传失败了）与错误：
// 调用方把部分坐标落链，比一行空白更能说明这条生成走到了哪一步。
func (c *StrategyAssetClient) Register(ctx context.Context, source string) (AssetRef, error) {
	needsArtifact, err := carrierNeedsArtifact(c.cfg.Carrier)
	if err != nil {
		return AssetRef{}, err
	}

	ref := AssetRef{Carrier: c.cfg.Carrier}

	id, err := c.createDefinition(ctx)
	if err != nil {
		return ref, fmt.Errorf("create definition: %w", err)
	}
	ref.StrategyID = id

	versionHash, err := c.putSource(ctx, id, source)
	if err != nil {
		return ref, fmt.Errorf("put source (strategy_id=%s): %w", id, err)
	}
	ref.VersionHash = versionHash

	if !needsArtifact {
		// dsl/builtin/external：没有产物可编，也没有 artifact_hash 可取。
		return ref, nil
	}

	jobID, err := c.submitBuild(ctx, id, versionHash)
	if err != nil {
		return ref, fmt.Errorf("submit build (strategy_id=%s version_hash=%s): %w", id, versionHash, err)
	}

	artifactHash, err := c.waitBuild(ctx, id, jobID)
	if err != nil {
		return ref, err
	}
	ref.ArtifactHash = artifactHash
	return ref, nil
}

type createDefinitionRequest struct {
	Carrier    string `json:"carrier"`
	ABIVersion string `json:"abi_version,omitempty"`
	Scope      string `json:"scope"`
	Entry      string `json:"entry"`
}

func (c *StrategyAssetClient) createDefinition(ctx context.Context) (string, error) {
	var out struct {
		ID string `json:"id"`
	}
	err := c.call(ctx, http.MethodPost, "/api/strategies", createDefinitionRequest{
		Carrier:    c.cfg.Carrier,
		ABIVersion: c.cfg.ABIVersion,
		Scope:      c.cfg.Scope,
		Entry:      c.cfg.Entry,
	}, &out, http.StatusCreated)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(out.ID) == "" {
		return "", fmt.Errorf("asset service returned an empty strategy id")
	}
	return out.ID, nil
}

func (c *StrategyAssetClient) putSource(ctx context.Context, id, source string) (string, error) {
	var out struct {
		VersionHash string `json:"version_hash"`
	}
	err := c.call(ctx, http.MethodPut, "/api/strategies/"+url.PathEscape(id)+"/source",
		map[string]string{"source": source}, &out, http.StatusOK)
	if err != nil {
		return "", err
	}
	if out.VersionHash == "" {
		return "", fmt.Errorf("asset service returned an empty version_hash")
	}
	return out.VersionHash, nil
}

type submitBuildRequest struct {
	VersionHash string `json:"version_hash"`
	Toolchain   string `json:"toolchain,omitempty"`
}

func (c *StrategyAssetClient) submitBuild(ctx context.Context, id, versionHash string) (string, error) {
	var out struct {
		JobID string `json:"job_id"`
	}
	err := c.call(ctx, http.MethodPost, "/api/strategies/"+url.PathEscape(id)+"/build",
		submitBuildRequest{VersionHash: versionHash, Toolchain: c.cfg.Toolchain},
		&out, http.StatusAccepted)
	if err != nil {
		return "", err
	}
	if out.JobID == "" {
		return "", fmt.Errorf("asset service returned an empty job_id")
	}
	return out.JobID, nil
}

type buildJobView struct {
	JobID        string `json:"job_id"`
	State        string `json:"state"`
	ArtifactHash string `json:"artifact_hash"`
	ErrorCode    string `json:"error_code"`
	Error        string `json:"error"`
}

// waitBuild 轮询编译作业直到终态或超时。
//
// 终态判定按 wire 字面量，不按「见到 hash 就算成功」：succeeded 而不带
// artifact_hash 是服务端违约，必须报错而不是拿空串当坐标。
func (c *StrategyAssetClient) waitBuild(ctx context.Context, id, jobID string) (string, error) {
	deadline := time.Now().Add(c.cfg.PollTimeout)
	path := "/api/strategies/" + url.PathEscape(id) + "/builds/" + url.PathEscape(jobID)
	for {
		var job buildJobView
		if err := c.call(ctx, http.MethodGet, path, nil, &job, http.StatusOK); err != nil {
			return "", fmt.Errorf("poll build job %s: %w", jobID, err)
		}
		switch job.State {
		case assetJobSucceeded:
			if job.ArtifactHash == "" {
				return "", fmt.Errorf("build job %s succeeded without artifact_hash", jobID)
			}
			return job.ArtifactHash, nil
		case assetJobFailed:
			return "", fmt.Errorf("build job %s failed: %s %s",
				jobID, job.ErrorCode, strings.TrimSpace(job.Error))
		}
		if time.Now().After(deadline) {
			return "", fmt.Errorf("build job %s still %q after %s", jobID, job.State, c.cfg.PollTimeout)
		}
		if err := sleepContext(ctx, c.cfg.PollInterval); err != nil {
			return "", err
		}
	}
}

// call 发一次 JSON 请求并核对状态码。非 2xx 的响应体（有界读取）进错误文案：
// 资产服务的 4xx 文案本来就是给人看的（`unsupported_carrier` 这类），
// 吞掉它只会让运维多点几次 curl。
func (c *StrategyAssetClient) call(ctx context.Context, method, path string, in, out any, wantStatus ...int) error {
	var body io.Reader
	if in != nil {
		raw, err := json.Marshal(in)
		if err != nil {
			return fmt.Errorf("encode request body: %w", err)
		}
		body = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.cfg.BaseURL+path, body)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	want := http.StatusOK
	if len(wantStatus) > 0 {
		want = wantStatus[0]
	}
	if resp.StatusCode != want {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("%s %s: status %d (want %d): %s",
			method, path, resp.StatusCode, want, strings.TrimSpace(string(msg)))
	}
	if out == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

// carrierNeedsArtifact 回答「这个载体的坐标里有没有 artifact_hash」。
// 未知载体直接报错 —— 与资产服务的 fail-closed 口径一致（不默认 builtin）。
func carrierNeedsArtifact(carrier string) (bool, error) {
	switch carrier {
	case AssetCarrierWASM, AssetCarrierSO:
		return true, nil
	case AssetCarrierDSL, AssetCarrierBuiltin, AssetCarrierExternal:
		return false, nil
	}
	return false, fmt.Errorf("strategy asset: unknown carrier %q", carrier)
}

func sleepContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
