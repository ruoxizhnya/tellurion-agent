# 本仓边界与依赖方向 — tellurion-agent

## 定位

**AI 实验员层**：实验循环 / 基因池 / 验证 / 工具编排 / Copilot / `ai.*` schema。

切分判据（DDD）：「怎么算」= core（仪器）；「试什么 / 怎么试 / 试完怎么判」= 本仓（实验员）。

## 依赖方向

```
tellurion-contracts ──┐
tellurion-kit ────────┴──► tellurion-agent ──(运行期 HTTP)──► core (tellurion)
                                   │
                                   └──(过渡期，S3 删)── require + replace ../tellurion
```

- **core 永不反向依赖本仓**：core 的 `internal/repoguard` 有护栏钉住（core 里不许出现 `pkg/ai`）。
- 本仓对 core 的编译期依赖是**过渡态**，S3 之后只剩 HTTP + 契约版本。

## 目录职责

| 路径 | 内容 |
|---|---|
| `pkg/ai/` | AI 编排层（loop / pipeline / gene_pool / tools / agents / causal …） |
| `pkg/copilot/` | `CopilotService`：自然语言 → 代码 → 沙箱编译 → 回测 |
| `cmd/ai/` | ai-service（:8086）：Copilot / pipeline / explore / tools 四族 HTTP 面 |
| `docs/hermes/` | Hermes agent 配置 / 提示词 / 技能 / 系统设计 / 工具契约 |

## 关键约束

1. **回测循环里不许出现网络调用**：逐 bar / 逐笔远程 = 硬否决。
2. **不直连 core 的业务表**：实验域数据写 `ai.*`（QT-17 完成后 core 侧旧表只读保留一个周期）。
3. **现在只有两个合法依赖路径**：`require core`（过渡）+ `require contracts/kit`（S2 后）。第三个 `replace` 一律不接受。
4. **单仓可构建是目标态，不是现状**：删 replace 的时点是 S3，前提是 QT-50 + QT-17。

## 未决 / 待定

- D24 的粗粒度动作清单缺「交易日历 / 上市窗口查询」一类，QT-17 开工前需补第 7 类。
- `ai.*` 与 core 旧表的双写期长度未定（QT-17 ④）。
