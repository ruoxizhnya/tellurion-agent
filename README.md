# tellurion-agent

> 5 分钟了解本服务：[SERVICE.md](SERVICE.md)

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
| `pkg/ai/` | AI 编排层（loop / pipeline / gene_pool / tools / agents / causal / …，20 包 = 根包 + 19 子包）——**自 core 仓迁入（2026-10-09，AI 拆仓阶段 2）** |
| `pkg/copilot/` | `CopilotService`（自然语言 → 生成代码 → 沙箱编译 → 回测）——自 core 的 `pkg/strategy/copilot.go` 归位 |
| `cmd/ai/` | **ai-service**（:8086）：Copilot / pipeline / explore / tools 四族 HTTP 面 + AI 侧装配 builder |
| `config/ai-service.yaml` | 本仓自持的 ai-service 配置（内容取自 core，只读复制） |
| `deploy/compose.yaml` | 单仓本地栈：本仓自带 PG（**无 Redis**：代码零读取点），ai-service 发布 `127.0.0.1:8086` |
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
AUTH_INSECURE=true AUTH_INSECURE_EXPOSURE=loopback-published SERVER_HOST=127.0.0.1 \
  go run ./cmd/ai

# 鉴权三态（详见 config/ai-service.yaml 注释与 docs/api-schema.md §1.0）：
#   - 配 JWT_SECRET             → 全闸；
#   - 只 AUTH_INSECURE=true     → 进程能起，但写面/工具执行 503 auth_unavailable，
#                                 读面公开；
#   - 再加 AUTH_INSECURE_EXPOSURE=loopback-published → 显式放开（本地/回环发布）。
# 非回环绑定 + 无 secret + 无声明会直接拒绝启动。
#
# 第 3 态的启动 WARN 会插值实际绑定地址；非回环（含空 host）会点名「已对外暴露」：
#   回环：auth: INSECURE open-access mode — 写面无鉴权：write endpoints and tool execution
#         accept unauthenticated requests; bind address "127.0.0.1:8086" is loopback;
#         AUTH_INSECURE_EXPOSURE=loopback-published is a declaration of the publish layer,
#         not a runtime guarantee
#   容器：auth: INSECURE open-access mode — 写面无鉴权 且已对外暴露：write endpoints and tool
#         execution accept unauthenticated requests; bind address "0.0.0.0:8086" is
#         non-loopback and 已对外暴露; AUTH_INSECURE_EXPOSURE=loopback-published is a
#         declaration of the publish layer, not a runtime guarantee
#   空 host（:8086，监听全部接口）与容器同口径，措辞用 ":8086"。

# 若本机 PG 用 scram-sha-256（宿主默认即是），上面的 DATABASE_PASSWORD 必须
# 是真实口令；不想用无鉴权模式就改注入 JWT_SECRET=<至少 32 字节随机值>。

# 或容器（单仓 compose；自带 PG、无 Redis，ai-service 发布 127.0.0.1:8086）
docker compose -f deploy/compose.yaml up --build
```

服务间地址默认是**宿主机视角**（`data_service.url` / `ai_service.factor_api_url` /
`strategy_asset.url` 都是 `http://127.0.0.1:<port>`），容器里由
`deploy/compose.yaml` 的 `DATA_SERVICE_URL` / `AI_SERVICE_FACTOR_API_URL` /
`STRATEGY_ASSET_URL` 覆盖成服务名。启动期会逐项解析这些主机名
（[cmd/ai/endpoints.go](file:///c:/Users/ruoxi/Workspace/tellurion-agent/cmd/ai/endpoints.go)）：
宿主机上误留容器 DNS 会在启动时报错并点名该注入哪个 env，不会拖到第一次
工具调用才 404。探针：`/health`（存活，不碰库）与 `/readyz`（就绪，校验
DB ping + `ai.*` 迁移关键表）。

```bash
curl -sS http://127.0.0.1:8086/health
curl -sS http://127.0.0.1:8086/readyz
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
