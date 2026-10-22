# TASKS 归档 — tellurion-agent（2026-10-22）

> 规则见 [`README.md`](README.md)：完成即在本仓台账标 ✅，随后整条搬进本目录，**旧代正文不改写**（要订正只在文件头加说明）。
> 本文件是历史证据，不作为当前依据引用；引用前先确认它没被新文档取代。

---

## REV-01-A · 消费侧对齐 refactor 修复传播（wire 契约 + 三值口径）✅ 完成（2026-10-22）

> **OBS-01-R2-A 结案后追加的输入（2026-10-22，开工前先读那单的「实测偏差」四条）**：本仓 `pkg/ai/client` 的请求与响应 DTO 与 core 线上形状**两处漂移**——
> ① 客户端发 `strategy_name`，core 认 `strategy`（`binding:"required"`）⇒ POST 恒 400；
> ② core 同步 POST 是 `contracts.BacktestResponse` 平铺体、无嵌套 `result` 键 ⇒ 200 时 `Result` 恒 nil。
> 三值透传已由 OBS-01-R2-A 修好并用 `Succeeded()`（`status==completed && Result!=nil`）兜住 nil 结果，但**这两条要按契约改本仓侧**（③-1），
> 且 ⑤-2「真请求核对」必须等这两条修完才能在活 core 上过。

> **来源**：`tellurion:docs/design/qt-decoupling/refactor-plan-review-2026-10-20.md` 的 H2（agent↔core wire 契约）与 C1（三值 status 映射）修复传播。规则口径冻结在 `tellurion:docs/TASKS.md` QT-50 ③-④ 与 QT-10 ③-④/QT-15 ⑤（**以彼处为准，本单只落本仓消费侧**）。与 OBS-01-R2-A 联动（那单解决三值透传结构，本单补 wire 契约与新增 invalid_reasons 码的消费口径）。

- **① 目标**：本仓对 core 的 HTTP 消费面按冻结 wire 契约对齐，且能正确判读 C1 修复后新增的 `invalid_reasons` code（`data_gap_skipped` / `session_terminated`），不把 `invalid` 误当成功、不把 `failed` 误当可重试。
- **② 上下文**（开工前逐一读）：
  - wire 契约真相源：core 仓 `tellurion:docs/openapi.yaml`（QT-50 ③-③ 落定后存在；落定前以 QT-50 ③-③ 的 DTO 清单为准）；生成物 `tellurion-contracts:dto/`（**禁手改**，见 `tellurion-contracts:docs/TASKS.md` CT-01）。
  - 三值映射口径（C1）：`completed` / `invalid` / `failed`（冻结于 `tellurion:docs/SPEC.md`）；`invalid` 必带 `invalid_reasons`（code 列表），新增 code `data_gap_skipped`（数据缺口跳天）与 `session_terminated`（session 终止）；`ErrStrategyFault` / `ErrContractBreach` → `failed` + `error`。下游判据：**只接受 `completed` 为成功**。
  - 本仓消费面：`pkg/ai/client/backtest_client.go`（`RunBacktest`/`GetBacktestResult`，见 OBS-01-R2-A ②）、`pkg/ai/loop` 候选评估、Copilot 播报。
- **③ 要求**：
  1. **wire 契约消费**：本仓调 core 的 HTTP 客户端以 `tellurion:docs/openapi.yaml` 为契约源（或经 `tellurion-contracts:dto/` 生成物类型），**禁止手写与 openapi 漂移的私有 DTO**；若发现现有手写 DTO 与契约不一致，按契约改本仓侧，不改契约。
  2. **鉴权消费端 fail-closed**：对 core 的请求按 QT-50 ④ 的 JWT+RBAC 面消费（本仓作为调用方持有/转发 token）；loopback-only insecure 模式仅限显式配置的本地开发，默认 fail-closed（无凭证即拒绝，不静默降级）。
  3. **三值消费口径**：`invalid_reasons` 出现 `data_gap_skipped` / `session_terminated` 时按 **invalid 语义**处理（结果可解释、不可当成功，也不当故障重试）；`failed` + `error`（`ErrStrategyFault`/`ErrContractBreach`）按**故障语义**处理（不重试当成功）；不新增第四值、不自行改判 status。
  4. 与 OBS-01-R2-A 共用 `BacktestOutcome` 小结构透传（若该单先行落地则直接在其上补 code 判读；若本单先行则预留字段，两单合流后无返工）。
- **④ 约束与非目标**：不改 core 侧契约与映射逻辑（C1 修复在 core，本单只消费）；不改前端（ui 仓另立 UI-02）；不借机重构 pipeline；不新增第三方依赖。
- **⑤ 验收**：
  1. 单测（`httptest` 假服务端）：对含 `data_gap_skipped` / `session_terminated` 的 `invalid` 响应判读为非成功非故障；对 `failed` 响应判读为故障；对 `completed` 判读为成功——三者可区分。
  2. **破坏验证**：把 `data_gap_skipped` 误判为成功（或误判为 failed）→ 对应用例红 → 还原绿；把手写 DTO 字段名改错 → 与 openapi 对拍（或类型编译）红 → 还原绿。
  3. `go build ./...`、`go vet ./...`、`go test ./pkg/ai/... -race -count=1` 全绿。
- **⑥ 边界**：只做本仓消费侧对齐与判读口径；wire 契约的制定与 openapi.yaml 落定在 `tellurion:docs/TASKS.md` QT-50；三值映射实现在 core QT-10/QT-15；前端展示口径在 `tellurion-ui:docs/TASKS.md` UI-02。

### 结案记录（2026-10-22）

**交付物**

| 文件 | 内容 |
|---|---|
| `pkg/ai/client/backtest_client.go` | 请求 DTO 改 `strategy`（契约字段）；响应 DTO 改成**三种线上形状**的超集（平铺 200 / 202 `job_id` / job 行 `result`）；新增 `ErrBacktestFailed` 哨兵、`InvalidReasonDataGapSkipped`/`InvalidReasonSessionTerminated` 常量与 `HasInvalidReason()`；新增鉴权面 `WithBearerToken` / `WithInsecureLoopback` 与默认 fail-closed 的 `authorize`；日期按 core 的 `2006-01-02` 解析，读不懂直接报错 |
| `pkg/ai/client/backtest_client_test.go` | 17 条：三值可区分、两个新 code、未知 code 原样保留、failed 的哨兵语义、空 status/第四值 fail-closed、202 pending、**请求线上键集**、**平铺响应指标落位**、坏日期 fail-closed、4xx、GET 两态（结果 status 优先 / 无 status fail-closed）、running、nil receiver、夹具自我核对 |
| `pkg/ai/client/backtest_client_auth_test.go` | 6 条：无凭证不发请求（假 core 零命中）、loopback 显式开启、insecure 只对 loopback、Bearer 头、token 优先、`/health` 不设门槛 |
| `pkg/ai/client/openapi_contract_test.go` | 4 条：请求键 ↔ openapi `BacktestRequest`（含 required 覆盖）、响应键 ↔ `BacktestResponse`/`BacktestJob`/202 内联 schema + 数值指标完整性、缺口 allowlist 防腐、**旧漂移形状必红**（破坏腿长期在案） |
| `pkg/ai/agents/obs01r2a_three_value_test.go` | 夹具改成 core 线上平铺形状；客户端按默认 fail-closed 显式开 loopback 开发模式 |
| `pkg/ai/agents/validate.go` / `pkg/ai/agents/validate_l4.go` / `pkg/ai/pipeline/strategy.go` | 跟进请求字段改名（`StrategyName` → `Strategy`），判读逻辑不变 |
| `docs/TASKS.md` | REV-01-A 标 ✅，六字段正文与结案记录搬进本文件，台账只留指针 |

**③ 要求逐条落点**

| 要求 | 落在哪 |
|---|---|
| ① wire 契约消费 | 请求/响应 DTO 按 `contracts.BacktestRequest` / `contracts.BacktestResponse` + job 行字段重写；`openapi_contract_test.go` 用反射出的 json 键与 core `openapi.yaml` 对拍（请求未知键 / 必填缺失 / 响应未知键 / 数值指标缺失四类红） |
| ② 鉴权 fail-closed | `WithBearerToken` 发 `Authorization: Bearer`；无凭证且未显式开 `WithInsecureLoopback` → 请求**在本地被拒**（假 core 零命中，有断言）；insecure 只对 loopback 生效；`/health` 公开不设门槛 |
| ③ 三值消费口径 | `invalid` 恒 `nil error` + `Succeeded()==false`；两个新 code 有常量与精确匹配的 `HasInvalidReason`，未知 code 原样透传；`failed` 返回 `ErrBacktestFailed` 哨兵（`errors.Is` 可判），与传输错误区分；未新增第四值 |
| ④ 共用 BacktestOutcome | 直接在 OBS-01-R2-A 的 `BacktestOutcome` 上补 `HasInvalidReason`，未新建并行结构 |

**⑤ 验收逐条（真退出码）**

1. **单测**：`go test ./pkg/ai/client/ -count=1` → **0**（27 条用例，含主判据 `TestRunBacktest_ThreeStatusesAreDistinguishable` 三条子腿与 `TestRunBacktest_InvalidReasonCodesAreStable`）。
   调用点侧 `go test ./pkg/ai/agents/ ./pkg/ai/pipeline/ -count=1` → **0**。
2. **破坏验证（现场，真退出码，在独立 worktree 上做，不动工作区）**：
   - 基线：worktree（HEAD + 本单新代码）`go test ./pkg/ai/client/` → **0**；
   - 破坏 A：把请求 tag `json:"strategy"` 改成 `json:"strategy_name"` → **1**，两条腿红：
     「请求缺契约字段 `strategy`」「请求里不许出现 `strategy_name`」+ openapi 对拍红；
   - 破坏 B：把 `Succeeded()` 改成把 `invalid` 也算成功 → **1**，三条腿红：
     `Succeeded() want false got true（status="invalid"）`、`invalid 不能被读成成功`、`job 行 completed + 结果 invalid 绝不能读成成功`；
   - 还原：从工作区拷回原文件 → sha256 与工作区**逐字节一致**（`d868c7d0…`）→ `go test ./pkg/ai/client/` 回到 **0**；`git worktree remove --force` 清理，主工作区零残留。
3. `go build ./...` → **0**；`go vet ./...` → **0**；`gofmt -l`（本单范围）空输出；`go test ./pkg/ai/... -count=1` → **0**（15 个包全绿）。
   ⚠️ **`-race` 这条腿未执行**：本机无 C 编译器（`CGO_ENABLED=1` → `cgo: C compiler "gcc" not found`），与 OBS-01-R2-A 登记的代价相同；新增用例无共享并发状态、无 `time.Sleep`。

**实测发现（写进代码注释与文档的口径，不是缺陷）**

- **openapi 漏登记三个字段**：`contracts.BacktestResponse` 有 `error` / `invalid_reasons` / `universe_max_size`，`tellurion:docs/openapi.yaml` 的 `BacktestResponse` **一个都没登记**（另：`BacktestJob` 也没登记 `error`）。
  本单不能改 core 契约（④），故用 `openapiKnownGaps` allowlist 显式登记缺口，并加 `TestOpenAPI_GapsAreStillGaps`：core 补登记后测试当场红，逼着删 allowlist —— 缺口不会被忘记。
- **GET 的 job 行 status 恒为 `completed`**：core `pkg/storage/backtest_jobs.go:39` 的 `UpdateJobCompleted` 对 invalid 结果也写 `completed`；真正的结论在落盘结果自带的 `status` 上。
  客户端因此定规则：**嵌套结果自带的 status 优先于 job 行状态**；job 行 completed 但结果没记 status → fail-closed（不许用间接信号推断成功）。
  core 侧是否要让 job 表状态反映 invalid 属 QT-15/QT-10，不在本单。
  > **订正（2026-10-22 review，原文保留）**：「恒为 completed」只对**同步**通道成立（`SaveSyncResult` → `UpdateJobCompleted`）。**异步** invalid 走 `UpdateJobFailed`（`pkg/backtest/job/job.go:493`），job 行是 `failed` 且结果体被丢弃——客户端在 GET 上读到的是 failed，不是带 `result.status` 的 completed。规则不变：能读到嵌套 result 时以结果自带 status 为准；**没有 result 时 fail-closed**（review 补的测试 `TestGetBacktestResult_CompletedWithoutResultFailsClosed` 钉住这条）。
- **两个新 code 今天没有生产者**：`data_gap_skipped` / `session_terminated` 在 core 源码里零命中（`tellurion-contracts:docs/API.md:147` 同样登记「今天源码里没有」）。本单只落消费口径与稳定常量，不伪造生产者、不新增第四值。
- **`tellurion-contracts:dto/` 生成物不存在**（契约仓 QT-05 未启动）。③-1 的「或经生成物」路线今天不可用，按同一句的「以 openapi 为契约源」落地：DTO 手写不可避免，但由**对拍测试**钉住（CI 会 checkout core，测试自动找到 `core/docs/openapi.yaml`；单仓 checkout 显式 Skip，用 `TELLURION_OPENAPI` 可在本地指定路径）。
- **鉴权默认 fail-closed 的实际影响面为零**：`NewBacktestClient` 今天在本仓没有生产装配点（只有测试构造），core 的 `/api/backtest` 也尚未挂 JWT 中间件（QT-50 ④ 未落）。接线时必须给 `WithBearerToken` 或显式 `WithInsecureLoopback`；这是刻意的——无凭证静默匿名调用比启动失败更难发现。
- **OBS-01-R2-A 的 ⑤-2「真请求核对」仍未执行**：本单修掉了它的红因（DTO 漂移），但本机 core 未起（需要 PG），没有假装跑过。该项仍归 OBS-01-R2-A 的验收尾巴，具备执行条件后补跑。

**边界**：core 仓零改动（未改 openapi、未改 contracts、未改 job 存储）；ui 仓零改动（展示口径归 UI-02）；未重构 pipeline 结构（`BacktestMetrics` 未动）；未新增第三方依赖（`gopkg.in/yaml.v3` 是既有直接依赖）；`go.mod` / `go.sum` 零变化。
