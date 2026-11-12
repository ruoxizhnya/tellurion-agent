---
service-card: 1
repo: tellurion-agent
kind: application
lifecycle: production
owner: ruoxizhnya
impact: degrading

requires:
  - tellurion-contracts
  - tellurion-kit
required-by: []
calls:
  - tellurion
  - tellurion-data
  - tellurion-strategy
called-by:
  - tellurion

truth:
  readme: README.md
  agents: AGENTS.md
  architecture: docs/ARCHITECTURE.md
  api: docs/api-schema.md
  db: docs/db-schema.md
  dataflow: docs/dataflow.md
  runbook: docs/runbook.md
  tasks: docs/TASKS.md

status: evergreen
last-verified: 2026-11-12
verified-by: 2026-11-12 补 `docs/runbook.md`（起停、`/health` vs `/readyz`、DB 不可达、`ai.*` 迁移缺失、`AI not configured`、`chain sink not configured`、`sandbox_rejected` 分支），命令/端口/env 逐项对照代码、`Dockerfile`、`deploy/compose.yaml`；此前 2026-11-11 实读 README / AGENTS / ARCHITECTURE / api-schema / db-schema / dataflow / config/ai-service.yaml / deploy/compose.yaml / go.mod；端口与端点数量未写入
---

# 服务卡：tellurion-agent

> **一句话**：AI 实验员层——实验循环、工具编排与 Copilot 生成；自己写 `ai.*`，不执行生成码。
>
> `application` · `production` · `degrading` · `ruoxizhnya`

## 1. 这是什么

它是系统的 AI 实验面：接收「试什么 / 怎么试 / 试完怎么判」的请求，在实验循环里生成假设、编排工具、调用 core 的粗粒度 HTTP 接口跑回测，并把每一段推进写进 `ai.*`。它与 core 的分工是 DDD 判据：core 是仪器（怎么算），本仓是实验员（试什么、怎么判）。

**它拥有**：`ai` schema 的 DDL、写权与迁移；Hermes agent 的配置 / 提示词 / 技能 / 工具契约（`docs/hermes/`）。
**它不拥有**：回测引擎、策略运行时、行情与原始数据、策略资产的 promote 结论。

## 2. 它不做什么

- 不 import core 的实现包，也不直连 core 的业务表：跨仓只走 `tellurion-contracts` 与 core 已登记的 HTTP 粗粒度端点。
- 不在回测循环里逐 bar / 逐笔远程调用。
- 不执行 LLM 生成的代码：生成物只做静态检查与编译校验，产物必须登记进资产服务并由人 promote。
- 不替资产服务做 promote，也不把「登记」当「放行」。
- 不做前端页面，不提交 `go.work` / `go.work.sum`。

## 3. 与谁相连

### 调用它（入边）

| 来源 | 协议 | 用来做什么 | 本仓挂了会怎样 |
|---|---|---|---|
| `tellurion` | HTTP | 对 `/api/{copilot,pipeline,explore,tools}` 做纯反代，并加 `x-served-by: agent` | 四族 AI 路由 503；core 的交易与回测主链路不受影响 |

> UI 不直连本仓：它经 core 的同一组前缀进来。四族之外没有第五族。

### 它调用（出边）

| 目标 | 协议 | 用来做什么 | 目标挂了会怎样 |
|---|---|---|---|
| `tellurion` | HTTP | 回测 / walk-forward / 表达式校验与目录 / 行情只读 / regime；凭证从入站请求转发 | 实验循环的评估步失败，run 停在失败态；本仓存活 |
| `tellurion-data` | HTTP | 交易日历等参考数据只读 | 依赖日历的工具报错或降级，不伪造数据 |
| `tellurion-strategy` | HTTP | 生成物登记：建定义 → 传源码 → 可选 build，取回服务端签发的坐标 | 生成链 `sandbox_rejected` 或登记失败；不产出无坐标的产物 |
| PostgreSQL | PG | `ai.*` 的 DDL、读与写 | 就绪探针 503；写不进留痕时生成任务直接失败，不调 LLM |
| LLM 服务（OpenAI 兼容） | HTTP | Copilot 自然语言 → 代码 | 未配置时 503 `AI not configured`；已配置但不可达则生成 job 失败 |

> 外部 LLM 的地址与凭证只从环境变量注入，不写进本卡。
> **因子求值能力当前未提供（ADR-041，已裁决；不是运行时断边）**：`pkg/ai/client/factor_client.go` 的 `factor.compute` / `factor.evaluate` / `compute_factor_ic` **不发起 HTTP 请求**，直接返回 `ErrFactorCapabilityUnavailable`；core 只注册治理面 `POST /api/factor/registrations`（admin），`GET /api/factors/:name` 已退役为 `410 factor_http_surface_retired`，求值 / IC 端点不存在。替代路径：提交回测并读结果（`backtest.run`）。

## 4. 运行时身份

- **常驻进程**：`cmd/ai`（ai-service，唯一 HTTP 监听）；实验循环与 pipeline 在进程内异步执行。
- **存活/就绪语义**：
  - `GET /health`：进程存活，**不碰库**。
  - `GET /readyz`（等价 `/api/readyz`）：DB ping 通过且 `ai.*` 迁移关键表存在；未就绪 503，`/health` 仍 200。
- **部署真相**：`deploy/compose.yaml`、`Dockerfile`、`config/ai-service.yaml`。
- **端口**：不在此卡；见上述部署真相。

## 5. 数据与契约归属

| 维度 | 归属 | 真相源 |
|---|---|---|
| DB schema | `ai` 的唯一写者与 DDL；不写 core / data / strategy 的表 | `docs/db-schema.md` |
| 跨仓 DTO / 契约 | 形状归 `tellurion-contracts`；本仓消费 `domain.FactorHypothesis` / `FactorType` 等 | `tellurion-contracts:docs/API.md` |
| 对外 HTTP | 四族：copilot / pipeline / explore / tools；本仓是它们的唯一完整定义 | `docs/api-schema.md` |

> **归属已裁（ADR-038，2026-11-11）**：`factor_returns` / `ic_analysis` 归 `tellurion`（core）——与因子值同源，零写者、不迁 data/analytics；`factor_hypothesis` 归 `ai.*`（本仓）。

## 6. 坏了怎么办

- **影响面**：`degrading`——AI 四族路由不可用；core 的交易、回测与风控主链路不受影响。写不进留痕时生成任务 fail-closed，不会产出无法复查的代码。
- **先确认（按顺序）**：
  1. `GET /health` 是否 200；不是则看进程日志与启动配置。
  2. `GET /readyz` 是否 200；503 时看 DB 连通与 `ai.*` 迁移是否执行。
  3. 生成 / 工具失败看 job 终态与 `build_error`：是否点名 `ai.generation_chain` / `chain sink not configured` / `AI not configured`。
- **升级路径**：`owner` → 跨仓争议 → core `docs/TASKS.md`
- **runbook**：[`docs/runbook.md`](docs/runbook.md)（起停、探针语义、四类失败的现象 → 先看什么 → 常见原因 → 日志 / job 终态）。

## 7. 5 分钟后去哪

| 我想…… | 读这个 |
|---|---|
| 知道四族接口的参数、响应与错误码 | `docs/api-schema.md` |
| 知道 `ai.*` 有哪些表、谁能写 | `docs/db-schema.md` |
| 知道数据怎么走、哪里 fail-closed | `docs/dataflow.md` |
| 起停 / 排障 / 配置键 | `docs/runbook.md`、`README.md`、`config/ai-service.yaml` |
| 当前在做什么 / 已知缺口 | `docs/TASKS.md` |
| 架构与决策 | `docs/ARCHITECTURE.md`、`docs/adr/` |
