# 智能体定制 V1 接线与使用

执行范围与权限以 #611 当前正文为准；领域合同见 [人工流程设计](../architecture/agent-customization-v1.md) 与 [私有交付设计](../architecture/private-agent-delivery-v1.md)。本功能由硕米平台专员人工处理，方案、费用与交付安排在线下确认。提交需求不收费，也不触发模型执行、支付或消息发送。

私有交付操作：专员在已确认方案的开发/交付阶段填写交付说明，选择“交付平台草稿资料质检智能体 v2.0.0”并保存。系统只授权给该需求原企业，平台不能输入其他企业、定义或版本。仅记录人工 DELIVERED 不创建可用入口。客户在 `/workbench/agents/mine` 的“企业私有智能体”进入使用，也可从定制进度的实际交付链接进入 `/workbench/agents/mine/private/<deliveryId>`。选择自己的供应链批次、SHEIN 美国站店铺和待补全/已适配草稿 → 检查并保存报告 → 我的已保存报告 → 查看报告。当前企业的 agent.read/use 权限与实时授权分别控制读取和执行；来源读取还需 collection.read、supply.read，选择店铺需 store.read。平台处理权限不替代企业执行权。

质检复用 Listing 平台草稿保存时的确定性校验，包括必填项、规格、图片及平台资料问题；不重新查询平台规则或图片，不调用模型，不修改或上传商品。报告绑定不可变草稿编号、revision、来源、店铺、商品版本及内容/规则/资产 hash，完整保留保存时问题。它不是最新规则检测、图片当前可访问性检测或上传批准。新检查拒绝已变化、优化中、待审核或已上传的当前草稿；历史报告保持原版本。报告仅原操作人可读，每次摘要/详情/回放仍核对原来源的当前权限，其他企业及同企业其他成员不能读取。列表最多20条摘要，选择后单独读取完整报告；发送的只有草稿编号与预期版本，最多1KiB。响应丢失时保留原引用与请求编号，只能核实同一次检查。

已发生的隔离试用 `1.0.0` 交付和手工报告保持只读历史，不支持新手工检查或升级原交付。通过新需求和专员正常交付创建 `2.0.0`，不迁移或重写原事实。正常停启保留数据库和命名卷。原引用/key 保存在当前标签页 sessionStorage；关闭标签页后的恢复不承诺，先查看已保存报告。

平台草稿质检还需 current-application 已启用现有 SupplyChain 模块及其正常 Product/Listing、店铺和来源读取依赖，并已有当前操作人保存的 SHEIN 美国站平台草稿。只安装定制数据库不能产生可选草稿；依赖未就绪时页面明确不可用，不创建模拟可用数据或报告。初始化/启用这些既有 owner 沿用各自文档和环境授权；本功能不取得其他 owner 的数据库池，不新增 provider 或 schema。

当前系统显式 schema 演进：在该独立数据库的获准维护窗口，使用当前 schema-init 增加 deliveries/quality_runs 并重授精确 serving 权限；不重建数据库/身份/示例，不删除已有需求或命名卷。停服后安装，安装成功再启动匹配版本。Serving启动检查新表，缺失时拒绝启动。整体备份覆盖需求、交付、事件、回执、附件及报告；没有后台 provider 或 pending-run 恢复器。

## 正常入口

在正常 Console 登录后选择企业：

- `/workbench/agents/custom`：介绍与定制流程。
- `/workbench/agents/custom/new`：提交业务需求、可选参考资料、联系人，并明确同意专员联系。
- `/workbench/agents/custom/progress`：查看当前企业需求、附件、方案与实际跟进记录。
- `/workbench/admin/agent-customization`：平台专员查看需求并记录处理进度。企业管理员角色不能代替平台权限。

需求、联系方式和附件是当前企业共享资料，具有 `workbench.agent.read` 的成员可读取；提交还需 `workbench.agent.use`。平台专员必须使用现有 verified platform-administrator 准入（配置的平台用户或平台角色）。这份说明不授予或修改任何人的权限。

专员每次可追加当前阶段记录，或前进一个阶段：提交需求 → 需求评估 → 方案确认 → 开发测试 → 交付使用。进入方案确认必须记录方案与报价；开始开发必须明确记录线下确认；每次更新必须有跟进说明。系统保存操作者、时间和版本；记录交付不会自动表示付款或用户验收。

## 正常 runtime 启用

智能体定制已经接入 `cmd/current-application` 的可选原生装配、Console 导航及 module catalog。用户将本功能接线交给 #611 串行完成；其他模块的接入仍由原 owner 维护。本功能使用已有正常认证/runtime，不新增独立 serving 程序。

先由获准的环境维护者准备一个空的独立 PostgreSQL 数据库及 `agent_customization_runtime` 登录角色。角色不得是 superuser、数据库/角色创建者、schema owner 或拥有 bypass-RLS 权限，不得继承 schema owner 或不可变资料的修改权限。安装者与运行者凭据必须分离。显式安装使用已有 `InstallSchema` / `GrantRuntime`，只创建本域 schema 和权限，不创建示例需求：

```powershell
# 在私有环境中设置安装者连接，避免写入仓库或终端记录。
$env:AGENT_CUSTOMIZATION_SCHEMA_DSN = '<private installer DSN>'
try {
  go run ./cmd/agent-customization-schema-init --runtime-role agent_customization_runtime
  if ($LASTEXITCODE -ne 0) { throw 'agent customization initialization failed' }
} finally {
  Remove-Item Env:AGENT_CUSTOMIZATION_SCHEMA_DSN
}
```

在现有完整的私有 current-application JSON manifest 中增加以下字段；这只是字段示例，不是可独立使用的完整 manifest：

```json
"agentCustomizationDatabase": {
  "host": "127.0.0.1",
  "port": 5432,
  "user": "agent_customization_runtime",
  "password": "<private serving password>",
  "database": "agent_customization",
  "maxConnections": 4
}
```

数据库必须与 source-account 及其他 owner 数据库分离，连接数最多4。正常启动沿用现有完整 identity/source-account 配置：

```powershell
go run ./cmd/current-application -config '<absolute private manifest path>'
```

未配置该字段时不挂载定制 API；配置但数据库不可达、缺表、有效权限不足或过大时启动失败。Serving 和 HTTP 请求都不执行 DDL 或自动修复权限。启动检查通过后，SQL adapter → Service → Handler 的批准 Routes 经正常 descriptor registry 挂载；企业路径为 `/api/v1/agent-customization/requests`，平台路径为 `/api/v1/admin/agent-customization/requests`。既有当前身份、LiveWrite、verified platform roles、超时及 permission 检查保持生效。

Console BFF 使用现有 `LISTINGKIT_SERVICE_API_BASE`（路径 `/api/v1`）、正常 Auth.js/Zitadel 会话、同源写配置与企业选择 cookie。`agent-custom` catalog 表示软件模块已实现，并投影既有 read/use 权限；实际权限和依赖可用性仍由正常 middleware/BFF/owner 判断。专员入口不出现在普通企业菜单中。

RUN-1 仍是原有身份/路由隔离环境，不因本功能自动增加 schema、角色或可选模块。共享或生产环境启用须按对应环境授权处理。

容器构建复用 `deployments/docker/Dockerfile.account-compose` 的 `schema-init` target，其中提供 `/usr/local/bin/agent-customization-schema-init`。Compose 的默认安装动作不会自动调用本命令；获准安装独立空库后，使用私有环境文件及该显式 entrypoint 按上述相同参数执行。

## 保存与失败处理

`agent_customization` schema 整体保存需求、附件 bytes、不可变事件和命令回执，须纳入正常 PostgreSQL 备份/恢复范围。正常停启保留数据库即可；本功能不提供删除 API，不要求旧业务迁移。

最多3个参考文件，每个2 MiB，支持 PDF、PNG、JPEG、UTF-8 纯文本/CSV。文件通过正常授权 BFF 下载，强制 attachment，不提供公开 URL 或在线执行预览。正文和联系字段按设计有界；JSON 读回最多4 MiB，覆盖合法20行页面及50条事件的转义文本。

结果未知时保留原身份、企业、key、载荷和版本，只能核实同一次操作。短命令保存在当前标签页 sessionStorage；大附件使用小型 session marker 与有界功能内存，普通导航后可恢复。若刷新当前标签页导致大附件载荷丢失，marker 继续阻止新提交：先查看实际定制进度，并由平台核实原请求。不要清除待确认记录来重提不同内容。

待确认记录只保证当前标签页会话；关闭标签页后不能保证小载荷或marker保留，本版未提供跨标签页/跨会话恢复。原操作尚未确认时请保留当前标签页与浏览器数据；新开页面不代表已核实原请求，应先查看实际进度并由平台核实。

专员遇到已知版本拒绝（412）时，读取最新进度并重新人工确认；系统不会自动覆盖另一位专员的更新。权限撤销或企业/身份改变不授权重放原命令。

## 验证交接

### 无官方凭据的隔离离线试用

仅用于已获准的独立本机项目，依据 [离线试用准入](../architecture/private-draft-offline-trial.md)。在私有 manifest 明确配置 `privateDraftTrial`，字段为 `acknowledgment: ISOLATED_OFFLINE_DRAFT_TRIAL_ONLY`、`organizationId`、`actorId`、当前实际 `memberId`、`storeId`。保留正常本机 HTTPS 身份、当前 Product/Store/Collection/customization owner 和 live permissions；禁止同时配置完整 SupplyChain、official applications 或 Agent/provider 执行。缺省关闭，配置冲突拒绝启动。

显式通过现有 native owner 准备测试店铺和一个 TargetRecord；店铺名称以“离线测试”开头，保持 pending activation/disconnected。准备步骤不进入启动、HTTP 或 worker，不直接写草稿/issues/报告。试用 Product role 使用现有 collections+SupplyChain grants；只挂七个 GET，不开放草稿保存、上传、操作或规则调用，Asset/Temporal 不参与试用读取。

UI server 环境 `LISTINGKIT_PRIVATE_DRAFT_TRIAL_ENABLED=true` 提供醒目说明；`LISTINGKIT_SUPPLY_CHAIN_ENABLED=false` 继续关闭完整供应链。环境开关不授权请求。实际报告由服务端检查测试 binding 后保存 `offlineTrial` 观察标记，摘要/详情与刷新读回保留测试规则说明。

正常登录后：我的智能体 → 已交付2.0.0私有质检 → 选择测试批次/离线测试店铺/待补全草稿 → 检查并保存 → 查看报告。正常 stop/start 保留所有命名卷及数据；不要重新初始化定制库/身份，不执行 down -v。内部受控运行检查不等于实际 SHEIN 接入或用户验收。

配置生命周期、正常路由授权及隔离 PostgreSQL 组合检查是开发自检，使用合成身份与需求，不能替代真实登录、专员处理或用户验收。获准启用环境后，按前述正常入口验证企业提交及读取、平台评估到交付、附件下载和权限隔离；由用户或其指定独立验证者确认使用效果。当前 HEAD、CI、独立复核、组合/用户验收状态维护在主要 PR，不在本文维护滚动状态。
