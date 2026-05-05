# ListingKit Real API Validation Checklist

目标：在内部测试阶段，用真实后端接口验证 `listing-kits/new -> status -> workspace -> submit` 主链路是否与当前前端状态机一致。

适用范围：
- `D:\code\task-processor\web\listingkit-ui`
- 仅覆盖 ListingKit UI 当前已产品化的核心链路
- 不覆盖登录、权限、租户、组织隔离

## 1. 创建任务

入口：
- `/listing-kits/new`
- `/listing-kits/shein`

要验证的真实接口结果：
- 创建任务成功后是否稳定返回 `task_id`
- 创建后初始状态是否可能出现：
  - `pending`
  - `queued`
  - `processing`
  - `running`
- 失败时是否返回用户可读 `message`
- 图片上传失败时是否返回明确错误，而不是空 body

前端期望行为：
- 创建成功后进入状态页
- 状态页能显示任务 ID、创建时间、最近更新时间
- `pending/queued` 显示“正在等待开始”
- `processing/running` 显示“正在生成图片”

重点记录：
- 创建接口是否有空响应
- 是否存在新的状态别名
- 失败时 `payload.message` 是否存在

## 2. 状态页

入口：
- `/listing-kits/[taskId]/status`

要验证的真实接口结果：
- 轮询接口是否会返回：
  - `pending`
  - `queued`
  - `processing`
  - `running`
  - `completed`
  - `succeeded`
  - `needs_review`
  - `review_ready`
  - `failed`
  - `error`
- `created_at` / `result.updated_at` / `completed_at` 是否稳定存在
- 失败时 `error` 是否挂在顶层或 `child_tasks`
- 人工确认原因是否来自：
  - `review_reasons`
  - `error` 拼接串

前端期望行为：
- `completed/succeeded` 允许自动进入工作台
- `needs_review/review_ready` 允许自动进入工作台
- `failed/error` 停留在状态页，不自动跳转
- 页面级失败时显示：
  - “任务状态暂时无法加载”
  - “任务状态暂未准备完成”

重点记录：
- 轮询停止点是否与真实终态一致
- 是否存在“状态已完成但时间未更新”的情况
- 失败时 `child_tasks` 是否足够定位问题

## 3. 工作台

入口：
- `/listing-kits/[taskId]/workspace`

要验证的真实接口结果：
- 预览接口是否稳定返回：
  - `preview`
  - `selected_platform`
  - `shein`
  - `submit_readiness`
  - `final_review`
- 会话接口是否稳定返回：
  - `session`
  - `platform_cards`
  - `sections`
  - `overview`
- 工作台读取失败时是否返回明确错误 body

前端期望行为：
- 页面头部显示任务标题、任务状态、最近更新
- 页面级失败时显示：
  - “工作台暂时无法加载”
  - “工作台数据暂未准备完成”
- `SHEIN 审核流程` 步骤条与真实数据一致
- 阻断项点击后能跳到正确区域：
  - 类目
  - 普通属性
  - 销售属性
  - 图片
  - 价格 / SKU

重点记录：
- 是否存在新的阻断项 key
- 是否存在新的图片相关别名 key
- 是否存在新的价格/库存相关 key

## 4. 提交准备

入口：
- 工作台右侧 `SHEIN 发布检查`
- 最终确认面板 `SHEIN 最终确认`

要验证的真实接口结果：
- readiness 状态是否可能出现：
  - `blocked`
  - `ready_with_warnings`
  - `ready`
- SHEIN workflow 是否可能出现：
  - `pending_confirmation`
  - `ready_to_submit`
  - `draft_saved`
  - `publish_failed`
  - `published`
- `blocking_items` / `warning_items` 是否含有新 key

前端期望行为：
- readiness 未通过时：
  - 不能提交
  - 有明确阻断项
  - 能跳到对应修复区
- readiness 通过但未确认最终草稿时：
  - 不能保存到 SHEIN 草稿箱
  - 不能发布到 SHEIN
- readiness 通过且已确认时：
  - 可保存草稿
  - 可正式发布

重点记录：
- 阻断项 key 是否和当前前端映射一致
- 是否存在只返回 `label/message` 不返回 `key` 的阻断项
- 最终图片角色是否在真实数据里完整返回

## 5. 保存草稿 / 正式发布

入口：
- `保存到 SHEIN 草稿箱`
- `发布到 SHEIN`

要验证的真实接口结果：
- 保存草稿成功后是否稳定回写最新提交记录
- 发布成功后是否稳定回写最新提交记录
- 失败时是否稳定返回：
  - `payload.message`
  - `submission.last_error`
  - `submission.last_result.validation_notes`

前端期望行为：
- 发布前必须进入确认态
- 失败后必须看到三段式反馈：
  - 发生了什么
  - 可能影响
  - 下一步怎么做
- 成功后要能在当前页看到最近结果摘要

重点记录：
- 失败是否只返回 HTTP code，没有 message
- 校验提示是否主要出现在 `validation_notes`
- 发布成功后 workflow 状态是否立即变化

## 6. 失败样例清单

联调时至少主动覆盖 1 次：
- 图片上传失败
- 任务生成失败
- 工作台预览缺失
- 类目搜索失败
- 保存草稿失败
- 正式发布失败

每个失败样例都要记录：
- 触发入口
- 实际接口返回
- 页面显示文案
- 是否知道下一步去哪
- 是否需要刷新页面才能恢复

## 7. 状态映射基线

当前前端已支持的任务状态映射：

| 后端状态 | 前端语义 |
| --- | --- |
| `pending` | 待开始 |
| `queued` | 待开始 |
| `processing` | 处理中 |
| `running` | 处理中 |
| `completed` | 已完成 |
| `succeeded` | 已完成 |
| `needs_review` | 待确认 |
| `review_ready` | 待确认 |
| `failed` | 失败 |
| `error` | 失败 |

当前前端已支持的工作台阻断项映射：

| 阻断项 key 或别名 | 跳转区域 |
| --- | --- |
| `category`, `category_review` | 类目确认 |
| `attributes`, `attribute_review`, `required_attribute` | 普通属性 |
| `sale_attributes`, `variants`, `variant_mapping` | 销售属性 |
| `images`, `preview_product`, `final_images` | 图片资料 |
| `price`, `inventory`, `stock`, `quantity` | 价格 / SKU |

如果真实接口返回新值：
- 先记录原始 key
- 确认应该跳到哪个区域
- 再补前端归一化映射

## 8. 联调通过标准

满足以下条件才算这轮真实接口联调通过：
- 成功创建任务并进入状态页
- 成功进入工作台并看到完整上下文
- 成功保存到 SHEIN 草稿箱
- 成功发布到 SHEIN
- 至少验证 1 条失败恢复链路
- 没有新的未知任务状态
- 没有新的未知阻断项 key
- 页面级错误态都能给出下一步动作

## 9. 联调记录模板

每次联调建议按下面格式记录：

```md
### 场景
- 页面：
- task_id：
- 操作：

### 实际接口
- 状态码：
- message：
- status：
- workflow_status：
- blocking_keys：

### 页面表现
- 是否知道当前在哪：
- 是否知道下一步做什么：
- 是否需要刷新页面：

### 结论
- 通过 / 不通过
- 需要补的字段或映射：
```
