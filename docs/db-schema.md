---
status: active
last-verified: 2026-10-23
verified-by: QT-17 ① —— ai.* DDL 与迁移机制落 pkg/aistore（逐表对照 core 现有定义）
---

# DB Schema — tellurion-agent

> **所有权声明**：本仓**拥有 `ai` schema 的 DDL 与迁移**（目标态，QT-17）。

**现状**：`ai` schema 的 DDL 与迁移入口已落本仓 [`pkg/aistore`](../pkg/aistore)（QT-17 ①，2026-10-23）——六张表逐列对照 core 定义搬入，`parent_id` 自引用与外键改指 `ai.*`。**读写路径仍走 core 的 `*PostgresStore`**（`experiments` / `factor_genes` / `strategy_genes` 在 core `public`，研究域三表在 core `research` schema）——数据归属切换（QT-17 ②：自有 store + 双写对拍 + 切读 + 停写）尚未开始。core 侧 DDL 唯一在 `tellurion:pkg/storage/postgres.go` 的 `migrationStatements()`（`experiments` :424、`factor_genes` :930、`strategy_genes` :954、研究域 :361-398）。
> 上级依据：`tellurion:docs/design/qt-decoupling/data-service-contract-and-ownership.md` §1 —— **DDL 只有一个 owner**，同一个 PG 实例、每服务一个 schema，将来拆库是"按 schema 搬走"而不是重写。
> 本文件是本仓表结构的**唯一真相源**：代码里的建表语句与它不一致时，**改代码**；要改表结构，先改本文件（结构级变更还要立 ADR）。

## 1. 归属表

### 目标：`ai` schema（QT-17 迁入）

| 表 | 内容 | 现状 |
|---|---|---|
| `ai.experiments` | 实验记录 | DDL 已落 `pkg/aistore/schema.go`（列对照 core `public.experiments`）；读写仍在 core |
| `ai.factor_genes` | 因子基因池 | DDL 已落（对照 core `public.factor_genes`）；读写仍在 core |
| `ai.strategy_genes` | 策略基因池 | DDL 已落（对照 core `public.strategy_genes`）；读写仍在 core |
| `ai.research_profile` | 研究画像 | DDL 已落（对照 core `research.profile`）；读写仍在 core |
| `ai.research_conclusion` | 研究结论 | DDL 已落（对照 core `research.conclusion`）；读写仍在 core |
| `ai.research_question` | 研究问题 | DDL 已落（对照 core `research.question`）；读写仍在 core |

> 迁入纪律（QT-17）：**以 core 现有 DDL 为准逐表搬**，不按类型名臆造表名；迁移后 core 侧定义标 `deprecated` 并登记删除期限。

## 2. 只读输入（不属于本仓，禁止写入）

| 数据 | 真实 owner |
|---|---|
| 行情 / 因子值 | `tellurion-data` |
| 回测结果 | core / `run` |
| 交易日历 / 上市窗口（仪器域） | core —— 经**只读 port** 取，不直连表 |
| 规则快照 | `tellurion-risk` |

**红线**：本仓不 import core 的 `pkg/storage`，不直连 core 的业务表。

## 3. 红线

1. **实验域数据自持**：写 `ai.*`，不写 core 的表（QT-17 完成后）。
2. **不 import core 实现包**（`pkg/storage` / `pkg/backtest` / `pkg/strategy`）；跨边界只走 contracts。
3. **逐 bar / 逐笔零远程**：回测循环里出现 HTTP 就是错的（当前引擎在进程内，这点天然成立）。
4. **双写期必须有对拍**：`ai.*` 与旧表行数 / 校验和一致后才切读，切读后才停写。
5. 实验记录必须能回答"用哪份策略 / 数据 / 规则"（三 hash，X0）。

## 4. 建表与迁移

**现状（必须先知道）**：core 的迁移是「**启动时按序执行一组幂等 DDL**」——唯一真相是 `pkg/storage/postgres.go` 的 `migrationStatements()` 数组（`migrate()` 在服务启动时逐条跑）；**没有版本表、没有 down、没有执行记录**（有意取舍，理由见 core 的 `migrations/README.md`）。`migrations/`（14 个）与 `docs/migrations/`（25 个）里的 `.sql` 是**历史镜像，不执行**——改它们不会改库。

**本仓约定**（从建仓第一天执行）：

- **执行机制（已定，2026-10-23）**：选 ①「core 式」——`pkg/aistore/schema.go` 的 `statements` 数组是 DDL 唯一真相，经 `kit/db.Migrate` 按序执行，幂等、无版本表、无 down。**不设 `migrations/*.sql` 镜像**（双份必然漂移）；历史镜像若将来需要，只能由数组机械生成。**启动接线随 QT-17 ② 的自有连接池一起落**——今天连接池仍由 core 的 `bootstrap.InitStore` 持有，本仓不借它的句柄跑迁移（D10：连接能力用 kit，数据归属自持）。
- 每条必须可重入（`IF NOT EXISTS`），且能在空库上一次跑完；`TestStatements_AreIdempotent` 与 `TestMigrate_Integration` 钉住这两点。
- 表名 / 列必须与 `tellurion:pkg/storage/postgres.go` 逐表一致；`TestSchema_ColumnsMatchCore` 直接读 core 源码对拍，多一列或少一列都红。
- 只前滚：已合并的迁移不改；修错靠新增一条。
- 跨 schema 取数只走**只读视图或 API**；出现 `join` 别人的表就是边界腐化的第一信号。
- 结构变更的同步义务：本文件 + `docs/ARCHITECTURE.md` 的「数据归属」节必须同一次提交里更新。

## 5. 待定

- 双写期长度未定（QT-17 ④）。
- 读写路径切换（自有 store 替换 12 处 `*PostgresStore` 用法）与双写对拍尚未开始（QT-17 ②④）。
