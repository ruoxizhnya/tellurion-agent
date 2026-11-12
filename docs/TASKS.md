---
status: active
last-verified: 2026-11-12
verified-by: 2026-11-12 `AGENT-REVIEW-PENDING-SOURCE` ✅ 完成（`GET /api/explore/review/pending` 上游读面，形状按 core 已定稿契约；施工记录与口径边界见归档）；此前同日 UI 子台账 B-06 批次——`AGENT-EXPLORE-INTERVENTIONS` ✅ 完成（三条干预写面 G2 指方向 / G3 追问 / G4 标记 + adoption/calibration 契约回显，4 个原子 commit `76c98ee`/`9b4e381`/`4d27919`/`2f4b8a1` 已推 `origin/main`）；再此前： 2026-11-12 台账卫生批次——8 条已完成项（`AGENT-FACTOR-TOOLS-UNAVAILABLE` / `AGENT-HERMES-DOCS-ADR041` / `AGENT-EXPERIMENT-COUNT-SURFACE` / `AGENT-INTERVENTION-ADOPTION-LEDGER` / `AGENT-HYPOTHESIS-FILTER` / `AGENT-LLM-BUDGET-GATE` / `AGENT-EXPLORE-STATE-REAP` / `AGENT-CAUSAL-PARSE-TESTS`）原文逐字移入归档并补索引行，活跃层仅剩 `AGENT-COPILOT-WORKING-DIR` ⛔。此前： 2026-11-12 挂起任务处置批次（主任指令）——`AGENT-RUNBOOK-GAP` ✅ 完成（新增 `docs/runbook.md`：起停、`/health` vs `/readyz`、DB 不可达、`ai.*` 迁移缺失、`AI not configured`、`chain sink not configured`、`sandbox_rejected` 分支；命令/端口/env 逐项对照代码、Dockerfile、compose；`SERVICE.md` 的 `truth.runbook` 已指向该文件）；`AGENT-COPILOT-WORKING-DIR` 复核维持 ⛔：Docker 29.9.3 实测可用，重开条件的环境半条已满足，剩主任投入决策（B/C/继续挂）；kit v0.5.0 行号施工时重核。此前： 2026-11-12 `X-LOW-CLEANUP` agent 腿——核 `/api/health` 豁免注册：本进程只注册 `/health` / `/readyz` / `/api/readyz` / `/metrics` / `/api/auth/*`，无 `/api/health`；`aiPublicPaths` 与 `rateLimitExemptPaths` 两表均删除该条并改写注释（不再宣称「与 core 逐字一致」）；同日 `X-DEP` 补 Dockerfile 工具链 `golang:1.26.11-alpine`（实建通过）；2026-11-12 `X-CI-RACE-STATICCHECK` 补 CI 的 `-race` 与 `staticcheck` 门（存量 23 条告警按裁决 A 修干净、无配置基线；SA4010/SA4006 各单独 commit；`-race` 用 Docker golang:1.26 全量跑通 EXIT=0）；2026-11-12 `X-AGENTS-DEPENDENCY-TABLE` `AGENTS.md` §4 依赖表按实际 `go.mod` 订正（contracts v0.10.0 / kit v0.5.0 / pgx v5.11.0）；2026-11-12 `X-DEP-BUMP-CONSUMERS` 本仓腿：kit `v0.2.0 → v0.5.0`、`go` 1.25.0 → 1.26.11、pgx 随 MVS 到 v5.11.0；CI 增 `govulncheck` 门；降版破坏验证复红/还原复绿；`build/vet/test` 全绿；2026-11-11 全面审查 W7 台账立单（A5 授权）——新立 7 条六字段单（`AGENT-HERMES-DOCS-ADR041` / `AGENT-EXPERIMENT-COUNT-SURFACE` / `AGENT-INTERVENTION-ADOPTION-LEDGER` / `AGENT-HYPOTHESIS-FILTER` / `AGENT-LLM-BUDGET-GATE` / `AGENT-EXPLORE-STATE-REAP` / `AGENT-CAUSAL-PARSE-TESTS`），证据坐标引 ODR-068/071（`tellurion-docs:docs/archive/odr/`）；`AGENT-COPILOT-WORKING-DIR` 经 W4 复核维持 ⛔（去重，不另立单）；本轮只立单不修复。此前 2026-11-11 `AGENT-COPILOT-WORKING-DIR` 取证与裁决：同进程「每作业临时目录」不满足四条底线（无命令白名单、`BuildExecutor` 无 env 导致 GOCACHE/HOME 落在沙箱根外、runner 无网络/FS 隔离），且运行镜像 `alpine:3.19` 无 Go 工具链；选定容器化构建沙箱，按指令只出方案与单（状态 ⛔，含 A/C 退出条件与重开条件）。此前 2026-11-10 补真库腿——`TestSaveFactorCandidates_Integration` 在独立 PG 实例（`127.0.0.1:5433`，库 `tellurion`）上真跑通过（此前记「无库 SKIP」已失效），`pkg/aistore` 全包绿。此前 QT-32-B 收口（2026-11-10）—— 生成物接资产服务：`pkg/ai/client` 新客户端（建定义 → 传源码 → 产物载体 build+轮询）、copilot 登记端口 fail-closed、`ai.generation_chain` 并列四列坐标（幂等 ALTER 兼容存量库）；4 条破坏验证改坏即红、还原复绿；真库 round-trip 通过；gofmt/build/vet 0，全仓仅两条 QT-17 ③ 前置既有红（本文件 :187）。此前 SVC-CORE-AI-RETIRE 收口（core 七表 DDL/实现/seed 清零；本仓反向护栏翻面 + 停写腿兼容旧表不存在）。此前 2026-10-25 QT-32-A（G8 agent 腿）—— ai.generation_chain 留痕 + copilot 的 promote 闸门；8 条破坏验证改坏即红、还原 SHA 一致；门禁 build/vet/test 全 EXIT=0。此前 QT-17 ④ 停写复核（同日）
---

# TASKS — tellurion-agent 未完成项

> **归档说明（2026-11-11）**：本文件已瘦身为「只装未做的事」。2026-11-10 及以前已收口（✅）的任务原文、施工记录、验收输出与破坏验证，已逐字移入 [`docs/archive/tasks-archive-2026-11.md`](archive/tasks-archive-2026-11.md)；该文件顶部有「检索索引」表（ID / 标题 / 收口日期 / commit / 证据链接）。检索：`grep -n "<任务ID或commit>" docs/archive/tasks-archive-2026-11.md`。
>
> 状态：`⬜ 待做` / `🔶 进行中` / `⛔ 阻塞（有未解除前置）` / `⏸ 挂起（外部依赖或已裁决暂不实施；**不计入开放项**，重开须另立单并满足该条「重开条件」）` / `✅ 完成（附日期，随即移入归档）`

## 一、未做 / 挂起 / 外部依赖

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
- **复核（2026-11-12，挂起任务处置批次，维持 ⛔）**：本机 Docker 实测可用（engine 29.9.3，`docker version` EXIT=0）——**重开条件的环境半条已满足**；剩余唯一前置 = 主任的投入决策（继续 B / 接受 C 退路 / 维持挂起）。两条事实更新：运行镜像仍 `alpine:3.19` 无 Go 工具链（builder 已 `golang:1.26.11-alpine`）；kit 已 `v0.5.0`，原 ② 引 `v0.2.0` 的 runner 行号，施工时按新版重核四条底线缺口。

**其余无未做 / 挂起 / 外部依赖项**（截至 2026-11-12 UI 子台账 B-06 批次）。已收口单的正文、验收与施工记录全部在归档文件里；新增工作一律另立单，不得以「补充说明」形式挂在本节。

## 二、跨仓指针（不在本仓，勿双写）

| 事项 | 去哪 |
|---|---|
| core 侧消费面与数据/风控/通道边界 | `tellurion:docs/TASKS.md` |
| 契约形状与版本 pin | `tellurion-contracts:docs/TASKS.md` |
| 前端侧 | `tellurion-ui:docs/TASKS.md` |
