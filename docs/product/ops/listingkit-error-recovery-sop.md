# ListingKit 错误恢复 SOP

## 目的

这份 SOP 用来把 ListingKit 失败场景从“找工程查日志”变成“运营先按页面提示处理，必要时带齐信息交给工程”。

适用范围：SDS / 来源素材到 SHEIN 工作台，再到保存草稿或发布的主链路。后续 TEMU、Amazon、Walmart 可以复用本模型。

## 最小记录字段

任何失败都至少记录：

```text
task_id
batch_id
tenant_id
store_id
target_platform
source_type
current_page
current_status
readiness blockers
submit attempt_id
submit phase
error_code
error_message
接口响应摘要
页面截图
用户已尝试动作
```

## 处理分级

| 分级 | 含义 | 处理方式 |
| --- | --- | --- |
| L1 运营可处理 | 页面能解释原因，运营能补资料、重试、切换入口或重新创建任务。 | 运营按 SOP 处理并记录结果。 |
| L2 运营负责人处理 | 影响一批任务、店铺配置、批量提交或重复失败。 | 负责人汇总 task_id / batch_id 后处理或升级。 |
| L3 工程介入 | 接口异常、未知状态、未知 blocker、数据不一致、远端状态不明。 | 带齐最小记录字段交给工程。 |

## 通用恢复流程

1. 在任务列表确认当前任务状态和下一步动作。
2. 如果有 readiness blocker，先进入工作台修复对应区域。
3. 如果是生成失败或提交失败，进入队列页查看 Review / Retry / Inspect 建议。
4. 如果页面提供 Retry，先重试一次。
5. 如果同类失败重复出现，按 batch / 店铺 / 失败阶段汇总。
6. 如果出现未知状态、未知 blocker、空错误响应或远端状态不明，升级工程。

## 错误场景

### 1. 图片上传失败

| 字段 | 内容 |
| --- | --- |
| 现象 | 图片区显示上传失败，或提交 phase 停在 `upload_images`。 |
| 可能原因 | 图片 URL 失效、对象存储失败、SHEIN 图片接口失败、图片格式或尺寸不符合要求。 |
| 运营动作 | 重新生成图片；替换失败图片；只重试失败项；再次执行保存草稿 / 发布。 |
| 工程边界 | 同一图片反复失败、远端返回空错误、对象存储不可用、图片状态本地和远端不一致。 |
| 必填记录 | task_id、图片 ID / URL、submit attempt_id、phase、错误响应。 |

### 2. SDS 同步失败

| 字段 | 内容 |
| --- | --- |
| 现象 | SDS 商品库无法加载、变体缺失、Studio 无法创建任务。 |
| 可能原因 | SDS 接口不可用、登录态失效、商品已下架、变体数据缺失。 |
| 运营动作 | 在设置页检查 SDS 连接；重新选择商品；刷新 SDS 数据；换可用变体。 |
| 工程边界 | SDS 接口持续失败、返回结构变化、变体映射异常。 |
| 必填记录 | SDS 商品 ID、变体 ID、task_id / batch_id、接口响应摘要。 |

### 3. 类目解析失败

| 字段 | 内容 |
| --- | --- |
| 现象 | readiness 出现 category blocker，工作台类目为空或不可信。 |
| 可能原因 | 来源商品信息不足、平台类目映射缺失、SHEIN 类目接口失败。 |
| 运营动作 | 进入类目修复区手动选择类目；保存后重新检查 readiness。 |
| 工程边界 | 类目接口异常、已知类目无法保存、保存后 blocker 不消失。 |
| 必填记录 | task_id、source title、推荐类目、用户选择类目、blocker key。 |

### 4. 属性缺失

| 字段 | 内容 |
| --- | --- |
| 现象 | readiness 出现 required attribute 或 sale attribute blocker。 |
| 可能原因 | 来源属性不足、AI 抽取失败、平台必填属性变更。 |
| 运营动作 | 进入普通属性或销售属性区补齐；保存后重新检查 readiness。 |
| 工程边界 | 必填属性列表错误、属性值无法保存、平台属性结构变化。 |
| 必填记录 | blocker key、属性 ID / 名称、当前值、期望值、保存响应。 |

### 5. workspace 缺数据

| 字段 | 内容 |
| --- | --- |
| 现象 | 任务已 completed / needs_review，但工作台区域为空。 |
| 可能原因 | 生成结果未持久化、聚合接口缺字段、任务状态提前完成。 |
| 运营动作 | 刷新任务；从任务状态页重新进入；查看队列页恢复建议。 |
| 工程边界 | payload 缺关键区域、状态和数据不一致、刷新后仍为空。 |
| 必填记录 | task_id、workspace payload 摘要、缺失区域、状态流转。 |

### 6. 保存草稿失败

| 字段 | 内容 |
| --- | --- |
| 现象 | 保存草稿失败，或 submit phase 停在 `pre_validate` / `submit_remote` / `persist_result`。 |
| 可能原因 | readiness 未通过、SHEIN token 失效、远端校验失败、网络超时、本地结果持久化失败。 |
| 运营动作 | 查看失败 phase；按 blocker 修复；检查店铺配置；使用同一任务重试。 |
| 工程边界 | 远端可能已创建草稿但本地失败、同一 idempotency key 返回不一致、空错误响应。 |
| 必填记录 | task_id、attempt_id、idempotency_key、phase、远端返回、是否出现 draft id。 |

### 7. 发布失败

| 字段 | 内容 |
| --- | --- |
| 现象 | 发布失败，或已保存草稿但正式发布失败。 |
| 可能原因 | 平台远端校验失败、店铺权限不足、价格 / 库存 / 图片规则不通过。 |
| 运营动作 | 查看提交报告；修复远端校验项；重新执行发布；必要时先保存草稿。 |
| 工程边界 | 远端状态不明、重复发布风险、发布成功但本地未记录。 |
| 必填记录 | task_id、attempt_id、remote product id、phase、错误响应。 |

### 8. AI 生成失败

| 字段 | 内容 |
| --- | --- |
| 现象 | 标题、卖点、描述、属性生成缺失或任务失败。 |
| 可能原因 | AI client 不可用、模型配置缺失、请求超时、输入内容不足。 |
| 运营动作 | 在设置页检查 AI client；补充输入信息；重新生成。 |
| 工程边界 | 模型接口持续失败、结构化输出无法解析、重试后仍失败。 |
| 必填记录 | task_id、模型配置名、错误码、输入摘要、输出摘要。 |

### 9. 配置缺失或失效

| 字段 | 内容 |
| --- | --- |
| 现象 | 多个任务在同一阶段失败，或新建任务前健康检查失败。 |
| 可能原因 | AI key、SHEIN token、SDS 登录态、图片模型、价格规则、对象存储配置异常。 |
| 运营动作 | 到设置页运行健康检查；修复配置后再重试任务。 |
| 工程边界 | 健康检查误报、权限接口异常、配置保存失败。 |
| 必填记录 | 配置项、健康检查结果、影响范围、失败任务样本。 |

## 升级工程时的消息模板

```text
问题类型：
影响范围：单任务 / 批量 / 店铺 / 全局
task_id：
batch_id：
store_id：
target_platform：
当前状态：
readiness blockers：
submit attempt_id：
submit phase：
error_code：
error_message：
接口响应摘要：
页面截图：
已尝试动作：
期望结果：
```

## QA 失败样例清单

详细执行步骤见 [`../validation/listingkit-qa-failure-samples.md`](../validation/listingkit-qa-failure-samples.md)。

| 样例 | 触发方式 | 期望页面表现 | 期望恢复动作 |
| --- | --- | --- | --- |
| 图片上传失败 | 使用失效图片 URL 或模拟图片接口失败 | 显示图片区失败和可重试入口 | 替换图片或重试失败项 |
| 属性缺失 | 构造缺少必填属性的商品 | blocker 跳到属性区 | 补齐属性后 readiness 通过 |
| 保存草稿失败 | 使用失效 SHEIN token | submit phase 显示失败 | 设置页修复 token 后重试 |
| 发布失败 | 构造远端校验不通过资料 | 提交报告显示远端原因 | 修复资料后重新发布 |
| unknown blocker | 模拟新 blocker key | 显示兜底，不让用户无路可走 | 记录并补 taxonomy |
