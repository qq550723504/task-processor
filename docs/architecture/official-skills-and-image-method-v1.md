# 官方标题与图片技能 V1

> Execution: [#774](https://github.com/qq550723504/task-processor/issues/774)，Refs #756 / #688 / #612。
> Design Basis: **Independent Architecture**；Admission: **IMPLEMENTATION_READY**。
> 开工基线：`5e738e3ea69ad7a940a67dc221f67e1f88f7ba23`。本文件与 Figma 先于生产实现；两轮适用独立检查已完成，冻结设计候选4149cbf307。

## 1. 产品决定与结果

2026-10-18，用户在原会话明确批准：先预置标题和图片的官方技能，技能创作助手作为后续增量；先更新 Figma 和 Issue，再开发；保留已有数据/配置，注意已有会话，不重复派工。

使用者为商品运营和平台模板维护者。交付一条可用路径：维护者安装六份真实官方包 → 用户查看准确版本/示例并按许可复制 → 在既有标题或整套图片操作中显式选择兼容版本 → 原计划/报价/确认、执行和采用。官方包是方法，不自行调用模型、选择新素材、增加任务、改变平台发布规则或扩大权限。

| 标题方法 | 图片方法 |
| --- | --- |
| 商品标题整理：关键属性排序、去重与清理促销词 | 标准商品套图：围绕已选主图/详情任务组织表达 |
| 标题本地化：目标语言自然表达与术语一致 | 场景化商品展示：真实用途、构图/光线与商品一致性 |
| 冗长标题精简：识别信息优先、压缩重复修饰 | 卖点详情图编排：已确认卖点、图文层级和阅读顺序 |

每份包包括适用范围、输入、步骤、输出/限制及一组明确标为示例的输入输出；不得把示例当商品事实。图片按已有任务选择，不强制生成 8+8；缺尺寸/配件/功效等依据时沿原证据检查补充或取消对应任务。

Must：六包可被原管理/准确版本/复制消费；清楚区分标题/图片适用能力；显式安装幂等且不覆盖任何现存版本/状态/个人团队数据；图片准确方法引用进入计划、指纹和确认；当前身份/企业/能力与方法可用性复验；新生成/采用受撤销栅栏约束；UNKNOWN不重发；不选技能保持原路径。

首批图片执行支持平台与团队已启用版本。个人图片定制可经现有 Consent/团队审核创建团队副本后使用；个人图片直接执行的独立准入不在本轮，不借用标题专属 allowPersonal 配置。标题个人执行与其原准入不变。

Should：明暗主题、移动布局、准确版本说明易读。Out of Scope：技能创作助手/AI整理草稿、图片个人直接执行准入、新规划模型调用、多技能调度、任意脚本/网络、POD扩展、公开/付费市场、自动发布、现有实例更新/目录写入、真实provider调用、迁移/清理。无新增 Accepted Risk。

## 2. Figma / Product Authority

沿用 [最终 UI/IA](../product/final-ui-ia-authority.md)、[Skill V1](skill-capability-v1.md)、[完整图片 V1](product-image-agent-v1.md)、原 ImageSet/Generation/Asset 和统一点数合同。

文件 `tg48P46SSXl6TBy9lZwg63` / 页面 `31:463`：

- 官方目录原节点原位更新：深色 [6676:3254](https://www.figma.com/design/tg48P46SSXl6TBy9lZwg63?node-id=6676-3254)、浅色 [6676:3344](https://www.figma.com/design/tg48P46SSXl6TBy9lZwg63?node-id=6676-3344)。保留分类选项卡/侧栏/原型入口，分成标题/图片两组，每组3个真实包入口；继续复用技能列表项组件 `6767:1671`。
- 未安装/维护者入口：深色 `6860:1532`、浅色 `6860:1719`；显示待安装及平台维护者按钮，不伪造六包已存在。
- 图片操作中的方法选择与计划确认摘要：深色 [6861:1826](https://www.figma.com/design/tg48P46SSXl6TBy9lZwg63?node-id=6861-1826)、浅色 `6861:1864`；可明确不使用，仅列兼容的官方/团队版本，选定后展示来源与准确版本。
- 失效版本阻止继续：深色 `6861:1845`、浅色 `6861:1883`；须重新选择或明确取消。不增加创作助手入口。

这些是设计态；运行内容必须读取实际目录、当前权限及实际计划/报价，不把 Figma 示例转为已有数据或成功状态。复用当前 Noto Sans SC、主题变量、按钮与选项卡，不建立另一设计系统。

## 3. Owner 与最小改动

| 事实/责任 | 当前 owner / 本轮变化 |
| --- | --- |
| Skill/不可变版本/复制来源/共享申请 | `internal/skill` + `internal/integration/persistence/skill`，原 RunDB；六包安装仍写原表，不增加平行目录或启动时 fallback |
| 身份、企业、成员、全局平台维护权限 | 原 auth/IAM；沿已验证 principal，安装仅原 platform maintainer |
| 标题执行与个人准入 | 原 Product Agent + agentconfig；入口/事务/准入不变 |
| 图片模板、任务、预算/模式确认 | 原 agentconfig / ImageSet；方法不能改任务集合、素材、规则、路由或报价 |
| 图片运行/slot/effect 与 UNKNOWN | 原 imageagent / Generation；仅增加可选准确方法绑定和输入，不新增状态机/重试 owner |
| 素材与正式采用 | 原 Product Asset；在原采用操作外层取得方法可用性栅栏，原 provenance/事务/幂等回执不变 |

现状缺口：Skill 使用/候选检查在原 Store 写死 product.content.agent；图片计划没有方法引用。不能仅在 UI 新增模板卡片或把方法塞进 background/brief 冒充准确 Skill 接入。

新包在现有受限 frontmatter.metadata 内使用明确的可选标记 `listingkit-method-v1: product-title | product-image` 和 `listingkit-display-name`。只对该标记的两个准确值投影 target；其余任意 metadata（包括既有 `target`、未知标记值）仍按原合同作为有界普通文本接受，不收紧共享包校验器，也不改写已保存文件或 digest。未明确标记的当前方法继续沿当前标题方法合同，图片必须明确标记 product-image；HTTP 请求的消费能力则必须命中枚举。标记只代表内容兼容性，不代表身份、工具或执行权限。解析、文件/字节限制、摘要与原 yaml.v3/goldmark 校验器复用，不引入新解析器。Package 展示投影可包含可选展示名/target；实际使用从准确版本文件重新验证，不能信客户端投影。这是同一当前包合同的可选解释，不是 Legacy 兼容层、双读或迁移。

## 4. 官方包安装

六份 SKILL.md/文本示例作为受版本控制的内置发布材料，复用现包校验器。运行时读、复制和执行一律消费原数据库准确版本；代码包不是第二套可执行目录，不与数据库双读/同步。

平台维护者通过原官方目录中的显式安装操作调用新有界安装命令；不是服务启动/GET 的隐式写入。后端解析 live principal 并复核原平台维护权限；客户端不能提供官方身份、包正文、ID或状态。

安装使用确定的六个发布身份（现 UUID 库），在原 Skill Store 单事务取得目录安装互斥锁。不存在的条目以原平台 owner、校验后的发布包及 allowCopy/allowExport 创建并沿既有 publish/enable 规则启用；不复制用户资产。已存在条目验证平台归属及原发布身份后跳过，不覆盖、不追加版本、不重新启用被停用/撤销的版本。身份/内容冲突失败，整个事务回滚；不按同名猜测身份。重复/并发安装返回相同身份，不依赖维护者 actor 生成新的六份副本。

接口响应只区分本次新增/已存在及准确版本引用；未安装时普通用户继续看到真实空态。模板升级不静默发生，本轮只发布 V1，不增加同步/迁移/修复既有数据的机制。

## 5. 合同 → 实现 → 注入 → 消费者

1. Skill Store 提供受当前消费能力约束的候选/准确选择与使用栅栏；保留标题现有入口。图片仅接受 PLATFORM / 当前 ORGANIZATION、共享且 ENABLED 的 product-image 方法；检查当前图片智能体启用及原图片使用权限。禁止用标题 allowPersonal 推导图片准入。
2. Skill BFF/候选可传经过枚举校验的消费能力，原格式/管理/复制服务复用。`SkillPicker` 增加明确的目标能力参数；标题默认路径不变。`product-image-set-panel.tsx` 的方法选择只发送 ID/version/digest；不发送 owner、权限、方法正文或声明的官方状态。
3. 图片 HTTP/应用装配在验证原身份/来源后解析准确选择，注入最小 `ImageMethodContext` 端口。该端口通过原 Skill Store 和现配置 pool 获取材料/栅栏；domain 不导入持久化或provider SDK。
4. `PrepareImageSetInput` / `ImageSetPlan` 增加可选准确方法绑定；未选时字段不序列化，原无方法 hash/回执保持。绑定至少包含 SkillVersionRef 和固定图片方法 prompt-policy 版本，参与 input/plan/quote/slot generation 指纹；同 request key 更换方法必须冲突。配置、素材、规则与任务检查仍来自原 owner。
5. 图片准备时读取并验证有界准确包、检查每个所选 slot 的完整 prompt 字节边界；超界明确拒绝，不截断。计划/API/checkpoint只保存准确引用及 digest，不复制方法正文。材料仅在调用前从准确 Skill 版本重新取得并验证 digest。
6. `GenerationExecutionDependencies` 增加图片方法材料/可用性端口。报价和新派发分别验证；provider 请求构造在原确定性商品策略之后加入显式隔离的 untrusted-method 区域，不能覆盖原 schema/商品事实/素材/预算/路由。实际请求的材料由已验证 ref/digest 和固定组合策略确定；InputDigest/fingerprint绑定这些输入。材料不进入普通日志、浏览器回执或 Temporal checkpoint。
7. 新派发先沿原 Generation fact 检查已存在 attempt；已有 prepared/UNKNOWN/success 仍只读回/交给原恢复 owner，绝不因方法变化或撤销取得新POST许可。仅新 effect 在原 reserve/dispatch/provider调用前取得可用性读锁，并持有到既有调用返回（含UNKNOWN），随后可靠释放。
8. 图片确认先读取并核对原 ImageDB admission/projection 和 `ReadImageRunAdmission` 的准确 action/plan/quote/input 回执；已有原回执只恢复同一 CAS/原 workflow 输入，不受后来方法停用阻断，也不因此授予新 provider POST。只有原 owner 确认没有准入回执时，新确认才取得 Skill 可用性栅栏，持有到原 admission 写入及 ImageDB CAS 返回。新的正式素材采用同样先查原准确 action receipt，再为新动作取得栅栏并持有到原 Asset commit 返回，不能在 candidate reader 返回时提前释放。回执重放不重复采用或重计费；新采用仍校验完整原 run/plan/slot/attempt/source/target/result digest。
9. `current_application.go` / `image_set_application.go` / 原图片 worker 装配同时注入准备、派发、采用所需端口；选择了方法但任一必要端口/ schema/受信 pool 不可用时 fail closed。未选方法时原功能继续沿原合同，不补猜测或兼容路径。

具体叶子端口命名可在实现时收敛；上述准确绑定、锁的持有范围及 owner 不可改变。不得把临时正文拼到长期模板、background 或用户配置，省略撤销/指纹检查。

## 6. 状态、事务与并发

不新增 Skill 状态或图片运行状态。新官方包沿 DRAFT → READY → ENABLED；现有状态/版本不可变规则保持。计划仍是 prepare → 原 quote/confirm → 逐 slot effect → 原候选/Asset 采用，原 failure/UNKNOWN/recovery 状态不变。

安装只有 Skill DB 单事务。方法撤销栅栏由原 Skill owner 的 PG shared-row lock 持有，方法停用/撤销写锁互斥；图片和 Asset 的写入仍各在原 owner 事务中。跨 pool 不伪称共享事务：栅栏只保证一次既有操作执行期间方法可用，不创建跨库写/补偿/第二回执。

锁顺序：先原身份/来源权限检查；再 Skill 可用性栅栏；后原图片确认/新 effect 或 Asset 提交。Skill 状态写路径不反向取得图片/Asset锁。不长期跨用户确认持锁。上下文取消/进程退出释放 PG tx；持有时间受既有请求/provider deadline 约束，不新增无限锁/自动重试。

| 场景 | 结果 |
| --- | --- |
| 方法在确认/新生成前被停用、撤销或失权 | 拒绝新动作；无新provider请求或正式采用 |
| 状态写与新动作并发 | 栅栏与状态写串行决定顺序，不容许检查后偷跑 |
| 计划/报价响应丢失，同键同载荷 | 原准确回执/计划读回；同键换方法冲突 |
| 确认准入成功但 ImageDB CAS/响应丢失，方法随后停用 | 先读并校验原准入回执，恢复同一 CAS/原 workflow；新 slot 派发仍须当前方法可用 |
| 已有effect UNKNOWN、方法后来变化 | 沿原恢复事实读回，不重新生成/预留，不把撤销当失败补偿 |
| provider返回后方法才停用 | 已发生effect仍沿原计量/结算；阻止后续新effect/新采用，不撤销真实费用 |
| 原采用响应丢失，方法随后撤销 | 同action读原回执，不重复Asset写入 |
| 切用户/企业/配置失效 | 原请求子树/选择清除；服务/worker各复验，不信浏览器缓存 |

## 7. Threat Model 与边界

保护当前 Must：跨企业/他人方法读取与使用、伪造官方身份/目标投影、技能正文覆盖系统策略、检查后撤销竞态、错误输入指纹、重复外部请求/预留/采用、原数据或配置被安装覆盖。

Skill 文本为不可信方法，不授予任何新能力；不承诺语义校验能识别所有恶意指令。真实商品依据、平台规则、权限/预算与输出校验由原代码保证。无需构建新的脚本沙箱、通用agent调度、内容审核平台或专项验收工具。个人图片直接执行未开放，不伪造可用入口。

Legacy decision: N/A；只复用当前 Skill/ImageSet/Generation/Asset，不依赖或包装 RETIRE 抽象，不新增数据迁移、双读双写或第二事实源。当前实例、数据、配置、密钥、卷和其它会话不操作。

## 8. 验证与交接

业务变更按 TDD：先目标缺陷 RED，再最小实现 GREEN。只补本范围必要断言，复用原 PG/Skill、ImageSet、Generation 与 Asset fixture，不建新runner。

- 六包通过原格式/字节/路径校验，目标/展示与示例准确；跨能力选择拒绝；未知目标/投影篡改不能改变服务判定。
- 安装首次/重复/并发/不同维护者/已有停用或撤销/身份冲突与部分失败，确认无覆盖和无半套事务。
- 原标题候选/个人准入/执行回归；图片官方与本企业团队可用、个人/跨企业/失权/失效版本拒绝；无方法原指纹/回执不变。
- 新方法进入 input/plan/quote/effect digest，换方法同键冲突；超界零派发；provider可见受隔离的方法正文，普通日志/API/checkpoint没有正文。
- 停用/撤销与确认、派发、采用的范围相关并发检查；已有UNKNOWN/采用回执重放不产生新效果。旧已确认且未变化的证据可明确复用，不重复全量矩阵。
- 前后端相关类型/lint/test/build及稳定候选必需CI，一次最终交付检查；隔离Edge对照Figma明暗/移动/原权限。

本批一个主要PR。设计独立检查最多两轮；先分类finding，只有命中当前Must BLOCKER才重开冻结基线。完成后交出代码/入口/安装操作和限制，状态In Review。真实provider、当前实例/目录安装、合并/部署/关闭Issue均未授权，不以CI或fixture代签用户产品验收。

## 9. 独立设计检查

Round1（03aa53a24，[检查记录](https://github.com/qq550723504/task-processor/pull/795#pullrequestreview-5558834422)）两项均分类 BLOCKER，影响原数据可读与确认核心路径：

- [metadata.target 反向收紧](https://github.com/qq550723504/task-processor/pull/795#discussion_r4303526542)：接受根因；采用可选明确标记，普通 metadata 的原校验不变，不改写任何数据。共享解析器回归覆盖已有任意 target/未知标记。
- [确认回执先于方法栅栏](https://github.com/qq550723504/task-processor/pull/795#discussion_r4303526546)：接受根因；第5节步骤8明确原准入回执优先、只有新准入持锁，测试覆盖准入成功/CAS丢失/后续停用的同 action 恢复。

Round2仅复核两项增量（4149cbf307，[完成记录](https://github.com/qq550723504/task-processor/pull/795#issuecomment-6236901704)）：未发现重大问题。两项BLOCKER已在设计层收敛，进入IMPLEMENTATION_READY；对应断言仍须实现测试，不把设计复核当运行验收。
