# ListingKit unknown 状态和 blocker 待关闭清单

本文件集中记录真实接口验收中出现的未知任务状态、未知 readiness blocker、空错误响应和 UI 无下一步动作问题。

每一条真实 run 报告完成后都必须检查本文件：如果出现新 unknown，新增记录；如果已补映射或明确不支持，更新状态和关闭证据。

## 使用规则

- `open`：已发现但还没有稳定映射、兜底展示或 SOP。
- `mapped`：后端 / 前端 / SOP 已经有明确映射，但还需要下一轮真实任务验证。
- `closed`：已经用真实 run 证明用户可理解、可恢复或可升级。
- `won't fix`：明确不支持，且 UI / SOP 已说明原因。

## 任务状态 unknown

| 首次发现日期 | run | 状态值 | 来源接口 | 页面表现 | 影响 | owner | 状态 | 关闭标准 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
|  |  |  |  |  |  |  |  |  |

## Readiness blocker unknown

| 首次发现日期 | run | blocker key / label | severity | domain | 来源接口 | 页面表现 | 是否阻断提交 | owner | 状态 | 关闭标准 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 2026-06-21 | `2026-06-21-shein-sds-batch-production-closure` | `shein_cookie_unavailable` | blocker | integration | `GET /api/v1/listing-kits/tasks/{task_id}/preview?platform=shein` | SHEIN store cookie unavailable；`sellerhub.shein.com` 访问返回 `net::ERR_PROXY_CONNECTION_FAILED`。 | yes | ops / platform | open | 恢复 sellerhub proxy/cookie 刷新后，同一 task 或后续真实 task readiness 不再出现该 blocker。 |
| 2026-06-21 | `2026-06-21-shein-sds-batch-production-closure` | `category_unresolved` | blocker | catalog | `GET /api/v1/listing-kits/tasks/{task_id}/preview?platform=shein` | SHEIN 类目未解析，提交被阻断。 | yes | listing / ops | open | workspace 补齐类目后，真实 `save_draft` 可进入提交阶段。 |
| 2026-06-21 | `2026-06-21-shein-sds-batch-production-closure` | `attributes_unmapped` / `required_attributes_pending` | blocker | attribute | `GET /api/v1/listing-kits/tasks/{task_id}/preview?platform=shein` | SHEIN 普通属性和必填属性未完成。 | yes | listing / ops | open | workspace 补齐属性映射后，真实 `save_draft` 不再因属性 blocker 停止。 |
| 2026-06-21 | `2026-06-21-shein-sds-batch-production-closure` | `sale_attributes_unresolved` | blocker | variation | `GET /api/v1/listing-kits/tasks/{task_id}/preview?platform=shein` | SHEIN 销售属性未完成。 | yes | listing / ops | open | workspace 补齐销售属性后，真实 `save_draft` 不再因销售属性 blocker 停止。 |

## 空错误响应

| 首次发现日期 | run | 失败阶段 | 接口 | 页面表现 | 是否可恢复 | owner | 状态 | 关闭标准 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
|  |  |  |  |  |  |  |  |  |

## UI 无下一步动作

| 首次发现日期 | run | 页面 / 区域 | 场景 | 用户看到什么 | 期望 next action | owner | 状态 | 关闭标准 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 2026-06-21 | `2026-06-21-shein-sds-batch-production-closure` | Studio batch task creation response | 重复 task creation 请求复用 durable links | task ID 未重复，但响应仍归入 `created_tasks` 而不是 `reused_tasks`。 | 将复用结果投影到 `reused_tasks`，或在 UI 明确显示“已存在/已复用”。 | engineering | open | 重复请求响应和前端展示能区分新建与复用，QA 不再需要比对 task ID 判断幂等。 |

## 已定位工程问题

| 首次发现日期 | run | 问题 | 根因 | 修复状态 | 关闭证据 |
| --- | --- | --- | --- | --- | --- |
| 2026-06-21 | `2026-06-21-shein-sds-batch-production-closure` | `sds_baseline_warmup_unavailable` | HTTP bootstrap 未把 `WarmSDSBaseline` service 注入 ListingKit handler。 | closed | 增加 `SDSBaselineWarmService` handler option 和 bootstrap 测试；真实 warmup 后两个 SDS selection readiness 均为 `ready`。 |
| 2026-06-21 | `2026-06-21-shein-sds-batch-production-closure` | batch generation AI client missing | tenant 286 未配置 generation 所需 AI client。 | closed | 使用本地环境配置补齐 tenant AI settings 后，batch 从 generation failure 恢复到 `review_ready`。 |

## 当前汇总

| 类别 | open | mapped | closed | won't fix |
| --- | --- | --- | --- | --- |
| 任务状态 unknown | 0 | 0 | 0 | 0 |
| Readiness blocker unknown | 4 | 0 | 0 | 0 |
| 空错误响应 | 0 | 0 | 0 | 0 |
| UI 无下一步动作 | 1 | 0 | 0 | 0 |
| 已定位工程问题 | 0 | 0 | 2 | 0 |
