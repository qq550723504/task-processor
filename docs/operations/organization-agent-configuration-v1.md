# 企业智能体配置 v1 交接

Design Basis：[已批准的 Agent D 合同](../architecture/organization-agent-configuration-v1.md)；执行 Issue #573。以下是安装和开发交接，未签发产品验收或部署批准。

## 用户入口

- 智能市场 → 智能体市场：`/workbench/agents/market`。
- 智能市场 → 我的智能体：`/workbench/agents/mine`，展示当前企业，非个人所有。
- 商品标题智能体配置：`/workbench/agents/mine/product.title.agent`。
- 使用入口：供应市场 → 1688 商品采集，打开已保存商品的采集结果，标题诊断与补全。

管理员显式启用后，成员确认精确模板版本、素材平台和是否引用企业知识，再生成标题建议。建议仍走已有人工审核与显式应用。模板默认只是预填；清空知识即不引用知识。更新模板生成新版本，不移动已保存的默认版本。

只有商品标题智能体 `product.title.agent / v1.0.0` 是本次可执行目录。图片执行、远端平台写入、自定义智能体及通用 Chat 未开放。`AVAILABLE` 表示当前配置接入，执行时仍重新确认权限、模型路由、额度和预算，不是 provider 健康检查或报价。

## 空安装与持久化

配置与既有 `product_agent_runs` 使用同一个 ProductAgentDB 和同一个 serving pool。配置 owner 位于 `agent_configuration` schema；企业启停、精确默认指针、模板版本、配置回执和 Start 快照保存在 PostgreSQL。业务请求和构造函数不执行 DDL，也不自动启用任何企业。

授权安装人员使用单独的 schema/operator 连接，运行：

```powershell
# 通过私密环境注入 AGENT_CONFIGURATION_SCHEMA_DSN；不要提交 DSN 或密码。
go run ./cmd/agent-configuration-schema-init --runtime-role <existing_serving_role>
```

该命令只创建本次新 schema 及非登录 `agent_configuration_owner`，给指定既有 serving role 最小表权限。Serving role 不得为 schema owner、superuser，或通过继承获得不可变表写权限。已存在的 foreign-owned schema 会被拒绝；不会改属或迁移旧数据。配置表无 DELETE API，版本与 Start 快照无 serving UPDATE/DELETE/TRUNCATE 权限。

原 Product Agent run schema 仍由原安装入口建立；本命令不创建、重写或迁移其记录。备份和恢复必须一起覆盖同库的 configuration/run 事实。缺少或不一致的引用会拒绝执行，不自动重建配置历史。

## 启动接线

使用既有 current-application 启动入口和配置文件（参见 [运行配置](../../internal/app/runtime/currentapplication/product_agent.go)）。

- 仅配置管理：保留 `productAgent` 节点，`enabled: false`，提供其 `database`。仅打开 RunDB，挂载配置 API；无需文本 provider、Review 或 Asset 连接。
- 标题执行：沿用既有 `enabled: true` 的 Product Agent 配置、当前组织准入名单、点数计价及数据库配置。配置模块复用相同 RunDB，且仍要求管理员显式启用当前企业。
- UI/BFF 沿用现有 ZITADEL 会话、企业选择 cookie 和 `LISTINGKIT_SERVICE_API_BASE`。不从浏览器接收企业、成员或凭据权威。

未接入执行时，市场和配置管理仍可使用；能力和本人最近任务如实显示不可用。Viewer 可读配置；Operator 可使用；Admin 可配置。配置权限不推导执行权限，执行仍需要原 Product 访问控制。

## 并发与失败处理

- 配置写操作必须携带 UUID `Idempotency-Key` 和强 `If-Match`；首次启用用 `If-None-Match: *`。缺少前置条件 428，版本失配 412。
- 同组织、同 actor、同 key 的相同已提交配置命令返回原回执，页面另行读取当前状态。变更载荷或路由返回冲突。
- 结果尚未确认时只重试原配置命令或核实原执行编号。页面不会自动换 key 发起新的模型调用。
- 停用阻止后续 Start/Resume；已领取的有限执行可完成。旧回执读取及已有 Review/Apply 保留。
- 新 Start 在同一 RunDB 事务内检查启用、epoch、模板未归档和当前硬上限，再领取既有 run；Prepared Start 经停用再启用仍拒绝。Resume 沿用冻结请求，检查当前启用及硬上限。
- 收紧运行上限时，先停止并排空旧执行进程，再声明新上限生效。未排空前旧进程仍使用旧代码上限；没有新增分布式配置协调器。
- 回滚不得在 Start/Resume 仍开放时移除准入 Guard，或只恢复 configuration/run 中的一侧。

## 开发证据及未执行项

使用任务隔离的 PostgreSQL、合成企业和 localhost 模型响应，验证了配置回执/CAS、不可变版本、默认归档限制、并发 Claim、停用 ABA、硬上限收紧、冻结配置重放，以及既有商品 → 标题建议 → 人工审核路径。Console/BFF 有身份/组织和严格载荷检查；页面测试覆盖迟到响应和 UNKNOWN 保留。桌面、手机及模板弹窗另做了 fixture 布局检查。

这些是开发自检。真实企业浏览器联调、付费模型、实际部署和用户验收未执行；运行证据、准确 HEAD、CI 和独立评审状态记录在主要 PR。依赖 #541 的实际 RUN-1 环节仍按该 Issue 状态处理，不以 fixture 代替。
