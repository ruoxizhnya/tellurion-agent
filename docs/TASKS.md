---
status: active
last-verified: 2026-11-11
verified-by: 2026-11-11 全面审查 W7 台账立单（A5 授权）——新立 7 条六字段单（`AGENT-HERMES-DOCS-ADR041` / `AGENT-EXPERIMENT-COUNT-SURFACE` / `AGENT-INTERVENTION-ADOPTION-LEDGER` / `AGENT-HYPOTHESIS-FILTER` / `AGENT-LLM-BUDGET-GATE` / `AGENT-EXPLORE-STATE-REAP` / `AGENT-CAUSAL-PARSE-TESTS`），证据坐标引 ODR-068/071（`tellurion-docs:docs/archive/odr/`）；`AGENT-COPILOT-WORKING-DIR` 经 W4 复核维持 ⛔（去重，不另立单）；本轮只立单不修复。此前 2026-11-11 `AGENT-COPILOT-WORKING-DIR` 取证与裁决：同进程「每作业临时目录」不满足四条底线（无命令白名单、`BuildExecutor` 无 env 导致 GOCACHE/HOME 落在沙箱根外、runner 无网络/FS 隔离），且运行镜像 `alpine:3.19` 无 Go 工具链；选定容器化构建沙箱，按指令只出方案与单（状态 ⛔，含 A/C 退出条件与重开条件）。此前 2026-11-10 补真库腿——`TestSaveFactorCandidates_Integration` 在独立 PG 实例（`127.0.0.1:5433`，库 `tellurion`）上真跑通过（此前记「无库 SKIP」已失效），`pkg/aistore` 全包绿。此前 QT-32-B 收口（2026-11-10）—— 生成物接资产服务：`pkg/ai/client` 新客户端（建定义 → 传源码 → 产物载体 build+轮询）、copilot 登记端口 fail-closed、`ai.generation_chain` 并列四列坐标（幂等 ALTER 兼容存量库）；4 条破坏验证改坏即红、还原复绿；真库 round-trip 通过；gofmt/build/vet 0，全仓仅两条 QT-17 ③ 前置既有红（本文件 :187）。此前 SVC-CORE-AI-RETIRE 收口（core 七表 DDL/实现/seed 清零；本仓反向护栏翻面 + 停写腿兼容旧表不存在）。此前 2026-10-25 QT-32-A（G8 agent 腿）—— ai.generation_chain 留痕 + copilot 的 promote 闸门；8 条破坏验证改坏即红、还原 SHA 一致；门禁 build/vet/test 全 EXIT=0。此前 QT-17 ④ 停写复核（同日）
---

# TASKS — tellurion-agent 未完成项

> **归档说明（2026-11-11）**：本文件已瘦身为「只装未做的事」。2026-11-10 及以前已收口（✅）的任务原文、施工记录、验收输出与破坏验证，已逐字移入 [`docs/archive/tasks-archive-2026-11.md`](archive/tasks-archive-2026-11.md)；该文件顶部有「检索索引」表（ID / 标题 / 收口日期 / commit / 证据链接）。检索：`grep -n "<任务ID或commit>" docs/archive/tasks-archive-2026-11.md`。
>
> 状态：`⬜ 待做` / `🔶 进行中` / `⛔ 阻塞（有未解除前置）` / `⏸ 挂起（外部依赖或已裁决暂不实施；**不计入开放项**，重开须另立单并满足该条「重开条件」）` / `✅ 完成（附日期，随即移入归档）`

## 一、未做 / 挂起 / 外部依赖

### AGENT-FACTOR-TOOLS-UNAVAILABLE · 三个因子工具打向 core 不存在的端点 [P0] ✅

> **施工记录（2026-11-11）**：`pkg/ai/client/factor_client.go` 新增 `ErrFactorCapabilityUnavailable`；`ComputeFactor`/`EvaluateFactor` 不再发 HTTP，直接返回该哨兵；三个工具 Description 加 `[unavailable]` 前缀并指向 ADR-041；`docker-compose.yml` 与 `cmd/ai/main.go` 的错误注释同批订正（core 仓提交）。测试：`TestFactorClient_UnavailableWithoutHTTP`、三个 `*_Execute_Unavailable_NoHTTP`（httptest 计数断言 0 请求）、`TestNoPhantomFactorEndpoints`（幻影端点回潮即红）。破坏验证：临时把 `ComputeFactor` 改回发 HTTP ⇒ 3 条用例红（client 2 + builtin 1），还原复绿。门禁：`gofmt -l` 空、`go build ./...`、`go vet ./...`、`go test ./... -count=1` 全绿。

- **① 目标**：让 `factor.compute` / `factor.evaluate` / `compute_factor_ic` 在「core 无因子求值面」这个事实下**显式返回「该能力当前未提供」**，而不是发一个必失败的 HTTP 请求或返回空值；裁决见跨仓 ADR-041。
- **② 上下文**：`pkg/ai/client/factor_client.go:51,93` 调 `POST /api/factor/compute` 与 `/api/factor/evaluate`；`compute_factor_ic` 走同一 `EvaluateFactor`（`pkg/ai/tools/builtin/factor_tools.go:336`）。2026-11-11 实测：core 只注册 `POST /api/factor/registrations`（`cmd/analysis/handlers_factor_registration.go:65`）与反代 `GET /api/factors/:factor_name`（`handlers_proxy.go:167`，已是死链）；`docs/openapi.yaml` 也只有这两条；`docs/SPEC.md:3197-3210` 列的 `compute-returns`/`compute-ic` **代码里从未实现**。链路上还有两处错误事实传播：`docker-compose.yml` 与 `cmd/ai/main.go:100-101` 的注释称「`/api/factor/*` 只在 core 注册」。
- **③ 要求**：① `FactorClient` 的三个方法改为返回**稳定的「能力未提供」错误**（哨兵 + 明确文案「core 当前无因子求值/IC 求值端点；ADR-041」），不发起 HTTP；② 三个工具的 Description 与注册状态标为 `unavailable`（说明原因与替代路径：提交回测读结果），避免 LLM 反复调用必失败工具；③ `pkg/ai/client/endpoint_contract_test.go` 的 allowlist 改成**断言显式不可用**：core 将来真提供端点则红、工具偷偷改回发请求也红；④ 同批订正 `docker-compose.yml` 与 `cmd/ai/main.go` 的注释；⑤ `tellurion:docs/SPEC.md` 因子端点节标注「历史契约 / 当前未实现」（该处属 core 仓文档，跨仓单见 `tellurion:CORE-SPEC-FACTOR-ENDPOINTS-STALE`）。
- **④ 约束与非目标**：**禁止**在 core 补 `/api/factor/compute|evaluate` 空壳端点；**禁止**为了让测试变绿改 allowlist 或放宽断言；不改 DSL 语义、不新造远程求值面（ADR-028 §6）。
- **⑤ 验收**：`gofmt -l` 空、`go build ./...`、`go vet ./...`、`go test ./... -count=1` 全绿；三条工具调用各有一条用例断言返回「未提供」且**零 HTTP 请求**（用 `httptest` 计数为 0 证明）；破坏验证：把哨兵错误改回「发请求」⇒ 用例红；还原复绿。
- **⑥ 边界**：本仓 `pkg/ai/client` 与 `pkg/ai/tools/builtin`；core 侧只改文档标注（另单），不新增运行时能力。
- **重开条件**：core 真正实现「公式 × 符号」求值面、或 ADR-028 §6 的 strategy 侧求值落地并提供稳定 HTTP 面（ADR-041 §6）——届时重新接线并在 ADR-041 追加裁决。

### AGENT-COPILOT-WORKING-DIR · 容器里 `copilot.working_dir` 为空，代码生成必被沙箱拒绝 [P1] ⛔

- **① 目标**：给容器形态的 copilot 一个**满足四条底线的构建沙箱**（每作业独立目录、路径不逃出沙箱根、跑完清理、命令白名单），让 `POST /api/copilot/generate` 能走到 build 腿；不是只填一个路径。
- **② 上下文（2026-11-11 取证，结论：同进程方案不够）**：
  - 生成物**不执行**：`pkg/copilot/copilot.go:151-157` 明确生成代码是可审阅 artifact，本地只有 staticcheck + `go build` 两道门；但 `go build` 本身是处理不可信输入的复杂工具。
  - 目录/清理：`copilot.go:494-499` 用 `os.MkdirTemp("", "copilot-*")` + `defer os.RemoveAll`，崩溃会遗留、无启动期清扫；`working_dir` 只作 `BuildExecutor.Run` 的 cwd（`:568`），不是写入目录。
  - 沙箱能力（kit v0.2.0 `sandbox/runner/runner.go:39-74`）：✅ 墙钟超时 / 内存 / POSIX CPU 秒 / 进程组；❌ **网络出网隔离** / ❌ **文件系统 chroot** / ❌ **syscall 过滤**；staticcheck 是正则，不是边界。
  - **命令白名单不存在**：runner 无 Allowlist/LookPath 策略，命令由调用方给；`BuildExecutor` 接口没有 env 参数，kit 的 `Options.Env`（`runner.go:218-224`）被接口丢掉，因此 `GOCACHE` / `HOME` / `GOTMPDIR` 无法收进作业目录——「路径不逃出沙箱根」从接口层就不成立。
  - **运行时镜像没有 Go 工具链**：`Dockerfile` 运行阶段是 `alpine:3.19`，即使填了 `working_dir`，`go build` 也是 `executable file not found`；compose 的 `ai-service` 无卷、无 tmpfs、无 `user:`（root）。
  - 配置层可用：`cmd/ai/lifecycle.go:58-59` 有 `AutomaticEnv` + `.`→`_`，`COPILOT_WORKING_DIR` 能覆盖 `config/ai-service.yaml:67-72`；空值 fail-closed 语义正确。
- **③ 要求（选定 B：容器化构建沙箱，本轮不实现）**：① 独立 build-runner 容器：无网络、工具链与模块缓存只读挂载、仅作业目录可写、非 root、沿用超时/内存/CPU 限制；② ai-service 通过受限协议提交源码、取回编译结论，**不把 Go 工具链装进持有 DB 口令与 AI key 的 ai-service 镜像**；③ 每个作业在沙箱根下建独立目录，源码/产物/GOCACHE/HOME/TMP 全在其中，跑完删除并做启动期遗留清扫；④ 命令白名单只允许 `go build` 的固定 argv 形状；⑤ 启动日志点名生效的沙箱根与 runner 地址。
- **④ 约束与非目标**：不改 `pkg/copilot` 的 fail-closed 语义（缺目录/缺 runner 就拒，不降级成「随便找个目录」）；不在 ai-service 镜像里加 Go 工具链；不引入服务发现。
- **⑤ 验收**：真栈 `POST /api/copilot/generate` 终态 `succeeded` 且产物只能在沙箱目录内看到；破坏验证四条：① 清空工作目录 ⇒ 仍 `sandbox_rejected` 且文案点名 env/配置键；② 构造 `../` 路径 ⇒ 被拒；③ 沙箱内 `curl`/`wget` 外网 ⇒ 失败；④ 作业结束后目录被清、崩溃遗留由下次启动清扫。
- **⑥ 边界**：配置面在 `config/ai-service.yaml` 与统一 compose 的 `ai-service`；沙箱 runner 镜像/协议归本单；生成链路其余部分（建定义 → 传源码 → 产物载体）归 `pkg/ai/client`。
- **否决 A（每作业临时目录 + 同进程）**：四条底线今天有两条做不到（无命令白名单、env 传不进去导致构建写在沙箱根外），runner 也明确无网络/FS 隔离；补齐它要把 Go 工具链装进网络面服务，攻击面反而更大。
- **否决 C（禁执行、只产出代码）**：最安全，但拆掉 ADR-007 的编译门，Copilot 价值缩水；仅作为 B 不可落地时的退路。
- **退出条件**：① 若目标环境明确不允许容器运行时，退回 A，但必须先满足「`BuildExecutor` 支持 env + 作业目录内 GOCACHE/HOME/TMP + 命令白名单 + 启动期清扫」四条，任一条不满足继续停在 fail-closed；② 若 B 在目标环境不可落地，改 C，并把 `/api/copilot/generate` 终态从 `sandbox_rejected` 改为明确的 `code_ready_no_build`、删除 build 相关配置键，不留「看起来能编、实际没编」的中间态。
- **重开条件**：目标部署环境提供容器运行时（Docker 套接字或等价隔离原语），且主任决定继续投入 Copilot 的编译门；或环境明确不允许容器运行时、转而接受 C 的退路。
- **审查复核（2026-11-11，不另立单）**：全面审查 W4（A-2 沙箱）独立复核四条底线缺口属实，与本条前提一致；见 ODR-071（`tellurion-docs:docs/archive/odr/odr-071-audit-wave4-agent-ai-risk.md`）。去重：不新建任务，以本条为唯一落点，维持 ⛔。

### AGENT-RUNBOOK-GAP · 本仓没有 runbook，起停与生成链失败只有 README 零散说明 [P1] ⬜

- **① 目标**：给 agent 一份最小 runbook，让「起不来 / readyz 503 / 生成链失败 / LLM 未配置」有固定入口。
- **② 上下文**：`SERVICE.md` 落地时实测——本仓 `docs/` 无 `runbook.md`；起停写在 `README.md`「启动 ai-service」，生成链失败语义散在 `docs/api-schema.md` §5 与 `docs/dataflow.md`。六个服务仓都有 `docs/runbook.md`，core / agent / ui 三个仓没有。
- **③ 要求**：① 覆盖起停、`/health` 与 `/readyz` 的语义差异、DB 不可达、`ai.*` 迁移未执行、`AI not configured`、`chain sink not configured`、`sandbox_rejected` 四类失败原文；② 每节写「现象 → 先看什么 → 常见原因 → 对应日志 / job 终态」，不重复 `docs/api-schema.md` 的参数表；③ 完成后把 `SERVICE.md` 的 `truth.runbook` 改为该文件路径。
- **④ 约束与非目标**：不写 SLO / 告警规则 / 值班表；不改生成链的 fail-closed 语义；不把 `config/ai-service.yaml` 的键值清单抄进 runbook。
- **⑤ 验收**：按 runbook 从冷机起一次单仓栈成功；四类破坏场景（停 PG / 不给 `AI_API_KEY` / 不装留痕落点 / 故意让静态检查拒）都命中文中登记的失败原文与处置步骤。
- **⑥ 边界**：本仓起停与生成链排查归本仓；core 的能力端点与错误语义归 `tellurion` 的 runbook / openapi。
- **owner**：待定
- **重开条件**：出现第二次「AI 路由失败但只能翻 api-schema 找原因」的排障经历，或接手人变更。

### AGENT-HERMES-DOCS-ADR041 · Hermes 文档/技能仍把已不可用的因子工具写成现役 [P1] ✅

> **施工记录（2026-11-12）**：`docs/hermes/tools-quant-backtest.yaml`（三工具 Description 加 `[unavailable]` + ADR-041、`validate_factor`/`save_factor` 的 L2 表述同步订正）、`docs/hermes/prompts/quant-research.md`（Capabilities、硬约束第 1/2 条、工作流步骤 4、示例步骤 4/8）、`docs/hermes/skills/autonomous_factor_mining.md`（Step 4 改为「未提供，跳过」、验收标准第 4 条）、`docs/hermes/e2e-acceptance-test.md`（成功条件第 5 条、轨迹断言集合、排障节）统一改为「未提供（ADR-041），改走 backtest.run 读结果」，与 `pkg/ai/client/factor_client.go` 的哨兵一致。验收：`grep -rn "factor.compute\|factor.evaluate\|compute_factor_ic" docs/hermes` 无「现役/可用」表述，全部带 unavailable/ADR-041 标注；纯文档改动，Go 门禁不受影响（`go build ./...` 通过）。

- **① 目标**：把 Hermes 配置/提示词/技能里对三个因子工具（`factor.compute`/`factor.evaluate`/`compute_factor_ic`）的「现役」表述订正为「未提供（ADR-041）」，与 `AGENT-FACTOR-TOOLS-UNAVAILABLE` 已落的代码事实对齐。
- **② 上下文**：审查发现 W1-F2（ODR-068）。`docs/hermes/tools-quant-backtest.yaml:62-107,122-137`、`docs/hermes/prompts/quant-research.md:21,26-27,109,192`、`docs/hermes/skills/autonomous_factor_mining.md:89,284` 仍把因子求值工具写成可用；`8ace9b9` 已改三工具代码（返回 `ErrFactorCapabilityUnavailable`），文档未跟上。裁决见跨仓 ADR-041。
- **③ 要求**：① 三处文档逐条标「未提供 / ADR-041」并给替代路径（提交回测读结果）；② 技能里「调用因子求值」的步骤改为「不可用，改走回测」；③ 不改工具代码（已落）。
- **④ 约束与非目标**：不恢复因子求值端点；不改 DSL 语义；不新造远程求值面（ADR-028 §6）。
- **⑤ 验收**：grep 三处文档无「因子求值工具现役」表述；`check_doc_links`（若适用）绿；与代码哨兵一致。
- **⑥ 边界**：本仓 `docs/hermes/`；core 侧端点事实归 `tellurion:CORE-SPEC-FACTOR-ENDPOINTS-STALE`。
- **owner**：待定

### AGENT-EXPERIMENT-COUNT-SURFACE · 实验计数是内存 seq+1，回测/走查工具零实验行 [P1] ✅

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

### AGENT-INTERVENTION-ADOPTION-LEDGER · 无干预/采纳台账，校准与对账无数据基础 [P1] ✅

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

### AGENT-HYPOTHESIS-FILTER · 因果维直通弱信号，假设过滤形同虚设 [P2] ⬜

- **① 目标**：让因果维对弱信号/空结果有**可判定的过滤**，而不是直通，避免「看起来审过、实际没拦」。
- **② 上下文**：审查发现 W1-F9（ODR-068）。`pkg/ai/causal/causal.go:34-48` 直通弱信号；`:127-129` 在 `blind.Result=nil` 时仍判成立。验证器链的因果维是唯一需要 LLM 的一维（ADR-023），直通会让红队失效。
- **③ 要求**：① 弱信号/空结果显式判「不成立 / 证据不足」，不直通；② `blind.Result=nil` 不得判成立；③ 补过滤判据用例（绝对断言）。
- **④ 约束与非目标**：不改五维确定性校验器；不引入多 agent 投票（ADR-023 拒绝）；过滤阈值写死可判定。
- **⑤ 验收**：弱信号/空结果用例判不成立；破坏验证 = 改回直通 ⇒ 用例红。
- **⑥ 边界**：本仓 `pkg/ai/causal`；叙述者解析测试归 `AGENT-CAUSAL-PARSE-TESTS`。
- **owner**：待定

### AGENT-LLM-BUDGET-GATE · ResearchBudget 零代码，日累计 USD 闸缺失（D4=a）[P1] ✅

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

### AGENT-EXPLORE-STATE-REAP · explore 运行态纯内存，崩溃即孤儿（D10=b）[P1] ⬜

- **① 目标**：给 explore 运行态一个**启动收尸**机制，让进程崩溃后遗留的 running 态能被回收，而不是永远卡在内存 map 里。
- **② 上下文**：审查发现 W4-F2（ODR-071）+ 裁决 D10=b（run 态易失 + 启动收尸）。`pkg/ai/handlers/handlers_explore.go:262-272` 运行态是内存 map；全仓 grep `stale`/`reap`/`recover`/`orphan` 零命中；`pipeline.go:759-764` 尽力而为日志。D10=b：接受 run 态易失，但启动时须收尸（标记遗留 running 为 failed/orphan）。
- **③ 要求**：① 启动期扫描遗留 running 态并标记 orphan/failed（收尸）；② 收尸口径写死（什么算遗留、怎么标）；③ 不引入跨请求持久 run 态（D10=b 明确易失）。
- **④ 约束与非目标**：不把 explore 态做成持久队列（那是 orchestration）；不改 pipeline 语义；收尸不静默丢日志。
- **⑤ 验收**：模拟崩溃遗留 → 重启后收尸标记可见；破坏验证 = 删收尸 ⇒ 遗留态永卡用例红。
- **⑥ 边界**：本仓 explore 启动收尸；持久 job 队列归 `tellurion-orchestration`。
- **owner**：待定

### AGENT-CAUSAL-PARSE-TESTS · 因果叙述者解析逻辑无直测，验证器侧用假叙述者 [P3] ⬜

- **① 目标**：给因果叙述者的解析/提取逻辑补**直接单元测试**，而不是只在 validation 侧用假叙述者间接覆盖。
- **② 上下文**：审查发现 W4-F3（ODR-071）。`pkg/ai/causal/narrator.go:121-143` 的提取逻辑无直测；`ValidateCausal` 的测试在 validation 侧用假叙述者，真解析路径（ malformed LLM 输出 / 空叙述 / 多段）未被钉住。
- **③ 要求**：① 对 `narrator.go:121-143` 补表驱动测试（正常/空/malformed/多段）；② 解析失败显式报错，不静默回落；③ 与 `AGENT-HYPOTHESIS-FILTER` 撞 causal 包按单号串行。
- **④ 约束与非目标**：不改叙述者生产语义；不依赖真 LLM（用固定输出夹具）。
- **⑤ 验收**：解析测试覆盖四类输入；破坏验证 = 改坏提取逻辑 ⇒ 用例红。
- **⑥ 边界**：本仓 `pkg/ai/causal/narrator.go` 测试；过滤判据归 `AGENT-HYPOTHESIS-FILTER`。
- **owner**：待定

**其余无未做 / 挂起 / 外部依赖项**（截至 2026-11-11 台账重整 + 全面审查 W7 立单）。已收口单的正文、验收与施工记录全部在归档文件里；新增工作一律另立单，不得以「补充说明」形式挂在本节。

## 二、跨仓指针（不在本仓，勿双写）

| 事项 | 去哪 |
|---|---|
| core 侧消费面与数据/风控/通道边界 | `tellurion:docs/TASKS.md` |
| 契约形状与版本 pin | `tellurion-contracts:docs/TASKS.md` |
| 前端侧 | `tellurion-ui:docs/TASKS.md` |
