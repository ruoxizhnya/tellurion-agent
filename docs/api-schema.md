---
status: active
last-verified: 2026-10-20
verified-by: 功能边界固化（三份契约：db / api / dataflow）
---

# API Schema — tellurion-agent

> **契约地位**：HTTP（ai-service，默认 :8086）。

**契约权威在 core 的 `openapi.yaml`**：本仓实现它，不另立一份。改端点 = 先改 core 的 openapi，再改这里。
> 本文件是**本仓对外接口的唯一真相源**。代码与它不一致时，先判断谁错：契约错 → 改本文 + 立 ADR；实现错 → 改代码。
> 跨仓消费方（core / agent / ui / 其他服务）按本文件编码，**不读本仓实现**。

## 1. 接口面

| 方法 | 路径 | 用途 |
|---|---|---|
| POST | `/api/explore/runs` | 启动一轮探索（实验循环） |
| GET | `/api/explore/runs/{runID}` | 查状态（快照） |
| GET | `/api/explore/runs/{runID}/stream` | SSE 推状态（替代 2.5s 轮询） |
| POST | `/api/explore/runs/{runID}/stop` | 停止（只停调度，不 cancel 已开跑的回测） |
| POST | `/api/copilot/generate` | 自然语言 → 代码生成（异步 job） |
| GET | `/api/copilot/generate/{job_id}` | 生成任务状态 |
| GET | `/api/copilot/stats` | Copilot 生成统计（接受率） |
| POST | `/api/pipeline/run` | 跑 pipeline |
| GET | `/api/pipeline/jobs` · `/jobs/{id}` | pipeline 任务列表 / 详情 |
| GET | `/api/tools` · `/api/tools/{name}` | 工具注册表 |
| POST | `/api/tools/{name}` | 执行工具 |
| GET | `/health` · `/metrics` | 运维面 |

> 这四族（copilot / pipeline / explore / tools）是对外全部；**没有第五族**。加族要先改 openapi 与本文。

## 2. 错误码与 fail-closed 语义

| 情况 | 返回 | 说明 |
|---|---|---|
| run 不存在 | 404 `run not found` | 与轮询端点同语义（不给"一连上就 error"的 SSE） |
| 工具未注册 | 404 | 不返回空结果 |
| 生成任务失败 | job 状态 `failed` + 原因 | 不静默重试 |
| 引擎不可用 | 5xx | 明确失败，不产出"部分结果"当成功 |

**SSE 契约**：事件名 `progress` / `complete`；`progress` 的 payload 与 `GET /runs/{id}` **同 JSON 形状**（客户端复用类型）。

## 3. 幂等性与重试

- `POST /explore/runs` 每次调用创建新 run（不复用），但同 `(strategy, params, universe)` 的结果应一致（确定性由引擎保证）。
- `stop` 幂等：已停止再停返回同状态。
- `POST /tools/{name}` 是否幂等由工具自己声明（`idempotent: true/false` 写进工具契约）。

## 4. 版本与兼容

- 端点只增不删；废弃走 `deprecated` 头。
- SSE 事件形状变更 = breaking（客户端解析会崩），需同步 ui。
- 引擎 ABI 变更（abi v2）会让本仓必须跟上（QT-01 之后）。

## 5. 不提供什么（边界）

- 不做回测引擎（在 core）
- 不做数据抓取（在 data）
- 不做策略执行沙箱（在 core 的 WASM host）
- 不直连 core 表（QT-17 之后连 `pkg/storage` 都不 import）
