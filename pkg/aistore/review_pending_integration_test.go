package aistore

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStore_CountPendingReviews 是真库腿：「待审阅」只数 outcome_hit 为空的行。
//
// 计数是全表口径（生产读面确实如此），所以用前后差值断言，避免共享库里
// 其他测试数据影响结果。
func TestStore_CountPendingReviews(t *testing.T) {
	store, closePool := connectTestStore(t)
	defer closePool()
	ctx := context.Background()

	const prefix = "itest_review_pending_%"
	cleanup := func() {
		_, _ = store.pool.Exec(ctx, "DELETE FROM ai.adoption_ledger WHERE proposal_id LIKE $1", prefix)
	}
	defer cleanup()
	cleanup()

	before, err := store.CountPendingReviews(ctx)
	require.NoError(t, err)

	hit := true
	// 两条待回填（其中一条是 holdout —— 它同样等待复盘）。
	_, err = store.InsertAdoptionDecision(ctx, AdoptionDecision{
		ProposalID: "itest_review_pending_1", Actor: "alice",
		Adopted: true, ClaimedConfidence: 0.6,
	})
	require.NoError(t, err)
	_, err = store.InsertAdoptionDecision(ctx, AdoptionDecision{
		ProposalID: "itest_review_pending_2", Actor: "alice",
		Adopted: false, ClaimedConfidence: 0.4, IsHoldout: true,
	})
	require.NoError(t, err)
	// 一条已回填：不应计入待审阅。
	_, err = store.InsertAdoptionDecision(ctx, AdoptionDecision{
		ProposalID: "itest_review_pending_3", Actor: "alice",
		Adopted: true, ClaimedConfidence: 0.7, OutcomeHit: &hit, ResultRef: "exp:9",
	})
	require.NoError(t, err)

	got, err := store.CountPendingReviews(ctx)
	require.NoError(t, err)
	assert.Equal(t, before+2, got, "只数 outcome_hit 为空的行")
}
