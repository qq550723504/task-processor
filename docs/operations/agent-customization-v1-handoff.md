# 智能体定制 V1 接线与使用

执行范围与权限以 #611 当前正文为准；领域合同见 [批准设计](../architecture/agent-customization-v1.md)。本功能由硕米平台专员人工处理，方案、费用与交付安排在线下确认。提交需求不收费，也不触发模型执行、支付或消息发送。

## 正常入口

在正常 Console 登录后选择企业：

- `/workbench/agents/custom`：介绍与定制流程。
- `/workbench/agents/custom/new`：提交业务需求、可选参考资料、联系人，并明确同意专员联系。
- `/workbench/agents/custom/progress`：查看当前企业需求、附件、方案与实际跟进记录。
- `/workbench/admin/agent-customization`：平台专员查看需求并记录处理进度。企业管理员角色不能代替平台权限。

需求、联系方式和附件是当前企业共享资料，具有 `workbench.agent.read` 的成员可读取；提交还需 `workbench.agent.use`。平台专员必须使用现有 verified platform-administrator 准入（配置的平台用户或平台角色）。这份说明不授予或修改任何人的权限。

专员每次可追加当前阶段记录，或前进一个阶段：提交需求 → 需求评估 → 方案确认 → 开发测试 → 交付使用。进入方案确认必须记录方案与报价；开始开发必须明确记录线下确认；每次更新必须有跟进说明。系统保存操作者、时间和版本；记录交付不会自动表示付款或用户验收。

## 共享 runtime Writer 接线

共享装配、正常启动、导航和 module catalog 由会话 `01a11f74-2565-78e0-9118-4fe1ff53e726` 独占维护。本模块提供以下入口，不能在 HTTP 请求或构造函数中安装 schema：

1. 显式安装步骤调用 `agentcustomizationpersistence.InstallSchema(ctx, installerDB)`，再调用 `GrantRuntime(ctx, installerDB, servingRole)`。installer 必须具备创建安全 schema owner/角色的权限；serving role 不得成为 schema owner，也不得继承破坏请求归属或不可变资料的权限。不要向 serving identity 授予 DDL 权限。
2. 使用已配置的正常 PostgreSQL serving pool：`store, err := agentcustomizationpersistence.New(servingDB)` → `service, err := agentcustomization.NewService(store)` → `handler, err := agentcustomizationhttp.NewHandler(service)`。
3. 将 `agentcustomizationhttp.Routes(handler)` 交给已有 descriptor registry/正常授权链。企业路径为 `/api/v1/agent-customization/requests`，平台路径为 `/api/v1/admin/agent-customization/requests`。使用其当前身份、LiveWrite、verified roles、超时和 permission 描述，不裸挂 Gin handler。
4. BFF 使用现有 `LISTINGKIT_SERVICE_API_BASE`（路径 `/api/v1`）、正常 Auth.js/Zitadel 会话、现有同源写配置与企业选择 cookie。专用 Next API 路由已经提供，不需向全局 workbench proxy 重复分发。
5. 更新 shared navigation 的 custom/new/progress 路径，并在实际模块已接入后更新 `module_catalog` 的 `agent-custom` 权限/可用性投影。专员入口不作为普通企业用户的快捷入口。依赖未接入时保持 unavailable；不能使用视觉 fixture 或假需求补齐可用状态。

Go import paths：`task-processor/internal/agentcustomization`、`task-processor/internal/agentcustomization/httpapi`、`task-processor/internal/integration/persistence/agentcustomization`（后者 package 名为 `agentcustomizationpersistence`）。本片不新增独立部署程序或第二套认证/runtime。

## 保存与失败处理

`agent_customization` schema 整体保存需求、附件 bytes、不可变事件和命令回执，须纳入正常 PostgreSQL 备份/恢复范围。正常停启保留数据库即可；本功能不提供删除 API，不要求旧业务迁移。

最多3个参考文件，每个2 MiB，支持 PDF、PNG、JPEG、UTF-8 纯文本/CSV。文件通过正常授权 BFF 下载，强制 attachment，不提供公开 URL 或在线执行预览。正文和联系字段按设计有界；JSON 读回最多4 MiB，覆盖合法20行页面及50条事件的转义文本。

结果未知时保留原身份、企业、key、载荷和版本，只能核实同一次操作。短命令保存在当前标签页 sessionStorage；大附件使用小型 session marker 与有界功能内存，普通导航后可恢复。若刷新/关闭页面导致大附件载荷丢失，marker 继续阻止新提交：先查看实际定制进度，并由平台核实原请求。不要清除待确认记录来重提不同内容。

专员遇到已知版本拒绝（412）时，读取最新进度并重新人工确认；系统不会自动覆盖另一位专员的更新。权限撤销或企业/身份改变不授权重放原命令。

## 验证交接

模块开发自检与隔离视觉检查不能替代正常 runtime 组合、专员真实处理或用户验收。实际组合须按前述正常入口验证企业提交及读取、平台评估到交付、附件下载和权限隔离；由用户或其指定独立验证者确认使用效果。当前 HEAD、CI、独立复核、组合/用户验收状态维护在主要 PR，不在本文维护滚动状态。
