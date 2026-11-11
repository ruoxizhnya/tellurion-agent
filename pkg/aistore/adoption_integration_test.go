package aistore

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStore_AdoptionLedgerLifecycle 是真库腿：台账写入 / 回读 / 重复拒收，
// 以及迁移重复执行不改形状（幂等）。
func TestStore_AdoptionLedgerLifecycle(t *testing.T) {
	store, closePool := connectTestStore(t)
	defer closePool()
	ctx := context.Background()

	const prefix = "itest_adopt_%"
	defer func() {
		_, _ = store.pool.Exec(ctx, "DELETE FROM ai.adoption_ledger WHERE proposal_id LIKE $1", prefix)
	}()
	_, _ = store.pool.Exec(ctx, "DELETE FROM ai.adoption_ledger WHERE proposal_id LIKE $1", prefix)

	// 迁移幂等：第二次执行不得报错，也不得改表形状。
	require.NoError(t, Migrate(ctx, store.pool), "重复迁移必须幂等")

	hit := true
	id, err := store.InsertAdoptionDecision(ctx, AdoptionDecision{
		ProposalID:        "itest_adopt_1",
		Actor:             "alice",
		Intervention:      "lookback 20->60",
		Adopted:           true,
		ClaimedConfidence: 0.6,
		OutcomeHit:        &hit,
		ResultRef:         "exp:1",
	})
	require.NoError(t, err)
	require.Positive(t, id)

	_, err = store.InsertAdoptionDecision(ctx, AdoptionDecision{
		ProposalID:        "itest_adopt_2",
		Actor:             "bob",
		Adopted:           false,
		ClaimedConfidence: 0.3,
		IsHoldout:         true,
	})
	require.NoError(t, err)

	// 同一 proposal 不能出现两条互相矛盾的裁决。
	_, err = store.InsertAdoptionDecision(ctx, AdoptionDecision{
		ProposalID:        "itest_adopt_1",
		Actor:             "bob",
		Adopted:           false,
		ClaimedConfidence: 0.1,
	})
	require.ErrorIs(t, err, ErrDuplicateAdoption)

	entries, err := store.ListAdoptionLedger(ctx)
	require.NoError(t, err)

	var got []AdoptionDecision
	for _, e := range entries {
		if e.ProposalID == "itest_adopt_1" || e.ProposalID == "itest_adopt_2" {
			got = append(got, e)
		}
	}
	require.Len(t, got, 2, "只应看到本次写入的两行")

	assert.Equal(t, "alice", got[0].Actor)
	assert.Equal(t, "lookback 20->60", got[0].Intervention)
	assert.True(t, got[0].Adopted)
	require.NotNil(t, got[0].OutcomeHit)
	assert.True(t, *got[0].OutcomeHit)
	assert.NotNil(t, got[0].ResolvedAt, "已回填结果的行必须落 resolved_at")

	assert.True(t, got[1].IsHoldout)
	assert.Nil(t, got[1].OutcomeHit)
	assert.Nil(t, got[1].ResolvedAt, "未回填结果的行不得有 resolved_at")
}

// TestStore_AdoptionLedgerRejectsOutOfRangeConfidence 锁 DDL 的 CHECK：
// 声称概率必须是 0..1，存储层也不放行。
func TestStore_AdoptionLedgerRejectsOutOfRangeConfidence(t *testing.T) {
	store, closePool := connectTestStore(t)
	defer closePool()
	ctx := context.Background()

	const proposal = "itest_adopt_bad_conf"
	defer func() {
		_, _ = store.pool.Exec(ctx, "DELETE FROM ai.adoption_ledger WHERE proposal_id=$1", proposal)
	}()

	_, err := store.InsertAdoptionDecision(ctx, AdoptionDecision{
		ProposalID:        proposal,
		Actor:             "alice",
		Adopted:           true,
		ClaimedConfidence: 1.5,
	})
	require.Error(t, err)
	assert.False(t, errors.Is(err, ErrDuplicateAdoption))
}
