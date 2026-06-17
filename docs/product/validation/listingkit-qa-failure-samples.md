# ListingKit QA 失败样例手册

本手册把错误恢复 SOP 中的 QA 样例拆成可执行检查项。每次主要版本验收至少选择一个样例执行，并把结果写入 `validation/runs/`。

所有样例都必须记录：

```text
task_id
batch_id
tenant_id
store_id
target_platform
source_type
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

## 样例 1：图片上传失败

目标：验证图片失败能进入图片区或提交阶段的可恢复路径。

### 触发方式

| 方式 | 适用环境 | 操作 |
| --- | --- | --- |
| 失效图片 URL | dev / staging | 创建任务时使用 404 图片 URL 或过期对象存储 URL。 |
| 模拟 SHEIN 图片接口失败 | staging / prod-like | 在测试店铺或 mock 网关中让图片上传接口返回非 2xx。 |
| 图片格式不合法 | dev / staging | 使用不符合 SHEIN 规则的图片格式或尺寸。 |

### 期望页面表现

- 图片区标记失败图片。
- 提交阶段显示 `upload_images` 失败。
- 页面提供替换图片、重新生成图片或重试失败项入口。

### 恢复动作

1. 替换失败图片或重新生成图片。
2. 重新运行 readiness。
3. 使用同一任务重试保存草稿或发布。
4. 在 run 报告中记录重试是否复用原 attempt 或产生新 attempt。

### 必填记录

| 字段 | 要求 |
| --- | --- |
| image id / image url | 记录失败图片和替换后图片。 |
| submit phase | 必须是 `upload_images` 或明确说明为何不是。 |
| remote response | 记录 SHEIN 图片接口错误码和 message。 |
| recovery result | `succeeded` / `failed` / `blocked`。 |

## 样例 2：属性缺失

目标：验证普通属性或销售属性 blocker 能跳到正确修复区。

### 触发方式

| 方式 | 适用环境 | 操作 |
| --- | --- | --- |
| 删除必填普通属性 | dev / staging | 创建任务后手动清空一个 SHEIN 必填普通属性。 |
| 删除销售属性 | dev / staging | 清空颜色、尺寸等销售属性映射。 |
| 使用属性不足的来源商品 | dev / staging | 选择缺少关键规格信息的 SDS / 来源素材。 |

### 期望页面表现

- readiness 返回稳定 blocker key。
- blocker 显示用户能理解的字段名。
- 点击 blocker 能跳到普通属性区或销售属性区。
- unknown blocker 必须进入 `unknown-state-and-blocker-tracker.md`。

### 恢复动作

1. 进入对应属性修复区。
2. 补齐属性值并保存。
3. 重新运行 readiness。
4. 验证 blocker 消失或被更具体的新 blocker 替代。

### 必填记录

| 字段 | 要求 |
| --- | --- |
| blocker key | 不允许只记录中文 message。 |
| attribute id / name | 记录平台属性 ID 和页面展示名。 |
| before / after value | 记录修复前后值。 |
| readiness result | 记录修复前后 blocker 列表。 |

## 样例 3：保存草稿失败

目标：验证保存草稿失败能显示 phase、原因和可恢复动作，且不会重复创建远端草稿。

### 触发方式

| 方式 | 适用环境 | 操作 |
| --- | --- | --- |
| 失效 SHEIN token | staging / prod-like | 使用测试店铺失效 token 执行保存草稿。 |
| 远端校验失败 | staging / prod-like | 构造本地 readiness 通过但 SHEIN pre-validate 不通过的 payload。 |
| 本地持久化失败 | dev | 使用测试 double 让 persist_result 返回错误。 |

### 期望页面表现

- 提交阶段显示 `pre_validate`、`submit_remote` 或 `persist_result`。
- 页面展示错误码、错误 message 和是否可重试。
- 重复点击不会产生重复远端草稿。

### 恢复动作

1. 如果是 token 失效，到设置页修复店铺连接。
2. 如果是远端校验失败，回工作台修复对应字段。
3. 使用同一任务重试保存草稿。
4. 比对远端 draft id 和本地 attempt 记录。

### 必填记录

| 字段 | 要求 |
| --- | --- |
| attempt_id | 每次保存草稿必须记录。 |
| idempotency_key | 重试和重复点击必须记录是否复用。 |
| remote draft id | 远端可能成功时必须记录。 |
| duplicate result | 明确 yes / no / unknown。 |

## 样例 4：发布失败

目标：验证发布失败能进入提交报告，用户知道是否先保存草稿、修资料或升级工程。

### 触发方式

| 方式 | 适用环境 | 操作 |
| --- | --- | --- |
| 店铺权限不足 | staging / prod-like | 使用无发布权限的测试店铺执行发布。 |
| 远端发布校验失败 | staging / prod-like | 构造价格、库存、图片或属性不符合 SHEIN 发布规则的 payload。 |
| 远端状态不明 | dev / staging | 模拟 submit_remote 超时或空响应。 |

### 期望页面表现

- 发布 attempt 显示失败 phase。
- 远端错误能映射到修复区或提交报告。
- 如果远端状态不明，页面提示不要重复发布并要求工程介入。

### 恢复动作

1. 查看提交报告。
2. 修复远端校验项。
3. 重新 readiness。
4. 重试发布或先保存草稿。

### 必填记录

| 字段 | 要求 |
| --- | --- |
| remote product id | 如果返回过必须记录。 |
| remote publish id | 如果返回过必须记录。 |
| phase | 必须记录失败阶段。 |
| engineering boundary | 远端状态不明必须升级工程。 |

## 样例 5：unknown blocker

目标：验证未知 blocker 有兜底展示，不会让用户无路可走。

### 触发方式

| 方式 | 适用环境 | 操作 |
| --- | --- | --- |
| mock 新 blocker key | dev | 在 readiness 测试 double 中返回未知 key。 |
| 真实远端新错误 | staging / prod-like | 遇到 SHEIN 新校验错误时不要直接映射为普通 message。 |

### 期望页面表现

- 页面展示 blocker 原始 key / label / message。
- 页面说明当前需要工程介入或进入提交报告。
- run 报告和 unknown tracker 都新增记录。

### 恢复动作

1. 记录原始 blocker key、message、来源接口。
2. 判断是否能映射到 category / attribute / sale_attribute / image / price / sku / store / remote / system。
3. 如果能映射，补 taxonomy 和跳转。
4. 如果不能映射，保留兜底展示并在 SOP 中写清升级字段。

### 必填记录

| 字段 | 要求 |
| --- | --- |
| raw blocker key | 必须记录原始值。 |
| source API | 记录来源接口。 |
| fallback UI | 记录页面兜底文案或截图。 |
| tracker status | 在 unknown tracker 中标为 `open`、`mapped` 或 `closed`。 |

## 执行完成标准

- 至少 1 个样例被真实执行，并写入 `validation/runs/`。
- 执行后更新 `unknown-state-and-blocker-tracker.md`。
- 如果样例没有恢复成功，run 报告中必须写清关闭标准。
- 如果样例需要工程介入，必须使用 SOP 的升级消息模板。
