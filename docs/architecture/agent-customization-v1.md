# 智能体定制需求与人工进度 V1

Status: APPROVED / IMPLEMENTATION_READY (2026-10-09). Design Basis: Independent Architecture. Execution: #611, parent #137.

## 产品结果与依据

2026-10-09 用户确认：硕米平台专员人工处理，后台维护评估、方案报价和开发交付进度，方案与费用在线下确认。当前内部试用企业用户提交定制需求，查看已保存需求与真实进度。需求本身不是可执行智能体。

Figma Authority 为 `docs/product/final-ui-ia-authority.md`、文件 `tg48P46SSXl6TBy9lZwg63` 页面 `31:463`。本轮实读可见非归档介绍 `428:1913`、提交 `428:2443` 及浅色对应节点。表单包括需求名称、主要场景、方向、描述、可选资料、联系人、手机或微信、明确联系同意。介绍规定提交不收费，先评估再确认方案报价。`428:2973` 虽名为定制进度，内容仍是旧管理示例；本片按介绍五阶段和用户人工处理决定投影真实需求及记录，不采用示例成功率或运行数量。

Must：提交/刷新持久化；同意不默认勾选；当前企业隔离；专员可查看需求并记录评估、方案报价、线下确认、开发及交付；进度有真实时间与处理记录；参考资料可保存下载；结果未知只核实原操作；正常空/拒绝/失败/未接入状态。

Out of scope：支付或账本、自动联系/通知、服务商分配、SLA承诺、任意提示词/工具/插件/编排、训练/模型调用、自动启用或执行新智能体、旧数据兼容迁移。线下报价文本不构成现金支付/结算事实；交付记录不构成智能体执行准入。

## Owner 与调用路径

新增有界 owner `internal/agentcustomization`，拥有定制需求、联系同意、私有参考资料、人工处理事件及命令回执。它不导入 agentconfig/Agent/AI/Knowledge/Product/Review 的实现，也不复制上述事实。

| 合同 → 实现 → 注入 → 消费 | 职责 |
| --- | --- |
| agentcustomization.Repository → integration/persistence/agentcustomization → NewService → feature-local HTTP | 需求、附件、事件、回执的单库事务 |
| Service → agentcustomization/httpapi Routes → current application（#611串行接线） → Console/BFF | 有界传输、当前身份及正常读写 |
| 现有 authidentity/authz/httproute → 既有身份 middleware → Routes | 企业 read/use、verified platform administrator |

复用现有 Go PostgreSQL 驱动、Gin、严格 JSON decoder、请求 deadline、Auth.js、Zod、ConsolePage/ConsoleSection/ConsoleState/Button 和 tokens。不创建通用工单、上传、权限或状态机框架。

当前 agentconfig 模板只能配置已有平台和知识选择；它不适合保存人工定制服务需求。生态服务的订单依赖合格第三方 listing、provider、支付及结算，不适用于已确认的硕米线下承接，禁止把该需求伪装成第三方订单。

## 持久化、状态与事务

一个 PostgreSQL pool/schema `agent_customization`，与既有 owner 无跨库一致性。表 requests 保存 ID、OrganizationID、提交actor、输入、当前阶段、version、创建/更新时间；events 保存需求ID、version、处理actor、阶段、说明及时间；commands 保存准确 scope+actor+UUID key、fingerprint、原回执；attachments 保存需求ID、序号、名称、content-type、SHA256及bytea。

首次提交在一事务写需求、初始 SUBMITTED 事件、全部附件和回执；任何一步失败回滚。命令身份 `(scope-kind, organization-id, actor-id, key)`：同键同意图返回原回执，同键不同payload/操作/资源/expectedVersion冲突。首次需求ID为绑定完整命令身份的稳定UUID。不能失败后自动换key。

五阶段仅是人工事实：SUBMITTED 提交需求 → EVALUATING 需求评估 → PROPOSED 方案报价 → DEVELOPING 开发测试 → DELIVERED 交付使用。专员可在当前阶段追加记录，或推进一个阶段；不自动估进度百分比。推进 PROPOSED 需要方案/报价说明；推进 DEVELOPING 需要专员明确记录线下确认说明；推进 DELIVERED 需要交付说明。线下确认是专员陈述及时间/actor的记录，不表示系统验证了付款或用户验收。

专员命令带强 If-Match/version 和UUID key：锁原回执、再锁需求、检查当前version和转换、更新需求version、追加不可变事件和回执，同一事务提交。并发冲突需刷新并人工确认，不能自动覆盖。已提交重放先返回原回执，再由客户端读取当前投影；旧回执不覆盖较新的进度。没有自动后台恢复器：数据库事务回滚或原命令重放足够，无外部副作用。

请求/事件/回执不提供 DELETE；serving role 不持有 schema ownership，事件/附件/回执无 UPDATE/DELETE/TRUNCATE。需求只允许当前投影UPDATE。schema由独立安装动作完成，构造函数和请求不执行DDL。空安装无需求和示例。备份/恢复覆盖该schema整体；不与AgentRun同步。新版本schema演进属于当前系统维护，不承接旧业务数据。

## 身份、权限与安全范围

企业路径使用当前 authidentity + Effective Organization + LiveWrite 授权（包含读取，避免读取已撤权上下文）；读取复用 `agent.read`，提交复用 `agent.use`。需求是当前企业共享事实，UI明确“当前企业”；有读取权的企业成员可查看该企业需求和联系信息。上述权限不授予平台处理能力。

平台专员路径复用现有 `PermissionListingKitPlatformAdm`、`AuthPolicyCurrentIdentityWithVerifiedRoles`、OrganizationAccessPolicyNone；清除企业上下文。跨企业仅在该平台路径允许，enterprise admin角色或请求payload不能生成平台权。Handler仅从已验证context构造scope，拒绝payload中的actor/org/platform。每个读写都经过middleware，原回执重放也不绕过当前授权。平台UI入口为 `/workbench/admin/agent-customization`，不向普通企业用户显示平台快捷入口。

正文和联系字段是非可信文字，React按文字渲染，不使用HTML。不把联系资料、正文、文件内容或SQL/内部错误放入日志或错误响应。consentVersion由服务固定，consent=true必须显式提交，服务记录actor/time；不自动发送消息。

附件与提交在同一事务保存，单文件最多2MiB、最多3个，总共6MiB； JSON（base64）最大9MiB，metadata字段严格、名称不含路径/control、MIME由服务探测，只接受PDF/PNG/JPEG/UTF-8文本（CSV按文本），不接受HTML/SVG/office宏/压缩包。引用现有有界文件校验行为，但不依赖生态服务文件owner。bytea避免本片额外引入对象库和上传恢复协议，当前有界试用规模足够；容量优化只有实际需求触发。下载检查同一需求scope，强制attachment、nosniff、private no-store，不执行/预览上传内容。文件JSON读投影仅metadata，无base64正文。

名称<=120字符、场景<=240、描述<=10000、联系人<=80、联系方法<=160、事件说明<=5000；严格UTF-8、控制字符限制（长文本允许换行/tab）。列表每页20、最多50，稳定UUID cursor；详情事件按version分页每页50，不能无限返回全部历史。请求最多30s；读取GET拒绝未读body及未知query；write拒绝编码/重复key/未知字段/多JSON值/无consent。

Console key包括user/org/permission，在切换/撤权时abort且卸载，迟到响应丢弃。BFF复用正常身份token/同源写保护/current企业选择cookie；mutation冻结原user/org/payload/key，未知时只重试同一冻结命令。已知CAS/输入拒绝可刷新后新的人工确认。文件下载仍走当前session/BFF，无公开对象URL。

客户端复用现有有界pending命令保存行为。短命令在scope绑定的sessionStorage完整保存；大附件原载荷在有界功能内存保留，小marker保存原key/path/version/digest。导航可恢复；刷新丢失大载荷时保留marker与未确认阻断，不允许生成新意图。正常使用与该限制见[接线交接](../operations/agent-customization-v1-handoff.md)。读回JSON上限4MiB覆盖合法分页与Go文字转义，不改变附件2MiB上限。

## UI与接线边界

页面：`/workbench/agents/custom` 介绍；`/workbench/agents/custom/new` 提交；`/workbench/agents/custom/progress` 列表与需求详情；`/workbench/admin/agent-customization` 专员列表与处理。Figma介绍/表单用现有shell、左右主体栏及可响应式表单；主要场景允许用户文字说明，不从原型空下拉猜业务枚举。方向为Figma四项。实际进度用五阶段、记录时间/文字和附件，平台页面是用户明确后台处理决定的最小投影。

用户于2026-10-09明确同意由本执行会话串行接手智能体定制接线，替代此前交给 `01a11f74-2565-78e0-9118-4fe1ff53e726` 的本功能接线责任；其他模块仍按各自owner维护。本片在同一主要PR接入当前native启动装配、shared navigation及module catalog，不改变全局workbench proxy分发。

私有manifest可选 `agentCustomizationDatabase` 使用既有DatabaseConfig与受限 `agent_customization_runtime` serving role，最多4个连接，与其他事实owner保持独立数据库/pool。未配置不挂载本模块；配置错误或依赖缺失拒绝启动。显式 `cmd/agent-customization-schema-init` 消费已有InstallSchema/GrantRuntime，凭据由私有环境提供；构造函数和Serving不执行DDL。current application仅构造本域SQL适配器、Service和Handler，消费批准Routes并按精确descriptor检查企业/平台权限，不能裸挂或弱化授权。

Legacy decision: EXTRACT 合格Console与当前身份/传输/事务行为；RETIRE 占位与旧AI团队管理示例。无legacy wrapper/fallback/第二事实源。

## 验证与验收

必要TDD：阶段前置与同意/文件边界；PostgreSQL单事务、重放/冲突/CAS、跨企业/附件隔离及重连读回；HTTP descriptor企业与平台权限、payload身份拒绝、有界请求；BFF同源/身份scope/UNKNOWN；UI提交后refetch、专员推进、切企业迟到响应。测试用新隔离数据库与合成数据，无付费模型/真实联系或交易。

稳定候选运行相关Go/Vitest、typecheck/lint和必需CI；独立最终检查真实diff与调用路径。明确区分单模块自检、runtime组合、用户试用。用户验收、真实专员联系、共享部署、付费provider NOT_RUN。实现者不签发用户验收或上线许可。

## Architecture Review

第1轮独立只读 Architecture Review（`/root/architecture_review`，草案commit `29e7e3b17`）：IMPLEMENTATION_READY，无架构BLOCKER。以下为IMPLEMENTATION_TEST，当前Must未满足时阻止本片合并：首次不存在命令行的并发幂等需事务内准确身份串行化+唯一键（不能修改不可变回执）；实际middleware/BFF验证企业管理员不能跨企业，平台scope组织字段明确非NULL；现有WithRequestBodyReadTimeout终止慢速读体，UNKNOWN冻结原意图并refetch。共同scope的advisory锁仅用于串行化，不作为事实身份，唯一键仍包含完整scope/actor/key。

本Writer负责`module_catalog`中agent-custom已实现标识、既有read/use权限投影及custom/new/progress导航；真实权限和依赖失败仍由当前middleware/BFF/owner判定。已批准业务设计冻结，本轮仅消费既有合同与正常runtime，不重开全局设计。运行/产品验收仍按上述边界。
