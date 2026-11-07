package mining

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"tellurion-agent/pkg/aistore"
)

// countingSink 记录「有多少行真的被要求落库」。
type countingSink struct {
	rows int
}

func (s *countingSink) SaveFactorCandidates(_ context.Context, candidates []aistore.FactorCandidate) error {
	s.rows += len(candidates)
	return nil
}

// TestGenerateAndSave_GenerationFailureWritesNothing 是反证腿：
// 候选生成失败 ⇒ 一行都不能落（行数必须是 0，不是「有行但为空」）。
//
// 第二个父公式是空串，生成到第 2 个候选时必定失败；此时 Generate 会返回
// 「已生成的第 1 个 + error」。GenerateAndSave 的失败即返回必须把这半批
// 丢掉 —— 删掉那一行，本用例会看到 rows>0。
func TestGenerateAndSave_GenerationFailureWritesNothing(t *testing.T) {
	g := NewGenerator(7, []string{"abs(close)", ""})
	sink := &countingSink{}

	candidates, err := GenerateAndSave(context.Background(), g, sink, 2)
	assert.Equal(t, 0, sink.rows, "候选生成失败时不许落下任何一行")
	assert.Nil(t, candidates)
	require.Error(t, err)
}

// TestGenerateAndSave_WritesWholeBatch：成功路径整批落库，并带可复现的 ID。
func TestGenerateAndSave_WritesWholeBatch(t *testing.T) {
	g1 := NewGenerator(42, []string{"ts_mean(close, 20)"})
	g2 := NewGenerator(42, []string{"ts_mean(close, 20)"})

	c1, err := g1.Generate(context.Background(), 3)
	require.NoError(t, err)
	c2, err := g2.Generate(context.Background(), 3)
	require.NoError(t, err)
	require.Len(t, c1, 3)
	assert.Equal(t, c1, c2, "同 seed + 同父公式 + 同 n 必须给出同一批候选")

	sink := &countingSink{}
	got, err := GenerateAndSave(context.Background(), NewGenerator(42, []string{"ts_mean(close, 20)"}), sink, 3)
	require.NoError(t, err)
	require.Len(t, got, 3)
	assert.Equal(t, 3, sink.rows)
	for _, c := range got {
		assert.NotEmpty(t, c.ID)
		assert.NotEmpty(t, c.Formula)
		assert.Equal(t, "mined", c.Category)
		assert.Contains(t, c.Description, "seed=42")
	}
}

// TestGenerate_RejectsEmptyParents：没有父公式时不该返回空切片冒充成功。
func TestGenerate_RejectsEmptyParents(t *testing.T) {
	_, err := NewGenerator(1, nil).Generate(context.Background(), 3)
	require.Error(t, err)
}
