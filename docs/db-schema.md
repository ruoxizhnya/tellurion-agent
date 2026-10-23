---
status: active
last-verified: 2026-10-23
verified-by: QT-17 ② —— aistore 自有 store 落地 + 双写期开始 + 读面切 ai.*
---

# DB Schema — tellurion-agent

> **所有权声明**：本仓**拥有 `ai` schema 的 DDL 与迁移**（目标态，QT-17）。

**现状（QT-17 ②，2026-10-23）**：`ai` schema 七张表的 DDL 与迁移入口在本仓 [`pkg/aistore`](../pkg/aistore)（逐列对照 core 定义搬入，`parent_id` 自引用与外键改指 `ai.*`）；自有 `Store`（`kit/db` 连接池 + 启动迁移 + 内置种子）已装配进 `cmd/ai`。**写路径处于双写期**：`experiments` 写路径经 `cmd/ai/experiment_dualwrite.go` 同时写 core `public.experiments`（legacy）与 `ai.experiments`（对拍期，停写待对拍零差异）；**读路径已切 `ai.*`**（research 域三表与 `factor_hypothesis`：core 侧 `research.*` 实测 0 行、`factor_hypothesis` 全为 builtin 种子且 agent 启动时镜像播种收敛，切读无数据损失；`experiments` 本仓只写不读，core 读方继续读旧表至停写切换）。core 侧 DDL 唯一在 `tellurion:pkg/storage/postgres.go` 的 `migrationStatements()`（`experiments` :424、`factor_hypothesis` :575、`factor_genes` :930、`strategy_genes` :954、研究域 :361-398）。
> 上级依据：`tellurion:docs/design/qt-decoupling/data-service-contract-and-ownership.md` §1 —— **DDL 只有一个 owner**，同一个 PG 实例、每服务一个 schema，将来拆库是"按 schema 搬走"而不是重写。
> 本文件是本仓表结构的**唯一真相源**：代码里的建表语句与它不一致时，**改代码**；要改表结构，先改本文件（结构级变更还要立 ADR）。

## 1. 归属表

### 目标：`ai` schema（QT-17 迁入）

| 表 | 内容 | 现状 |
|---|---|---|
| `ai.experiments` | 实验记录 | **双写期**：写路径经 dualwrite 同写 `ai.experiments` 与 core `public.experiments`（BIGSERIAL 各自发号，适配器内记 legacyID→aiID 映射）；本仓只写不读，core 读方仍读旧表 |
| `ai.factor_hypothesis` | 内置/人写因子假设 | **读写均已切**（对照 core `public.factor_hypothesis`）；builtin 种子启动时 `ON CONFLICT DO NOTHING` 播种 |
| `ai.factor_genes` | 因子基因池 | DDL 已落（对照 core `public.factor_genes`）；读写仍在 core（无写入方，见 TASKS 停写前检查项） |
| `ai.strategy_genes` | 策略基因池 | DDL 已落（对照 core `public.strategy_genes`）；读写仍在 core（无写入方，见 TASKS 停写前检查项） |
| `ai.research_profile` | 研究画像 | **读已切**（对照 core `research.profile`）；本仓只读不写，写方在 core（其研究域 handler 落旧表，core 侧停写是它的待办） |
| `ai.research_conclusion` | 研究结论 | **读已切**（对照 core `research.conclusion`）；本仓只读不写，写方在 core |
| `ai.research_question` | 研究问题 | **读已切**（对照 core `research.question`）；本仓只读不写，写方在 core |

> 迁入纪律（QT-17）：**以 core 现有 DDL 为准逐表搬**，不按类型名臆造表名；迁移后 core 侧定义标 `deprecated` 并登记删除期限。

## 2. 只读输入（不属于本仓，禁止写入）

| 数据 | 真实 owner |
|---|---|
| 行情 / 因子值 | `tellurion-data` |
| 回测结果 | core / `run` |
| 交易日历 / 上市窗口（仪器域） | core —— 经**只读 port** 取，不直连表 |
| 规则快照 | `tellurion-risk` |

**红线**：本仓生产代码不 import core 的 `pkg/storage`。过渡期白名单三条（`cmd/ai/main.go` / `cmd/ai/experiment_dualwrite.go` / `cmd/ai/handlers_explore.go`）登记在 `internal/repoguard` 的 `storageWhitelist`，每条带理由与过期条件，由 AST 护栏钉住（条目失效即红）。

## 3. 红线

1. **实验域数据自持**：写 `ai.*`，不写 core 的表（双写期例外：`experiments` 写路径经 dualwrite 同写 legacy 旧表，停写后收回）。
2. **不 import core 实现包**（`pkg/storage` / `pkg/backtest` / `pkg/strategy`）；跨边界只走 contracts。
3. **逐 bar / 逐笔零远程**：回测循环里出现 HTTP 就是错的（当前引擎在进程内，这点天然成立）。
4. **双写期必须有对拍**：`ai.*` 与旧表行数 / 校验和一致后才切读，切读后才停写。
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

## 5. 待定

- 双写期长度未定（QT-17 ④）：对拍零差异 → 切读（experiments 的读方在 core）→ 停写。
- **停写前检查项**：`factor_genes` / `strategy_genes` 当前无写入方（真库 0 行）；将来接写时必须直接写 `ai.*`，不得经 core 旧表。
- 研究域三表的写入方在 core（本仓只读）；core 侧停写切 `ai.*` 归 core 的待办，不在本仓台账。
- 76 处其余 core 实现包 import 归 S3（依赖 QT-50 的 HTTP 粗粒度），见 `docs/TASKS.md` QT-17 的 S3 前置状态。
