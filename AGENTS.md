# AGENTS — tellurion-agent

> 本文件是**本仓**的唯一入口。判据：一个 agent 只 checkout 本仓，就知道「怎么构建、边界在哪、依赖哪版契约、活在哪、禁止什么」。

## 1. 这是什么仓

**AI 实验员层**：实验循环（loop）、基因池、验证与因果判定、工具编排、Copilot（自然语言 → 代码 → 沙箱编译 → 回测），以及 `ai.*` schema 的数据自持（QT-17）。

切分判据是 DDD：「怎么算」= core（仪器）；「试什么 / 怎么试 / 试完怎么判」= 本仓（实验员）。

## 2. 构建 / 测试

```bash
go build ./...
go vet ./...
go test ./...
go test ./cmd/ai/... -race -count=1     # 端点/SSE 这类并发面必带 -race
```

⚠️ **本仓目前不是单仓可构建（known-gap，对应 QT-07 的 S3 段）**：`go.mod` 里有 `replace tellurion => ../tellurion`，CI 也靠 checkout core 再改写 replace。这是**过渡期的唯一例外**，不是惯例：

- 不要在 `../tellurion` 之外再加任何仓外 `replace`；
- 不要把实现从 core 抄进来"消除依赖"（那是把边界搞坏，不是修好）；
- S3 的判据是：**删掉这个 replace 后 `go build ./...` 仍然绿**（依赖 QT-50 的 HTTP 粗粒度 + QT-17 的数据自持）。

## 3. 边界（硬约束）

**允许**

- 实验循环 / 基因池 / 验证 / 工具编排 / Copilot
- `ai.*` schema 的 DDL 与迁移（QT-17）
- 通过 core 的**只读 port** 取仪器域数据（交易日历 / 上市窗口，待 D24 补第 7 类动作）

**禁止**

- import core 的**实现包**（`pkg/storage` / `pkg/backtest` / `pkg/strategy` …）——契约走 `tellurion-contracts`
- 逐 bar / 逐笔的远程调用（回测循环里出现 HTTP 就是错的）
- 直连 core 的业务表（实验域数据必须写自己的 `ai.*`）
- 提交 `go.work` / `go.work.sum`

## 4. 依赖的契约版本

| 依赖 | 版本 | 说明 |
|---|---|---|
| `tellurion`（core） | `v0.1.0` + `replace ../tellurion` | **过渡期**，S3 拆掉；拆掉前本仓 R1 不成立 |
| `github.com/ruoxizhnya/tellurion-contracts` | 待定（S2 后 `v0.1.0`） | 契约只认版本号 |
| `github.com/ruoxizhnya/tellurion-kit` | 待定（S2 后 `v0.1.0`） | 同上 |

## 5. 活在哪、文档在哪

- **本仓台账**：`docs/TASKS.md`（唯一；OPS-01-A 等历史单在这里）。**没有单号别开工。**
- 文档导航：`docs/README.md`；边界与依赖：`docs/ARCHITECTURE.md`；决策：`docs/adr/`。
- Hermes agent 的配置/提示词/技能/工具契约：`docs/hermes/`（属本仓资产，改动同样要立单）。
- 跨仓背景（只读）：`tellurion:docs/TASKS.md`、`tellurion:docs/design/qt-decoupling/repo-topology-and-naming.md`。

> 跨仓引用一律写 `repo:path` 纯文本，**不做可点击相对链接**。
