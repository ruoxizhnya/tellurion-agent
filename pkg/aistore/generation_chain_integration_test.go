package aistore

// QT-32-A 的真库腿（opt-in）。
//
// 判据是 upsert 的**幂等**与 DDL CHECK 的**牙齿**——这两件事静态解析证明不了：
// 前者只有真库才知道 ON CONFLICT 撞的是哪个键，后者只有真库才会拒一个没登记的
// 编译状态。TEST_DATABASE_DSN 未设置时显式 skip，不装成通过。

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func chainFixtureFingerprint(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// TestGenerationChain_UpsertIsIdempotentPerAttempt 走一遍「同一 job 同一
// attempt 写两次 = 一行，且是最后一次的状态」与「两个 attempt = 两行，按序」。
// 这两条是留痕表的全部语义：链要能被重放写，也要能区分每次尝试。
func TestGenerationChain_UpsertIsIdempotentPerAttempt(t *testing.T) {
	store, closePool := connectTestStore(t)
	defer closePool()
	ctx := context.Background()

	const jobID = "qt32a-itest-chain"
	defer func() {
		_, _ = store.pool.Exec(ctx, "DELETE FROM ai.generation_chain WHERE job_id=$1", jobID)
	}()

	source := "package x\nfunc f() {}\n"
	fp := chainFixtureFingerprint(source)

	first := &GenerationChain{
		JobID: jobID, Attempt: 0, Prompt: "动量", Universe: "csi300",
		StartDate: "2025-01-01", EndDate: "2025-12-31",
		Source: source, SourceSha256: fp, BuildState: GenerationBuildNone,
	}
	require.NoError(t, store.UpsertGenerationChain(ctx, first))

	// 同 (job_id, attempt) 再写一次：推进到 passed + 闸门拦下，必须覆盖不是追加。
	second := *first
	second.BuildState = GenerationBuildPassed
	second.BlockedReason = GenerationBlockedPromoteGate
	second.BuildError = "编译通过；未过 promote 闸门"
	require.NoError(t, store.UpsertGenerationChain(ctx, &second))

	rows, err := store.ListGenerationChain(ctx, jobID)
	require.NoError(t, err)
	require.Len(t, rows, 1, "同一次尝试写两次必须是一行（ON CONFLICT 撞的就是 (job_id, attempt)）")

	got := rows[0]
	assert.Equal(t, GenerationBuildPassed, got.BuildState, "读回应是最后一次推进的状态")
	assert.Equal(t, GenerationBlockedPromoteGate, got.BlockedReason)
	assert.Equal(t, "编译通过；未过 promote 闸门", got.BuildError)
	assert.Equal(t, "动量", got.Prompt)
	assert.Equal(t, "csi300", got.Universe)
	assert.Equal(t, "2025-01-01", got.StartDate)
	assert.Equal(t, "2025-12-31", got.EndDate)

	// CHAR(64) 装 sha256 hex：长度刚好 64 时不会补空格。短一位就会被补白，
	// 读回来的「指纹」就不再等于任何一份源码的 sha256 —— 所以这一腿同时钉住
	// 「写进去的是 64 位」与「读回来逐字节相同」。
	assert.Len(t, fp, 64)
	assert.Equal(t, fp, got.SourceSha256, "指纹读回来必须逐字节相同（CHAR 补白会把它变成对不上任何源码的值）")
	assert.Equal(t, source, got.Source)

	// 另一个 attempt 是另一行：链要留得住「中间试过什么」。
	third := *first
	third.Attempt = 1
	third.Source = source + "\nvar fixed = 1\n"
	third.SourceSha256 = chainFixtureFingerprint(third.Source)
	third.BuildState = GenerationBuildFailed
	require.NoError(t, store.UpsertGenerationChain(ctx, &third))

	rows, err = store.ListGenerationChain(ctx, jobID)
	require.NoError(t, err)
	require.Len(t, rows, 2, "两个 attempt 就是两行")
	assert.Equal(t, []int{0, 1}, []int{rows[0].Attempt, rows[1].Attempt}, "按 attempt 升序读回")
	assert.NotEqual(t, rows[0].SourceSha256, rows[1].SourceSha256,
		"改过源码的那一次必须是另一个指纹 —— 同指纹就是「两行其实是一份」")
}

// TestGenerationChain_RejectsUnregisteredBuildState 是 DDL CHECK 的牙齿：
// 没登记的编译状态写不进去，报错点名是 check 约束违例，而不是静默收下。
func TestGenerationChain_RejectsUnregisteredBuildState(t *testing.T) {
	store, closePool := connectTestStore(t)
	defer closePool()
	ctx := context.Background()

	const jobID = "qt32a-itest-badstate"
	defer func() {
		_, _ = store.pool.Exec(ctx, "DELETE FROM ai.generation_chain WHERE job_id=$1", jobID)
	}()

	err := store.UpsertGenerationChain(ctx, &GenerationChain{
		JobID: jobID, Attempt: 0, Prompt: "x", BuildState: "almost_passed",
	})
	require.Error(t, err, "没登记的编译状态必须被库拒绝")
	assert.True(t, strings.Contains(err.Error(), "check constraint") ||
		strings.Contains(err.Error(), "build_state_check"),
		"错误要点名是哪条约束：got %v", err)
}

// TestGenerationChain_RejectsEmptyJobID：身份缺失在打库之前就得拒 ——
// 没有 job_id 的一行不属于任何一次生成，读的时候永远找不到它。
//
// 这条腿**不连库**（校验在 Exec 之前，nil 池也走得通），所以 CI 里没有 DSN
// 也真的在跑；放在集成文件里只是因为它是同一段存储代码的另一半。
func TestGenerationChain_RejectsEmptyJobID(t *testing.T) {
	store := NewStoreWithPool(nil)

	err := store.UpsertGenerationChain(context.Background(), &GenerationChain{Prompt: "x"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty job id")
}

// TestGenerationChain_ListUnknownJobIsEmpty：「没生成过」不是错误，是空列表。
// 把它编成错误会让调用方拿 err != nil 去兜底一份假链。
func TestGenerationChain_ListUnknownJobIsEmpty(t *testing.T) {
	store, closePool := connectTestStore(t)
	defer closePool()

	rows, err := store.ListGenerationChain(context.Background(), "no-such-job-qt32a")
	require.NoError(t, err)
	assert.Empty(t, rows)
}
