---
status: active
last-verified: 2026-11-12
verified-by: AGENT-LLM-BUDGET-GATE —— 新增本仓自持表 `ai.ai_costs`（LLM 日累计成本账本，日累计 USD 闸的持久化来源）；此前 AGENT-INTERVENTION-ADOPTION-LEDGER 新增 `ai.adoption_ledger`（干预/采纳台账 + holdout 对账集）；QT-32-B `ai.generation_chain` 加资产坐标四列
---

# DB Schema — tellurion-agent

> **所有权声明**：本仓**拥有 `ai` schema 的 DDL 与迁移**（目标态，QT-17）。

**现状（QT-17 ⑤，2026-10-25；④ 2026-10-24；③ 2026-10-25）**：`ai` schema 七张表的 DDL 与迁移入口在本仓 [`pkg/aistore`](../pkg/aistore)（逐列对照 core 定义搬入，`parent_id` 自引用与外键改指 `ai.*`）；自有 `Store`（`kit/db` 连接池 + 启动迁移 + 内置种子）已装配进 `cmd/ai`。**写路径已停写 legacy**（QT-17 ④）：`cmd/ai/main.go` 直接把 `*aistore.Store` 注入 pipeline / explore 的 sink，双写适配器 `cmd/ai/experiment_dualwrite.go` 已整体删除；`public.experiments` 冻结在 30 行（2026-10-24 一次性回填归档进 `ai.experiments`，行级 `EXCEPT` 双向对拍 0 差异）。**读路径已切 `ai.*`**（research 域三表与 `factor_hypothesis`），仪器域日历走 tellurion-data 的 `GET /api/v1/trading/calendar`，上市窗口无服务提供（QT-17 ③；core QT-67 已退役）。core 侧这七张表的遗留 DDL 已于 **2026-11-10（`SVC-CORE-AI-RETIRE`）删除**（原在 `tellurion:pkg/storage/postgres.go` 的 `migrationStatements()`）——存量环境的旧表只保留不删，core 侧导出符号随 DDL 一并退役；`ai.*` 的列形状以本仓 `pkg/aistore` 的 `Statements()` 为运行时真值。**基因池读写已切 `ai.*`**（QT-17 ⑤，2026-10-25）：`pkg/ai/gene_pool` 的表引用全部显式限定 `ai.factor_genes` / `ai.strategy_genes`，连接池改为 `aistore.Store.Pool()`（自持句柄），不再借 core 的 `store.DB()` 按 `search_path` 落到 `public.*`。
> 上级依据：`tellurion:docs/design/qt-decoupling/data-service-contract-and-ownership.md` §1 —— **DDL 只有一个 owner**，同一个 PG 实例、每服务一个 schema，将来拆库是"按 schema 搬走"而不是重写。
> 本文件是本仓表结构的**变更权威**：要改表结构，先改本文件（结构级变更还要立 ADR）再改代码；代码里 [`pkg/aistore/schema.go`](../pkg/aistore/schema.go) 的 `Statements()` 是运行时真值，两者不一致即缺陷。

## 1. 归属表

### 目标：`ai` schema（QT-17 迁入）

| 表 | 内容 | 现状 |
|---|---|---|
| `ai.experiments` | 实验记录 | **单写已切（QT-17 ④）**：写路径只剩 `ai.experiments`；旧表 `public.experiments` 停写、冻结 30 行（历史行已于 2026-10-24 回填归档，行级对拍 0 差异）。core 的建表 DDL 已于 **2026-11-10 随 `SVC-CORE-AI-RETIRE` 删除**（存量环境仍冻结、新环境无此表）。本仓只写不读 |
| `ai.factor_hypothesis` | 内置/人写因子假设 | **唯一 owner**：core 的 `public.factor_hypothesis` DDL 与启动 seed 已于 2026-11-10 退役；builtin 种子由本仓启动时 `ON CONFLICT DO NOTHING` 播种 |
| `ai.factor_genes` | 因子基因池 | **读写已切（QT-17 ⑤）**：SQL 全部限定 `ai.factor_genes`，池为 `aistore` 自持；旧表 `public.factor_genes` 停写（真库 0 行，未产生过脏数据），core DDL 已于 2026-11-10 删除 |
| `ai.strategy_genes` | 策略基因池 | **读写已切（QT-17 ⑤）**：SQL 全部限定 `ai.strategy_genes`，池为 `aistore` 自持；旧表 `public.strategy_genes` 同上（core DDL 已于 2026-11-10 删除） |
| `ai.research_profile` | 研究画像 | **唯一 owner**：core `research.profile` DDL 与只读实现已于 2026-11-10 退役；本仓只读不写，**全工作区不存在 `research.*` 的写入方**（2026-10-24 实测：core 侧无研究域 handler，真库 0 行） |
| `ai.research_conclusion` | 研究结论 | **唯一 owner**：core `research.conclusion` DDL 已于 2026-11-10 退役；本仓只读不写，写方不存在 |
| `ai.research_question` | 研究问题 | **唯一 owner**：core `research.question` DDL 已于 2026-11-10 退役；本仓只读不写，写方不存在 |
| `ai.generation_chain` | **AI 生成链**：一次生成的意图、源码、指纹、编译结论、资产坐标、链断在哪一环 | **QT-32-A 新增（2026-10-25）、QT-32-B 加坐标列（2026-11-10），本仓自持、core 无对应表**。一行 = 一次编译尝试，主键 `(job_id, attempt)`；列：`prompt`（人给的自然语言意图，NOT NULL）/ `universe`·`start_date`·`end_date`（人要求的回测区间，闸门拦下也照样留）/ `source`（这一 attempt 的源码）/ `source_sha256`（**同一行 source** 的 sha256）/ `strategy_id`·`asset_carrier`·`version_hash`·`artifact_hash`（**资产服务签发的原值**，QT-32-B；dsl 载体的 `artifact_hash` 为空）/ `build_state`（`none｜failed｜passed`，DDL CHECK 闭合词表）/ `build_error` / `blocked_reason`（`promote_gate` 或空）/ `created_at`·`updated_at` |
| `ai.adoption_ledger` | **人工干预 / 采纳台账 + holdout 对账集** | **AGENT-INTERVENTION-ADOPTION-LEDGER 新增（2026-11-12），本仓自持、core 无对应表**。一行 = 一个提案（`proposal_id` 唯一）的一次人工裁决；列：`actor`（干预/裁决者）/ `intervention`（改了什么，可空）/ `adopted`（采纳与否，NOT NULL）/ `claimed_confidence`（提案方自报命中概率，CHECK 0..1）/ `is_holdout`（留出对账集，**永不进校准分母**）/ `outcome_hit`（结果命中与否，NULL = 未回填，单列 pending）/ `result_ref`（结果坐标）/ `created_at`·`resolved_at`。读面 `/api/pipeline/calibration` 口径见 `pkg/ai/adoption`：校准误差 = \|实际命中率 − 平均声称概率\|，holdout 只单独报 |
| `ai.ai_costs` | **LLM 日累计成本账本** | **AGENT-LLM-BUDGET-GATE 新增（2026-11-12），本仓自持、core 无对应表**。一行 = 一个 UTC 日 × 一个模型，主键 `(day, model)`；列：`prompt_tokens`·`completion_tokens`·`cost_usd`·`calls`（均默认 0，`calls` 每次调用 +1）/ `updated_at`。唯一写入口 `pkg/aistore.AddLLMCost`（upsert 累加），读面 `DailyLLMCost`（当日 SUM）。**日累计 USD 闸**（`pkg/ai/budget`）在每次 LLM 调用前读这里、成功后写回：超限拒绝并返回 `llm_budget_exhausted`，重启后从本表重读，不归零 |
| `ai.explore_marks` | **explore 结果流上的人工轻量标注（G4）** | **AGENT-EXPLORE-INTERVENTIONS 新增（2026-11-12），本仓自持、core 无对应表**。一行 = 一次「派生子 run / 提升权重」标注；列：`run_id`·`attempt_seq`（与 `ai.experiments` 同坐标）/ `kind`（`derive｜promote`，DDL CHECK 闭合词表）/ `note`（自由文本）/ `actor` / `experiment_id`（join 回那次尝试）/ `created_at`；索引 `(run_id, attempt_seq)`。**不是裁决**：不参与校准、不覆盖历史，同一坐标允许重复标记。写面要求内存运行态里 run 与 seq 都存在（重启后旧 run 不可再标） |

> **`ai.generation_chain` 的三条边界**（决策全文见 [`docs/adr/0002`](adr/0002-AI-生成链留痕归本仓-闸门结论不归本仓.md)）：
> 1. `source_sha256` **不是** `version_hash` 也不是 `artifact_hash`。前者按 `tellurion-contracts:engine/provenance.go:65` 需要资产服务签发的 `strategy_id`，后者按 `:39` 只覆盖编译产物字节。**QT-32-B 起坐标列存的是资产服务返回的原值**（`pkg/ai/client` 取回、本仓不计算），但 `source_sha256` 仍只是同字节的指纹，别拿它当执行真相。
> 2. **不存 promote 结论**。闸门真相在 `tellurion-strategy:internal/asset/ddl.go` 的 `strategy_artifacts.status` 状态机（`built→validated→promoted/rejected/deprecated`）；本仓再存一份就是两处真相。坐标是**身份**，不是结论。
> 3. 它是 QT-17「逐表搬、不臆造」的**显式例外**，登记在 `pkg/aistore/schema_core_drift_test.go` 的 `agentOwnedTables`；那张豁免名单由 `TestSchema_AgentOwnedTablesAreDeclared` 四条腿钉住（没登记 / DDL 里没有 / 与 core 退役对照同时出现 / 理由为空 —— 任一都红）。

> 迁入纪律（QT-17）：**以 core 现有 DDL 为准逐表搬**，不按类型名臆造表名。**2026-11-10 起**（`SVC-CORE-AI-RETIRE`）：七张 core 遗留 DDL 已删除，`TestSchema_ColumnsMatchCore` 的逐列对拍随之退场，改为 `TestSchema_RetiredCoreTablesAreGone` —— core 里七表零 `CREATE TABLE` 才绿。`ai.*` 的列形状以本仓 `Statements()` 为运行时真值（本文件同步登记）。

## 2. 只读输入（不属于本仓，禁止写入）

| 数据 | 真实 owner |
|---|---|
| 行情 / 因子值 | `tellurion-data` |
| 回测结果 | core / `run` |
| 交易日历 / 上市窗口（仪器域） | tellurion-data（日历，`GET /api/v1/trading/calendar`）/ 无服务（上市窗口，重开见 data `TASKS.md` ⏸）—— 经**只读 HTTP** 取，不直连表 |
| 规则快照 | `tellurion-risk` |

**红线**：本仓生产代码不 import core 的 `pkg/storage`。2026-10-25（QT-17 ③）起过渡期白名单**已清零**：`cmd/ai/main.go`（改走 `kit/db.BuildDSN`）、`cmd/ai/experiment_dualwrite.go`（随停写删除）、`cmd/ai/handlers_explore.go`（仪器域 `ListingWindow` 改走只读端口；core QT-67 已于 2026-11-11 退役，上市窗口无服务提供）三条全部收回。`storageWhitelist` 现在是空登记簿，由 AST 护栏钉住（任何新 import 当场红；条目失效同样红）。

## 3. 红线

1. **实验域数据自持**：写 `ai.*`，不写 core 的表（双写期已于 2026-10-24 结束，例外收回）。
2. **不 import core 实现包**（`pkg/storage` / `pkg/backtest` / `pkg/strategy`）；跨边界只走 contracts。
3. **逐 bar / 逐笔零远程**：回测循环里出现 HTTP 就是错的。引擎在 core 进程内执行；本仓只按整段区间发起一次 HTTP 请求（QT-17 S3-c-3），逐 bar 仍在单个进程内完成。
4. **停写后旧表零增长**：`public.experiments` 行数与序列不再前进，由 `cmd/ai` 的真库腿 `TestStopWrite_LegacyExperimentsFrozen` 与 `internal/repoguard` 的 import 护栏共同钉住。
5. 实验记录必须能回答"用哪份策略 / 数据 / 规则"（三 hash，X0）。

## 4. 建表与迁移

**现状（必须先知道）**：本仓的迁移是「**启动时按序执行一组幂等 DDL**」——唯一真相是 [`pkg/aistore/schema.go`](../pkg/aistore/schema.go) 的 `statements` 数组（`kit/db.Migrate` 在服务启动时逐条跑）；**没有版本表、没有 down、没有执行记录**（与 core 同口径，理由见 core 的 `migrations/README.md`）。core 的 `pkg/storage/postgres.go` 已随 `SVC-CORE-AI-RETIRE`（2026-11-10）删除这七张表的 DDL；core 的 `migrations/` 与 `docs/migrations/` 只是 core 自己的历史镜像，与本仓无关。

**本仓约定**（从建仓第一天执行）：

- **执行机制（已定，2026-10-23）**：选 ①「core 式」——`pkg/aistore/schema.go` 的 `statements` 数组是 DDL 唯一真相，经 `kit/db.Migrate` 按序执行，幂等、无版本表、无 down。**不设 `migrations/*.sql` 镜像**（双份必然漂移）；历史镜像若将来需要，只能由数组机械生成。**启动接线已落（QT-17 ②）**：`cmd/ai` 用自有 `kit/db` 连接池装配 `aistore.NewStore`（连接 + 迁移 + 内置种子一步到位），与 core 的 `bootstrap.InitStore` 并行、互不借句柄（D10：连接能力用 kit，数据归属自持）。
- 每条必须可重入（`IF NOT EXISTS`），且能在空库上一次跑完；`TestStatements_AreIdempotent` 与 `TestMigrate_Integration` 钉住这两点。
- 表名 / 列曾与 `tellurion:pkg/storage/postgres.go` 逐表一致，由 `TestSchema_ColumnsMatchCore` 直接读 core 源码对拍；**2026-11-10 起**该对拍随 core 七表 DDL 退役翻成 `TestSchema_RetiredCoreTablesAreGone`（core 重建即红），`ai.*` 的列形状以本仓 `Statements()` 为准。
- 只前滚：已合并的迁移不改；修错靠新增一条。
- 跨 schema 取数只走**只读视图或 API**；出现 `join` 别人的表就是边界腐化的第一信号。
- 结构变更的同步义务：本文件 + `docs/ARCHITECTURE.md` 的「数据归属」节必须同一次提交里更新。

## 5. 待定 / 已知未落项（如实登记，不写成已完成）

1. ✅ **已收口（QT-17 ⑤，2026-10-25）：基因池读写已切 `ai.*`**。本条原先记录的事故面——SQL 用未限定表名、经 core 的 `store.DB()` 按 `search_path` 落到 `public.*`——已消除：`pkg/ai/gene_pool` 的 18 处表引用全部写成 `ai.factor_genes` / `ai.strategy_genes`，`cmd/ai/main.go` 改注入 `aistore.Store.Pool()`。护栏两条：`TestSQL_TableRefsAreSchemaQualified`（静态，只解析字符串字面量）与 `TestPools_WriteOnlyAiSchema`（真库腿，写后 `public.*` 同 id 行必须为 0）。旧表 `public.factor_genes` / `public.strategy_genes` **停写**（切换前真库两表各 0 行，无脏数据），core 侧 DDL 已于 2026-11-10 随 `SVC-CORE-AI-RETIRE` 删除。
2. **`research.*` 的「停写」是无操作，且表已退役**：全工作区不存在 `research.profile/conclusion/question` 的生产写入方；core 的只读实现 `pkg/storage/research.go` 与三表 DDL 已于 2026-11-10 随 `SVC-CORE-AI-RETIRE` 删除，真库 0 行（历史数据未删，留给运维归档）。此前本文与 `docs/TASKS.md` 写的「写方在 core / core 侧停写是它的待办」**与代码事实不符**，已在本版订正。因此这三表不存在「停写」动作，只存在「将来接写时直写 `ai.*`」的约束。
3. **`ai.research_*` 目前也没有写入方**：本仓只有 `aistore.GetResearchProfile`（读）；`research.profile` 工具的降级回退是 vault 镜像，不是写库。将来接写时直接写 `ai.*`。
4. **core 侧 deprecated 标记与删除期限**：已于 2026-11-10 随 `SVC-CORE-AI-RETIRE` 一次性删除（比建议期限 2027-01-31 提前）；交接件见 `docs/TASKS.md` QT-17 的「core 侧交接件」小节（历史记录保留）。
5. **`grep -r "pkg/storage"` 的现状**：**生产代码已 0 处**（QT-17 ③，2026-10-25：仪器域上市窗口改走只读端口；core QT-67 已于 2026-11-11 退役）；剩余命中只在 `_test.go`（如 `pkg/ai/validation/live_backtest_integration_test.go` 查仪器域）、文档与护栏注释里。整体归零依赖 S3 删 `replace`（QT-50），见 `docs/TASKS.md` QT-17 的 S3 前置状态。
6. **`ai.generation_chain` 里 `artifact_hash` / `version_hash` 一个都没有（QT-32-A 有意留空，不是漏做）**：本仓 `go build` 的临时可执行文件不是资产服务认可的载体，产不出 `artifact_hash`；`version_hash` 的六项输入含资产服务签发的 `strategy_id`，本仓自算只会造一个没人认的地址。等「AI 生成物的载体」裁下来（wasm + host ABI / so / 维持 ADR-024 的「只审阅不执行」）再立单接 `PUT /api/strategies/{id}/source` + `POST .../build`。**不留空列**：留了就是给将来的假 hash 预留位置。
7. **链的第五环（core 的 run 身份）今天接不上**：`BacktestResponse.ID` 其实已经解到 `BacktestOutcome.JobID`（`pkg/ai/client/backtest_client.go:285`），丢在适配层 `cmd/ai/http_runner.go:163`；但 `contracts.BacktestRunner` 的签名不带它（`pkg/ai/pipeline/backtest_runner_alias_test.go` 还钉着那是个类型别名）→ 要穿过这一层是跨仓契约变更，另单。落点已登记在 `docs/TASKS.md` QT-32-A ⑦-2。
8. **真库腿本机未跑**：`pkg/aistore/generation_chain_integration_test.go` 的**三条** opt-in（同 `(job_id, attempt)` 写两次 = 一行且是最后状态、再写第三个 attempt = 两行且按序且指纹不同 / DDL `CHECK` 拒未登记状态 / 未知 job 读回空列表不是 error）都要 `TEST_DATABASE_DSN` 才跑；本机无 PG 监听、本轮未起 PG 容器（Docker 已达，2026-10-26 订正）→ 只证到显式 skip，不装成通过。第四条 `RejectsEmptyJobID` 刻意**不连库**（校验在 `Exec` 之前）。不需要库、CI 真在跑的是静态腿：`generation_chain_sql_test.go` 四条（`INSERT`/`SELECT`/`DO UPDATE SET` 列集与 DDL 逐列对拍 + 冲突键 + 词表↔`CHECK`）与 `cmd/ai/chain_sink_test.go` 五条（两套词表映射闭合、未登记值报错不传库）。
