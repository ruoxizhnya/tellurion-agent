---
status: active
last-verified: 2026-10-20
verified-by: 功能边界固化（三份契约：db / api / dataflow）
---

# Dataflow — tellurion-agent

> 目的：把「数据从哪来、在谁手里变形、到哪去」写死。**任何跨仓数据交换都必须能在本图上找到一条边**；找不到 = 这段代码在偷偷建第二条路。

## 1. 主链路

```
ui ──(HTTP/SSE)──► tellurion-agent (ai-service :8086)
                        │  ① explore loop：生成假设 → 组策略 → 跑回测
                        │  ② copilot：自然语言 → 代码 → 沙箱编译 → 回测
                        │  ③ pipeline / tools：编排与工具执行
                        ↓
              core（进程内库：引擎 / 表达式 / 行情）—— 过渡期编译依赖
                        ↓
              ai.*（本仓自持）/ run（结果）/ market（数据）
```

## 2. 关键时序

### 探索一轮
1. `POST /api/explore/runs` → 建 run（内存态）+ 返回 `runID`
2. 后台 goroutine 跑 loop：每轮 attempt → 组策略 → 调 core 引擎回测
3. 每次 attempt 落 `ai.experiments`（+ 三 hash）
4. ui 订阅 `GET .../stream`：每 tick 推 `progress`（快照与轮询端点同形状）
5. 结束推 `complete`；`POST .../stop` 只停调度，不 cancel 正在跑的回测

## 3. 失败模式（每一处 fail-closed 都要能指名）

| 失败点 | 症状 | 兜底 |
|---|---|---|
| 引擎 panic / trap | 整轮探索挂掉 | attempt 级隔离 + 记录失败原因 |
| SSE 断线 | ui 看不到进度 | 只停推、不取消探索；客户端重连拿快照 |
| 双写不一致 | 切读后数据缺 | 行数 / 校验和对拍，不一致不切 |
| 工具执行超时 | 卡住 pipeline | 每工具独立超时 + 归类为失败 |

## 4. 不变量（可被测试钉住的）

1. `ai.*` 与 core 旧表在双写期行数一致（对拍报告归档）。
2. 同一实验配置 → 同一结果（引擎确定性 + 三 hash 留痕）。
3. 断线不影响 run 的生命周期。
4. 本仓不出现 `pkg/storage` 的 import（QT-17 后）。

## 5. 与契约文档的关系

- 表结构 → `docs/db-schema.md`
- 接口形状 → `docs/api-schema.md`
- 本文件只讲**流向与顺序**，不重复字段定义。
