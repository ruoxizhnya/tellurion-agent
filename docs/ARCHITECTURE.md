---
status: active
last-verified: 2026-10-20
verified-by: 功能边界固化（三份契约：db / api / dataflow）
---

# 架构 — tellurion-agent

## 1. 定位

AI 实验员层：**实验循环 · 基因池 · 验证 · 工具编排 · Copilot**。

切分判据（DDD）：「怎么算」= core（仪器）；「试什么 / 怎么试 / 试完怎么判」= 本仓（实验员）。

## 2. 分层

```
HTTP 面（:8086）
  /api/explore · /api/copilot · /api/pipeline · /api/tools
        ↓
编排层
  loop（假设→尝试→评估） · gene_pool · pipeline · tools · causal
        ↓
执行（HTTP 粗粒度，QT-17 S3-c-3 起）
  core analysis-service :8085
    POST /api/backtest · POST /api/walkforward · POST /api/expression/validate
    GET  /api/expression/catalog · POST /api/v1/listing-windows · POST /api/risk/detect_regime
        ↓
数据
  ai.*（自持，QT-17）· run（结果）· market（数据）
```

> **本仓进程内不再有回测引擎 / 表达式求值 / 行情 provider**（QT-17 S3-c-3）。
> 逐 bar 执行、DSL 求值、取数全部留在 core 进程内；本仓一次请求覆盖整段区间，
> 凭证从入站请求转发。

## 3. 组件职责

| 包 | 职责 | 不做 |
|---|---|---|
| `pkg/ai/loop` | 实验循环：attempt、评估、择优 | 不实现回测 |
| `pkg/ai/gene_pool` | 因子/策略基因池 | 不做因子计算 |
| `pkg/ai/tools` | 工具注册与执行 | 不做鉴权（用 kit） |
| `pkg/ai/causal` | 因果判定 | 不改结果 |
| `pkg/copilot` | 自然语言 → 代码 → 编译 → 回测 | 不做沙箱（用 core） |
| `pkg/aistore` | `ai.*` 的 DDL、迁移与（QT-17 ② 起）实验域读写 | 不碰仪器域表；不持有连接池（池归装配层） |
| `cmd/ai` | HTTP 面与装配 | 不含业务逻辑 |

## 4. 三份契约（功能的边界就在这三份文件里）

| 维度 | 文件 | 回答什么问题 |
|---|---|---|
| 数据形状 | [`db-schema.md`](db-schema.md) | 本仓拥有哪些表？谁能写？ |
| 接口形状 | [`api-schema.md`](api-schema.md) | 对外暴露什么？错误码与幂等怎么做？ |
| 流向与顺序 | [`dataflow.md`](dataflow.md) | 数据怎么流？哪里 fail-closed？ |

## 5. 依赖方向

```
contracts + kit
      ↑
tellurion-agent  ──(过渡期，S3 删)── require + replace ../tellurion
      └──(运行期 HTTP)──► core 的粗粒度 port
```

## 6. 关键决策

- [`adr/0001-编译期依赖-core-是过渡态.md`](adr/0001-编译期依赖-core-是过渡态.md)

## 7. 非目标（明确不做）

- 不做引擎、不做数据抓取、不做执行沙箱
- 不做前端（在 ui）
- 不做策略资产版本管理（在 strategy）
- 不做实盘下单（在 core + broker）

## 8. 分期与状态

| 阶段 | 内容 | 状态 |
|---|---|---|
| QT-17 | `ai.*` 自持 + 双写对拍 + 停写 | 🔶 ① DDL + 迁移机制已落 `pkg/aistore`（2026-10-23）；② 自有 store / 双写 / 切读已落（2026-10-23）；**④ 停写已落（2026-10-24，ODR-001）**：写路径只剩 `ai.experiments`，旧表冻结；③ 仪器域只读 port 等 D24 第 7 类动作 |
| S3 | 删 `replace ../tellurion`，改 HTTP | 进行中：S3-a/b 已落；S3-c-1/2/3 已落（risk / 策略目录 / 回测+walk-forward 均 HTTP 化，进程内引擎装配已删）；S3-c-4（expression）与 S3-d（bootstrap/observability 换 kit、测试腿清零、删 replace）待做 |
| — | SSE 已上线（OPS-01-A ✅ 2026-10-15） | ✅ |
