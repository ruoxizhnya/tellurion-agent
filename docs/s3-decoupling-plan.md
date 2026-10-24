---
status: active
last-verified: 2026-10-25
verified-by: B6 拆解（QT-17 S3 前置）—— 15 个 core 实现包逐包归属、动作与子批顺序（实测口径，未改代码）
---

# S3 去 core 实现包 import · 15 包拆解清单

> **本文件回答**：删掉 `replace tellurion => ../tellurion` 之前，本仓还欠哪些活、先后怎么排、哪些根本不该搬。
> **判据（唯一）**：删 `replace` 后 `go build ./...` 仍然绿（R1）。任何一包没有着落，这条就是红的。
> 上游依据：`tellurion-agent:docs/adr/0001-编译期依赖-core-是过渡态.md`；core 侧台账 `tellurion:docs/TASKS.md` QT-07 ⑥。

## 0. 实测口径（2026-10-25）

按 AST / 剥离注释与原始字符串的口径统计（脚本读源码，不进仓库）：

| 指标 | 实测值 | 说明 |
|---|---|---|
| core import path | **17** 个 | 其中 **15 个进入生产代码**，2 个只在 `_test.go`（`strategy/examples`、`storage`） |
| 生产文件 | **22** 个 | 同一文件可能 import 多个 core 包（`validate.go` / `main.go` / `builders.go` / `loader.go`） |
| 测试文件 | **19** 个 | 删 `replace` 前必须一并清零 |
| import 语句 | **59** 处 | 生产 22 + 测试 37 |
| 符号引用（近似） | **230** 处 | 按 `pkg.Symbol` 形态计数，不是调用图 |

> ⚠️ `go.mod` 末尾注记写的是「15 包 / 64 处 import / 43 文件」——那是 `grep` 粗口径：**多出 2 个伪命中**（`pkg/ai/prompts.go` 的 prompt 模板文本里有一行示例 `import "tellurion/pkg/strategy"`；`internal/repoguard/storage_import_test.go` 的护栏常量 `"tellurion/pkg/storage"`）。真实 import 是 59 处 / 41 文件。S3 收尾时注记按本表订正。

## 1. 三类定义（按「用什么机制消掉这条 import」分，不按包名分）

| 类 | 含义 | 判据 |
|---|---|---|
| **HTTP 化** | 功能留在服务端（引擎进程 / data-service），本仓改为网络调用；本地实现删除 | 该能力是热路径、有状态、或必须唯一真相（引擎、注册表、表达式求值） |
| **契约化** | 共享**形状**搬去 `tellurion-contracts`，实现不跨边界 | 该形状被 ≥2 个仓使用（contracts 硬规则） |
| **去重** | 本仓已有等价能力（kit / contracts）或根本不需要这份能力，直接换源或删除 | 无行为变化，或能力归属本仓 |

> 分配原则：**一个包只挂一个主类**；同一包内确有第二类动作的（如 `strategy` 的类型部分、`bootstrap` 的引擎装配部分），在「迁移动作」列里写清拆分。
> 契约化只有 2 个包**不是遗漏**：contracts 的准入门槛是「≥2 个仓使用」。绝大多数 core 包在本仓是「运行期服务能力」（该 HTTP 化）或「已经有 kit 副本」（该去重），把它们塞进 contracts 才是边界腐化。

## 2. 汇总表

| # | core 包 | 类 | 生产/测试文件 | 调用点 | 迁移动作 | 风险 |
|---|---|---|---|---|---|---|
| 1 | `statistics` | 去重 | 5 / 0 | 12 | → `tellurion-kit/statistics`（同名函数：Mean / Pearson / SampleStdDev / SampleVariance / PopulationStdDev / Spearman） | 低：kit 包由 core 同源迁出，仍需逐函数对拍边界（空切片 / 常量序列） |
| 2 | `httpserver` | 去重 | 3 / 0 | 9 | → `tellurion-kit/httpserver`（`Fail` / `Error` / `Wrap` 同名同签名） | 低：错误响应形状必须逐字段对拍（前端只认一种形状） |
| 3 | `observability` | 去重 | 1 / 0 | 1 | → `tellurion-kit/observability`（`Handler`） | 低：仅 `/metrics` 挂载 |
| 4 | `sandbox/runner` | 去重 | 1 / 0 | 11 | → `tellurion-kit/sandbox/runner`（`New` / `Limits` / `Options` / `With*` / `ErrTimeout`） | 低：kit 是 core 该包的迁出副本；Windows Job Object 语义需一条限流用例 |
| 5 | `sandbox/staticcheck` | 去重 | 1 / 1 | 2 | → `tellurion-kit/sandbox/staticcheck`（**kit 已有同函数** `CheckOrError`，纯换源） | 低：包内是纯静态分析，无 I/O |
| 6 | `auth` | 去重 | 1 / 1 | 35 | → `tellurion-kit/auth`（`TokenService.Middleware` / `RequireRole` / `RoleFromContext`）+ 本仓角色常量；**不再读 core 的 `users` 表** | **高**：现实现经 core `auth.NewService` 直连 `users` 表，属数据归属越界；用户库归属已裁（§6：留 core），但迁移面是 handler 鉴权链 + 启动装配 |
| 7 | `bootstrap` | 去重 | 2 / 1 | 19 | 拆两半：**工具腿**（Logger / Metrics / Config / HTTPClient / GinMode）→ kit；**服务生命周期腿**（BuildRouter / StartHTTPServer / Shutdown）本仓自持 ~60 行（kit 是库，不收应用生命周期）；**引擎装配腿**（BuildBacktestEngine / BuildRiskManager / BuildExecutionTrader / InitStore / InitAuth）随 HTTP 化删除 | 中：`BuildRouter` 牵动全部 handler 的中间件顺序，需回归鉴权与错误形状 |
| 8 | `marketdata` | 去重 | 1 / 2 | 4 | 无替代调用：**随进程内引擎装配整体删除**；本仓行情需求已走 `DataSourceClient`（data-service HTTP） | 低：删的是装配代码，不删 data-service 能力 |
| 9 | `strategy/monitor` | 去重 | 1 / 1 | 11 | 纯函数迁入本仓（建议 `pkg/ai/monitor`，与 `pkg/ai/drift` 同层）；类型留在本仓**不进 contracts**（只有本仓用） | 中：滚动指标/告警阈值的边界条件多，需用现有序列用例对拍 |
| 10 | `data/source` | 契约化 | 3 / 3 | 17 | → `tellurion-contracts/dataprotocol`（`UnifiedDataPoint` + `DataType*`）。core 侧该包**已是 contracts 的别名**，形状零变化 | 低：纯 import 替换；因子投影逻辑不变 |
| 11 | `strategy/expression` | 契约化 | 1 / 1 | 17 | 配置形状不再共享：`ExpressionStrategyConfig`/`SignalConfig`/`SizingConfig`/`RiskConfig` 只在引擎侧构造；本仓改为**提交 YAML/参数**（HTTP 请求体里的 `strategy_params`，形状见 `contracts/engine.BacktestRequest`） | 中：YAML → 配置的组装从本仓挪到引擎，须补一条「同 YAML 同结果」对拍 |
| 12 | `strategy` | HTTP 化 | 3 / 6 | 37 | 注册表（`GlobalRegisterFactory` / `GlobalGet` / `GlobalNewInstance` / `GlobalListWithInfo`）→ `GET /api/strategies`（默认分支 `ListWithDB` 已合并**注册表 ∪ DB 活跃项**）/ `POST /api/strategies`；参数经 `contracts/engine.BacktestRequest.StrategyParams`；本仓不再实现 `strategy.Strategy` | 中：`GET /api/strategies/{id}` **只查 DB**（内置策略 404），`strategy.get` 要改成从 list 结果里过滤；prompt 模板里的接口文案也要同步 |
| 13 | `expression` | HTTP 化 | 4 / 6 | 32 | 静态校验 + 算子/字段目录 → core 新端点（core **当前无此类端点**）；本仓不再持有 parser/evaluator | 中：4 个生产文件里 **3 个零生产构造点**（见 §4 的 S3-0），真正在生产跑的只有 `factor_tools.go` |
| 14 | `backtest` | HTTP 化 | 2 / 2 | 16 | → `POST /api/backtest`、`POST /api/walkforward`（已有端点）；删除 `backtest.Engine` / `WalkForwardEngine` 装配与 `cmd/ai` 内的 newEngine 闭包 | **高**：`/api/backtest` 的绑定结构**已嵌入** `contracts.BacktestRequest`，`strategy_ref` / `strategy_params` 运行时就能收（openapi 只是没登记）；但 `walkforward.WalkForwardRequest` 是 core 自有类型、**没有** `strategy_params`，且缺 `x-agent-coarse-action` |
| 15 | `risk` | HTTP 化 | 1 / 2 | 7 | → `POST /api/risk/detect_regime`（已有端点）；删除本地 `RiskManager` 构造 | 中：端点收**内联 OHLCV 数组**，而本仓工具正是为了不把 200+ 行塞进上下文才在本地算；接 HTTP 一次会把省下的 token 还回去 |

**仅测试的 2 个**（删 `replace` 前同样要清零）：

| 包 | 测试文件 | 动作 |
|---|---|---|
| `strategy/examples` | 2 | 用本仓 fake 策略替代（只为进程内集成腿提供样本） |
| `storage` | 1 | `pkg/ai/validation/live_backtest_integration_test.go` 改走 HTTP 回测腿或删除 |

> 上表文件数按包计：同一文件 import 多个 core 包会重复计入（`main.go` 4 个、`builders.go` 4 个、`validate.go` 3 个）；去重后是 **22 个生产文件 / 19 个测试文件**，与 §0 一致。

## 3. 必须留在引擎进程内（不搬）

这些能力**不能**以「搬进 agent」的方式消依赖；只能 HTTP 化，或删掉 agent 侧那份需求。

| 能力 | 为什么留下 | 本仓的正确姿势 |
|---|---|---|
| 回测主循环 / 撮合 / 组合结算 / 现金账户 | 逐 bar、每标的调用；进程化把 ns 级调用变成 ms 级往返（设计文档 `tellurion:docs/design/qt-decoupling/project-engineering-file.md` 已列硬否决） | 提交 job，读结果；绝不本地跑引擎 |
| 表达式解析与逐 bar 求值 | 同上，且 DSL 求值是信号生成的执行路径 | 只做**静态**校验（HTTP），不做逐 bar 求值 |
| 策略运行时实例与注册表工厂 | 每 run 一份实例（QT-65）；共享实例会被引擎 fail-closed 拒成 `ErrSharedStrategy` | 只传参数 / YAML，不注册实例 |
| 风控同步裁决（`CheckOrder` / 逐 bar 的 `DetectRegime`） | 零网络是硬约束（risk 仓 AGENTS §3） | 工具只取**快照**（一次粗粒度调用） |
| 行情读取 provider（PG / adapter） | 引擎热路径；数据面唯一入口是 `tellurion-data` | 走 data-service HTTP（`DataSourceClient` 已有） |
| 编译 / 执行 AI 生成代码的沙箱 | 产物不加载、不执行（ADR-024）；真编译只发生在服务端的校验/构建链路 | 本仓只保留 kit runner 做「编译校验」这一件工具，不搬引擎侧执行 hook |

## 4. S3 子批顺序（每批一个 atomic commit，批内按包再拆）

### S3-0｜先砍死代码（可选前置，能直接缩小依赖面）

实测（2026-10-25，全仓 grep 构造点，排除 `_test.go`）**零生产构造点**的文件：

- `pkg/ai/agents/research.go`（`NewResearchAgent` 无生产调用方；文件自身注释已写明「当前零生产构造点」）
- `pkg/ai/agents/validate.go`（`NewValidateAgent*` 只在测试里构造）
- `pkg/ai/pipeline/discovery.go`（`NewDiscoveryPipeline` 只在 `pipeline_test.go` 构造）
- `pkg/ai/pipeline/strategy.go`（`NewStrategyPipeline` 无调用方）

影响：这四个文件贡献了 `expression` 4 个生产文件里的 3 个、`statistics` 5 个生产文件里的 1 个。若确认删除，S3-c 的实际工作量显著下降。

**为什么单列且不急**：删档要走 ODR（本仓 AGENTS §6「删档 → 一条 ODR」），且 `pkg/ai/agents/doc.go` 目前写的「pipeline 在用这些 agent」与实测不符——先订正文案再删，不要拿"死代码"当免 ODR 的理由。**本文件只登记，不执行删除。**

### S3-a｜纯机械换源（0 跨仓依赖，可立即开工）✅ 完成（2026-10-25）

- 包：`statistics` → kit；`httpserver` → kit；`observability` → kit；`sandbox/runner` → kit；`sandbox/staticcheck` → kit；`data/source` → contracts/dataprotocol。
- 规模：6 包 / 14 个生产文件 / 52 处符号引用。
- 依据：kit `v0.1.0` 已含全部五个 kit 目标包（含 `sandbox/staticcheck`），contracts `v0.6.0` 已含 `dataprotocol`；两者都已在 `go.mod`，**不需要任何发版**；data/source 在 core 侧已是 contracts 别名（形状零变化）。
- 门：`go build/vet/test ./...` 绿；kit 与 core 同名函数逐个对拍（空输入、常量序列、超时）。
- 为什么先做：零行为变化、零外部等待，做完立刻把 40% 的 import 面消掉。

> **施工记录（2026-10-25，commit 随本行落）**：
> - 已换源 **5 个包**：`statistics`（5 文件：`drift/detector.go`、`agents/validate.go`、`metrics/turnover.go`、`metrics/ic.go`、`validation/redundancy.go`）、`httpserver`（3 文件：`handlers_explore.go` / `handlers_copilot.go` / `handlers_pipeline.go`）、`sandbox/runner` + `sandbox/staticcheck`（同 `builders.go`，另 `copilot_test.go` 静态检查腿）、`data/source`（3 生产 + 3 测试：`factor/sentiment.go` / `sector_rotation.go` / `capital_flow.go`，import 改 `contracts/dataprotocol` 且符号前缀同步改名）。
> - **`observability` 未在本批换源（如实登记，不是遗漏）**：唯一调用点是 `cmd/ai/main.go:136` 的 `observability.Handler(m)`，而 `m` 来自 `bootstrap.InitMetrics`（core 类型 `*tellurion/pkg/observability.Metrics`）。kit `Handler` 形参是 `*kit/observability.Metrics`，两个结构体均带私有 `registry`，**不能隐式转换**；若本地另建 kit `Metrics`，`/metrics` 会丢掉 core `Metrics.Register()` 已注册的 14 个族（含 `backtest_duration_seconds` / `llm_tokens_total` / `http_client_requests_total` 等），违反本批「零行为变化」。因此 `observability` 与 `bootstrap` 工具腿（`InitLogger` / `InitMetrics` / `HTTPClient`）一起留到 **S3-d**，届时一次性换 kit。
> - **对拍取证**（临时 harness，不进仓库）：`statistics` 的 `Mean` / `SampleVariance` / `SampleStdDev` / `PopulationStdDev` / `Pearson` / `Spearman` 在空切片、单元素、常量序列、随机序列上与 core 逐值一致；`staticcheck.CheckOrError` 4 组样本返回文案一致；`httpserver` 的 `Fail` / `Error` 状态码与响应体 `{error,code}` 逐字段一致（5xx 原文只进日志）；`sandbox/runner` 在 300ms 超时下与 core 同返回 `ErrTimeout`。
> - **破坏验证**：把 `validation/redundancy.go` 的 `statistics.Pearson` 取反 → `TestRedundancy_NegativeCorrelationIsFlagged` 红；逐字节还原后绿。`go build ./... && go vet ./... && go test ./...` 全绿。
> - import 面：本批清掉 17 处 core import（生产 13 + 测试 4），core 实现包 import 59 → 42；`observability` 1 处留 S3-d。

### S3-b｜本仓内去重（无跨仓依赖）✅ 完成（2026-10-25）

- 包：`strategy/monitor` → `pkg/ai/monitor`（本地纯函数）；`auth` → kit/auth + 本仓角色。
- 规模：2 包 / 2 个生产文件 / 46 处符号引用。
- **auth 的裁决（本文件直接定，不再拖）**：用户库留在 core（单人实验室没有第二套用户体系），本仓**只消费 token claims**（kit/auth 解析 + 角色常量），删除 `bootstrap.InitAuth(v, store, …)` 这条「借 core 连接池读 `users`」的路径。代价：本仓不再能自助建用户；代价可接受，因为建用户本来就不是 agent 的职责。
- 门：工具级用例（`monitor.strategy_health`、角色矩阵 viewer/trader/admin 三条）全绿。
- 说明：**不需要任何发版**——kit 已提供 `auth` 中间件与 `RequireRole`。

> **施工记录（2026-10-25，commit 随本行落）**：
> - `strategy/monitor` → `pkg/ai/monitor`：整包迁入，**可执行逻辑与 core `pkg/strategy/monitor/monitor.go` 零差异**（`git diff --no-index` 仅 10 insertions / 4 deletions，全部是包注释、import 行、两处注释里的路径指代）；唯一实质改动是 `tellurion/pkg/statistics` → `kit/statistics`（S3-a 已对拍六个函数逐值一致）。调用点 `pkg/ai/tools/builtin/strategy_health_tool.go` 与其测试切到本仓包。
> - `auth` → `kit/auth` + 本仓角色常量：删除 `cmd/ai/main.go` 的 `bootstrap.InitAuth(v, store, …)`（借 core 连接池读 `users`），改 `initTokenService` —— `bootstrap.DecideAuthStartup` 启动门保留，token 服务用 `kitauth.NewTokenService`，PublicPaths 与 core 的 `auth.PublicPaths()` 逐字一致；`bootstrap.BuildRouter`（形参是 core `*auth.Service`）换成本仓 `buildRouter`（kit CORS + core bootstrap 的限流/请求日志 + kit 中间件）。`handlers_tools.go` 与其 RBAC 测试改用 `*kitauth.TokenService` 与 `RoleViewer` / `RoleTrader` / `RoleAdmin` 三个本仓常量。
> - **如实登记的行为变化（不是遗漏）**：① 本进程不再写 `audit_logs`（core 表；旧 `AuditMiddleware` 随 `InitAuth` 一起删除），审计写入此后只剩 analysis-service；② 本进程不再能自助建用户（用户库留 core 已决）。两者都是「只消费 token claims」这条裁决的直接后果。
> - **遗留 1 处 core import**：`buildRouter` 的 insecure 形态仍调 `tellurion/pkg/httpserver.LoopbackOnly()` —— kit `v0.1.0` 没有这个中间件，而复制一份安全门会造成两份真相；S3-d 把 router 生命周期收进本仓时再决定「上收 kit」还是「本地自持」。
> - **验收（真实输出）**：`go test ./pkg/ai/tools/builtin -run StrategyHealth -count=1` → 7/7 PASS；`go test ./cmd/ai -run TestTools_ -count=1` → 6/6 PASS（viewer 禁 write / trader 放行 / viewer 可 read / 未分类 fail-closed 到 admin / disabled open-access / list 不设卡）。
> - **破坏验证**：把 `pkg/ai/monitor/monitor.go` 的 `dd := (peak - e) / peak` 改成 `(e - peak) / peak` ⇒ `TestStrategyHealthTool_ComputesRollingDrawdown` 红（`"0" is not greater than "0"`，消息「后段持续下跌，滚动回撤必须为正」）；逐字节还原后绿。
> - **边界**：`grep -rn '"tellurion/pkg/auth"'` 与 `'"tellurion/pkg/strategy/monitor"'` 均**零命中**。core 实现包 import 面：本批前 42 → **39 处**（清掉生产 2 + 测试 2，新增 1 处 `tellurion/pkg/httpserver` 别名，见上）；分布为 **23 个文件 / 10 个包**（生产 10 文件、测试 13 文件）。`go build ./... && go vet ./... && go test ./...` 全绿。

### S3-c｜HTTP 化（依赖 core 端点；子序按「端点已具备 → 端点待补」）

1. **risk** ✅ 完成（2026-10-25）：直接接 `POST /api/risk/detect_regime`。core 的**粗粒度形状已落地**（QT-70 ✅ 2026-10-24：`symbol + start_date + end_date` 由服务端取数，未接线 503 / 无行 404），agent 不必先把 OHLCV 拉进上下文再回传。零 core 改动。
2. **strategy 注册表** ✅ 完成（2026-10-25）：接 `GET /api/strategies`（默认分支已合并注册表 ∪ DB 活跃项，`ListWithDB`）；`strategy.get` 改为从 list 结果过滤（`GET /{id}` 只查 DB）。**零 core 改动**，只需行为对拍。
3. **backtest**：接 `POST /api/backtest` + `POST /api/walkforward`；删除 `cmd/ai` 的 `newEngine` / `pgProvider` / `dataAdapter` / `wfEngine` 装配。`/api/backtest` 运行时已能收 `strategy_ref` / `strategy_params`（绑定结构嵌入 `contracts.BacktestRequest`）；**walkforward 的 `strategy_params` 已由 core QT-68 ✅（2026-10-24）补齐**，且 `POST /api/walkforward` 已登记为 D24 第 8 类粗粒度动作（core 裁-12，2026-10-25）。
4. **expression**：接 core 新端点（静态校验 + 目录），删除 parser/evaluator 依赖。**前置已满足**：QT-69 ✅（2026-10-24）落 `POST /api/expression/validate` + `GET /api/expression/catalog`（字段三态、探测 fail-closed）。
- **不做本地兜底（2026-10-25 用户确认，硬约束）**：S3-c 四项一律 **fail-closed** —— core 不可达 / 5xx / 超时**直接报错**（错误文案点名端点与原因），**不保留** agent 侧本地实现副本、**不静默降级**到旧路径。理由：双实现必漂移，且会掩盖「core 没起来」这个真问题；与 core 裁-11（配置了但取不到即 run `failed` + `risk_rules_unavailable`）同口径。
- 规模：4 包 / 10 个生产文件 / 92 处符号引用（`strategy` 3 + `expression` 4 + `backtest` 2 + `risk` 1）；S3-c-1 已清 `risk` 1 个生产文件 / 7 处符号引用；S3-c-2 已清 `strategy` 注册表 1 个生产文件 + 1 个测试文件（`strategy` 仍有 2 个生产文件：`pipeline/pipeline.go`、`yaml/loader.go`，属类型/配置面，不在 S3-c-2 范围）；剩余 `strategy` 类型/配置面、`expression`、`backtest`。
- 门：用到的端点必须在 core `docs/openapi.yaml` 有登记（本仓已有 `TestClientEndpoints_AreRegisteredInCoreOpenAPI` 护栏，端点字面量自动纳入）；同参数同数据下，HTTP 回测与进程内回测的关键指标对拍（Sharpe / 总收益 / 交易数）。

> **S3-c-1 施工记录（risk 工具 HTTP 化，2026-10-25，commit 随本行落）**：
> - **改了什么**：新增 `pkg/ai/client/risk_client.go`（`RiskClient.DetectRegime(ctx, symbol, startDate, endDate)` → `POST /api/risk/detect_regime`，30s 超时，与 `FactorClient` 同源基址、同样不带 bearer token）。`pkg/ai/tools/builtin/market_regime_tool.go` 由「注入 core `*risk.RiskManager` + 本地取 OHLCV 算指标」改为「注入一个窄接口 `RegimeClient`，逐字段透传服务端判定」；`cmd/ai/builders.go` 的 `BuildToolsRegistry` 删掉 `regimeDetector` 形参，Group 9 直接装配 `builtin.NewGetMarketRegimeTool(client.NewRiskClient(factorAPIURL))`。工具体积 309 → 194 行（净 −115，未增长）。
> - **§4 表 row 15 的顾虑已解**：原判断「端点只收内联 OHLCV，接 HTTP 会把省下的 token 还回去」被 QT-70 的粗粒度形状推翻 —— 请求体只有 `symbol/start_date/end_date` 三个短字段，200+ 根 bar 留在服务端。本仓不再 import core `pkg/risk`。
> - **如实登记的行为变化（不是遗漏）**：输出删掉 `bars_analyzed`。粗粒度端点不回报参与判定的 bar 数，旧值是本仓自己数的；填 0 或猜一个数都是「没量过的数字」，故整字段移除，`OutputSchema` 由 8 字段改 7 字段。`as_of` 语义改为「服务端完成判定的时间戳」（core 侧是 `time.Now()`，非最后一根 bar 的日期）。
> - **对拍口径**：工具已不参与 regime 计算，改钉「请求形状 + 逐字段透传」。golden 取自 core `risk.RegimeDetector`（fast_ma=10 / slow_ma=20 / vol_lookback=60，与 `config/ai-service.yaml` 一致）对固定序列（250 根、2024-01-01 起逐日、close=100+i）的判定：`trend="sideways" volatility="low" sentiment=0.3220913107511046`；`market_regime_tool_test.go` 断言工具输出逐字段相同。
> - **端点登记**：路径字面量写在 `pkg/ai/client/risk_client.go` 非测试源码里，自动进入 `TestClientEndpoints_AreRegisteredInCoreOpenAPI` 的扫描面；另加 `TestRiskClient_EndpointIsScannedByContractTest` 直接断言扫描结果包含 `/api/risk/detect_regime`。
> - **验收（真实输出）**：`go build ./...` → BUILD OK；`go vet ./...` → VET OK；`go test ./...` → 全部包 `ok`（`cmd/ai 0.309s`、`pkg/ai/client 0.239s`、`pkg/ai/tools/builtin 0.162s`、`pkg/ai/pipeline 4.255s`，无 FAIL）。
> - **破坏验证**：把 `risk_client.go` 请求标签 `json:"start_date"` 改成 `json:"start"` ⇒ `TestRiskClient_DetectRegime_CoarseShape` 红（实际体 `"start":"2024-01-01"` vs 期望 `"start_date"`）；逐字节还原后同一用例绿。
> - **边界**：`grep '"tellurion/pkg/risk"'` 生产**零命中**，只剩 2 个测试文件（`pkg/ai/validation/economic_integration_test.go`、`live_backtest_integration_test.go`，随 S3-d 测试腿清零）。core 实现包 import 面：39 → **38 处**（清掉生产 1 处），分布 **22 文件 / 11 包**（生产 9 文件 15 处、测试 13 文件 23 处；`prompts.go` 3 处与 `repoguard` 4 处伪命中不计）。

> **S3-c-2 施工记录（strategy 注册表 HTTP 化，2026-10-25，commit 随本行落）**：
> - **改了什么**：新增 `pkg/ai/client/strategy_client.go`（`StrategyClient.ListStrategies(ctx)` → `GET /api/strategies`，**不发任何 query**，30s 超时，与 `FactorClient` / `RiskClient` 同源基址、同样不带 bearer token）。本地 DTO `StrategyInfo` / `StrategyParameter` 逐字镜像 core 的 json tag（`name` / `description` / `parameters` / `type` / `default` / `min` / `max`），不 import core 包。`pkg/ai/tools/builtin/strategy_registry.go` 的两个工具改为注入窄接口 `StrategyCatalog`（`NewStrategyListTool` / `NewStrategyGetTool` 形参由空变一，nil 在装配时报 panic）；`cmd/ai/builders.go` 的 `BuildToolsRegistry` **形参未变**，Group 4 用 `client.NewStrategyClient(factorAPIURL)` 一次构造、两工具共用。
> - **`strategy.get` 语义**：改为从 list 结果里按 `name` **精确匹配**（与旧 `GlobalGet` 的 map 键语义一致），不再单查 `GET /api/strategies/{id}` —— 那条路径只查 DB（内置策略 404），无法覆盖合并目录。未命中错误仍以 `strategy.get:` 开头且不是 `tools.ErrInvalidArgs`（参数合法、策略不存在）；参数非法时**不打下游**（测试钉住 `catalog.calls == 0`）。
> - **对拍口径**：响应形状直接取自 core `DB.ListWithDB`（先 DB 活跃项、DB 条目 `name = strategy_id`，再补注册表未覆盖项；DB 的 params 对象被投影成 `Parameter` 数组、键排序）。`strategy_client_test.go` 用两段真实形状（描述符数组 / `parameters:null`）逐字段解码；`StrategyGetTool` 的过滤语义在 builtin 侧用 fake catalog 覆盖。
> - **如实登记（不是遗漏）**：core `docs/openapi.yaml` 的 `/api/strategies` 200 schema 声明 items 为 `StrategyConfig`（DB 配置形状），而默认分支**实际返回**的是 `StrategyInfo`（合并目录）——这是 core 的文档缺口，本仓不代改；本仓的护栏只要求路径有出处（`TestClientEndpoints_AreRegisteredInCoreOpenAPI` 绿），不对此 DTO 做 openapi 键对拍。
> - **端点登记**：路径字面量写在 `pkg/ai/client/strategy_client.go` 非测试源码里，自动进入 `TestClientEndpoints_AreRegisteredInCoreOpenAPI` 的扫描面；另加 `TestStrategyClient_EndpointIsScannedByContractTest` 直接断言扫描结果包含 `/api/strategies`。「无 query」由 `TestStrategyClient_ListStrategies_RequestShape` 钉死（带 `?active=true` 会切到只查 DB 分支，内置策略整批消失）。
> - **验收（真实输出）**：`go build ./...` → BUILD OK；`go vet ./...` → VET OK；`go test ./...` → 全部包 `ok`（无 FAIL）。
> - **破坏验证**：把 `strategy_client.go` 响应封装标签 `json:"strategies"` 改成 `json:"items"` ⇒ `TestStrategyClient_ListStrategies_RequestShape` / `_FieldsFlowThrough` / `TestStrategyClient_ResponseMatchesCoreListWithDBShape` 三条红（`"[]" should have N item(s)`）；从备份逐字节还原（SHA256 `031D3C16…1681ECAC` 与改前一致）后同一组用例绿。
> - **边界**：`grep '"tellurion/pkg/strategy"'` 在**注册表工具**生产代码零命中；但 `strategy` 包仍有 **2 个生产文件**（`pkg/ai/pipeline/pipeline.go`、`pkg/ai/yaml/loader.go`，用的是类型/工厂注册面）与 5 个测试文件，属 S3-c 的后续段（不虚报为已清）。core 实现包 import 面：38 → **36 处**（清掉生产 1 + 测试 1），分布 **20 文件**（生产 8 文件 14 处、测试 12 文件 22 处；`prompts.go` 3 处伪命中与 `repoguard` 4 处伪命中不计）。

> **S3-c-3 施工记录（backtest / walk-forward HTTP 化，2026-10-25，commit 随本行落）**：
> - **动手前证伪了一条前提**：`POST /api/backtest` 能收 `strategy_params` 不等于 agent 的表达式策略能被执行 —— core 的 analysis 二进制**没有导入** `pkg/strategy/expression`（`go list -deps ./cmd/analysis | grep -c` 实测 **0**），`expression_template` 不在注册表里；DB 策略只进目录、不可执行（引擎只走 `DefaultRegistry.NewInstance`）。用户授权后在 **core `cmd/analysis/main.go` 加一行空导入**（core commit `faec551`），这是 S3-c-3 的第五个交接件（原 §5 只列了四条）。
> - **提交形状**：pipeline 不再在本地注册策略。YAML 的 expression 段经 `yaml.ResolveExpression` 映射成 `strategy: expression_template` + `strategy_params`（键名与 core `ExpressionStrategy.Parameters()` 逐字一致；无 expression 段时不发 `signal_expr`，让 core 用清单默认值）。YAML 里的 `strategy.name` 降为实验日志标签。
> - **装配删除**：`cmd/ai/main.go` 删掉 `BuildBacktestEngine` / `BuildRiskManager` / `BuildExecutionTrader` / `InitStore` / `newEngine` / `pgProvider` / `dataAdapter` / `wfEngine`；`builders.go` 删 `StrategyEngineAdapter` / `WalkForwardEngineAdapter` / `buildBacktestRequest`（指数识别迁到 `cmd/ai/http_runner.go`）。新增 `HTTPBacktestRunner` / `HTTPWalkForwardRunner`（`cmd/ai/http_runner.go`）。
> - **凭证**：回测端点至少 trader。`client.BacktestClient` 新增 `WithTokenFromContext` + `WithBearerTokenInContext`：handler 把入站 JWT 放进请求 ctx，客户端转发；静态 token > ctx token > loopback 开发模式，三者皆无时 fail-closed。异步链路（copilot `context.WithoutCancel`、explore 的独立 ctx）显式保留该值。
> - **`domain.WalkForwardParams` 没有 json tag**（实测）：线上键是 `TrainDays` 等 PascalCase。客户端**直接序列化契约类型**，不自定义 snake_case 镜像 —— core 用同一类型解码，拼错键名会被静默忽略并回落默认窗口。
> - **如实登记的行为变化**：① pipeline 的注册阶段变成解析阶段，非法 DSL 表达式的失败点从 Stage 3 后移到 Stage 5（core 回测入口）；本仓不再本地解析 DSL。② `strategy.get` 之外，`backtest.run` / `walk_forward_validate` 工具仍按调用方给的名字跑 core 已注册策略（语义不变）。③ copilot 生成代码后按名字回测的既有行为不变（那个名字从来不在 core 注册表里，旧进程内引擎同样报 not found）。
> - **验收（真实输出）**：`go build ./...` → **0**；`go vet ./...` → **0**；`go test ./... -count=1` → 全部 `ok`（`cmd/ai 0.556s`、`pkg/ai/client 0.491s`、`pkg/ai/pipeline 4.864s`，无 FAIL）。生产代码 `grep '"tellurion/pkg/backtest'` **零命中**；`marketdata` 同步清零。core 实现包 import 面：生产 14 → **10 处 / 8 → 7 文件**（清 `backtest` 2 + `marketdata` 1 + `strategy` 1），测试 22 → **19 处 / 12 → 9 文件**；合计 **36 处 / 20 文件 → 29 处 / 16 文件**。
> - **破坏验证**：把 `http_runner.go` 的 `req.StrategyParams = strategyParams` 临时改成丢弃参数 ⇒ `TestHTTPBacktestRunner_SubmitsIndexAndParams` 红（`strategy_params` 缺失）；逐字节还原（SHA256 一致）后同一用例绿。
> - **未取到的证据（如实）**：本机无 PG（`127.0.0.1:5432` 拒绝连接）且 data-service 未起，**未跑真库 A/B 指标对拍**（Sharpe / 总收益 / 交易数）。现有证据是请求形状+响应逐字段映射的单测；端到端腿 `TestData01_IndexUniverse_EndToEndBacktest` 已改为打活的 `ANALYSIS_URL`，有库环境可直接取证。

### S3-d｜装配收尾 + 测试腿清零 + 删 `replace`

- `bootstrap`：工具腿（`logging` / `observability` / `httpclient` / `httpserver.ApplyGinMode`）换 kit；服务生命周期（Router 组装 / 启动 / 优雅退出）本仓自持；引擎与 store 装配腿删除。**core `bootstrap` 的 import 在这一批才彻底消失**（S3-a/b 期间它还留着引擎装配调用）。
- 19 个测试文件清零：`strategy/examples` / `storage` 两个仅测试包 + 进程内集成腿改 HTTP 或删除。
- 删 `replace tellurion => ../tellurion` 与 CI 的 rewrite 步骤；`go.mod` 注记按 §0 口径订正。
- 门（S3 的最终判据）：**干净 checkout + 删掉 `replace` 后** `go build ./...`、`go vet ./...`、`go test ./...` 全绿；`grep -rn '"tellurion/' --include='*.go'` 只剩 0 命中；core 仓的 `agent-compat` 腿仍绿。

## 5. core 侧前置（交接件——本仓**不改 core**，报协调者）

| # | 需要 core 做什么 | 卡住哪一批 | 状态 / 备注 |
|---|---|---|---|
| 1 | `POST /api/walkforward` 的请求体补 `strategy_params`，并登记 `x-agent-coarse-action` | S3-c-3 | ✅ QT-68（2026-10-24）补参数面；`x-agent-coarse-action: submit_walkforward` 由 core 裁-12 于 2026-10-25 落地 |
| 2 | openapi 为 `/api/backtest` 补登 `strategy_ref` / `strategy_params` | S3-c-3（文档项） | ✅ 随 QT-68 落地；运行时早已接受，属文档缺口 |
| 3 | 表达式端点：`POST /api/expression/validate` + `GET /api/expression/catalog`（算子 / 字段 / 阈值） | S3-c-4 | ✅ QT-69（2026-10-24）；目录能表达「字段不可用 ≠ 空」（三态 + fail-closed 503） |
| 4 | `POST /api/risk/detect_regime` 的粗粒度形状（`symbol + start_date + end_date`，引擎内部取数） | S3-c-1（可选） | ✅ QT-70（2026-10-24）；未接线 503 / 无行 404，内联形状优先 |

> **结论**：S3-c 的四个 core 前置**全部就位**（2026-10-25 复核）。S3-c 不再有外部等待，只剩本仓改写与对拍。

## 6. 未决/已决一览（避免二次讨论）

| 岔路 | 结论 | 代价 |
|---|---|---|
| auth 的用户表归谁 | 留 core；本仓只消费 claims | 本仓不能自助建用户 |
| expression 目录放 contracts 还是 core 端点 | core 端点（唯一真相） | core 一条新端点 + openapi |
| `strategy/monitor` 放 contracts 还是本仓 | 本仓（只有本仓用，contracts 门槛是 ≥2 仓） | 无 |
| `strategy/expression` 配置形状放 contracts | **不放**：HTTP 化后只有引擎用 | 本仓改提交 YAML/参数 |
| staticcheck 放 kit 还是本仓 | kit 已有 `sandbox/staticcheck`，纯换源 | 无 |
| 零生产构造点的 agents/pipeline 文件 | **先订正文案再删**（走 ODR），不在 S3 里顺手删 | 一次 ODR + 一次台账订正 |

## 7. 不在本清单内

- 删 `replace` 本身、CI rewrite 步骤（S3-d 的最后一笔，条件满足才动）。
- core 旧表 `public.experiments` / `public.factor_genes` / `public.strategy_genes` 的退役（core QT-05 ④，2027-01-31）。
- 任何跨仓代码改动：本仓只读 core / contracts / kit 的源码做核对。
