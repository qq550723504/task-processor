# Account Audit 完整历史筛选 V1

状态：IMPLEMENTATION_READY。Issue #581；独立 reviewer `audit_summary_review` 对 design-only HEAD `aa0177cf397928c0181e44100e5d6ab6952d0436` / 本文 blob `37c3ee1aa4f15640c9241181e2e0e9dabedd1489` 完成只读 Architecture Review，设计级 `BLOCKER=0`。其 `IMPLEMENTATION_TEST` 指出 AI usage owner 原生单次 limit≤50，适配层须分批并用跨第50条的稀疏命中测试证明完整分页；该 finding 阻本片合并，不阻设计准入。本准入不授权合并、部署或产品验收。

## 用户结果与依据

已获当前企业操作记录读取权限的用户，在 `/workbench/account/organization/audit` 按内容/对象、时间范围、受影响或消耗资源的成员定位六个已接入 owner 的真实历史事件；与原操作人、操作类型条件组合，并从完整匹配集合翻页。当前事件覆盖范围如页面现有说明；不声称包含所有业务操作。

Authority：Issue #581、父项 #469、[Final UI/IA](../product/final-ui-ia-authority.md)、Figma `tg48P46SSXl6TBy9lZwg63 / 1532:525`（2026-09-30 实读代码与截图：搜索、近30天、全部操作、全部成员、筛选按钮）；用户 2026-09-30 决定“成员筛选”按受影响或消耗资源者，已有操作人筛选独立保留。[四卡与当前列表基线](account-audit-summary-v1.md) 不定义本次三项筛选，四卡继续独立读取完整近30天窗口。

Scope：当前 Account Audit 六个源、查询/游标/HTTP/BFF/strict client/页面，以及必要的原 owner 只读查询局部优化。Out of Scope：导出、历史补写、第二事实表或统一索引、姓名/联系方式的历史快照或跨源用户资料搜索、其他业务事件、四卡语义、状态机、授权/租户政策、写入或外部副作用、部署和真实数据操作。Figma 的示例文字/数字不是真实审计事实；没有承诺下拉选项或姓名查询。

## 筛选口径

列表页面初始选择“近30天”，另提供“近7天”“全部时间”。7/30 天为服务器 UTC `asOf` 往前 7/30 × 24 小时的 `[from, asOf)`；“全部时间”为当前 owner 保留的历史中 `< asOf` 的事件。页面显式发送 `period=7d|30d|all`。为了不改变内部四卡遍历及旧 API 调用，未传 period 的旧 `ReadFiltered`/HTTP 请求仍保留原无时间限制行为；新页面必须显式发送默认 `30d`。每次首次列表请求冻结 `asOf` 并随游标传递；切换条件或显式刷新重开窗口。使用事件原 owner 时间；现有跨库实时 keyset 不是全局快照，期间迟到/回填的旧时间事件可能在下次刷新才看到，不宣称强快照。

内容/对象：`query` 最多 80 个 UTF-8 字节，去首尾空白后不能为空，拒绝控制字符；Unicode 大小写不敏感的连续子串。只匹配本批次可解释的展示内容：事件操作中文名称及固定模块/对象名称、原 `operation`/`objectType` 字符串、`objectReference`、可见的 `relation.reference`、资源种类/数量、usage/points 的成员 ID/数量。界面展示的名称词表由 Account Audit 查询合同固定，不从无界业务载荷、provider 响应或当前资料推断。`actor` 不属于内容搜索；人名、登录名、手机号、邮箱不保证可搜；当前事件没有这些历史快照。查询结果仍完整显示原事实与 ID，不以展示文字替换 owner 数据。服务器同时执行匹配，页面不在当前页二次过滤。

成员：`member` 是当前身份合同的非空有界成员/user ID，空值表示全部。匹配关系严格为：profile 更新的 target `UserID`、membership 操作的 `TargetUserID`、member-resource/月限的 `MemberID`、AI usage 与图片点数的 `MemberID`。Source-account 操作没有受影响成员字段，不因 `ActorSubject` 等于选择值而命中；actor 只受独立 `actor` 条件控制。UI 使用可输入的成员 ID 选择控件，并在事件详情中提供可复制的成员 ID；当前成员可通过现有成员页查 ID，已移除成员可使用历史事件中原 ID。当前目录不是完整历史成员目录，也不能成为本筛选的事实/授权前置；控件明确说明历史成员需输入原 ID，不能承诺“按姓名选出全部历史成员”。未来如需姓名式历史选择，须单独决定可信历史显示名来源，不能把当前目录误称完整历史。

操作类型与操作人沿现有 allowlist、权限及原 owner pushdown；usage 无 actor，填 actor 时不命中。所有条件按 AND；服务端拒绝未知/重复/空参数、不合法值和过长查询。条件变化、身份/企业/角色切换卸载旧列表、清除旧游标；旧条件/旧企业游标请求返回 `INVALID_REQUEST`，不会回退第一页。

## 完整分页、失败与资源边界

Account Audit 只读投影按原 owner 的稳定时间/键降序批量读取，合并后再按上述三项条件筛选。每个已跳过的候选也推进该源位置；跨源合并以当前既有时间/源/键排序。一次响应只有收集 `limit` 条匹配且确认还有下一条匹配后才给 `nextCursor`；精确耗尽则 null。确认下一条时，游标停在该条之前，不能把预读匹配吞掉。可返回少于 `limit` 的成功页仅在匹配集合已完整耗尽时。全范围无匹配才返回成功空页；当前批次无匹配但后面有匹配时继续读，不能返回假空页或“下一页再找”。

外部游标仍是 canonical base64url JSON；绑定组织、actor/operation、query、member、period、冻结 `asOf` 及六源位置，拒绝不匹配/超长/非规范/缺必要字段。尽量复用现有 `positionWire` 与源位置；增量扫描需要保留每个已消费事件之后的六源状态，并在预读下一匹配前截取状态。不得把一整个内层游标再 base64 嵌套导致溢出。请求最多每源100条/批，只在单请求中持有一批候选和最多100条结果；不存储搜索事实、服务端 session 或后台 job。

沿现有后端10s、BFF/客户端15s deadline、单页上限100、128KiB 响应、no-store 和取消传播。扫描没有静默行数上限；如果完整页或“无下一匹配”的判断无法在 deadline 内完成，返回现有 504 `DEADLINE_EXCEEDED`，丢弃本次部分结果，UI 显示读取失败而非0条。若未来引入显式扫描上限，也只能返回错误而非部分成功；本批次不增加任意数量门槛。必要时可在原 owner 只读 query 增加等价下推，但查询必须保留同一排序与游标/匹配语义；不新增 schema/index/第二 owner 作为开工前置。现有少量读次数可满足阶段需求，实际规模瓶颈由运行数据触发下一步。

## owner、接线和安全

Fact owner 保持 source-account、accountprofile、organization membership、AI invocation usage、orgresource 图片点数/成员资源各自数据库/事件；Account Audit 不拥有新事实。读取合同 → `accountaudit.Query.ReadFiltered` → `accountAuditModule` `GET /api/v1/account/audit` → Auth.js BFF `GET /api/account/audit` → strict typed client → AuditPage。只扩展同一路由的 `query/period/member` 参数；四卡 `GET .../summary` 和现有事实 payload 不变。以原 `source_account.read`、VerifiedIdentity、LiveWrite fresh organization resolver、source owner read 授权，每次分页重新校验；查询字段不影响权限。选择历史成员不能访问该用户所属其他企业，因为每源先由当前组织限定，再匹配 ID。

请求大小预算需覆盖六源位置游标、URL 编码查询和成员 ID；实现采用游标最多3072字符、URL query 最多4096字符，HTTP、BFF、client 对齐并测试最长合法组合，超界统一 `INVALID_REQUEST`。当前 `source` 字段固定带源账号前缀，其他后缀取决于读取页的源数据；它不证明全量搜索完整性。跨源缺源/查询错误、身份过期、组织撤权和结果校验失败均 fail closed，不返回已经扫描的部分匹配；AI 用量读取须同时具备 image/product 两个当前 namespace 的 owner pool，缺任一 pool 时筛选请求返回不可用，不能将未启用或未接入的来源当成历史为空；这不改变现有不带本次筛选条件的列表及独立 summary 路径。两个 pool 均存在而组织无记录才是真实空历史。前端请求 cache key 包含全部筛选/游标/企业/身份；加载期间隐藏旧结果，summary 保持独立可读。

不新增持久化、事务、状态机、补偿、retry/UNKNOWN owner、provider 调用或不可安全重复的副作用。Legacy decision：N/A；仅消费当前 owner，不接入已 RETIRE 的旧 Audit/Task 抽象。若实现发现需改变现有权限、事实 owner 或恢复协议，停止生产修改并重新走适用设计准入。

## 验证与交接

TDD 先复现稀疏匹配跨源跨页、预读边界和误导空页，再实现。必要测试：六源内容/对象及成员映射、source-account actor≠member、usage 无 actor、7/30/all 上下边界和冻结 asOf、组合 actor/operation、只在第 N 批命中/最后一条恰好满页、旧游标换组织/条件拒绝、依赖失败/超时不泄漏、请求/游标最长值、HTTP/BFF/client 严格解析、UI 选项/重置/历史 ID 输入/错误空态、summary 独立。复用既有测试与构造器，不建设专门 runner/验收平台。完整用户路径形成后一次最终独立复核；必要 CI 与精确 HEAD 写 PR。实际浏览器/用户产品验收在获授权环境另行记录 `NOT_RUN`，开发自检和 PR CI 不代签。
