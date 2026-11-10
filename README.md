# tellurion-agent

Quant Lab 的 **AI 实验员层**。从 monorepo `tellurion` 拆出（2026-10-09）。

## 定位与边界

本仓含 **AI 实验员**领域的全部代码与文档。后端内核（回测引擎、策略运行时、表达式 DSL、
数据供给）在 [`tellurion`](https://github.com/ruoxizhnya/tellurion)（core，
本仓**单向依赖**它），前端在 [`tellurion-ui`](https://github.com/ruoxizhnya/tellurion-ui)，
跨领域文档在 [`tellurion-docs`](https://github.com/ruoxizhnya/tellurion-docs)。

切分判据是 **DDD**：「怎么算」= core（仪器）；「试什么 / 怎么试 / 试完怎么判」= AI（实验员）。

## 目录

| 路径 | 内容 |
|---|---|
| `pkg/ai/` | AI 编排层（loop / pipeline / gene_pool / tools / agents / causal / …，24 包）——**自 core 仓迁入（2026-10-09，AI 拆仓阶段 2）** |
| `pkg/copilot/` | `CopilotService`（自然语言 → 生成代码 → 沙箱编译 → 回测）——自 core 的 `pkg/strategy/copilot.go` 归位 |
| `cmd/ai/` | **ai-service**（:8086）：Copilot / pipeline / explore / tools 四族 HTTP 面 + AI 侧装配 builder |
| `config/ai-service.yaml` | 本仓自持的 ai-service 配置（内容取自 core，只读复制） |
| `deploy/compose.yaml` | 单仓本地栈：本仓自带 PG / Redis，ai-service 发布 `127.0.0.1:8086` |
| `deploy/wait-for-deps.sh` | 容器启动的有界依赖等待（取自 core，需 `nc`） |
| `docs/hermes/` | Hermes agent 的配置 / 提示词 / 技能 / 系统设计 / 工具契约 |

## 依赖方向（结构性不变量）

```
agent ──HTTP──► core（tellurion；运行期调用，零编译依赖 / 无 replace）
core ────────────► ✗ 永不反向依赖 agent
```

core 仓的 `internal/repoguard` 有护栏钉住这个方向：core 里**不许出现 `pkg/ai` 目录**、
生产代码不许 import `pkg/ai` 或 agent 模块。反向边 = 重新制造仓级环 = Go module 直接拒绝编译。
（2026-10-09 四族 handler 先进独立进程、analysis 改反代；QT-17 S3 删除
`replace tellurion => ../tellurion` 后，本仓编译不再需要 core checkout。）

## 构建与测试

```bash
go build ./... && go vet ./... && go test ./... -count=1
```

CI（`.github/workflows/ci.yml`）单仓 checkout（不拉 core、不改 replace）。

## 启动 ai-service

```bash
# 本机直跑（需可达的 PG；配置默认读 config/ai-service.yaml）
# 口令只从 env 注入（config/ai-service.yaml 的 database.password 恒为空，
# 空口令会在启动期被 kitdb.BuildDSN 拒绝，而不是连库时才报认证失败）：
CONFIG_PATH=config/ai-service.yaml \
DATABASE_HOST=127.0.0.1 DATABASE_PASSWORD='<本机 postgres 用户的口令>' \
AUTH_INSECURE=true SERVER_HOST=127.0.0.1 \
  go run ./cmd/ai

# 若本机 PG 用 scram-sha-256（宿主默认即是），上面的 DATABASE_PASSWORD 必须
# 是真实口令；不想用无鉴权模式就改注入 JWT_SECRET=<至少 32 字节随机值>。

# 或容器（单仓 compose；自带 PG / Redis，ai-service 发布 127.0.0.1:8086）
docker compose -f deploy/compose.yaml up --build
```

## ai-service 的镜像构建

单仓构建即可（go.mod 已无 replace，不再需要 core 作为命名构建上下文）：

```bash
docker build -t ai-service .
```

ai-service 在 compose 里发布 `127.0.0.1:8086`，analysis-service 对
`/api/{copilot,pipeline,explore,tools}` 做纯反代 —— 前端与 openapi 契约不感知拆仓。

## 文档入口

- 文档导航：`docs/README.md`
- 本仓入口（构建/边界/禁止）：`AGENTS.md`
- 本仓台账（唯一）：`docs/TASKS.md`
- 边界与依赖方向：`docs/ARCHITECTURE.md`
