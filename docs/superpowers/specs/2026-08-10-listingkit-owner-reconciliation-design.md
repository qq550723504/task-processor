# ListingKit Owner Reconciliation Design

## Goal

为 ListingKit 历史 owner 为空的数据提供一个可审计的收口流程：默认只读生成脱敏报告，只对能由已验证迁移元数据唯一确定的 ZITADEL subject 提供显式、可回滚意识的分批回填；不恢复运行时 legacy user 兼容。

## Context and invariants

- 生产预检已经发现 8,142,702 行空 owner；发布必须继续 fail closed，直到这些行被处理。
- `projections.user_metadata5` 中的 `yudao_user_id` + `yudao_tenant_id` 是旧用户到 canonical subject 的唯一受信迁移来源；组织映射必须同时匹配 `projections.org_metadata2`。
- 旧 `creator`、`created_by`、店铺 creator 和导入任务 creator 只能作为候选来源，不能单独证明当前 owner。
- 每行只能在所有可用候选解析为同一个 subject 时自动回填；无候选或候选冲突进入 unresolved 报告。
- ListingKit 原生表的空 `user_id` 没有旧用户映射来源，不能套用 legacy numeric 规则。
- 任何生产写入都必须由用户显式选择 `-Execute`；默认模式不执行 `UPDATE`、不创建表、不创建数据库、不修改 ZITADEL。

## Chosen approach

新增 PowerShell 编排脚本，复用仓库已有的 kubectl/psql 调用约定和 Yudao→ZITADEL 元数据迁移脚本，不引入第二套身份服务或运行时兼容层。

1. 从目标 PostgreSQL 只读读取 ZITADEL user/org metadata，构建进程内 legacy pair → subject 映射。
2. 对固定的 owner-scoped 表执行预定义、参数化的只读聚合查询，计算 direct creator、import-task creator、store creator 候选。
3. 输出只包含表名、tenant/owner 的短指纹、行数、候选状态和原因；绝不输出 raw tenant、旧用户 ID、subject、用户名、token 或 SQL body。
4. `-Execute` 仅更新 `owner_user_id`/`user_id` 目标列，并要求输入报告指纹与当前 dry-run 指纹一致；每个表按主键范围分批、事务执行，更新后重新计数。
5. unresolved 行永远不自动选租户成员；工具只生成处置清单，等待管理员提供明确的映射/归档策略。

## Safety and failure behavior

- 默认 `-DryRun`；`-Execute` 缺少显式报告确认、数据库目标或批次参数时立即失败。
- 所有 SQL 标识符来自脚本内固定白名单，值使用参数或安全 SQL literal 构造；禁止用户提供表名/列名。
- 数据库连接使用现有非创建连接路径；不执行迁移、DDL、`CREATE DATABASE` 或 ZITADEL 写操作。
- 任一表查询、报告校验、批次更新或复核失败时立即停止，并返回非零退出码；不产生“部分成功”结论。
- 报告包含运行时间、脚本版本、目标数据库逻辑名、表级计数和报告 fingerprint，不包含秘密或身份明文。

## Testing

- PowerShell 单元测试覆盖：默认 dry-run、显式 execute 门禁、metadata 映射唯一性、候选冲突、空候选、固定表白名单、脱敏输出、批次大小与报告 fingerprint mismatch。
- SQL 逻辑使用临时 PostgreSQL fixture 或现有 SQL-mock seam 验证：只读模式无 UPDATE，execute 只更新允许列，冲突行不更新，事务失败停止。
- 在生产目标仅运行 dry-run；报告与预检结果对齐后，另行授权 execute。回填完成后重新运行 ListingKit identity preflight，再授权发布。
