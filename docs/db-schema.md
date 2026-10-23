---
status: active
last-verified: 2026-10-24
verified-by: QT-17 ④ —— 停写 legacy public.experiments（写路径只剩 ai.experiments）+ 30 行历史回填归档，行级对拍 0 差异
---

# DB Schema — tellurion-agent

> **所有权声明**：本仓**拥有 `ai` schema 的 DDL 与迁移**（目标态，QT-17）。

**现状（QT-17 ④，2026-10-24）**：`ai` schema 七张表的 DDL 与迁移入口在本仓 [`pkg/aistore`](../pkg/aistore)（逐列对照 core 定义搬入，`parent_id` 自引用与外键改指 `ai.*`）；自有 `Store`（`kit/db` 连接池 + 启动迁移 + 内置种子）已装配进 `cmd/ai`。**写路径已停写 legacy**（QT-17 ④）：`cmd/ai/main.go` 直接把 `*aistore.Store` 注入 pipeline / explore 的 sink，双写适配器 `cmd/ai/experiment_dualwrite.go` 已整体删除；`public.experiments` 冻结在 30 行（2026-10-24 一次性回填归档进 `ai.experiments`，行级 `EXCEPT` 双向对拍 0 差异）。**读路径已切 `ai.*`**（research 域三表与 `factor_hypothesis`）。core 侧 DDL 唯一在 `tellurion:pkg/storage/postgres.go` 的 `migrationStatements()`（`experiments` :424、`factor_hypothesis` :575、`factor_genes` :930、`strategy_genes` :954、研究域 :361-398）——**旧表只保留不删**，core 侧标 deprecated 与删除期限见 `docs/TASKS.md` 的交接件（属 core 的待办，本仓不改）。
> 上级依据：`tellurion:docs/design/qt-decoupling/data-service-contract-and-ownership.md` §1 —— **DDL 只有一个 owner**，同一个 PG 实例、每服务一个 schema，将来拆库是"按 schema 搬走"而不是重写。
> 本文件是本仓表结构的**唯一真相源**：代码里的建表语句与它不一致时，**改代码**；要改表结构，先改本文件（结构级变更还要立 ADR）。

## 1. 归属表

### 目标：`ai` schema（QT-17 迁入）

| 表 | 内容 | 现状 |
|---|---|---|
| `ai.experiments` | 实验记录 | **单写已切（QT-17 ④）**：写路径只剩 `ai.experiments`；旧表 `public.experiments` 停写、冻结 30 行（历史行已于 2026-10-24 回填归档，行级对拍 0 差异）。本仓只写不读 |
| `ai.factor_hypothesis` | 内置/人写因子假设 | **读写均已切**（对照 core `public.factor_hypothesis`）；builtin 种子启动时 `ON CONFLICT DO NOTHING` 播种 |
| `ai.factor_genes` | 因子基因池 | DDL 已落（对照 core `public.factor_genes`）；**读写尚未切**——`pkg/ai/gene_pool` 现以未限定表名写 core `public.factor_genes`（见 §5 待定 1） |
| `ai.strategy_genes` | 策略基因池 | DDL 已落（对照 core `public.strategy_genes`）；**读写尚未切**——同 `factor_genes` |
| `ai.research_profile` | 研究画像 | **读已切**（对照 core `research.profile`）；本仓只读不写，**全工作区不存在 `research.*` 的写入方**（2026-10-24 实测：core 侧无研究域 handler，真库 0 行） |
| `ai.research_conclusion` | 研究结论 | **读已切**（对照 core `research.conclusion`）；本仓只读不写，写方不存在 |
| `ai.research_question` | 研究问题 | **读已切**（对照 core `research.question`）；本仓只读不写，写方不存在 |

> 迁入纪律（QT-17）：**以 core 现有 DDL 为准逐表搬**，不按类型名臆造表名；迁移后 core 侧定义标 `deprecated` 并登记删除期限。

## 2. 只读输入（不属于本仓，禁止写入）

| 数据 | 真实 owner |
|---|---|
| 行情 / 因子值 | `tellurion-data` |
| 回测结果 | core / `run` |
| 交易日历 / 上市窗口（仪器域） | core —— 经**只读 port** 取，不直连表 |
| 规则快照 | `tellurion-risk` |

**红线**：本仓生产代码不 import core 的 `pkg/storage`。停写后过渡期白名单**只剩一条**（`cmd/ai/handlers_explore.go` 的 `ListingWindow` 仪器域读），登记在 `internal/repoguard` 的 `storageWhitelist`，带理由与过期条件，由 AST 护栏钉住（条目失效即红）。原先的 `cmd/ai/main.go`（改走 `kit/db.BuildDSN`）与 `cmd/ai/experiment_dualwrite.go`（随停写删除）两条已收回。

## 3. 红线

1. **实验域数据自持**：写 `ai.*`，不写 core 的表（双写期已于 2026-10-24 结束，例外收回）。
2. **不 import core 实现包**（`pkg/storage` / `pkg/backtest` / `pkg/strategy`）；跨边界只走 contracts。
3. **逐 bar / 逐笔零远程**：回测循环里出现 HTTP 就是错的（当前引擎在进程内，这点天然成立）。
4. **停写后旧表零增长**：`public.experiments` 行数与序列不再前进，由 `cmd/ai` 的真库腿 `TestStopWrite_LegacyExperimentsFrozen` 与 `internal/repoguard` 的 import 护栏共同钉住。
5. 实验记录必须能回答"用哪份策略 / 数据 / 规则"（三 hash，X0）。

## 4. 建表与迁移

**现状（必须先知道）**：core 的迁移是「**启动时按序执行一组幂等 DDL**」——唯一真相是 `pkg/storage/postgres.go` 的 `migrationStatements()` 数组（`migrate()` 在服务启动时逐条跑）；**没有版本表、没有 down、没有执行记录**（有意取舍，理由见 core 的 `migrations/README.md`）。`migrations/`（14 个）与 `docs/migrations/`（25 个）里的 `.sql` 是**历史镜像，不执行**——改它们不会改库。

**本仓约定**（从建仓第一天执行）：

- **执行机制（已定，2026-10-23）**：选 ①「core 式」——`pkg/aistore/schema.go` 的 `statements` 数组是 DDL 唯一真相，经 `kit/db.Migrate` 按序执行，幂等、无版本表、无 down。**不设 `migrations/*.sql` 镜像**（双份必然漂移）；历史镜像若将来需要，只能由数组机械生成。**启动接线已落（QT-17 ②）**：`cmd/ai` 用自有 `kit/db` 连接池装配 `aistore.NewStore`（连接 + 迁移 + 内置种子一步到位），与 core 的 `bootstrap.InitStore` 并行、互不借句柄（D10：连接能力用 kit，数据归属自持）。
- 每条必须可重入（`IF NOT EXISTS`），且能在空库上一次跑完；`TestStatements_AreIdempotent` 与 `TestMigrate_Integration` 钉住这两点。
- 表名 / 列必须与 `tellurion:pkg/storage/postgres.go` 逐表一致；`TestSchema_ColumnsMatchCore` 直接读 core 源码对拍，多一列或少一列都红。
- 只前滚：已合并的迁移不改；修错靠新增一条。
- 跨 schema 取数只走**只读视图或 API**；出现 `join` 别人的表就是边界腐化的第一信号。
- 结构变更的同步义务：本文件 + `docs/ARCHITECTURE.md` 的「数据归属」节必须同一次提交里更新。

## 5. 待定 / 已知未落项（如实登记，不写成已完成）

1. **`factor_genes` / `strategy_genes` 仍写 core 旧表（2026-10-24 实测，本次新发现）**：`pkg/ai/gene_pool` 的 SQL 用**未限定表名**（`INSERT INTO factor_genes …`），按 `search_path`（`"$user", public`）解析到 `public.factor_genes` / `public.strategy_genes`；`cmd/ai/main.go` 又把 core 的 `store.DB()` 池注入两个 pool。真库当前两表各 0 行（`save_factor` / `save_strategy` 尚未被调过），所以尚未产生脏数据。**处置**：切换到 `ai.*` 属独立一小步（要先把池换成自有池并给 SQL 加 schema 限定 + 补对拍），不在 QT-17 ④ 的停写范围内，**未做**。下一次接写前必须先落这一步，否则会持续写别人的表。
2. **`research.*` 的「停写」是无操作**：全工作区不存在 `research.profile/conclusion/question` 的生产写入方（core 的 `pkg/storage/research.go` 只有读方法 `GetResearchProfile`，`tellurion:cmd/analysis` 无研究域 handler），真库 0 行。此前本文与 `docs/TASKS.md` 写的「写方在 core / core 侧停写是它的待办」**与代码事实不符**，已在本版订正。因此这三表不存在「停写」动作，只存在「将来接写时直写 `ai.*`」的约束。
3. **`ai.research_*` 目前也没有写入方**：本仓只有 `aistore.GetResearchProfile`（读）；`research.profile` 工具的降级回退是 vault 镜像，不是写库。将来接写时直接写 `ai.*`。
4. **core 侧 deprecated 标记与删除期限**：属 core 仓待办，本仓**不改** core。交接件见 `docs/TASKS.md` QT-17 的「core 侧交接件」小节（含要标记的符号清单与建议删除期限 2027-01-31，需协调者/core 确认）。
5. **`grep -r "pkg/storage"` 尚未归零**：生产代码只剩 `cmd/ai/handlers_explore.go`（仪器域 `ListingWindow`，等 D24 第 7 类动作）；其余命中在 `_test.go`、文档与护栏注释里。整体归零依赖 S3 删 `replace`（QT-50），见 `docs/TASKS.md` QT-17 的 S3 前置状态。
