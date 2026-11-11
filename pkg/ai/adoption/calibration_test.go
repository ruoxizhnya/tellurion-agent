package adoption

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"tellurion-agent/pkg/aistore"
)

func hit(v bool) *bool { return &v }

// TestCompute_CalibrationAndHoldoutIsolation 锁「校准误差 + holdout 隔离」口径。
//
// 3 条校准集（声称 0.6，实际 2/3 命中）+ 1 条 holdout（命中）+ 1 条待回填。
// 若把 holdout 行混进校准集，sample_size 会变成 4、hit_rate 变成 0.75，
// 下面每一条断言都会红 —— 这就是「去掉 holdout 隔离应红」的判据。
func TestCompute_CalibrationAndHoldoutIsolation(t *testing.T) {
	entries := []aistore.AdoptionDecision{
		{ProposalID: "p1", ClaimedConfidence: 0.6, OutcomeHit: hit(true), ResultRef: "exp:1"},
		{ProposalID: "p2", ClaimedConfidence: 0.6, OutcomeHit: hit(true), ResultRef: "exp:2"},
		{ProposalID: "p3", ClaimedConfidence: 0.6, OutcomeHit: hit(false), ResultRef: "exp:3"},
		{ProposalID: "h1", ClaimedConfidence: 0.9, IsHoldout: true, OutcomeHit: hit(true), ResultRef: "exp:4"},
		{ProposalID: "pending1", ClaimedConfidence: 0.5},
	}

	r := Compute(entries)

	assert.Equal(t, 3, r.SampleSize, "holdout 与 pending 都不进校准集")
	assert.InDelta(t, 2.0/3.0, r.HitRate, 1e-9)
	assert.InDelta(t, 0.6, r.ClaimedRate, 1e-9)
	assert.InDelta(t, 2.0/3.0-0.6, r.CalibrationError, 1e-9)
	assert.InDelta(t, (0.16+0.16+0.36)/3.0, r.BrierScore, 1e-9)
	assert.Equal(t, 1, r.Pending)

	assert.Equal(t, 1, r.Holdout.SampleSize, "holdout 只单独报")
	assert.InDelta(t, 1.0, r.Holdout.HitRate, 1e-9)
}

func TestCompute_EmptyIsZeroNotNaN(t *testing.T) {
	r := Compute(nil)
	assert.Equal(t, 0, r.SampleSize)
	assert.Equal(t, 0.0, r.HitRate)
	assert.Equal(t, 0.0, r.CalibrationError)
	assert.Equal(t, 0.0, r.BrierScore)
	assert.Equal(t, 0, r.Holdout.SampleSize)
}
