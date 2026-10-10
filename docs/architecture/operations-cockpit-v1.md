# 运营驾驶舱首版：人工经营事实与有据可查的经营投影

Refs [执行 Issue #627](https://github.com/qq550723504/task-processor/issues/627)、#137。

- Design Basis: **Independent Architecture**。
- Admission Status: **NOT_READY / 第1轮独立评审待执行**。正式生产代码尚未开工。
- 调查基线：`main 6db1bbc5529433d37b49708828b7d2b621bf9bc2`（含 #615）。
- 唯一 Writer：chat `01a11f6e-6586-7e13-ba3a-1758a7123e90`，`codex/operations-cockpit`。

## 1. 用户结果与决定

当前硕米内部试用的获授权运营人员，在当前企业选择自己有权限的店铺，录入各期经营收入与成本，设置利润目标，查看同周期店铺矩阵、真实经营预警及相应处理建议。四个菜单同一设计批、一个主要 PR。

用户于本线程 2026-10-10 明确：

1. **首版由运营人员录入店铺每期收入和各项成本，系统计算利润；平台财务数据后续再接。**
2. **经营建议先提供有数据依据的规则建议，跳转已有页面由人处理；AI方案后续接入。**

这些决定替代 Figma 中首版自动财务分析、AI生成方案/任务的开放语义；Figma仍决定页面归属、名称、布局和有效交互。原型示例不成为经营事实。

Must：目标配置/启用与版本记录真实持久化；经营数据可保存、修正和重读；同一周期比较2–5家当前获授权店铺；缺失数据不能当0、缺数据不能显示健康；预警/建议必须可读判断依据和数据时间；既有订单/库存观察保留其自己的范围和覆盖；跨企业和未授权店铺不可读写。

Out of Scope：自动抓取财务、汇率换算、平台资金结算、Commercial账本变更、广告/转化/处罚数据抓取、低库存安全量或预测损失的猜测、AI/provider调用、任务创建/自动经营操作、通用BI/规则/告警/通知平台、定时任务、ERP、旧数据迁移或双事实源。没有自动监测或告警生命周期，因此不显示“实时监测中”“处理中/已恢复”的虚构状态。

当前Threat Model：复用CurrentIdentity、Effective Organization、live member/module权限和Store member grant；经营数据正确归属、有界金额/输入/响应、防跨店铺泄漏和撤权后读回；本批无凭据、外部mutation或资金操作。不自行扩大为额外IAM或通用审计框架。未声明新的Accepted Risk。

## 2. Authority与当前根因

引用 [最终UI/IA](../product/final-ui-ia-authority.md)、[Greenfield](../product/greenfield-no-legacy-migration.md)、[Store](store-center-current-application-v1.md)、[平台观察](store-center-platform-observations-v1.md)。

2026-10-10实查 Figma `tg48P46SSXl6TBy9lZwg63` / `31:463` 当前可见非归档节点：

| 页面 | 深色 / 浅色 | 首版消费 |
| --- | --- | --- |
| 目标管理 | `1413:359` / `1413:628` | 生效配置、目标/评判/人工处理边界、真实历史版本 |
| 设置经营目标 | `1423:365` / `1423:591` | 店铺多选、日/周/月、日期、利润金额、可选利润率、预览、保存并启用 |
| 店铺矩阵 | `427:851` / `427:1051` | 统一周期/筛选、真实统计、表格、2–5店比较 |
| 经营预警 | `427:1379` / `427:1579` | 有依据的当前问题、等级/类型筛选、详情、查看建议 |
| 经营建议 | `427:1907` / `427:2107` | 建议队列、选中详情、依据、到现有页面人工处理 |

现有导航/authz中四个菜单均unavailable；Console overview仅空状态。SHEIN observations订单金额与供货价不是净利润，缺采购、广告、退款等完整经营事实；将其直接求和叫利润是架构错误。本首版不以Commercial钱包支出冒充店铺经营成本，也不修改Product/SHEIN canonical事实。

## 3. 单一owner与调用链

| 事实 | owner |
| --- | --- |
| 人工录入经营期收入/成本、修订与目标启用历史 | 新有界 `internal/operationscockpit` |
| 店铺身份/名称/平台/站点/状态/服务、成员grant | `internal/storecenter` |
| 已取得平台商品/订单/物流、同步范围/覆盖 | `internal/marketplace/shein/observations` |
| 钱包、资源、费用、订阅、AI调用与Product事实 | 原owner，本批不复制/修改 |
| 数据库/官方观察消费/身份adapter | feature-local integration；app只组合 |

链路：Console/BFF → CurrentIdentity + live Effective Organization/module permission → Cockpit usecase（scope只来自服务端）→ 当前member-scoped Store窄reader → Cockpit repository → 纯计算projection → 最后live权限/店铺重查 → 当前企业响应。

Domain不importGin/GORM/app/SDK。Repository只访问自己的schema，当前Store reader消费既有owner；禁止直接联表读Store/Commercial/Product绕过授权。SHEIN观察通过现有已授权Service读取，不直读观察SQL，不发新的官方API调用；缺观察permission单独标不可用，不能因此删掉已授权人工经营数据。

## 4. 经营事实及计算合同

录入一条事实绑定`organization + store + record UUID`，携带`startDate + endDate`，日期为UTC+8自然日闭区间（1–366日），结束不晚于昨日。运营人员可按每日/每周/每月或明确自定义已完成区间录入；当期累计录入可到昨日，今日不完整数据不冒充完整自然日。新记录不得与该店现有记录区间重叠；稳定record ID/store不变，修正金额或误输日期都创建新revision，事务内排除该记录自身后重新检查与其他当前记录不重叠，保留原revision。不能因为永久绑定错误日期而留下无合法修复路径的数据。

币种首版CNY，与Figma人民币目标一致。UI明确“人工录入·人民币”；外币平台观察绝不自动混加。不用浮点金额：每项非负整数分，单项≤10^12分；持久和JSON整数都在JS安全范围。录入字段：销售收入（退款前）、退款、本期已销售商品采购成本、物流成本、平台费用、广告成本、其他成本、备注（≤1000字符，可空）。采购入库付款不自动当本期售出成本，录入人员明确填写对应期经营成本。

净收入=销售收入−退款；净利润=净收入−各项成本；利润率=净利润÷净收入，净收入≤0则不评价利润率。允许退款超过本期销售/亏损，不能压成0。所有加总和差值使用checked arithmetic，输出绝对值不得超过JS安全整数；超过则拒绝该汇总并提示缩小查询，不能wrap、截断或产生近似经营事实。目标/比例阈值交叉乘法使用标准库math/big精确比较。录入不是支付/会计账本、商品供货成本或预测；显示录入者、更新时间和revision。修正保留旧revision，不提供删除/暗改历史。

查询区间只能加总**完整落在查询内**的非重叠记录，不能按天摊销一个月总数。若存在跨越查询边界的记录，或没有覆盖区间所有自然日，返回`coverage=incomplete`并列缺口/被排除区间；可显示“已录入部分”，不得以其评价完整利润/同比/目标完成度。完整且显式0录入才0。最多366天，最多500家可访问店铺；每页≤50，服务器aggregate不从前端页态求和。

矩阵同周期及其前一等长周期使用相同规则；完整且上期利润>0才给变化百分比，否则“不可比较”。净利润排序仅在覆盖完整店铺间，缺失排后；filters不会扩大Store授权。2–5店比较为同查询事实的只读子集，非新owner。企业多店总目标不自动分配给各店：单店状态表达“亏损/收支平衡/录入完整/数据不完整”，完整且盈利也不冒称全维度经营健康；目标正常/关注/异常只评价其完整目标范围。

## 5. 目标与评判

企业目标配置：一个当前启用revision，引用1–50家明确Store ID、日/周/月（周以周一为起点）、对应自然日期、正人民币利润目标、可选最低净利润率（0–100%，整数basis points）、正常/关注阈值（推荐90%/70%，可配置，0<关注<正常≤100%）。相同店铺组不推断企业全部店铺。

保存并启用一次事务：append不可变version + 更新当前head + append操作回执；编辑仅浏览器草稿。恢复历史版本也是以当前权限验证后创建新revision，不回滚行或复用旧approval。所有读目标/历史/恢复必须当前可访问其全部店铺；任何店铺撤权时不泄漏名称、目标金额或关联数据，返回不可访问。

**目标归属待用户决定，阻正式准入**：本草案暂拟企业singleton，但普通运营不可读旧head时既不能获得expectedRevision，也不能盲覆盖他人范围。已向用户提交两种具体路径：企业共同目标/管理员设置，或每成员个人目标。确定前不实现singleton schema或角色规则。若选择企业共同，旧/新目标写均只允许当前企业管理员，运营只读/录入；若选择个人，则head/revision/receipt均必须加入actor身份、个人只修改自己的目标，不能在企业singleton上隐藏actor过滤。无论哪种路径，原scope权限失效的历史恢复仍拒绝，目标范围内店铺被退休时只显示安全失效信息，由合法目标owner以当前version创建新scope，不尝试读取退休店铺历史来“恢复兼容”。

评估到`min(昨日,目标结束日)`，未开始/今日目标尚无完整日则`not_started/pending_data`；截至该日全部所选店铺完整覆盖才计算。应达利润=目标金额×已过去自然日÷目标总自然日，使用整数/有界有理数比较，不浮点舍入触发阈值。完整数据下完成度≥正常阈值为正常、≥关注阈值为需关注、否则异常；利润率低于启用底线至少需关注。目标过期不隐式续期。

Figma的安全底线只消费真正存在的平台异常事实；库存为0/平台订单异常独立显示，不能称处罚/账户限制或凭空给经营状态降级。无事实无法判断，不默认健康。不推荐利润目标金额（缺已完成参照时不可推造25k/27k/30k）；有完整上期可展示参照，不替用户设定目标。

## 6. 预警与建议

首版纯projection，刷新即基于当前数据重新判断，没有持久alert状态机/确认/忽略/自动恢复。

| 条件 | 预警依据 | 建议与正常入口 |
| --- | --- | --- |
| 当前目标完整数据低于阈值/利润率底线 | 精确目标revision、截止日、录入revision、公式 | 检查收入/成本构成；到目标配置/矩阵录入核对 |
| 店铺选定周期完整净利润<0 | 该店期收入/退款/各成本/版本 | 核查亏损构成与录入；到矩阵详情 |
| 周期经营数据缺失/跨界 | 区间和缺口 | 补录已完成期间；不做盈利等级判断 |
| 已保存SHEIN订单Exceptional事实 | 原观察generation/时间/范围/完整性、原理由 | 查看现有订单页面，发货/售后到平台处理 |
| 已保存有明确数量的SKU各仓均为0 | 原商品generation/时间、SKU/仓数量 | 到店铺商品核查，再由人到平台处理；不预测缺货天数或金额 |

观察取得时间始终显示；跨当前筛选周期的观察不混入人工经营同期指标。同步/源绑定失败、缺权限/模式不支持、最新失败、覆盖不完整显式表达；旧观察不能叫实时正常。大集无新跨记录扫描合同则只消费已有真正aggregate/有界分页并明确列示范围，不用前50项下推全店告警数量。优先先交付目标/利润/数据覆盖和现有订单异常aggregate，库存详情如缺完整aggregate不伪装覆盖全部店铺。

建议标题使用“经营建议/规则依据”，不冒称AI推理；不显示预计节省、转化提升或补货数；操作就是合法固定已有页面link与原事实详情，不签发已处理。AI方案/确认任务/外部执行明确未开放，并由用户新决定延期。

## 7. 授权、事务、幂等与恢复

新增有界四read permission：`workbench.cockpit.goals.read`、`.stores.read`、`.alerts.read`、`.advice.read`；`workbench.cockpit.manage`用于事实/目标写。沿当前native enterprise module grant、平台既有角色policy、当前tenant admin规则；不开放旧viewer/operator角色，不创建新角色体系。读取相应read并需要Store read/current member grant；写需要manage和Store当前访问；模块授予manage仅适用目标/矩阵，不以alerts/advice只读授予写。

全部请求live重验当前member/module/organization；不信客户端org/user/member/角色。持久化使用Store专用数据库中的独立`operations_cockpit` schema，便于借用当前Store事务和grant行锁。本repo不拥有Store表。feature-local integration在写事务中借用同一DB transaction的既有member-scoped Store repository，按排序Store ID锁定当前记录/grant并验证；锁保持到commit，避免撤权写穿透。当前Get只在非admin路径锁grant，不锁Store，不能当作满足此合同：由Store当前owner最小提供借用tx的锁读能力，同时admin路径锁Store，integration不得自己直读/锁Store表。当前IAMlive recheck在进入事务及提交前执行；不创长期凭据/工作流身份。

每个写绑定`org + actor + UUID operationKey + normalized intent hash`（含expected revision、精确stores/周期/金额）。org-head/店铺事实head锁并由DB唯一键串行化；同key同payload回放原已提交结果，同key不同payload409。expectedRevision必填，create=0，stale=412。一次事务包含新revision、head及成功回执；未commit无结果，响应丢失按原key重发/读回。不blind生成新key；权限撤销后的回放仍重验原资源，不泄漏旧回执。

无跨数据库业务写、无外部副作用、无outbox/Saga/Temporal/UNKNOWN新协议。取消/timeout在commit前rollback；已commit后保留原结果。revision使用int64+字符串输出避免溢出；溢出拒绝。并发相同资源/不同区间重叠只允许一个成功。只保留本新系统事实和操作回执，不迁移旧系统，不自行增加清理器。

## 8. 持久化、接线与共享Writer

最小表：企业串行head；店铺事实锁head；经营区间事实head及不可变revision；目标head及不可变revision；actor-scoped command receipts。payload有界JSON，显式check/唯一键/索引，SQL schema-qualified。原生Store行/grant只由当前owner操作；初始化是显式空schema安装，同事务，serving只VerifySchema、不建表；运行账号只获得本schema必要SELECT/INSERT/UPDATE，不授予DDL/DELETE。

共享增量已在[#137接单通知](https://github.com/qq550723504/task-processor/issues/137#issuecomment-6091453232)登记：Console四菜单/三级goal路径、authz四module/policy、currentapplication config/schema initializer/HTTP injection/Store serving role本schema grant和preflight。**未协调前不写共享路径**。feature-local可在准入后由本唯一Writer实现，完整运行交付仍需共享owner接线。

拟正常页面：`/workbench/overview/goals`（设置子页`/settings`）、`/workbench/overview/stores`（录入/详情在此）、`/workbench/overview/alerts`、`/workbench/overview/advice`；复用WorkspaceAppShell、ConsolePage、Card/Input/Button、现有主题/tokens。feature-local CSS表达Figma的两列目标、矩阵表/选择条、预警列表、建议队列详情。既有品牌及导航资产复用确认完全相同的asset，不新增临时URL。

HTTP拟`/api/v1/workbench/operations-cockpit/{capabilities,stores,facts,goals,alerts,advice}`；GET显式query白名单、拒绝未读body；POST严格JSON（未知/重复字段拒绝）、16KiB body、10s deadline、UUID/idempotency/expected revision；response≤1MiB、no-store。逐页history ≤20、rows≤50；error安全码不暴露SQL/凭据。BFF复用当前token、Expected Organization/User headers、同源/CSRF、strict JSON/schema/bounds；企业/用户query-key、晚到响应scope重验、切换清理旧表单/详情/receipt，UNKNOWN HTTP结果保留原key与payload。

## 9. 验证、交接与Legacy

TDD先RED捕获：非重叠/修订/负利润/退款/金额溢出、缺口和跨界不得完整、按period前期比较、目标自然日比例/阈值边界/未开始、幂等同/异payload及旧revision、scope/store撤权和回放、原子head/version/receipt及并发。复用现有真实临时PostgreSQL/testcontainers、schema权限检查和前端Vitest/Playwright；不建runner/故障平台。

独立第1轮检查新owner/授权/共享事务，最多两轮正常准入；形成完整用户路径后一次最终diff检查。业务规则/持久化/跨企业反例与必要Go/UI/typecheck/最终必需CI即可；不自行扩全面验收矩阵。真实provider/经营业务数据/共享部署/用户验收NOT_RUN，开发自检不签产品验收。

Legacy decision: N/A。已有合格Store、observations、Console/authz合同按当前owner复用；不依赖RETIRE ListingKit task/queue/sheinsync/tenantbridge。若调查旧行为只作EXTRACT/RETIRE，无兼容/迁移。

交接包含实际入口或正常启动命令、模块/店铺权限、录入真实期数据/修订/目标启用/比较/预警依据操作、保存位置与stop/restart保存方式、未开放AI/外部自动操作及NOT_RUN。未获授权不操作既有试用volume/schema/真实店铺数据，不合并/关单/部署。

## 10. Architecture Review

第1轮独立评审待执行。设计未经准入不能写正式生产路径。
