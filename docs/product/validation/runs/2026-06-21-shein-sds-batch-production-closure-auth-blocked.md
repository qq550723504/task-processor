# ListingKit 真实接口验收报告：SHEIN SDS Batch Production Closure 认证阻塞

## 1. 基本信息

| 字段 | 内容 |
| --- | --- |
| Run ID | 2026-06-21-shein-sds-batch-production-closure-auth-blocked |
| 日期 | 2026-06-21 |
| 记录人 | Codex |
| 环境 | local replay against `product-listing-api-local` |
| 租户 | 未取得 ZITADEL session / bearer token |
| 店铺 | 未进入设置健康检查，无法确认 |
| 目标平台 | SHEIN |
| 来源类型 | SDS |
| task_id | 未创建 |
| batch_id | 6e10be71-d37c-4d99-bc12-a444730378e4 |
| 结论 | blocked |

## 2. 输入信息

### 来源素材

```text
计划验证现有 SDS batch:
6e10be71-d37c-4d99-bc12-a444730378e4
```

### 创建任务请求摘要

```json
{
  "source": "SDS",
  "target_platform": "SHEIN",
  "store_id": "",
  "payload_summary": "未能进入真实 API 读取和操作阶段；当前环境缺 ZITADEL session / bearer token。"
}
```

### 创建任务响应摘要

```json
{
  "task_id": "",
  "status": "not_started",
  "message": "blocked before real task creation by ZITADEL authentication"
}
```

## 3. 状态流转

| 时间 | 状态 | 页面 / 接口 | 说明 | 是否符合预期 |
| --- | --- | --- | --- | --- |
| 2026-06-21 09:42 +08:00 | service_ready | `http://localhost:3000/healthz` | UI health returned 200. | yes |
| 2026-06-21 09:42 +08:00 | service_ready | `http://localhost:8085/health` | Local API returned `{"status":"ok"}`. | yes |
| 2026-06-21 09:42 +08:00 | blocked | `http://localhost:3000/api/zitadel-auth/session` | Returned 401: `Missing ZITADEL session`. | yes |
| 2026-06-21 09:42 +08:00 | blocked | `http://localhost:8085/api/v1/listing-kits/settings-health` | Returned 401: `Missing ZITADEL bearer token`. | yes |
| 2026-06-21 09:42 +08:00 | blocked | `http://localhost:8085/api/v1/listing-kits/studio/batches/6e10be71-d37c-4d99-bc12-a444730378e4` | Returned 401: `Missing ZITADEL bearer token`. | yes |

### 未知状态

| 状态值 | 来源接口 | 页面表现 | 处理结论 |
| --- | --- | --- | --- |
| 无 | 无 | 无 | 当前未进入 batch / task 状态读取阶段。 |

## 4. Workspace payload 验收

| 区域 | 是否有数据 | 问题 | 备注 |
| --- | --- | --- | --- |
| 商品事实 | no | 认证阻塞 | 未能读取真实 batch/task。 |
| 类目 | no | 认证阻塞 | 未进入 workspace。 |
| 普通属性 | no | 认证阻塞 | 未进入 workspace。 |
| 销售属性 | no | 认证阻塞 | 未进入 workspace。 |
| 图片 | no | 认证阻塞 | 未进入 workspace。 |
| 价格 | no | 认证阻塞 | 未进入 workspace。 |
| SKU / 变体 | no | 认证阻塞 | 未进入 workspace。 |
| 提交报告 | no | 认证阻塞 | 未触发保存草稿 / 发布。 |
| 历史版本 | no | 认证阻塞 | 未进入 workspace。 |

## 5. Readiness 验收

| blocker key | severity | domain | 页面展示 | 是否可跳转修复 | 备注 |
| --- | --- | --- | --- | --- | --- |
| 无 |  |  |  |  | 当前未进入 readiness 阶段。 |

### 未知 blocker

| blocker key / label | 来源接口 | 页面表现 | 是否阻断提交 | 后续处理 |
| --- | --- | --- | --- | --- |
| 无 | 无 | 无 | no | 当前阻塞是认证前置条件，不是未知 readiness blocker。 |

## 6. 人工修复记录

| 时间 | 修复区域 | 修改内容 | 保存结果 | 备注 |
| --- | --- | --- | --- | --- |
| 2026-06-21 09:42 +08:00 | 认证 / 环境 | 检查 UI、API、ZITADEL session 和 bearer token。 | blocked | 需要登录后的 ZITADEL session 或有效 bearer token。 |

## 7. 提交验收

### 保存草稿

| 字段 | 内容 |
| --- | --- |
| action | save_draft |
| idempotency_key | 未生成 |
| attempt_id | 未生成 |
| 最终状态 | skipped |
| 失败 phase | preflight_auth |
| 远端 draft id | 无 |
| 是否重复提交 | no |

### 发布

| 字段 | 内容 |
| --- | --- |
| action | publish |
| idempotency_key | 未生成 |
| attempt_id | 未生成 |
| 最终状态 | skipped |
| 失败 phase | preflight_auth |
| 远端 product / publish id | 无 |
| 是否重复提交 | no |

### 提交阶段记录

| 时间 | phase | 状态 | 错误码 | 错误信息 | 是否可恢复 |
| --- | --- | --- | --- | --- | --- |
| 2026-06-21 09:42 +08:00 | validate | blocked | zitadel_token_invalid | Missing ZITADEL session | yes |
| 2026-06-21 09:42 +08:00 | validate | blocked | zitadel_token_missing | Missing ZITADEL bearer token | yes |
| 2026-06-21 09:42 +08:00 | prepare_product | skipped |  | 未执行。 |  |
| 2026-06-21 09:42 +08:00 | upload_images | skipped |  | 未执行。 |  |
| 2026-06-21 09:42 +08:00 | pre_validate | skipped |  | 未执行。 |  |
| 2026-06-21 09:42 +08:00 | submit_remote | skipped |  | 未执行。 |  |
| 2026-06-21 09:42 +08:00 | persist_result | skipped |  | 未执行。 |  |

## 8. 失败恢复

| 失败类型 | 用户看到什么 | 运营动作 | 是否恢复 | 是否需要工程介入 | 记录字段 |
| --- | --- | --- | --- | --- | --- |
| ZITADEL session 缺失 | UI / API 返回 401 | 在 `http://localhost:3000` 完成 ZITADEL 登录，或为 API 请求提供有效 bearer token。 | no | no | `zitadel_token_invalid`, `zitadel_token_missing` |

## 9. 证据附件

- 页面截图：未获取；浏览器插件连接失败，API 证据来自 HTTP 请求。
- 接口响应摘要：
  - `GET http://localhost:3000/healthz` -> 200 `{"ok":true}`
  - `GET http://localhost:8085/health` -> 200 `{"status":"ok"}`
  - `GET http://localhost:3000/api/zitadel-auth/session` -> 401 `{"error":"zitadel_token_invalid","message":"Missing ZITADEL session"}`
  - `GET http://localhost:8085/api/v1/listing-kits/settings-health` -> 401 `{"error":"zitadel_token_missing","message":"Missing ZITADEL bearer token"}`
  - `GET http://localhost:8085/api/v1/listing-kits/studio/batches/6e10be71-d37c-4d99-bc12-a444730378e4` -> 401 `{"error":"zitadel_token_missing","message":"Missing ZITADEL bearer token"}`
- 日志关键字段：未进入业务执行阶段。
- 远端平台记录：无。

## 10. 结论

```text
本轮是否通过：blocked
主要问题：本地 UI/API 均可用，但当前运行环境缺 ZITADEL session / bearer token，无法读取真实 batch，更不能安全执行 ListingKit/SHEIN 操作。
必须关闭的问题：在同一套 local replay 环境完成 ZITADEL 登录，或提供有效 API bearer token；然后重新执行 Task 12 的 batch detail、duplicate create、structured rejection、save draft 和 partial submission 验证。
可后续优化的问题：为本地只读验证提供明确的 auth bootstrap SOP，避免把认证阻塞误判成 batch 业务失败。
是否允许进入下一轮：允许继续做代码层验证；真实 SDS -> ListingKit -> SHEIN draft 验证必须等待认证前置条件满足。
```

## 11. Follow-up

| 优先级 | 问题 | owner | 截止时间 | 状态 |
| --- | --- | --- | --- | --- |
| P0 | 在 `localhost:3000` 完成 ZITADEL 登录，或提供可用于 `localhost:8085` 的 bearer token。 | ops / engineering | 下一轮真实联调前 | open |
| P0 | 重新读取 batch `6e10be71-d37c-4d99-bc12-a444730378e4` 并记录 batch/items/designs/task links。 | QA / engineering | 认证恢复后 | open |
| P0 | 创建或复用真实 ListingKit task，并保存至少 1 个 SHEIN draft。 | QA / engineering | 认证恢复后 | open |
| P1 | 如果再次出现 unknown 状态、unknown blocker、空错误或 UI 无下一步动作，更新 `../unknown-state-and-blocker-tracker.md`。 | QA / engineering | 每轮 run 后 | open |
