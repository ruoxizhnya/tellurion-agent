package main

// QT-32-A 的装配根护栏：词表映射与留痕落点的缺失形态。
//
// pkg/copilot 与 pkg/aistore 各持一套编译状态 / 拦截原因的常量，中间由
// chain_sink.go 翻译。两个包各自演进时，这里就是唯一能发现「词表裂开」的地方，
// 所以它必须在**没有库**的时候也真的跑（CI 无 PG）：不连库、只核映射。

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tellurion-agent/pkg/aistore"
	"tellurion-agent/pkg/copilot"
)

// TestChainSink_BuildStateVocabularyIsFullyMapped 逐条钉住两个包的取值一一对应。
// 断言用**绝对取值**而不是「映射后非空」：非空的垃圾值同样能过后者。
func TestChainSink_BuildStateVocabularyIsFullyMapped(t *testing.T) {
	cases := []struct {
		from string
		want string
	}{
		{copilot.BuildStateNone, aistore.GenerationBuildNone},
		{copilot.BuildStateFailed, aistore.GenerationBuildFailed},
		{copilot.BuildStatePassed, aistore.GenerationBuildPassed},
	}
	for _, c := range cases {
		got, err := mapBuildState(c.from)
		require.NoError(t, err, "%q 应被认下", c.from)
		assert.Equal(t, c.want, got, "%q 映射到的取值必须逐字节等于 aistore 侧那一个", c.from)
	}

	// 反过来也要闭合：aistore 的每个取值都得有人从 copilot 侧映射过来，
	// 否则库里能存在一个本仓永远写不出、也没人解释得了的状态。
	reached := map[string]bool{}
	for _, c := range cases {
		got, err := mapBuildState(c.from)
		require.NoError(t, err)
		reached[got] = true
	}
	for _, want := range []string{aistore.GenerationBuildNone, aistore.GenerationBuildFailed, aistore.GenerationBuildPassed} {
		assert.True(t, reached[want], "aistore 的取值 %q 没有任何 copilot 侧的映射能到达", want)
	}
}

// TestChainSink_BlockedReasonVocabularyIsFullyMapped：空串 = 没被拦，也要显式认下。
func TestChainSink_BlockedReasonVocabularyIsFullyMapped(t *testing.T) {
	got, err := mapBlockedReason("")
	require.NoError(t, err)
	assert.Equal(t, "", got, "没拦下时不能编出一个原因")

	got, err = mapBlockedReason(copilot.BlockedPromoteGate)
	require.NoError(t, err)
	assert.Equal(t, aistore.GenerationBlockedPromoteGate, got)
}

// TestChainSink_UnknownVocabularyIsRejectedNotPassedThrough：没登记的取值必须
// 报错，而不是原样传下去让库用 CHECK 兜（那等于把「两处词表裂开」降级成
// 一次运行期约束违例，报错点离原因最远）。
func TestChainSink_UnknownVocabularyIsRejectedNotPassedThrough(t *testing.T) {
	// 空串单独一条：它不是「任何一个已登记状态」，而 Contains(err, "") 恒真，
	// 混在下面那条循环里会是一条假断言。
	_, err := mapBuildState("")
	require.Error(t, err, "空串不是任何一个已登记的编译状态")

	for _, bad := range []string{"rejected", "almost_passed", "PASSED"} {
		_, err := mapBuildState(bad)
		require.Error(t, err, "%q 不该被认下", bad)
		assert.Contains(t, err.Error(), bad, "错误要点名是哪个值")
	}

	for _, bad := range []string{"no_chain_sink", "gate", "promote_gate "} {
		_, err := mapBlockedReason(bad)
		require.Error(t, err, "%q 不是已登记的拦截原因", bad)
		assert.Contains(t, err.Error(), bad)
	}
}

// TestChainSink_MapsBeforeTouchingTheStore：映射失败必须发生在打库之前。
// store 是 nil —— 真走到 Upsert 就是 nil pool 崩，测试当场挂；拿到 error 才
// 证明「说不清的链根本不会去碰库」。
func TestChainSink_MapsBeforeTouchingTheStore(t *testing.T) {
	sink := &chainSink{store: nil}

	err := sink.SaveChain(context.Background(), copilot.ChainEntry{
		JobID: "qt32a-x", BuildState: "not_a_state",
	})
	require.Error(t, err, "没登记的编译状态要报错")
	assert.Contains(t, err.Error(), "not_a_state")

	err = sink.SaveChain(context.Background(), copilot.ChainEntry{
		JobID: "qt32a-x", BuildState: copilot.BuildStateNone, BlockedReason: "not_a_reason",
	})
	require.Error(t, err, "没登记的拦截原因要报错")
	assert.Contains(t, err.Error(), "not_a_reason")
}

// TestNewChainSink_NilStoreMeansNoSink：装配根给不出 store 时，返回的是
// **真正的 nil**（不是装着 nil 指针的非空接口）。CopilotService 靠
// s.chain == nil 判「没装留痕落点」并 fail-closed；返回带类型的 nil 会让那个
// 判断永远为假，然后第一次写链就 panic。
func TestNewChainSink_NilStoreMeansNoSink(t *testing.T) {
	assert.Nil(t, NewChainSink(nil), "nil store 必须换回 nil sink，让 copilot 走未注入的 fail-closed 分支")
	assert.NotNil(t, NewChainSink(&aistore.Store{}), "装了 store 就该有落点（这里不连库，只核装配形态）")
}
