# 0003 · 候选生成的形状归属：agent 侧生成并落 ai.factor_genes，core mining 退役

- **状态**：已裁决（2026-11-07，DBT-18）
- **场景**：DBT-18 要求把「候选生成」从投递 JSON 文件升级成本仓的真实生成器。候选的形状（公式 / 参数 / 代际）必须有一个归属：是 core 的 mining pipeline 继续写 `factor_genes`，还是 agent 生成并写 `ai.factor_genes`？两套并存会同时写两处，留痕与代际链会对不上。跨仓方向已定：`tellurion:SVC-CORE-AI-RETIRE` 要 core 停写 AI 遗留表。
- **决策**：
  1. 采用 **(a)**：候选只在 agent 侧生成，落本仓 `ai.factor_genes`；core 的 mining pipeline 随 `SVC-CORE-AI-RETIRE` 退役，本仓不调用、不 import、不等待它。
  2. 提议器落在 `pkg/ai/loop` 的 `Proposer` 接口下：新增确定性 MCTS（`pkg/ai/search/mcts.go` + `pkg/ai/loop/mcts_proposer.go`），与 TPE 同接口，端点用 `proposer=tpe|mcts` 切换；不另起编排者。
  3. 候选生成复用基因池变异算子（`pkg/ai/mining/generator.go`），用**单事务整批**写 `ai.factor_genes`（`pkg/aistore/factor_candidate.go`）；生成失败即返回，不落半成品行。
  4. `seed` 是显式请求参数；缺省才退回墙上时钟，且实际值必须写进启动/状态响应、日志和每次尝试的 `Hypothesis`（随 `ai.experiments` 落库）。
  5. 新路径不调用模型，因此不产生 LLM 成本；未来 LLM 提议器必须走 `pkg/ai/cost.go` 的 `CostTable`，不得另起计费。
- **理由**：
  - (b) core 保留 pipeline + 新增候选投递端点：会新增一条跨仓契约与鉴权面（core 另单），agent 仍依赖 core 的落库语义，且与「AI 数据归 agent」的既有方向相悖。
  - (c) 沿用 JSON 文件投递：文件不是数据归属，回答不了「这次候选由谁生成、用哪个 seed、属于第几代」；本单会退化成只改生成算法。
  - (a) 与 `SVC-CORE-AI-RETIRE` 同向：写者只剩一处，代际链与实验留痕都能在 `ai.*` 内闭合。
- **代价 / 后果**：
  - core 侧删落库腿与 DDL 需要跨仓排期，属于 `SVC-CORE-AI-RETIRE` 的验收，不在本仓完成；本仓先只保证「不再新增对 core 的写依赖」。
  - 生成质量与可复现性完全由本仓承担：MCTS 的桶宽、迭代数是先验参数，需要靠对比 TPE 的实测结果调；同一 seed 只能保证可复现，不保证更好。
  - 机械检查：`pkg/ai/loop` 的两个提议器共享 `Proposer` 接口（编译期断言）；`pkg/aistore` 的写入口只出现在 `ai.*` 的 SQL 里；`go test` 的复现腿与 `GenerateAndSave` 反证腿守住 seed 与整批语义。
- **替代关系**：无（与 ADR-0002「生成链留痕归本仓」同向，不替代它）。
