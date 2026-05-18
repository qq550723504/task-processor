# Updater Third-Party Validation

## Goal

在不改变当前上线行为的前提下，为后续评估第三方自更新库建立一个明确的验证边界。

本阶段不替换现有 updater 实现，只通过 `internal/app/updater/autoupdate_adapter.go` 引入适配层，保证现有版本查询、校验、分阶段替换和重启逻辑继续可用。

## Current Behavior That Must Be Preserved

以下行为在引入第三方方案前必须保持一致：

1. 通过当前远程版本元数据接口拉取 `VersionInfo`
2. 使用下载地址和 SHA-256 做完整性校验
3. 下载失败时保留现有重试语义
4. 更新失败时记录 `update-error.log`
5. 重启前创建更新标记，避免更新循环
6. Windows 上继续采用延迟替换策略，避免运行中直接覆盖可执行文件
7. Windows 上替换完成后仍能从正确工作目录重启新版本

## Candidate Libraries

### `github.com/sanbornm/go-selfupdate`

适合验证的原因：

- Go 生态里较常见的自更新实现
- 覆盖版本检查、下载和应用更新的基础能力

待验证问题：

- 是否能直接适配当前自定义版本元数据结构
- 是否能保留现有 Windows 延迟替换流程，而不是假设立即覆盖

### `github.com/creativeprojects/go-selfupdate`

适合验证的原因：

- 提供更完整的 release/update 工作流封装
- 社区中常被用于自更新场景评估

待验证问题：

- 是否要求 GitHub Releases 或特定分发模型
- 是否允许继续使用当前私有下载源和校验策略

## Validation Criteria

候选库只有在全部满足以下条件时，才允许替换默认 adapter 实现：

1. **元数据兼容性**  
   能消费当前版本源，或只需很薄的映射层就能消费当前 `VersionInfo` 结构。

2. **校验边界完整**  
   在可执行文件替换前完成 SHA-256 或等效强校验，且失败时不会继续安装。

3. **Windows 替换安全性**  
   不要求直接覆盖运行中的 `.exe`；可以保留或兼容当前 `.new` -> 延迟替换 -> 重启 的模式。

4. **失败可观测性**  
   下载、校验、替换、重启前任一步骤失败时，调用侧仍能记录明确错误并保留 `update-error.log` 语义。

5. **重启控制权**  
   不强制接管整个进程生命周期；调用侧仍能决定何时标记已更新、何时延迟、何时重启。

6. **分发源兼容性**  
   不依赖 GitHub Releases、特定签名服务或固定目录布局；可以对接当前私有发布方式。

7. **回滚与残留文件风险可控**  
   替换失败、重启失败或 Windows 中途中断时，不会让当前程序不可启动，也不会破坏现有 `.old`/`.new` 处置方式。

## Recommended Validation Process

1. 保持 `defaultAutoUpdateAdapter` 作为基线实现
2. 为候选库新增并行 adapter 原型，不替换生产路径
3. 在 Windows 环境验证：
   - 下载成功并通过校验
   - 下载成功但校验失败
   - 程序运行中触发更新
   - 延迟替换后重启
   - 重启前异常退出
4. 在非 Windows 环境验证：
   - 正常替换
   - 失败回滚/残留文件行为
5. 对比现有实现与候选库在日志、错误、工作目录和产物文件上的差异

## Decision Rule

- 如果候选库全部通过验证条件，可以新增一个 third-party adapter 并在后续任务中切换默认实现。
- 如果任何一项关键条件不满足，保留当前默认 adapter，继续使用现有 updater 组件。
