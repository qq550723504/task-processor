# 企业空间操作记录 v1

本页是 #412 对 #37 Audit Projection 的首个产品消费切片。入口为
`/workbench/account/organization/audit`，复用 Account Shell；不提供管理动作。

## 事实来源与范围

| 当前来源 | v1 决策 | 原因 |
| --- | --- | --- |
| `sourceaccountregistry` / `source_account_operations` | 纳入 | 现有事务与业务变更一起写入不可变成功回执，可按企业查询 |
| `storecenter` / `workbench_store_audit_logs` | 暂不纳入 | 独立领域审计，留待后续明确消费合同 |
| `workbenchcontext` identity / organization switch logger | 不纳入 | structured logger 没有稳定查询端口，不能伪造持久记录 |
| Product revision、Listing submission、Tool/AI ledger | 暂不纳入 | 不属于首片覆盖范围，不重建领域状态机 |

只展示源账号 `register / enable / disable` 的已提交成功回执；成员、权限、续费、失败尝试、未确定结果和安全日志均未覆盖。UI 必须明确此范围，不能把空列表解释为“企业从未有过任何操作”。

## 调用与权限

浏览器专属 client → `/api/account/audit` Auth.js BFF →
`GET /api/v1/account/audit` → `internal/app/accountaudit` 传输投影 →
`sourceaccountregistry.HistoryService` → 现有 persistence repository 的只读查询。

每页通过 verified identity、显式 Effective Organization 和现有
`source_account.read` 权限判定。HTTP 使用现有 LiveWrite resolution 策略获取实时 grants；这是实时授权选择，不授予写权限。viewer/operator/admin 继续使用现有权限矩阵。
HistoryService 复用当前源账号 Service 的授权能力；不新增 IAM。

BFF 只转发服务器 access token，验证预期 user/org 与选中企业 cookie，
不信任浏览器提供的 bearer 或 requested-org。路由使用 `force-dynamic`，响应 no-store。
企业切换、身份变化、退出、重新授权和重新读取时清除旧记录；迟到响应不得写入新企业视图。

## Allowlist 与分页

记录只含固定 event type、actor subject reference、time、business object type/reference、
operation、`succeeded` 和 source-account version relation。actor 是身份引用，不冒充人名。
不返回自由文本、display name、idempotency key、fingerprint、完整账号状态或 provider 原始信息。

顺序为 `(created_at DESC, account_id DESC, resulting_version DESC)`。
时间是原 owner 固定的业务操作时间，不是数据库 commit time，也不是全局因果序。
当前 owner 为登记生成 version 1，后续锁定单账号递增 version；重放不新增回执，回滚不产生可见回执。

Cursor 为严格 canonical base64url JSON：org、微秒时间、UUIDv7 account、十进制字符串 version。
version 保留 int64 精度，禁止经 JavaScript number 转换；不同企业不能复用 cursor。
默认每页 20，允许 1–100；查询 limit+1，不返回总数或伪造指标。
这是实时 keyset 浏览，不提供跨页冻结快照：翻页期间新增记录应通过刷新首页读取。

## 资源与失败边界

复用当前 SourceAccount DB pool，不增表、索引、迁移、runtime grant 或 config。
实际默认装配增加一个 GET，custom factory 为 nil 保持原 route set；提供的 factory 失败或返回 nil 时关闭装配。

Go 查询与路由最多 10 秒，BFF 整体（包括认证）15 秒；请求体拒绝。
响应最多 128 KiB；错误仅返回现有安全 code。授权失败、查询失败、超时和无记录必须区别表达。

现有 organization PK 前缀、keyset 和 LIMIT 不保证高基数扫描量恒定；不能据此宣称常数复杂度。
保留 deadline/cancellation，不在本片擅自增加 schema/index。规模样本只说明该测试数据量的表现。

## 隔离验证入口

- `go test ./internal/app/httpapi ./internal/app/accountaudit ./internal/sourceaccountregistry ./internal/integration/persistence/sourceaccountregistry`
- `go test -tags=integration ./internal/integration/persistence/sourceaccountregistry -run TestAccountAuditCommittedHistoryPostgres -count=1 -v`
- `go test -tags=integration ./internal/app/httpapi -run TestAccountAuditBrowserFixture -count=1 -v`
- 在 `web/listingkit-ui` 运行 `node scripts/account-audit-fixture.mjs --serve`。

fixture 创建任务独有 PostgreSQL 和进程，以现有业务 Service 生成记录，以实际
NewCurrentApplication、Next/Auth.js/BFF/client 查询；启动后输出临时证据目录及浏览器 bootstrap URL。
外部 identity HTTP 和加密会话签发是明确的 synthetic boundary，不是官方 IAM 或真实环境验收。
在证据目录写 `stop-fixture` 结束；脚本停止自有进程，Go 验证资源与回执事实没有变化后清理容器，输出 cleanup.json。
不访问共享数据库、IAM 或付费 provider。滚动 SHA、CI、review 和授权证据只维护在 PR。
