package mining

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/rand"
	"strings"

	"tellurion-agent/pkg/ai/gene_pool"
	"tellurion-agent/pkg/aistore"
)

// 候选生成（DBT-18）—— 归属结论 (a)：候选只在 agent 侧生成，落 ai.factor_genes；
// core 的 mining pipeline 按 tellurion:SVC-CORE-AI-RETIRE 退役。
//
// 本包只做一件事：用基因池已有的变异算子（gene_pool.Mutator）从父公式生成
// 候选。它不编排回测、不评价好坏、不写 core 的表——评价是验证器的事，
// 落库是 aistore 的事。
//
// 确定性口径：seed 显式传入，所有随机性来自它；父公式按输入顺序轮转
// （不额外抽签），所以同 seed + 同父公式 + 同 n ⇒ 同一批公式与同一批 ID。
// 本包不调用任何模型，因此没有 LLM 成本；将来若接入 LLM 提议器，成本必须
// 走 pkg/ai/cost.go 的 CostTable，不能另起一套计费。
type Generator struct {
	mutator    *gene_pool.Mutator
	rng        *rand.Rand
	seed       int64
	parents    []string
	generation int
}

// NewGenerator 构造候选生成器。parents 是父公式（非空、非空白字符串）。
func NewGenerator(seed int64, parents []string) *Generator {
	return &Generator{
		mutator: gene_pool.NewMutator(seed),
		rng:     rand.New(rand.NewSource(seed)),
		seed:    seed,
		parents: append([]string(nil), parents...),
	}
}

// SetGeneration 标记这批候选属于第几代（落 ai.factor_genes.generation）。
func (g *Generator) SetGeneration(gen int) { g.generation = gen }

// Generate 生成 n 个因子候选。
//
// 失败时返回**已经生成的那部分 + error**，这是刻意的：调用方必须能看到
// 「生成到一半断了」，并且整批丢弃（见 GenerateAndSave）。返回 nil 会把
// 失败伪装成「这次本来就没有候选」。
func (g *Generator) Generate(ctx context.Context, n int) ([]aistore.FactorCandidate, error) {
	if n <= 0 {
		return nil, fmt.Errorf("mining: candidate count must be positive, got %d", n)
	}
	if len(g.parents) == 0 {
		return nil, fmt.Errorf("mining: no parent formula to mutate")
	}

	out := make([]aistore.FactorCandidate, 0, n)
	for i := 0; i < n; i++ {
		if err := ctx.Err(); err != nil {
			return out, err
		}
		parent := g.parents[i%len(g.parents)]
		if strings.TrimSpace(parent) == "" {
			return out, fmt.Errorf("mining: parent formula %d is empty", i)
		}

		formula, mutType, err := g.mutator.Mutate(parent)
		if err != nil {
			return out, fmt.Errorf("mining: mutate parent %q: %w", parent, err)
		}
		// 一半概率再叠一次变异：单次变异容易永远停在父公式的邻域，
		// 两次能让这批候选拉开一点距离。失败就保留第一次的结果。
		if g.rng.Intn(2) == 0 {
			if next, nextType, err := g.mutator.Mutate(formula); err == nil {
				formula, mutType = next, nextType
			}
		}

		id := candidateID(g.seed, g.generation, formula)
		out = append(out, aistore.FactorCandidate{
			ID:          id,
			Name:        "mined-" + strings.TrimPrefix(id, "cand-"),
			Category:    "mined",
			Formula:     formula,
			Description: fmt.Sprintf("seed=%d generation=%d parent=%q", g.seed, g.generation, parent),
			Rationale:   fmt.Sprintf("gene-pool mutation %s from %q", mutType, parent),
			Generation:  g.generation,
			Status:      "pending",
		})
	}
	return out, nil
}

// CandidateSink 是候选的落点。*aistore.Store 满足它。
type CandidateSink interface {
	SaveFactorCandidates(ctx context.Context, candidates []aistore.FactorCandidate) error
}

// GenerateAndSave 是「先生成、后落库」的唯一入口。
//
// 那一行 `if err != nil { return nil, err }` 是**反证用例钉住的语义**：
// 候选没有完整生成就绝不调用 sink。删掉它，Generate 返回的「半批 + error」
// 就会被写进 ai.factor_genes，流水线落下半成品行——这正是本单要防的。
func GenerateAndSave(ctx context.Context, g *Generator, sink CandidateSink, n int) ([]aistore.FactorCandidate, error) {
	candidates, err := g.Generate(ctx, n)
	if err != nil {
		return nil, err
	}
	if err := sink.SaveFactorCandidates(ctx, candidates); err != nil {
		return nil, err
	}
	return candidates, nil
}

func candidateID(seed int64, generation int, formula string) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%d|%d|%s", seed, generation, formula)))
	return "cand-" + hex.EncodeToString(sum[:])[:16]
}
