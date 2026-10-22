---
status: active
last-verified: 2026-10-22
verified-by: A4 只列不改交付（AST 实测 + 两段临时副本 dry-run）
---

# contracts 迁移 · agent 侧 import 全替清单（B4 输入）

> **本文件是 A4 的交付物、B4 的输入。A4 只列不改**：清单由 `go/parser` AST 实测生成，并已在临时副本上做过两段 dry-run 验证；本仓代码零改动。
> **锚点**：agent `a5e847e`（A4 推送后的 `main`）；contracts `fbe9a2d`（`origin/main`，已推、**未打 tag**）。
> **临时件生命周期**：B4 执行完 → 把本文件标成已执行、移入 `docs/archive/`，并删除 `internal/repoguard/import_inventory_test.go`。那条护栏变红只有两种含义：① 源码漂移了，清单要重生成；② B4 做完了，护栏该退场。

## 1. 计数口径：台账的 48+5+3 是 grep 数，真实 import 是 46+5+3

| 口径 | domain | engine/contracts | fees | 合计 |
|---|---:|---:|---:|---:|
| 台账原文（`git grep -l` 文件数） | 48 | 5 | 3 | 56 |
| **实测真实 import（AST）** | **46** | **5** | **3** | **54 位点 / 48 文件** |
| 差值 | 2 | 0 | 0 | 2 |

差出来的 2 个文件不是 import，是**字符串字面量**：

- `pkg/ai/prompts.go` —— `SystemPrompt` 文本里 3 处提到旧路径（`:56` / `:85` / `:106`）。
- `pkg/ai/validator/code_validator_test.go` —— 测试夹具的源码字符串（`:57`）。

它们**是真依赖**（§6 单列、B4 必须处理），但不是 import 语句；`grep -l` 把「文本提到路径」与「编译期 import」混成了同一个数。
另：agent 无 `tellurion/pkg/domain/market` 直接 import（实测 0），所以本清单没有 domain/market 组。

- 54 个 import 位点分布在 **48 个文件**：6 个文件各 import 两条旧路径（`builders.go`、`pipeline.go`、`backtest_runner_alias_test.go`、`economic_integration_test.go`、`live_backtest_integration_test.go`、`copilot.go`）。
- 台账里的「48」恰好等于本清单的**唯一文件数**，但它在台账语境里指 domain 的文件数（实为 46）——两个数不是一回事，别互相引用。

汇总（护栏逐格核对）：

| 清单 | 旧 import | 新 import | 位点 | 唯一文件 |
|---|---|---|---:|---:|
| A | `tellurion/pkg/domain` | `github.com/ruoxizhnya/tellurion-contracts/domain` | 46 | 46 |
| B | `tellurion/pkg/backtest/contracts` | `github.com/ruoxizhnya/tellurion-contracts/engine` | 5 | 5 |
| C | `tellurion/pkg/fees` | `github.com/ruoxizhnya/tellurion-contracts/fees` | 3 | 3 |
| 合计 | — | — | 54 | 48 |

## 2. 替换规则

| 旧 import | 新 import | 目标包名 | 调用点前缀 |
|---|---|---|---|
| `tellurion/pkg/domain` | `github.com/ruoxizhnya/tellurion-contracts/domain` | `domain` | 不改 |
| `tellurion/pkg/backtest/contracts` | `github.com/ruoxizhnya/tellurion-contracts/engine` | `contracts` | **不改**（目录叫 engine，包名仍是 contracts） |
| `tellurion/pkg/fees` | `github.com/ruoxizhnya/tellurion-contracts/fees` | `fees` | 不改 |

- **只换 import 行**。三个目标包的包名与旧包一致，当前也**没有 alias**（实测），所以 `contracts.X` / `domain.X` / `fees.X` 一处都不用动。
- **不要**顺手把 `domain.OHLCV` 改成 `market.OHLCV`——`contracts/domain` 自带 `domain/market` 的别名，改前缀只会扩大 diff。
- 改完跑 `gofmt -w`：新路径会让 import 在分组内重新排序。

## 3. 清单 A · domain（46 位点 / 46 文件）

<!-- group tellurion/pkg/domain count=46 -->
| # | 文件 | 行 | 旧 import | 新 import |
|---:|---|---:|---|---|
| 1 | `cmd/ai/builders.go` | 27 | `tellurion/pkg/domain` | `github.com/ruoxizhnya/tellurion-contracts/domain` |
| 2 | `cmd/ai/handlers_explore.go` | 21 | `tellurion/pkg/domain` | `github.com/ruoxizhnya/tellurion-contracts/domain` |
| 3 | `cmd/ai/handlers_explore_test.go` | 19 | `tellurion/pkg/domain` | `github.com/ruoxizhnya/tellurion-contracts/domain` |
| 4 | `cmd/ai/handlers_pipeline_test.go` | 10 | `tellurion/pkg/domain` | `github.com/ruoxizhnya/tellurion-contracts/domain` |
| 5 | `cmd/ai/tools_registry_test.go` | 15 | `tellurion/pkg/domain` | `github.com/ruoxizhnya/tellurion-contracts/domain` |
| 6 | `pkg/ai/agents/validate_l4.go` | 11 | `tellurion/pkg/domain` | `github.com/ruoxizhnya/tellurion-contracts/domain` |
| 7 | `pkg/ai/client/backtest_client.go` | 16 | `tellurion/pkg/domain` | `github.com/ruoxizhnya/tellurion-contracts/domain` |
| 8 | `pkg/ai/loop/loop.go` | 18 | `tellurion/pkg/domain` | `github.com/ruoxizhnya/tellurion-contracts/domain` |
| 9 | `pkg/ai/loop/loop_integration_test.go` | 14 | `tellurion/pkg/domain` | `github.com/ruoxizhnya/tellurion-contracts/domain` |
| 10 | `pkg/ai/loop/loop_test.go` | 11 | `tellurion/pkg/domain` | `github.com/ruoxizhnya/tellurion-contracts/domain` |
| 11 | `pkg/ai/loop/loop_validation_test.go` | 10 | `tellurion/pkg/domain` | `github.com/ruoxizhnya/tellurion-contracts/domain` |
| 12 | `pkg/ai/loop/obs01_invalid_run_test.go` | 12 | `tellurion/pkg/domain` | `github.com/ruoxizhnya/tellurion-contracts/domain` |
| 13 | `pkg/ai/pipeline/backtest_runner_alias_test.go` | 26 | `tellurion/pkg/domain` | `github.com/ruoxizhnya/tellurion-contracts/domain` |
| 14 | `pkg/ai/pipeline/e2e_test.go` | 17 | `tellurion/pkg/domain` | `github.com/ruoxizhnya/tellurion-contracts/domain` |
| 15 | `pkg/ai/pipeline/experiment_log_integration_test.go` | 12 | `tellurion/pkg/domain` | `github.com/ruoxizhnya/tellurion-contracts/domain` |
| 16 | `pkg/ai/pipeline/experiment_log_test.go` | 15 | `tellurion/pkg/domain` | `github.com/ruoxizhnya/tellurion-contracts/domain` |
| 17 | `pkg/ai/pipeline/obs01_invalid_run_test.go` | 13 | `tellurion/pkg/domain` | `github.com/ruoxizhnya/tellurion-contracts/domain` |
| 18 | `pkg/ai/pipeline/pipeline.go` | 19 | `tellurion/pkg/domain` | `github.com/ruoxizhnya/tellurion-contracts/domain` |
| 19 | `pkg/ai/pipeline/pipeline_execute_test.go` | 13 | `tellurion/pkg/domain` | `github.com/ruoxizhnya/tellurion-contracts/domain` |
| 20 | `pkg/ai/pipeline/pipeline_test.go` | 18 | `tellurion/pkg/domain` | `github.com/ruoxizhnya/tellurion-contracts/domain` |
| 21 | `pkg/ai/tools/builtin/backtest_test.go` | 12 | `tellurion/pkg/domain` | `github.com/ruoxizhnya/tellurion-contracts/domain` |
| 22 | `pkg/ai/tools/builtin/factor_hypothesis_tool.go` | 9 | `tellurion/pkg/domain` | `github.com/ruoxizhnya/tellurion-contracts/domain` |
| 23 | `pkg/ai/tools/builtin/factor_hypothesis_tool_test.go` | 9 | `tellurion/pkg/domain` | `github.com/ruoxizhnya/tellurion-contracts/domain` |
| 24 | `pkg/ai/tools/builtin/market_regime_tool.go` | 11 | `tellurion/pkg/domain` | `github.com/ruoxizhnya/tellurion-contracts/domain` |
| 25 | `pkg/ai/tools/builtin/market_regime_tool_test.go` | 13 | `tellurion/pkg/domain` | `github.com/ruoxizhnya/tellurion-contracts/domain` |
| 26 | `pkg/ai/tools/builtin/strategy_registry_test.go` | 14 | `tellurion/pkg/domain` | `github.com/ruoxizhnya/tellurion-contracts/domain` |
| 27 | `pkg/ai/tools/builtin/summarize_tool.go` | 10 | `tellurion/pkg/domain` | `github.com/ruoxizhnya/tellurion-contracts/domain` |
| 28 | `pkg/ai/tools/builtin/summarize_tool_test.go` | 14 | `tellurion/pkg/domain` | `github.com/ruoxizhnya/tellurion-contracts/domain` |
| 29 | `pkg/ai/tools/builtin/walkforward_tool.go` | 9 | `tellurion/pkg/domain` | `github.com/ruoxizhnya/tellurion-contracts/domain` |
| 30 | `pkg/ai/tools/builtin/walkforward_tool_test.go` | 13 | `tellurion/pkg/domain` | `github.com/ruoxizhnya/tellurion-contracts/domain` |
| 31 | `pkg/ai/validation/aggregate.go` | 8 | `tellurion/pkg/domain` | `github.com/ruoxizhnya/tellurion-contracts/domain` |
| 32 | `pkg/ai/validation/aggregate_test.go` | 8 | `tellurion/pkg/domain` | `github.com/ruoxizhnya/tellurion-contracts/domain` |
| 33 | `pkg/ai/validation/causal.go` | 9 | `tellurion/pkg/domain` | `github.com/ruoxizhnya/tellurion-contracts/domain` |
| 34 | `pkg/ai/validation/causal_test.go` | 8 | `tellurion/pkg/domain` | `github.com/ruoxizhnya/tellurion-contracts/domain` |
| 35 | `pkg/ai/validation/economic_integration_test.go` | 28 | `tellurion/pkg/domain` | `github.com/ruoxizhnya/tellurion-contracts/domain` |
| 36 | `pkg/ai/validation/live_backtest_integration_test.go` | 34 | `tellurion/pkg/domain` | `github.com/ruoxizhnya/tellurion-contracts/domain` |
| 37 | `pkg/ai/validation/redundancy.go` | 8 | `tellurion/pkg/domain` | `github.com/ruoxizhnya/tellurion-contracts/domain` |
| 38 | `pkg/ai/validation/redundancy_integration_test.go` | 11 | `tellurion/pkg/domain` | `github.com/ruoxizhnya/tellurion-contracts/domain` |
| 39 | `pkg/ai/validation/robustness.go` | 9 | `tellurion/pkg/domain` | `github.com/ruoxizhnya/tellurion-contracts/domain` |
| 40 | `pkg/ai/validation/robustness_test.go` | 8 | `tellurion/pkg/domain` | `github.com/ruoxizhnya/tellurion-contracts/domain` |
| 41 | `pkg/ai/validation/turnover.go` | 6 | `tellurion/pkg/domain` | `github.com/ruoxizhnya/tellurion-contracts/domain` |
| 42 | `pkg/ai/validation/turnover_test.go` | 8 | `tellurion/pkg/domain` | `github.com/ruoxizhnya/tellurion-contracts/domain` |
| 43 | `pkg/ai/yaml/default_expression_test.go` | 11 | `tellurion/pkg/domain` | `github.com/ruoxizhnya/tellurion-contracts/domain` |
| 44 | `pkg/ai/yaml/loader.go` | 16 | `tellurion/pkg/domain` | `github.com/ruoxizhnya/tellurion-contracts/domain` |
| 45 | `pkg/ai/yaml/loader_test.go` | 11 | `tellurion/pkg/domain` | `github.com/ruoxizhnya/tellurion-contracts/domain` |
| 46 | `pkg/copilot/copilot.go` | 17 | `tellurion/pkg/domain` | `github.com/ruoxizhnya/tellurion-contracts/domain` |

## 4. 清单 B · engine/contracts（5 位点 / 5 文件）

<!-- group tellurion/pkg/backtest/contracts count=5 -->
| # | 文件 | 行 | 旧 import | 新 import |
|---:|---|---:|---|---|
| 1 | `cmd/ai/builders.go` | 25 | `tellurion/pkg/backtest/contracts` | `github.com/ruoxizhnya/tellurion-contracts/engine` |
| 2 | `pkg/ai/pipeline/backtest_runner_alias_test.go` | 25 | `tellurion/pkg/backtest/contracts` | `github.com/ruoxizhnya/tellurion-contracts/engine` |
| 3 | `pkg/ai/pipeline/pipeline.go` | 18 | `tellurion/pkg/backtest/contracts` | `github.com/ruoxizhnya/tellurion-contracts/engine` |
| 4 | `pkg/ai/tools/builtin/backtest.go` | 18 | `tellurion/pkg/backtest/contracts` | `github.com/ruoxizhnya/tellurion-contracts/engine` |
| 5 | `pkg/copilot/copilot.go` | 16 | `tellurion/pkg/backtest/contracts` | `github.com/ruoxizhnya/tellurion-contracts/engine` |

## 5. 清单 C · fees（3 位点 / 3 文件）

<!-- group tellurion/pkg/fees count=3 -->
| # | 文件 | 行 | 旧 import | 新 import |
|---:|---|---:|---|---|
| 1 | `pkg/ai/validation/economic_integration_test.go` | 29 | `tellurion/pkg/fees` | `github.com/ruoxizhnya/tellurion-contracts/fees` |
| 2 | `pkg/ai/validation/live_backtest_integration_test.go` | 35 | `tellurion/pkg/fees` | `github.com/ruoxizhnya/tellurion-contracts/fees` |
| 3 | `pkg/ai/yaml/generator.go` | 8 | `tellurion/pkg/fees` | `github.com/ruoxizhnya/tellurion-contracts/fees` |

## 6. 非 import、但依赖旧路径的落点（5 处 / 2 文件）

**这些不是 import，AST 换不到；漏掉不会在 agent 自己编不过时暴露，而会在 core 删掉 `pkg/domain` 之后以「夹具编不过 / 生成代码校验失败」的形式炸。**

| 文件 | 行 | 旧路径 | 形态 | B4 动作 |
|---|---:|---|---|---|
| `pkg/ai/validator/code_validator_test.go` | 57 | `tellurion/pkg/domain` | 测试夹具的 Go 源码字符串 | **必换**：换成 `...-contracts/domain`，否则 core 删旧包后 `TestValidateCompilation_CanResolveProjectImports` 编不过 |
| `pkg/ai/prompts.go` | 56 | `tellurion/pkg/domain` | `SystemPrompt` 标题 | **随 B4 换**：生成代码的 import 指示 |
| `pkg/ai/prompts.go` | 85 | `tellurion/pkg/domain` | `SystemPrompt` 的 import 指令 | **随 B4 换**：否则生成的策略仍 import 已删除的旧包 |
| `pkg/ai/prompts.go` | 106 | `tellurion/pkg/domain` | `SystemPrompt` 示例 import | **随 B4 换**（同上） |
| `pkg/copilot/copilot.go` | 103 | `tellurion/pkg/domain` | 注释 | 可选：无行为影响，建议随全替一起改 |

提示：`prompts.go` 里同时提到的 `tellurion/pkg/strategy` **不在本清单**（strategy 运行时留 core），不要一起换。
A4 的 dry-run 已用「全局文本替换」验证过：这 5 处一起换掉后，agent 全套测试仍绿（§8）——所以放进同一次替换是安全的；但提示词属于生成契约，B4 要**有意识地**改，而不是被 sed 顺手带走。

## 7. 前置与时序（硬）

1. **contracts 已推**：`fbe9a2d` 在 `origin/main`；**没有 tag**（S2 未做）⇒ B4 用 `go get github.com/ruoxizhnya/tellurion-contracts@<伪版本>` 固定，**不得加 `replace`**。
2. **core 腿必须先落**（`tellurion-contracts:docs/TASKS.md` 下游腿交接单 A-1..A-4）。A4 实测：只换 agent、core 未迁移时 `go build ./...` 必红，两处类型身份冲突：
   - `pkg/ai/tools/builtin/market_regime_tool.go:83`：`*risk.RiskManager` 的 `DetectRegime` 收 core `domain.OHLCV`，而本仓接口要 `contracts/domain.OHLCV` —— 同一个名字、两个包，Go 视为不同类型。
   - `pkg/ai/yaml/loader.go:247`：`buildSignalConfig` 构造的是 core `expression.SignalConfig`，它的 `Direction` 字段是 core `domain.Direction`；而 `dir` 已经是 `contracts/domain.Direction` —— 赋不进去。
   core 切完后两处自然消失（dry-run B 全绿），**不需要**在 agent 侧写类型转换。
3. **agent 的 `replace tellurion => ../tellurion` 保留**（S3 才删）；contracts 不走 replace。
4. 本地多仓联调用的 `go.work` **不入库**（`.gitignore` 已挡）。

## 8. A4 实测记录（真退出码）

| 证据 | 命令 | 结果 |
|---|---|---|
| 计数 | AST 扫 `*.go` 的 import | domain 46 / engine 5 / fees 3 ＝ **54 位点 / 48 文件**；`domain/market` 0 |
| 误报定位 | `git grep -l` vs AST 差集 | `pkg/ai/prompts.go`、`pkg/ai/validator/code_validator_test.go`（均为字符串） |
| dry-run A：只换 agent，core 未迁 | 临时副本 `go build ./...` | **exit 1**，两处类型冲突（§7-2） |
| dry-run B：core + agent 都迁 | 临时副本 `go build ./...` | core **exit 0**；agent **exit 0** |
| dry-run B：agent 全量门禁 | 临时副本 `go vet ./...` / `go test ./... -count=1` | **exit 0 / exit 0**（20 个包全 ok） |

两段 dry-run 都在工作区外的临时副本里做，未改动本仓任何文件。

## 9. B4 执行清单

1. `go get github.com/ruoxizhnya/tellurion-contracts@<伪版本>` + `go mod tidy`（要求 core 腿已落，见 §7）。
2. 按 §2 规则替换 §3~§5 的 **54 个 import 位点（48 文件）**。
3. 处理 §6 的 **5 处非 import 落点**（其中 4 处必换、1 处可选）。
4. `gofmt -w` 触碰过的文件；`gofmt -l .` 必须为空。
5. 验收（逐条真退出码）：
   - `git grep -nE '"tellurion/(pkg/domain|pkg/backtest/contracts|pkg/fees)"' -- '*.go'` → **0 行**（§6 也换完的前提下）。
   - `go list -m all | grep tellurion-contracts` → 真实伪版本，**不出现 `=>`**。
   - `go build ./...` / `go vet ./...` / `go test ./... -count=1` → 全 0。
   - CI 绿。
6. 收尾：删除 `internal/repoguard/import_inventory_test.go`；本文件标为已执行并移入 `docs/archive/`；在 `docs/TASKS.md` 记录 B4 完成（该单执行的是 contracts 交接单 B-1..B-3，本身没有独立单号）。

## 10. 机器护栏

`internal/repoguard/import_inventory_test.go` 把本文件的 54 行 import 清单、汇总计数与非 import 落点逐条钉在源码树上（AST 比对，行号精确到行）。它红的时候先看是哪种：

- 源码新增 / 删除了旧 import → 清单过期，重生成；
- B4 已做完 → 按 §9-6 退场。

**不要**为了让护栏变绿而改清单去迁就源码——清单是 B4 的输入，不是事后记录。
