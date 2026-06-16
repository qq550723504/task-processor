# ListingKit 错误恢复 SOP

## 目的

这份 SOP 用于把 ListingKit 的失败处理从“找工程查日志”推进到“运营可理解、可处理、可升级”。

它覆盖真实运营中常见的失败：生成失败、workspace 缺数据、readiness 阻断、保存草稿失败、发布失败、远端校验失败、配置失效等。

## 使用对象

| 角色 | 使用方式 |
| --- | --- |
| 上架运营 | 根据页面错误和下一步动作自行修复、重试或升级 |
| 运营负责人 | 每日复盘失败任务，判断重复问题和配置风险 |
| QA | 主动触发失败样例，确认页面和接口表现符合预期 |
| 工程 | 根据 task_id、attempt_id、接口响应和失败阶段定位问题 |

## 通用分级

| 等级 | 含义 | 处理方式 |
| --- | --- | --- |
| P0 | 阻断保存草稿 / 发布，且用户无恢复路径 | 必须立即修复或提供兜底恢复 |
| P1 | 阻断当前任务，但用户可通过明确动作恢复 | 进入恢复流程，记录样例 |
| P2 | 不阻断提交，但影响质量或效率 | 记录并排期优化 |

## 通用处理流程

```text
看到失败
  -> 记录 task_id / batch_id / attempt_id
  -> 判断失败阶段
  -> 判断是否 recoverable
  -> 优先执行 UI 给出的下一步动作
  -> 如果恢复成功，记录恢复动作
  -> 如果恢复失败，升级工程并附带接口响应和页面表现
```

## 升级工程时必须提供的信息

```text
task_id
batch_id（如果有）
attempt_id（如果是提交失败）
tenant / shop / target_platform
来源类型：SDS / 1688 / image / text / manual
失败页面 URL
失败阶段：generation / workspace / readiness / save_draft / publish / remote_validation / config / unknown
错误码
错误 message
接口请求摘要
接口响应摘要
页面截图或页面表现描述
用户已尝试的恢复动作
是否重复点击或重复提交
```

## 错误场景矩阵

| 场景 | 用户看到什么 | 运营动作 | 工程介入边界 | 必填记录 |
| --- | --- | --- | --- | --- |
| 图片上传失败 | 图片区失败、提交阶段卡在 upload_images | 重试上传 / 检查图片 / 替换图片 | 多次重试失败、远端返回未知错误、对象存储异常 | task_id、image id、attempt_id、phase、remote response |
| SDS 同步失败 | SDS 商品缺失或 Studio 无法创建任务 | 刷新 SDS / 重新选择商品 / 检查登录态 | SDS 接口不可用、登录态反复失效 | SDS product id、variant id、接口响应 |
| 类目解析失败 | readiness 阻断或类目为空 | 手动选择类目 / 重跑类目推荐 | 类目接口异常、无可选类目、映射规则错误 | task_id、source category、target category response |
| 属性缺失 | 普通属性或销售属性 blocker | 补齐属性 / 保存修改 / 重新 readiness | 必填属性列表错误、属性保存失败 | blocker_key、attribute id、payload |
| workspace 缺数据 | 工作台空白或某区域缺失 | 刷新任务 / 回状态页检查生成结果 | payload 缺核心字段、接口 500、数据不一致 | workspace response、task status |
| 保存草稿失败 | 提交失败，phase 显示失败 | 查看失败阶段 / 重试 / 修改资料 | 远端已成功但本地未记录、重复提交风险 | attempt_id、idempotency_key、phase、remote response |
| 发布失败 | 发布阶段失败 | 按页面提示修复 / 重试发布 | SHEIN 远端状态不明、重复发布风险 | attempt_id、remote id、phase、error |
| SHEIN 远端校验失败 | pre_validate 或 submit_remote 失败 | 查看远端校验项 / 回工作台修复 | 远端错误无法映射到 blocker | remote error code、message、payload |
| AI 生成失败 | 标题 / 卖点 / 属性生成失败 | 重试生成 / 检查输入 | AI client 不可用、模型配置错误 | prompt id、model、error |
| 图片模型配置缺失 | 图片生成或处理失败 | 到设置页检查图片配置 | 配置正确但仍失败 | config health result、task_id |
| 店铺 token / 权限失效 | 保存草稿 / 发布 / 类目接口失败 | 到设置页重新检查店铺连接 | token 刷新失败、权限异常 | shop id、scope、接口响应 |

## 场景 SOP

### 1. 图片上传失败

典型表现：

```text
提交阶段停在 upload_images
图片区显示上传失败
保存草稿或发布失败原因指向图片上传
```

运营动作：

1. 打开工作台图片区，确认失败图片是否可预览。
2. 如果图片缺失或损坏，替换图片或重新生成图片。
3. 如果图片正常，点击重试上传或重试保存草稿。
4. 重试后观察提交阶段是否继续进入 `pre_validate`。

需要工程介入：

- 同一张图片多次上传失败。
- 所有图片上传失败。
- 远端返回未知错误。
- 对象存储 URL 无法访问。
- UI 没有给出可重试动作。

必须记录：

```text
task_id
attempt_id
image id / image url
phase = upload_images
remote response
object storage url 是否可访问
```

### 2. SDS 同步失败

典型表现：

```text
SDS 商品库缺数据
Studio 无法载入商品、变体、模板或 mockup
创建 ListingKit 任务失败
```

运营动作：

1. 刷新 SDS 商品库。
2. 检查筛选条件是否过窄。
3. 重新选择 SDS 商品和变体。
4. 到设置页检查 SDS 连接或登录态。

需要工程介入：

- SDS 接口不可用。
- 登录态反复失效。
- 指定 SDS 商品在接口有数据但 UI 不展示。
- Studio 上下文缺模板、印花层、可印刷尺寸或 mockup。

必须记录：

```text
SDS product id
variant id
batch_id
task_id（如果已创建）
SDS 接口响应摘要
```

### 3. 类目解析失败

典型表现：

```text
类目为空
readiness 返回 category blocker
提交前校验提示类目缺失或不匹配
```

运营动作：

1. 打开类目修复区。
2. 使用推荐类目或手动选择类目。
3. 保存类目。
4. 重新运行 readiness。

需要工程介入：

- 类目接口失败。
- 没有任何可选类目。
- 保存类目成功但 readiness 仍使用旧类目。
- blocker 无法跳转到类目修复区。

必须记录：

```text
task_id
source title / source category
target category id
blocker_key
类目接口响应
保存类目请求 / 响应
```

### 4. 属性缺失

典型表现：

```text
普通属性 blocker
销售属性 blocker
SKU 或变体缺失属性
```

运营动作：

1. 点击 blocker，进入属性修复区域。
2. 补齐必填属性。
3. 保存修改。
4. 重新运行 readiness。

需要工程介入：

- blocker 没有明确 attribute id。
- 必填属性无法编辑。
- 保存成功但 readiness 不更新。
- SHEIN 属性规则与 UI 展示不一致。

必须记录：

```text
task_id
blocker_key
attribute id
attribute value
保存请求 / 响应
readiness 前后结果
```

### 5. Workspace 缺数据

典型表现：

```text
工作台某区域空白
任务显示完成但 workspace payload 缺核心字段
页面无法进入修复流程
```

运营动作：

1. 返回状态页确认任务是否真正完成。
2. 刷新 workspace。
3. 如果页面提供恢复入口，执行恢复。
4. 如果仍缺数据，升级工程。

需要工程介入：

- task status 是 completed，但 workspace payload 为空。
- 商品事实、图片、类目或属性核心区域缺失。
- UI 没有兜底说明。
- 接口返回未知结构。

必须记录：

```text
task_id
task status response
workspace response
缺失区域
页面表现
```

### 6. 保存草稿失败

典型表现：

```text
点击保存草稿后失败
提交阶段显示 validate / prepare_product / upload_images / pre_validate / submit_remote / persist_result 之一失败
```

运营动作：

1. 查看失败阶段。
2. 如果是 readiness 或资料问题，回工作台修复。
3. 如果是图片上传失败，按图片失败 SOP。
4. 如果是 SHEIN 远端校验失败，查看远端错误并修复对应字段。
5. 如果页面显示可重试，使用同一任务的重试入口。

需要工程介入：

- 远端可能已保存草稿但本地状态失败。
- 重复点击可能导致重复草稿。
- attempt 状态和远端状态不一致。
- idempotency key 缺失。
- phase 不可见或错误信息为空。

必须记录：

```text
task_id
attempt_id
idempotency_key
phase
action = save_draft
remote draft id（如果有）
请求 / 响应摘要
```

### 7. 发布失败

典型表现：

```text
点击发布后失败
保存草稿可能已成功，但发布未成功
远端返回发布校验错误
```

运营动作：

1. 查看失败阶段和错误原因。
2. 如果是资料问题，回工作台修复。
3. 如果是远端校验失败，按提示修复字段或图片。
4. 如果页面显示可重试，重试发布。
5. 确认是否已有远端 product / publish id。

需要工程介入：

- SHEIN 远端状态不明。
- 本地显示失败但远端已发布。
- 重试可能重复发布。
- 远端错误无法映射到 blocker。

必须记录：

```text
task_id
attempt_id
idempotency_key
action = publish
phase
remote product id
remote publish id
remote response
```

### 8. SHEIN 远端校验失败

典型表现：

```text
pre_validate 失败
submit_remote 返回字段级错误
错误来自 SHEIN 远端而非本地 readiness
```

运营动作：

1. 查看远端错误 message。
2. 如果错误能映射到类目、属性、图片、价格或 SKU，进入对应修复区。
3. 修复后重新 readiness。
4. 再次保存草稿或发布。

需要工程介入：

- 远端错误无法映射到任何修复区。
- 远端错误只有空 message。
- 本地 readiness 通过但远端稳定失败。
- 同类错误重复出现。

必须记录：

```text
task_id
attempt_id
phase = pre_validate / submit_remote
remote error code
remote error message
对应 blocker_key（如果有）
```

## 运营和工程边界

运营可处理：

- 补齐类目、属性、销售属性、价格、SKU。
- 替换或重新生成图片。
- 使用 UI 提供的重试入口。
- 重新检查设置页配置状态。
- 根据明确 blocker 跳转修复资料。

需要工程介入：

- unknown task status。
- unknown blocker key 且 UI 无修复入口。
- 空错误响应。
- 本地状态和远端状态不一致。
- 重复提交风险。
- 接口 500 / 数据结构异常。
- 已确认配置正常但接口仍失败。

## QA 失败样例建议

每个主要版本至少触发：

```text
1 个图片上传失败样例
1 个属性缺失 blocker 样例
1 个 readiness blocked -> 修复 -> ready 样例
1 个保存草稿失败恢复样例
1 个重复点击保存草稿幂等样例
1 个 SHEIN 远端校验失败样例
```

## SOP 维护规则

- 真实接口验收报告中出现的新错误，必须判断是否更新本 SOP。
- unknown blocker 不能长期停留在报告里，必须补 taxonomy 或明确不支持原因。
- 同一错误连续出现 3 次，应提升为产品或工程修复项。
- 若某错误需要工程介入，UI 至少应告诉用户需要提供哪些字段。
