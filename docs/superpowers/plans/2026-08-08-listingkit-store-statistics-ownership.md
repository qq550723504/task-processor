# ListingKit 店铺统计权限范围修复实施计划

> **For the executing agent:** REQUIRED SUB-SKILL: Use `superpowers:executing-plans` to implement this plan task-by-task.

**目标：** 修正店铺统计的租户/平台数据范围，并保持历史无归属店铺不被错误分配。

**方案：** 将现有统计 handler 固定为租户范围，新增平台全局统计 handler 与路由；前端按 `tenant`/`platform` 页面变体选择对应接口；通过 handler、路由和 API 调用测试锁定范围。

## 任务 1：锁定后端范围行为

**文件：** `internal/listingadmin/store_statistics_handler_test.go`（若测试文件位置不同，以现有 handler 测试结构为准）

- 为租户统计增加平台角色场景，断言租户条件仍保留。
- 为平台统计增加全局场景，断言租户和 owner 条件被清除。
- 先运行定向测试，确认新断言失败。

## 任务 2：实现后端统计接口

**文件：** `internal/listingadmin/store_statistics_handler.go`、路由注册文件及其测试

- 抽取共享的统计执行逻辑，避免租户和平台 handler 复制指标计算。
- 让现有租户 handler 始终使用租户 scope。
- 增加平台 handler，仅允许平台访问并使用全局 scope。
- 注册 `/api/v1/listing-kits/platform/store-statistics`。
- 保持 owner 过滤安全策略，不自动放开空 owner 店铺。

## 任务 3：锁定并实现前端调用范围

**文件：** `web/listingkit-ui/src/lib/api/admin-stores.ts`、`web/listingkit-ui/src/pages/StoreStatisticsAdminPage.*` 及现有测试位置

- 增加平台统计 API 方法。
- `tenant` 变体调用租户接口，`platform` 变体调用平台接口。
- 增加调用路径回归测试；若当前测试基础设施不覆盖该页面，则至少运行类型检查/构建并验证调用点。

## 任务 4：验证与交付检查

- 运行 `go test ./internal/listingadmin ./internal/listingkit/httpapi`。
- 运行 ListingKit UI 相关的 lint/typecheck/test 命令，以仓库现有脚本为准。
- 检查 `git diff`、`git status`，确认没有生产数据写入、无无关文件改动。
