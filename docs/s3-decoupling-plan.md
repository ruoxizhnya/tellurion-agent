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

### S3-a｜纯机械换源（0 跨仓依赖，可立即开工）

- 包：`statistics` → kit；`httpserver` → kit；`observability` → kit；`sandbox/runner` → kit；`sandbox/staticcheck` → kit；`data/source` → contracts/dataprotocol。
- 规模：6 包 / 14 个生产文件 / 52 处符号引用。
- 依据：kit `v0.1.0` 已含全部五个 kit 目标包（含 `sandbox/staticcheck`），contracts `v0.6.0` 已含 `dataprotocol`；两者都已在 `go.mod`，**不需要任何发版**；data/source 在 core 侧已是 contracts 别名（形状零变化）。
- 门：`go build/vet/test ./...` 绿；kit 与 core 同名函数逐个对拍（空输入、常量序列、超时）。
- 为什么先做：零行为变化、零外部等待，做完立刻把 40% 的 import 面消掉。

### S3-b｜本仓内去重（无跨仓依赖）

- 包：`strategy/monitor` → `pkg/ai/monitor`（本地纯函数）；`auth` → kit/auth + 本仓角色。
- 规模：2 包 / 2 个生产文件 / 46 处符号引用。
- **auth 的裁决（本文件直接定，不再拖）**：用户库留在 core（单人实验室没有第二套用户体系），本仓**只消费 token claims**（kit/auth 解析 + 角色常量），删除 `bootstrap.InitAuth(v, store, …)` 这条「借 core 连接池读 `users`」的路径。代价：本仓不再能自助建用户；代价可接受，因为建用户本来就不是 agent 的职责。
- 门：工具级用例（`monitor.strategy_health`、角色矩阵 viewer/trader/admin 三条）全绿。
- 说明：**不需要任何发版**——kit 已提供 `auth` 中间件与 `RequireRole`。

### S3-c｜HTTP 化（依赖 core 端点；子序按「端点已具备 → 端点待补」）

1. **risk**：直接接 `POST /api/risk/detect_regime`（零 core 改动）。代价：请求体是内联 OHLCV，token 省不回来。**可选优化**（记入 core 交接件）：加 `symbol + date range` 的粗粒度形状，避免 agent 先取数再回传。
2. **strategy 注册表**：接 `GET /api/strategies`（默认分支已合并注册表 ∪ DB 活跃项，`ListWithDB`）；`strategy.get` 改为从 list 结果过滤（`GET /{id}` 只查 DB）。**零 core 改动**，只需行为对拍。
3. **backtest**：接 `POST /api/backtest` + `POST /api/walkforward`；删除 `cmd/ai` 的 `newEngine` / `pgProvider` / `dataAdapter` / `wfEngine` 装配。`/api/backtest` 运行时已能收 `strategy_ref` / `strategy_params`（绑定结构嵌入 `contracts.BacktestRequest`），**walkforward 的请求体仍是 core 自有类型，需 core 补 `strategy_params`**。
4. **expression**：接 core 新端点（静态校验 + 目录），删除 parser/evaluator 依赖。前置：端点先落地。
- 规模：4 包 / 10 个生产文件 / 92 处符号引用（`strategy` 3 + `expression` 4 + `backtest` 2 + `risk` 1）。
- 门：用到的端点必须在 core `docs/openapi.yaml` 有登记（本仓已有 `TestClientEndpoints_AreRegisteredInCoreOpenAPI` 护栏，端点字面量自动纳入）；同参数同数据下，HTTP 回测与进程内回测的关键指标对拍（Sharpe / 总收益 / 交易数）。

### S3-d｜装配收尾 + 测试腿清零 + 删 `replace`

- `bootstrap`：工具腿（`logging` / `observability` / `httpclient` / `httpserver.ApplyGinMode`）换 kit；服务生命周期（Router 组装 / 启动 / 优雅退出）本仓自持；引擎与 store 装配腿删除。**core `bootstrap` 的 import 在这一批才彻底消失**（S3-a/b 期间它还留着引擎装配调用）。
- 19 个测试文件清零：`strategy/examples` / `storage` 两个仅测试包 + 进程内集成腿改 HTTP 或删除。
- 删 `replace tellurion => ../tellurion` 与 CI 的 rewrite 步骤；`go.mod` 注记按 §0 口径订正。
- 门（S3 的最终判据）：**干净 checkout + 删掉 `replace` 后** `go build ./...`、`go vet ./...`、`go test ./...` 全绿；`grep -rn '"tellurion/' --include='*.go'` 只剩 0 命中；core 仓的 `agent-compat` 腿仍绿。

## 5. core 侧前置（交接件——本仓**不改 core**，报协调者）

| # | 需要 core 做什么 | 卡住哪一批 | 备注 |
|---|---|---|---|
| 1 | `POST /api/walkforward` 的请求体补 `strategy_params`（现为 core 自有 `WalkForwardRequest`，只有策略名），并登记 `x-agent-coarse-action` | S3-c-3 | 8 条粗粒度动作里没有 walkforward；`/api/backtest` 侧无需改代码（已嵌入 contracts 结构） |
| 2 | openapi 为 `/api/backtest` 补登 `strategy_ref` / `strategy_params` | S3-c-3（文档项） | 运行时已接受，属文档缺口，不阻塞代码 |
| 3 | 表达式端点：`POST /api/expression/validate` + `GET /api/expression/catalog`（算子 / 字段 / 阈值） | S3-c-4 | 当前 openapi 无任何表达式端点；目录要能表达「字段不可用 ≠ 空」 |
| 4 | `POST /api/risk/detect_regime` 的粗粒度形状（`symbol + start_date + end_date`，引擎内部取数） | S3-c-1（可选） | 不做也能过 S3，只是 token 成本回到切换前 |

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
