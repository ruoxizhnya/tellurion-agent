---
status: active
last-verified: 2026-11-11
verified-by: 裁决 2026-11-11 —— 本文升级为四组端点的**唯一完整定义**（参数 / 响应 / 错误码），并钉住 core 侧只做纯反代 + `x-served-by: agent`；此前补 `/readyz` / `/api/readyz`（`cmd/ai/readyz.go` 实际注册）、QT-32-A promote 闸门与留痕 fail-closed、QT-32-B 字段实况
---

# API Schema — tellurion-agent

> **契约地位**：HTTP（ai-service，默认 :8086）。

**这四组的契约权威就是本文**（2026-11-11 裁决）：`/api/{copilot,pipeline,explore,tools}` 的唯一完整定义在这里，core 侧只做纯反代。core 的 `openapi.yaml` 若仍列这些路径，只是前端索引，以本文为准；改端点 = 改本文 + 本仓实现。
> 本文件是**本仓对外接口的唯一真相源**。代码与它不一致时，先判断谁错：契约错 → 改本文 + 立 ADR；实现错 → 改代码。
> 跨仓消费方（core / agent / ui / 其他服务）按本文件编码，**不读本仓实现**。

## 1. 接口面

> **真相源归属（2026-11-11 裁决）**：`/api/{copilot,pipeline,explore,tools}` 的**实现与契约都在本仓**（ai-service，缺省 :8086）。core 的 analysis-service 对这四个前缀**只做纯反代**：不重新定义参数、响应或错误码，只转发并在响应上加 `x-served-by: agent`。core 侧保留的是反代入口与这一个头，**不是第二份定义**。改契约 = 只改本文 + 本仓实现；core 侧无需跟改。

| 方法 | 路径 | 用途 |
|---|---|---|
| POST | `/api/explore/runs` | 启动一轮探索（实验循环），立即返回 `run_id` |
| GET | `/api/explore/runs/{runID}` | 查状态（快照） |
| GET | `/api/explore/runs/{runID}/stream` | SSE 推状态（`progress` / `complete`） |
| POST | `/api/explore/runs/{runID}/stop` | 停止（只停调度，不 cancel 已开跑的回测） |
| POST | `/api/copilot/generate` | 自然语言 → 代码生成（异步 job） |
| GET | `/api/copilot/generate/{job_id}` | 生成任务状态 |
| GET | `/api/copilot/stats` | Copilot 生成统计（接受率） |
| POST | `/api/pipeline/run` | 跑 pipeline（同步返回整条结果） |
| GET | `/api/pipeline/jobs` · `/jobs/{id}` | pipeline 任务列表 / 详情 |
| GET | `/api/pipeline/experiments/count` | 累计实验计数（由 `ai.experiments` 行派生） |
| POST | `/api/pipeline/adoption` | 记录一次人工干预/采纳裁决（`ai.adoption_ledger`） |
| GET | `/api/pipeline/calibration` | 校准误差 + holdout 命中率 |
| GET | `/api/tools` · `/api/tools/{name}` | 工具注册表 |
| POST | `/api/tools/{name}` | 执行工具 |
| GET | `/health` · `/metrics` | 运维面（存活 / 指标） |
| GET | `/readyz` · `/api/readyz` | 就绪面：DB ping + `ai.*` 迁移关键表；未就绪 503，`/health` 仍 200 |

> 这四族（copilot / pipeline / explore / tools）是对外全部；**没有第五族**。加族要先改 openapi 与本文。
> 路径前缀：`explore` / `copilot` / `pipeline` 挂 `/api/*`；`tools` 同时接受 `/api/tools` 与 `/api/tools/`（尾斜杠变体）。

### 1.0 鉴权三态（`cmd/ai/lifecycle.go::decideAuthStartup`）

启动期一次性裁决，进程生命周期内不再变化；口径与其余五仓统一：

| 态 | 条件 | 行为 |
|---|---|---|
| 1. 全闸 | `JWT_SECRET` 非空 | 中间件校验 token；写面按 `role` 做 RBAC |
| 2. 降级（`auth_unavailable`） | 无 secret + `AUTH_INSECURE=true` + 回环绑定 + 无显式声明 | 进程能起；**读面公开**；**全部写方法与工具执行一律 503** `{"code":"auth_unavailable"}` |
| 3. 放开 | 无 secret + `AUTH_INSECURE=true` + `AUTH_INSECURE_EXPOSURE=loopback-published` | 整体放行；启动 WARN 点名「写面与工具执行无鉴权」；回环绑定时再叠逐请求来源门 |

- 「非回环绑定 + 无 secret + 无声明」**直接拒绝启动**（容器绑 `0.0.0.0` 时不再出现「什么门都不装」）。
- 第 2 态的判定按 **HTTP 方法**：`GET`/`HEAD`/`OPTIONS` 公开，其余方法 503；新增写端点无需来本文登记。
- 第 3 态是**双开关**（ADR-025）：只给 `AUTH_INSECURE_EXPOSURE` 而不设 `AUTH_INSECURE=true` 不算放开。
- 工具执行还有一层按副作用等级的路由内 RBAC（见 §1.4）；无鉴权（第 2 态）时不会 `return true` 放行，而是同样 503。

### 1.1 Explore（`cmd/ai/handlers_explore.go`）

**`POST /api/explore/runs`** — 请求体 `StartExploreRequest`：

| 字段 | 类型 | 必填 | 默认 | 说明 |
|---|---|---|---|---|
| `description` | string | 是 | — | 自然语言目标，例如「做一个动量策略」 |
| `max_tries` | int | 否 | `20` | `<=0` 时取 20 |
| `lookback_min` | int | 否 | `5` | 回看窗口搜索下界；`<=0` 时取 5 |
| `lookback_max` | int | 否 | `250` | 上界；`<=lookback_min` 时取 250 |
| `seed` | int64 | 否 | 墙上时钟 | 提议器随机种子；实际使用值在响应 / 状态 / 日志回显 |
| `proposer` | string | 否 | `tpe` | 仅 `tpe` / `mcts`；其他值 400 |

响应 `200`：`{"run_id": "<uuid>", "max_tries": <int>, "seed": <int64>, "proposer": "<tpe|mcts>"}`
错误：`400`（JSON 非法 / 未知 proposer）。

**`GET /api/explore/runs/{runID}`** — 响应 `200` `ExploreStatusResponse`：

| 字段 | 类型 | 说明 |
|---|---|---|
| `run_id` | string | 本轮标识 |
| `running` | bool | 是否仍在跑 |
| `stopped` | string | 仅停止后出现 |
| `attempts` | `AttemptView[]` | 逐次尝试（见下） |
| `best_seq` | int? | 当前最优尝试序号，无则省略 |
| `seed` | int64 | 本轮实际种子 |
| `proposer` | string | 本轮提议器 |

`AttemptView`：`seq`(int) / `num_trials`(int) / `hypothesis`(string) / `params`(object) / `experiment_id`(int64) / `sharpe`(float) / `ok`(bool) / `error`(string, 仅失败) / `verdict`(object, 仅成功尝试；失败为 null)。`num_trials` 是统计维实际使用的累计尝试数（含持久化基数），与进程内 `seq` 不同：重启后 `seq` 归零，`num_trials` 不会。
错误：`404` `run not found`。

**`GET /api/explore/runs/{runID}/stream`** — SSE；`progress` 的 payload 与上面的快照**同 JSON 形状**，跑完推一帧 `complete`；`404` 同快照端点。断线只停推，不取消 run。

**`POST /api/explore/runs/{runID}/stop`** — 响应 `200` `{"run_id": "...", "stopping": true}`；`404`。

### 1.2 Copilot（`cmd/ai/handlers_copilot.go` · `pkg/copilot`）

**`POST /api/copilot/generate`** — 请求体 `GenerateParams`：

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `description` | string | 是 | 自然语言策略描述 |
| `universe` | string | 否 | `csi300` / `csi500` / `csi800` / `all` |
| `start_date` | string | 否 | `YYYY-MM-DD` |
| `end_date` | string | 否 | `YYYY-MM-DD` |

响应 `202`：`{"job_id": "...", "status": "pending"}`。
错误：`503` `AI not configured (set AI_API_KEY and AI_API_URL)`；`400` `invalid request: <原因>`。

**`GET /api/copilot/generate/{job_id}`** — 响应 `200`：

| 字段 | 出现条件 | 说明 |
|---|---|---|
| `job_id` | 恒有 | — |
| `status` | 恒有 | 今天实际取值：`pending`（运行中）/ `sandbox_rejected` / `build_failed` / `llm_failed`。契约另预留 `generated` / `backtest_complete` / `backtest_failed`（前端白名单在消费，今天不产生） |
| `generated_code` | 非空时 | 生成的 Go 源码 |
| `build_error` | 非空时 | 给人看的失败解释（编译失败 / 闸门拦下 / 资产登记失败） |
| `strategy_name` | 非空时 | 今天恒不出现（promote 闸门拦在生产回测之外） |
| `backtest_result` | 非空时 | 今天恒不出现（同上） |
| `backtest_error` | 非空时 | 今天恒不出现（同上） |

错误：`404` `job not found`。

**`GET /api/copilot/stats`** — 响应 `200`：`{"generated": int, "buildable": int, "backtest_valid": int, "acceptance_rate": float}`。`backtest_valid` / `acceptance_rate` 结构上恒为 0（promote 闸门之后没有回测跳；不是回归，见 §2 注）。

### 1.3 Pipeline（`cmd/ai/handlers_pipeline.go` · `pkg/ai/pipeline`）

**`POST /api/pipeline/run`** — 请求体 `{"description": "<string，必填>"}`。同步执行整条流水线，响应 `200` `PipelineJobResponse`（即使某阶段失败也返回**部分结果**，不抛 5xx）：

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | string | 作业 ID |
| `status` | string | pipeline 结果状态 |
| `intent` | object? | 意图解析结果 |
| `yaml_config` | string? | 生成的策略 YAML |
| `generated_code` | string? | 生成的 Go 源码 |
| `build_error` | string? | 编译失败原因 |
| `backtest_result` | object? | 回测结果 |
| `backtest_error` | string? | 回测失败原因 |
| `started_at` | string | `2006-01-02T15:04:05Z` |
| `completed_at` | string? | 同上格式，未完成省略 |
| `duration_ms` | int64 | 耗时 |
| `logs` | string[]? | 阶段日志 |

错误：`400`（请求体非法）。

**`GET /api/pipeline/jobs`** — 响应 `200`，**今天恒为 `[]`**：pipeline 任务未持久化，列表端点没有数据源（不是空指针，是明确的空数组）。
**`GET /api/pipeline/jobs/{id}`** — 响应 `200` `PipelineJobResponse`；`404` `job not found`。

**`GET /api/pipeline/experiments/count`** — 响应 `200` `{"count": <int64>}`：`ai.experiments` 的累计行数（explore 尝试 + `backtest.run` / `walk_forward_validate` 工具直调写下的行）。计数由行派生，进程重启不丢。未接计数来源时 `503`（不返回 `0` 冒充「没跑过」）；读库失败 `500`。

**`POST /api/pipeline/adoption`** — 请求体：

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `proposal_id` | string | 是 | 提案唯一标识；重复写入 → `409`（决策不可回写） |
| `actor` | string | 是 | 干预/裁决者 |
| `intervention` | string | 否 | 人改了什么（空 = 无干预直接裁决） |
| `adopted` | bool | 是 | 采纳与否；缺失 → `400`（不得默认成否决） |
| `claimed_confidence` | float | 是 | 提案方自报命中概率，必须在 `[0,1]`，否则 `400` |
| `is_holdout` | bool | 否 | `true` = 进留出对账集，永不进校准分母 |
| `outcome_hit` | bool | 否 | 结果命中与否；缺省 = 未回填（pending） |
| `result_ref` | string | 否 | 结果坐标（如 `exp:123`） |

响应 `201` `{"id": <int64>, "proposal_id": "..."}`；`409` 重复提案；`503` 台账未接。`outcome_hit` 非空时自动落 `resolved_at`。

**`GET /api/pipeline/calibration`** — 响应 `200`：

```json
{
  "sample_size": 3,
  "hit_rate": 0.6667,
  "claimed_rate": 0.6,
  "calibration_error": 0.0667,
  "brier_score": 0.2267,
  "pending": 1,
  "holdout": {"sample_size": 1, "hit_rate": 1.0}
}
```

口径（实现在 [`pkg/ai/adoption`](../pkg/ai/adoption)）：校准集 = 已回填结果且 `is_holdout=false` 的行；`calibration_error = |hit_rate - claimed_rate|`；`brier_score = mean((claimed_confidence - outcome)^2)`；`is_holdout=true` 的行只进 `holdout`，不参与校准；`outcome_hit` 为空的行只进 `pending`。未接台账时 `503`。

### 1.4 Tools（`cmd/ai/handlers_tools.go` · `pkg/ai/tools`）

**`GET /api/tools`** — 响应 `200`：`{"tools": ToolInfo[], "count": int}`（按 name 升序）。
`ToolInfo`：`name`(string) / `description`(string) / `parameters`(object[]，实现 `SchemaProvider` 才有) / `output_schema`(object，同上)。
**`GET /api/tools/{name}`** — 响应 `200` 单个 `ToolInfo`；错误 `404` `{"error":"...","code":"TOOL_NOT_REGISTERED"}`。
**`POST /api/tools/{name}`** — 请求体 `{"args": {<tool 参数>}}`；成功 `200` `{"result": <任意 JSON>}`。

执行的鉴权在**路由内**按工具的副作用等级判定（路由是通配 `:name`，无法用中间件表达）：

| 工具等级 | 允许角色 |
|---|---|
| read | `viewer` / `trader` / `admin` |
| write | `trader` / `admin` |
| admin（含未登记分类，fail-closed） | `admin` |

错误体统一为 `{"error": "...", "code": "<CODE>"}`：

| HTTP | code | 触发 |
|---|---|---|
| 400 | `INVALID_BODY` | 请求体不是合法 JSON / 缺 `args` 结构 |
| 400 | `INVALID_ARGS` | 工具参数校验失败（`tools.ErrInvalidArgs`） |
| 401 | `UNAUTHENTICATED` | 鉴权开启但上下文无身份 |
| 403 | `INSUFFICIENT_ROLE` | 角色不足（另带 `tool` / `effect` / `have` / `need`） |
| 404 | `TOOL_NOT_REGISTERED` | 工具名未注册 |
| 404 | `NOT_FOUND` | 请求合法但记录不存在（如 `research.profile` 无档案） |
| 503 | `auth_unavailable` | 未配置鉴权（无 `JWT_SECRET` 且未显式声明放开，见 §1.0）；工具执行 fail-closed |
| 500 | `TOOL_EXECUTION_FAILED` | 下游 / 执行错误 |

> 鉴权第 3 态（显式声明放开）不做 RBAC；第 2 态（`auth_unavailable`）不放行任何执行；
> 第 1 态按上表 RBAC。任何形态都不允许「没装配鉴权 ⇒ 默认放行」。

### 1.5 运维面

`GET /health`（存活，不碰库）/ `GET /metrics` / `GET /readyz` · `/api/readyz`（DB ping + `ai.*` 关键表；未就绪 503，`/health` 仍 200）。

## 2. 错误码与 fail-closed 语义

| 情况 | 返回 | 说明 |
|---|---|---|
| run 不存在 | 404 `run not found` | 与轮询端点同语义（不给"一连上就 error"的 SSE） |
| 工具未注册 | 404 | 不返回空结果 |
| 生成任务失败 | job 状态 `failed` + 原因 | 不静默重试 |
| 引擎不可用 | 5xx | 明确失败，不产出"部分结果"当成功 |
| **AI 产物未过 promote 人工闸门** | job 终态 `sandbox_rejected`，`build_error` 只给人看的解释（「编译通过并已登记资产坐标…；未过 promote 人工闸门，不进回测」）；链上那一行用 `blocked_reason=promote_gate` 说话 | **编译通过也不去跑回测**：坐标由资产服务签发（QT-32-B）也不是放行证 —— 由人 promote 才成为可执行资产。两条轴不共用一列（「编不过」≠「编过了但不许跑」）。闸门结论的真相在 `tellurion-strategy`，本仓不存第二份 |
| **资产服务不可达 / 登记被拒 / 编译作业 failed** | job 终态 `sandbox_rejected` + `build_error` 点名「资产登记失败」并带上游诊断；链上 `build_state=passed`、`blocked_reason` 留空 | **没有坐标就不算登记成功**：不把环节错误伪装成闸门拦下（否则读库的人分不清「过了没放行」与「根本没登记成」） |
| 生成链写不进去 / 没注入留痕落点 | job 终态 `sandbox_rejected` + `build_error` 点名 `ai.generation_chain` 或 `chain sink not configured` | **一条记不下来的生成 = 不做的生成**；LLM 在这一步之前不会被调用（不花钱买一份无法复查的产物） |

> **`/api/copilot/generate/{job_id}` 今天的字段实况（QT-32-A 2026-10-25；QT-32-B 2026-11-10 复核）**：`strategy_name` / `backtest_result` / `backtest_error` 与状态取值 `generated` / `backtest_complete` / `backtest_failed` **仍不会出现** —— 闸门把这条路拦在生产回测之外。它们仍是契约的一部分（`tellurion-ui:web/src/types/api.ts` 与 `api/copilot.ts` 的终态白名单在消费），**不删**。QT-32-B 已把资产坐标（`strategy_id` / `version_hash`，dsl 无 `artifact_hash`）落进 `ai.generation_chain`，但**跑回测**是 core 执行侧的一跳：填上 `backtest_result` 需要那一跳落地，不是本单的 wire 变更。同理 `/api/copilot/stats` 的 `backtest_valid` 与 `acceptance_rate` 结构上为 0 —— 这不是回归：旧实现拿硬编码策略名去 core 跑回测，那个名字从未注册过，两个字段此前也恒为 0。

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
