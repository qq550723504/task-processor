# 指定企业平台草稿质检：隔离离线试用装配

Status: IMPLEMENTATION_READY. Independent review /root/architecture_review bound to design 0657282dcf541415695d47adbb621e5dbb090ec4, no BLOCKER. Issue #611, same Delivery Batch / PR #617. Listed IMPLEMENTATION_TEST items are required before completion.

## Product outcome and authority

2026-10-10 用户明确同意：没有 SHEIN 应用和连接店铺时，先在当前独立本机实例完成“选择实际保存的平台草稿 → 检查 → 保存报告 → 刷新读回”。这是明确标记的离线试用；测试店铺、测试规则和报告不得呈现为真实平台连接、最新官方规则或上传许可。原本地依赖准备已完成，旧“等待实际凭据才能开展完整浏览器路径”由本阶段决定替代。真实平台接入及用户验收仍 NOT_RUN。

沿用 `private-agent-delivery-v1.md` 2.0.0 产品/权限/报告合同、`my-supply-chain-shein-v1.md` 的 native TargetRecord owner，以及 `final-ui-ia-authority.md` / Figma tg48P46SSXl6TBy9lZwg63 My Agents 4703:429。没有新增菜单、页面、人工审核步骤。现有私有详情增加离线说明；持久化报告记录测试规则来源，历史读回也显示说明。

Must：正常身份登录、当前企业和 actor/source 授权；限定指定测试 scope；原生不可变 TargetRecord 和保存时 goods 校验；精确 record/revision 幂等、真实 customization 报告保存/刷新/保留停启；明确测试来源。零模型、计费、发布或远端调用。

Out of scope：真实应用/连接/官方规则、用户编辑测试草稿、完整供应链开放、批量生成、自动恢复平台、通用 fixture/验收工具、共享/生产部署、旧数据迁移、删除、合并和关单。

## Current obstruction and minimal change

目前只读草稿 inspector 随完整 SupplyChain builder 构造，因而依赖官方应用、Asset pool、Temporal worker 和上传执行对象。这让无外部凭据的只读试用无法完成。抽取现有 native 草稿读取装配，供正式 SupplyChain 和明确 opt-in 的离线试用共用；不建立第二套草稿业务实现或事实源。`Application.ReadTarget` 使用已有 Records owner 读取 head，并保持同一 scope/source/current permission 核对，不为了读取构造可写 TargetService。

## Admission, permissions and injection

新增可选 privateDraftTrial 配置：固定 acknowledgment `ISOLATED_OFFLINE_DRAFT_TRIAL_ONLY`，一个完整 Organization/Actor/Member scope 和测试 storeId。只有本机 HTTPS 身份、loopback listener、当前本地 Product/Store owner、collections 和 customization DB 已装配时允许启动。与 SupplyChain、LocalTrial、official applications、ImageAgent、ProductAgent、BrowserCollector 不兼容；配置不完整或冲突拒绝启动，绝不降级正式 SupplyChain。没有新的凭据、数据库、schema、Temporal worker、provider 或静默 seed。

contract → implementation → injection → consumer：private manifest explicit trial → currentapplication feature → current HTTP composition 的共享 native draft read core → 正常已验证身份/capability binder + 既有 live collection/preparation/Store 授权 → native preparation/source/record/stage GET 与原私有 inspector → customization immutable report → 正常 BFF/private detail。

读取 core 使用当前 native Product collections、preparation repository/source selector、record repository、Review projection 和 official receipt repository。完整 SupplyChain 再装配现有写服务、规则、Asset 和 worker。离线仅安装选择所需的既有 GET 子集：list、preparation、sources、source、stages、target、record。所有其他 Supply route（包括 POST、transfer、rules、upload、optimization）不挂载；route admission 精确区分这7个 GET 和完整模式。

离线 authorizer 在现有 ContextAuthorizer 成功后进一步要求 scope 与配置完全一致。当前 StoreMember repository 仍核对 store.read/成员访问、实际 active SHEIN 店铺，并要求名称离线标记、无 connectionRef。离线 stage/publication adapter 只提供预先约定的测试 merchant binding；所有规则查询方法拒绝，不创建 official client。测试 binding 的 ApplicationID 明确为 offline-private-draft-trial，无真实 app 配置/密钥/connection row，不能进入真实上传 owner。首执和历史读取还核对保存 record 的测试 binding/目标店铺；不能用本模式读取混入的真实草稿。

Product runtime 继续使用既有 source_acquisition_runtime 的 collections+SupplyChain 当前 capability grants，serving 无 DDL。离线进程没有相应 mutation routes 或 worker。Store 使用既有受限 role/member repository，customization 使用独立原 pool。试用只读不需要额外 Asset serving pool；已准备的 Asset/Temporal 保留闲置。

## Fixture preparation and durable facts

仅在已获准的 retained 本机项目，显式一次准备：使用已通过正常 UI 保存的虚构商品及批次、原生 preparation.Transfer 和 TargetService.Create，保存一个待补全草稿。固定 operation keys 和载荷支持原生 replay；失败核实同 key，不直接 INSERT 伪造 record/issues/报告。店铺使用现有 Store aggregate/repository 创建 active、pending activation、disconnected 的“离线测试”记录，不购买服务、不创建 merchant connection、不伪造已连接成功。

准备源码/私密参数放本机 handoff，明确 maintenance fixture，不装配到 HTTP/启动，不扩展通用 runner。只有命名 synthetic scope；准备时核实该 scope 来源于本实例已有当前 membership 和已保存 batch。维护身份不进入 serving 授权链。跨 Product/Store 的准备没有业务外部副作用；各 owner 原幂等 key 和 receipt 独立保存，可核实后继续，不假装跨库原子。运行前记录原事实数量/备份；不重新初始化定制库或身份、不删除卷/历史。

canonical owners 不变：Catalog 商品，collection/preparation 来源，TargetRecord 草稿和保存时校验，Store 店铺，customization 报告。`DraftBinding` 增加可选 offlineTrial 布尔观察标记（由 inspector 从准入模式与测试 binding 推导，不接受浏览器提交）。同现有报告 JSON 保存，无新表/迁移。旧字段缺失视为非离线观察，旧1.0.0只读不变。客户端严格解码并在摘要/详情显示测试规则；全局详情说明由 server-only 环境开关提供，不能授权请求。

## Invariants, failures and review classification

原 normal identity、Org/Actor/Member、live collection/supply/store 权限、source archive、exact revision/head、pending review、publication exclusion、1 KiB command、20s读请求、报告4 MiB、事务/幂等/UNKNOWN 合同不变。试用额外 scope 和 disconnected Store 检查不放宽已有授权。数据库/Store/规则绑定不一致拒绝读取和写报告。撤权、跨企业、暂时读失败、响应丢失仍由原 owner 和 frozen key 处理；离线观察不成为上传锁/许可。

Legacy decision: EXTRACT 当前 native 读取/显式 owner 创建；RETIRE 对读取不必要的完整执行装配依赖。禁止消费旧 LocalTrial/Listing Workspace、fallback、双读/双写或第二规则引擎。

必要 IMPLEMENTATION_TEST：配置缺省/冲突/非本机拒绝，精确7个 GET admission 与 mutation 未挂载，scope/Store/binding 拒绝，真实 native PG 草稿与报告持久化，正常 Edge 选择/保存/刷新，实际保留停启读回。仅变更风险相关，不建设额外故障矩阵。跨租户/错误授权/不可完成当前 happy path 为 BLOCKER；其他建议按当前 Must 分类。独立高风险准入及最终交付检查各一次，修复仅复核增量。

内部试用 PASS 只代表受控开发检查；真实平台、用户验收、生产可用、merge 仍分别保持 NOT_RUN / NOT_AUTHORIZED。
