---
status: active
last-verified: 2026-10-24
verified-by: QT-17 ④ —— 停写 legacy public.experiments，写路径只剩 ai.experiments
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
3. 每次 attempt 落 `ai.experiments`（**QT-17 ④ 起单写**）：`main.go` 把 `*aistore.Store` 直接注入 pipeline / explore 的 sink，双写适配器已删除；legacy `public.experiments` **停写并冻结**在 30 行（历史行已回填归档，行级对拍 0 差异）。`parent_id` 因此直接在 `ai` 的 id 空间内自洽，不再需要 legacyID→aiID 映射
4. ui 订阅 `GET .../stream`：每 tick 推 `progress`（快照与轮询端点同形状）
5. 结束推 `complete`；`POST .../stop` 只停调度，不 cancel 正在跑的回测

### 研究域 / 因子假设读取
- `research.profile` / `research.conclusion` / `research.question` 与 `factor_hypothesis` 的**读面已切 `ai.*`**（自有 `aistore`）；`factor_hypothesis` 的 builtin 种子启动时 `ON CONFLICT DO NOTHING` 镜像播种。研究域三表**目前没有写入方**（core 侧也无研究域 handler，真库 0 行）——将来接写时直写 `ai.*`，不得落 legacy `research.*`。

## 3. 失败模式（每一处 fail-closed 都要能指名）

| 失败点 | 症状 | 兜底 |
|---|---|---|
| 引擎 panic / trap | 整轮探索挂掉 | attempt 级隔离 + 记录失败原因 |
| SSE 断线 | ui 看不到进度 | 只停推、不取消探索；客户端重连拿快照 |
| 双写不一致 | 切读后数据缺 | 双写期已结束；停写前已做一次全量回填 + 行级对拍（0 差异），此后旧表零增长 |
| 工具执行超时 | 卡住 pipeline | 每工具独立超时 + 归类为失败 |

## 4. 不变量（可被测试钉住的）

1. `ai.*` 与 core 旧表在停写切换时行级一致（回填归档 + `EXCEPT` 双向 0 差异，见 ODR-001）；停写后 `public.experiments` 行数与序列不再前进。
2. 同一实验配置 → 同一结果（引擎确定性 + 三 hash 留痕）。
3. 断线不影响 run 的生命周期。
4. 本仓生产代码不出现 `pkg/storage` 的 import——停写后白名单只剩一条（`handlers_explore.go` 的仪器域 `ListingWindow`，等 D24 第 7 类动作），由 `internal/repoguard` AST 护栏钉住；S3（QT-50 后）收归零。

## 5. 与契约文档的关系

- 表结构 → `docs/db-schema.md`
- 接口形状 → `docs/api-schema.md`
- 本文件只讲**流向与顺序**，不重复字段定义。
