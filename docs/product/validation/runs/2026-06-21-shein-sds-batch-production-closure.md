# ListingKit 真实接口验收报告：SHEIN SDS Batch Production Closure

## 1. 基本信息

| 字段 | 内容 |
| --- | --- |
| Run ID | 2026-06-21-shein-sds-batch-production-closure |
| 日期 | 2026-06-21 |
| 记录人 | Codex |
| 环境 | local replay against `product-listing-api-local` |
| 租户 | `286` |
| 店铺 | `874` |
| 目标平台 | SHEIN |
| 来源类型 | SDS |
| task_id | `fc471099-45a4-4f91-9d61-26ee1984b276` |
| batch_id | `ec72e850-d360-4c22-be7b-dfefa2f6be36` |
| 结论 | partial |

## 2. 输入信息

### 来源素材

```text
SDS parent product: 295974
Product name: 彩绘玻璃挂饰-圆形（包邮仅限美国直发）
Prototype group: 32940
Layer: 912897720021245952
Printable: 2000 x 2000
Selections:
- 295974:32940:295975:912897720021245952:295975, SKU MG80061028001, 16x16cm, white
- 295974:32940:295977:912897720021245952:295977, SKU MG80061028003, 25x25cm, white
Compatibility key: compat:6e462bf5f092b01de4afa3b5835ad5ac4c33bb25
Prompt: Minimal clean floral stained glass ornament artwork, centered, high contrast, production ready
```

### 创建任务请求摘要

```json
{
  "source": "SDS",
  "target_platform": "SHEIN",
  "store_id": "874",
  "payload_summary": "2 generated designs x 2 compatible SDS selections, task creation fan-out through Go API with tenant 286."
}
```

### 创建任务响应摘要

```json
{
  "created_tasks": 4,
  "rejected_tasks": 0,
  "failed_tasks": 0,
  "batch_status": "tasks_created"
}
```

## 3. 状态流转

| 时间 | 状态 | 页面 / 接口 | 说明 | 是否符合预期 |
| --- | --- | --- | --- | --- |
| 2026-06-21 | ready | `POST /api/v1/listing-kits/sds/baseline/warm` | 两个 SDS selection baseline 均预热到 `ready`。 | yes |
| 2026-06-21 | review_ready | `POST /api/v1/listing-kits/studio/batches` + generation retry | AI 客户端设置补齐后，batch 生成 2 个 design。 | yes |
| 2026-06-21 | tasks_created | `POST /api/v1/listing-kits/studio/batches/{batch_id}/tasks` | 2 designs x 2 selections fan-out 生成 4 个 ListingKit tasks。 | yes |
| 2026-06-21 | tasks_created | repeat task creation request | 重复请求后 task 数量仍为 4，ID 不变，没有重复任务。 | yes |
| 2026-06-21 | partial | controlled rejection batch | 一个有效店铺候选创建成功，一个无效店铺候选被结构化拒绝。 | yes |
| 2026-06-21 | blocked | `GET /api/v1/listing-kits/tasks/{task_id}/preview?platform=shein` | SHEIN submit readiness 被 cookie/proxy、类目和属性映射阻塞。 | yes |
| 2026-06-21 | failed | `POST /api/v1/listing-kits/tasks/{task_id}/submit` | `save_draft` 返回 400 `submit_failed`，原因是 readiness 未通过。 | yes |

### 未知状态

| 状态值 | 来源接口 | 页面表现 | 处理结论 |
| --- | --- | --- | --- |
| 无 | 无 | 无 | 本轮未出现未知 task/batch 状态。 |

## 4. Workspace payload 验收

| 区域 | 是否有数据 | 问题 | 备注 |
| --- | --- | --- | --- |
| 商品事实 | yes | 无 | 可从 SDS selection 和 task preview 读取商品、规格、图片事实。 |
| 类目 | partial | `category_unresolved` | SHEIN 类目未解析完成，阻断提交。 |
| 普通属性 | partial | `attributes_unmapped`, `required_attributes_pending` | SHEIN 必填属性未完成。 |
| 销售属性 | partial | `sale_attributes_unresolved` | 销售属性映射未完成。 |
| 图片 | yes | 无 | 生成 design 已落入 batch，并参与 task creation。 |
| 价格 | partial | 依赖提交资料包继续补齐 | 本轮重点是 batch fan-out 和 readiness。 |
| SKU / 变体 | yes | 无 | 两个 SDS variant 均进入任务。 |
| 提交报告 | yes | `submit_failed` | 提交阶段被 readiness 阻断，未到远端 SHEIN draft。 |
| 历史版本 | not verified | 未进入人工修复保存流 | 本轮未覆盖 workspace 修复版本。 |

## 5. Readiness 验收

| blocker key | severity | domain | 页面展示 | 是否可跳转修复 | 备注 |
| --- | --- | --- | --- | --- | --- |
| `shein_cookie_unavailable` | blocker | integration | SHEIN store cookie unavailable | no | `sellerhub.shein.com` 访问失败，Playwright 返回 `net::ERR_PROXY_CONNECTION_FAILED`。 |
| `category_unresolved` | blocker | catalog | SHEIN category unresolved | yes | 需要补齐 SHEIN 类目解析。 |
| `attributes_unmapped` | blocker | attribute | SHEIN attributes unmapped | yes | 普通属性映射未完成。 |
| `required_attributes_pending` | blocker | attribute | Required attributes pending | yes | SHEIN 必填属性未完成。 |
| `sale_attributes_unresolved` | blocker | variation | Sale attributes unresolved | yes | 销售属性映射未完成。 |

### 未知 blocker

| blocker key / label | 来源接口 | 页面表现 | 是否阻断提交 | 后续处理 |
| --- | --- | --- | --- | --- |
| 无 | 无 | 无 | no | 本轮 blocker 均有 key 和可解释消息。 |

## 6. 人工修复记录

| 时间 | 修复区域 | 修改内容 | 保存结果 | 备注 |
| --- | --- | --- | --- | --- |
| 2026-06-21 | Go API handler wiring | 修复 SDS baseline warmup service 未注入 handler 的问题。 | success | 修复前 warmup 返回 `sds_baseline_warmup_unavailable`，修复后两个 selection readiness 均为 `ready`。 |
| 2026-06-21 | Tenant AI settings | 使用现有本地环境配置补齐 tenant 286 的 generation 客户端设置。 | success | 不记录任何密钥值；补齐后 batch generation 从失败恢复为 `review_ready`。 |
| 2026-06-21 | Store profile | 使用 tenant 286 的 SHEIN store `874` 和 US site profile。 | success | store resolution 命中 profile `5`。 |

## 7. 提交验收

### 保存草稿

| 字段 | 内容 |
| --- | --- |
| action | save_draft |
| idempotency_key | `codex-validation-20260621-draft-fc471099` |
| attempt_id | 未生成 |
| 最终状态 | failed |
| 失败 phase | validate / readiness |
| 远端 draft id | 无 |
| 是否重复提交 | no |

### 发布

| 字段 | 内容 |
| --- | --- |
| action | publish |
| idempotency_key | 未生成 |
| attempt_id | 未生成 |
| 最终状态 | skipped |
| 失败 phase | readiness |
| 远端 product / publish id | 无 |
| 是否重复提交 | no |

### 提交阶段记录

| 时间 | phase | 状态 | 错误码 | 错误信息 | 是否可恢复 |
| --- | --- | --- | --- | --- | --- |
| 2026-06-21 | validate | failed | `submit_failed` | submit blocked by readiness: 当前仍有关键字段未完成，SHEIN 资料包还不能直接进入提交态 | yes |
| 2026-06-21 | prepare_product | skipped |  | readiness 未通过。 |  |
| 2026-06-21 | upload_images | skipped |  | readiness 未通过。 |  |
| 2026-06-21 | pre_validate | skipped |  | readiness 未通过。 |  |
| 2026-06-21 | submit_remote | skipped |  | 未触发 SHEIN 远端保存草稿。 |  |
| 2026-06-21 | persist_result | skipped |  | 未产生远端结果。 |  |

## 8. 失败恢复

| 失败类型 | 用户看到什么 | 运营动作 | 是否恢复 | 是否需要工程介入 | 记录字段 |
| --- | --- | --- | --- | --- | --- |
| SDS baseline warmup handler 未注入 | warmup unavailable | 已修复 Go API 注入并增加覆盖测试。 | yes | no | handler option / bootstrap test |
| AI client settings missing | generation failed | 已补齐 tenant 286 的本地 AI generation 设置。 | yes | no | tenant AI settings |
| 重复创建 task | 重复点击后仍看到原 task 集合 | 无需恢复；ID 不变。 | yes | no | batch `ec72e850-d360-4c22-be7b-dfefa2f6be36` |
| 无效店铺候选 | 单个候选 `store_invalid` rejected | 修正店铺 ID 后重试该候选。 | yes | no | batch `1c7721ec-d30e-4dc3-898e-1a1a79e94936` |
| SHEIN cookie/proxy unavailable | submit readiness 阻断 | 恢复 sellerhub proxy/cookie 刷新能力。 | no | yes | `shein_cookie_unavailable` |
| SHEIN 类目/属性未映射 | submit readiness 阻断 | 在 workspace 补齐类目、普通属性、销售属性。 | no | no/yes | readiness blockers |

## 9. 证据附件

- 主 batch：`ec72e850-d360-4c22-be7b-dfefa2f6be36`
- 生成 designs：
  - `22fde620-0ef2-473b-94f5-b74133e75a22`
  - `15ff5439-c90c-4d67-a318-f79c04c7f7fc`
- 创建 tasks：
  - `fc471099-45a4-4f91-9d61-26ee1984b276` -> design `15ff5439-c90c-4d67-a318-f79c04c7f7fc`, selection `295974:32940:295975:912897720021245952:295975`
  - `56fd090e-32f5-44eb-9ff8-170c9897b4cb` -> design `15ff5439-c90c-4d67-a318-f79c04c7f7fc`, selection `295974:32940:295977:912897720021245952:295977`
  - `60064cb0-2e59-4970-9b3a-b70375af4a41` -> design `22fde620-0ef2-473b-94f5-b74133e75a22`, selection `295974:32940:295975:912897720021245952:295975`
  - `cc35b141-d7f2-41af-b4d9-ca985bef1571` -> design `22fde620-0ef2-473b-94f5-b74133e75a22`, selection `295974:32940:295977:912897720021245952:295977`
- 重复创建证据：重复调用后 task count 仍为 4，task ID 集合不变；响应仍归入 `created_tasks`，不是 `reused_tasks`。
- 受控拒绝 batch：`1c7721ec-d30e-4dc3-898e-1a1a79e94936`
  - created: `7b1b1370-afac-43b0-8d44-66f474632d69`
  - rejected: selection `295974:32940:295977:912897720021245952:295977`, reason_code `store_invalid`, message `SHEIN store 999999999 is invalid`
- `save_draft` 证据：`POST /api/v1/listing-kits/tasks/fc471099-45a4-4f91-9d61-26ee1984b276/submit` 返回 HTTP 400，error `submit_failed`。
- 远端平台记录：未产生 SHEIN draft id；提交被 readiness 阶段阻断。

## 10. 结论

```text
本轮是否通过：partial
主要问题：真实 SDS batch fan-out、重复请求幂等、受控拒绝均已通过；真实 SHEIN save_draft 未通过。
必须关闭的问题：恢复 SHEIN sellerhub cookie/proxy；补齐 SHEIN 类目、普通属性、必填属性和销售属性映射，然后重新执行 save_draft。
可后续优化的问题：重复创建请求复用已有 task 时，响应仍进入 created_tasks；建议按 reused_tasks 投影，减少前端和 QA 误判。
是否允许进入下一轮：允许继续做 Phase 2 的边界收缩准备，但 Phase 1 退出条件中的“真实 SDS-to-SHEIN 草稿通过”仍未关闭。
```

## 11. Follow-up

| 优先级 | 问题 | owner | 截止时间 | 状态 |
| --- | --- | --- | --- | --- |
| P0 | 恢复本地/联调环境访问 `sellerhub.shein.com` 的 proxy 和 store cookie 刷新。 | ops / platform | 下一轮真实 SHEIN 提交前 | open |
| P0 | 补齐 task workspace 的 SHEIN 类目、普通属性、必填属性、销售属性映射。 | listing / ops | 下一轮真实 SHEIN 提交前 | open |
| P0 | readiness 清零后重新执行 `save_draft`，记录 attempt id 和远端 draft id。 | QA / engineering | readiness 修复后 | open |
| P1 | 调整重复 task creation 响应投影，将 durable link 复用结果归入 `reused_tasks`。 | engineering | Phase 1 hardening | open |
| P1 | 在真实环境复测 partial submission：一个候选提交失败后，后续候选仍被尝试并记录独立结果。 | QA / engineering | SHEIN cookie/proxy 恢复后 | open |
