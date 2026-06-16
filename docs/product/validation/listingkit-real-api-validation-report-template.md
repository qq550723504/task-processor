# ListingKit 真实接口验收报告模板

> 复制本模板到 `docs/product/validation/runs/YYYY-MM-DD-<scenario>.md`，每次真实联调一份报告。

## 1. 基本信息

| 字段 | 值 |
| --- | --- |
| 报告日期 | YYYY-MM-DD |
| 验收人 |  |
| 环境 | dev / staging / prod-like |
| 租户 |  |
| 用户 / 角色 |  |
| 目标平台 | SHEIN |
| 店铺 |  |
| 来源类型 | SDS / 1688 / image_url / text / manual |
| 场景名称 |  |
| 预期结果 | 保存草稿 / 发布 / 失败恢复 |
| 结论 | pass / fail / blocked |

## 2. 输入信息

### 2.1 来源素材

```text
来源 URL / SDS product id / batch id / 图片 URL / 文本摘要：
```

### 2.2 关键配置

| 配置项 | 状态 | 说明 |
| --- | --- | --- |
| AI client | unknown / ok / failed |  |
| SHEIN token / 权限 | unknown / ok / failed |  |
| SHEIN 类目接口 | unknown / ok / failed |  |
| SDS 连接 / 登录态 | unknown / ok / failed |  |
| 图片模型配置 | unknown / ok / failed |  |
| 价格规则 | unknown / ok / failed |  |
| 对象存储 | unknown / ok / failed |  |
| Queue / Worker | unknown / ok / failed |  |

## 3. 任务创建

| 字段 | 值 |
| --- | --- |
| 创建入口 | `/listing-kits/new` / `/listing-kits/shein` / Studio / API |
| 请求时间 |  |
| task_id |  |
| batch_id |  |
| 请求是否成功 | yes / no |
| 页面跳转是否正确 | yes / no |

### 3.1 请求摘要

```json
{}
```

### 3.2 响应摘要

```json
{}
```

### 3.3 问题记录

```text
无 / 记录异常：
```

## 4. 状态流转

| 时间 | 状态 | 页面表现 | 后端响应摘要 | 是否符合预期 |
| --- | --- | --- | --- | --- |
|  | pending / queued |  |  | yes / no |
|  | processing / running |  |  | yes / no |
|  | completed / succeeded |  |  | yes / no |
|  | needs_review / review_ready |  |  | yes / no |
|  | failed / error |  |  | yes / no |

### 4.1 未知状态

| 状态值 | 来源接口 | 页面表现 | 处理结论 |
| --- | --- | --- | --- |
|  |  |  |  |

要求：未知状态不能静默通过，必须补映射或记录为待关闭问题。

## 5. Workspace 验收

| 区域 | 是否可见 | 是否有数据 | 是否可编辑 | 说明 |
| --- | --- | --- | --- | --- |
| 商品事实 | yes / no | yes / no | n/a / yes / no |  |
| 类目 | yes / no | yes / no | yes / no |  |
| 普通属性 | yes / no | yes / no | yes / no |  |
| 销售属性 | yes / no | yes / no | yes / no |  |
| 图片 | yes / no | yes / no | yes / no |  |
| 价格 | yes / no | yes / no | yes / no |  |
| SKU | yes / no | yes / no | yes / no |  |
| 历史版本 | yes / no | yes / no | yes / no |  |
| 提交区 | yes / no | yes / no | yes / no |  |

### 5.1 Workspace payload 摘要

```json
{}
```

### 5.2 缺失数据

| 缺失字段 / 区域 | 是否阻断 | 用户是否可理解 | 处理结论 |
| --- | --- | --- | --- |
|  | yes / no | yes / no |  |

## 6. Readiness 验收

| 字段 | 值 |
| --- | --- |
| readiness 结果 | ready / warning / blocked |
| 是否允许确认最终稿 | yes / no |
| 是否允许保存草稿 | yes / no |
| 是否允许发布 | yes / no |

### 6.1 Blocker 明细

| blocker_key | severity | domain | message | repair_target | 是否能跳转 | 是否已知 |
| --- | --- | --- | --- | --- | --- | --- |
|  | blocker / warning / info | category / attribute / sale_attribute / image / price / sku / store / remote / system |  |  | yes / no | yes / no |

### 6.2 Unknown blocker

| 原始 key / label / message | 来源接口 | 页面兜底表现 | 关闭动作 |
| --- | --- | --- | --- |
|  |  |  |  |

要求：unknown blocker 必须进入待关闭清单，不能只靠前端 message 临时展示。

## 7. 修复动作验收

| 修复项 | 入口 | 操作结果 | readiness 是否更新 | 是否符合预期 |
| --- | --- | --- | --- | --- |
| 类目 |  |  | yes / no | yes / no |
| 普通属性 |  |  | yes / no | yes / no |
| 销售属性 |  |  | yes / no | yes / no |
| 图片 |  |  | yes / no | yes / no |
| 价格 |  |  | yes / no | yes / no |
| SKU |  |  | yes / no | yes / no |

## 8. 最终稿确认

| 字段 | 值 |
| --- | --- |
| 是否能确认最终稿 | yes / no |
| 确认前 readiness | ready / warning / blocked |
| 确认后提交状态 | pending_confirmation / ready_to_submit / other |
| 历史版本是否记录 | yes / no |

### 8.1 请求 / 响应摘要

```json
{}
```

## 9. 保存草稿验收

| 字段 | 值 |
| --- | --- |
| 是否执行 | yes / no |
| idempotency_key |  |
| attempt_id |  |
| 最终状态 | succeeded / failed / skipped |
| SHEIN 远端 draft id |  |
| 是否重复点击验证 | yes / no |
| 是否重复远端提交 | yes / no / unknown |

### 9.1 提交阶段记录

| 时间 | phase | status | 页面表现 | 错误信息 |
| --- | --- | --- | --- | --- |
|  | validate | pending / running / succeeded / failed |  |  |
|  | prepare_product | pending / running / succeeded / failed |  |  |
|  | upload_images | pending / running / succeeded / failed |  |  |
|  | pre_validate | pending / running / succeeded / failed |  |  |
|  | submit_remote | pending / running / succeeded / failed |  |  |
|  | persist_result | pending / running / succeeded / failed |  |  |

### 9.2 请求 / 响应摘要

```json
{}
```

## 10. 发布验收

| 字段 | 值 |
| --- | --- |
| 是否执行 | yes / no |
| idempotency_key |  |
| attempt_id |  |
| 最终状态 | succeeded / failed / skipped |
| SHEIN 远端 product / publish id |  |
| 是否重复点击验证 | yes / no |
| 是否重复远端提交 | yes / no / unknown |

### 10.1 提交阶段记录

| 时间 | phase | status | 页面表现 | 错误信息 |
| --- | --- | --- | --- | --- |
|  | validate | pending / running / succeeded / failed |  |  |
|  | prepare_product | pending / running / succeeded / failed |  |  |
|  | upload_images | pending / running / succeeded / failed |  |  |
|  | pre_validate | pending / running / succeeded / failed |  |  |
|  | submit_remote | pending / running / succeeded / failed |  |  |
|  | persist_result | pending / running / succeeded / failed |  |  |

### 10.2 请求 / 响应摘要

```json
{}
```

## 11. 失败恢复验收

| 字段 | 值 |
| --- | --- |
| 是否发生失败 | yes / no |
| 失败阶段 |  |
| 错误码 |  |
| 错误信息 |  |
| 是否 recoverable | yes / no / unknown |
| UI 是否说明下一步 | yes / no |
| 运营是否可自行处理 | yes / no |
| 是否需要工程介入 | yes / no |
| 恢复动作 | retry / edit / refresh / inspect / engineering |
| 恢复结果 | succeeded / failed / skipped |

### 11.1 恢复过程记录

```text
记录操作步骤、页面表现、接口响应和最终状态：
```

## 12. 指标记录

| 指标 | 值 |
| --- | --- |
| 从选品到任务创建耗时 |  |
| 从任务创建到完成耗时 |  |
| 从任务完成到进入工作台耗时 |  |
| 从任务完成到可提交耗时 |  |
| 修复次数 |  |
| readiness 是否一次通过 | yes / no |
| 保存草稿是否成功 | yes / no / skipped |
| 发布是否成功 | yes / no / skipped |
| 是否无需工程介入恢复 | yes / no / n/a |
| 未知状态数量 |  |
| 未知 blocker key 数量 |  |

## 13. 验收结论

### 13.1 结论

```text
pass / fail / blocked
```

### 13.2 必须关闭的问题

| 编号 | 问题 | 严重级别 | 负责人 | 关闭标准 |
| --- | --- | --- | --- | --- |
|  |  | P0 / P1 / P2 |  |  |

### 13.3 后续建议

```text
记录下一轮联调需要重点验证的内容：
```
