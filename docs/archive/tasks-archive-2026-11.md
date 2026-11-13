# TASKS 归档 — tellurion-agent（2026-11）

> **本文件是 `docs/TASKS.md` 的历史归档，进 git、不删除**。归档时间 2026-11-11；收录范围 = 截至 2026-11-10 已收口（✅）的任务原文与当时的施工记录，**逐字搬入、未改写**（原文件 frontmatter 留在新台账；任务标题与正文原样保留）。

> **检索方式**：按任务 ID `grep -n "<ID>" docs/archive/tasks-archive-2026-11.md`；按 commit `grep -n "<sha>" docs/archive/tasks-archive-2026-11.md`；索引表在本节下方，含 ID / 标题 / 收口日期 / commit / 证据链接。

## 检索索引

| ID | 标题 | 收口日期 | commit / 正文所记哈希 | 证据链接 |
|---|---|---|---|---|
| B4 | agent 侧 contracts import 全替 | 2026-10-23 | —（原文未记） | [原文](#B4)；[`archive/contracts-import-replacement-2026-10-23.md`](archive/contracts-import-replacement-2026-10-23.md) |
| OPS-01-A | 观察页 run 状态 SSE 推送端点 | 2026-10-15 | —（原文未记） | [原文](#OPS-01-A) |
| OBS-01-R1-A | 偏差维输出一个它没量过的数字 | 2026-10-14 | `f432074`、`5225df5` | [原文](#OBS-01-R1-A) |
| OBS-01-R2-A | AI 客户端把 K9 三值丢在边界上 | 2026-10-22 | —（原文未记） | [原文](#OBS-01-R2-A) |
| REV-01-A | 消费侧对齐 refactor 修复传播（wire 契约 + 三值口径）✅ 完成（2026-10-22，已归档） | 2026-10-22 | —（原文未记） | [原文](#REV-01-A)；[`archive/TASKS-2026-10-22-archived.md`](archive/TASKS-2026-10-22-archived.md) |
| SVC-AGENT-G0 | 服务就绪门 G0：自有 config 与部署（ADR-036） | 2026-10-26 | —（原文未记） | [原文](#SVC-AGENT-G0) |
| SVC-CORE-AI-RETIRE | core 侧 AI 遗留表与写方法退役（ADR-036 G3） | 2026-11-10 | —（原文未记） | [原文](#SVC-CORE-AI-RETIRE) |
| DBT-18 | mining 的真实候选生成器（LLM / MCTS / 基因池 → `ai.*`） | 2026-11-07 | —（原文未记） | [原文](#DBT-18) |
| QT-17 | agent 数据自持（D10：`ai.*` schema 建表与迁移）✅（2026-10-25） | 2026-10-25 | `2a4b076`、`865f7bf`、`b1a8fcd` | [原文](#QT-17)；[`docs/s3-decoupling-plan.md`](s3-decoupling-plan.md) |
| QT-61 | QT-61（agent 腿）· CI 基线归零——OBS-08 注册表/护栏测试漂移 | 2026-10-21 | `7f711e5` | [原文](#QT-61) |
| QT-65 | QT-65（agent 腿）· QT-11 跨仓回归：pipeline / yaml 生产路径改工厂注册 | 2026-10-24 | `67bee39`、`2629ac7`、`b1a8fcd` | [原文](#QT-65) |
| QT-32-A | QT-32-A（agent 腿）· AI 生成链留痕 + promote 闸门的生产者侧 fail-closed | 2026-10-25 | `55dbf05` | [原文](#QT-32-A) |
| QT-32-B | AI 生成物接资产服务，拿真 `version_hash` / `artifact_hash` | 2026-11-10 | —（原文未记） | [原文](#QT-32-B) |
| AUTH-3STATE | 鉴权未启用时默认放行（三态落闸，含工具 RBAC） | 2026-11-12 | `9c048b4` | [施工记录](#AUTH-3STATE)；[`api-schema.md` §1.0](api-schema.md)、[`AGENTS.md` §2.1](../AGENTS.md) |
| X-DEP-BUMP-CONSUMERS | 消费方依赖抬升（kit v0.5.0 / pgx v5.11.0 / go 1.26.11）+ govulncheck 门 | 2026-11-12 | 见本条 | [施工记录](#X-DEP-BUMP-CONSUMERS) |
| X-AGENTS-DEPENDENCY-TABLE | `AGENTS.md` §4 依赖表按实际 `go.mod` 订正 | 2026-11-12 | 见本条 | [施工记录](#X-AGENTS-DEPENDENCY-TABLE) |
| X-CI-RACE-STATICCHECK | CI 补 `-race` 与 `staticcheck`（含 20+ 条存量告警清理） | 2026-11-12 | 见本条 | [施工记录](#X-CI-RACE-STATICCHECK) |
| X-LOW-CLEANUP（agent 腿） | 核 `/api/health` 豁免注册并订正（两处豁免表都不再收它） | 2026-11-12 | 见本条 | [施工记录](#X-LOW-CLEANUP-agent) |
| AGENT-RUNBOOK-GAP | 本仓没有 runbook，起停与生成链失败只有 README 零散说明 | 2026-11-12 | `docs(agent): AGENT-RUNBOOK-GAP 补齐 runbook` | [施工记录](#AGENT-RUNBOOK-GAP) |
| AGENT-FACTOR-TOOLS-UNAVAILABLE | 三个因子工具打向 core 不存在的端点 | 2026-11-11 | 见本条 | [施工记录](#AGENT-FACTOR-TOOLS-UNAVAILABLE) |
| AGENT-HERMES-DOCS-ADR041 | Hermes 文档/技能仍把已不可用的因子工具写成现役 | 2026-11-12 | 见本条 | [施工记录](#AGENT-HERMES-DOCS-ADR041) |
| AGENT-EXPERIMENT-COUNT-SURFACE | 实验计数是内存 seq+1，回测/走查工具零实验行 | 2026-11-12 | 见本条 | [施工记录](#AGENT-EXPERIMENT-COUNT-SURFACE) |
| AGENT-INTERVENTION-ADOPTION-LEDGER | 无干预/采纳台账，校准与对账无数据基础 | 2026-11-12 | 见本条 | [施工记录](#AGENT-INTERVENTION-ADOPTION-LEDGER) |
| AGENT-HYPOTHESIS-FILTER | 因果维直通弱信号，假设过滤形同虚设 | 2026-11-12 | 见本条 | [施工记录](#AGENT-HYPOTHESIS-FILTER) |
| AGENT-LLM-BUDGET-GATE | ResearchBudget 零代码，日累计 USD 闸缺失（D4=a） | 2026-11-12 | 见本条 | [施工记录](#AGENT-LLM-BUDGET-GATE) |
| AGENT-EXPLORE-STATE-REAP | explore 运行态纯内存，崩溃即孤儿（D10=b） | 2026-11-12 | 见本条 | [施工记录](#AGENT-EXPLORE-STATE-REAP) |
| AGENT-CAUSAL-PARSE-TESTS | 因果叙述者解析逻辑无直测，验证器侧用假叙述者 | 2026-11-12 | 见本条 | [施工记录](#AGENT-CAUSAL-PARSE-TESTS) |
| AGENT-EXPLORE-INTERVENTIONS | UI 子台账 B-06 的三条干预写面（G2 指方向 / G3 追问 / G4 标记）缺端点 | 2026-11-12 | `76c98ee`、`9b4e381`、`4d27919`、`2f4b8a1` | [施工记录](#AGENT-EXPLORE-INTERVENTIONS) |
| AGENT-REVIEW-PENDING-SOURCE | core「待审阅 N」聚合面已定稿，agent 缺上游读面 `GET /api/explore/review/pending` | 2026-11-12 | 见本条 | [施工记录](#AGENT-REVIEW-PENDING-SOURCE) |
| AGENT-ADOPTION-DECISION-JSON-TAGS | `AdoptionDecision` 缺 json tag，`calibration.entries[]` 实际输出 PascalCase，与 api-schema 不一致 | 2026-11-12 | 见本条 | [施工记录](#AGENT-ADOPTION-DECISION-JSON-TAGS) |
| AGENT-PROBE-HEALTHZ | 增加 `/healthz` 并保留 `/health` 兼容别名 | 2026-11-13 | 见本条 | [原文与施工记录](#AGENT-PROBE-HEALTHZ) |
| AGENT-README-DEPENDENCY-WORDING | README 依赖口径澄清（无 Go 编译依赖 vs 运行期 HTTP 单向依赖） | 2026-11-13 | 见本条 | [原文与施工记录](#AGENT-README-DEPENDENCY-WORDING) |
| AGENT-CI-GATES | CI 起 PG + gofmt 门 + 覆盖率门（库侧 21 条 SKIP→PASS；实测 65.6% → 门 60%） | 2026-11-13 | 见本条 | [原文与施工记录](#AGENT-CI-GATES) |

---

## 原文归档（逐字）

# TASKS — tellurion-agent 未完成项

> 本仓（AI 实验员层）的任务台账。**六字段规范同 core 仓 `AGENTS.md` §8.5**（`tellurion` 仓）：目标 / 上下文 / 要求 / 约束与非目标 / 验收 / 边界，且须满足「独立 agent 拿着本单 + 它显式引用的冻结契约，不需读其他实现、不需问人，就能完成并自证」。
> 状态：`⬜ 待做` / `🔶 进行中` / `⛔ 阻塞` / `✅ 完成（附日期）`。
>
> **台账终扫（2026-11-10）**：任务标题带 `⬜` / `🔶` / `⛔` 的 = **0 条**；正文旧标记以标题与最新「收口」块为准。**无开放项**。
> 本文件自 core 台账 OPS-01 改判（2026-10-14）派生——原单前提（core 侧做 explore SSE）因 AI 拆仓失效，实现落此仓。

> **第三轮裁决指针（2026-10-25）**：⑩ AI 生成物载体 = **默认 `carrier="dsl"`**，仅 L3（自定义状态机 / L2 表达不了）才 `carrier="wasm"`，**`carrier="so"` 删除**；`artifact_hash` 用 `contracts.engine.StrategyVersionInput{Carrier:"dsl", Source:<配置文本>}.Hash()`，复用 QT-41 资产服务。正文见 `tellurion:docs/design/qt-decoupling/cross-repo-decision-record-2026-10-25.md` §五。

---


## 施工序（候选单，开工前必须展开成六字段）

> 这张表是**施工顺序**，不是任务台账：每条在执行前要展开成六字段单（目标/上下文/要求/约束/验收/边界），并写明冻结契约的路径与节号。
> 顺序有依赖，不要跳级。
> 2026-10-21：QT-17 已展开为本文件末尾的正式六字段条目（「QT-17 · agent 数据自持（D10：`ai.*` schema 建表与迁移）」），表内一行仅存施工顺序索引。

| 阶段 | 内容 | 上游单号 | 前置 |
|---|---|---|---|
| QT-17 | `ai.*` schema 自持 + 双写对拍 + 切读 + 停写 ✅ 完成（2026-10-25，①~⑤ + ④ 复核；正文见下） | `QT-17` | D24 补第 7 类动作 |
| S3 | 删 `replace ../tellurion`，改 HTTP 粗粒度 ✅ 完成（2026-10-25，S3-a~d） | `QT-07 / QT-50` | QT-17 + QT-50 |
| QT-65 | QT-11 跨仓回归（agent 腿）：pipeline / yaml 生产路径改**工厂注册** ✅ 完成（2026-10-24，见下方完成记录） | `QT-11` | 无硬前置 |
| QT-32-A | AI 生成链留痕 + promote 闸门的生产者侧 fail-closed ✅ 完成（2026-10-25，正文见下） | `QT-32` | QT-03 / QT-26 / QT-41b（均已 ✅） |
| QT-32-B | 生成物接资产服务（`PUT .../source` + `POST .../build`）拿真 `version_hash` / `artifact_hash` ✅ 完成（2026-11-10，正文见下） | `QT-32` | 载体已裁：2026-10-25 第三轮裁决 ⑩（本文件头部第 13 行）= 默认 `carrier="dsl"`、仅 L3 才 `carrier="wasm"`、`carrier="so"` 删除。**代裁指针，待主任追认** |

> **QT-65 证据（2026-10-23，C4 期间发现；2026-10-24 已闭环）**：core QT-11（`67bee39`）把注册表改存构造函数后，`strategy.GlobalRegister(实例)` 注册的共享条目在执行路径被 `Registry.NewInstance` fail-closed 拒绝（`ErrSharedStrategy`）。本仓两个**生产**调用点没跟上：`pkg/ai/pipeline/pipeline.go`（`registerOrConfigure` 的注册分支，ExecuteFromYAML 链路）与 `pkg/ai/yaml/loader.go`（`LoadAndRegister`）。现有测试当时全绿是因为没有用例在注册后真经引擎跑（latent，非当前红）。

> **S1 下游腿（contracts 四包切换）的 agent 侧落点**：import 全替清单见 [`archive/contracts-import-replacement-2026-10-23.md`](archive/contracts-import-replacement-2026-10-23.md)（A4 只列不改产出，**B4 已于 2026-10-23 执行完毕**；临时件已归档）。清单已实测把台账的「48+5+3」订正为 **46+5+3 = 54 位点 / 48 文件**，并写明 core 腿必须先落——否则两处 core/contracts 类型身份冲突会让构建红。


<a id="B4"></a>
## B4 · agent 侧 contracts import 全替 ✅ 完成（2026-10-23，已归档）

> 六字段正文与执行记录（54 位点 / 48 文件逐条替换、§6 五处文本落点、护栏退场、真退出码）在
> [`archive/contracts-import-replacement-2026-10-23.md`](archive/contracts-import-replacement-2026-10-23.md) §11。
> **对后续的直接后果**：本仓已依赖 `tellurion-contracts` 真实伪版本（无 `replace`）；`replace tellurion => ../tellurion` 仍保留，S3 才删；
> `tellurion-kit` 已 `go get` 但无代码引用，`go mod tidy` 已按 Go 规则移除，B5 之后若需要再按使用方实际引用引入。


<a id="OPS-01-A"></a>
## OPS-01-A · 观察页 run 状态 SSE 推送端点 ✅ 完成（2026-10-15）

> **收口记录（2026-10-15，实现 + 自证）**：新增 `cmd/ai/handlers_explore.go` 的 `Stream`（路由 `GET /explore/runs/:runID/stream`）+ 抽出可测的 `pumpStream(ctx, interval, snapshot, emit)`；把 `Status` 与 `Stream` 共用 `statusOf` 保证两条路 JSON 一字不差。硬约束守住：断线只停推、`Stream`/`pumpStream` 结构上拿不到 run 的 cancel，绝不取消探索；未动 core 限流器；轮询端点 `GET /runs/:runID` 行为不变。**验证**（本机无 Go SDK，在 `golang:1.25-alpine` 容器内跑，离线用宿主 module 缓存）：`gofmt -l` 空、`go build ./...` 过、`go vet ./cmd/ai/...` 过、`go test ./cmd/ai/... -run 'Explore|PumpStream' -race -count=1` **绿**（含全部既有 explore 测试回归）；新增测试 `handlers_explore_stream_test.go`：3 个 pumpStream 逻辑测（完成→progress+complete、running 每 tick progress、ctx 取消停在 progress 不发 complete）+ 2 个 HTTP 测（未知 run 404、已完成 run 真实 SSE 帧）。**破坏验证**：把 `emit("progress"` 改 `emit("process"` → HTTP 测当场红（帧变 `event:process` 不含 `event:progress`）→ 还原绿。注：gin v1.10 SSE 帧冒号后无空格（`event:progress`）。


- **① 目标**：ai-service 暴露 `GET /api/explore/runs/:runID/stream`（SSE），把一轮探索的实时状态推给客户端，使观察页从「每 2.5s 轮询 `GET /runs/:runID`」变成「开一条长连接」，消除 429 与真失败混淆的土壤。**core 侧无需改动**（ai-proxy 已 `FlushInterval=-1` 做 SSE 逐写直通）。
- **② 上下文**（本仓代码锚点，开工前逐一读）：
  - 现轮询端点 `Status`：`cmd/ai/handlers_explore.go:340`（路由 `g.GET("/runs/:runID", h.Status)`，注册于 `:164`）。它从 `run.mu` 保护的快照拼 `ExploreStatusResponse{RunID, Running: !run.done, Stopped, Attempts, BestSeq}`（类型定义 `:142`）。
  - 运行态来源：`Start`（`:203`）起一个**独立于请求生命周期**的 goroutine（`:220-222` 注释：用 `context.Background()` 派生 cancel，绝不能用 `c.Request.Context()`），异步经 `Config.OnAttempt`（`:250`）把每次尝试 append 进 `run.attempts`，跑完在 `defer` 里置 `run.done = true`（`:242`）。
  - **无 pub/sub**：`run`（`exploreRun` 结构 `:105`）只是被 poll 的内存快照——与 core 先例 data-service `sseProgressHandler`（core 仓 `cmd/data/sync_handlers.go:761`，每 2s 轮询 store 再转推 SSE）**同构**。本单复用该「轮询快照→SSE」形状，不另造订阅机制。
  - 冻结契约（照此对齐，勿改）：`loop.Attempt`（`pkg/ai/loop/loop.go:71`）、`loop.RunResult` / `RunResult.Stopped StopReason`（`pkg/ai/loop/loop.go:89`、`:94`）、`ExploreStatusResponse`（`handlers_explore.go:142`）。
  - 裁决/因果维落库在 `OnAttempt` / `reviewBestCausally` 里**持 `run.mu` 锁**读写同一 run（`:251`、`:331`）——SSE 侧读快照也必须持同一把锁，不得绕过。
  - 前端自救（`POLL_MS=2500` + 连续 6 次失败才认断线 + e2e 钉住）在 **ui 仓**，接线另立小单（跨仓两段式）。core 台账登记见 core `docs/TASKS.md` OPS-01 改判记录。
- **③ 要求**：
  1. `RegisterExploreRoutes`（`handlers_explore.go:160`）内新增 `g.GET("/runs/:runID/stream", h.Stream)`。
  2. **开流前先校验 run 存在**：不存在 → `httpserver.Fail(c, 404, "run not found")` 直接返回（沿用 `Status` 的 not-found 语义），不给一个「一连上就 error」的 EventSource。
  3. SSE 头与事件形状对齐 data-service 先例：`Content-Type: text/event-stream`、`Cache-Control: no-cache`、`X-Accel-Buffering: no`；用 gin `c.SSEvent(name, data)` 且每次 `c.Writer.Flush()`。事件名：`progress`（payload = 当下 `ExploreStatusResponse` 快照，**与轮询端点同 JSON 形状**，便于 ui 复用类型）、终止时 `complete`（最终快照）、客户端断线 `close`、取快照出错 `error`。
  4. 主循环（ticker，间隔复用先例语义即可，如 2s）：`select` 两路——`<-ctx.Done()`（`c.Request.Context()`，仅控**推送**）→ 发 `close` 返回；`<-tick` → 持 `run.mu` 取快照发 `progress`，若 `run.done` 则发 `complete` 后返回。
  5. **⚠️ 断线不叫停探索**：`Stream` 用的请求 ctx 取消只是停推，绝不能触碰 `run.cancel` / `run.ctrl`（叫停只走既有 `POST /runs/:runID/stop`）。
  6. **只增不改**：保留 `GET /runs/:runID` 轮询端点（其他调用方 + ui 回退路），`Status`/`Start`/`Stop` 的签名与 JSON 契约不变。
- **④ 约束与非目标**：不改 `Status`/`Start`/`Stop` 既有行为；**不动 core 限流器**（§八 逐字匹配裁决继续有效——本单靠「N 请求→1 长连接」省 429，不靠豁免路径）；不动 loop / pipeline 编排逻辑；不做前端接线（ui 另立）；不新增第三方依赖；**不引入多订阅 fan-out**（单观察者轮询快照足够，过度设计留给真有需求时）。
- **⑤ 验收**：
  1. 新增单测（`package main`，gin + `httptest` + testify，**禁 `time.Sleep` 同步**，用可控 fake / `require.Eventually`）：注入一条可控的 run（fake `loop.TryRunner` 或直接摆 `run.attempts`/`done`），断言 `GET /runs/:id/stream` 按序发出 ≥1 个 `event: progress`、终止时发 `event: complete` 后连接结束；`run` 不存在时返回 404（且未开流）。
  2. **破坏验证**：① 把终止判定取反 / 让 `done` 永不为真 → 断言 `complete` 的用例变红；② 改事件名（如 `progress`→`process`）→ 断言事件名的用例变红；③ 让 Stream 误用 `run.cancel`（断线即叫停探索）→ 断言「探索仍在跑」的用例变红；均还原绿。
  3. 回归：现有 `Status` 相关测试不退化；`go build ./...`、`go vet ./...`、`go test ./cmd/ai/... ./pkg/ai/loop/... -race` 全绿。
- **⑥ 边界**：只加 explore run 状态的 SSE 通道；copilot job / pipeline 等其他轮询端点另立。

---


<a id="OBS-01-R1-A"></a>
## OBS-01-R1-A · 偏差维输出一个它没量过的数字 ✅ 完成（2026-10-14；2026-10-16 自 core 台账 OBS-01-R1 迁入）

> **迁入说明（2026-10-16）**：本单实现整体在本仓（`pkg/ai/validation/bias.go` + `cmd/ai/handlers_explore.go`），core 侧原条目已移出、只留索引指针（core `docs/TASKS.md` §三）。以下六字段与完成记录逐字保留自 core 原条目，不改写。

- **① 目标**：验证器偏差维在池子规模未测量时**不打印数量**（「0 只」不再冒充实测值），屏幕上每个数字都可被当场质问。
- **② 上下文**：归档卷 §八工单表 #7（2026-10-14 立单，**代码在 tellurion-agent 仓**）：`cmd/ai/handlers_explore.go:50` 设 `PoolSource=PoolSourceCurrent` 但从不设 `PoolSize`（只有 `:96` 预热成功路径才填）⇒ `pkg/ai/validation/bias.go:296-303` 无条件把 `in.PoolSize`（Go 零值 0）插进 blocking 文案 ⇒「股票池按今天的上市名单取（**0 只**）」。**0 是零值不是测量结果**（愿景原则 4：每个数字可被当场质问）。`PoolSize<=0` 的「未测量」分支已有先例（`bias.go:202` 与 `:314`），只有 `PoolSourceCurrent` 这支漏了。**概率侧没有 bug**：`biasProbability` 对 `PoolSourceCurrent` 提前返回 `BiasProbFloor`，低分有意为之，错的只有那句话。
- **③ 要求**：`PoolSize<=0` 时不带数量措辞（对齐 `bias.go:202`/`:314` 既有「未测量」分支先例）；文案改动最小化。
- **④ 约束与非目标**：**不改 `biasProbability` 与 `PoolSourceCurrent` 的概率**；不借机重构偏差维；不动其他维。
- **⑤ 验收**：单测：`PoolSize=0`（未测量）输出无数量词、`PoolSize>0` 输出带真实数量；**破坏验证**：改回无条件打印 → 新测试红，还原绿。正常 run 输出文案回归不变（有测量值场景）。
- **⑥ 边界**：只改这一处措辞分支。
- **✅ 完成记录（2026-10-14）**：**代码修复由并发 agent 落地**（tellurion-agent 仓 `f432074`）：`biasChallenges` 幸存者 switch 新增 `PoolSource == PoolSourceCurrent && PoolSize <= 0` 前置分支，文案改「（池子规模未测量）」，对齐 `bias.go:202`（概率侧未评估返回 0）与 `bias.go:323-328`（文案侧「幸存者偏差未评估」note）既有先例（任务原文引 `:314` 系笔误，该行实为「池子不小却无退市股」的带数量 warning 分支）；severity 保持 blocking，`biasSurvivorship` 对 `PoolSourceCurrent` 仍返回 `BiasProbFloor`——④「概率不动」达成，③「文案改动最小化」达成（+9 行）。**两腿单测**：`TestBias_CurrentPoolWithoutSizeSaysUnmeasured`（零值输出含「未测量」且不含「0 只」）+ `TestBias_CurrentPoolWithSizePrintsCount`（PoolSize=300 带真实数量，⑤ 回归场景）。**本单收口复验（独立重做，不轻信原提交声称）**：`go test ./pkg/ai/validation/ -count=1` 全绿（42.4s）、gofmt/vet 干净；**破坏验证**：把该分支改回原始 bug 形态（无条件 `fmt.Sprintf` 打印零值）→ `TestBias_CurrentPoolWithoutSizeSaysUnmeasured` 精确变红 → 还原 → 复绿，`git diff` 零残留；另做过一次「只拆条件不改文案」的变体验证（红在 WithSize 腿，亦还原），两变体共同证明护栏非空转。**接线现状核对**（`cmd/ai/handlers_explore.go`）：静态 `exploreBias`（:40-53）不带 `PoolSize` = 未测量路径，文案不再冒充实测值；`biasInput()`（:86-103）在引擎预热到上市日历时填 `PoolSize=len(windows)` = 测量路径带真实数量。**提交方式说明**：本标记在 D3 批次（core `5225df5`）提交后以 pathspec 单文件提交，未触碰并发 agent 在途的 7 个无关脏文件。

---


<a id="OBS-01-R2-A"></a>
## OBS-01-R2-A · AI 客户端把 K9 三值丢在边界上 ✅ 完成（2026-10-22）

> **来源与裁决**：本单自 core 仓 `docs/TASKS.md` 的 `OBS-01-R2` 改判迁出（core 侧已关闭、保留可追溯指针，见其改判记录与 §三 跨仓指针区）。core 侧裁决 **(b)**：`RunBacktest` 返回带 `Status`/`Error` 的小结构，**不给 core `domain.BacktestResult` 加 `Status`**——core `pkg/backtest/contracts/contracts.go:247-264`「ResultSpec 冻结（K9）」明写 `status` 是调用方唯一判据、三值属**响应层**，下沉进 domain 指标集合会造成同一概念两处真相。**core 零改动**，故本单非跨仓原子提交。

- **① 目标**：AI 侧消费者（Copilot 播报、实验日志、`pkg/ai/loop` 候选评估）能读到 `completed` / `invalid` / `failed` 三值状态，不再只靠 `InvalidReasons` 反推。
- **② 上下文**（本仓 + core 冻结契约锚点，开工前逐一读）：
  - 缺陷点：`pkg/ai/client/backtest_client.go:47` 的 `RunBacktest` 返回 `(*domain.BacktestResult, error)`，而响应类型 `BacktestResponse`（`:39`）已解出 `Status string json:"status"`（`:43`）——`btResp.Status` 在 `:78` 被丢掉。`GetBacktestResult`（`:82`）同病（`:103` 解出、返回只 `*domain.BacktestResult`）。
  - 冻结契约（core 仓 `tellurion`，照此对齐、勿改、勿加第四值）：`pkg/backtest/contracts/contracts.go:247-264` 的 `StatusCompleted` / `StatusInvalid`（InvalidReasons 必填、Error 必空）/ `StatusFailed`（Error 必填、InvalidReasons 必空）；护栏 `resultspec_test.go`。
  - core `pkg/domain/backtest.go:5-39` 的 `BacktestResult` **没有也不该有** `Status`（裁决 (a) 已排除）；`InvalidReasons`（`:38`）由引擎挂，客户端只读不重算。
  - `RunBacktest` 的 3 处调用点：`pkg/ai/agents/validate.go:345`、`pkg/ai/pipeline/strategy.go:99`、`pkg/ai/agents/validate_l4.go:104`（后者经 `httpBacktestRunner` 转 `BacktestMetrics`）。
  - 前端不动：ui 仓 A-07 已按三值接住并自救，7 条单测钉住；本单登记的是**契约残缺**，不是用户可见假成功。
- **③ 要求**：
  1. `RunBacktest` 返回改为小结构（如 `BacktestOutcome{Result *domain.BacktestResult; Status string; Error string}`），三值**原样透传**，不新增取值。
  2. `GetBacktestResult` 同口径改。
  3. 3 处调用点跟进解包；`invalid` 时 `Result.InvalidReasons` 非空（读引擎挂的 Code，不在客户端重算）。
  4. core 仓零改动；不改 `domain.BacktestResult`。
- **④ 约束与非目标**：不改前端；不改 K9 三值语义与字段集；不借机重构 pipeline 结果结构（`BacktestMetrics` 只做最小解包适配）；不加新依赖；不动 HTTP 请求/超时行为。
- **⑤ 验收**：
  1. 单测（`httptest` 假服务端，禁 `time.Sleep` 同步）：同一 `RunBacktest` 对三份响应（`completed` / `invalid`+`invalid_reasons` / `failed`+`error`）分别给出可区分的判读——invalid 时 `InvalidReasons` 非空、failed 时 `Error` 非空且不被误当成功。
  2. 真请求核对：对活 core `POST /api/backtest`，断言线上 JSON `status` 与解出的结构体字段一致。
  3. **破坏验证**：把 `Status` 重新丢掉（只 `return btResp.Result, nil`）→ 三值区分用例当场红 → 还原绿。
  4. `go build ./...`、`go vet ./...`、`gofmt -l`（本单文件）干净；`go test ./pkg/ai/client/... ./pkg/ai/agents/... ./pkg/ai/pipeline/... -race -count=1` 全绿。
- **⑥ 边界**：只补三值透传与调用点解包；前端接线、pipeline 结构重构、`BacktestMetrics` 口径变更另立。

> **✅ 完成记录（2026-10-22）**
>
> **交付**：`pkg/ai/client/backtest_client.go` 新增 `BacktestOutcome{Result,Status,Error,JobID}` + `StatusCompleted/Invalid/Failed`
> 与 `JobStatusPending/Running` 取值常量 + `Succeeded()`/`InvalidReasons()` 两个判据 + `classify()` 一条判读路径（两个方法共用，避免两处判据漂移）。
> `RunBacktest`/`GetBacktestResult` 均改回 `(*BacktestOutcome, error)`；三处调用点跟进解包：
> `agents/validate.go`（L2：非 completed → 记 warning + 分数不加成）、`pipeline/strategy.go`（**invalid 不再进基因池**：以前 `err == nil` 就写 `Status:"validated"`）、
> `agents/validate_l4.go`（`httpBacktestRunner`：invalid → 报错中止本段 walk-forward，不交「成功 + 零指标」）。**core 仓零改动**，`domain.BacktestResult` 未碰（③-4 达成）。
>
> **⑤ 验收逐条（真退出码）**
> 1. 单测：`pkg/ai/client/backtest_client_test.go` 13 条（主判据 `TestRunBacktest_ThreeStatusesAreDistinguishable` 三份响应三条子腿 +
>    `InvalidReasons` 只读不重算 + failed 带出原话且 outcome 非 nil + 空 status / 第四值 `done` fail-closed +
>    pending 不再 `(nil,nil)` + `completed` 无 `result` 不算成功 + 4xx 无 outcome + GET 同口径 + running 点名 status + nil receiver 安全 +
>    夹具报文自我核对），`go test ./pkg/ai/client/ -count=1` → **ok**。
>    调用点侧另加 4 条 `pkg/ai/agents/obs01r2a_three_value_test.go`（真客户端打 `httptest` 假 core，验 L4 那条通道的三种判读），`go test ./pkg/ai/agents/ -count=1` → **ok**。
> 2. **真请求核对：未执行**。本机 `curl http://localhost:8085/api/health` → `000`（core 未起）；且见下面偏差 #2 ——
>    这条腿在活 core 上今天**必然红**，红因是请求 DTO 漂移，不是三值。不假装跑过。
> 3. **破坏验证（现场，真退出码）**：把 `classify` 上游的 `Status: r.Status` 改成 `Status: ""`（即「把三值重新丢在边界上」）→
>    `go test ./pkg/ai/client/` 退出码 **1**，**8 条用例红**（三条子腿各报 `status 必须原样透传：want "completed" got ""`，
>    另有 InvalidReasons/Failed/CompletedWithoutResult/SameThreeValueReading/RunningNamesTheStatus）→ 还原 → 退出码 **0**，`grep -c 破坏验证腿` = 0（无残留）。
>       首次尝试时改坏脚本因 `Path.read_text(newline=...)` 抛异常而**没真的改到代码**，那次 `BROKEN_TEST_EXIT=0` 是无效证据，已重做（见 `feedback-test-verdicts-need-exit-codes` 同一类教训：动作没发生，退出码就没有意义）。
> 4. `go build ./...` + `go vet ./...` 干净；`gofmt -l` 干净；`go test ./pkg/ai/client/... ./pkg/ai/agents/... ./pkg/ai/pipeline/... -count=1` 全绿（pipeline 4.2s）。
>    ⚠️ **`-race` 这条腿未执行**：本机无 C 编译器（`CGO_ENABLED=1` → `cgo: C compiler "gcc" not found`），`-race` 直接 build failed。
>    新增用例全是单 goroutine + `httptest`（无共享状态、无 `time.Sleep`），代价登记在此：并发面（SSE / 端点）仍按 AGENTS §2 的口径要求带 `-race`，本单没碰并发。
>
> **实测偏差（回源码核对，四条是 REV-01-A 的具体输入）**
> 1. **同步 POST 是平铺响应，客户端 DTO 期待嵌套 `result`**：core `cmd/analysis/handlers_backtest.go:151` `c.JSON(http.StatusOK, result)` 里的
>    `result` 是 `*contracts.BacktestResponse`（`pkg/backtest/contracts/contracts.go:211` 起，`status`/`error` 与 `total_return`/`sharpe_ratio`/`portfolio_values` **同层平铺**），
>    没有 `result` 这个键 ⇒ 200 时 `btResp.Result` **恒为 nil**。旧代码 `return btResp.Result, nil` 于是交出 `(nil, nil)`，
>    `pipeline/strategy.go:112` 的 `btResult.TotalReturn` 直接 nil-deref。**这就是「三值被丢掉」的真实后果链**，也是本单为什么把 `Succeeded()` 定义成
>    `status == completed && Result != nil`：判据不能只信一个字段。按契约把平铺体映射进 Result 属 REV-01-A ③-1（禁手写漂移 DTO），不在本单。
> 2. **请求字段名漂移，POST 走不到 200**：客户端发 `strategy_name`（`BacktestRequest` `json:"strategy_name"`），
>    core 认 `contracts.BacktestRequest.Strategy` 的 `json:"strategy" binding:"required"`（`handlers_backtest.go:172`），
>    两条分支都不命中 ⇒ 落到 `:157` 的 400「must provide strategy+stock_pool (old format) or strategy_id+universe (new format)」，
>    且 `StrategyName: result.Formula`（把公式串当策略名）说明这条链的意图与端点语义也对不上。④ 明令「不动 HTTP 请求/超时行为」，本单**没改请求形状**；
>    这条属 REV-01-A ③-1，且必须先于任何「AI 真跑一次回测」的验收。
> 3. **GET 那条路是 job 表，status 取值不止三值**：`GET /api/backtest/:id` → `c.JSON(200, job)`（`pkg/backtest/job/job.go:209` 的 `Job`，
>    `status` ∈ `pending|running|completed|failed`，标识字段是 `id` 不是 `job_id`，`result` 是 `json.RawMessage`）。
>    `invalid` 不来自 job 表，而来自内存态 / `/:id/report` 的 `buildResponseFromState`（`handlers_backtest.go:176-180` 注记）。
>    ⇒ 本单把 `pending/running` 与「三值之外」分开处理并都 fail-closed，`outcome.JobID` 由请求方问的那个 id 回填。
> 4. **既有 OBS-01 那条判读没动**：`pipeline/obs01_invalid_run_test.go` 测的 `Pipeline.Execute` 走 `BacktestRunner` 接口（进程内 mock），
>    它**拿不到 status**，只能继续从 `InvalidReasons` 反推 —— 与本单是两个通道，不冲突也不重复。把三值送进那条链接口要动 `BacktestRunner`，属 ⑥ 边界之外。
>
> **判据口径（本单做的技术决定，代价如实）**：`failed` 仍返回 `error`（既有三个调用点的中止语义逐字节不变，且「不把 failed 当成功」由它保证）；
> `invalid` 返回 `nil error` + 三值（它是结果属性不是故障，K9 明写「无效不进 Error」）；
> **三值之外的一切取值（含空串、`done`、`pending`）一律报错** —— 代价：core 若日后新增第四值，本仓会当场红而不是静默当成功，
> 这正是 K9「不把没记的说成记了」要的形态，加值应走 contracts 的变更评审而不是在本仓补一个 `case`。

---


<a id="REV-01-A"></a>
## REV-01-A · 消费侧对齐 refactor 修复传播（wire 契约 + 三值口径）✅ 完成（2026-10-22，已归档）

> 六字段正文与结案记录（③ 逐条落点、⑤ 验收真退出码、独立 worktree 上的两条破坏腿与逐字节还原凭据、六条实测发现）在
> [`archive/TASKS-2026-10-22-archived.md`](archive/TASKS-2026-10-22-archived.md)。本文件按规则只留未完成项，这里只留指针。
> **对后续的直接后果**：① core 的 `openapi.yaml` 仍漏登记 `error` / `invalid_reasons` / `universe_max_size`，客户端用缺口 allowlist 顶着，
> 补登记后 `TestOpenAPI_GapsAreStillGaps` 会红并提醒删行；② GET 的 job 行 status 恒 `completed`，客户端已改为「结果自带 status 优先」，
> core 侧是否让 job 表反映 invalid 归 QT-15/QT-10；③ UI-02 的展示口径可直接对齐本单的 `StatusInvalid` + `invalid_reasons` 原样透传。

---

<a id="SVC-AGENT-G0"></a>
### SVC-AGENT-G0 · 服务就绪门 G0：自有 config 与部署（ADR-036）[P0] ✅（2026-10-26）

- **① 目标**：本仓作为 AI 应用服务，自带 config 与 deploy，不再借 core 的配置目录。
- **② 上下文**：本仓已有 `Dockerfile` 与 `cmd/ai`，但没有 `config/`；compose 通过 `CONFIG_PATH=../tellurion/config/ai-service.yaml` 借用 core 配置。
- **③ 要求**：新增本仓 `config/ai-service.yaml`（模型/预算/端口等，密钥走 env）；新增 `deploy/compose.yaml`（`:8086`，`CONFIG_PATH` 指向本仓）；README 写单仓启动命令；健康检查用现有端点或 G1 补 `/healthz`。
- **④ 约束与非目标**：不改 AI 逻辑与 `ai.*` schema；不重新 import core；不引入 K8s。
- **⑤ 验收**：`docker compose -f deploy/compose.yaml config` 通过；单仓启动不依赖 `../tellurion/config`；无 `replace` / `go.work`。
- **⑥ 边界**：只改本仓 config/deploy；core 代理面不变。

> **施工记录（2026-10-26）**：
> - **落点**：`config/ai-service.yaml`（core 同名文件逐字节复制，SHA256 `F3DF46D4…C2C51B2`；core 未改）；`deploy/wait-for-deps.sh`（core 同名脚本逐字节复制，SHA256 `18EF9DAA…D39DA88`）；`deploy/compose.yaml`（自带 `postgres` / `redis` + `ai-service`，`:8086`、`CONFIG_PATH=/app/config/ai-service.yaml`、PG/Redis 走服务名，变量沿用 `DATABASE_*` / `REDIS_URL` / `AUTH_*` / `JWT_SECRET` 等既有 env）；`Dockerfile` 删三处 `--from=core`，改为本仓 `COPY config/ai-service.yaml` 与 `COPY deploy/wait-for-deps.sh`，补 `netcat-openbsd` 供等待脚本使用，`CMD` 不变；README 补目录、启动命令与单仓镜像构建。
> - **验收（真实输出）**：`grep -c 'from=core' Dockerfile` = 0；`grep -c 'COPY config/ai-service.yaml' Dockerfile` = 1；`docker compose -f deploy/compose.yaml config -q` → exit 0；`config --services` → `postgres` / `redis` / `ai-service`；`go build ./...` / `go vet ./...` / `go test ./... -count=1` 全绿（21 个包，`pkg/ai/pipeline 4.205s`）；`gofmt -l .` 空。
> - **破坏验证**：把 `COPY config/ai-service.yaml` 改回 `COPY --from=core config/ai-service.yaml` → `grep -c 'from=core'` = 1（红）→ 逐字节还原后 0（绿，SHA256 一致）。
> - **如实登记（2026-10-26 复测订正）**：本机 Docker daemon **本轮可达**，上一版「daemon 不可达、镜像实构建未取证」的陈述已失效。实测：`docker build -t tellurion-agent:g0-verify .` 成功（构建上下文只有本仓，无 `core` 命名上下文，即「删掉 core 源码也能构建」已取证）；镜像配临时 PG / Redis 起容器后 `curl -sS http://127.0.0.1:8086/health` → `200 {"service":"ai-service","status":"ok"}`，日志显示监听 `0.0.0.0:8086`。**未做**：core `docker-compose.yml` 的全栈 `up` 健康检查腿（core 侧已同步：删 `additional_contexts`、`entrypoint` 改 `/app/deploy/wait-for-deps.sh`，2026-10-26 `docker compose build ai-service` 成功、镜像内两文件在位）；`SVC-CORE-AI-RETIRE` 已于 2026-11-10 由 core 执行完毕（见本节）。
> - **施工记录（2026-11-10，本机环境收口：空口令）**：`database.password` 从「只靠 viper AutomaticEnv 的隐式映射」改为 `cmd/ai/lifecycle.go` 显式 `BindEnv("database.password", "DATABASE_PASSWORD")`（口令只从 env 注入，YAML 恒空）；`config/ai-service.yaml` 的 `database` 块补注释（恒空是 fail-closed，不是漏填，并写清本机注入什么）；README 启动示例改占位符，不再写死口令。**真进程验证**（宿主 PG 16.15 `127.0.0.1:5432`，`scram-sha-256`）：只给 `DATABASE_PASSWORD` + `JWT_SECRET`，`ai-service` 起在 `127.0.0.1:18086` → `/health` 200、`/metrics` 正常、`ai.*` 8 张表在位。**启动建表腿**：在全新空库 `tellurion_verify_1110` 上启动 → 本仓自动建 `ai.*` 8 张表（取证后已 DROP）。**破坏验证**：不注入 `DATABASE_PASSWORD` → exit 1 且日志为 `database password is empty: set DATABASE_PASSWORD — a ${...} placeholder in config does NOT work (there is no env expander)`（点名 env，不是连接期的认证失败）。同轮 core `analysis-service` 真进程验证发现两处启动阻断式配置缺口（risk resolve 端口沿用旧默认 `8086`、`BROKER_*` 三件必填），已在 core 仓 README 与 `config/analysis-service.yaml` 登记。

<a id="SVC-CORE-AI-RETIRE"></a>
### SVC-CORE-AI-RETIRE · core 侧 AI 遗留表与写方法退役（ADR-036 G3）[P1] ✅（2026-11-10，core 执行）

> **正文已迁 core（代裁，待主任追认）**：删除对象全在 core，按 core 台账原注的第一个选项，六字段正文与施工记录迁入 `tellurion:docs/TASKS.md` §六之二 **串行第 10**；本仓只留指针 + 本仓停写验收，不再双写。
> **结果（core 当场实测）**：① 写点 **6 → 0**；② `SeedBuiltinFactorHypotheses` **3 → 0**；③ 七表 `CREATE TABLE` **7 → 0**；`pkg/storage/{experiments,factor_hypothesis,research}.go` 已删。delete 对象里唯一的本仓腿是反向护栏与停写取证。
> **本仓腿（2026-11-10）**：`pkg/aistore/schema_core_drift_test.go` 的 `TestSchema_ColumnsMatchCore` 翻成 `TestSchema_RetiredCoreTablesAreGone`（core 里七表零 `CREATE TABLE`；正向对照要求 core 仍至少有一张建表）；`cmd/ai/experiment_stopwrite_integration_test.go` 先 `to_regclass('public.experiments')`，表已不存在时照跑 `ai.*` 全链路并把 legacy 冻结段记为「无对象」，存量环境仍逐行冻结。破坏验证：往 core `postgres.go` 加回一行 `CREATE TABLE IF NOT EXISTS factor_genes (...)` ⇒ 本仓护栏红，还原后 SHA256 一致复绿。门禁：`gofmt`/`go build ./...`/`go vet ./...` 全 0；`go test ./... -count=1` 除 `pkg/ai/client` 两条**前置既有**红外全绿 —— `TestClientEndpoints_AreRegisteredInCoreOpenAPI` / `TestOpenAPI_ReferenceDataShapesMatchContract` 要求 core openapi 登记 `/api/v1/trading-calendar`、`/api/v1/listing-windows`，而 core 已在 `SVC-CORE-DATA-RETIRE`（2026-11-08）把这两条端点随数据面退役（`cmd/analysis/agent_actions.go:78-80` 留有退役注记）。与本单（AI 实验域退役）无交集，未在本轮改动；需另立 agent 单决定「客户端改道数据服务 / 登记进 `clientEndpointGaps`」并回写 core openapi 口径。

- **① 目标**：core 不再写 `experiments` / `factor_hypothesis` / `factor_genes` / `strategy_genes`；遗留写方法与 DDL 删除或标只读。
- **② 上下文**：本仓 QT-17 已停写 `ai.experiments`；core `pkg/storage/experiments.go:108`、`factor_hypothesis.go:35` 仍有 `INSERT`，后者生产零调用方，core `postgres.go` 仍建裸表。
- **③ 要求**：core 删除/停用上述写方法；确认无生产调用；裸表标 `legacy` 或按迁移 DROP；`ai.*` 为本仓唯一写者。
- **④ 约束与非目标**：不删除历史数据；不在本仓改 core 代码；不扩大到 `quant.*` 运行态表。
- **⑤ 验收**：core 侧这四张表零写入；`grep` 写方法零生产调用；本仓 `ai.*` 行数持续增长而旧表不增长。**以上均已由 core 侧验收并回写；本仓腿见上。**
- **⑥ 边界**：跨仓任务，正文归 `tellurion:docs/TASKS.md` §六之二 串行第 10（原指向 `SVC-RETIRE-INDEX`，2026-11-10 迁单）；本仓只留指针与本仓停写验收。


<a id="DBT-18"></a>
## DBT-18 · mining 的真实候选生成器（LLM / MCTS / 基因池 → `ai.*`）[P3] ✅（归属已裁 = 本仓；2026-11-07 已落；2026-11-10 `-race` 腿经 Docker + CGO 实跑通过）

> **立单依据（把押下的那半补齐）**：core 侧 DBT-18 已 ✅，但它的 ⑤ 验收原文是「归属结论成文 + 指针登记；**跨仓那半在其归属仓台账里写全六字段**」，而裁决 ⑧ 在本仓「QT-17 前置核对」那段（条目 7，**按锚点找：全文搜 `DBT-18（mining 的真实 Proposer）归属本仓`**）写的「本仓的动作：**立自己的六字段单**（含 LLM/MCTS Proposer 的实现位置）」此前**没有条目**（实测本文件只有那条裁决指针，无 `DBT-18` 单）。本单就是那半。裁决全文见 `tellurion:docs/design/qt-decoupling/cross-repo-decision-record-2026-10-25.md` ⑧。

- **① 目标**：候选生成从「读 JSON 文件的 `fileProposer`」升级为本仓的真实生成器（LLM 提议 / MCTS / 基因池复用），**落库只落本仓 `ai.*`**，不再依赖 core 的 mining 落库腿。
- **② 上下文**：core 侧接口是冻结契约 `tellurion:pkg/mining/pipeline.go:33`（`Proposer.Propose(ctx) ([]Candidate, error)`）与 `pkg/mining/mining.go:74`（`Candidate` 形状）——**本仓不得 import 它**（AGENTS §3 禁 import core 实现包），所以「实现同一个接口」不是选项，正确形态是**本仓自己的候选类型 + HTTP 投递或本仓自持流水线**；现状：本仓已有 `pkg/ai/loop/loop.go:35` 的 `Proposer`（控制器用的参数提议者，`cmd/ai/handlers_explore.go:263` 只接了 `NewTPEProposer`，seed 取 `time.Now().UnixNano()`）、`pkg/ai/search/{tpe,genetic}.go`、`pkg/ai/gene_pool/mutation.go`，落库面 `pkg/aistore` 已持 `ai.factor_genes` / `ai.strategy_genes` / `ai.generation_chain` / `ai.experiments`；core 侧 `cmd/data mining_cmd.go` 的 mining 落库腿在 `tellurion:SVC-CORE-AI-RETIRE` 里**要被删**（材料 §4.7）。
- **③ 要求**：① 先给**形状归属结论**（三选一并写进 `docs/ARCHITECTURE.md` + 一条本仓 ADR）：(a) 候选只在 agent 侧生成并落 `ai.factor_genes`，core 的 mining pipeline 退役（与 `SVC-CORE-AI-RETIRE` 同向，**建议**）；(b) core 保留 pipeline、新增一条候选投递 HTTP 端点（则端点与鉴权归 core 侧另单，本仓只写客户端）；(c) 沿用 JSON 文件投递（则本单降级为「生成器改善候选质量」，落库不变）；② 按结论落实现：新增 `MCTS` 或 `LLM` 候选提议器接进 `pkg/ai/loop` 的 `Proposer`（与 TPE 同一接口，可切换、可对比）；③ **确定性口径**：LLM/MCTS 都必须带可复现 seed 或留痕（`ai.experiments` 已有三 hash 坐标；seed 不许默认取墙上时钟——`handlers_explore.go:263` 今天就是 `time.Now().UnixNano()`，本单要么给它显式 seed，要么写明为什么可接受）；④ 候选落库走 `pkg/aistore` 的 `ai.*`，**不直连 core 业务表**；⑤ LLM 成本与预算沿用 `pkg/ai/cost.go` 的预算器，不许绕过。
- **④ 约束与非目标**：**不 import core 实现包**、**不直连 core 的业务表**（AGENTS §3 两条硬约束）；不新增仓外 `replace`；不在本仓实现「第二个编排者」（ADR-023 禁，core 侧 DBT-18 ④ 原文同判）；不改 core 的 `mining.Pipeline` 签名（那是 core 的冻结契约）；不在本单删 core 侧任何 mining 代码（那是 `SVC-CORE-AI-RETIRE`，串行第 10）；不改 `pkg/ai/validation` 的六维口径。
- **⑤ 验收**：① 归属结论成文（ADR 或 ARCHITECTURE 一节 + 三选一的代价写清），且与 `tellurion:SVC-CORE-AI-RETIRE` 的方向**不矛盾**（矛盾即报协调者，不许两仓各按各的写）；② 新提议器至少两条用例：同 seed 两次 `Propose` 产出可复现（若选 (a)/(c)），以及与 TPE 的同接口替换可用；③ 一条反证：候选生成失败 ⇒ 流水线**不落半成品行**（断言行数为 0，不是「有行但为空」）；④ `ai.*` 落库有测试库腿（无库如实 skip）；⑤ `go build/vet/test ./...` 三条绿 + `go test ./cmd/ai/... -race -count=1`（本仓 AGENTS §2 要求端点/SSE 面带 `-race`）。
- **⑥ 边界**：本仓的候选生成与 `ai.*` 落库；core 侧 mining 落库腿的删除、`SeedBuiltinFactorHypotheses` 的 seed 腿、7 张遗留表的退役全部归 `tellurion:SVC-CORE-AI-RETIRE`（2026-11-10 已由 core 执行完毕，正文见 core `docs/TASKS.md` §六之二 串行第 10）。

> **🔶 施工记录（2026-11-07，本单）**
>
> **① 归属结论**：选 **(a)** —— agent 侧生成候选、落 `ai.factor_genes`，core mining 随 `SVC-CORE-AI-RETIRE` 退役。成文落点：`docs/ARCHITECTURE.md` §2.1（含三选一代价表）+ `docs/adr/0003-候选生成归本仓-core-mining-退役.md`（并登记进 ADR 索引与 ARCHITECTURE §6）。与 `SVC-CORE-AI-RETIRE` 同向，无矛盾。
>
> **② 新提议器**：`pkg/ai/search/mcts.go`（确定性 UCT 树搜索：参数切桶 + 历史 reward + 最近邻 rollout 代理）与 `pkg/ai/loop/mcts_proposer.go`（`MCTSProposer`，与 `TPEProposer` 同 `loop.Proposer` 接口）。`cmd/ai/handlers_explore.go` 的 `StartExploreRequest.Proposer` 支持 `tpe|mcts`，未知值返回 400；编排层未改。
>
> **③ 确定性口径**：`StartExploreRequest.Seed *int64` 显式优先；缺省退回 `time.Now().UnixNano()`，但实际 seed 写进启动响应、状态响应、日志，并写进每次尝试的 `Hypothesis`（随 `ai.experiments.hypothesis` 落库）。`TPE(seed=…)` / `MCTS(seed=…)` 都可回放。用例：`TestMCTSProposer_SameSeedReplays`（同 seed 两个实例、8 次建议逐字段相等）、`TestExploreHandler_SeedReplaysMCTSSequence`（端点级两次 run 参数与 hypothesis 相等）。
>
> **④ 落库**：`pkg/ai/mining/generator.go` 复用 `gene_pool.Mutator` 生成 `aistore.FactorCandidate`；`pkg/aistore/factor_candidate.go` 的 `SaveFactorCandidates` 先整批校验、再单事务写 `ai.factor_genes`（按 id upsert）。不直连 core 业务表。
>
> **⑤ 反证（真实输出）**：`TestGenerateAndSave_GenerationFailureWritesNothing` —— 父公式 `["abs(close)", ""]` 在第 2 个候选中失败，`GenerateAndSave` 在落库前返回；断言 `sink.rows == 0`。真库腿 `TestSaveFactorCandidates_Integration` 另验非法批次不留前一行、同 id 重写为更新。
>
> **破坏验证（逐字节还原 SHA256 一致）**：
> - 把 `NewMCTSProposer` 的 seed 改成「墙上时钟 + 原子计数器」（模拟非确定 seed）→ `TestMCTSProposer_SameSeedReplays` 红（第 0 次建议不一致：lookback 18 vs 32）→ 还原后 `F263EE53…2EEE0492` 与改前一致 → 复绿。
> - 删掉 `GenerateAndSave` 的 `if err != nil { return nil, err }` → 反证腿红（`expected 0, actual 1`，消息「候选生成失败时不许落下任何一行」）→ 还原后 `4A138ECC…2E5F69FF` 与改前一致 → 复绿。
>
> **门禁（真实退出码）**：`gofmt -l .` 无输出（EXIT=0；顺带移除 `pkg/ai/drift/detector.go`、`pkg/ai/validation/redundancy.go` 两个**既有** BOM，仅 BOM，无逻辑改动）；`go build ./...` EXIT=0；`go vet ./...` EXIT=0；`go test ./... -count=1` EXIT=0（cmd/ai 0.563s、pkg/ai/mining 0.293s、pkg/aistore 0.075s）。
>
> **未执行 / 如实登记**：
> - `go test ./cmd/ai/... -race -count=1`：**2026-11-10 已补跑**——本机无 C 编译器（`CGO_ENABLED=0`），改用 Docker 官方 `golang:1.25` 镜像（CGO 可用）挂载本仓：`docker run --rm -v <agent>:/src -w /src -e CGO_ENABLED=1 golang:1.25 sh -c "go test ./cmd/ai/... -race -count=1"` → **`ok tellurion-agent/cmd/ai 1.211s`，RACE_EXIT=0**（真实 race 检测，非 skip；未改任何代码）。
> - `TestSaveFactorCandidates_Integration`：**2026-11-10 已补真库腿**——系统 PG（`:5432`）不可用（本会话早前一次 `Set-Content -Encoding utf8` 给宿主 `pg_hba.conf` 写了 BOM，服务端拒绝加载；文件待授权按字节还原），改用 `initdb` 起的独立实例 `127.0.0.1:5433`（库 `tellurion`，trust）；`TEST_DATABASE_DSN=postgres://postgres:postgres@127.0.0.1:5433/tellurion?sslmode=disable go test ./pkg/aistore/ -count=1` → **绿**（`TestSaveFactorCandidates_Integration` / `TestMigrate_*` / `TestStore_*` 均真跑，`-v` 逐条确认无 SKIP）。
> - LLM 提议器未实现（本单选 MCTS + 基因池变异）；将来接入时必须走 `pkg/ai/cost.go` 的 `CostTable`，已在 ADR-0003 与 ARCHITECTURE §2.1 写明。
> - core 侧 mining 落库腿 / DDL 的删除不在本仓，仍归 `SVC-CORE-AI-RETIRE`。


<a id="QT-17"></a>
## QT-17 · agent 数据自持（D10：`ai.*` schema 建表与迁移）✅（2026-10-25）

> **迁入说明（2026-10-21）**：本单自 core 仓 `tellurion:docs/TASKS.md`「S2~S3 · 拆仓」迁入（任务台账批量重分布：实现整体落本仓的单迁入本仓台账）。六字段正文逐字保留、不改写；core 侧同号条目已改为指针。

- **① 目标**：agent 不再通过 core 的 `*PostgresStore` 读写实验域数据；`ai.*` 的 DDL 与 migrate 由 agent 仓持有——**这是 S3 删 replace 的数据前置**（storage 依赖没有别的出路）。
- **② 上下文**：ADR-D10（实验域数据归 agent）；实测 agent 用到 storage 的 16 个符号 / 14 个文件：连接层 `PostgresStore`(12) `NewPostgresStore`(3) `BuildDSN` `DatabaseConfig`；实验域 `Experiment`(5) `ExperimentMetrics`(6) `ExperimentUpdate`(4) `ExperimentStatus*`；研究域 `ResearchProfile`(10) `ResearchQuestion` `ResearchConclusion`；**仪器域** `TradingCalendar` `ListingWindow`(4)。`kit/db` 由 QT-05 提供。
- **③ 要求**：
  ① agent 仓建 `ai` schema 的 DDL（表名以 core `pkg/storage/experiments.go` / `research.go` / `postgres.go` 的现有定义为准，逐表核对后迁移，**不凭类型名臆造表名**）与自己的 migrate 入口；
  ② agent 用 `kit/db` 建自己的 store，替换 12 处 `PostgresStore` 用法；
  ③ **仪器域不迁**：`TradingCalendar` / `ListingWindow` 属 core，agent 改走 core 的**只读 port**（D10 §87「只读视图」通道）——**第 7 类动作已于 2026-10-24 用户确认**（core 侧端点单号 QT-67，**已落地并推送：`2a4b076`**，本仓可开工消费）；该动作的 endpoint + DTO + 鉴权随 QT-50 ③-③/④ 统一落 `docs/openapi.yaml` 并纳入路由表对拍（勿另起手写 DTO，refactor-plan-review H2）；
  ④ 双写期：旧表与 `ai.*` 同时写 → 行数/校验和对拍 → 切读 → 停写；core 侧 `experiments.go` / `research.go` 定义标 deprecated 并登记删除期限；
  ⑤ 护栏：agent 禁止 import `pkg/storage`（AST）。
- **④ 约束与非目标**：**不删 core 旧表**（保留只读一个周期）；不迁行情/回测结果/因子定义（仪器域）；不改 core 的实验 handler；不在本单做 HTTP 化（QT-50）。
- **⑤ 验收**：agent `grep -r "pkg/storage"` = 0；`ai.*` 行数与旧表对拍一致（报告归档）；停写后 core 旧表不再增长；core 侧 deprecated 标记存在；日历/上市窗口经 port 取到且值一致。
- **⑥ 边界**：只做实验/研究域的数据归属；`ai.*` 之外的 schema 不动；X 线服务的数据归属按 QT-40。

> **🔶 进度（2026-10-23，① 腿）**：`pkg/aistore` 落 `ai` schema 六表 DDL（experiments / research_profile / research_conclusion / research_question / factor_genes / strategy_genes）+ `Migrate`（`kit/db.Migrate`，core 式幂等、无版本表）。表名与列**逐表对照** `tellurion:pkg/storage/postgres.go`，`parent_id` 自引用与外键改指 `ai.*`；`TestSchema_ColumnsMatchCore` 直接读 core 源码对拍（单仓 checkout 显式 Skip，CI checkout core 后生效），`TestStatements_AreIdempotent` / `TestMigrate_Integration`（`TEST_DATABASE_DSN` opt-in）钉住幂等与真库可跑。
> **🔶 进度（2026-10-23，② + ⑤ 腿，本次）**：
> - **补表**：① 腿漏了 `factor_hypothesis`（core `postgres.go:575` 有定义、① 腿搬表清单没抄到）——`ai.factor_hypothesis` DDL + drift 映射补齐。
> - **自有 store**：`aistore.Store`（`kit/db` 连接池 + 启动迁移 + builtin 种子；`NewStore` 启动路径 / `NewStoreWithPool` 库侧测试），语义逐条对照 core `experiments.go` / `research.go` / `factor_hypothesis.go` 搬入：失败也落行、running 是被叫停的合法终态、两步写（先落行再补参数）、`Get*` 未找到返回 `(nil, nil)`、`RecordVerdict` 与 `CompleteExperiment` 分离。**替换完成**：pipeline 的 `ExperimentSink`、loop 的 DatasetSplit、research / factor_hypothesis 工具、`cmd/ai` builders 全部切 `aistore` 类型——`12 处 PostgresStore 用法`里属于本仓读写面的已清零。
> - **④ 双写开始（对拍期）**：`cmd/ai/experiment_dualwrite.go` 双写 `ai.experiments` + legacy `public.experiments`；两张表 BIGSERIAL 各自发号，适配器内记 legacyID→aiID 映射（进程内，重启即失，差异由对拍测试暴露）。**读面已切 `ai.*`**（research 三表 + factor_hypothesis）——实证：真库 `research.*` 0 行、`factor_hypothesis` 12 行全 builtin 种子且 agent 启动镜像播种收敛、experiments 本仓只写不读。**停写待办**：对拍零差异 → 停写 legacy（检查项见 `docs/db-schema.md` §5：`factor_genes` / `strategy_genes` 无写入方，将来接写必须直写 `ai.*`）。
> - **⑤ 护栏**：`internal/repoguard/storage_import_test.go`——非测试 `.go` 禁 import `tellurion/pkg/storage`，白名单三条（`cmd/ai/main.go` 引擎仪器域装配 / `experiment_dualwrite.go` 双写期与 replace 同寿 / `handlers_explore.go` ListingWindow 仪器域待 D24 第 7 类 port），每条带理由+过期条件；**夹具正反取证**（违规→红 / 干净→绿 / 白名单放行 / 条目不再 import→红 / 条目文件消失→红 / 测试文件不在范围），真仓腿另查「白名单条目必须还活着」。
> - **验收（真库腿已跑）**：`TEST_DATABASE_DSN` 下全量 `go test ./...` —— aistore / pipeline / loop / cmd / repoguard 全绿；`pkg/ai/validation` 5 例红为**既有跨仓回归**（core 未推 QT-11~13 注册表改构造函数，与本单正交，处置归 C4/推送前）。构建取证：core 工作树有另一会话的 QT-52 WIP 会污染 replace 链，本单在 core HEAD `865f7bf` 隔离 worktree 下完成 `go build`/`go vet`/`go test` 取证。
> - **仍待**：S3 去 replace（依赖 QT-17 ③ + QT-50，见下）。③ 仪器域只读 port 已于 2026-10-25 消费 QT-67 落地（见下节）；core 侧 deprecated 标记已由 core 仓 QT-05 ④ 落地（删除期限见下交接件）。
> **🔶 进度（2026-10-24，④ 停写切换，本次）**：见下行专节。
>
> #### ④ 停写切换 ✅（2026-10-24，ODR-001；2026-10-25 复核）
>
> **已落**：
> 1. **对拍零差异**：双写适配器的真库对拍腿（完成/失败/裁决/父子链四条路径）切换前 `PASS`；存量单独数过——旧表 30 行（8 run / 20 行有父代）、`ai.experiments` 0 行（30 行全部早于双写期），故先做一次性回填归档。
> 2. **回填归档（幂等）**：`INSERT … ON CONFLICT (run_id, seq) DO NOTHING` 搬 30 行，再按 `(run_id, parent.seq)` 把 `parent_id` 从 legacy id 空间重映射到 ai 空间（20 行）。行级 `EXCEPT` 双向对拍：`legacy_only=0 / ai_only=0`。
> 3. **停写**：`cmd/ai/main.go` 直接注入 `*aistore.Store`（同时满足 `pipeline.ExperimentSink` 与 `verdictSink`）；删除 `cmd/ai/experiment_dualwrite.go` 及其对拍测试；`BuildDSN` 改走 `kit/db`，`main.go` 不再 import storage。
> 4. **护栏**：`internal/repoguard` 白名单三条 → 一条（只剩 `handlers_explore.go` 仪器域 `ListingWindow`）；新增真库腿 `TestStopWrite_LegacyExperimentsFrozen` 钉住「旧表行数与 `max(id)` 都不前进」。
>
> **验收取证（真实输出）**：停写驱动一轮完整写入后 `public.experiments` 仍 **30 行 / max(id)=1203**、`ai.experiments` 30 行；`go build ./...`、`go vet ./...`、`go test ./...`（含 `TEST_DATABASE_DSN` 真库腿）全绿。破坏验证：在 `main.go` 临时重新 import `tellurion/pkg/storage` → `internal/repoguard` 当场红（点名 `cmd/ai/main.go -> tellurion/pkg/storage`）→ 逐字节还原（SHA256 一致：`8D27E45D…E5B9FB`）→ 复绿，零残留。
>
> **未落 / 如实登记**：
> - `research.*` 的「停写」是**无操作**：全工作区不存在其生产写入方（core `pkg/storage/research.go` 只有 `GetResearchProfile`，`cmd/analysis` 无研究域 handler；真库 0 行）。此前台账/`db-schema.md` 写的「写方在 core」**与代码事实不符**，已订正。
> - `factor_genes` / `strategy_genes` **当时仍写 core `public.*`**（`pkg/ai/gene_pool` 用未限定表名，经 `search_path` 落到 `public`，池也是 core 的 `store.DB()`）；两表真库当前 0 行。**已于 2026-10-25 由 ⑤ 收口**（见下节），本条保留为当日的如实记录。
> - `grep -r "pkg/storage"` **未归零**：生产代码 1 处（`handlers_explore.go`，等 D24 第 7 类动作），其余在 `_test.go` / 文档 / 护栏注释；整体归零依赖 S3 删 replace（QT-50）。

> **W7 复核（2026-10-25，④ 收口）**
> - **写路径**：`cmd/ai/experiment_dualwrite.go` 不存在（`b1a8fcd` 删除）；`cmd/ai/main.go` 只注入 `*aistore.Store`（同时满足 `pipeline.ExperimentSink` 与 `verdictSink`）。
> - **对拍零差异报告（归档）**：ODR-001 `§Metrics` —— 回填后 `public.experiments` / `ai.experiments` 各 30 行、`parent_id` 非空 20；行级 `EXCEPT` 双向 `legacy_only=0 / ai_only=0`；停写取证 `public.experiments` 30 行 / `max(id)=1203` 不变。
> - **护栏**：`storageWhitelist` 为空（`experiment_dualwrite.go` 条目已随文件删除）；`TestStorageImportBoundary_Fixtures` 五条反证腿常驻（违规→红 / 干净→绿 / 白名单放行 / 条目不再 import→红 / 条目文件消失→红）。
> - **反证（本轮实测，两条腿）**：① 新建 `cmd/ai/experiment_dualwrite_LEG.go`（`import _ "tellurion/pkg/storage"`）→ `TestStorageImportBoundary` 红并点名该文件；删除后复绿（工作区零残留）。② 临时给已删文件登记白名单条目 → 同一条腿红（`storageWhitelist 有条目已失效 … cmd/ai/experiment_dualwrite.go`）→ 逐字节还原（`SHA256=2F818BE3…3572425B` 与改前一致）→ 复绿。
> - **core 侧**：`pkg/storage/experiments.go` / `research.go` 当时标 `Deprecated:`、删除期限 2027-01-31；**已于 2026-11-10 随 `SVC-CORE-AI-RETIRE` 提前删除**（含 `deprecated_legacy_test.go`），本节保留当日记录。
> - **门禁（真实输出）**：`go build ./...` / `go vet ./...` / `go test ./... -count=1` 全绿（`cmd/ai 0.478s`、`internal/repoguard 0.344s`、`pkg/ai/pipeline 4.259s`）。`gofmt -l .` 命中两个**既有** BOM 文件（`pkg/ai/drift/detector.go`、`pkg/ai/validation/redundancy.go`），与本次改动无关，未动。
> - **未落（如实）**：`TestStopWrite_LegacyExperimentsFrozen` 本机无 `TEST_DATABASE_DSN` → skip；本轮未重跑真库腿，2026-10-24 的停写取证仍为最后一次真库证据。生产 import 已零；`grep -r "pkg/storage"` 仍命中护栏注释与断言文案（不是 import）。

> #### ③ 仪器域只读 port 消费（2026-10-25，QT-67）
>
> **已落**：
> 1. **客户端**：`pkg/ai/client/reference_data_client.go` 新增 `ReferenceDataClient`，消费 core QT-67 的两条只读动作（`GET /api/v1/trading-calendar`、`POST /api/v1/listing-windows`）。DTO 与 core `docs/openapi.yaml` 逐键对拍（`TestOpenAPI_ReferenceDataShapesMatchContract`），端点字面量自动进入 `TestClientEndpoints_AreRegisteredInCoreOpenAPI`。凭证**每次调用显式传入**：handler 从入站请求取 `Authorization: Bearer …` 转发，客户端不签发、不刷新、不降级成匿名（缺 token 在发请求前 `ErrReferenceDataNoCredentials`）。
> 2. **偏差维接线**：`cmd/ai/handlers_explore.go` 的 `listing` 从 `engine.ListingWindows`（core 进程内实现）改为 `client.ListingWindows`；偏差维的 `PoolSource` / `PoolSize` / `DelistedInPool` 语义不变（`delist_date != nil` 计为池内已摘牌）。窗口读不到时**保持最保守口径** `PoolSourceCurrent` 并留日志 —— 不把一次失败当成「没有数据」。
> 3. **护栏清零**：`internal/repoguard` 的 `storageWhitelist` 三条 → 一条 → **0 条**；`handlers_explore.go` 同时删掉 `tellurion/pkg/storage` 与 `tellurion/pkg/backtest` 两个 core 实现包 import（`registerExploreRoutes` 不再需要 `*backtest.Engine`）。
> 4. **依赖联动**：core 当日已把 contracts pin 切到 `v0.6.0`；本仓 `replace tellurion => ../tellurion` 期间 core 的 pin 参与本仓模块图，故同批 `go mod tidy`（contracts `v0.5.0` → `v0.6.0`，新增 `wazero` indirect）。这是仓内 AGENTS §4 已登记的联动，不是新增依赖。
>
> **验收取证（真实退出码）**：`gofmt -l`（改动的 7 个文件）→ 空；`go build ./...` → **0**；`go vet ./...` → **0**；`go test ./cmd/ai/... ./pkg/ai/client/... ./internal/repoguard/... -count=1` → 全 `ok`，**0**。
> **破坏验证**：临时新建 `cmd/ai/__storage_probe.go`（`import _ "tellurion/pkg/storage"`）→ `TestStorageImportBoundary` 当场红并点名 `cmd/ai/__storage_probe.go -> tellurion/pkg/storage`（**EXIT=1**）→ 删除探针后复跑 → **EXIT=0**，工作区零残留。
> **仍未做（如实）**：`guard` 判据只覆盖「生产 .go 不 import storage」；`_test.go` 里的合法用法（如 `pkg/ai/validation/live_backtest_integration_test.go` 查仪器域）仍在护栏范围外，单独 `grep -r` 不会归零。整体归零仍等 S3 删 `replace`（QT-50）。

> #### ⑤ 基因池读写切 `ai.*`（2026-10-25）
>
> **已落**：
> 1. **SQL 全限定**：`pkg/ai/gene_pool/factor_pool.go` / `strategy_pool.go` 的 18 处表引用全部写成 `ai.factor_genes` / `ai.strategy_genes`（`grep -E "(FROM|INTO|UPDATE)\s+(factor|strategy)_genes"` 零命中）。
> 2. **连接池自持**：`aistore.Store` 新增 `Pool()`；`cmd/ai/main.go` 的 `gene_pool.NewFactorPool/NewStrategyPool` 改注入 `aiStore.Pool()`，不再借 core 的 `store.DB()`（同一 `database.*` DSN，仅句柄归属变成本仓）。
> 3. **护栏**：`pkg/ai/gene_pool/sql_scope_test.go` 的 `TestSQL_TableRefsAreSchemaQualified` 静态解析两个源文件的**字符串字面量**（注释不参与），裸表名即红；`pool_ai_schema_integration_test.go` 的 `TestPools_WriteOnlyAiSchema` 是真库对拍腿（CRUD 后 `ai.*` 有行、`public.*` 同 id 行必须为 0，旧表不存在也算通过）。
>
> **验收取证（真实退出码）**：`gofmt -l pkg/ai/gene_pool pkg/aistore cmd/ai` → 空；`go build ./...` → **0**；`go vet ./...` → **0**；`go test ./pkg/ai/gene_pool/... ./pkg/aistore/... ./cmd/ai/... -count=1` → 全 `ok`，**0**。
> **破坏验证**：把 `factor_pool.go` 一条 SQL 临时改回 `FROM factor_genes` → `TestSQL_TableRefsAreSchemaQualified` 当场红并点名 `factor_pool.go: 未限定 schema 的表引用 "factor_genes"`（**EXIT=1**）→ 逐字节还原 → 复绿（**EXIT=0**），工作区零残留。
> **未做 / 环境限制（如实；2026-10-26 订正）**：真库腿 `TestPools_WriteOnlyAiSchema` 本机未执行——`127.0.0.1:5432` 无 PG 监听；Docker daemon 本轮**已可达**（Engine 29.8.0），但**未起专用 PG 容器**，故仍只证到「无 DSN 时显式 skip」，**未取到真库绿**。有库环境跑 `TEST_DATABASE_DSN=... go test ./pkg/ai/gene_pool -run TestPools_WriteOnlyAiSchema -count=1` 或 `docker run postgres` 后同命令。
> **不在本单**：旧表 `public.factor_genes` / `public.strategy_genes` 当时保留不删（切换前真库各 0 行、无脏数据），退役原定随 core QT-05 ④（2027-01-31）；**已于 2026-11-10 随 `SVC-CORE-AI-RETIRE` 提前删除**。

> #### core 侧交接件（QT-17 ④ 的另一半——本仓**不改 core**，报协调者）
>
> **状态（2026-10-25；2026-11-10 订正）**：core 侧已落 —— `experiments.go` / `research.go` 的导出符号带 `Deprecated:` 说明，删除期限登记在 `tellurion:docs/TASKS.md` QT-05 ④；**2026-11-10 已随 `SVC-CORE-AI-RETIRE` 提前删除**。本仓此处保留交接要求作为来源记录。
>
> **要做**：把 core `pkg/storage` 的旧表定义与存取方法标 `deprecated`，并登记删除期限。
> - `tellurion:pkg/storage/experiments.go`：`Experiment` / `ExperimentMetrics` / `ExperimentUpdate` / `ExperimentStatus*` / `DatasetSplit*` 及 `InsertExperiment` / `GetExperiment` / `ListExperiments` / `UpdateExperiment` / `CompleteExperiment` / `RecordVerdict`。
> - `tellurion:pkg/storage/research.go`：`ResearchProfile` / `ResearchConclusion` / `ResearchQuestion` / `GetResearchProfile`（注：**该文件只有读方法，core 侧无写入方**）。
> - `tellurion:pkg/storage/postgres.go` 的 `migrationStatements()`：`experiments`（:424、`verdict` :449 的 ALTER）、研究域三表（:361-398）——DDL 暂留（不删表），标注 owner 已迁至 `tellurion-agent:pkg/aistore`。
> - **前置事实（供 core 复核）**：`GetExperiment` / `ListExperiments` 无生产调用方（仅 `*_test.go`）；core 无实验日志 HTTP 端点；`tellurion-ui` / `tellurion-analytics` 均不读这两块数据。
> - **建议删除期限**：**2027-01-31**（保留只读一个季度，覆盖一次发布周期后再删）。需协调者/core 确认；本仓不代改。
> - **删除前置**：S3（QT-50 完成、agent 删 `replace`）先落，否则 core 的 `storage` 包仍被 agent 的生产/测试代码引用。

> **S3 前置状态（2026-10-25 实测，已于同日 S3-d 收口）**：core `QT-50` 已 ✅（2026-10-24），当时的卡点在本仓：**15 个 core 实现包 / 22 个生产文件 / 19 个测试文件 / 59 处 import**（`go.mod` 注记的 64 处 / 43 文件含 2 个伪命中，见拆解清单 §0）。逐包归属、动作与子批顺序见 [`docs/s3-decoupling-plan.md`](s3-decoupling-plan.md)（B6）。**收口判据已满足**：`go.mod` 无 core require / `replace`，CI 只 checkout 本仓，单仓 `go build/vet/test` 全绿，`grep '"tellurion/'` 零命中。
>
> **S3 子批进度（2026-10-25）**：S3-a ✅（6 包换源，59 → 42 处）、S3-b ✅（`auth` + `strategy/monitor` 去重，42 → **39** 处：清 4 处、另新增 1 处 `tellurion/pkg/httpserver` 别名用于 insecure 回环门，理由见拆解清单 S3-b 施工记录）、S3-c-1 ✅（`risk` 工具 HTTP 化：接 `POST /api/risk/detect_regime` 粗粒度形状，删本地 `RiskManager` 构造，39 → **38** 处，生产 `pkg/risk` 零命中；如实登记输出删 `bars_analyzed`，见拆解清单 S3-c-1 施工记录）、S3-c-2 ✅（`strategy` 注册表 HTTP 化：`strategy.list` 接 `GET /api/strategies` 合并目录、`strategy.get` 改从 list 过滤，注册表工具生产代码不再 import `tellurion/pkg/strategy`，38 → **36** 处；`strategy` 包仍有 loader / pipeline 2 个生产文件的类型面，如实登记，见拆解清单 S3-c-2 施工记录）、**S3-c-3 ✅（2026-10-25，本次）**：backtest / walk-forward 全部走 HTTP，`cmd/ai` 的进程内引擎装配（`newEngine` / `pgProvider` / `dataAdapter` / `wfEngine`）已删；pipeline 改提交 `strategy=expression_template` + `strategy_params`。**动手前证伪一条前提**：core 的 analysis 二进制原先没导入 `pkg/strategy/expression`（`go list -deps` 实测 0），`expression_template` 不可执行；经授权在 core 加一行空导入（core `faec551`）。生产 import 14 → **10 处 / 8 → 7 文件**（清掉 `backtest` 2 + `marketdata` 1 + `strategy` 1），测试 22 → **19 处 / 12 → 9 文件**；合计 36 处 / 20 文件 → **29 处 / 16 文件**。如实登记：本机无 PG、未跑真库 A/B 指标对拍；非法 DSL 的失败点从 Stage 3 后移到 Stage 5。详见拆解清单 S3-c-3 施工记录。**S3-c-4 ✅（2026-10-25，本次）**：expression 静态校验与目录改走 `POST /api/expression/validate` + `GET /api/expression/catalog`；`validate_factor` 工具、`ValidateAgent` 的 L1/L2、研究/策略提示词的 DSL 段全部改为消费 core 的判定与目录，本仓 **`tellurion/pkg/expression` import 归零**（生产 4 文件 / 测试 6 文件清零）。生产 core import 10 → **6 处 / 7 → 3 文件**（清 `expression` 4 处；剩余 `bootstrap` 2 / `httpserver` 1 / `observability` 1 / `strategy`+`strategy/expression` 2 全在 loader），测试 19 → **12 处 / 9 → 3 文件**。如实登记：L2 取数从「进程内 sample provider」改为 core 的 `/api/factor/compute`（默认票池 × 2023 全年）；L1 的「复杂度 >10 警告」删除（core 判定不返回节点数）；目录/闸门一致性用例改为 **env-gated**（设 `ANALYSIS_URL` 才打真 core），本地 skip 不算通过。详见拆解清单 S3-c-4 施工记录。**S3-d ✅（2026-10-25，本次）**：`bootstrap` / `observability` / `httpserver` 换 kit 或本仓自持（`cmd/ai/lifecycle.go`：配置 / 日志 / 指标 / 限流 / 回环门 / 启动关停 / 鉴权启动门），CI 删掉 core checkout 与 `go mod edit -replace`，`go.mod` 删 `tellurion` require 与 `replace`，`grep '"tellurion/'` **零命中**；删掉只会 import core 的测试腿（`validation` 4 个 in-process 引擎取证文件）与死代码 `yaml/loader.go`（LoadStrategy/LoadAndRegister 零生产调用方）。如实登记：这 4 个取证文件所验的「验证器接得上引擎 / 真库行情形状」不再有本地用例，执行与数据形状的取证归 core + env-gated 端到端腿。详见拆解清单 S3-d 施工记录。

---


<a id="QT-61"></a>
## QT-61（agent 腿）· CI 基线归零——OBS-08 注册表/护栏测试漂移 ✅ 完成（2026-10-21）

> 跨仓同号同单，完整四腿收口记录见 `tellurion:docs/TASKS.md` QT-61；此处只登记本仓腿。

- **要点**：判定 `turnover_rate` **确可被 DSL 求值**（注册于 `pkg/expression/registry.go:222`，Source=activity、DependsOn=market_activity，有物理表 `market_activity`（Migration 056））⇒ **修测试不改注册表**：把 `turnover_rate` 从两项护栏的幻影清单移除。
- **要点（根因与收敛）**：根因是清单在两个测试文件各抄一份、D5 加注册项后没同步删 → 收敛为单一来源 `dslPhantomFields` + `phantomViolations` 的「过时自检」，加 `TestPhantomGuard_DetectsStaleRegistry` 三腿破坏验证。
- **要点（验证）**：本地复现红→绿；首跑 CI 实测（2026-10-21）`7f711e5` 后 agent main 全绿。

---


<a id="QT-65"></a>
## QT-65（agent 腿）· QT-11 跨仓回归：pipeline / yaml 生产路径改工厂注册 ✅ 完成（2026-10-24）

- **① 目标**：把本仓两条生产注册路径从「注册共享实例」改为「注册工厂」，使引擎的 `Registry.NewInstance(name)` 能拿到每 run 一份的实例；同时把旧「就地 reconfigure 已注册实例」语义改成「重新注册工厂的 YAML 源」，不得再把已配置实例塞回共享注册表。
- **② 上下文**：core QT-11（`67bee39`）把注册表改存构造函数后，`strategy.GlobalRegister(实例)` 的共享条目在 `NewInstance` 处 fail-closed 返回 `ErrSharedStrategy`。原调用点：`pkg/ai/pipeline/pipeline.go` 的 `registerOrConfigure` 注册分支（`Execute` / `ExecuteFromYAML` 共用）与 `pkg/ai/yaml/loader.go` 的 `LoadAndRegister`。
- **③ 落点与实现**：
  1. `yaml.LoadAndRegister` 改为 `strategy.GlobalRegisterFactory`；工厂体是 `yamlSource.build`，每次调用按**当前 YAML 文本**重建一份完整配置的 `ExpressionStrategy`（手写互斥保护，`NewInstance` 可并发调用）。
  2. `yamlSource` 按策略名登记在包内 `strategySources`：同名再次 `LoadAndRegister` 只替换 YAML 源，注册表里的工厂始终指向同一源对象 —— 工厂注册表没有 replace/unregister 入口，这是「重新注册工厂」在不改 core 前提下的落地形态。
  3. pipeline 删除 `registerOrConfigure`（含 `GlobalGet` → 就地 `Configure` 分支），`buildAndRegister` 统一走 `yaml.LoadAndRegister`；非表达式同名冲突由 YAML 层显式报错，不再静默改原型。
  4. 回归用例走真实执行路径：`pkg/ai/pipeline` 的 `TestExecuteFromYAML_RegistersFactoryForEngine` 用一个只经 `strategy.GlobalNewInstance` 取实例并真的调用 `GenerateSignals` 的 runner；`pkg/ai/yaml` 的同名重登记用例断言新实例按最新 YAML 重建（`signal_expr`）。
- **④ 约束与非目标**：不删 `replace tellurion => ../tellurion`（S3 未到）；不 import `tellurion/pkg/storage`（repoguard 白名单已清零）；不在本仓实现编译器；不改 core（deprecated 标记 / D24 第 7 类只读 port / QT-50 HTTP 端点均跨仓）。
- **⑤ 验收取证（真实输出）**：
  - `gofmt -l pkg/ai/pipeline pkg/ai/yaml` → 空。
  - `go build ./...` → EXIT=0；`go vet ./...` → EXIT=0。
  - `go test ./... -count=1` → 全部 `ok`（含 `pipeline 4.910s` / `validation 7.209s` / `repoguard 0.543s`），EXIT=0。取证环境：`golang:1.25-alpine` 容器 + core HEAD `2629ac7` 的 `git archive` 快照（replace 链指向快照，避免 core 工作树其他会话 WIP 污染）。
  - 目标用例：`TestExecuteFromYAML_RegistersFactoryForEngine`、`TestPipeline_ExecuteFromYAML_RegistrationCollision`、`TestLoadAndRegister_HappyPath`、`TestLoadAndRegister_ReregisterSwapsFactory` 全 PASS。
- **⑥ 破坏验证**：把 `LoadAndRegister` 的 `strategy.GlobalRegisterFactory(src.build)` 临时改回 `strategy.GlobalRegister(s)`（即回到两处原语义）→ 上述两条执行路径用例当场红：
  `engine strategy lookup failed: strategy is registered as a shared instance: ... (registered with Register; a per-run instance needs RegisterFactory)`。
  从备份逐字节还原后 `SHA256=0EB49D35…77C59D7A` 与改前一致，复跑全绿。
- **⑦ 停止项（按派发要求不动）**：`QT-17 ③`（D24 第 7 类只读 port）、`QT-07 ⑥ S3`（删 replace）、`QT-32`（依赖 G1-QT-26 / G5-QT-41b）继续等待；QT-17 ④ 停写切换已于 `b1a8fcd` 提交，本轮不重复施工。
  > **该停止项已于 2026-10-25 解除**：QT-26 ✅（core 2026-10-24）、QT-41b/QT-41c ✅（strategy 仓 2026-10-24/25）、QT-03/QT-63 ✅。agent 腿已在本仓立单执行 → 见下方 **QT-32-A**。

---


<a id="QT-32-A"></a>
## QT-32-A（agent 腿）· AI 生成链留痕 + promote 闸门的生产者侧 fail-closed ✅ 完成（2026-10-25，范围 = ① ~ ⑥；⑦ 全部是未落的后续单，不算已完成）

> **立单来历**：core `docs/TASKS.md` QT-32 ⑥「生成质量与实验编排属 agent 仓（其台账另立单）」+ ④「不把 AI 编排逻辑搬进 core（ADR-023）」。
> 2026-10-25 派发 G8 时实测本仓台账**没有** QT-32 的六字段条目（`grep QT-32 docs/TASKS.md` 只有上方那条「继续等待」），故先立单再施工。
> 本单只做 **AI 侧**；闸门结论的真相在资产服务（下方 ② 锚点），本仓不建第二套。

- **① 目标**：一条 AI 生成的策略在**本仓**这一段可追溯——prompt、生成源码、源码指纹、静态闸与编译结果**逐环落进 `ai.*`**，且在没有任何可核验产物坐标时**响亮地拒绝**把它送进回测（链断在哪一环、为什么断，都能被查出来）。
  判据不是「生成了」，是「生成过的那一份，事后还能不能拿出来说明它是哪一份、编过没编过、为什么没跑」。
- **② 上下文**（开工前逐一读；全部为 2026-10-25 实测）：
  - **今天一条留痕都没有**：`pkg/copilot/copilot.go:95` 的 `jobs sync.Map` 是内存态，进程重启即失；源码写到 `os.MkdirTemp` 的临时目录、`defer os.RemoveAll` 删掉（`:341-349`）；`ai.experiments`（`pkg/aistore/schema.go:55-72`）**没有任何一列**装 prompt / 源码 / hash / 产物身份，copilot 路径连实验行都不写。
  - **链的最后一环是假的**：`:411` 只 `go build -o <临时可执行文件>`（**不是** `-buildmode=plugin`），`:457` 却拿它去 `plugin.Open`（`:492-507`）——必然失败，回落成硬编码字面量 `"generated-strategy"`；`:470-487` 再拿这个名字调回测。实测 core 从未注册过该名字（`grep -rn 'generated-strategy' tellurion:pkg tellurion:cmd` = 0 命中），且 `tellurion:pkg/strategy/registry.go:123` 对未注册名报 `strategy not found` ⇒ 这条回测**从来没跑成过**，只是断在 core 而不是断在本仓。`plugin.Open` 这个形状本身还违反 ADR-024（「LLM 生成的 Go 代码只是可审阅 artifact，不加载、不执行」）与本仓 `docs/api-schema.md` §5（「不做策略执行沙箱」）。
  - **core 的 run 身份其实拿得到**：`pkg/ai/client/backtest_client.go:285` 已从 `BacktestResponse.ID` 解出并放进 `BacktestOutcome.JobID`；丢它在适配层——`cmd/ai/http_runner.go:163` 只 `return outcome.Result, nil`。但 `contracts.BacktestRunner` 的签名（本仓 `pkg/copilot/copilot.go:69` 以别名复用，且 `pkg/ai/pipeline/backtest_runner_alias_test.go:43-46` 钉住它是**类型别名**）不带这个字段 → 要把它送过这一层是跨仓契约变更，另单（见 ⑦）。
  - **闸门真相在资产服务，不在本仓**：`tellurion-strategy:internal/httpapi/api.go:42-51` 已实装 `POST /api/strategies`、`PUT .../source`、`POST .../build`、`POST .../versions/{hash}/promote`、`GET .../resolve`；`tellurion-strategy:internal/asset/ddl.go:53-63` 的 `strategy.strategy_artifacts.status` 是 `built→validated→promoted/rejected/deprecated` 状态机，`api.go:53-54` 明写「promote 是人的闸门」。**本仓因此不存「是否已 promote」这个结论**——存了就成两处真相；本仓只按「有没有可核验的产物坐标」拦。
  - **本仓自己算不出 `version_hash`**：`tellurion-contracts:engine/provenance.go:65-72` 的六项输入含 `strategy_id`，而 id 由资产服务发号（`POST /api/strategies` → `201 {id}`）。所以本单的指纹列**明确叫 `source_sha256`**，不冒充 `version_hash` / `artifact_hash`（后者按 `provenance.go:39` 只覆盖编译产物字节）。
  - **③-⑤「沿用既有 AST/白名单」这句前提要订正**：本仓代码生成侧的闸是**正则**，不是 AST——`tellurion-kit:sandbox/staticcheck/staticcheck.go:156-235` 的 13 条 `builtinPatterns`，它自己的文档（`:7-18`、`:44-88`）写明「Why regex and not go/ast」与六条绕过形状、「not a security boundary」，真边界在第二层 `sandbox/runner`。本仓唯一的 AST 护栏是**构建期仓内卫生**（`internal/repoguard/storage_import_test.go`），与代码生成无关。本单**沿用现状不新造**，代价在此如实登记。
  - **wire 契约不能顺手改**：ui 侧 `tellurion-ui:web/src/api/copilot.ts:30-37` 用**终态白名单**判定轮询结束（`generated` / `backtest_complete` / `llm_failed` / `sandbox_rejected` / `build_failed` / `backtest_failed`），`web/src/types/api.ts:182-190` 登记同一套取值。新增状态串会让前端一路轮到超时。故本单的拦下态**复用 `sandbox_rejected`**。
  - `pkg/aistore/schema_core_drift_test.go:136-153` 现在要求 `Tables` 里每张表都有 core 来源（QT-17「逐表搬、不臆造」的腿）。本单新增的表**core 没有对应物**（AI 侧留痕本来就随拆仓迁到本仓），所以那条腿必须显式扩一个「本仓自持表」声明，而不是把表藏起来不加登记。
- **③ 要求**：
  1. `ai.generation_chain` 新表（`pkg/aistore/schema.go` 的 `statements` 末尾追加幂等 DDL，并登记进 `Tables`）：**一行 = 一次编译尝试**，主键 `(job_id, attempt)`；列 = `job_id`（本仓 copilot 作业身份）/ `attempt` / `prompt`（NOT NULL）/ `universe`·`start_date`·`end_date`（人要求的回测区间，被拦也照样留）/ `source` / `source_sha256` / `build_state`（DDL `CHECK` 闭合词表 `none｜failed｜passed`）/ `build_error` / `blocked_reason` / `created_at`·`updated_at`。列的含义与三条边界写进 `docs/db-schema.md`。
     > **工单原文的两处在本单内被改对（不是扩范围，是原写法机械不可执行）**：① 原来只写「`job_id` 主键」——那样重试会覆盖上一 attempt，而重试恰恰是另一份字节、另一个指纹，「中间试过什么」正是复查生成质量要看的东西；② 原来没有回测区间三列——「人当时要求跑什么」本来就是链上的一环，闸门拦下了也得留。
  2. `pkg/aistore` 新增 `GenerationChain` 结构 + `UpsertGenerationChain`（同 `(job_id, attempt)` 重复写 = 更新，不是插第二条）+ `ListGenerationChain`。
     > 原写的是 `GetGenerationChain(jobID)`：链的答案在**序列**上（每个 attempt 一行），单行 Get 要么强制调用方先知道 attempt 号、要么悄悄只给最后一行 —— 交付时改成 List，没有第三种读法。
  3. `pkg/copilot` 新增 `ChainSink` port 与 `WithChainSink` 注入点，**与 `WithCodeChecker` / `WithBuildExecutor` 同形：未注入即 fail-closed**（记不下链 = 不许生成）。装配根 `cmd/ai/builders.go` 注入 `*aistore.Store` 的适配器。
  4. 逐段落库：作业创建时（prompt）→ LLM 出码后（source + sha256）→ 静态闸结论 → 编译结论 → 终止原因。任一环失败都要留下一行，不留「跑过但查不到」。
  5. **闸门**：删除 `loadPluginSymbol`（含 `plugin.Open`）与硬编码 `"generated-strategy"`；编译成功后**不再**自行决定回测，改为按「有无产物坐标」判定——本仓今天产不出资产服务认可的坐标 ⇒ 一律拦，链上 `blocked_reason=promote_gate`，终态 `sandbox_rejected`，给人看的解释走 job 的 `build_error`。
     > **施工中的订正（两条轴不许共用一列）**：工单原文那句「`build_error` 说明为什么拦」会让链上的 `build_error` 装着闸门的措辞 —— 读库的人就分不出「编不过」和「编过了但不许跑」，而那正是要看的差别。交付拆成两个终态出口：`stop`（环节错误 → 写 `build_error`）与 `stopAtGate`（闸门 → 只写 `blocked_reason`），并由破坏验证 G 条钉住（把闸门改走 `stop` 当场红）。
  6. `source_sha256` = 对**最终送编译的那份源码字节**取 sha256（重试修过就是另一份）；注释写明它与 `version_hash` / `artifact_hash` 不是同一件事。
- **④ 约束与非目标**：
  - **不改 wire**：`/api/copilot/*` 的路径、状态取值集合、`/stats` 的 `backtest_valid` / `acceptance_rate` 字段都不动（后者结构上恒 0，直到 ⑦ 的 A2 接上——这是**如实反映**，它此前也从未非 0 过：回测从没跑成）。
  - **不在本仓建第二套闸门真相**：不存 promote 结论、不存资产服务的 status；只在运行时按可核验坐标拦。
  - **不接资产服务**（A2，见 ⑦）：本单不新增 `pkg/ai/client` 客户端、不加 `ai_service.asset_api_url` 配置键。
  - 不动 `pipeline` / `explore` 的执行链（它们的载体是表达式 + `strategy_params`，不是 AI 生成的 Go）；不改 `contracts.BacktestRunner` 别名及其护栏；不 import core 任何实现包；不新增第三方依赖；不放宽 `staticcheck` / 沙箱预算任何一条。
  - 不删 core 侧的 `strategies` / `quant.run_provenance` 相关内容（本仓不改 core）。
- **⑤ 验收**（真退出码，逐条贴输出）：
  1. 留痕用例：一次生成后 `ai.generation_chain` 有一行，`prompt` / `source` / `source_sha256` / `build_state` 齐全；**改源码一个字节 → 指纹变**（同 prompt 同 job 不留旧指纹）。
  2. 闸门用例：注入「编译成功」的执行器 ⇒ 断言链上最后一行 `build_state=passed` 且 `blocked_reason=promote_gate`、job 终态 `sandbox_rejected`、`build_error` 点名「不进回测」、`Stats()` 的 `backtested == 0`。
     ⚠️ **本条判据在施工中被改写过（如实登记）**：原写法是「注入一个会记录调用的假 `BacktestRunner`，断言它被调用 0 次」。施工中发现 `Generate` 的 runner 参数已无可达路径，留着就是一个「看着还能跑回测」的假接口，于是删掉参数与 `plugin.Open`；进程内**不再有回测通道**，那条计数断言随之不可表达。替代腿是源码级的 `TestQT32A_NoFabricatedStrategyPathInProductionSource`（本包生产源码里出现 `plugin.Open` / `"generated-strategy"` / `"copilot-generated"` / `RunBacktest(` 任一即红），它的牙齿由破坏验证 B 腿取证。
  3. 未注入 sink fail-closed 用例：状态终态且错误点名「chain sink not configured」（与既有 nil-checker / nil-executor 两条腿同形）。
  4. store 用例（**没有内存版 Store 假件**：`aistore.Store` 绑 `pgxpool`，为这条腿再造一个内存实现等于开第二条存取路径；替代形态见下）：
     - 既有静态腿自动覆盖新表：`TestStatements_AreIdempotent` / `TestStatements_OnlyTouchAISchema` / `TestTables_MatchStatements` / `TestStatements_IndexesFollowTheirTables` / `TestStatements_NoForeignSchemaReferences`；
     - 新增 `TestGenerationChain_InsertCoversEveryColumn` / `_SelectCoversEveryColumn` / `_UpsertRewritesEveryMutableColumn`：把 `INSERT` / `SELECT` / `DO UPDATE SET` 三处列清单与 DDL 的列集**逐列对拍**（core 没有对应表，就自己当自己的尺子），外加冲突键必须等于主键；
     - 新增 `TestGenerationChain_BuildStateVocabularyMatchesDDL`：Go 侧编译状态常量 ↔ DDL `CHECK` 取值双向闭合；
     - 真库 opt-in 三条：同 `(job_id, attempt)` 写两次 = 一行且是最后状态（同一条用例里再写第三个 attempt = 两行、按 attempt 升序、两个指纹不同）/ 未登记的 `build_state` 被 `CHECK` 拒 / 未知 job 读回空列表不是 error；另一条 `空 job_id 被拒` 刻意**不连库**（校验在 Exec 之前），CI 无 PG 也真的在跑。
  5. drift 护栏的新腿：本仓自持表**必须**在 `Tables` 且**不许**出现在 core 对照表；把已登记为 core 搬迁的表改成本仓自持 → 红（防止用豁免名单夹带）。
  6. **破坏验证两条**：① 绕过闸门（把拦下分支删掉，恢复拿硬编码名跑）→ 用例 2 当场红 → 逐字节还原 → 绿；② 把 `source_sha256` 写成固定常量 → 用例 1 的红指纹腿当场红 → 还原 → 绿。两条都要贴改前/改坏/还原三次退出码。
     > 交付时扩到 **8 条**（闸门留痕 A、两条轴分列 G、重试不留旧行 H、假策略名 B、指纹脱离源码 C、写失败取反 D、SQL 列缺 E、豁免夹带 F），逐条输出见下方施工记录。
  7. 门禁：`gofmt -l`（改动文件）空、`go build ./...` / `go vet ./...` EXIT=0、`go test ./... -count=1` 全绿且**不新增红**。
- **⑥ 边界**：只做 copilot 这一段生成链的留痕与生产者侧闸门；资产登记/编译产物/promote 审批面属 `tellurion-strategy`，`strategy_ref` 解析与 `quant.run_provenance` 属 core（QT-32 主单），生成质量（提示词、表达式与载体形状）属下一单。
- **⑦ 未落 / 后续（如实登记，不写成已完成）**：
  1. **`artifact_hash` 在本单没有产出方**。本仓 `go build` 产出的是临时可执行文件，不是资产服务认可的载体 ⇒ 表里**不留** artifact 列（留了就是空列或假 hash）。要产出真坐标必须先把「AI 生成物的载体」裁决下来：`carrier=wasm` + 12 个 host 函数 ABI（对齐 QT-26/30/31 的 `wasmSignalProvider`）/ `carrier=so`（Windows 不支持 `plugin`，core AGENTS §2 已记）/ 维持 ADR-024 的「只审阅不执行」。这是**跨仓 + 契约层**决定，需协调者裁。裁完才立 A2：接 `PUT /api/strategies/{id}/source` + `POST .../build`，拿真 `version_hash`（含资产服务发的 `strategy_id`）与 build job 状态。
  2. **链的第五环（core run 身份）本单接不上**：闸门拦下后不回测；且 `contracts.BacktestRunner` 签名不带 run 身份（`pkg/ai/pipeline/backtest_runner_alias_test.go:43-46` 钉住别名）。A3 = 让 run 身份穿过这一层（跨仓契约变更，要走 contracts 兼容矩阵 MINOR），落点在 `cmd/ai/http_runner.go:163`。
  3. **闸门只在本仓生产侧**；「未过 promote 的产物被回测路径拒绝」的**执行侧**那条腿在 core + 资产服务（QT-32 主单 ⑤），本仓无法替它作证。
  4. **服务间凭证（QT-71）不是本单的阻塞，但 core 那条登记已过期**：本单不新增跨服务调用，所以与它无关；施工时顺手核了远端 —— `git ls-remote --tags origin` 实测 `tellurion-kit` 已有 **`v0.1.2` / `v0.1.3`**（core 台账 QT-71 登记的是「远端只有 v0.1.0/v0.1.1」，那条前提**今天不成立**）。**报协调者**：QT-71 是否解除归 core 判，本仓不改 core。另一条相关事实：`tellurion-strategy:internal/httpapi` 今天**没有任何鉴权面**（全文件零 auth 引用），所以 A2 若要调资产服务，凭证不是它的前置。
  5. **`-race` 未跑**：本机无 C 编译器（`CGO_ENABLED=1` → `cgo: C compiler "gcc" not found`），`-race` 直接 build failed。本单动了 copilot goroutine 里的写库调用，并发面按 AGENTS §2 本该带 `-race`，代价登记在此。
  6. **真库腿本机未跑**：本机无 PG 监听、本轮未起 PG 容器（Docker 已可达，见 2026-10-26 订正）⇒ opt-in 用例只证到「无 `TEST_DATABASE_DSN` 时显式 skip」，不装成通过。
  7. **DBT-18（mining 的真实 Proposer）归属本仓** —— **裁决（2026-10-25，主任口径）⑧**：**归 agent 仓**（core 只保留 `mining.Proposer` 接口作为冻结契约 + 跨仓任务索引一条指针，**不镜像正文**）。**依据**：DBT-18 ② 原文「⇒ 本单**主体不在 core**」；③ ② 原文给了归 agent 时的处置；④ 原文「core 不引入 LLM SDK；**不在本仓实现第二个编排者（ADR-023 禁）**」。**若归 core 的那条路不采纳**——与 ADR-023 直接冲突。**本仓的动作**：立自己的六字段单（含 LLM/MCTS Proposer 的实现位置）。裁决全文见 `tellurion:docs/design/qt-decoupling/cross-repo-decision-record-2026-10-25.md` ⑧。

> **✅ 施工记录（2026-10-25，G8 / agent 腿，提交 `55dbf05`）**
>
> **交付**
> - `pkg/aistore/schema.go`：`ai.generation_chain` 幂等 DDL（主键 `(job_id, attempt)`、`build_state` 带 `CHECK`）+ `Tables` 登记。
> - `pkg/aistore/generation_chain.go`：`GenerationChain` + `UpsertGenerationChain`（整行覆盖）+ `ListGenerationChain`（按 attempt 升序）+ 词表常量。
> - `pkg/copilot/copilot.go`：`ChainSink` / `ChainEntry` / `WithChainSink`（未注入即 fail-closed）+ 逐段写链 + `sha256Hex`；**删除** `loadPluginSymbol`（`plugin.Open`）、硬编码 `"generated-strategy"` / `"copilot-generated"`、`parseUniverse`、以及 `Generate` 的 runner 参数；promote 闸门落在「编译通过」之后那一步。
>   ⚠️ 自审时补的一处（不是新功能，是留痕本身漏的一环）：LLM 修码重试**换下一版之前**没把这一 attempt 的失败结论落链，于是链上只剩 `none`（「这一版要编」），「第一版确实编不过」这个证据被重试覆盖了 —— 而 ①「一次尝试一行」买的就是这个证据。补 `trace` 一行在前，钉子 `TestQT32A_RetryKeepsTheFailedAttemptRow`，牙齿是破坏腿 H。
> - `cmd/ai/chain_sink.go`：装配根适配器（`NewChainSink`；词表映射**认不出就报错**，不静默传库）；`cmd/ai/builders.go` + `main.go` 改注入留痕落点、`registerCopilotRoutes` 去掉 runner。
> - 测试：`pkg/copilot/copilot_chain_test.go`（8 条，含自审后补的 `TestQT32A_RetryKeepsTheFailedAttemptRow`）+ `pkg/aistore/generation_chain_sql_test.go`（4 条静态形状/词表）+ `pkg/aistore/generation_chain_integration_test.go`（3 条真库 opt-in + 1 条不连库）+ `cmd/ai/chain_sink_test.go`（5 条）+ `pkg/aistore/schema_core_drift_test.go` 的 `agentOwnedTables` 豁免与它的四条反证腿。
> - 文档：`docs/db-schema.md`（新表 + 三条边界 + §5 增 6/7/8 项）、`docs/api-schema.md`（§2 两行 fail-closed + 一段字段实况）、`docs/dataflow.md`（② 链路终点改写 + 生成链小节 + 失败模式两行 + 不变量 5/6）、`docs/adr/0002-AI-生成链留痕归本仓-闸门结论不归本仓.md`（+ ADR 索引）、`AGENTS.md` §1 的 copilot 一行。
> - 联动退场：`pkg/ai/pipeline/backtest_runner_alias_test.go` 的 copilot 一侧断言随 `copilot.BacktestRunner` 一起删（本包不再有回测通道），保留 pipeline↔contracts 那条腿并在文件头写明**为什么少了一边**。
>
> **⑤ 验收逐条（真退出码，本机 `~/sdk/go1.25.0`）**
> 1. 留痕：`TestQT32A_TraceCarriesPromptSourceAndSelfConsistentFingerprint` PASS —— 且「链上那份 = 编译时磁盘上那份」是拿执行器记下的实际文件内容对拍的，不在断言里重写去围栏逻辑；`TestQT32A_RetryKeepsTheFailedAttemptRow` PASS —— 链上是**两行**（attempt 0 `failed` 带当时的诊断 + attempt 1 `passed`），重试不覆盖前一次。`pkg/copilot -run TestQT32A` → 8 PASS / 0 FAIL。
> 2. 闸门：`TestQT32A_PromoteGateBlocksEvenAfterSuccessfulBuild` PASS（`build_state=passed` + `blocked_reason=promote_gate` + 终态 `sandbox_rejected` + `backtested==0`）；`TestQT32A_BuildFailureIsNotReportedAsGateBlock` PASS（编译失败不得冒充闸门拦下）。
> 3. 未注入 sink / 写失败：`TestQT32A_NilChainSink_FailsClosedBeforeCallingLLM`、`TestQT32A_TraceWriteFailure_AbortsBeforeLLM` PASS —— 两条都额外断言 **LLM 调用次数为 0**。
> 4. store：`-run 'TestGenerationChain|TestSchema|TestStatements|TestTables'` → **21 PASS / 3 SKIP**（skip 的三条正是需要 `TEST_DATABASE_DSN` 的那三条 —— `UpsertIsIdempotentPerAttempt` / `RejectsUnregisteredBuildState` / `ListUnknownJobIsEmpty`；`RejectsEmptyJobID` 不连库、真的在跑）。
> 5. drift 豁免：`TestSchema_AgentOwnedTablesAreDeclared` PASS（不读 core 源码，单仓也跑）。
> 6. **破坏验证 8 条**（脚本 `.qt32a_break.py` 现场执行后删除；改坏 → 目标用例红 → 逐字节还原 → 核 SHA256 一致。`go test` 退出码直接取，不经管道）：
>    基线 `go test ./pkg/copilot ./pkg/aistore ./cmd/ai -count=1` → `ok / ok / ok`，EXIT=**0**。
>
>    | # | 改坏的东西 | 目标用例 | 红时退出码 | 还原 |
>    |---|---|---|---|---|
>    | A | `entry.BlockedReason = BlockedPromoteGate` → `= ""`（闸门不再留痕） | `TestQT32A_PromoteGateBlocksEvenAfterSuccessfulBuild` | **1** | SHA 一致 |
>    | G | 闸门改走 `stop`（把「未过 promote」那句塞进链的 `build_error`） | 同上，红在「闸门拦下不得占用 build_error」那条断言 | **1** | SHA 一致 |
>    | H | 删掉重试分支里「先落这一 attempt 的失败结论」那五行 | `TestQT32A_RetryKeepsTheFailedAttemptRow`（红在「第 0 次尝试的失败结论必须留在链上」与「那一行要带当时的诊断」） | **1** | SHA 一致 |
>    | B | 生产源码里放回 `const legacyFakeStrategyName = "generated-strategy"` | `TestQT32A_NoFabricatedStrategyPathInProductionSource` | **1** | SHA 一致 |
>    | C | `sha256Hex(code)` → `sha256Hex("fixed-placeholder")`（2 处：静态闸行 + 循环行） | `TestQT32A_TraceCarriesPromptSourceAndSelfConsistentFingerprint` | **1** | SHA 一致 |
>    | D | `trace` 的失败分支取反（`err != nil` → `err == nil`：写失败不再中止，写成功反倒中止） | `TestQT32A_TraceWriteFailure_AbortsBeforeLLM` | **1** | SHA 一致 |
>    | E | `INSERT` 列清单删掉 `blocked_reason` | `TestGenerationChain_InsertCoversEveryColumn` | **1** | SHA 一致 |
>    | F | `agentOwnedTables` 里夹带一张没登记、理由为空的表 | `TestSchema_AgentOwnedTablesAreDeclared` | **1** | SHA 一致 |
>
>    还原后 `git status` 只剩本单的文件，`.bak` / 临时脚本零残留。**注意两条**：G 红的不是「少写了原因」，而是「原因写错了列」；H 红的是「上一版的失败结论被重试覆盖掉」—— 这两类断言只有改坏一次才知道自己有没有牙齿。
>    D 这条**第一次跑的没落**（锚点在文件里有 3 处 `SaveChain` 失败分支，脚本按「恰好 1 处」才动手）：`stop` / `stopAtGate` 那两处的写失败**刻意只记日志**（终态已落定，不能因审计口失灵把 job 改回「还在跑」），所以不能整片短路。改成只取 `trace` 的函数签名当锚点，才只命中 1 处。
> 7. 门禁（真实输出）：`gofmt -l`（本单 16 个 .go 文件，含 5 个新建）→ 空；`go build ./...` → **EXIT=0**；`go vet ./...` → **EXIT=0**；`go test ./... -count=1` → **EXIT=0**，`^ok` 20 个包、`FAIL` 0 行（另 4 个包 no test files）。全仓 `gofmt -l .` 仍命中两个**既有 BOM** 文件（`pkg/ai/drift/detector.go`、`pkg/ai/validation/redundancy.go`），与本次改动无关、未动（QT-17 W7 已在册）。
>    同一结论在**干净 checkout** 上复取一次（core AGENTS §6 那条实测代价：`gofmt -l` 判工作区、CI 判提交出去的内容，两者可以不同）：`git worktree add --detach <tmp> HEAD` 里 `gofmt -l .` → 只列上面那两个既有 BOM 文件、`go build ./...` / `go vet ./...` / `go test ./... -count=1` → **全 EXIT=0**（`^ok` 20 包 / `FAIL` 0），用完 `git worktree remove --force`。
>
> **本单做的技术决定（代价如实）**
> 1. **删掉 `Generate` 的 runner 参数与 `plugin.Open`**，而不是「留着但恒不执行」：留着就是一条看着还能跑回测的假接口。代价 —— `JobResult.StrategyName/BacktestResult/BacktestErr` 与状态 `generated`/`backtest_complete`/`backtest_failed` 今天不可达、`/stats` 的 `backtest_valid`/`acceptance_rate` 恒 0；**字段与取值集合不动**（`tellurion-ui:web/src/api/copilot.ts` 的终态白名单 + `types/api.ts` 在消费，删是跨仓 breaking），A2 填上它们不需要改 wire。这不是回归：那条回测此前也从没跑成（core 未注册那个名字）。
> 2. **留痕是前置**：sink 没装 / 写失败 → LLM 一次都不调。代价 = 库不可用时 copilot 直接不可用（研究路径受影响），换来的是「存在过的生成都查得到」。这与 S3 那条「四项一律 fail-closed，不做本地兜底」同向。
> 3. **一 attempt 一行**：链能回答「最后编进去的是哪一版」，也留得住中间版本；代价是行宽（prompt + 源码全量入库、不压缩）。
> 4. **闸门只按「有没有可核验坐标」拦，不在本仓存 promote 结论**；代价是「拦下的理由」今天只有一种（`promote_gate`），将来 A2 才有第二个判据。
> 5. **`source_sha256` 命名不改**：它不是 `version_hash` / `artifact_hash`，注释与 DDL 都写明；将来接资产服务时**并列加列**，不复用这个名字。
> 6. **`agentOwnedTables` 是给豁免装的护栏，不是给新表开的后门**：core 的表若哪天新增同名表，需要把它从豁免名单移回 `coreToAI`（那条动作会同时让「自持」断言变红，逼人来改）。

---


<a id="QT-32-B"></a>
## QT-32-B · AI 生成物接资产服务，拿真 `version_hash` / `artifact_hash` ✅ 完成（2026-11-10）

> **拆单说明**：本单是 `QT-32` 的 B 腿（A 腿 = 生成链留痕 + 生产者侧闸门，2026-10-25 已收口）。
> 载体口径**不再等裁**：2026-10-25 第三轮裁决 ⑩ 已定默认 `carrier="dsl"`、仅 L3 才 `carrier="wasm"`、`carrier="so"` 删除；
> `artifact_hash` 按 `contracts.engine.StrategyVersionInput{Carrier:"dsl", Source:<生成物>}.Hash()` 口径理解（见 ④ 的实况订正）。
> 本单把 QT-32-A 闸门段那句「本仓今天没有那个坐标可交」变成「本仓交出资产服务签发的坐标」。

- **① 目标**：一次编译通过的 AI 生成物，由本仓**主动登记**到 `tellurion-strategy` 资产服务，拿回**服务端签发**的 `strategy_id` / `version_hash`（产物载体再加 `artifact_hash`），与生成链同一笔落库；登记失败即 fail-closed，不静默放过、不伪造坐标。
- **② 上下文**（开工前逐一读；引用的是**冻结 wire**，不是实现细节）：
  - 资产服务端点（`tellurion-strategy:internal/httpapi/api.go:117-126`、`:182-234`）：
    `POST /api/strategies`（body `{carrier, abi_version, scope, capability, entry}` → **201** `{id}`）、
    `PUT /api/strategies/{id}/source`（body `{source}` → **200** `{version_hash}`，同源码幂等）、
    `POST /api/strategies/{id}/build`（body `{version_hash, toolchain}` → **202** `{job_id}`）、
    `GET /api/strategies/{id}/builds/{job_id}`（→ 200 `BuildJob`，`state ∈ queued|running|succeeded|failed`，成功带 `artifact_hash`，失败带 `error_code`+`error`）。四个端点今天都是 `openRoute`，无鉴权面（同 QT-32-A 台账 :408 的实测）。
  - **关键实况（决定了本单的形状）**：`SubmitBuild` 对 `!def.Carrier.NeedsArtifact()` 的载体**结构性拒绝**（`internal/asset/service.go:227-229`，`ErrUnsupportedCarrier: carrier %s produces no artifact`）。`dsl` 不产字节码 ⇒ **dsl 腿根本没有 `POST .../build` 可调，也没有 `artifact_hash` 可取**。这不是缺陷，是「dsl 是解释执行的文本」的定义。
  - `Definition.Validate()`（`internal/asset/model.go`）：`carrier` 必须合法、`scope ∈ market|symbol`、`entry` 非空；`dsl` 的 `abi_version` 必须为空，`wasm` 必须非空。
  - 本仓现状：`pkg/copilot/copilot.go:573-591` 的闸门段只拦不登记；`ai.generation_chain` 有 `source_sha256` 但**没有**坐标列（QT-32-A 台账 :455 明写「将来接资产服务时并列加列」）。
  - 资产服务**没有 promote 读面**：promote 与回测执行仍不归本仓（③-⑤ 与 ⑥）。
- **③ 要求**：
  1. 新增资产服务客户端（`pkg/ai/client`，wire DTO 本地化、不 import `contracts`）：一次 `Register(ctx, source)` 按「建定义 → 传源码 →（仅产物载体）触发编译并轮询到终态」取回 `strategy_id` / `version_hash` / `artifact_hash`；非 2xx、编译 `failed`、轮询超时、`succeeded` 却无 `artifact_hash` 一律返回错误（错误文案带步骤与 job_id）。
  2. 载体策略：`Dsl` / `Builtin` / `External` **不调** `POST .../build`（调用必被服务端拒），`artifact_hash` 留空；`Wasm` / `So` 必须调 build 并把 `artifact_hash` 取回。未知载体在开工前拒绝。
  3. `pkg/copilot` 增 `AssetRegistrar` 端口 + `WithAssetRegistrar`；**未注入即 fail-closed**（与 chain sink / codeChecker / buildExecutor 同形），顺序固定为「编译通过 → 登记 → promote 闸门」。
  4. 坐标进链：`ai.generation_chain` **并列加列** `strategy_id` / `asset_carrier` / `version_hash` / `artifact_hash`（nullable），INSERT / SELECT / DO UPDATE 三处列表都覆盖；**不加 promote 结论列**。
  5. 登记失败：job 终态仍用 ui 白名单内的 `sandbox_rejected`，原因写进链的 `build_error`（此时 `build_state=passed`、`blocked_reason` 必须为空 —— 登记失败不是闸门拦下）；`Register` 返回的部分坐标（已建定义/已传源码）照样落链。
  6. 装配根：`cmd/ai` 新增 `strategyAssetRegistrar` 适配器（形状同 `chainSink`），`BuildCopilot` 多一个 `assets` 参数；`config/ai-service.yaml` 增 `strategy_asset` 块（`url` 空 = 未注入 = fail-closed）。
- **④ 约束与非目标**：不改 strategy 任何端点语义与错误码；**不调 promote、不跑回测、不存 promote 结论**；不引 `contracts` 依赖、不动 `go.mod` pin；不新建 DSL 校验器（校验归资产服务）；不改生成提示词；不把 `source_sha256` 复用成坐标（见 ③-4）。
  **实况订正（代裁，待主任追认）**：生成侧今天产出的仍是 **Go 源码**（`pkg/ai/prompts.go` 的提示词未改）。按 ⑩ 默认 `carrier="dsl"` 登记意味着「这份文本作为 dsl 载体的源码入库」，但 dsl 的期望形态是可解释配置文本；两者的口径差属生成侧改造（提示词/产出形态）的工作，不在本单偷偷改。本单的处理是：**登记字节 = 生成物原文**，`asset_carrier` 如实记 `dsl`，并在这里点名该差额。
- **⑤ 验收**：
  1. `pkg/ai/client` httptest 假资产服务：dsl 腿**恰两次**请求（建定义 → 传源码），且**零次** build；wasm 腿四步按序、轮询从 `running` 到 `succeeded` 后取回 `artifact_hash`；编译 `failed` → 错误含 `error_code`；`succeeded` 无 hash → 报错；非 2xx → 错误含状态码。
  2. `pkg/copilot`：正常腿终态 `sandbox_rejected` + `blocked_reason=promote_gate` + 链上坐标等于假件返回值 + 登记收到的字节 = 编译时那份；登记失败腿 `blocked_reason` 为空、`build_error` 点名登记失败、部分坐标仍在；未注入腿失败且不写闸门原因。
  3. `pkg/aistore`：DDL / INSERT / SELECT / DO UPDATE 四处理解一致（既有静态护栏自动变红再变绿的破坏腿）。
  4. 破坏验证：① 不写 `version_hash` 列 → copilot 用例红；② dsl 也调 build → 客户端用例红；③ 吞掉 `Register` 错误 → 失败腿红；④ INSERT 漏列 → SQL 形状用例红。均逐字节还原。
  5. 门禁：`gofmt -l` 空、`go build ./...` / `go vet ./...` / `go test ./... -count=1` 全 EXIT=0。
- **⑥ 边界**：只做「生成物 → 资产坐标」这一跳。promote 人工闸门、resolve/promote 的判定、以及回测执行仍归 `tellurion-strategy` / core；ui 不感知坐标（`api-schema` 的 copilot job 形状不变）。

> **施工记录（2026-11-10，本单翻 ✅）**
>
> **落点**：
> - `pkg/ai/client/strategy_asset_client.go`（新）：`StrategyAssetClient.Register` 按「建定义 → 传源码 →（仅产物载体）build + 轮询」取坐标；非 2xx / `failed` / 超时 / `succeeded` 无 `artifact_hash` 一律报错（错误带步骤、状态码、`job_id`、上游 `error_code`）。载体表：`dsl|builtin|external` 不产字节码（不调 build），`wasm|so` 必须调；未知载体在构造期拒绝。
> - `pkg/copilot/copilot.go`：新增 `AssetRegistrar` / `AssetRef` 端口与 `WithAssetRegistrar`；顺序固定「编译通过 → 登记 → promote 闸门」；未注入 → fail-closed（`asset registrar not configured`）；登记失败 → `stop`（终态 `sandbox_rejected`、`build_state=passed`、`blocked_reason` 留空、`build_error` 点名「资产登记失败」），`Register` 返回的部分坐标照样落链。
> - `pkg/aistore`：`ai.generation_chain` 并列四列 `strategy_id` / `asset_carrier` / `version_hash` / `artifact_hash`（可空；既有库靠四条幂等 `ALTER TABLE ... ADD COLUMN IF NOT EXISTS` 补齐），`INSERT / SELECT / DO UPDATE` 三处同步；`cmd/ai/chain_sink.go` 映射、`cmd/ai/asset_registrar.go` 适配器、`cmd/ai/builders.go` 多一个 `assets` 参数、`main.go` 装配。
> - `config/ai-service.yaml` 新增 `strategy_asset` 块（`url` 空 = 未注入 = fail-closed）；`deploy/compose.yaml` 注入 `STRATEGY_ASSET_URL`（默认 `http://host.docker.internal:8090`）。
> - 文档：`docs/db-schema.md`（列 + 三条边界改写）、`docs/dataflow.md`（流向图 + 故障表 + 不变量 7）、`docs/api-schema.md`（闸门文案 + 登记失败一行）、`docs/adr/0002`（补充段）。
>
> **⑤ 验收（真实输出）**：
> - `pkg/ai/client`：`TestStrategyAssetClient_DSLRegistersSourceWithoutBuild`（恰两步、零 build、`abi_version` 不发）、`_WasmBuildsAndPollsToArtifact`（四步 + 两次轮询、build 带 `version_hash`/`toolchain`）、`_BuildFailureCarriesErrorCode`、`_SucceededWithoutArtifactIsError`、`_PollTimeout`、`_Non2xxSurfacesStatusAndBody`、`_RejectsUnknownCarrier`、`_RequiresBaseURL`、`_Defaults`、`_BuildFailureWithoutDetailIsStillReadable` 全绿。
> - `pkg/copilot`：`TestQT32B_RegistersCompiledSourceAndRecordsIssuedCoordinate`（登记字节 == 执行器读到的字节；链上坐标 == 假件返回值；第一行无坐标）、`TestQT32B_RegistrationFailureIsNotGateBlock`（`blocked_reason` 空、部分坐标在、错误带 503）、`TestQT32B_NilRegistrarFailsClosedAfterBuild`（LLM 已调、终态 fail-closed、不污染词表）；既有 QT-32-A 六条回归全绿，闸门用例新增坐标断言。
> - 真库：`TEST_DATABASE_DSN=…/tellurion_agent_qt32b go test ./pkg/aistore -run TestGenerationChain -count=1` 通过（临时库 `DROP/CREATE` 后跑，新列 round-trip：`version_hash` 读回 64 位、`artifact_hash` 读回空；事后已 `DROP DATABASE`）。
> - 门禁：`gofmt -l .` 空、`go build ./...` / `go vet ./...` EXIT=0、`go test ./... -count=1` 仅剩**两条前置既有红**（本文件 :187 已登记：`pkg/ai/client` 的 `/api/v1/listing-windows`、`/api/v1/trading-calendar` 未被 core openapi 登记，属 QT-17 ③ 待另立单，与本单无关；本单新增的 `/api/strategies*` 字面量经同一条护栏均判为已登记）。破坏验证四腿见下。
>
> **破坏验证（逐腿改坏 → 红 → 逐字节还原 → 绿）**：
> 1. 链上不写 `VersionHash`（改成 `entry.VersionHash = ""`）→ `TestQT32B_RegistersCompiledSourceAndRecordsIssuedCoordinate` 红，报 `expected "…"` vs `actual ""`。
> 2. 让 dsl 也走 build（`if !needsArtifact` 改 `if false && !needsArtifact`）→ `TestStrategyAssetClient_DSLRegistersSourceWithoutBuild` 红（请求序列与期望两步不一致，多出 `POST .../build`）。
> 3. 吞掉 `Register` 错误（`if regErr != nil` 改 `if false && regErr != nil`）→ `TestQT32B_RegistrationFailureIsNotGateBlock` 红：`blocked_reason` 变成 `promote_gate`、`build_error` 里没有「资产登记失败」。
> 4. `INSERT` 列清单漏 `version_hash` → `TestGenerationChain_InsertCoversEveryColumn` 红（列集与 DDL 不等）。
> 四腿还原后 `gofmt/build/vet` 0、上述用例复绿，`git diff` 无残留（见本单提交）。
>
> **如实登记（两点，均未缩小目标）**：
> 1. **生成物语言 vs 载体口径（代裁，待主任追认）**：提示词仍产出 Go 源码，按 ⑩ 以 `carrier="dsl"` 登记——登记字节 = 生成物原文。dsl 的期望形态是可解释配置文本，这一差额属**生成侧改造**（提示词/产出形态），本单不顺手改。
> 2. **dsl 在当前资产服务里无法 promote**：`Promote` 要求 `validated` 产物，而 dsl `NeedsArtifact()=false` ⇒ 没有 artifact 可 validate。也就是说本单交出的是**身份坐标**，离「可执行」还差资产服务侧对 dsl 的放行口径（或改走 wasm）。这是策略服务的设计决定，不在本仓擅改；已在本单 ⑥ 与 ADR-0002 补充段点名。

---

<a id="AUTH-3STATE"></a>
## AUTH-3STATE · 鉴权未启用时默认放行（三态落闸，含工具 RBAC）✅ 完成（2026-11-12）

> **台账说明**：本仓 `docs/TASKS.md` 没有本单条目；按主任指示**不新开任务单**，只在归档补这条施工记录。
> cross-ref 锚点：[`docs/api-schema.md` §1.0](../api-schema.md)（三态接口口径）、[`AGENTS.md` §2.1](../AGENTS.md)（硬约束）、`config/ai-service.yaml` 的 `auth` 注释、`cmd/ai/lifecycle.go::decideAuthStartup`。

- **① 缺陷原文**（修复前，`9c048b4^`；语义 = 「没有身份不报错」+「没有鉴权就放行」）：

  ```go
  // cmd/ai/main.go：容器 host=0.0.0.0 时 isLoopbackHost 为假 ⇒ 连回环门都不装
  if authSvc.Enabled() {
      router.Use(authSvc.Middleware())
      // ...
  } else if isLoopbackHost(v.GetString("server.host")) {
      router.Use(loopbackOnly())
  }
  ```

  ```go
  // cmd/ai/handlers_tools.go：工具 RBAC 整体失效
  if h.authSvc == nil || !h.authSvc.Enabled() {
      // Auth not wired or not enabled — open-access mode, same
      // semantics as Service.Middleware().
      return true
  }
  ```

  这类写法等价于「取身份失败不返回 error，而是返回空身份 `("", nil)`；随后 `authorizeTool` 直接 `return true`」——没有身份的请求被当成合法请求继续执行。

- **② 实际放行面**：`deploy/compose.yaml:49-51` 默认 `AUTH_INSECURE=true` 且 `JWT_SECRET` 为空；容器绑 `0.0.0.0` ⇒ 中间件完全缺位。可被匿名调用的写面：`POST /api/copilot/generate`、`POST /api/pipeline/run`、`POST /api/pipeline/adoption`、`POST /api/explore/runs`（及其 `:runID/stop`）、`POST /api/tools/:name`；工具侧连 `save_factor` / `save_strategy` 这类带副作用的执行也不再校验角色。

- **③ 三态裁决**（`cmd/ai/lifecycle.go::decideAuthStartup`，启动期一次定形）：
  1. 有 `JWT_SECRET` → `authModeEnforced`，全闸；
  2. 无 secret + `AUTH_INSECURE=true` + 回环绑定 + 无显式声明 → `authModeUnavailable`：进程能起，**读面（GET/HEAD/OPTIONS）公开，其余方法一律 503 `{"code":"auth_unavailable"}`**，工具执行同样 503（`authorizeTool` 不再 `return true`）；
  3. 无 secret + `AUTH_INSECURE=true` + `AUTH_INSECURE_EXPOSURE=loopback-published` → `authModeOpen`：显式放开，启动 WARN 点名「写面与工具执行无鉴权」；回环绑定时保留逐请求来源门。
  - 「非回环绑定 + 无 secret + 无声明」**拒绝启动**（修掉旧实现「`0.0.0.0` 反而什么门都不装」的反向逻辑）。
  - 第 3 态比字面三态**更严**：保留 ADR-025 的双开关，只给 `AUTH_INSECURE_EXPOSURE` 而不设 `AUTH_INSECURE=true` 不算放开。该加严已获主任认可（2026-11-12：「双开关那档比要求更严，保留」）。

- **④ 三态实测矩阵**（httptest 走真实 `buildRouter` 链）：

  | 态 | 输入 | `POST /api/copilot/generate` | `POST /api/tools/:name` | `GET /api/pipeline/jobs` |
  |---|---|---|---|---|
  | 1 全闸 | `JWT_SECRET` 非空 | 401（无 token）/ 200（admin token） | 按副作用分级 RBAC | 需 token |
  | 2 降级 | 无 secret + `AUTH_INSECURE=true` + `127.0.0.1` + 无声明 | **503 `auth_unavailable`** | **503 `auth_unavailable`** | **200** |
  | 3 放开 | 无 secret + `AUTH_INSECURE=true` + `AUTH_INSECURE_EXPOSURE=loopback-published` | 200（回环来源）/ 403（非回环来源，绑回环时） | 200 | 200 |
  | 拒启 | 无 secret + `AUTH_INSECURE=true` + `0.0.0.0` + 无声明 | 启动即 `Fatal`，不监听 | — | — |

- **⑤ 破坏验证**（改坏 → 红 → 逐字节还原 → 绿，`git status` 干净后复跑）：
  1. `TestBuildRouter_State2_Unavailable`：把写面中间件从 `AbortWithStatusJSON(503)` 改成 `c.Next()` 放行 → 红，报 `expected: 503 actual: 200`，body `{"ok":true}`。
  2. `TestDecideAuthStartup_ThreeStates`：把非回环分支从 `Refuse: true` 改成 `Mode: authModeOpen` → 红，两条 `expected: true actual: false` + `reason=` 为空。
  3. `TestTools_ZeroMode_FailsClosed`：`authorizeTool` 无鉴权分支改回 `return true` → 红，报 `expected: 503 actual: 200`，body `{"result":{"stub":"backtest.run"}}`。
  4. `TestTools_AuthUnavailable_ExecuteReturns503`：同一条改坏 → 红，报 `expected: 503 actual: 200`，body `{"result":{"stub":"save_factor"}}`。
  - 还原后四条全 PASS（`ok tellurion-agent/cmd/ai 0.095s`），并跑全量 `gofmt -l` 空、`go build ./...` / `go vet ./...` / `go test ./... -count=1` 全 EXIT=0。

- **⑥ 收口**：commit `9c048b4`（`9c048b41f10fd61720833db7c9c3552b519b5925`，`fix(ai): 鉴权三态落闸，无 secret 时写面与工具执行 503 auth_unavailable`），13 files changed；**未 push**。`docs/TASKS.md` 保持不动；`SERVICE.md` 未触碰。


<a id="X-DEP-BUMP-CONSUMERS"></a>
### X-DEP-BUMP-CONSUMERS · 消费方依赖抬升（本仓部分，跨仓伞 `tellurion:X-DEP-BUMP-CONSUMERS`）

> **施工记录（2026-11-12）**：
> - **变更**：kit `v0.2.0 → v0.5.0`；`go` directive `1.25.0 → 1.26.11`（kit v0.5.0 要求 `go >= 1.26.11`）；MVS 连带 pgx `v5.7.3 → v5.11.0`、`x/net v0.55.0 → v0.60.0`、`x/text v0.38.0 → v0.42.0`、`x/crypto v0.53.0 → v0.57.0`、`x/sync v0.21.0 → v0.23.0`、`x/sys v0.46.0 → v0.48.0`。contracts 保持 `v0.10.0` 不动。
> - **CI**：`go-version` 由 `1.25` 改为 `1.26.11`；在 `Vet` 之后新增 `Govulncheck` 步（与 core 腿同一做法）。
> - **验收（真实输出）**：`govulncheck ./...` → `No vulnerabilities found.` / `Your code is affected by 0 vulnerabilities.`，EXIT=0；`gofmt -l` 空；`go build/vet/test ./... -count=1` 全绿。
> - **破坏验证（真做真还原）**：`go mod edit` 把 kit 降回 `v0.2.0`、pgx 降回 `v5.7.3` 并 `go mod tidy` ⇒ `GO-2026-5004`、EXIT=1；从备份逐字节还原（`go.mod 4A69119E…84FA`、`go.sum 5A84331F…8F0C`）后复跑 EXIT=0。**过程留痕**：首次还原误用了 bump 前的备份（kit v0.2.0），随即重跑 `go get kit@v0.5.0 + go mod tidy` 回到目标态，哈希与 bump 后一致，未把旧 pin 留在工作树。
> - **补丁（2026-11-12 发现，同批补）**：`go` directive 升到 1.26.11 后，`Dockerfile` 仍是 `golang:1.25-alpine` 且 `GOTOOLCHAIN=local`，`docker build` 在 `go mod download` 处硬失败（`go.mod requires go >= 1.26.11 (running go 1.25.14)`）；builder 升 `golang:1.26.11-alpine`，实建 `docker build -f Dockerfile` 通过。
> - **边界**：只动本仓 `go.mod`/`go.sum`/CI/文档；AGENTS §4 的版本订正单列在 `X-AGENTS-DEPENDENCY-TABLE`。

<a id="X-AGENTS-DEPENDENCY-TABLE"></a>
### X-AGENTS-DEPENDENCY-TABLE · `AGENTS.md` §4 依赖表按实际 `go.mod` 订正（跨仓伞 `tellurion:X-AGENTS-DEPENDENCY-TABLE`）

> **施工记录（2026-11-12）**：
> - **订正前**：表里 contracts 写 `v0.9.0`、kit 写 `v0.2.0`、pgx 写 `v5.7.3`；实际 `go.mod` 为 contracts `v0.10.0`、kit `v0.2.0`、pgx `v5.7.3`。
> - **订正后**：contracts → `v0.10.0`（注明 2026-11-12 按实际 `go.mod` 订正）、kit → `v0.5.0`（随 `X-DEP-BUMP-CONSUMERS`）、pgx → `v5.11.0`（MVS 抬升，消除 GO-2026-5004，并记 `go` directive 1.26.11）。
> - **验收**：逐行与 `go.mod` 对读一致；`go build/vet/test` 全绿（同 `X-DEP-BUMP-CONSUMERS` 提交）。
> - **边界**：只改 `AGENTS.md` §4 表格，不动 §1–§3 与代码。

<a id="X-CI-RACE-STATICCHECK"></a>
### X-CI-RACE-STATICCHECK · CI 补 `-race` 与 `staticcheck`（本仓部分，跨仓伞 `tellurion:X-CI-RACE-STATICCHECK`）

> **施工记录（2026-11-12）**：
> - **改了什么**：`.github/workflows/ci.yml` 在 `Test` 之后新增 `Test（-race，数据竞争即红）` 与 `Staticcheck` 两步；`govulncheck` 门已在 `X-DEP-BUMP-CONSUMERS` 腿落地。
> - **补门前先分诊**（存量 23 条，未加配置基线排除）：
>   - **疑似真 bug 2 条，各自单独 commit**：`pkg/ai/metrics/ic.go:89` SA4010 —— 查证 `ICResult` 无日期字段、调用方不消费，属死计算，`d752477` 删除，非指标静默丢失；`pkg/ai/yaml/generator_test.go:460` SA4006 —— `Validate(minimal)` 的 err 赋而不用，属假绿，`10e282c` 补 `require.NoError` 并断言三段齐全（实测 minimal 本就合法，是补断言不是改实现）。
>   - **死代码与写法 21 条，`d4cbf27` 一并清理**：U1000 删 `chatResult` / `doChatOnce` / `Narrator.model` / `Limiter.mu` / `TPEOptimizer.mu` / `resultWithCurve` / `parseTimeArg` 及两个测试替身未使用字段；S1039 ×4；S1017；ST1005 ×2（`pipeline_test` 断言同步）；SA1029 改用 `testCtxKey`。
>   - **两个被删 `mu` 的并发审计（2026-11-12 补，见本条「并发审计」提交）**：`Limiter` 可能随 Client 被多个 HTTP handler 共享，`Wait/Allow` 并发调用，但唯一可变状态是内部 `*rate.Limiter`（自身并发安全），`Now` 构造后只读——原 `mu` 从未 Lock，删除不引入 race、无需补锁；`TPEOptimizer` 持有非并发安全的 `*rand.Rand`，但每个 `TPEProposer` 独占实例，`Suggest/Optimize` 只在单 goroutine 搜索循环调用（`loop.go` 顺序 for、`mcts_proposer.go` 单次）——同样无需锁。两条结论已写进类型文档注释；若将来并行 rollout 共享实例，必须在 TPE 侧补锁（注释已注明）。
>   - **`runtime.GOROOT` SA1019 用行内豁免**：用途是 PATH 里找不到 `go` 时定位 `<GOROOT>/bin/go`，再跑 `go list -m` 找 conformance 语料目录（**构建/测试路径，不是打日志**）；改 `go env GOROOT` 在 go 不在 PATH 时同样不可用，会丢掉这条兜底。按裁决不用 `staticcheck.conf` 批量排除，行内 `//lint:ignore SA1019 <理由>` 可被 blame 追到。
> - **验收（真实输出）**：`gofmt -l .` 空；`go build/vet ./...` EXIT=0；`staticcheck ./...` EXIT=0；`go test ./... -count=1` 全绿（25 包 ok，无测试删除）。
> - **`-race` 本地覆盖（Docker 容器，非仅 CI）**：`docker run --rm -v <repo>:/w -w /w golang:1.26 sh -c "go test -race ./... -count=1"` → 全部包 ok、EXIT=0。本机 Windows 无 cgo 的限制已用容器绕开，`-race` 不再列为盲区。
> - **边界**：只动本仓 CI/代码/文档；data / strategy / contracts 的同类告警与门由各自伞腿承担。

---

<a id="X-LOW-CLEANUP-agent"></a>
### X-LOW-CLEANUP（agent 腿）· 核 `/api/health` 豁免注册并订正（跨仓伞 `tellurion:X-LOW-CLEANUP`）

> **施工记录（2026-11-12）**：
> - **取证**：ODR-069 W2-F7「命名/分页/死注释残余」与 W2-F5（k8s 探针）都涉及 `/health` 口径；本仓 `aiPublicPaths`（`cmd/ai/main.go`）与 `rateLimitExemptPaths`（`cmd/ai/lifecycle.go`）各有一条 `/api/health`。
> - **核查结论**：本进程只注册 `/health`（存活）、`/readyz` 与 `/api/readyz`（就绪）、`/metrics` 及 `/api/auth/*`；**没有** `/api/health` 路由（全仓注册点 grep 仅豁免表两处 + 注释）。两张表里的 `/api/health` 是照抄 core 的残留。
> - **订正**：两处豁免表都删掉 `/api/health`；`lifecycle.go` 顶部注释由「与 core 逐字一致」改为「按本进程实际注册的探针收敛」；`main.go` 注释写清为什么不收 `/api/health`（豁免一条不存在的路径只会把 404 伪装成「路由在、服务异常」）。
> - **行为影响**：对 `/api/health` 的请求由「免鉴权 + 免限流后 404」变为「免鉴权 → 过限流 → 404」，状态码不变（仍 404），不改变任何已注册端点的鉴权限流姿势。
> - **验收（真实执行）**：`gofmt -l .` 空；`go build ./...` / `go vet ./...` EXIT=0；`go test ./... -count=1` 全 `ok`。
> - **边界**：只改 `cmd/ai/main.go` / `cmd/ai/lifecycle.go` 两张表与注释；不动 auth 中间件、限流参数、路由注册。

---

<a id="AGENT-RUNBOOK-GAP"></a>
### AGENT-RUNBOOK-GAP · 本仓没有 runbook，起停与生成链失败只有 README 零散说明 ✅（2026-11-12）

> 以下六字段为收口前的台账原文，逐字保留：

- **① 目标**：给 agent 一份最小 runbook，让「起不来 / readyz 503 / 生成链失败 / LLM 未配置」有固定入口。
- **② 上下文**：`SERVICE.md` 落地时实测——本仓 `docs/` 无 `runbook.md`；起停写在 `README.md`「启动 ai-service」，生成链失败语义散在 `docs/api-schema.md` §5 与 `docs/dataflow.md`。六个服务仓都有 `docs/runbook.md`，core / agent / ui 三个仓没有。
- **③ 要求**：① 覆盖起停、`/health` 与 `/readyz` 的语义差异、DB 不可达、`ai.*` 迁移未执行、`AI not configured`、`chain sink not configured`、`sandbox_rejected` 四类失败原文；② 每节写「现象 → 先看什么 → 常见原因 → 对应日志 / job 终态」，不重复 `docs/api-schema.md` 的参数表；③ 完成后把 `SERVICE.md` 的 `truth.runbook` 改为该文件路径。
- **④ 约束与非目标**：不写 SLO / 告警规则 / 值班表；不改生成链的 fail-closed 语义；不把 `config/ai-service.yaml` 的键值清单抄进 runbook。
- **⑤ 验收**：按 runbook 从冷机起一次单仓栈成功；四类破坏场景（停 PG / 不给 `AI_API_KEY` / 不装留痕落点 / 故意让静态检查拒）都命中文中登记的失败原文与处置步骤。
- **⑥ 边界**：本仓起停与生成链排查归本仓；core 的能力端点与错误语义归 `tellurion` 的 runbook / openapi。
- **owner**：待定
- **重开条件**：出现第二次「AI 路由失败但只能翻 api-schema 找原因」的排障经历，或接手人变更。

> **施工记录（2026-11-12，`docs(agent): AGENT-RUNBOOK-GAP 补齐 runbook`）**：
> - **新增** `docs/runbook.md`：§1 起停（compose 冷启动 / 宿主机直跑 / 停服）、§2 `/health` vs `/readyz` 语义表、
>   §3 失败排查（DB 不可达、`ai.*` 迁移缺失、`AI not configured`、`chain sink not configured`、
>   `sandbox_rejected` / `build_failed` 分支）、§4 一次生成请求的排查顺序、§5 命令速查。
> - **事实核对（逐项对代码 / Dockerfile / compose）**：端口 `127.0.0.1:8086`、PG 宿主 `15432`→容器 `5432`、
>   库名 `tellurion`；env 名按 viper `AutomaticEnv` + `.`→`_` 规则（`DATABASE_*` / `SERVER_*` / `AUTH_*` /
>   `COPILOT_WORKING_DIR` / `AI_API_KEY` / `AI_API_URL` / `AI_MODEL`）；`CONFIG_PATH` 默认 `config/ai-service.yaml`；
>   `AI_API_URL` 是完整 chat-completions 端点（客户端直接 POST，不再拼路径）。
> - **发现并订正一处代码错误**：`pkg/copilot/copilot.go` 的 `working_dir` 失败文案原写
>   `set it in analysis-service.yaml`（该文件不存在），改为 `ai-service config (default config/ai-service.yaml) ... or via COPILOT_WORKING_DIR`；
>   测试断言只看 `copilot.working_dir`，不受影响。
> - **报告但不改的既有不一致**（超出本单边界，登记待办）：`SERVICE.md` §1 称因子工具会打 core
>   `/api/factor/compute|evaluate`，与 `cmd/ai/main.go` 注释（ADR-041：core 从未有这两个端点，工具直接 unavailable）不一致；
>   `deploy/wait-for-deps.sh` 仍提示 `tools/local-infra.sh`（本仓不存在该脚本）。两者均不影响 runbook 命令可执行性。
> - **验收（真实执行）**：
>   1. 冷启动单仓栈：`docker compose -p tellurion-agent-runbook-verify -f deploy/compose.yaml up --build -d --wait`
>      两服务 healthy；`/health` 200、`/readyz` 200；
>   2. 停 PG：`docker compose ... stop postgres` ⇒ `/health` 200、`/readyz` 503 且 `checks.database` 含
>      `aistore: ping failed`；`start postgres` 后 `/readyz` 回 200；
>   3. 不给 `AI_API_KEY`：`POST /api/copilot/generate` ⇒ 503，body 含
>      `AI not configured (set AI_API_KEY and AI_API_URL)`，不建 job；
>   4. `ai.*` 迁移缺失：`DROP TABLE ai.experiments CASCADE` ⇒ `/readyz` 503，`checks.database` 含
>      `required relation ai.experiments is missing (startup migration did not run or was rolled back)`；
>      `restart ai-service` 后 `Migrate` 重建、`/readyz` 回 200；
>   5. 留痕落点缺失：`go test ./pkg/copilot -run TestQT32A_NilChainSink_FailsClosedBeforeCallingLLM -v`
>      命中 `chain sink not configured; call WithChainSink() at the composition root`；
>   6. 静态检查拒绝：`go test ./pkg/copilot -run TestCopilotService_Run_StaticcheckRejectsDangerousCode -v` 命中
>      `staticcheck rejected generated code (N finding(s)):`（runbook §3.5 已写明这是 job 级证据）。
> - **门禁（改动后真跑）**：`gofmt -l .` 空；`go build ./...`、`go vet ./...` 均 EXIT=0；
>   `go test ./... -count=1` 全部 `ok`（28 个含测试包，EXIT=0）。
> - **边界**：只改本仓文档 + 上述一处失败文案；不动生成链语义、镜像、compose。
> - **补记（2026-11-12，主任指令，`docs(agent): SERVICE.md 因子工具现状与 wait-for-deps 提示订正`）**：
>   ① `SERVICE.md` §1 的「已知 P0 断边」改为 ADR-041 现状——三个因子工具**不发 HTTP**、直接返回
>   `ErrFactorCapabilityUnavailable`；core 只注册治理面 `POST /api/factor/registrations`，
>   `GET /api/factors/:name` 已退役为 `410 factor_http_surface_retired`；替代路径 `backtest.run` 读结果。
>   依据：ADR-041 + core 路由只读核对（2026-11-12，HEAD `9858dd6`）。
>   ② `deploy/wait-for-deps.sh` 超时提示原指向不存在的 `tools/local-infra.sh`，改为可照抄的
>   `docker compose -f deploy/compose.yaml up -d postgres`（脚本逻辑未动，`bash -n` EXIT=0）。
> - **同类残留（已列未改，待主任定）**：`config/ai-service.yaml:71-73` 注释仍称三个因子工具经
>   `factor_api_url` 打 analysis 的 `/api/factor/*`（该键现用于 QT-67 日历/上市窗口只读）；
>   `pkg/copilot/copilot.go:285` 与 `copilot_test.go:351` 注释仍提 `analysis-service.yaml`；
>   `SERVICE.md` §5 仍写「`factor_returns` / `ic_analysis` 归属尚未裁决」（ADR-038 已裁归 core）。
> - **残留收口（2026-11-12，`docs(agent): 因子配置/归属与 copilot 注释订正`）**：
>   ① `config/ai-service.yaml` 注释改为「键名历史沿用，现用于 QT-67 日历/上市窗口只读；三个因子工具
>   不再经 HTTP（ADR-041），替代路径 `backtest.run`」，配置键保留；
>   ② `copilot.go:284-285` 与 `copilot_test.go:351` 注释的 `analysis-service.yaml` 改为
>   `config/ai-service.yaml`（并把 `cmd/analysis/main.go` 订正为实际调用点 `cmd/ai/builders.go`）；
>   ③ `SERVICE.md` §5 改为「归属已裁（ADR-038，2026-11-11）：`factor_returns`/`ic_analysis` 归 core，
>   零写者、不迁 data/analytics；`factor_hypothesis` 归 `ai.*`」。
>   门禁：`go build ./...` EXIT=0；`go test ./pkg/copilot/ -count=1` ok（0.273s）。

---

<a id="AGENT-FACTOR-TOOLS-UNAVAILABLE"></a>
### AGENT-FACTOR-TOOLS-UNAVAILABLE · 三个因子工具打向 core 不存在的端点 [P0] ✅

> 以下六字段为收口前的台账原文，逐字保留：

> **施工记录（2026-11-11）**：`pkg/ai/client/factor_client.go` 新增 `ErrFactorCapabilityUnavailable`；`ComputeFactor`/`EvaluateFactor` 不再发 HTTP，直接返回该哨兵；三个工具 Description 加 `[unavailable]` 前缀并指向 ADR-041；`docker-compose.yml` 与 `cmd/ai/main.go` 的错误注释同批订正（core 仓提交）。测试：`TestFactorClient_UnavailableWithoutHTTP`、三个 `*_Execute_Unavailable_NoHTTP`（httptest 计数断言 0 请求）、`TestNoPhantomFactorEndpoints`（幻影端点回潮即红）。破坏验证：临时把 `ComputeFactor` 改回发 HTTP ⇒ 3 条用例红（client 2 + builtin 1），还原复绿。门禁：`gofmt -l` 空、`go build ./...`、`go vet ./...`、`go test ./... -count=1` 全绿。

- **① 目标**：让 `factor.compute` / `factor.evaluate` / `compute_factor_ic` 在「core 无因子求值面」这个事实下**显式返回「该能力当前未提供」**，而不是发一个必失败的 HTTP 请求或返回空值；裁决见跨仓 ADR-041。
- **② 上下文**：`pkg/ai/client/factor_client.go:51,93` 调 `POST /api/factor/compute` 与 `/api/factor/evaluate`；`compute_factor_ic` 走同一 `EvaluateFactor`（`pkg/ai/tools/builtin/factor_tools.go:336`）。2026-11-11 实测：core 只注册 `POST /api/factor/registrations`（`cmd/analysis/handlers_factor_registration.go:65`）与反代 `GET /api/factors/:factor_name`（`handlers_proxy.go:167`，已是死链）；`docs/openapi.yaml` 也只有这两条；`docs/SPEC.md:3197-3210` 列的 `compute-returns`/`compute-ic` **代码里从未实现**。链路上还有两处错误事实传播：`docker-compose.yml` 与 `cmd/ai/main.go:100-101` 的注释称「`/api/factor/*` 只在 core 注册」。
- **③ 要求**：① `FactorClient` 的三个方法改为返回**稳定的「能力未提供」错误**（哨兵 + 明确文案「core 当前无因子求值/IC 求值端点；ADR-041」），不发起 HTTP；② 三个工具的 Description 与注册状态标为 `unavailable`（说明原因与替代路径：提交回测读结果），避免 LLM 反复调用必失败工具；③ `pkg/ai/client/endpoint_contract_test.go` 的 allowlist 改成**断言显式不可用**：core 将来真提供端点则红、工具偷偷改回发请求也红；④ 同批订正 `docker-compose.yml` 与 `cmd/ai/main.go` 的注释；⑤ `tellurion:docs/SPEC.md` 因子端点节标注「历史契约 / 当前未实现」（该处属 core 仓文档，跨仓单见 `tellurion:CORE-SPEC-FACTOR-ENDPOINTS-STALE`）。
- **④ 约束与非目标**：**禁止**在 core 补 `/api/factor/compute|evaluate` 空壳端点；**禁止**为了让测试变绿改 allowlist 或放宽断言；不改 DSL 语义、不新造远程求值面（ADR-028 §6）。
- **⑤ 验收**：`gofmt -l` 空、`go build ./...`、`go vet ./...`、`go test ./... -count=1` 全绿；三条工具调用各有一条用例断言返回「未提供」且**零 HTTP 请求**（用 `httptest` 计数为 0 证明）；破坏验证：把哨兵错误改回「发请求」⇒ 用例红；还原复绿。
- **⑥ 边界**：本仓 `pkg/ai/client` 与 `pkg/ai/tools/builtin`；core 侧只改文档标注（另单），不新增运行时能力。
- **重开条件**：core 真正实现「公式 × 符号」求值面、或 ADR-028 §6 的 strategy 侧求值落地并提供稳定 HTTP 面（ADR-041 §6）——届时重新接线并在 ADR-041 追加裁决。

<a id="AGENT-HERMES-DOCS-ADR041"></a>
### AGENT-HERMES-DOCS-ADR041 · Hermes 文档/技能仍把已不可用的因子工具写成现役 [P1] ✅

> 以下六字段为收口前的台账原文，逐字保留：

> **施工记录（2026-11-12）**：`docs/hermes/tools-quant-backtest.yaml`（三工具 Description 加 `[unavailable]` + ADR-041、`validate_factor`/`save_factor` 的 L2 表述同步订正）、`docs/hermes/prompts/quant-research.md`（Capabilities、硬约束第 1/2 条、工作流步骤 4、示例步骤 4/8）、`docs/hermes/skills/autonomous_factor_mining.md`（Step 4 改为「未提供，跳过」、验收标准第 4 条）、`docs/hermes/e2e-acceptance-test.md`（成功条件第 5 条、轨迹断言集合、排障节）统一改为「未提供（ADR-041），改走 backtest.run 读结果」，与 `pkg/ai/client/factor_client.go` 的哨兵一致。验收：`grep -rn "factor.compute\|factor.evaluate\|compute_factor_ic" docs/hermes` 无「现役/可用」表述，全部带 unavailable/ADR-041 标注；纯文档改动，Go 门禁不受影响（`go build ./...` 通过）。

- **① 目标**：把 Hermes 配置/提示词/技能里对三个因子工具（`factor.compute`/`factor.evaluate`/`compute_factor_ic`）的「现役」表述订正为「未提供（ADR-041）」，与 `AGENT-FACTOR-TOOLS-UNAVAILABLE` 已落的代码事实对齐。
- **② 上下文**：审查发现 W1-F2（ODR-068）。`docs/hermes/tools-quant-backtest.yaml:62-107,122-137`、`docs/hermes/prompts/quant-research.md:21,26-27,109,192`、`docs/hermes/skills/autonomous_factor_mining.md:89,284` 仍把因子求值工具写成可用；`8ace9b9` 已改三工具代码（返回 `ErrFactorCapabilityUnavailable`），文档未跟上。裁决见跨仓 ADR-041。
- **③ 要求**：① 三处文档逐条标「未提供 / ADR-041」并给替代路径（提交回测读结果）；② 技能里「调用因子求值」的步骤改为「不可用，改走回测」；③ 不改工具代码（已落）。
- **④ 约束与非目标**：不恢复因子求值端点；不改 DSL 语义；不新造远程求值面（ADR-028 §6）。
- **⑤ 验收**：grep 三处文档无「因子求值工具现役」表述；`check_doc_links`（若适用）绿；与代码哨兵一致。
- **⑥ 边界**：本仓 `docs/hermes/`；core 侧端点事实归 `tellurion:CORE-SPEC-FACTOR-ENDPOINTS-STALE`。
- **owner**：待定

<a id="AGENT-EXPERIMENT-COUNT-SURFACE"></a>
### AGENT-EXPERIMENT-COUNT-SURFACE · 实验计数是内存 seq+1，回测/走查工具零实验行 [P1] ✅

> 以下六字段为收口前的台账原文，逐字保留：

> **施工记录（2026-11-12）**：计数不另立表，直接由 `ai.experiments` 行派生（生产单写者仍在 agent）。`pkg/aistore/experiment.go` 新增 `TrialStats` / `ExperimentTrialStats(runID)` / `CountExperiments()`；`pkg/ai/loop` 新增 `TrialCounter` 端口与 `WithTrialCounter`，`Run` 开场先读已落库计数（读失败整轮拒绝开跑），`NumTrials = 基数 + seq + 1`，`Attempt`/`AttemptView` 暴露 `num_trials`；`pkg/ai/tools/builtin` 新增 `ExperimentRecorder` 端口，`backtest.run` 与 `walk_forward_validate` 每次调用（成功/失败）都写一行并收尾；`cmd/ai` 把 `*aistore.Store` 经 `builders.go` 接入工具、经 `handlers_explore.go` 接入 loop，并新增读面 `GET /api/pipeline/experiments/count`（未接 → 503，不返回 0）。`docs/api-schema.md` 同步。
>
> **验收（真实输出）**：`go test ./pkg/ai/loop ./pkg/ai/tools/builtin ./cmd/ai` 全绿；真库腿 `TEST_DATABASE_DSN=...tellurion_agent_qt go test ./cmd/ai -run TestExperimentCount_PersistsAcrossRestart -count=1` → PASS（工具真跑 3 次 +3；新连接池模拟重启后计数不变；读面返回同值）。**破坏验证**：(a) `loop.go` 改回 `NumTrials = seq + 1` → `TestRun_NumTrialsIncludesPersistedBase` 红（expected 8 actual 1），逐字节还原后绿；(b) 集成腿去掉 `.WithExperimentRecorder` → 计数 +0 而非 +3，红；还原后绿。

- **① 目标**：让「跑了多少次实验」有一个**持久、可回读**的计数面，而不是进程内 `seq+1`；让回测/走查工具真写实验行。
- **② 上下文**：审查发现 W1-F8①②（ODR-068）。`pkg/ai/loop/loop.go:235` 的 `NumTrials` = 内存 seq+1（重启归零）；`pkg/ai/tools/builtin/backtest.go:126-154` 与 `walkforward_tool.go:199` 零实验行写入；`docs/db-schema.md:21` 的实验表只写不读。计数不可信 ⇒ 校准/对账无基础。
- **③ 要求**：① 实验计数落 `ai.*` 持久表（可回读、重启不丢）；② 回测/走查工具各写一条实验行（含 run 坐标）；③ 读面暴露累计次数；④ 与 `AGENT-INTERVENTION-ADOPTION-LEDGER` 撞 `ai.*` 表时按单号串行。
- **④ 约束与非目标**：不造假实验行；不改回测引擎语义；计数口径写死（什么算一次实验）。
- **⑤ 验收**：真跑 N 次回测后计数 = N（重启后仍 N）；破坏验证 = 改回内存 seq ⇒ 重启丢计数用例红。真库腿走 `_test`。
- **⑥ 边界**：本仓实验计数与工具写行；校准消费归 `AGENT-INTERVENTION-ADOPTION-LEDGER`。
- **owner**：待定

<a id="AGENT-INTERVENTION-ADOPTION-LEDGER"></a>
### AGENT-INTERVENTION-ADOPTION-LEDGER · 无干预/采纳台账，校准与对账无数据基础 [P1] ✅

> 以下六字段为收口前的台账原文，逐字保留：

> **施工记录（2026-11-12）**：新增 `ai.adoption_ledger`（`pkg/aistore/schema.go`，幂等 DDL + `is_holdout` 索引，drift 护栏 `agentOwnedTables` 登记豁免）；`pkg/aistore/adoption.go` 提供 `InsertAdoptionDecision`（`proposal_id` 唯一，重复返回 `ErrDuplicateAdoption`）与 `ListAdoptionLedger`；新增 `pkg/ai/adoption` 把口径写死：校准集 = 已回填且 `is_holdout=false`，holdout 只单独报、`outcome_hit` 为空单列 pending，校准误差 = |实际命中率−平均声称概率|；`cmd/ai/handlers_pipeline.go` 新增写入面 `POST /api/pipeline/adoption`（缺 `adopted`/置信度越界 → 400，重复 → 409，未接 → 503）与读面 `GET /api/pipeline/calibration`；`docs/db-schema.md`、`docs/api-schema.md` 同步。
>
> **验收（真实输出）**：`go test ./pkg/ai/adoption ./cmd/ai -run "TestCompute|TestRecordAdoption|TestGetCalibration"` PASS；真库 + 真 HTTP 腿 `TEST_DATABASE_DSN=...tellurion_agent_qt go test ./pkg/aistore -run TestStore_AdoptionLedger`（含重复迁移幂等 + 重复提案拒收 + 未回填无 `resolved_at`）与 `go test ./cmd/ai -run TestAdoptionLedger_EndToEnd`（POST 3 条校准 + 1 条 holdout → 命中率 2/3、校准误差 0.0667、holdout 单独 1/1）均 PASS。**破坏验证**：删掉 `Compute` 的 holdout 分支 → `TestCompute_CalibrationAndHoldoutIsolation` 红（sample 4、hit_rate 0.75、holdout 0），逐字节还原后绿。

- **① 目标**：给「人工干预 / 提案采纳 / holdout 对账」一个台账，让 ADR-023 的「校准优于准确率」有可计算的数据基础。
- **② 上下文**：审查发现 W1-F8③（ODR-068）。`ai.*` 无 actor/intervention 列、无采纳表、无 holdout、无对账 job；PRODUCT/ADR-023 要求考核校准误差（声称 60% 应实际命中 50–70%），但今天没有任何持久数据能算它。
- **③ 要求**：① 建干预/采纳台账（actor + 提案 + 采纳与否 + 结果坐标）；② holdout 对账 job 的口径写死（哪些进 holdout、怎么算命中）；③ 校准误差读面；④ 与 `AGENT-EXPERIMENT-COUNT-SURFACE` 撞表按单号串行。
- **④ 约束与非目标**：不伪造采纳数据；不改验证器五维语义；不引入第二个编排者（ADR-023）。
- **⑤ 验收**：真台账写入 + 校准误差可计算（一组已知提案 → 已知命中率）；破坏验证 = 删 holdout 隔离 ⇒ 对账用例红。
- **⑥ 边界**：本仓台账与校准读面；验证器概率输出归 `pkg/ai/validation`。
- **owner**：待定

<a id="AGENT-HYPOTHESIS-FILTER"></a>
### AGENT-HYPOTHESIS-FILTER · 因果维直通弱信号，假设过滤形同虚设 [P2] ✅

> 以下六字段为收口前的台账原文，逐字保留：

> **施工记录（2026-11-12）**：现状核查 —— 审计所指 `pkg/ai/causal/causal.go:34-48` 已重构为 `pkg/ai/validation/causal.go`，「直通弱信号 / `blind.Result=nil` 仍判成立」在 P2-9f 已修：`ValidateCausal` 把 `blind.Result` 物理置 nil 只给叙述者，验证用原始 `Result`；`verifyTheory` 只统计**可验证**预测，`Testable=0` 给固定 0.25 + blocking，证伪预测逐条 blocking；叙述者 nil = 未评估、空理论 = 显式报错。本单补的是**绝对判据用例**（原先只覆盖「1 中 1 不中」，空结果与弱信号没有直测）：`TestValidateCausal_NilResultIsInsufficientEvidence`（`Result=nil` → `testable=0`、概率**恰好 0.25**、必有 blocking、每条预测 `Verifiable=false` 且带原因）、`TestValidateCausal_WeakSignalIsNotAccepted`（4 条可验 1 中 3 不中 → 概率 `(1+1)/(4+2)≈0.333 < 0.5`、必有 blocking）。
>
> **验收（真实输出）**：`go test ./pkg/ai/validation -run "TestValidateCausal|TestAttachCausal"` PASS（8 例）。**破坏验证**：在 `verifyTheory` 顶部直接返回 `Probability=1, Passed=len(Predictions)`（即改回直通）→ 6 个用例红：`空结果下不该有任何可证伪预测，实际 testable=2 passed=2 failed=0`、`应是 4 条可验、1 中 3 不中，实际 testable=4 passed=4 failed=0`、`一条可证伪的都没有，概率应很低，实际 1.000`、`概率应为 0.80，实际 1.000`、`应 1 中 1 不中，实际 passed=2 failed=0`、`只有第三条可证伪，实际 testable=3`；逐字节还原后全包绿（无残留标记）。

- **① 目标**：让因果维对弱信号/空结果有**可判定的过滤**，而不是直通，避免「看起来审过、实际没拦」。
- **② 上下文**：审查发现 W1-F9（ODR-068）。`pkg/ai/causal/causal.go:34-48` 直通弱信号；`:127-129` 在 `blind.Result=nil` 时仍判成立。验证器链的因果维是唯一需要 LLM 的一维（ADR-023），直通会让红队失效。
- **③ 要求**：① 弱信号/空结果显式判「不成立 / 证据不足」，不直通；② `blind.Result=nil` 不得判成立；③ 补过滤判据用例（绝对断言）。
- **④ 约束与非目标**：不改五维确定性校验器；不引入多 agent 投票（ADR-023 拒绝）；过滤阈值写死可判定。
- **⑤ 验收**：弱信号/空结果用例判不成立；破坏验证 = 改回直通 ⇒ 用例红。
- **⑥ 边界**：本仓 `pkg/ai/causal`；叙述者解析测试归 `AGENT-CAUSAL-PARSE-TESTS`。
- **owner**：待定

<a id="AGENT-LLM-BUDGET-GATE"></a>
### AGENT-LLM-BUDGET-GATE · ResearchBudget 零代码，日累计 USD 闸缺失（D4=a）[P1] ✅

> 以下六字段为收口前的台账原文，逐字保留：

> **施工记录（2026-11-12）**：新增 `ai.ai_costs`（`pkg/aistore/schema.go`，`PRIMARY KEY(day, model)`，幂等 DDL + `agentOwnedTables` 豁免）；`pkg/aistore/llm_cost.go` 提供唯一写入口 `AddLLMCost`（upsert 累加 token/USD/calls）与 `DailyLLMCost`（当日 SUM）。新增 `pkg/ai/budget`：日累计闸 `Gate`（`Check` 读账、`Record` 记账、`Spent` 读面），错误码 `llm_budget_exhausted` / `llm_budget_ledger_error`，读账与记账失败都 fail-closed，额度默认 5 USD（对齐 hermes.yaml）可用 `AI_DAILY_BUDGET_USD` 覆盖，非法值回落默认而非「不设闸」。`pkg/ai/client.go` 加 `budgetGate` + `NewClientWithBudget`，`Chat` 委托 `ChatWithUsage` 使所有生产调用都带 usage 记账；闸在限流等待之前检查，被拒时一次 HTTP 都不发。装配根 `cmd/ai/builders.go` 新增 `newLLMClient(store)` 与 `buildPipeline(sink)`，把 copilot、pipeline 的意图解析/代码修正、explore 因果叙述者全部收口到同一个带闸客户端；`docs/db-schema.md`、`docs/hermes/config/hermes.yaml` 注释同步。
>
> **验收（真实输出）**：`go test ./pkg/ai ./pkg/ai/budget ./cmd/ai -run "Budget|Gate"` PASS；真库腿 `TEST_DATABASE_DSN=...tellurion_agent_qt go test ./cmd/ai -run TestLLMBudgetGate`（预置 1 USD 到上限 → 拒调用且 HTTP 命中 0；重建连接池 + Store + Gate 后仍拒；单次百万 token 调用把累计推过上限并落库，重启后新闸仍拒）PASS。**破坏验证**：把 `Gate.Check` 的读账改为常量 0、`Gate.Record` 跳过 `AddLLMCost`（即删掉持久化）→ `pkg/ai/budget` 5 个用例与 `cmd/ai` 两个真库用例全红（`到达日累计上限后必须拒调用` / `百万 token 的调用必须把花费推过上限并落库`），逐字节还原后全绿。

- **① 目标**：把 `hermes.yaml` 声明的预算（`max_cost_per_session`）落成**代码侧日累计 USD 闸 + 持久化**，让 LLM 花费真能被拦住，而不是配置里写着、代码里没人读。
- **② 上下文**：审查发现 W4-F1（ODR-071）+ 裁决 D4=a。`ResearchBudget` 零代码（`docs/hermes/config/hermes.yaml:57,72` `max_cost_per_session:5.0` + skill:257）；`pkg/ai/metrics/cost.go:136` 的 `ai_costs` 零写者；`metrics.go:35` `dailyCosts` 是进程内 map（重启归零）；`ratelimit.go:29` 10/min + `handlers_explore.go:212-214` MaxTries 20 ⇒ 理论 14,400 次/天，无 USD 上限。
- **③ 要求**：① 日累计 USD 闸（代码侧，持久化到 `ai.*`，重启不丢）；② `ai_costs` 真写者（每次 LLM 调用记 token×单价）；③ 超预算 fail-closed（拒后续调用，点名预算键）；④ 与 `AGENT-EXPERIMENT-COUNT-SURFACE` 撞 `ai.*` 按单号串行。
- **④ 约束与非目标**：不硬编码 API key；不改 LLM 提供商；单价口径写死（可配置但默认值明确）。
- **⑤ 验收**：真累计到上限 ⇒ 拒调用（E1，可用假 LLM 计数）；破坏验证 = 删持久化 ⇒ 重启绕闸用例红。
- **⑥ 边界**：本仓预算闸与成本写者；hermes.yaml 声明归 `AGENT-HERMES-DOCS-ADR041` 同批核对。
- **owner**：待定

<a id="AGENT-EXPLORE-STATE-REAP"></a>
### AGENT-EXPLORE-STATE-REAP · explore 运行态纯内存，崩溃即孤儿（D10=b）[P1] ✅

> 以下六字段为收口前的台账原文，逐字保留：

> **施工记录（2026-11-12）**：收尸落点选 `ai.experiments` 的 `running` 行 —— 那才是崩溃后真正遗留、且有观察面的东西（运行态 map 仍按 D10=b 保持易失，不新增持久 run 表）。`pkg/aistore/experiment.go` 新增 `ReapRunningExperimentsBefore(ctx, before, reason)`：`UPDATE ... SET status='failed', error_message=$reason, finished_at=NOW() WHERE status='running' AND created_at < before`，判据写死为「本进程启动前就已存在」；固定原因常量 `ExperimentOrphanReason`；标 failed 而不删行，保留「断在哪」的证据；重复执行返回 0（幂等）。同时修正该文件顶部关于 running 终态语义的注释：running 作为「被叫停」证据只在单进程生命周期内成立，跨重启遗留按孤儿收尸。`cmd/ai/handlers_explore.go` 新增 `reapExploreLeavings`（收尸失败只 WARN、不阻断启动），`cmd/ai/main.go` 在 Store 就绪后、路由注册前调用，`before = time.Now()`。
>
> **验收（真实输出）**：`go test ./cmd/ai ./pkg/aistore -run "Reap"` PASS。真库腿 `TestStore_ReapRunningExperimentsBefore`：插入 3 行（旧 running / 未来 created_at 的 running / 已 completed），收尸后旧行 `running → failed` 且 `error_message=orphaned: ...`、`finished_at` 非空，新行仍 running、completed 行不变；第二次收尸 0 行。装配腿 `TestReapExploreLeavings_*`：真实调用一次、cutoff 与原因原样传入、nil/错误不阻断。**破坏验证**：`reapExploreLeavings` 直接 `return 0` + `ReapRunningExperimentsBefore` 直接 `return 0, nil`（即删收尸）→ `cmd/ai` 与 `pkg/aistore` 的 Reap 用例全红（`expected: 1 actual: 0`、`遗留 running 必须标成 failed` 实际仍 `running`），逐字节还原后全绿（无残留标记）。

- **① 目标**：给 explore 运行态一个**启动收尸**机制，让进程崩溃后遗留的 running 态能被回收，而不是永远卡在内存 map 里。
- **② 上下文**：审查发现 W4-F2（ODR-071）+ 裁决 D10=b（run 态易失 + 启动收尸）。`pkg/ai/handlers/handlers_explore.go:262-272` 运行态是内存 map；全仓 grep `stale`/`reap`/`recover`/`orphan` 零命中；`pipeline.go:759-764` 尽力而为日志。D10=b：接受 run 态易失，但启动时须收尸（标记遗留 running 为 failed/orphan）。
- **③ 要求**：① 启动期扫描遗留 running 态并标记 orphan/failed（收尸）；② 收尸口径写死（什么算遗留、怎么标）；③ 不引入跨请求持久 run 态（D10=b 明确易失）。
- **④ 约束与非目标**：不把 explore 态做成持久队列（那是 orchestration）；不改 pipeline 语义；收尸不静默丢日志。
- **⑤ 验收**：模拟崩溃遗留 → 重启后收尸标记可见；破坏验证 = 删收尸 ⇒ 遗留态永卡用例红。
- **⑥ 边界**：本仓 explore 启动收尸；持久 job 队列归 `tellurion-orchestration`。
- **owner**：待定

<a id="AGENT-CAUSAL-PARSE-TESTS"></a>
### AGENT-CAUSAL-PARSE-TESTS · 因果叙述者解析逻辑无直测，验证器侧用假叙述者 [P3] ✅

> 以下六字段为收口前的台账原文，逐字保留：

> **施工记录（2026-11-12）**：新增 `pkg/ai/causal/narrator_test.go`，对 `parseTheory` 做**纯函数直测**（固定夹具，不触发任何 LLM 调用），四类输入各自有断言：① 正常 —— 裸 JSON、`min`/`max`/`top_k` 逐字段原样读出；② 空 —— 空串、纯空白、`{}`、`null` 全部显式报错且返回 nil 理论；③ malformed —— 非 JSON、截断 JSON 报「JSON 解析失败」；④ 多段 —— 前后夹散文并包在 json 代码块里、仅代码块、JSON 后跟总结、机制为空但预测齐全，均能正确提取。另加 `TestParseTheory_ErrorCarriesOriginalSnippet` 钉住错误信息必须带原文片段，便于线上判断模型到底吐了什么。生产语义未改动：解析失败仍显式报错，不静默回落成空理论。
>
> **验收（真实输出）**：`go test ./pkg/ai/causal -v` PASS（12 个子用例 + 1 个片段用例）。**破坏验证**：同时做两处改坏 —— 删掉花括号提取（`if false` 包住 `Index`/`LastIndex` 段）、`json.Unmarshal` 失败改为返回空理论且无错误（静默回落）→ 7 例红：`空：空字符串`/`空：只有空白`/`malformed：不是_JSON`/`malformed：截断的_JSON`（`应当显式报错，实际返回理论: &{Mechanism: Predictions:[]}`）、`多段：前后夹散文_+_json_代码块`/`多段：JSON_后跟一段总结`（`机制应为 "..."，实际 ""`）、`TestParseTheory_ErrorCarriesOriginalSnippet`（`应当报错`）；逐字节还原后全绿（无残留标记）。

- **① 目标**：给因果叙述者的解析/提取逻辑补**直接单元测试**，而不是只在 validation 侧用假叙述者间接覆盖。
- **② 上下文**：审查发现 W4-F3（ODR-071）。`pkg/ai/causal/narrator.go:121-143` 的提取逻辑无直测；`ValidateCausal` 的测试在 validation 侧用假叙述者，真解析路径（ malformed LLM 输出 / 空叙述 / 多段）未被钉住。
- **③ 要求**：① 对 `narrator.go:121-143` 补表驱动测试（正常/空/malformed/多段）；② 解析失败显式报错，不静默回落；③ 与 `AGENT-HYPOTHESIS-FILTER` 撞 causal 包按单号串行。
- **④ 约束与非目标**：不改叙述者生产语义；不依赖真 LLM（用固定输出夹具）。
- **⑤ 验收**：解析测试覆盖四类输入；破坏验证 = 改坏提取逻辑 ⇒ 用例红。
- **⑥ 边界**：本仓 `pkg/ai/causal/narrator.go` 测试；过滤判据归 `AGENT-HYPOTHESIS-FILTER`。
- **owner**：待定

<a id="AGENT-EXPLORE-INTERVENTIONS"></a>
### AGENT-EXPLORE-INTERVENTIONS · UI 子台账 B-06 的三条干预写面（G2 指方向 / G3 追问 / G4 标记）缺端点 [P1] ✅

- **① 目标**：把 `tellurion-ui:docs/design/stitch_quantlab_ux_redesign/TASKS.md` B-06 与 `ANNOTATIONS.md` §2/§6 里画了三屏、但一直 `disabled` 的三条干预写面补齐：指方向（G2）、追问（G3）、标记（G4），并确认既有 `adoption` / `calibration` 两条读写的契约稳定。
- **② 上下文**：`cmd/ai/handlers_explore.go` 此前只有 `POST /api/explore/runs`、`GET .../{runID}`、`GET .../stream`、`POST .../stop` 四条；`pkg/ai/loop.Controller` 没有方向注入面；`ai.*` 没有存人工标注的表。UI 侧三处上下文入口（候选卡 / 被放弃的尝试 / 结果流）共用 `run_id`（+ `attempt_seq`）坐标。
- **③ 要求与实现**：
  - **G2 指方向**：`POST /api/explore/runs/{runID}/steer`，`direction`（≤1000 rune）+ 可选 `attempt_seq`。`loop.Controller` 新增 `Steer/Directions/applyDirections`：方向只影响**后续**尝试，并进描述（底座约束）与 `hypothesis`（落 `ai.experiments` 可查）；已结束 run 返回 `409`，不接受永远不会生效的承诺。commit `76c98ee`。
  - **G3 追问**：`POST /api/explore/runs/{runID}/attempts/{seq}/ask`，`question`（≤500 rune）。答案只从该次尝试自身的证据拼装（错误、参数、验证器裁决），`source:"evidence"`，不额外调模型、不编造该次从未跑过的结果；参数按键排序保证可复现；无裁决时不伪造 0 分裁决。commit `9b4e381`。
  - **G4 标记**：`POST /api/explore/runs/{runID}/attempts/{seq}/mark`，`kind` 仅 `derive`/`promote`，`note` 自由文本（≤2000 rune）。新增自持表 `ai.explore_marks`（`run_id` + `attempt_seq` 与 `ai.experiments` 同坐标，DDL CHECK 闭合 kind 词表，索引 `(run_id, attempt_seq)`）；不是裁决、不参与校准、同一坐标允许重复标记；写面要求内存运行态里 run/seq 都存在（否则 `404`），存储未接 `503`。commit `4d27919`。
  - **adoption / calibration 契约确认**：`POST /api/pipeline/adoption` 的 201 从 `{id, proposal_id}` 扩展为整条决策回显（含 `intervention` 自由文本与 `claimed_confidence` 概率快照，`is_holdout` 恒显式）；`GET /api/pipeline/calibration` 的 `Report` 增加 `entries` 原样快照（含 pending/holdout 行），校准口径不变。commit `2f4b8a1`。
- **④ 契约形状（openapi 以 core `docs/openapi.yaml` 定稿为准，本仓只写实现）**：
  - `POST /api/explore/runs/{runID}/steer` req `{direction:string!, attempt_seq:int?}` → 200 `{run_id, accepted:true, count:int, directions:[{text, attempt_seq?, at}], applied_to:"next_attempt"}`；400/404/409。
  - `POST /api/explore/runs/{runID}/attempts/{seq}/ask` req `{question:string!}` → 200 `{run_id, seq, question, answer, source:"evidence", evidence:{hypothesis, params, experiment_id, num_trials, ok, error?, sharpe, verdict_probability?, blocking, challenges:[]}}`；400/404。
  - `POST /api/explore/runs/{runID}/attempts/{seq}/mark` req `{kind:"derive"|"promote"!, note?, actor?}` → 201 `{id, run_id, attempt_seq, kind, note, actor, experiment_id, created_at}`；400/404/503。
  - `POST /api/pipeline/adoption` → 201 增加 `actor / intervention / adopted / claimed_confidence / is_holdout / outcome_hit / result_ref`；`GET /api/pipeline/calibration` → 增加 `entries: AdoptionDecision[]`。
- **⑤ 验收（真实输出）**：`go test ./pkg/ai/loop ./cmd/ai ./pkg/aistore` 全绿；真库腿 `TestStore_ExploreMark`、`TestAdoptionLedger_EndToEnd` PASS（DSN=`.../tellurion_agent_qt`）。每条端点都有契约测试：G2 覆盖注入只影响后续/空超长缺字段 400/未知 run 404/结束 run 409；G3 覆盖失败尝试答案、blocking 证据、404/400；G4 覆盖落库字段/note 去空白/重复标记/kind 与坐标拒绝/503/三入口同坐标。**破坏验证**（逐条改坏即红、还原复绿、无残留标记）：G2 去掉 `applyDirections` 调用 → 控制器与端点用例红；G3 让 `explainAttempt` 返回空答案 → 追问用例红；G4 去掉 kind 校验 → `kind=archive` 意外 201，用例红。
- **⑥ 边界与已知限制**：openapi 以 core `docs/openapi.yaml` 定稿为准，本仓只写实现并回报形状；G3 当前只用证据拼装（未接 LLM 个性化回答）；G4 严格要求内存运行态存在，重启后旧 run 不可再标（标注本身持久，读取面由后续 UI 读 `ai.explore_marks`）。

<a id="AGENT-REVIEW-PENDING-SOURCE"></a>
### AGENT-REVIEW-PENDING-SOURCE · 顶栏「待审阅 N」的 agent 上游读面 [P1] ✅

- **① 目标**：core 已定稿 `GET /api/review/pending` 多源聚合（`CORE-REVIEW-PENDING-AGGREGATE`），其 `ai_explore` 源要求 agent 提供 `GET {ai_service.url}/api/explore/review/pending` → `200 {"pending": <int>, "as_of": "<RFC3339>"}`。本条补齐该上游读面。
- **② 契约来源（先确认再实现，不自行发明）**：`tellurion:docs/openapi.yaml`（`/api/review/pending` 的 description 明写上游契约）与 `tellurion:cmd/analysis/handlers_review_pending.go`（`aiReviewPendingResponse` 只认 `pending` / `as_of`；非 200、缺字段、负数一律转 `unavailable`）。
- **③ 实现**：
  - `pkg/aistore/adoption.go` 新增 `CountPendingReviews`：`SELECT count(*) FROM ai.adoption_ledger WHERE outcome_hit IS NULL`。
  - `cmd/ai/handlers_explore.go` 新增 `GET /api/explore/review/pending` 与 `reviewPendingSource` 接口，装配从 sink 上探得 `*aistore.Store` 后注入；响应 `{pending, as_of}`，`as_of` 统一 UTC。
  - 错误面：存储未接 → `503`；取数失败 → `500`；真的 0 条 → `200 {"pending":0}`（不让 core 误判为「不知道」）。
- **④ 口径与边界（须产品确认的一条）**：`pending` 只数**采纳台账里结果未回填**（`outcome_hit IS NULL`）的行 —— 这是本仓唯一持久、可机器判定的待办队列。探索候选的运行态是纯内存（D10=b），且 `ai.experiments` 与 `ai.adoption_ledger` 之间没有稳定关联键（`result_ref` 是自由文本），把它算进来只能靠猜测的 join，会让角标时真时假。若产品要的是「未采纳候选数」，需先补持久候选索引 + 关联键，另立单。
- **⑤ 验收（真实输出）**：`go test ./cmd/ai -run ReviewPending` 4 组 PASS（形状、真 0 是数字、未接 503、取数失败 500）；真库腿 `TestStore_CountPendingReviews` PASS（前后差值断言 = +2，已回填行不计入）。**破坏验证**：SQL 去掉 `WHERE outcome_hit IS NULL` → 真库腿红；JSON tag `pending` 改成别的 → 契约用例红；均还原复绿、无残留标记。**镜像重建后可达性（2026-11-13 复核）**：`docker build -t tellurion-agent:verify .` 后单容器起服务（`-p 18086:8086`，`DATABASE_URL` 指向本机 PG），`curl /api/explore/review/pending` → `200 {"pending":0,"as_of":"..."}`；同一容器 `/api/explore/review/nope` → `404`、`/health` → `200`。core 隔离栈探到的 404 来自旧镜像，需重建 ai-service 镜像后重试。
- **⑥ 契约形状（与 core 已定稿一致，无需 contracts 新增）**：`GET /api/explore/review/pending` → `200 {"pending": int, "as_of": RFC3339}`；`503`/`500` 由 core 编码为源 `unavailable`。

<a id="AGENT-ADOPTION-DECISION-JSON-TAGS"></a>
### AGENT-ADOPTION-DECISION-JSON-TAGS · `AdoptionDecision` 序列化键名与文档不一致 [P1] ✅

- **① 目标**：contracts 形状核对发现 `aistore.AdoptionDecision` 没有 json tag，`GET /api/pipeline/calibration` 的 `entries[]` 实际序列化为 `ID` / `ProposalID` / `CreatedAt` 等 PascalCase，而 `docs/api-schema.md` 与 `POST /api/pipeline/adoption` 的 201 回显都是 snake_case。
- **② 外部消费约定核查**：全仓检索 `AdoptionDecision` 的序列化点只有两处 —— `POST /api/pipeline/adoption` 的 201（`gin.H`，本就是 snake_case）与 `calibration.Report.Entries`（直接序列化结构体）；`tellurion-ui` 当前无任何 `AdoptionDecision` 字段消费（`claimed_confidence` / `result_ref` 均无命中）。**不存在必须保留 PascalCase 的外部约定**，按仓内其它 DTO 惯例统一 snake_case。
- **③ 实现**：给 `AdoptionDecision` 补 11 个 snake_case tag（`id / proposal_id / actor / intervention / adopted / claimed_confidence / is_holdout / outcome_hit / result_ref / created_at / resolved_at`），`outcome_hit`、`resolved_at` 不加 `omitempty` —— 未回填时显式 `null`，与既有「`is_holdout` 恒显式」一致。
- **④ 验收**：新增 `pkg/ai/adoption/calibration_test.go:TestReport_EntriesJSONKeysAreSnakeCase`，对实际 `json.Marshal(Report)` 的 `entries[0]` 键集合做精确断言（排序后逐项相等，PascalCase 混入即红）。`go test ./pkg/aistore ./pkg/ai/adoption ./cmd/ai` 全绿。**破坏验证**：把 `proposal_id` 的 tag 改成 `proposalId` → 该用例红；还原复绿、无残留标记。
- **⑤ 契约形状（不变，只是实际输出归位）**：`calibration.entries[]` 每条键名固定为上述 11 个 snake_case；`adoption` 201 回显字段不变。无 contracts 仓改动需求。

---

## 2026-11-13 收口

<a id="AGENT-PROBE-HEALTHZ"></a>
### AGENT-PROBE-HEALTHZ · 增加 `/healthz` 并保留 `/health` 兼容别名 [P2] ✅ 2026-11-13

> 档 3（实现）· 来源：02-implementation P2-1 · 顺序 3-8 · 依赖：core 伞 `tellurion:docs/TASKS.md` → `X-PROBE-CANONICAL-HEALTHZ`

- **① 目标**：agent 探针名与其他 11 仓统一为 `/healthz`，`/health` 在兼容期内保留。
- **② 上下文**：`cmd/ai/main.go:128` 只注册 `GET /health`；`cmd/ai/lifecycle.go:143` 的免鉴权白名单写死 `/health`；`docker-compose.yml` 的 healthcheck 也按 `/health` 配置。
- **③ 要求**：① 注册 `GET /healthz`（与 `/health` 同 handler、同响应体）；② `/health` 保留 deprecated 别名一个周期；③ 同步免鉴权白名单、rate-limit 豁免表、compose healthcheck、`SERVICE.md` 与测试；④ 删除别名时另立单。
- **④ 约束与非目标**：不删 `/health`；不改 `/readyz` / `/api/readyz` 语义；不改鉴权策略。
- **⑤ 验收**：`/healthz` 200 且与 `/health` 同构；白名单无遗漏（未鉴权访问 `/healthz` 200）；compose 健康检查绿；破坏验证 = 删 `/healthz` 注册 ⇒ 用例红。
- **⑥ 边界**：agent server + lifecycle + compose + SERVICE.md。

**施工记录（2026-11-13）**：
- **落点**：`cmd/ai/main.go` 抽出 `registerHealthProbes`，同一 handler 同时注册 `GET /healthz`（规范名）与 `GET /health`（兼容别名，同响应体 `{"service":"ai-service","status":"ok"}`）；`aiPublicPaths` 与 `lifecycle.go` 的 `rateLimitExemptPaths` 两表都收 `/healthz`；`deploy/compose.yaml` healthcheck 改 `http://127.0.0.1:8086/healthz`；`SERVICE.md`、`docs/api-schema.md`、`docs/runbook.md`、`README.md` 统一写成「`/healthz` 规范名，`/health` 兼容别名」；`/readyz` / `/api/readyz` 语义未动。
- **测试**：`cmd/ai/main_test.go` 新增两条——`TestHealthProbesCanonicalAndAlias`（两路 200 且响应体逐字节相同）与 `TestHealthProbesArePublicAndUnthrottled`（两张豁免表都含两路，漏 `/healthz` 即红）。
- **破坏验证（真做真还原）**：删 `router.GET("/healthz", healthz)` 一行 ⇒ `TestHealthProbesCanonicalAndAlias` 红（404）；把 `/healthz` 从 `aiPublicPaths` 摘掉 ⇒ 用例红；均逐字节还原、`git diff` 为空后复绿。
- **未改**：core 侧 compose 健康检查切换归 core（本仓不动 core）；`/health` 别名删除另立单。

<a id="AGENT-README-DEPENDENCY-WORDING"></a>
### AGENT-README-DEPENDENCY-WORDING · README 依赖口径澄清 [P3] ✅ 2026-11-13

> 档 3（实现）· 来源：02-implementation P3-1 · 顺序 3-9 · 依赖：无

- **① 目标**：README 不再让读者以为"单向依赖 core"与"零编译依赖"互相矛盾。
- **② 上下文**：`README.md:9-11` 自述"单向依赖 core"，`:31` 又写"零编译依赖"，中间未说明前者是运行期 HTTP 依赖。
- **③ 要求**：统一表述为"无 Go 编译依赖；运行期通过 HTTP 单向依赖 core 的 `POST /api/backtest` 等端点"，并列出现有 URL 依赖（backtest、explore、copilot 相关）。
- **④ 约束与非目标**：只改 README 文案；不动代码、`AGENTS.md` 依赖表与架构描述。
- **⑤ 验收**：两处口径一致；无"零依赖"这类绝对措辞；链接检查通过。
- **⑥ 边界**：`tellurion-agent/README.md`。

**施工记录（2026-11-13）**：`README.md` 开头改为「运行期通过 HTTP 单向依赖它，**无 Go 编译依赖**」；依赖方向代码块改为 `agent ──HTTP──► core / data-service / strategy-asset（运行期调用，无 Go 编译依赖 / 无 replace）`；新增一段列出全部运行期 URL 依赖——core 的 `POST /api/backtest`、`GET /api/backtest/{id}`、`POST /api/walkforward`、`POST /api/expression/validate`、`GET /api/expression/catalog`、`POST /api/risk/detect_regime`、`GET /api/strategies`，data-service 的 `POST /api/factor/compute`、`POST /api/factor/evaluate`，strategy-asset-service 的 `/api/strategies*` 写面，并明确「零编译依赖」只指 `go.mod` 不 require core、无 `replace`。只动 README，未改代码 / `AGENTS.md` / 架构描述。

<a id="AGENT-CI-GATES"></a>
### AGENT-CI-GATES · CI 起 PG + gofmt 门 + 覆盖率门（16 条 DB 用例不再 skip）[P2] ✅ 2026-11-13

> 档 4（测试）· 来源：03-testing P1-3 / P2-2 / P3-1 · 顺序 4-8 · 依赖：core 伞 `X-COVERAGE-FLOOR` / `X-CI-BASELINE-PIN`

- **① 目标**：CI 真跑 DB 腿（16 条按设计 skip 的用例）、补 gofmt 门与覆盖率门。
- **② 上下文**：Test job 无 `services: postgres` / 无 DSN，库侧用例长期 skip；CI 无 `gofmt -l` 门；无覆盖率门槛。
- **③ 要求**：① 加 `postgres:16` service + healthcheck；② 注入本仓测试 DSN（以测试代码为准确认变量名）；③ Test 前应用本仓迁移；④ 本地无 DSN 仍 skip；⑤ 加 `gofmt -l`（非空即红）；⑥ 覆盖率门 = 实测 - 5。
- **④ 约束与非目标**：不改业务代码；不把 skip 改成假通过；不顺手改探索循环。
- **⑤ 验收**：CI 中 DB 用例 PASS（数量 > 0）；故意留一处未格式化 ⇒ 红；删测试 ⇒ 覆盖率门红；本地无 DSN ⇒ skip 且原因可读。
- **⑥ 边界**：本仓 CI + 测试夹具。

**施工记录（2026-11-13）**：
- **DSN / 迁移口径（实测取得）**：库侧用例统一读 `TEST_DATABASE_DSN`；迁移合法入口 = `pkg/aistore.Migrate`（`pkg/aistore/migrate.go` 是 `ai.*` schema 唯一建表真相），服务启动也调它。CI 迁移步复用现成的幂等用例 `TestMigrate_Integration`，不手写 SQL、不复制 core 脚本。
- **CI 改动**：`.github/workflows/ci.yml` 的 Test job 起 `postgres:16` service（`pg_isready`，5s/5s/10 次）+ job 级注入 `TEST_DATABASE_DSN`（CI 内临时库 `tellurion_agent_test`，**绝不指向部署库**）；新增 `Gofmt` 门（非空 `exit 1`）与 `Migrate（aistore.Migrate）` 步；`Test` 步改 `go test ./... -count=1 -p 1 -coverprofile=coverage.out`；新增 `Coverage gate` 步（`go tool cover -func` 最后一行取 total，解析不到也 `exit 1`，低于 60 即红）；`-race` 步同步加 `-p 1`；`govulncheck` / `staticcheck` 不变。
- **`-p 1` 是实测必需**：真库腿共享同一个 `ai.*` schema，跨包并行互相污染——并行跑时 `TestExperimentCount_PersistsAcrossRestart` 实测 6≠7、`TestRun_RealPipelineAndRealDB` 行数 1≠5；串行后全绿。
- **实测（隔离 PG 16 容器 55432 / 库 `tellurion_agent_test`，未碰部署库 `tellurion`）**：带 DSN `1161 PASS / 9 SKIP`（analysis 联调、表达式对拍等设计性 skip）、覆盖率 `total: 65.6%`（7,202 条语句）；无 DSN `1140 PASS / 30 SKIP`、`62.5%`。单内标题的「16 条」是旧口径，**实测库侧 21 条**由 SKIP 转 PASS，已按实测改正（不改测试凑数）。门 = 65 取整 - 5 = **60%**。
- **破坏 / 负向验证（真做真还原）**：① 删 `pkg/ai/gene_pool/integration_test.go` ⇒ **64.0%**（仍门内，单文件最大影响 1.6 点）；② 删 `pkg/ai/tools/builtin` + `pkg/ai/pipeline` 两包全部测试 ⇒ **46.8% < 60% 门红**，逐字节还原后 `git status` 为空；③ 删到破坏编译的用例 ⇒ 测试步 `EXIT=1`；④ **如实登记**：漏配 DSN 实测 `62.5% > 60%`，本仓这项**不**单独触红（测试面 7,202 条语句），门的定位是防累计漂移 + 拦大块测试丢失。
- **文档**：`AGENTS.md` §2 补 CI 链（gofmt / 迁移 / 覆盖率门）、两条 bullet（DB 腿 21 条 SKIP→PASS、门 60% 的来源与边界）与 `-p 1` 必需说明；`docs/TASKS.md` 三单收口指针、frontmatter 记 `completed-2026-11-13`。
- **门禁**：`gofmt -l .` 无输出；`go build ./...` / `go vet ./...` / `go test ./... -count=1 -p 1`（无 DSN，库侧按设计 skip）全绿；`python -c "import yaml;yaml.safe_load(open('.github/workflows/ci.yml'))"` 解析通过。
