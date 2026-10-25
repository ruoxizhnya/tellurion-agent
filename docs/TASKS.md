---
status: active
last-verified: 2026-10-25
verified-by: QT-32-A（G8 agent 腿）—— ai.generation_chain 留痕 + copilot 的 promote 闸门；8 条破坏验证改坏即红、还原 SHA 一致；门禁 build/vet/test 全 EXIT=0。此前 QT-17 ④ 停写复核（同日）
---

# TASKS — tellurion-agent 未完成项

> 本仓（AI 实验员层）的任务台账。**六字段规范同 core 仓 `AGENTS.md` §8.5**（`tellurion` 仓）：目标 / 上下文 / 要求 / 约束与非目标 / 验收 / 边界，且须满足「独立 agent 拿着本单 + 它显式引用的冻结契约，不需读其他实现、不需问人，就能完成并自证」。
> 状态：`⬜ 待做` / `🔶 进行中` / `⛔ 阻塞` / `✅ 完成（附日期）`。
> 本文件自 core 台账 OPS-01 改判（2026-10-14）派生——原单前提（core 侧做 explore SSE）因 AI 拆仓失效，实现落此仓。

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
| QT-32-B | 生成物接资产服务（`PUT .../source` + `POST .../build`）拿真 `version_hash` / `artifact_hash` ⬜ **待裁** | `QT-32` | **需协调者裁「AI 生成物的载体」**（wasm + host ABI / so / 维持 ADR-024「只审阅不执行」），见 QT-32-A ⑦-1 |

> **QT-65 证据（2026-10-23，C4 期间发现；2026-10-24 已闭环）**：core QT-11（`67bee39`）把注册表改存构造函数后，`strategy.GlobalRegister(实例)` 注册的共享条目在执行路径被 `Registry.NewInstance` fail-closed 拒绝（`ErrSharedStrategy`）。本仓两个**生产**调用点没跟上：`pkg/ai/pipeline/pipeline.go`（`registerOrConfigure` 的注册分支，ExecuteFromYAML 链路）与 `pkg/ai/yaml/loader.go`（`LoadAndRegister`）。现有测试当时全绿是因为没有用例在注册后真经引擎跑（latent，非当前红）。

> **S1 下游腿（contracts 四包切换）的 agent 侧落点**：import 全替清单见 [`archive/contracts-import-replacement-2026-10-23.md`](archive/contracts-import-replacement-2026-10-23.md)（A4 只列不改产出，**B4 已于 2026-10-23 执行完毕**；临时件已归档）。清单已实测把台账的「48+5+3」订正为 **46+5+3 = 54 位点 / 48 文件**，并写明 core 腿必须先落——否则两处 core/contracts 类型身份冲突会让构建红。

## B4 · agent 侧 contracts import 全替 ✅ 完成（2026-10-23，已归档）

> 六字段正文与执行记录（54 位点 / 48 文件逐条替换、§6 五处文本落点、护栏退场、真退出码）在
> [`archive/contracts-import-replacement-2026-10-23.md`](archive/contracts-import-replacement-2026-10-23.md) §11。
> **对后续的直接后果**：本仓已依赖 `tellurion-contracts` 真实伪版本（无 `replace`）；`replace tellurion => ../tellurion` 仍保留，S3 才删；
> `tellurion-kit` 已 `go get` 但无代码引用，`go mod tidy` 已按 Go 规则移除，B5 之后若需要再按使用方实际引用引入。

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

## REV-01-A · 消费侧对齐 refactor 修复传播（wire 契约 + 三值口径）✅ 完成（2026-10-22，已归档）

> 六字段正文与结案记录（③ 逐条落点、⑤ 验收真退出码、独立 worktree 上的两条破坏腿与逐字节还原凭据、六条实测发现）在
> [`archive/TASKS-2026-10-22-archived.md`](archive/TASKS-2026-10-22-archived.md)。本文件按规则只留未完成项，这里只留指针。
> **对后续的直接后果**：① core 的 `openapi.yaml` 仍漏登记 `error` / `invalid_reasons` / `universe_max_size`，客户端用缺口 allowlist 顶着，
> 补登记后 `TestOpenAPI_GapsAreStillGaps` 会红并提醒删行；② GET 的 job 行 status 恒 `completed`，客户端已改为「结果自带 status 优先」，
> core 侧是否让 job 表反映 invalid 归 QT-15/QT-10；③ UI-02 的展示口径可直接对齐本单的 `StatusInvalid` + `invalid_reasons` 原样透传。

---

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
> - **core 侧**：`pkg/storage/experiments.go` / `research.go` 的导出符号均带 `Deprecated:`，删除期限 **2027-01-31**；`pkg/storage/deprecated_legacy_test.go` 逐符号钉住（core 仓）。
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
> **未做 / 环境限制（如实）**：真库腿 `TestPools_WriteOnlyAiSchema` 本机无法执行——本机无 PG 服务（`127.0.0.1:5432` 拒绝连接）且 Docker daemon 未起，故只证到「无 DSN 时显式 skip」，**未取到真库绿**。有库环境跑 `TEST_DATABASE_DSN=... go test ./pkg/ai/gene_pool -run TestPools_WriteOnlyAiSchema -count=1`。
> **不在本单**：旧表 `public.factor_genes` / `public.strategy_genes` 不删（切换前真库各 0 行、无脏数据），退役随 core QT-05 ④（2027-01-31）。

> #### core 侧交接件（QT-17 ④ 的另一半——本仓**不改 core**，报协调者）
>
> **状态（2026-10-25）**：core 侧已落 —— `experiments.go` / `research.go` 的导出符号带 `Deprecated:` 说明，删除期限登记在 `tellurion:docs/TASKS.md` QT-05 ④；本仓此处保留交接要求作为来源记录。
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

## QT-61（agent 腿）· CI 基线归零——OBS-08 注册表/护栏测试漂移 ✅ 完成（2026-10-21）

> 跨仓同号同单，完整四腿收口记录见 `tellurion:docs/TASKS.md` QT-61；此处只登记本仓腿。

- **要点**：判定 `turnover_rate` **确可被 DSL 求值**（注册于 `pkg/expression/registry.go:222`，Source=activity、DependsOn=market_activity，有物理表 `market_activity`（Migration 056））⇒ **修测试不改注册表**：把 `turnover_rate` 从两项护栏的幻影清单移除。
- **要点（根因与收敛）**：根因是清单在两个测试文件各抄一份、D5 加注册项后没同步删 → 收敛为单一来源 `dslPhantomFields` + `phantomViolations` 的「过时自检」，加 `TestPhantomGuard_DetectsStaleRegistry` 三腿破坏验证。
- **要点（验证）**：本地复现红→绿；首跑 CI 实测（2026-10-21）`7f711e5` 后 agent main 全绿。

---

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
  6. **真库腿本机未跑**：本机无 PG 服务、Docker daemon 未起 ⇒ opt-in 用例只证到「无 `TEST_DATABASE_DSN` 时显式 skip」，不装成通过。
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
