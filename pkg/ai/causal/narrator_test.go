package causal

import (
	"strings"
	"testing"

	"tellurion-agent/pkg/ai/validation"
)

// 叙述者解析的直测（AGENT-CAUSAL-PARSE-TESTS）。
//
// 这里不碰真 LLM：parseTheory 是纯函数，四类输入（正常 / 空 / malformed /
// 多段包裹）各用固定夹具钉死。validation 侧用假叙述者只能证明「讲了什么会
// 被怎么验」，证明不了「模型那段脏输出能不能被读对」—— 后者是这一层的职责。
func TestParseTheory(t *testing.T) {
	tests := []struct {
		name       string
		resp       string
		wantErr    bool
		wantMech   string
		wantPreds  int
		wantErrSub string
		check      func(t *testing.T, th *validation.CausalTheory)
	}{
		{
			name:      "正常：裸 JSON",
			resp:      `{"mechanism":"动量：涨得好的短期继续涨","predictions":[{"kind":"win_rate","statement":"胜率过半","min":0.5},{"kind":"holding_days","statement":"持有 5~30 天","min":5,"max":30}]}`,
			wantMech:  "动量：涨得好的短期继续涨",
			wantPreds: 2,
			check: func(t *testing.T, th *validation.CausalTheory) {
				p0 := th.Predictions[0]
				if p0.Kind != validation.PredictionWinRate || p0.Min == nil || *p0.Min != 0.5 {
					t.Fatalf("第一条预测没被原样读出: %+v", p0)
				}
				p1 := th.Predictions[1]
				if p1.Kind != validation.PredictionHoldingDays || p1.Min == nil || *p1.Min != 5 ||
					p1.Max == nil || *p1.Max != 30 {
					t.Fatalf("第二条预测的边界丢了: %+v", p1)
				}
			},
		},
		{
			name:      "正常：可选 top_k 保留",
			resp:      `{"mechanism":"收益应来自很多天","predictions":[{"kind":"return_concentration","statement":"前 3 天不超过四成","max":0.4,"top_k":3}]}`,
			wantMech:  "收益应来自很多天",
			wantPreds: 1,
			check: func(t *testing.T, th *validation.CausalTheory) {
				if got := th.Predictions[0].TopK; got != 3 {
					t.Fatalf("top_k 应读成 3，实际 %d", got)
				}
			},
		},
		{
			name:       "空：空字符串",
			resp:       "",
			wantErr:    true,
			wantErrSub: "JSON 解析失败",
		},
		{
			name:       "空：只有空白",
			resp:       "  \n\t ",
			wantErr:    true,
			wantErrSub: "JSON 解析失败",
		},
		{
			name:       "空：空对象",
			resp:       `{}`,
			wantErr:    true,
			wantErrSub: "没有给出机制和预测",
		},
		{
			name:       "空：JSON null",
			resp:       `null`,
			wantErr:    true,
			wantErrSub: "没有给出机制和预测",
		},
		{
			name:       "malformed：不是 JSON",
			resp:       "模型今天决定写散文，不写 JSON。",
			wantErr:    true,
			wantErrSub: "JSON 解析失败",
		},
		{
			name:       "malformed：截断的 JSON",
			resp:       `{"mechanism":"动量","predictions":[{"kind":"win_rate"`,
			wantErr:    true,
			wantErrSub: "JSON 解析失败",
		},
		{
			name:      "多段：前后夹散文 + json 代码块",
			resp:      "好的，分析如下：\n```json\n{\"mechanism\":\"反转：超跌反弹\",\"predictions\":[{\"kind\":\"win_rate\",\"statement\":\"胜率过半\",\"min\":0.5}]}\n```\n以上。",
			wantMech:  "反转：超跌反弹",
			wantPreds: 1,
		},
		{
			name:      "多段：只有代码块标记没有散文",
			resp:      "```json\n{\"mechanism\":\"质量溢价\",\"predictions\":[{\"kind\":\"max_drawdown\",\"statement\":\"回撤小于 20%\",\"max\":0.2}]}\n```",
			wantMech:  "质量溢价",
			wantPreds: 1,
		},
		{
			name:      "多段：JSON 后跟一段总结",
			resp:      "{\"mechanism\":\"价值回归\",\"predictions\":[{\"kind\":\"trade_count\",\"statement\":\"至少 10 笔\",\"min\":10}]}\n\n总结：以上是可证伪的预测。",
			wantMech:  "价值回归",
			wantPreds: 1,
		},
		{
			name:      "多段：机制为空但预测齐全，仍应接受",
			resp:      `{"mechanism":"","predictions":[{"kind":"win_rate","statement":"胜率过半","min":0.5}]}`,
			wantMech:  "",
			wantPreds: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			th, err := parseTheory(tt.resp)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("应当显式报错，实际返回理论: %+v", th)
				}
				if th != nil {
					t.Fatalf("解析失败必须返回 nil 理论，实际: %+v", th)
				}
				if tt.wantErrSub != "" && !strings.Contains(err.Error(), tt.wantErrSub) {
					t.Fatalf("错误信息应包含 %q，实际: %v", tt.wantErrSub, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("不该报错: %v", err)
			}
			if th == nil {
				t.Fatal("正常输入不该返回空理论")
			}
			if th.Mechanism != tt.wantMech {
				t.Fatalf("机制应为 %q，实际 %q", tt.wantMech, th.Mechanism)
			}
			if len(th.Predictions) != tt.wantPreds {
				t.Fatalf("预测应有 %d 条，实际 %d: %+v", tt.wantPreds, len(th.Predictions), th.Predictions)
			}
			if tt.check != nil {
				tt.check(t, th)
			}
		})
	}
}

// 解析失败的错误里要带原文片段 —— 否则线上只看到「解析失败」，无从判断
// 模型到底吐了什么。
func TestParseTheory_ErrorCarriesOriginalSnippet(t *testing.T) {
	_, err := parseTheory("不是 JSON 的一段话")
	if err == nil {
		t.Fatal("应当报错")
	}
	if !strings.Contains(err.Error(), "不是 JSON 的一段话") {
		t.Fatalf("错误信息应带原文片段，实际: %v", err)
	}
}
