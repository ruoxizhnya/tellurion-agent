package aistore

import (
	"context"
	"fmt"
	"time"
)

// 生成链（QT-32-A）——一次 AI 生成在本仓留下的证据。
//
// 它存在的理由很简单：在此之前，一次 copilot 生成只活在内存里的 job 表和一个
// 用完即删的临时目录里。进程一重启，那次生成就像从未发生过——既不能说「跑了
// 什么 prompt、产出哪份源码」，也不能说「为什么没跑成」。
//
// 一行 = 一次编译尝试（主键 job_id + attempt）。重试让 LLM 交出另一份字节，
// 那是另一个指纹、另一行；只留最后一版就把「中间试过什么」抹掉了。
//
// 三条口径是刻意的，别在改动时把它们抹平：
//   - SourceSha256 **不是** version_hash，也不是 artifact_hash。前者需要资产服务
//     签发的 strategy_id 才编得出来（六项输入见
//     tellurion-contracts:engine/provenance.go:65），后者只覆盖编译产物字节
//     （同文件 :39）。这里存的是**同一行 Source** 的 sha256。
//   - 不存 promote 结论。闸门的真相在资产服务
//     （tellurion-strategy:internal/asset/ddl.go 的 strategy_artifacts.status
//     状态机），本仓再存一份就是两处真相，且一定会漂移。
//   - BlockedReason 为空 = 链没被拦；非空 = 点名是哪一道闸把链停在这里。

// 编译环节的状态。它是**这一环**的结论，不是作业终态（终态由 copilot 的 job
// 状态机负责，两边不是一回事，别互相推断）。
const (
	// GenerationBuildNone：还没走到编译（LLM 出码前、或被静态闸拒了）。
	GenerationBuildNone = "none"
	// GenerationBuildFailed：送进沙箱了，`go build` 没过。
	GenerationBuildFailed = "failed"
	// GenerationBuildPassed：沙箱里编译过了。过了 ≠ 可以拿去跑 —— 那要看闸门。
	GenerationBuildPassed = "passed"
)

// 链被拦下的原因。加一种取值就是加一道闸，要同步 docs/db-schema.md 与
// cmd/ai/chain_sink.go 的映射。
const (
	// GenerationBlockedPromoteGate：AI 产物没有可核验的资产坐标，未过 promote
	// 人工闸门，不进回测（QT-32 ③-②）。
	GenerationBlockedPromoteGate = "promote_gate"
)

// GenerationChain 是 ai.generation_chain 的一行：某次生成第 Attempt 次尝试的
// 当前全量状态。
type GenerationChain struct {
	JobID   string
	Attempt int

	// Prompt 是这次生成的自然语言意图。NOT NULL：作业一开就写得出来。
	Prompt string
	// Universe / StartDate / EndDate 是人要求的回测区间。闸门把它拦下了，
	// 但「当时要求跑什么」仍是链上的一环。
	Universe  string
	StartDate string
	EndDate   string

	// Source 是这一 attempt 的源码；SourceSha256 覆盖的就是它。
	Source       string
	SourceSha256 string

	BuildState    string
	BuildError    string
	BlockedReason string

	CreatedAt time.Time
	UpdatedAt time.Time
}

// UpsertGenerationChain 按 (job_id, attempt) 写入整行当前状态（重复写 = 更新，
// 不插第二条）。
//
// 整行覆盖而不是逐列 COALESCE：调用方（copilot）手里就是这条链的完整当前
// 状态，每段推进带着全量写一次。好处是「读到哪一行就是那次推进结束时的事实」，
// 代价是调用方不能拿半截状态来写 —— 别处想补一列，得先读回整行。
func (s *Store) UpsertGenerationChain(ctx context.Context, g *GenerationChain) error {
	if g.JobID == "" {
		return fmt.Errorf("aistore: upsert generation chain: empty job id")
	}
	buildState := g.BuildState
	if buildState == "" {
		buildState = GenerationBuildNone
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO ai.generation_chain
			(job_id, attempt, prompt, universe, start_date, end_date,
			 source, source_sha256, build_state, build_error,
			 blocked_reason, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, NOW(), NOW())
		ON CONFLICT (job_id, attempt) DO UPDATE SET
			prompt         = EXCLUDED.prompt,
			universe       = EXCLUDED.universe,
			start_date     = EXCLUDED.start_date,
			end_date       = EXCLUDED.end_date,
			source         = EXCLUDED.source,
			source_sha256  = EXCLUDED.source_sha256,
			build_state    = EXCLUDED.build_state,
			build_error    = EXCLUDED.build_error,
			blocked_reason = EXCLUDED.blocked_reason,
			updated_at     = NOW()
	`, g.JobID, g.Attempt, g.Prompt, nullableString(g.Universe),
		nullableString(g.StartDate), nullableString(g.EndDate),
		nullableString(g.Source), nullableString(g.SourceSha256),
		buildState, nullableString(g.BuildError), nullableString(g.BlockedReason),
	)
	if err != nil {
		return fmt.Errorf("aistore: upsert generation chain %s/%d: %w", g.JobID, g.Attempt, err)
	}
	return nil
}

// ListGenerationChain 按尝试顺序读回一次生成的整条链。
//
// 没有单行 Get：链的用途是「这一次生成到底发生了什么」，答案在序列上而不是
// 最后一行。未找到返回空切片 + nil error —— 「没生成过」不是错误，调用方要
// 如实上报，不要拿零值冒充一次生成。
func (s *Store) ListGenerationChain(ctx context.Context, jobID string) ([]GenerationChain, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT job_id, attempt, prompt, universe, start_date, end_date,
		       source, source_sha256, build_state, build_error,
		       blocked_reason, created_at, updated_at
		  FROM ai.generation_chain
		 WHERE job_id = $1
		 ORDER BY attempt
	`, jobID)
	if err != nil {
		return nil, fmt.Errorf("aistore: list generation chain %s: %w", jobID, err)
	}
	defer rows.Close()

	var out []GenerationChain
	for rows.Next() {
		var (
			g                            GenerationChain
			universe, startDate, endDate *string
			source, sha, buildErr, block *string
		)
		if err := rows.Scan(&g.JobID, &g.Attempt, &g.Prompt, &universe, &startDate, &endDate,
			&source, &sha, &g.BuildState, &buildErr, &block, &g.CreatedAt, &g.UpdatedAt); err != nil {
			return nil, fmt.Errorf("aistore: scan generation chain %s: %w", jobID, err)
		}
		g.Universe = derefString(universe)
		g.StartDate = derefString(startDate)
		g.EndDate = derefString(endDate)
		g.Source = derefString(source)
		g.SourceSha256 = derefString(sha)
		g.BuildError = derefString(buildErr)
		g.BlockedReason = derefString(block)
		out = append(out, g)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("aistore: read generation chain %s: %w", jobID, err)
	}
	return out, nil
}

func derefString(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}
