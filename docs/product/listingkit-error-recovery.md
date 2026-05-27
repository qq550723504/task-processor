# ListingKit 错误恢复手册

## 目的

这份手册用于处理 ListingKit 真实运营中的失败场景。它关注用户如何恢复任务、QA 如何复现问题、工程如何拿到足够信息定位问题。

恢复原则：

1. 先判断用户是否能在 UI 内恢复。
2. 再判断是否需要重新提交、重新生成或重新配置。
3. 最后才升级给工程排查日志和接口数据。

## 通用记录字段

所有失败都至少记录：

- 页面路径
- task_id
- 操作动作
- 当前任务状态
- 当前 workflow / submission 状态
- 接口状态码
- 后端返回 message
- 页面展示文案
- 是否需要刷新页面才能恢复
- 用户下一步是否明确

如果是 SHEIN workspace 相关问题，还要记录：

- readiness status
- blocking item keys
- warning item keys
- selected platform
- latest submission action
- latest submission status
- validation notes

## 1. 任务创建失败

### 典型表现

- 创建任务接口返回非 2xx。
- 页面没有拿到 `task_id`。
- 图片上传失败。
- 返回空 body 或没有用户可读 message。

### 用户可处理动作

- 检查是否至少提供一种有效来源：文本、图片 URL、商品 URL 或 SDS 选择。
- 检查图片 URL 是否可访问。
- 检查 1688 商品链接是否是支持的详情页格式。
- 如果来自 SHEIN Studio，回到 SDS 选择确认变体、模板和图层信息。

### 需要工程介入

- 后端返回空 body。
- 错误信息无法说明失败原因。
- 同一输入多次失败且没有字段级提示。
- 创建成功但没有 task_id。

### 验收标准

- 创建失败时页面必须能说明失败原因或给出可执行下一步。
- 成功创建后必须稳定进入状态页或任务跟踪视图。

## 2. 任务状态长时间停留

### 典型表现

- 长时间停留在 `pending`、`queued`、`processing` 或 `running`。
- 状态显示完成但页面没有跳转。
- completed 时间或 updated 时间没有变化。

### 用户可处理动作

- 等待当前轮询继续刷新。
- 使用任务列表或状态页重新打开任务。
- 如果页面给出重试入口，先重试加载状态。

### 需要工程介入

- 状态超过预期处理窗口仍不变。
- child task 已失败但顶层状态未失败。
- 后端返回未知状态。
- 状态是 `completed` / `succeeded`，但 workspace 数据缺失。

### 验收标准

- 非终态要有明确等待文案。
- 终态要进入正确下一步：工作台、失败页或恢复入口。
- 未知状态必须被记录，不能静默当作处理中。

## 3. 工作台无法加载

### 典型表现

- `/listing-kits/[taskId]/workspace` 显示加载失败。
- preview、session 或 task result 任意一个接口失败。
- 页面提示“工作台数据暂未准备完成”。

### 用户可处理动作

- 点击页面重试。
- 回到状态页确认任务是否真的已经完成。
- 从任务列表重新进入工作台。

### 需要工程介入

- preview 缺少 `shein`、`submit_readiness` 或 `final_review`。
- session 缺少 `session`、`platform_cards`、`sections` 或 `overview`。
- task result 成功但 workspace 聚合失败。
- 页面需要刷新多次才恢复。

### 验收标准

- 工作台失败态必须有重试入口。
- 如果数据未准备好，页面要说明是等待还是失败。
- 缺字段要进入联调记录，不应靠前端猜测补齐。

## 4. SDS 同步失败

### 典型表现

- SHEIN Studio 已选择 SDS 变体，但任务创建后 SDS 相关信息缺失。
- `sds_design_result.status=failed`。
- 兼容字段 `sds_sync.status` 会同步保留。
- 设计图没有正确进入 SDS 设计页或目标图层。

### 用户可处理动作

- 回到 SDS 选择，重新确认变体。
- 确认发货地、模板组和图层信息。
- 重新选择设计图并创建任务。

### 需要工程介入

- SDS 登录态或接口不可用。
- 选中的 variant_id、prototype_group_id、layer_id 与 SDS 返回不一致。
- 后端返回 SDS 同步失败但没有明确原因。

### 验收标准

- SDS 同步失败不应伪装成完整成功。
- 页面要能区分 ListingKit 任务创建成功和 SDS 同步失败。

## 5. 类目解析失败

### 典型表现

- readiness 出现 category / category_review 阻断。
- SHEIN 在线类目解析失败后降级到离线解析。
- 类目缺失或类目置信度不足。

### 用户可处理动作

- 点击阻断项进入类目确认。
- 手动搜索或选择正确类目。
- 保存 revision 后重新检查 readiness。

### 需要工程介入

- SHEIN 店铺 ID 缺失导致在线类目不可用。
- 类目搜索接口失败。
- 类目选择保存后 readiness 不更新。

### 验收标准

- 类目阻断项必须能跳到类目审核区。
- 手动修复后 readiness 应重新计算。

## 6. 属性或销售属性缺失

### 典型表现

- readiness 出现 attributes、required_attribute、sale_attributes、variants 或 variant_mapping 阻断。
- SHEIN 必填属性缺失。
- SKU、颜色、尺码或销售属性映射不完整。

### 用户可处理动作

- 普通属性问题进入属性审核区。
- 变体映射问题进入销售属性区。
- 补齐必填值后保存 revision。
- 重新检查 readiness。

### 需要工程介入

- 后端返回阻断项只有 label/message，没有 key。
- 保存后字段丢失。
- SHEIN 属性模板加载失败。
- 变体数量和 SDS 选择不一致。

### 验收标准

- 所有属性类阻断项都必须可定位。
- 保存后页面应显示实际更新字段或新的阻断项。

## 7. 图片资料失败

### 典型表现

- 图片上传失败。
- 最终图缺失。
- 图片比例和 SDS 可印刷区域不匹配。
- readiness 出现 images、preview_product 或 final_images 阻断。
- 其中 `preview_product` 仍是当前 readiness key，底层主字段已切到 `preview_payload`。

### 用户可处理动作

- 回到图片资料区检查最终图。
- 在 SHEIN Studio 重新生成或选择图片。
- 对图库导入图，检查比例是否超出阈值。
- 重新创建任务或保存 revision。

### 需要工程介入

- 图片代理无法加载。
- 图片模型失败但没有 fallback 或错误提示。
- SHEIN 图片上传返回失败但 submission report 缺失。
- final image role 不完整。

### 验收标准

- 图片阻断项必须能跳到图片资料区。
- 用户能区分生成失败、上传失败和平台校验失败。

## 8. 价格、库存或 SKU 阻断

### 典型表现

- readiness 出现 price、inventory、stock 或 quantity 阻断。
- SKU 数量不完整。
- 价格规则缺失或结果不合法。

### 用户可处理动作

- 进入价格 / SKU 区域。
- 补齐价格、库存和数量。
- 确认是否符合当前店铺价格规则。
- 保存 revision 并重新检查 readiness。

### 需要工程介入

- 设置页价格规则缺失但工作台没有提示。
- SKU 数据保存后未进入最终 payload。
- 后端 readiness 仍返回旧阻断项。

### 验收标准

- 价格和库存阻断项不能只显示文本，必须能定位到可修复区域。
- 修复后 readiness 要刷新。

## 9. 保存到 SHEIN 草稿箱失败

### 典型表现

- 点击 `保存到 SHEIN 草稿箱` 后失败。
- submission report 中 `last_action=save_draft` 且状态失败。
- 失败信息来自图片上传、预校验或 SHEIN 远端接口。

### 用户可处理动作

- 查看失败提示和 validation notes。
- 如果是 readiness 阻断，回工作台修复。
- 如果是图片问题，回图片资料区处理。
- 状态刷新后重新点击保存草稿。

### 需要工程介入

- 没有 current phase 或 last error。
- 重复点击导致多次远端保存。
- 远端返回成功但本地状态未更新。

### 验收标准

- 失败后页面必须说明本次没有保存成功。
- 用户应知道先修图片、资料还是配置。
- 成功后能看到最新保存记录。

## 10. 发布到 SHEIN 失败

### 典型表现

- 点击 `发布到 SHEIN` 后失败。
- workflow 状态为 `publish_failed`。
- validation notes 返回平台校验问题。

### 用户可处理动作

- 查看提交失败详情。
- 按 validation notes 修复对应字段。
- 保存 revision。
- 重新确认最终稿。
- 重新发布。

### 需要工程介入

- 发布失败没有 validation notes、last error 或 message。
- 远端接口超时后本地状态不确定。
- 同一任务重复发布产生重复远端记录。

### 验收标准

- 发布失败要说明“没有正式发布成功”。
- 修复后必须重新确认最终稿。
- 发布成功后 workflow 状态和最近提交记录要同步更新。

## 11. 未知状态或未知阻断项

### 典型表现

- 后端返回不在前端基线里的 task status。
- readiness 返回新 blocker key。
- blocker 只有 label/message，没有 key。

### 用户可处理动作

- 如果页面有兜底文案，按页面提示处理。
- 不要把未知 blocker 当作已通过。

### 需要工程介入

- 任何未知状态或未知 key 都需要进入联调记录。
- 工程需要判断是否补后端 taxonomy、前端映射或兼容文案。

### 验收标准

- 未知状态不能静默变成处理中或成功。
- 未知 blocker 不能静默丢失。
- 修复后要更新 `REAL_API_VALIDATION_CHECKLIST.md` 的映射基线。

## 升级给工程的最小信息包

当问题需要工程介入时，至少提供：

```md
### 场景
- 页面：
- task_id：
- 操作：

### 状态
- task_status：
- readiness_status：
- workflow_status：
- submission_action：
- submission_status：

### 接口
- 失败接口：
- 状态码：
- message：
- blocking_keys：
- validation_notes：

### 页面表现
- 页面文案：
- 是否有下一步：
- 是否刷新后恢复：

### 期望
- 用户原本想完成什么：
- 当前卡在哪里：
```

这个信息包的目标是让工程不用反复追问上下文，就能开始定位。
