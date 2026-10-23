# ODR-001: QT-17 ④ 停写 legacy 实验域表——public.experiments 写路径收回 + 30 行历史回填归档

> **Status**: Completed
> **Date**: 2026-10-24
> **Category**: Migration
> **Related ADRs**: ADR-D10（实验域数据归 agent，`tellurion:docs/design/qt-decoupling/target-module-topology.md`）
> **Supersedes**: 无

## Context

QT-17 ②（2026-10-23）起 `experiments` 写路径处于双写期：`cmd/ai/experiment_dualwrite.go` 同时写 core `public.experiments`（legacy）与本仓 `ai.experiments`，等对拍零差异后切读、停写。④ 的前置是对拍证据与「旧表里没有 ai 侧缺的行」。

停写前实测（真库 `quant_trading`，2026-10-24）：

- `public.experiments` 30 行（8 个 run_id，20 行有 `parent_id`），全部创建于 2026-09-27 ~ 2026-10-14，**早于双写期**，因此从未被镜像进 `ai.experiments`（0 行）。
- 双写适配器的对拍腿（已删除的 `TestDualWrite_ParityAgainstLegacy`）真库通过：完成/失败/裁决/父子链四条路径两边逐行一致。
- 谁的读：core `pkg/storage/experiments.go` 的 `GetExperiment` / `ListExperiments` **无生产调用方**（仅 `*_test.go`）；core `cmd/analysis` 无实验日志端点（只做 `/api/{copilot,pipeline,explore,tools}` 纯反代）；`tellurion-ui` 无实验日志页（其 `Explore.vue` 读 ai-service 的**进程内存**快照，不查表；实验日志深查询在其文档里登记为缺口 G5）；`tellurion-analytics` 零命中。
- 谁的写：除本仓双写适配器外，没有任何生产代码写 `public.experiments`。

结论：停写对唯一读方无影响（不存在），但 30 行历史若留在旧表就成了「数据在不再拥有写入权的表里」，所以停写前必须先归档。

## Decision

1. **回填归档（一次性、幂等）**：把 `public.experiments` 的 30 行搬进 `ai.experiments`（`ON CONFLICT (run_id, seq) DO NOTHING`），再按 `(run_id, parent.seq)` 把 `parent_id` 从 legacy id 空间重映射到 ai id 空间（20 行）。
2. **停写**：`cmd/ai/main.go` 直接把 `*aistore.Store` 注入 pipeline / explore 的 sink；删除 `cmd/ai/experiment_dualwrite.go`。
3. **护栏收紧**：`internal/repoguard` 的 `storageWhitelist` 三条收一条（删 `experiment_dualwrite.go`、`main.go` 改走 `kit/db.BuildDSN` 后收回）；新增真库腿 `cmd/ai` `TestStopWrite_LegacyExperimentsFrozen` 钉「旧表行数与序列不再前进」。
4. **不删旧表**：按 QT-17 ④ 约束保留只读一个周期；core 侧标 deprecated 与删除期限属 core 待办（交接件见 `docs/TASKS.md`）。

为什么选「先回填再停写」而不是「直接停写、历史留在旧表」：停写后本仓对 `public.experiments` 既无写权也无法保证它不被将来清理；历史实验是「试了几次才撞出来」的证据（过拟合检测的唯一数据源），不能留在别人的表里等它哪天被回收。

为什么不在本次一并切换 `factor_genes` / `strategy_genes`：④ 的范围是 `experiments` + `research.*`；基因池的 SQL 用未限定表名经 core 池写 `public.*`，切换涉及换池 + 加 schema 限定 + 补对拍，是独立一步，已登记为 `docs/db-schema.md` §5 待定 1。

## Consequences

- 正面：`ai.experiments` 成为完整历史（30 行归档 + 后续增量），写路径单点；`public.experiments` 冻结，序列不再前进；本仓对 core storage 的生产依赖从 3 处降到 1 处。
- 负面 / 需要知道：旧表仍有 30 行（保留只读一个周期），任何人若误以为它还会更新会读到陈旧数据 —— 由 `docs/db-schema.md` 与护栏共同标注；回填只做一次，将来旧表若有新行（不应发生）需人工筛查。
- `research.*` 的「停写」是**无操作**：全工作区不存在其生产写入方（core 无研究域 handler，真库 0 行）；此前文档写的「写方在 core」与代码事实不符，已订正。

## Artifacts

- 删除：`tellurion-agent:cmd/ai/experiment_dualwrite.go`、`tellurion-agent:cmd/ai/experiment_dualwrite_integration_test.go`
- 修改：`tellurion-agent:cmd/ai/main.go`（sink 直连 aistore；`BuildDSN` 改 kit）、`tellurion-agent:internal/repoguard/storage_import_test.go`（白名单收一条）
- 新增：`tellurion-agent:cmd/ai/experiment_stopwrite_integration_test.go`
- 数据：`ai.experiments` +30 行（父代重映射 20 行）
- 文档：`docs/db-schema.md`、`docs/dataflow.md`、`docs/ARCHITECTURE.md`、`docs/TASKS.md`

## Metrics

- 回填前：`public.experiments` 30 / `ai.experiments` 0；回填后：30 / 30，`parent_id` 非空 20。
- 行级对拍（`EXCEPT` 双向，父代折成 `parent_seq` 比）：`legacy_only = 0`、`ai_only = 0`。
- 停写取证：驱动一轮完整写入后 `public.experiments` 仍 30 行、`max(id)` 仍 1203。
- 生产代码 `pkg/storage` import 位点：3 → 1。

## Lessons Learned

对拍（双写一致性）与归档（历史归属）是两件事：对拍只能证明「新写入两边一样」，证明不了「旧数据已经在新表里」。停写前必须单独数一次旧表存量并按主键/业务键回填，否则历史会静默留在不再被拥有的表里。