# AGENTS — tellurion-agent

> 本文件是**本仓**的唯一入口。判据：一个 agent 只 checkout 本仓，就知道「怎么构建、边界在哪、依赖哪版契约、活在哪、禁止什么」。

## 1. 这是什么仓

**AI 实验员层**：实验循环（loop）、基因池、验证与因果判定、工具编排、Copilot（自然语言 → 代码 → 沙箱编译 → **promote 人工闸门**；AI 产物没过闸门不进回测，QT-32-A），以及 `ai.*` schema 的数据自持（QT-17）。

切分判据是 DDD：「怎么算」= core（仪器）；「试什么 / 怎么试 / 试完怎么判」= 本仓（实验员）。

## 2. 构建 / 测试

```bash
go build ./...
go vet ./...
go test ./...
go test ./cmd/ai/... -race -count=1     # 端点/SSE 这类并发面必带 -race
```

✅ **本仓单仓可构建（QT-17 S3-d，2026-10-25）**：`go.mod` 已无 `tellurion` require / `replace`，CI 只 checkout 本仓。R1/R2 成立：单独 checkout 跑上面三条即绿。

- 不要再加任何仓外 `replace`；
- 不要把 core 的实现抄进来"消除依赖"（那是把边界搞坏，不是修好）；
- 需要 core 的数据 / 计算 → 走它已登记的 HTTP 粗粒度端点（见 `docs/dataflow.md`）。

## 3. 边界（硬约束）

**允许**

- 实验循环 / 基因池 / 验证 / 工具编排 / Copilot
- `ai.*` schema 的 DDL 与迁移（QT-17）
- 通过**只读 HTTP** 取仪器域数据：交易日历走 tellurion-data 的 `GET /api/v1/trading/calendar`（`pkg/ai/client/reference_data_client.go`，契约见 data 仓 `docs/api-schema.md`）；上市窗口**今天没有任何服务提供**（core QT-67 已随 `SVC-CORE-DATA-RETIRE` 退役），重开条件见 tellurion-data 仓 `TASKS.md` 的 ⏸ 条目；护栏 `internal/repoguard/storage_import_test.go` 的仪器域白名单仍为零

**禁止**

- import core 的**实现包**（`pkg/storage` / `pkg/backtest` / `pkg/strategy` …）——契约走 `tellurion-contracts`
- 逐 bar / 逐笔的远程调用（回测循环里出现 HTTP 就是错的）
- 直连 core 的业务表（实验域数据必须写自己的 `ai.*`）
- 提交 `go.work` / `go.work.sum`

## 4. 依赖的契约版本

| 依赖 | 版本 | 说明 |
|---|---|---|
| `github.com/ruoxizhnya/tellurion-contracts` | `v0.9.0` | 2026-11-07 发版序列（CT-03 `risk/` / CT-04 `run/` / CT-05 `broker/` 三个新增包），tag 与 `main` 于 2026-11-08 推远端；**本仓 2026-11-08 按裁决 #6 切到 `v0.9.0`**（裁决原文写 `v0.7.0`，按「取最新兼容序列」前移，待主任追认） |
| `github.com/ruoxizhnya/tellurion-kit` | `v0.2.0` | 2026-10-25 发版（KT-11：`role=service` 的非数字 subject 不再误判 401）；`pkg/aistore` 用 `kit/db` 的迁移机制（QT-17 ①），连接池由调用方持有；**本仓 2026-11-08 由 `v0.1.0` 升到 `v0.1.4`**（裁决 #6），**2026-11-10 随 kit `KT-13` 升到 `v0.2.0`** |
| `github.com/jackc/pgx/v5` | `v5.7.3` | `pkg/aistore` 直连 PG（连接池 / `ErrNoRows` 哨兵，QT-17 ①②）；随 `kit/db` 同版本，未单独升级 |

> **契约一致性（QT-09 ③）**：`internal/conformance` 读 `tellurion-contracts:conformance/cases.json`（三仓唯一真相）实算——`fee_defaults_via_yaml` 钉住生成器的 `commission_rate`/`slippage_rate` 必须等于 `fees.DefaultCommissionRate`/`DefaultSlippageRate`（历史漂移点：滑点写成 0.001、佣金写成 0.00025）。语料定位失败一律 `t.Fatalf`，不许 `t.Skip`。core 侧编排：`cd ../tellurion && make contracts-conformance`。

## 5. 活在哪、文档在哪

- **本仓台账**：`docs/TASKS.md`（唯一；OPS-01-A 等历史单在这里）。**没有单号别开工。**
- 文档导航：`docs/README.md`；边界与依赖：`docs/ARCHITECTURE.md`；决策：`docs/adr/`。
- Hermes agent 的配置/提示词/技能/工具契约：`docs/hermes/`（属本仓资产，改动同样要立单）。
- 跨仓背景（只读）：`tellurion:docs/TASKS.md`、`tellurion:docs/design/qt-decoupling/repo-topology-and-naming.md`。

> 跨仓引用一律写 `repo:path` 纯文本，**不做可点击相对链接**。

## 6. 文档真相源（改功能前先看这里）

本仓的「功能边界」由三份文件钉住，**它们是唯一真相源，不是参考读物**：

| 维度 | 文件 |
|---|---|
| 数据形状（我拥有哪些表、谁能写） | `docs/db-schema.md` |
| 接口形状（对外暴露什么、错误码、幂等） | `docs/api-schema.md` |
| 流向与顺序（数据怎么走、哪里 fail-closed） | `docs/dataflow.md` |

**变更触发是硬约束**（完整表见 `docs/README.md` §3）：改表 → 改 `db-schema.md`；改接口 → 改 `api-schema.md`；改链路 → 改 `dataflow.md`；改分层 → 改 `ARCHITECTURE.md` + 一条 ADR；删档/迁移/换工具 → 一条 ODR。PR 里代码动了而对应文档没动，应被打回。
