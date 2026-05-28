# ListingKit Semantic Field Migration Checklist

## 背景

ListingKit 和 SHEIN 发布链路已经完成一轮语义收口：

- `SDSSync` -> `SDSDesignResult`
- `RequestDraft` -> `DraftPayload`
- `PreviewProduct` -> `PreviewPayload`
- `Submission` -> `SubmissionState`
- `FinalDraft` -> `FinalSubmissionDraft`

当前策略不是硬切旧字段，而是：

1. 内部业务逻辑优先使用新语义字段。
2. JSON 结果同时暴露旧字段和新字段。
3. 兼容层以新语义字段为准，旧字段只做镜像，不再作为主状态来源。

这份清单用于前端、接口调用方、任务恢复逻辑和后续清理阶段统一对齐。

## 当前推荐原则

### 协作约定

从现在开始，代码评审和日常开发默认按下面这条执行：

1. 新业务代码优先读写新语义字段。
2. 旧字段只允许出现在兼容 helper、协议模型、历史数据 fallback、测试夹具里。
3. 如果确实需要新增旧字段使用，PR 里必须说明为什么它属于兼容边界，而不是普通业务逻辑。
4. reviewer 看到新增 `request_draft / preview_product / final_draft / sds_sync` 直接读取或写入时，应默认要求改成新字段或 helper。

### 读取原则

调用方从现在开始应优先读取新字段：

- `sds_design_result`
- `draft_payload`
- `preview_payload`
- `submission_state`
- `final_submission_draft`

旧字段只作为兼容兜底：

- `sds_sync`
- `request_draft`
- `preview_product`
- `submission`
- `final_draft`

### 写入原则

新代码不要再直接写旧字段。

应该只写：

- `SDSDesignResult`
- `DraftPayload`
- `PreviewPayload`
- `SubmissionState`
- `FinalSubmissionDraft`

兼容字段的镜像由语义归一化 helper 负责，不应在业务代码里手工维护。

## 字段对照

| 语义层 | 旧字段 | 新字段 | 推荐状态 |
|---|---|---|---|
| ListingKit result | `sds_sync` | `sds_design_result` | 新字段主用 |
| SHEIN package | `request_draft` | `draft_payload` | 新字段主用 |
| SHEIN package | `preview_product` | `preview_payload` | 新字段主用 |
| SHEIN package | `submission` | `submission_state` | 新字段主用 |
| SHEIN package | `final_draft` | `final_submission_draft` | 新字段主用 |

## 已暴露的新字段位置

### 任务结果

- `ListingKitResult`
  文件：[D:\code\task-processor\internal\listingkit\model_result.go](D:/code/task-processor/internal/listingkit/model_result.go:19)
- `StandardProductSnapshot`
  文件：[D:\code\task-processor\internal\listingkit\model_result.go](D:/code/task-processor/internal/listingkit/model_result.go:64)

### SHEIN package

- `shein.Package`
  文件：[D:\code\task-processor\internal\publishing\shein\model.go](D:/code/task-processor/internal/publishing/shein/model.go:18)

### 预览和导出

- `SheinPreviewPayload`
  文件：[D:\code\task-processor\internal\listingkit\preview_model.go](D:/code/task-processor/internal/listingkit/preview_model.go:119)
- `SheinExportPayload`
  文件：[D:\code\task-processor\internal\listingkit\export_model.go](D:/code/task-processor/internal/listingkit/export_model.go:63)

## 兼容层位置

下面这些 helper 是当前迁移的真实边界，调用方不需要自己发明同步规则：

- SHEIN package 语义归一化
  文件：[D:\code\task-processor\internal\publishing\shein\semantic_fields.go](D:/code/task-processor/internal/publishing/shein/semantic_fields.go:1)
- ListingKit result 语义归一化
  文件：[D:\code\task-processor\internal\listingkit\semantic_fields.go](D:/code/task-processor/internal/listingkit/semantic_fields.go:1)
- 预览/导出语义归一化
  文件：[D:\code\task-processor\internal\listingkit\preview_export_semantic_fields.go](D:/code/task-processor/internal/listingkit/preview_export_semantic_fields.go:1)

## 调用方迁移顺序

### 阶段 1：只读迁移

适用对象：

- 前端页面
- 内部 API client
- 导出消费方
- 任务查看/审计工具

动作：

1. 先改成优先读新字段。
2. 保留旧字段 fallback。
3. 不要求立刻修改历史数据。

推荐读取顺序：

1. `draft_payload`
2. `request_draft`

1. `preview_payload`
2. `preview_product`

1. `submission_state`
2. `submission`

1. `final_submission_draft`
2. `final_draft`

1. `sds_design_result`
2. `sds_sync`

### 阶段 2：写路径约束

适用对象：

- 新增业务逻辑
- 修订逻辑
- 提交链路
- 预览重建逻辑

动作：

1. 禁止新增代码直接写旧字段。
2. 统一调用语义 helper。
3. 如果需要重建 preview，统一通过 `SetPreviewPayload(...)`。

参考：

- helper 文件：[D:\code\task-processor\internal\publishing\shein\semantic_fields.go](D:/code/task-processor/internal/publishing/shein/semantic_fields.go:31)

### 阶段 3：消费方完成切换

进入这个阶段前，至少应满足：

- 前端已不再依赖旧字段
- 导出消费方已不再依赖旧字段
- 内部 API client 已优先读取新字段一段稳定时间
- 历史任务恢复、提交恢复、revision 回放没有再出现旧字段污染问题

### 阶段 4：兼容字段退场

只有在确认没有外部调用方依赖旧字段后，才做：

1. 从 JSON 输出中移除旧字段
2. 删除旧字段 deprecated 注释对应的字段
3. 删除兼容镜像逻辑
4. 删除旧字段相关测试

这一步不建议和当前业务迭代混做。

## 当前风险提示

### 1. 旧字段和新字段同时出现在 JSON 中

这是当前设计的有意行为，用于平滑迁移，不是重复数据 bug。

### 2. 兼容层必须语义优先

如果后续有人把兼容层改回“仅在 nil 时补值”，会重新引入旧字段陈旧状态覆盖新状态的问题。

特别敏感的字段：

- `preview_payload / preview_product`
- `submission_state / submission`
- `sds_design_result / sds_sync`

### 3. 不要手工双写业务代码

业务代码里零散写：

- `pkg.PreviewPayload = ...`
- `pkg.PreviewProduct = ...`

这种做法后续容易再次分叉。优先走 helper。

## 回归测试

迁移相关测试已覆盖：

- package JSON 同时包含新旧字段
- result JSON 同时包含新旧字段
- preview/export JSON 同时包含新旧字段
- 关键 submit phase 不会被旧字段陈旧值污染

相关测试文件：

- [D:\code\task-processor\internal\publishing\shein\semantic_fields_test.go](D:/code/task-processor/internal/publishing/shein/semantic_fields_test.go:1)
- [D:\code\task-processor\internal\listingkit\semantic_fields_test.go](D:/code/task-processor/internal/listingkit/semantic_fields_test.go:1)
- [D:\code\task-processor\internal\listingkit\preview_export_semantic_fields_test.go](D:/code/task-processor/internal/listingkit/preview_export_semantic_fields_test.go:1)

## 一句话结论

现在可以开始迁移调用方了，推荐策略是：

先读新字段，旧字段兜底；新代码只写新字段；等消费方稳定后，再安排旧字段退场。
