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

> **本仓进程内不再有回测引擎 / 表达式 parser 与求值 / 行情 provider**（QT-17 S3-c-3/4）。
> 逐 bar 执行、DSL 解析与求值、取数全部留在 core 进程内；本仓一次请求覆盖整段区间，
> 凭证从入站请求转发。

## 3. 组件职责

| 包 | 职责 | 不做 |
|---|---|---|
| `pkg/ai/loop` | 实验循环：attempt、评估、择优 | 不实现回测 |
| `pkg/ai/gene_pool` | 因子/策略基因池 | 不做因子计算 |
| `pkg/ai/tools` | 工具注册与执行 | 不做鉴权（用 kit） |
| `pkg/ai/causal` | 因果判定 | 不改结果 |
| `pkg/copilot` | 自然语言 → 代码 → **静态沙箱门 + 编译**（⚠️ **终点不是回测**：`copilot.go:119` 明写「deliberately does not run a backtest with the generated code」，`:188` 的 `StrategyName` / `BacktestResult` 结构上恒不可填——未过 promote 人工闸门的产物不进回测，`BlockedPromoteGate`）| ~~不做沙箱（用 core）~~ → **沙箱在本仓进程内**：`CodeChecker`（正则静态门）+ `buildExecutor`（进程隔离 runner），实现取自 kit（`cmd/ai/builders.go` 引 `tellurion-kit/sandbox/*`；S7-P1-2 起本仓 `internal/sandbox` 已删，实测 `internal/` 只剩 `conformance` / `repoguard`）。⚠️ 今天 `config/ai-service.yaml:101` 的 `copilot.working_dir: ""` ⇒ **构建期恒 fail-closed**（`status="sandbox_rejected"`），部署上没人配 working_dir 就没有一次编译能过 |
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
tellurion-agent
      └──(运行期 HTTP)──► core analysis-service 的粗粒度端点
```

> QT-17 S3-d（2026-10-25）起编译期对 core **零依赖**（`go.mod` 无 require / replace）。

## 6. 关键决策

- [`adr/0001-编译期依赖-core-是过渡态.md`](adr/0001-编译期依赖-core-是过渡态.md)（S3-d 已收口：过渡态结束）

## 7. 非目标（明确不做）

- 不做引擎、不做数据抓取、~~不做执行沙箱~~ → **不执行生成的代码**（ADR-024：LLM 产物只编译校验、不加载不运行）。本仓**有**编译期的沙箱门（正则静态检查 + 进程隔离 runner，实现取自 kit），与「不执行生成码」不冲突——判据见 §3 的 `pkg/copilot` 行
- 不做前端（在 ui）
- 不做策略资产版本管理（在 strategy）
- 不做实盘下单（在 core + broker）

## 8. 分期与状态

| 阶段 | 内容 | 状态 |
|---|---|---|
| QT-17 | `ai.*` 自持 + 双写对拍 + 停写 | ✅ **完成（2026-10-25，①~⑤ + ④ 停写复核；本仓台账 `docs/TASKS.md:192`）**：① DDL + 迁移机制已落 `pkg/aistore`（2026-10-23）；② 自有 store / 双写 / 切读已落（2026-10-23）；**④ 停写已落（2026-10-24，ODR-001）**：写路径只剩 `ai.experiments`，旧表冻结；~~③ 仪器域只读 port 等 D24 第 7 类动作~~ → **③ 已落（2026-10-25）**：D24 第 7 类由 core `QT-67` 提供两条只读端点，本仓 `pkg/ai/client/reference_data_client.go` 消费并与 openapi 逐键对拍 |
| S3 | 删 `replace ../tellurion`，改 HTTP | ✅ 2026-10-25：S3-a/b/c-1~4/d 全部落；`go.mod` 无 core require / replace，CI 单仓 checkout，`grep '"tellurion/'` 零命中 |
| — | SSE 已上线（OPS-01-A ✅ 2026-10-15） | ✅ |
