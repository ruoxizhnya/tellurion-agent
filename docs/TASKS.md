---
status: active
last-verified: 2026-11-10
verified-by: 2026-11-10 补真库腿——`TestSaveFactorCandidates_Integration` 在独立 PG 实例（`127.0.0.1:5433`，库 `tellurion`）上真跑通过（此前记「无库 SKIP」已失效），`pkg/aistore` 全包绿。此前 QT-32-B 收口（2026-11-10）—— 生成物接资产服务：`pkg/ai/client` 新客户端（建定义 → 传源码 → 产物载体 build+轮询）、copilot 登记端口 fail-closed、`ai.generation_chain` 并列四列坐标（幂等 ALTER 兼容存量库）；4 条破坏验证改坏即红、还原复绿；真库 round-trip 通过；gofmt/build/vet 0，全仓仅两条 QT-17 ③ 前置既有红（本文件 :187）。此前 SVC-CORE-AI-RETIRE 收口（core 七表 DDL/实现/seed 清零；本仓反向护栏翻面 + 停写腿兼容旧表不存在）。此前 2026-10-25 QT-32-A（G8 agent 腿）—— ai.generation_chain 留痕 + copilot 的 promote 闸门；8 条破坏验证改坏即红、还原 SHA 一致；门禁 build/vet/test 全 EXIT=0。此前 QT-17 ④ 停写复核（同日）
---

# TASKS — tellurion-agent 未完成项

> **归档说明（2026-11-11）**：本文件已瘦身为「只装未做的事」。2026-11-10 及以前已收口（✅）的任务原文、施工记录、验收输出与破坏验证，已逐字移入 [`docs/archive/tasks-archive-2026-11.md`](archive/tasks-archive-2026-11.md)；该文件顶部有「检索索引」表（ID / 标题 / 收口日期 / commit / 证据链接）。检索：`grep -n "<任务ID或commit>" docs/archive/tasks-archive-2026-11.md`。
>
> 状态：`⬜ 待做` / `🔶 进行中` / `⛔ 阻塞（有未解除前置）` / `⏸ 挂起（外部依赖或已裁决暂不实施；**不计入开放项**，重开须另立单并满足该条「重开条件」）` / `✅ 完成（附日期，随即移入归档）`

## 一、未做 / 挂起 / 外部依赖

### AGENT-COPILOT-WORKING-DIR · 容器里 `copilot.working_dir` 为空，代码生成必被沙箱拒绝 [P1] ⬜

- **① 目标**：给容器形态的 copilot 一个合法的代码生成工作目录，让 `POST /api/copilot/generate` 能走到 build 腿，而不是在 LLM 返回后被沙箱当场拒掉。
- **② 上下文**：2026-11-11 真栈实测（统一 compose 的 `ai-service`）。LLM 调通后作业终态 `llm_failed`（`AI_HTTP_TIMEOUT` 已另修，默认 180s）；而启动日志里的 `{"working_dir":""}` 是下一道必红的闸——`pkg/copilot/copilot.go` 在 `working_dir` 为空时返回 `sandbox_rejected`（fail-closed），日志明确要求「set it in analysis-service.yaml under the copilot: key」，但容器里既没有该 yaml、也没有挂载工作目录。也就是说：**即使 LLM 正常返回，代码生成也一定失败**。
- **③ 要求**：① 定工作目录归属（容器内临时目录 vs 宿主机挂载卷）；② 明确沙箱边界（能写什么、能执行什么，不能把宿主仓根挂进去）；③ 在 compose 里以 env/挂载表达，不把路径写死进镜像；④ 启动日志要点名生效值。
- **④ 约束与非目标**：不在本轮修；不改 `pkg/copilot` 的 fail-closed 语义（缺目录就该拒，不是降级成「随便找个目录」）。
- **⑤ 验收**：真栈 `POST /api/copilot/generate` 终态 `succeeded` 且产物能在沙箱目录里看到；破坏验证 = 清空工作目录后仍 `sandbox_rejected` 且文案点名 env/配置键。
- **⑥ 边界**：配置面在 `config/ai-service.yaml` 与统一 compose 的 `ai-service` 服务；生成链路其余部分（建定义 → 传源码 → 产物载体）归 `pkg/ai/client`。

**其余无未做 / 挂起 / 外部依赖项**（截至 2026-11-11 台账重整）。已收口单的正文、验收与施工记录全部在归档文件里；新增工作一律另立单，不得以「补充说明」形式挂在本节。

## 二、跨仓指针（不在本仓，勿双写）

| 事项 | 去哪 |
|---|---|
| core 侧消费面与数据/风控/通道边界 | `tellurion:docs/TASKS.md` |
| 契约形状与版本 pin | `tellurion-contracts:docs/TASKS.md` |
| 前端侧 | `tellurion-ui:docs/TASKS.md` |
