#!/bin/sh
# 等外部依赖可达后再启动服务（「数据库跑宿主机、服务跑容器」形态）。
#
# 为什么需要它：应用连不上 PG 时是 logger.Fatal() 直接退出，**没有重试**。
# 这条启动顺序原本由 compose 的 `depends_on: {postgres: {condition: service_healthy}}`
# 保证 —— 当 postgres 不再是 compose 成员（改连宿主机）时，那个保证就消失了。
#
# **本仓只等 PostgreSQL**：agent 的 Go 代码零 Redis 读取点（`go-redis` 仅经
# `tellurion-kit/db` 间接引入），`deploy/compose.yaml` 也不提供 redis 服务、
# 不注入 REDIS_URL。若将来真的接了 Redis，再在这里加一条推导规则。
#
# 用法（compose 的 entrypoint + command）：
#   entrypoint: ["/bin/sh", "/app/deploy/wait-for-deps.sh"]
#   command: ["/app/ai-service"]
#
# ── 等什么：从**应用自己的配置**推导，不另维护一份依赖清单 ──────────────
#
#   DATABASE_HOST + DATABASE_PORT  →  等 <host>:<port>
#
# 这样「等的」与「连的」在结构上不可能不一致。
#
# 覆盖手段：设 WAIT_FOR_DEPS（空格分隔的 host:port 列表）可以完全接管；
# 超时由 WAIT_FOR_DEPS_TIMEOUT 控制（秒，默认 60）。超时即 exit 1，并把
# 「是哪个依赖、等了多久、怎么查」明确打出来 —— 不静默失败。
set -eu

TIMEOUT="${WAIT_FOR_DEPS_TIMEOUT:-60}"

if [ "$#" -eq 0 ]; then
    echo "wait-for-deps: 用法: wait-for-deps.sh <cmd> [args...] —— 必须给出要执行的命令" >&2
    exit 2
fi

targets=""
if [ -n "${DATABASE_HOST:-}" ]; then
    targets="$targets ${DATABASE_HOST}:${DATABASE_PORT:-5432}"
fi
# 显式覆盖优先（也给「等一个不在配置里的东西」留出口）
targets="${WAIT_FOR_DEPS:-$targets}"

if [ -z "$targets" ]; then
    echo "wait-for-deps: 没有可推导的依赖（DATABASE_HOST 为空），直接启动" >&2
    exec "$@"
fi

for target in $targets; do
    host="${target%%:*}"
    port="${target##*:}"
    waited=0
    while ! nc -z "$host" "$port" 2>/dev/null; do
        if [ "$waited" -ge "$TIMEOUT" ]; then
            echo "wait-for-deps: 等 ${host}:${port} 超过 ${TIMEOUT}s 仍不可达，放弃启动。" >&2
            echo "wait-for-deps: 宿主机上的数据库/缓存起了吗？本机用 tools/local-infra.sh start 启停。" >&2
            exit 1
        fi
        sleep 1
        waited=$((waited + 1))
    done
    echo "wait-for-deps: ${host}:${port} 就绪（等待 ${waited}s）"
done

exec "$@"
