---
status: active
last-verified: 2026-10-25
verified-by: QT-32-A —— copilot 那条链路终点改到「沙箱编译 + promote 闸门」，生成链逐段落 ai.generation_chain
---

# Dataflow — tellurion-agent

> 目的：把「数据从哪来、在谁手里变形、到哪去」写死。**任何跨仓数据交换都必须能在本图上找到一条边**；找不到 = 这段代码在偷偷建第二条路。

## 1. 主链路

```
ui ──(HTTP/SSE)──► tellurion-agent (ai-service :8086)
                        │  ① explore loop：生成假设 → 组策略 → 跑回测
                        │  ② copilot：自然语言 → 代码 → 沙箱编译 → 【闸门】
                        │  ③ pipeline / tools：编排与工具执行
                        ↓
              core analysis-service（HTTP 粗粒度：回测 / walk-forward /
              表达式校验与目录 / 行情只读 / regime）—— 凭证从入站请求转发
                        ↓
              ai.*（本仓自持）/ run（结果）/ market（数据）
```

> **QT-32-A（2026-10-25）改了 ② 这条路的终点**：copilot 到「沙箱编译」为止，**不再**把 AI 产物送去回测。
> 判据是「有没有可核验的产物坐标」：产物要先登记进资产服务、由人 promote，才可能成为可执行资产
> （闸门真相在 `tellurion-strategy:internal/asset/ddl.go` 的 `strategy_artifacts.status`）。
> 旧实现在这里做的事是：`plugin.Open` 打开一个非 plugin 产物必然失败 → 回落成硬编码字面量
> `"generated-strategy"` → 拿它去 core 跑回测。core 从未注册过那个名字，所以这条路从来没通过，
> 只是断点在远端、本仓既没说谎也没留痕。
>
> **QT-32-B（2026-11-10）补上了那个坐标**：编译通过后本仓调 `tellurion-strategy` 的
> `POST /api/strategies` → `PUT /api/strategies/{id}/source`（→ `POST .../build` + 轮询，仅
> wasm/so 等产物载体），拿回**服务端签发**的 `strategy_id` / `version_hash`（产物载体再加
> `artifact_hash`），与生成链同一笔落库。载体默认 `carrier="dsl"`（第三轮裁决 ⑩），dsl 不产
> 字节码 ⇒ 不调 build、`artifact_hash` 为空。**闸门没变**：登记 ≠ 放行，promote 仍是人的闸门，
> 本仓不调 promote、不存结论、不跑回测。

### 生成链（QT-32-A / QT-32-B）

一次 copilot 生成的每一段推进都写 `ai.generation_chain`，一行 = 一次编译尝试：

```
POST /api/copilot/generate
  → 链第 1 行（prompt + 回测区间，build_state=none）   ← 写不进去就到此为止，连 LLM 都不调
  → LLM 出码 → staticcheck 闸（拒 → 源码+指纹留下，停在这行）
  → 每次尝试一行：source + source_sha256 → 先写「要编」，再跑沙箱 go build → 覆盖成 failed / passed
  → 编译通过 → 登记资产服务：strategy_id / asset_carrier / version_hash（+产物载体的 artifact_hash）
       ↳ 登记失败 = fail-closed（终态 sandbox_rejected，blocked_reason 留空，build_error 点名）
  → promote 闸门拦下：blocked_reason=promote_gate，job 终态 sandbox_rejected
```

`source_sha256` 覆盖的是**同一行 `source`** 的字节，不是 `version_hash` 也不是 `artifact_hash`；
后两者是资产服务签发的原值（QT-32-B），本仓只照抄、不自己算。链的最后一环 —— core 的 run 身份 ——
今天仍接不上，`BacktestResponse.ID` 已在客户端解出（`pkg/ai/client/backtest_client.go:285`），
丢在 `cmd/ai/http_runner.go:163`；把它送过 `contracts.BacktestRunner` 那一层是跨仓契约变更
（`docs/TASKS.md` QT-32-A ⑦-2）。

> **QT-17 S3-c-3 起**：本仓进程内没有引擎。pipeline 把 YAML 的表达式段映射成
> core 内置 `expression_template` 的 `strategy_params`，一次 `POST /api/backtest`
> 提交；walk-forward 一次 `POST /api/walkforward`。逐 bar 调用为零。

## 2. 关键时序

### 探索一轮
1. `POST /api/explore/runs` → 建 run（内存态）+ 返回 `runID`
2. 后台 goroutine 跑 loop：每轮 attempt → 组策略 → 一次 `POST /api/backtest`（core 的 `expression_template` + `strategy_params`）。异步 goroutine 的 ctx 必须保留入站 JWT（`WithBearerTokenInContext`），否则回测客户端 fail-closed
3. 每次 attempt 落 `ai.experiments`（**QT-17 ④ 起单写**）：`main.go` 把 `*aistore.Store` 直接注入 pipeline / explore 的 sink，双写适配器已删除；legacy `public.experiments` 在存量环境**停写并冻结**在 30 行（历史行已回填归档，行级对拍 0 差异），**core 的建表 DDL 已于 2026-11-10 随 `SVC-CORE-AI-RETIRE` 删除**（新环境无此表）。`parent_id` 因此直接在 `ai` 的 id 空间内自洽，不再需要 legacyID→aiID 映射
4. ui 订阅 `GET .../stream`：每 tick 推 `progress`（快照与轮询端点同形状）
5. 结束推 `complete`；`POST .../stop` 只停调度，不 cancel 正在跑的回测

### 候选生成与落库（DBT-18）
- `POST /api/explore/runs` 接受 `seed`（未传则墙上时钟，实际值回显并写入日志 / `ai.experiments.hypothesis`）和 `proposer=tpe|mcts`；两者都实现 `loop.Proposer`，编排层不变。
- `pkg/ai/mining.GenerateAndSave` 先完整生成候选，再调 `aistore.SaveFactorCandidates`；生成失败直接返回，**不落半成品**。落库是 `ai.factor_genes` 的单事务整批写入（upsert by id），不写 core 的 `public.factor_genes`。
- 候选生成不调用模型，因此没有 LLM 成本；LLM 提议器将来必须走 `pkg/ai/cost.go`。

### 研究域 / 因子假设读取
- `research.profile` / `research.conclusion` / `research.question` 与 `factor_hypothesis` 的**读面已切 `ai.*`**（自有 `aistore`）；`factor_hypothesis` 的 builtin 种子由本仓启动时 `ON CONFLICT DO NOTHING` 播种。研究域三表**目前没有写入方**（core 侧也无研究域 handler，真库 0 行）——将来接写时直写 `ai.*`，不得落 legacy `research.*`。**2026-11-10 起**：core 这四张表的 DDL 与只读实现已随 `SVC-CORE-AI-RETIRE` 删除，本仓是唯一 owner。

### 表达式校验与目录（QT-17 S3-c-4）
- `validate_factor` 工具（L1 闸门）把表达式发到 core 的 `POST /api/expression/validate`，把 `{valid,inputs,ast,error,stage}` 原样映射成 GateDecision；`valid=false` 是判定，非 200 是工具错误。
- 失败建议里的合法字段清单来自 `GET /api/expression/catalog`；`unavailable`（源表为空 / 未登记）字段**保留在目录里并点名禁用**，不会从数组消失。目录拉不到时建议明说清单缺失。
- 研究与策略生成的 DSL 语法段同样从 catalog 派生（按 core 的算子 category 分组；`source=group` 不是数据字段，不列）。
- L2 的因子值由 core 的 `POST /api/factor/compute` 按默认票池 × 2023 全年算出；NaN 比例与方差判定留在本仓。本仓进程内**没有** DSL parser / evaluator。

## 3. 失败模式（每一处 fail-closed 都要能指名）

| 失败点 | 症状 | 兜底 |
|---|---|---|
| core 回测不可达 / 5xx | attempt 失败 | 记录失败原因；不重试（重试只会把超时叠成更长的超时） |
| 表达式校验不可达 / 非 200 | `validate_factor` 返回工具错误 | **不**降级成 `valid=false`（服务没答 ≠ 公式不合法）；也不本地解析兜底 |
| 目录不可达 | 失败建议缺少字段清单 | 建议里明说「字段目录暂不可用」；不编造字段名 |
| 回测请求缺凭证 | 客户端 fail-closed，请求不发出 | 显式报「没有凭证」；不降级成匿名调用 |
| SSE 断线 | ui 看不到进度 | 只停推、不取消探索；客户端重连拿快照 |
| 双写不一致 | 切读后数据缺 | 双写期已结束；停写前已做一次全量回填 + 行级对拍（0 差异），此后旧表零增长 |
| 工具执行超时 | 卡住 pipeline | 每工具独立超时 + 归类为失败 |
| **AI 产物编译通过但没过 promote 闸门** | 链停在 `blocked_reason=promote_gate`，job 终态 `sandbox_rejected` | **不猜策略名、不本地兜底**：坐标由资产服务签发（QT-32-B）后仍不自动放行 —— promote 是人的闸门。资产服务那侧的 resolve/promote 才是判据来源 |
| **资产服务不可达 / 登记被拒 / 编译作业 failed** | job 终态 `sandbox_rejected`；`build_state` 保留 `passed`（编译确实过了），`blocked_reason` 留空，`build_error` 点名「资产登记失败」+ 上游诊断 | **没有坐标就不算登记成功**：不静默降级成「先跑起来再说」，也不把登记失败伪造成闸门拦下 |
| **留痕落点没装 / 写不进去** | job 终态 `sandbox_rejected`，错误点名 `chain sink not configured` 或 `ai.generation_chain` | LLM **一次都不调**：宁可这次不做，也不产出一份事后查不到的生成 |

## 4. 不变量（可被测试钉住的）

1. `ai.*` 与 core 旧表在停写切换时行级一致（回填归档 + `EXCEPT` 双向 0 差异，见 ODR-001）；停写后 `public.experiments` 行数与序列不再前进。**2026-11-10 起** core DDL 已退役：存量环境该表仍在但冻结，新环境不存在（`experiment_stopwrite_integration_test.go` 用 `to_regclass` 判在不在，两种环境都验）。
2. 同一实验配置 → 同一结果（引擎确定性 + 三 hash 留痕）。
3. 断线不影响 run 的生命周期。
4. 本仓生产代码不出现 `pkg/storage` 的 import——**白名单已清零（QT-17 ③，2026-10-25）**：仪器域不再 import core，交易日历改走 tellurion-data 的 `GET /api/v1/trading/calendar`（凭证从入站请求转发），上市窗口无服务提供（core QT-67 已随 `SVC-CORE-DATA-RETIRE` 退役），由 `internal/repoguard` AST 护栏钉住；**QT-17 S3-d 起全仓源码对 core 的 import 字面量归零**（`grep '"tellurion/'` 无命中），`replace` 已删除。
5. **未过 promote 闸门的 AI 产物不进回测**（QT-32-A）：`pkg/copilot` 的生产源码里不出现 `plugin.Open` / `"generated-strategy"` / `RunBacktest(` 三种形状，由 `TestQT32A_NoFabricatedStrategyPathInProductionSource` 读源码钉住（守卫自身的牙齿靠破坏验证取证，见 `docs/TASKS.md` QT-32-A 施工记录）。
6. **每一次生成都留下一条链**（QT-32-A）：留痕写不进去就中止，不产出无法复查的产物；`ai.generation_chain` 的 `INSERT / SELECT / DO UPDATE` 三处列清单由静态护栏与 DDL 逐列对拍（`pkg/aistore/generation_chain_sql_test.go`）。
7. **链上的资产坐标只能来自资产服务**（QT-32-B）：`strategy_id` / `version_hash` / `artifact_hash` 由 `pkg/ai/client` 从 `POST /api/strategies` → `PUT .../source`（→ `POST .../build` + 轮询）取回，本仓不算；未注入登记端口则编译通过也中止（`TestQT32B_NilRegistrarFailsClosedAfterBuild`），登记失败不会被写成 `blocked_reason=promote_gate`（`TestQT32B_RegistrationFailureIsNotGateBlock`）。

## 5. 与契约文档的关系

- 表结构 → `docs/db-schema.md`
- 接口形状 → `docs/api-schema.md`
- 本文件只讲**流向与顺序**，不重复字段定义。
