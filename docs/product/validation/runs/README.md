# ListingKit 真实接口验收记录

本目录用于保存每轮真实接口联调和端到端验收报告。

## 命名规范

```text
YYYY-MM-DD-<platform>-<source>-<scenario>.md
```

示例：

```text
2026-06-16-shein-sds-save-draft-success.md
2026-06-16-shein-sds-publish-failed-recovery.md
2026-06-16-shein-1688-readiness-blocked.md
```

## 使用方式

1. 复制 [`../listingkit-real-api-validation-report-template.md`](../listingkit-real-api-validation-report-template.md)。
2. 填写真实 task_id、输入、状态流转、workspace payload、readiness、提交阶段、失败恢复和最终结论。
3. 每轮联调后必须更新 unknown state / unknown blocker 待关闭项。
4. 如果报告结论是 `blocked` 或 `fail`，必须在报告里写清关闭标准。

## 最低验收集

每个主要版本至少保留：

- 1 条 SHEIN SDS 保存草稿成功路径。
- 1 条 SHEIN SDS 发布成功路径。
- 1 条可恢复失败路径。
- 1 条 readiness blocked 修复路径。

## 不能忽略的问题

以下问题不能只记录为备注，必须进入待关闭清单：

- 未知任务状态。
- 未知 readiness blocker key。
- 空错误响应。
- UI 无下一步动作。
- 重复点击导致重复远端提交。
- 保存草稿 / 发布状态和 SHEIN 远端结果不一致。
