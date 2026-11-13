# ai-service runbook（tellurion-agent）

> 适用对象：`cmd/ai`（ai-service，默认发布到 `127.0.0.1:8086`）。
> 本文只讲**起停**与**失败排查**；接口参数、响应字段、错误码见
> [`docs/api-schema.md`](api-schema.md)，不在此重复。
> 所有命令均按当前仓库代码 / `Dockerfile` / `deploy/compose.yaml` 核对过（2026-11-12）。

## 0. 结论先行

- **活着**看 `/healthz`（兼容别名 `/health`）：只证明进程与 HTTP 栈在，**不碰数据库**。
- **能接流量**看 `/readyz`（等价 `/api/readyz`）：池能 ping 通，且 `ai.*` 关键表在位；否则 503。
- **生成链失败**先看 job 终态：`GET /api/copilot/generate/{job_id}` 的 `status` 与 `build_error`。
- 全链路 fail-closed：不会返回空结果，也不会跳过失败步骤静默降级。

## 1. 起停

### 1.1 单仓 compose 冷启动（推荐）

前置：Docker 引擎可用；宿主 `8086` 与 `15432` 未被占用。

```bash
cd tellurion-agent
docker compose -f deploy/compose.yaml up --build -d --wait
docker compose -f deploy/compose.yaml ps
```

成功判据（三条都满足）：

```bash
curl -fsS http://127.0.0.1:8086/healthz
# {"service":"ai-service","status":"ok"}

curl -fsS http://127.0.0.1:8086/readyz
# {"status":"ready","service":"ai-service","checks":{"database":"ok","schema":"ok"},"timestamp":"..."}

docker compose -f deploy/compose.yaml exec -T postgres \
  pg_isready -U postgres -d tellurion
# ... accepting connections
```

- 端口：ai-service 只发布到 `127.0.0.1:8086`；PG 发布到 `127.0.0.1:15432`（容器内仍是 `postgres:5432`），避免和本机原生 PG 5432 撞号。
- 环境：compose 默认 `POSTGRES_USER/POSTGRES_PASSWORD=postgres`、`POSTGRES_DB=tellurion`；容器内 `CONFIG_PATH=/app/config/ai-service.yaml`。
- 日志：`docker compose -f deploy/compose.yaml logs -f ai-service`。

> **当前单仓栈的生成链边界（先知道，少走弯路）**：
> 1. compose 默认 `AI_API_KEY` / `AI_API_URL` 为空/占位，`POST /api/copilot/generate` 会先 503 `AI not configured`；
> 2. 即使配了 AI，`config/ai-service.yaml` 的 `copilot.working_dir` 出厂为空 ⇒ `sandbox_rejected`；
> 3. 运行镜像 `alpine:3.19` **没有 Go 工具链** ⇒ 填了目录也会在 `go build` 处 `build_failed`。
> 这三层都是设计内的 fail-closed，不是配置 bug；编译沙箱的能力补齐见
> `docs/TASKS.md` 的 `AGENT-SANDBOX-*` 条目。

### 1.2 宿主机直跑（改代码 / 调试单测）

先只起 PG，再在宿主机跑二进制；`go run` 默认读 `config/ai-service.yaml`。

```bash
cd tellurion-agent
docker compose -f deploy/compose.yaml up -d postgres

DATABASE_HOST=127.0.0.1 DATABASE_PORT=15432 \
DATABASE_USER=postgres DATABASE_PASSWORD=postgres \
DATABASE_DATABASE=tellurion DATABASE_SSLMODE=disable \
SERVER_HOST=127.0.0.1 SERVER_PORT=8086 \
AUTH_INSECURE=true AUTH_INSECURE_EXPOSURE=loopback-published \
go run ./cmd/ai
```

- 若 compose 的 ai-service 已占用 `8086`，先 `docker compose -f deploy/compose.yaml stop ai-service`，或把 `SERVER_PORT` 改成空闲端口。
- 直跑时 `CONFIG_PATH` 未设置，读仓库内 `config/ai-service.yaml`；要换配置就显式 `CONFIG_PATH=/abs/path/ai-service.yaml`。
- `DATABASE_PASSWORD` 不能为空：`kitdb.BuildDSN` 在启动期就拒绝空口令（fail-closed）。

### 1.3 停服

```bash
docker compose -f deploy/compose.yaml stop           # SIGTERM，保留容器与卷
docker compose -f deploy/compose.yaml down           # 删除容器，保留卷
docker compose -f deploy/compose.yaml down -v        # 连卷删除；下一轮是全新空库
```

宿主机直跑时 `Ctrl-C`（SIGINT）或 SIGTERM，日志应出现
`Shutdown signal received; beginning graceful drain`，随后进程退出。

## 2. `/healthz`（兼容 `/health`）与 `/readyz` 的语义差异

| 探针 | 判据 | 失败时 | 用途 |
|---|---|---|---|
| `GET /healthz`（兼容 `/health`） | 进程与 HTTP 栈存活，**不访问 DB** | 进程不在 → 连接失败 | 存活探针 / compose healthcheck |
| `GET /readyz`、`GET /api/readyz` | 2s 预算内池 ping 通 **且** `ai.experiments` / `ai.factor_genes` / `ai.strategy_genes` / `ai.research_profile` 四张表在位 | 503 + `checks.database` 给具体错误 | 就绪探针；决定是否接流量 |

- 两个路径都免鉴权、免限流（编排器不持有用户 token；429 会被读成「不 ready」）。
- `readyz` 为 `not_ready` 时进程仍活着（`/healthz` 200），**不要靠重启掩盖 DB 问题**。

## 3. 失败排查（现象 → 先看什么 → 常见原因 → 日志 / 终态）

### 3.1 起不来 / 就绪 503：数据库不可达

**现象**

- 启动即退出（compose 因 `restart: unless-stopped` 反复拉起），日志：
  `Failed to initialize ai store`，其下带 `aistore: connect: ...` 或 `aistore: migrate: ...`。
- 已在运行、DB 中途挂掉：`/healthz` 仍 200，`/readyz` 503，`checks.database` 含
  `aistore: ping failed: ...`。

**先看什么**

```bash
docker compose -f deploy/compose.yaml ps
docker compose -f deploy/compose.yaml logs --tail=100 ai-service | grep -E 'Failed to initialize|aistore:'
docker compose -f deploy/compose.yaml exec -T postgres pg_isready -U postgres -d tellurion
docker compose -f deploy/compose.yaml exec -T ai-service printenv DATABASE_HOST DATABASE_PORT DATABASE_USER DATABASE_DATABASE DATABASE_SSLMODE
```

**常见原因**

- PG 没起 / 还没 healthy；`depends_on: condition: service_healthy` 已挡住启动，但直跑时不会挡。
- `DATABASE_*` 指错（容器内必须是服务名 `postgres` + `5432`；宿主机直跑才是 `127.0.0.1` + `15432`）。
- `DATABASE_PASSWORD` 为空 ⇒ DSN 构造期就失败（日志点名空口令，不是认证失败）。
- 库名不一致：compose 默认 `tellurion`；换成别的 `POSTGRES_DB` 时 `DATABASE_DATABASE` 要同步。

**处置**：修 env 或先把 PG 起好，再 `docker compose ... restart ai-service`。PG 恢复后
`/readyz` 会自动回 200，不需要重启进程。

### 3.2 就绪 503：`ai.*` 迁移未执行 / 关键表缺失

**现象**：进程活着（`/healthz` 200），`/readyz` 503，`checks.database` 为：

```
aistore: required relation ai.experiments is missing (startup migration did not run or was rolled back)
```

（也可能是 `ai.factor_genes` / `ai.strategy_genes` / `ai.research_profile`。）

**先看什么**

```bash
docker compose -f deploy/compose.yaml logs ai-service | grep -i -E 'migrate|migration'
docker compose -f deploy/compose.yaml exec -T postgres \
  psql -U postgres -d tellurion -c "\dt ai.*"
```

**常见原因**

- 迁移被回滚 / 手工 `DROP` 了表；或 `down -v` 后连到了另一个空库。
- PG 从备份恢复时只恢复了业务库，没带 `ai` schema。
- 手工只补了部分表（`ai.generation_chain` 等不在就绪判据里，但生成链会用到）。

**处置**：重启 ai-service（`Migrate` 是 `CREATE ... IF NOT EXISTS`，会重建缺失表）；
若整个 schema 缺失或权限不足，先建库/建 schema 再重启。**不要手工只补单表**，schema 归 `Migrate` 所有。

### 3.3 `POST /api/copilot/generate` 直接 503：`AI not configured`

**现象**：HTTP 503，响应体含：

```
AI not configured (set AI_API_KEY and AI_API_URL)
```

不会创建 job（请求在 handler 第一道门就被拒）。

**先看什么**

```bash
docker compose -f deploy/compose.yaml exec -T ai-service \
  sh -c 'echo KEY=${AI_API_KEY:+set}; echo URL=$AI_API_URL; echo MODEL=${AI_MODEL:-<default>}'
```

**常见原因**

- `AI_API_KEY` 为空，或 `AI_API_URL` 为空。
- `AI_API_URL` 必须是**完整的 chat-completions 端点**，客户端直接 POST 该地址、不再拼路径；
  compose 默认 `https://api.openai.com/v1/chat/completions`。
- 把 key 写进了 `config/ai-service.yaml`（本仓约定密钥只从 env 注入，YAML 恒为空）。

**处置**：注入 `AI_API_KEY` / `AI_API_URL` 后 `docker compose ... up -d ai-service`（重建容器）；
再 POST 一次，期望 `202` + `job_id`（之后仍可能停在 3.4 / 3.5 的沙箱门，那是下一层）。

### 3.4 job 终态 `sandbox_rejected`：留痕落点未接（`chain sink not configured`）

**现象**：job `status=sandbox_rejected`，`build_error` 精确为：

```
chain sink not configured; call WithChainSink() at the composition root
```

**先看什么**：这是**装配根漏接**，不是配置键。标准 `cmd/ai` 恒接
`NewChainSink(aiStore)`；若出现，说明二进制/装配根回归，或嵌入式/测试装配没给 chain。
确定性复现（不需要真模型）：

```bash
go test ./pkg/copilot -run TestQT32A_NilChainSink_FailsClosedBeforeCallingLLM -v
```

**常见原因**：改了 `BuildCopilot(...)` 的调用，`WithChainSink` 被删；或测试里自己拼
`CopilotService` 时漏了 chain。

**处置**：在装配根恢复 `WithChainSink`，确认 `aiStore` 非 nil。**不要**为了「先出代码」
绕开留痕——一条记不下来的生成等于无法复查的生成。

### 3.5 job 终态 `sandbox_rejected` / `build_failed`：沙箱门

这一族有多个来源，**看 `build_error` 的第一句**区分，不要笼统归因：

| `build_error` 特征 | 含义 | 处置 |
|---|---|---|
| `staticcheck rejected generated code (N finding(s)):` + 规则/行号 | 生成代码含禁用模式（如 `os/exec`、网络、文件系统写） | 收紧 description 重新生成；**不要关闸**；反复命中就报 owner |
| `copilot.working_dir is not configured; ...` | 编译沙箱目录未配 | 配 `copilot.working_dir`（配置键）或 `COPILOT_WORKING_DIR`（env），指向含 `go.mod` 的目录 |
| `asset registrar not configured; call WithAssetRegistrar() ...` | 装配根漏接资产登记 | 恢复 `WithAssetRegistrar`；`strategy_asset.url` 为空也会 fail-closed |
| `资产登记失败：...` | strategy 资产服务不可达 / 拒绝（401 等） | 查 `STRATEGY_ASSET_URL` / `STRATEGY_ASSET_TOKEN` 与 strategy 服务 |
| `生成链写不进去（ai.generation_chain）：...` | 链落库失败 | 查 DB 与 `ai.generation_chain`；这是 3.2 的变体 |
| `...未过 promote 人工闸门，不进回测...` | **不是错误**：编译通过 + 已登记坐标，停在人工 promote 闸门 | 去 strategy 侧走 promote；本仓不代签 |
| `build_failed` 且 `build_error` 为空 | 运行镜像无 Go 工具链（`exec: "go": executable file not found`） | `docker compose ... exec ai-service sh -c 'command -v go'` 确认；当前单仓镜像无工具链，属已知边界 |

**先看什么**

```bash
# 取 job 终态（把 <id> 换成 generate 返回的 job_id）
curl -fsS http://127.0.0.1:8086/api/copilot/generate/<id> | jq '{status,build_error,strategy_id,version_hash}'
docker compose -f deploy/compose.yaml logs --tail=200 ai-service | grep -E 'sandbox|staticcheck|build|asset'
```

**常见原因**

- `copilot.working_dir` 出厂为空（见 `config/ai-service.yaml` 的 `copilot:` 段）；容器内路径还必须是**容器里存在且可写**的目录，并含能解析 `tellurion-contracts/domain` 的 `go.mod`。
- 运行镜像只有 `alpine:3.19` + `ca-certificates/curl/netcat`，**没有 Go**；填了目录也会在 `go build` 处失败（已知缺口，见 `AGENT-SANDBOX-*`）。
- 生成代码触发 staticcheck 规则；retry 的修复稿会**再过一次** staticcheck，过不了就终止。

**处置**：按上表逐项修；`working_dir` 改动后重启 ai-service。不要在镜像里临时装 Go
来「让它能编」——那会把工具链塞进持有 DB 口令与 AI key 的进程，违反既有沙箱裁决。

## 4. 一次生成请求的排查顺序

1. `POST /api/copilot/generate`，body 至少 `{"description":"..."}`。
   - 503 `AI not configured` → §3.3；400 `description is required` → 补参数；202 → 拿 `job_id`。
2. `GET /api/copilot/generate/{job_id}` 轮询，`status` 取值：
   `pending`（刚建）→ `llm_failed` / `build_failed` / `sandbox_rejected`（终态）。
3. 按 `status` 分流：
   - `llm_failed`：LLM 调用失败，`build_error` 是上游错误；查 AI 端点连通与配额。
   - `build_failed`：§3.5 的编译/工具链分支。
   - `sandbox_rejected`：§3.4 / §3.5 的链、目录、静态检查、登记、promote 闸门分支。
4. 需要看落库证据时查 `ai.generation_chain`（每次 attempt 一行：源码指纹、编译状态、错误、阻断原因）。

## 5. 常用命令速查

```bash
# 起停
docker compose -f deploy/compose.yaml up --build -d --wait
docker compose -f deploy/compose.yaml ps
docker compose -f deploy/compose.yaml logs -f ai-service
docker compose -f deploy/compose.yaml restart ai-service
docker compose -f deploy/compose.yaml down -v

# 探针
curl -fsS http://127.0.0.1:8086/healthz
curl -fsS http://127.0.0.1:8086/readyz

# DB
docker compose -f deploy/compose.yaml exec -T postgres pg_isready -U postgres -d tellurion
docker compose -f deploy/compose.yaml exec -T postgres psql -U postgres -d tellurion -c "\dt ai.*"

# 生成链
curl -fsS -X POST http://127.0.0.1:8086/api/copilot/generate \
  -H 'Content-Type: application/json' -d '{"description":"..."}'
curl -fsS http://127.0.0.1:8086/api/copilot/generate/<job_id>

# 代码门禁（改代码后）
go build ./... && go vet ./... && go test ./... -count=1
```
