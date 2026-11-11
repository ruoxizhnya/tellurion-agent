package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"tellurion-agent/pkg/aistore"
)

// fakeReaper 记录收尸调用的坐标，用于钉住启动期装配的判据。
type fakeReaper struct {
	called int
	before time.Time
	reason string
	n      int64
	err    error
}

func (f *fakeReaper) ReapRunningExperimentsBefore(_ context.Context, before time.Time, reason string) (int64, error) {
	f.called++
	f.before, f.reason = before, reason
	return f.n, f.err
}

func TestReapExploreLeavings_CallsReaperWithStartupCutoff(t *testing.T) {
	f := &fakeReaper{n: 3}
	cutoff := time.Date(2026, 11, 12, 8, 0, 0, 0, time.UTC)

	got := reapExploreLeavings(context.Background(), f, cutoff, zerolog.Nop())

	assert.Equal(t, int64(3), got)
	assert.Equal(t, 1, f.called, "启动收尸必须真的调用一次")
	assert.Equal(t, cutoff, f.before, "判据是「本进程启动前就已存在」的 running 行")
	assert.Equal(t, aistore.ExperimentOrphanReason, f.reason, "收尸原因必须写死，观察面才可判定")
}

func TestReapExploreLeavings_NilStoreIsNoop(t *testing.T) {
	assert.Zero(t, reapExploreLeavings(context.Background(), nil, time.Now(), zerolog.Nop()))
}

func TestReapExploreLeavings_ErrorIsNotFatal(t *testing.T) {
	f := &fakeReaper{err: errors.New("db down")}
	got := reapExploreLeavings(context.Background(), f, time.Now(), zerolog.Nop())

	assert.Zero(t, got, "收尸失败不能把服务搞挂，也不该谎报回收行数")
	assert.Equal(t, 1, f.called)
}
