---
status: active
last-verified: 2026-10-20
verified-by: 功能边界固化（三份契约：db / api / dataflow）
---

# DB Schema — tellurion-agent

> **所有权声明**：本仓**拥有 `ai` schema 的 DDL 与迁移**（目标态，QT-17）。

**现状**：实验 / 研究 / 基因数据仍住在 core 的 `quant` schema（表由 core `pkg/storage/experiments.go`、`research.go` 的内联 DDL 定义，`factor_genes` / `strategy_genes` 见 core 的 `migrations/023_add_gene_pool_tables.sql`）。本仓当前通过 core 的 `*PostgresStore` 读写 —— 这是**要拆掉的耦合**。
> 上级依据：`tellurion:docs/design/qt-decoupling/data-service-contract-and-ownership.md` §1 —— **DDL 只有一个 owner**，同一个 PG 实例、每服务一个 schema，将来拆库是"按 schema 搬走"而不是重写。
> 本文件是本仓表结构的**唯一真相源**：代码里的建表语句与它不一致时，**改代码**；要改表结构，先改本文件（结构级变更还要立 ADR）。

## 1. 归属表

### 目标：`ai` schema（QT-17 迁入）

| 表 | 内容 | 现状 |
|---|---|---|
| `ai.experiments` | 实验记录 | 现住 core `quant.experiments`（`pkg/storage/postgres.go:424`） |
| `ai.factor_genes` | 因子基因池 | 现住 core（`migrations/023_add_gene_pool_tables.sql`） |
| `ai.strategy_genes` | 策略基因池 | 同上 |
| `ai.research_*` | 研究域（profile / question / conclusion） | 现住 core `pkg/storage/research.go` —— **表名以 core 现有定义为唯一来源**，迁移时逐表核对，本文不预写 |

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

- 迁移文件只放 `migrations/`，命名 `<序号>_<动作>.sql`，**本仓内**递增（不与别的仓共享序列，也不复用号）。
- **执行机制随第一个建表单定，二选一（不能"都不选"）**：
  1. **core 式**：Go 里的 `migrationStatements()` 数组 + 启动执行，幂等、无版本表——与 core 一致、运维零新增；
  2. **版本化**：引入迁移库 + `schema_migrations` 表（可 down、有执行记录）——服务变多后更稳，但要一次性 baseline。
  **取向**：X1~X3 期间优先 ①（少一层依赖），服务数 ≥3 或出现"必须先降级"的需求时重估 ②。
- **只前滚**：已合并的迁移不改；修错靠新增一条。
- 每条必须可重入（`IF NOT EXISTS` / `IF EXISTS`），且能在空库上一次跑完。
- 跨 schema 取数只走**只读视图或 API**；出现 `join` 别人的表就是边界腐化的第一信号。
- 结构变更的同步义务：本文件 + `docs/ARCHITECTURE.md` 的「数据归属」节必须同一次提交里更新。

## 5. 待定

- 双写期长度未定（QT-17 ④）。
- `research_*` 的确切表名与列，需在 QT-17 开工时从 core `pkg/storage/research.go` 读取后回填本文件。
