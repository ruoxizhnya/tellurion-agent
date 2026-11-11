package ai

import (
	"testing"
	"time"
)

// AI_HTTP_TIMEOUT 的口径（2026-11-11）：默认 180s，可被 Go duration 或裸秒数
// 覆盖；空 / 不可解析 / 非正值一律回落默认值 —— 拼错不该把客户端变成
// "永不超时"（那是比超时更难查的故障形态）。
func TestResolveHTTPTimeout(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want time.Duration
	}{
		{"unset", "", defaultHTTPTimeout},
		{"duration", "240s", 240 * time.Second},
		{"bare seconds", "90", 90 * time.Second},
		{"zero", "0", defaultHTTPTimeout},
		{"negative", "-5", defaultHTTPTimeout},
		{"garbage", "three minutes", defaultHTTPTimeout},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(httpTimeoutEnv, tc.raw)
			if got := resolveHTTPTimeout(); got != tc.want {
				t.Fatalf("resolveHTTPTimeout(%q) = %v, want %v", tc.raw, got, tc.want)
			}
		})
	}
}

// 出厂值必须给足：copilot 一次生成要吐整段策略代码，30s 在真栈里恒超时。
func TestDefaultHTTPTimeoutCoversCodeGeneration(t *testing.T) {
	if defaultHTTPTimeout < 180*time.Second {
		t.Fatalf("出厂超时退化到 %v，真栈实测不足以完成一次代码生成", defaultHTTPTimeout)
	}
}
