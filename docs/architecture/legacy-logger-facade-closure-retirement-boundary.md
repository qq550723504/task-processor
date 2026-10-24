> #301 / #937 **USER_CONFIRMED**：[五组闲置支持门面完整退休闭包](legacy-idle-support-facade-closure-retirement-boundary.md) §1–§7（含§6.1）已获用户确认，Reuse Existing Architecture / Ready与Issue/PR读回6386195192/6386203704先于源码修改。仅收窄manager/测试广泛KEEP中的SetGlobalLogLevel全局便利别名及唯一专属测试。实例SetLevel/GetLevel、原锁/并发/singleton/初始化/获取/hook/writer/输出/关闭与其余全部测试保持，不恢复core/logger或新增全局门面。只替代准确旧KEEP；以下完整原Git正文/既有顶注/数字/勾选/批准/历史证据逐字保留。workflow/完整inventory/依赖/保护与既有实例/数据/凭据/进程零改，不增试用前置，保留独立SHEIN登录与Store同步，#301 OPEN。

> #301 / #934 **USER_CONFIRMED**：[旧闲置 adapter/helper 完整退休闭包](legacy-idle-adapter-helper-closure-retirement-boundary.md) §1–§7（含 §6.1）已获用户确认，Reuse Existing Architecture / Ready 与 Issue/PR 读回先于源码修改。仅替代 platform/logging/helpers.go 的必须可读/保活含义：原文件转明确缺席，原八项数字 Task/Store 日志禁入迁入同一原 owner-qualified AST guard；core/logger 根禁入、SourceIdentity/当前身份、实际 LogManager/配置/hook/writer 及所有其它断言保持。只按准确矩阵替代本项旧 KEEP；以下原文/已有顶注/数字/勾选/批准/历史证据完整逐字保留。workflow/完整 inventory/依赖/保护及既有实例/数据/凭据/进程零改，保留独立 SHEIN 登录与 Store 同步，不增试用前置，#301 OPEN。

> #301 / #926 **SUPERSEDED，仅本批旧平台调用闭包**：用户已确认[完整退休设计 §1–§7/§6.1](legacy-platform-api-closure-retirement-boundary.md)。本文涉及旧 Amazon/TEMU API、SHEIN 闲置 Client/manager/DTO、SPAPI/旧平台包装配置的 KEEP/当前 owner 建议，只在该准确删除/保留矩阵内被替代；不移植旧 auth refresh/retry 或平台副作用。当前 SHEIN DTO/完整 JSON/有效规则、SessionTransport/三个读取端点、采集/原 Amazon 浏览器/debug、官方 Store、独立 SHEIN 登录与 Store 同步及原 owner/fence/UNKNOWN 保留。只抽原仓库两纯函数，不加 compat；workflow/完整 inventory/计数/检查逻辑及既有实例/数据/凭据/进程不动，不增加试用前置。以下原完整正文、历史数字/勾选/批准/证据逐字保留。

> #301 / #923 **SUPERSEDED，仅旧租户 Prompt 闭包**：用户已确认[完整退休设计 §1–§7（含 §4.2/§6.1）](legacy-tenant-prompt-closure-retirement-boundary.md)。本文涉及旧 `internal/prompt`、`internal/promptmgmt`、`shared/tenantctx`、五 YAML 及其现役 KEEP/复用建议仅在该准确矩阵内被退休决定替代；如涉及镜像打包，仅退出两条 `prompts` COPY。当前 AI/图像/采集、身份/有效规则、独立 SHEIN 登录与 Store 同步及其它 KEEP 不变。停止全新 schema 创建旧 Prompt 表，不改 Goose 登记或已有表/行，不新增替代 Store/迁移/兼容。以下原正文、历史数字/勾选/证据完整保留，不扩大实施、合并或真实资源权限。

# #301 旧日志门面与全部消费接线完整退休设计

Status: **USER_CONFIRMED / SOURCE_IMPLEMENTED，待原实现 CI 与最终交付复核**。用户已确认冻结设计 cace5c1f4 §1–§7（含 §6.1），Reuse Existing Architecture / Ready 已在 Issue/PR 读回后才改生产源码；授权/准入证据 [#301 / 6342053569](https://github.com/qq550723504/shuomi-ai-commerce/issues/301#issuecomment-6342053569)。本批在 #893 整批实施，#301 OPEN，#29 不重开。准确实现 HEAD、CI、独立复核及后续批准留 PR，不在本文滚动追加状态提交。

调查基线 main **cbf2821d9a7fa18319e40907c8eae08dd184c28b**。#891 已按 ea36d44d1 正常合并，准确 push/main CI39777246449 attempt1 完整 SUCCESS；这是前批证据，不代替本设计或未来源码 CI。接单见 [#301 / 6341047238](https://github.com/qq550723504/shuomi-ai-commerce/issues/301#issuecomment-6341047238)。

## 1. 用户结果、完整范围与权限

使用者为维护当前应用、采集、平台 API、图像、Store 和独立 SHEIN worker 的维护者。源码交付结果是完整退出 forwarding-only 的 core/logger 与所有真实消费接线；有效日志继续由当前 runtime/Platform 和 Logrus 提供，不再通过旧兼容门面或把业务改为直接依赖 Platform。没有证据表明残留门面正在阻塞当前核心用户操作；本批由用户明确选择的退休目标驱动，不新增产品或内部试用前置。

闭包包含：core/logger 两源、publishing/shein 专属全局转发 helper、17 直接消费包的79个已知生产文件，以及这些构造/辅助函数向上到装配入口、向下到子组件和测试/debug 的全部日志传参尾部。保留当前业务行为，必要的注入字段/签名调整在同一 Delivery Batch 完成；不按文件数、目录或审核机械拆 PR。79是调查到的直接消费清单，不是用来遗漏无直接 import 的 constructor/caller 的文件上限。

设计阶段仅本文、Architecture 索引和 Legacy Register；用户确认后实施严格覆盖 §5/§6.1 的日志依赖切换、原 guard 转换和两条敏感日志参数修订。历史、计数、ceilings/其它检查、lint/workflow、依赖/锁及保护保持。设计/实施授权均不包括合并、部署、关单或真实资源操作。

不读取既有凭据、私密配置/状态/日志、Cookie/Profile/缓存，不执行应用/worker、浏览器/下载器/安装器、登录、provider、数据库或 Redis，不操作既有实例、数据、凭据或进程，不清 ignored 文件。调查只读 Git 追踪源码、文档及依赖元数据；测试、CI、编译与运行验收分别记录。

## 2. Design Basis、Authority 与保留不变量

设计阶段 **Design Basis: N/A**（只读调查/三文档）；用户确认后的源码实施为 **Reuse Existing Architecture / Ready**，完整复用以下现有 owner/合同，不新增领域或持久化事实：

- [Greenfield](../product/greenfield-no-legacy-migration.md)、[Hard-Cut Policy](../refactoring/legacy-hard-cut-policy.md)、[Legacy Register](../refactoring/legacy-register.md)和[Issue 驱动规则](../engineering/issue-driven-development.md)：只 EXTRACT/RETIRE，无旧 compatibility、双读/双写、迁移或第二事实源。
- [Project Target Architecture](project-target-architecture.md)、[Project Boundaries](project-boundaries.md) §3.6 和 [Phase2 runtime inventory](../refactoring/phase2-runtime-inventory.md)：Platform 保有日志实现/生命周期，当前门面只是暂留债务，业务最终使用消费端窄依赖。历史 ceiling 82/9 不是允许把79处业务 import直接替换为 Platform 的依据。
- [独立 SHEIN 身份/持久化边界](shein-worker-numeric-persistence-retirement-v1.md)、[当前值合同](legacy-numeric-value-contract-closure-retirement-boundary.md)、[旧采集尾部边界](legacy-numeric-acquisition-tail-closure-retirement-boundary.md)和[来源业务闭环](source-business-closure-v1.md)：当前采集、有效 API/规则、Store/身份/计费、独立 SHEIN 登录/同步及 UNKNOWN/fence 保留。

本批会跨多个源码子系统且超过30文件，但只退出已批准退休的门面、把已有 Logrus 日志依赖放回已有 owner/装配路径。日志管理、文件输出、当前领域合同/事实 owner、schema、状态/事务、身份/租户/权限、计费和 retry/UNKNOWN/recovery/平台副作用均不变；不靠拆 PR 规避准入。消费端只接收技术依赖，不创建业务状态或新的日志框架。若实现必须改变上述边界，停止该部分并升级 Independent Architecture，先达到 IMPLEMENTATION_READY。

Must：完整门面/调用尾部退出；保持当前能力、必要/安全检查和字段/级别/输出配置；业务不新增 platform/logging 或 appenv 全局门面依赖；§6.1 CURRENT 只做17删/1增；历史/计数/ceilings/accuracy/parser 和其它检查、全部 workflow/依赖/锁/保护不改；移除已发现的两条 Cookie 值日志；无真实状态操作或试用前置。Should：优先已有 WithLogger 构造器、Logrus/标准库及现有 guard，不造共享日志端口包、Registry/注入框架、脱敏框架或验收平台。Out of Scope：全仓 config 拆分、采集/浏览器/代理/平台规则重构、日志持久化/采样/轮转策略、更多理论安全增强、运行验收/文件清理。Accepted Risks：无新增；真实外部手工调用与物理状态 UNKNOWN，运行/用户验收 NOT_RUN。UI/IA/Figma 增量 N/A。

精确 supersession：具体设计获确认后，只在本文矩阵上取代 Phase2/core/logger 暂留、#868 对 compat.go 的精确读取和早期“继续迁移到 façade”的含义；旧历史正文/计数/登记行不改。当前 core/config、platform/logging、workerpool、provider Logger 端口和有效业务合同不被退休。

## 3. 实际消费者与明确的消费合同

### 3.1 当前事实

core/logger/compat.go 无独立实现或状态：5个类型 alias、13个字段 alias及函数变量转发至 platform/logging。现有生产调用只用 GetGlobalLogger，以及 appenv 的 LogConfig/LevelFileConfig/InitGlobalLogger/GetGlobalLogManager 和 shared/browser 的一次原始 Logrus 获取；无生产赋值替换这些门面函数变量的依据。compat_test.go 只验证 façade 与现 owner 是同一对象，不是保活 authority。

根模块依赖图有17个生产 importer；追踪源清单为79生产文件，覆盖别名和全部构建目标，并另读 debug 独立模块及当前入口。平台日志当前直接 importer 是 core/config、core/logger、platform/workerpool；业务不能通过新增16个直接 Platform importer绕过既有方向/ceiling。

| 当前链 | 有效行为/缺口 | 完整切换 |
| --- | --- | --- |
| appenv → façade → Platform | Setup/Apply 使用当前 manager/config/raw logger；属于现有 runtime helper | appenv 直接使用 Platform 的同一类型/函数；启动参数、配置结构、输出/级别/hooks语义保持，不保留旧别名 |
| crawler 1688/Amazon/shared → façade | 解析器、池/实例/超时、安装/下载和辅助函数按组件取日志；真实采集仍有效 | 显式传入消费端日志回调，逐层传递到全部子组件/无状态函数；业务算法/外部调用/超时/retry不动 |
| shared/browser ChromeDownloader | httpclient.New 需要当前 raw *logrus.Logger | 从注入日志 Entry.Logger取得相同 SDK 对象；不向业务暴露 LogManager 或新增管理访问 |
| current localagent/来源 adapter/debug/installer | 根模块 zero-importer扫描会漏掉手工debug；当前 CLI仍构建现有采集器 | 同批更新所有真实构造调用，不执行这些入口，不删除采集/设备授权/安装能力 |
| prompt registry/loader/watcher | 已有 NewPromptLoaderWithLogger/NewFileWatcherWithLogger；默认构造器及 NewRegistry(nil)仍查旧全局 | 复用现有显式入口；registry同一依赖传给loader/watcher；默认无日志包装退出，无新Registry/模板/持久化owner |
| publishing/shein logging.go → assembler | helper只把一个组件字符串交旧全局；assembler纯草稿能力保留 | 转为assembler实例日志依赖，专属全局helper同退；当前 Builder/Brand/价格/属性/审核语义不动 |
| Amazon/SHEIN/TEMU API、content规则 | 认证/HTTP/敏感词/纯规则有效；部分constructor内部取全局日志 | 从既有构造链传日志；仅调整技术依赖，平台请求/身份/重试/返回值不变，敏感词文件/保存worker owner不动 |
| sheinlogin包初始化全局 Entry | service.go在包初始化时捕获默认manager，automation_login_flow.go后续使用；可早于实际worker日志设置 | 删除该包级捕获；从worker runtime/build/service/automation实例携带日志。原Redis/PG/Profile/凭据/Store/授权/fence/UNKNOWN不变 |
| SMS handler | app/httpapi失败分支仅记录provider_code，但仍读旧门面 | 从现有HTTP装配传Entry/回调，记录相同安全字段；HMAC原body/限额/状态/代码/权限不动 |

### 3.2 固定技术合同：显式日志值/窄回调

1. 业务仅消费现有 `*logrus.Entry`，或当原来逐次取组件日志时消费 **`func(component string) *logrus.Entry`**。回调是实例/构造依赖，不能新建 shared/logger 包、统一注册器、全局 setter/factory 或把 façade 换名搬到 appenv/业务目录。消费包不得 import platform/logging、core/logger 或用 appenv 自行取全局日志。
2. 既有 WithLogger 构造器优先复用；已经有 Entry 的对象直接携带，不再创建第二接口。需要保留逐次获取语义的对象/纯辅助函数显式携带上述窄回调，父对象原样传给子对象。新增私有 logger_port.go 仅限 §5 的既有消费包，内容限该技术依赖/标准 Logrus 归一化，不能吸收业务逻辑。
3. 当前 runtime/CLI/HTTP/build 依据已有 logger 传入 Entry/组件回调；需要当前全局配置语义时，由 appenv/runtime 唯一适配到现 Platform getter。业务不接收 LogManager、Init/Close、配置读取或文件路径权限；函数依赖不进入 core/config、私密JSON、数据库、HTTP DTO、规范化或hash合同。
4. 未提供依赖或回调返回nil时，只使用 Logrus + io.Discard 的实例级禁用日志值；不得查旧/新全局、启动日志manager、读配置/文件或引入业务失败。现有有效运行入口必须在装配中传入日志，并以合成fixture证明；不能悄悄让当前worker/采集变成静默。这里是可选诊断依赖，不是旧业务数据或服务fallback，不新增启动/试用门禁。
5. 初始化仍归当前 runtime。配置在构造消费对象前按现有顺序生效；重复Get、级别更新、hook/输出引用使用原Platform/Logrus行为。逐次getter的路径使用同一runtime回调；不把它提前缓存成另一个manager快照。去掉SHEIN包初始化捕获可让日志使用实际worker设置，不构成登录/恢复协议修改。
6. 组件名、字段、级别、trace/request上下文及现有安全字段保持；日志不能用作 actor/组织/授权/状态的事实来源。platform/helpers中的字符串标签“core/logger”不等于包依赖，不盲改日志标签。仅 §4 指定的敏感value参数删去。
7. 不以 variadic/旧构造器 wrapper 保住旧日志获取路径；必要技术签名变化同批更新生产、测试、独立debug与命令调用。业务方法、CLI flags、持久化和平台 wire 合同保持；新确认的实际外部可观察兼容义务按 §7 停止报告。

### 3.3 父目标剩余项的处置边界

| 剩余源码 | 本次核对结论 | 本批处置 |
| --- | --- | --- |
| core/logger及全部实际消费者 | 明确的forward-only兼容债务；当前行为已有Platform/Logrus owner | 本文完整RETIRE/消费接线提案，不只删两个文件或清几个import |
| core/config、仍被消费的crawler及source adapter | MIXED：包含现役配置/采集/解析/风险/浏览器行为。当前Logrus切换不授予替换其业务合同 | 本批只调整日志依赖；不据旧目录名、Legacy构造名或迁移文档整体删除；若后续继续退休须单独明确完整当前行为owner |
| localagent/httpapi、Prompt API、Amazon API、SHEIN/TEMU规则等零默认生产importer项 | 根模块零importer不是退休证据；独立debug、库/平台合同与显式KEEP必须一起核对 | 保留原能力/合同，不把这次日志迁移顺带写成取消采集/设备授权/平台API |
| platform/logging及其当前manager/测试 | 是有效当前基础层，原实现已经在正确owner | KEEP全部实现与安全/并发测试；只退旧转发门面 |

本批完成不能推导#301已全部完成，也不因此重开#29或新增运行/资格验收前置。

## 4. 配置、持久化与副作用 owner

| 边界 | 本批处置 | 明确保留 |
| --- | --- | --- |
| 日志 manager/registry、文件、轮转、hooks | Platform 现实现原位保留，appenv/runtime调用同一当前owner；不新建manager/输出生命周期 | manager/context/helpers、lazy/concurrency、默认不创建运行文件、原logger级别/格式及配置类型/默认/env |
| 业务日志获取 | 只换显式依赖/传参，按原位置/级别记录，不添加后台队列、重试、日志上传或第二配置源 | 同一业务输出/错误/取消/超时/并发/外部请求语义 |
| Prompt/敏感词 | 只传日志，复用现有WithLogger；不调用构造器读取当前文件或启动watch/save worker | 通用Prompt/租户覆盖/管理/既有文件语义；敏感词当前内存/只读/持久化选择、goroutine关闭与recovery |
| SHEIN/Store/采集/下载/安装 | 只改日志依赖。Config/secret/Profile/Redis/PG/绑定、attempt、fence、UNKNOWN、文件名/输出、浏览器持有者均不变 | 独立登录、Store同步、当前采集/来源、代理/风险/浏览器管理、API/规则、当前能力 |
| **TEMU SetCookieValue两条敏感日志** | client.go更新/新增Cookie两个分支当前把value传给Infof；同批精确改为仅记录Cookie名称/动作，删除value参数。用合成secret和内存log capture断言不输出value | Cookie选择、值赋写、请求头、返回、认证/平台副作用原样；不读取/处理真实Cookie，不新增脱敏框架/认证需求 |
| 当前身份/计费/SMS | 技术日志注入不改变HMAC/鉴权、provider_code选择或计费 | 不新发SMS/登录/AI请求、不增加授权/准入门禁，不把日志注入扩为权限或计费设计 |

## 5. 删除、保留、抽取与完整文件闭包

### 5.1 RETIRE / KEEP / EXTRACT

- **RETIRE**：internal/core/logger/compat.go、compat_test.go；internal/publishing/shein/logging.go 专属全局helper；sheinlogin_service 的包级全局捕获；Prompt的无显式日志构造包装和NewRegistry(nil)全局获取分支。原名称如存在有效当前业务含义，只去全局获取，复用已有显式构造，不保留桥接facade。
- **EXTRACT/REWIRE**：下表79文件里除专属helper外的78个直接消费者，把现有组件日志行为交给消费端Logrus值/窄回调及当前装配；没有需要从compat.go复制的新算法。其完整构造/子组件/调用者尾部一并更新。
- **KEEP**：platform/logging全部有效实现/类型/字段/测试及core/config；provider当前Logger接口/适配器和SDK/SSA约束；当前采集/图像/身份/计费/Product/Catalog/Asset/Store、有效平台API/规则/DTO、AI/prompt/PriceManager/PriceCalculator/MappingInfo/两种SHEIN属性/四零值JSON及规范化hash；独立SHEIN登录与Store同步。任何名字含logger/Task/legacy不是扩大删除理由。

### 5.2 已核对的17包/79直接生产文件

| 现有消费包 | 基线下准确文件 |
| --- | --- |
| internal/amazon/api（3） | auth.go; client.go; ratelimit.go |
| internal/app/httpapi（1） | zitadel_sms_handler.go |
| internal/crawler/alibaba1688（9） | browser_manager.go; captcha_handler.go; captcha_other.go; captcha_slider.go; page_operator.go; processor.go; product_checker.go; single_processor.go; url_helper.go |
| internal/crawler/alibaba1688/extractor（14） | attribute_extractor.go; basic_info_extractor.go; category_extractor.go; detail_extractor.go; image_extractor.go; pack_info_extractor.go; price_extractor.go; product_extractor.go; shipping_extractor.go; specification_extractor.go; supplier_extractor.go; title_extractor.go; variant_extractor.go; variant_values_extractor.go |
| internal/crawler/amazon/browser（17） | browser_pool.go; config_manager.go; currency_setter.go; health_checker.go; instance_manager.go; manager.go; pool_manager.go; proxy_pool.go; regional_prompt.go; zipcode_getter.go; zipcode_input_handler.go; zipcode_setter.go; zipcode_strategy_city_dropdown.go; zipcode_strategy_japanese.go; zipcode_strategy_standard.go; zipcode_utils.go; zipcode_validator.go |
| internal/crawler/amazon（6） | captcha_handler.go; failure_artifact_store.go; instance_processor.go; processor.go; product_checker.go; timeout_manager.go |
| internal/crawler/amazon/extractor（11） | availability_extractor.go; description_extractor.go; extractor.go; image_extractor.go; parent_asin_extractor.go; price_extractor.go; price_parser.go; price_validator.go; rating_extractor.go; title_extractor.go; video_extractor.go |
| internal/crawler/amazon/variations（2） | extractor.go; mapper.go |
| internal/crawler/shared/browser（6） | chrome_downloader.go; installer.go; ip_location.go; launcher.go; launcher_context.go; random_config.go |
| internal/localagent（1） | service.go |
| internal/pkg/appenv（1） | apputil.go |
| internal/prompt（3） | loader.go; registry.go; watcher.go |
| internal/publishing/shein（1） | logging.go |
| internal/shein/client（1） | error_handler.go |
| internal/shein/content（1） | word.go |
| internal/sheinlogin（1） | service.go |
| internal/temu/api/client（1） | client.go |

### 5.3 同批注入/调用尾部边界

清单是完整调用合同，不以79个import文件截断。以下现有owner的构造、私有依赖字段、实例方法/无状态函数参数、调用点和直接测试均属同批；只准改日志传递及§4两条敏感日志，不改其业务实现：

| owner/现有文件或包 | 必须贯通的当前路径与保留条件 |
| --- | --- |
| appenv/apputil.go | Setup/Apply改用当前Platform；为装配提供现有日志的组件getter。仅runtime使用，不形成业务可用的新facade |
| app/httpapi/zitadel_sms_handler.go、zitadel_sms.go、current_application.go | HTTP当前logger → module → handler；HMAC/私密配置读取、request body/状态/授权实现不变，测试用合成配置，不读真实文件 |
| app/runtime/sheinloginworker/runtime.go、sheinlogin/bootstrap/build.go、service.go、automation.go、automation_login_flow.go | worker现有logger → build → service/automation → shared browser。依赖走代码参数，不能加到私密Config/Store表或业务协议；原控制/fence/recovery全保留 |
| crawler/alibaba1688、extractor；crawler/amazon、browser、extractor、variations；crawler/shared/browser | 现有factory/processor/pool/manager/extractor及全部子构造/辅助函数贯通同一技术依赖；包括当前无直接logger import的manager/adapter和构造站点。采集、质量/zipcode、失败证据、安装/下载及浏览器策略不重构 |
| integration/crawler/a1688/processor.go、amazon/processor.go | 当前source adapter向现有crawler传日志；SourceEnvelope/DTO mapping/身份/地域策略/Shutdown合同保持 |
| localagent/service.go、runner.go及client/HTTP当前调用点 | 当前CLI/Runner调用采集器时传日志；不改内存任务/租约/安全校验/设备身份或接口合同，不顺带退休该能力 |
| cmd/1688-local-agent、1688-batch-import、1688-public-browser-collector、fingerprint-browser-installer | 保留CLI flags和现有调用，只将现有诊断logger传给采集/浏览器/安装器；不执行入口、不改workflow编译目标 |
| hack/debug/test-1688、test-amazon、test-real-track-slider、test-recorder | 独立模块实际构造站点同步技术签名；保留有效工具/模块文件，不执行、不增依赖、不处理ignored状态 |
| prompt/registry.go、loader.go、watcher.go；app/schema/productlisting/runtime.go及promptmgmt实际调用 | 复用现有WithLogger，registry把同一依赖传loader/watcher；当前Prompt全局注册/租户覆盖/缓存和持久化owner不变 |
| publishing/shein/assembler.go、marketplace/shein/draft/builder.go | 实例日志替代专属全局helper；草稿生成/规则接口和纯值输出不变 |
| publishing/shein/submit_prep_sensitive_adapter.go、shein/content 当前构造链 | 只传logger到保留敏感词服务，保留当前内存/只读选择及安全规则 |
| shein/client、shein/api/product/client.go、sheinlogin/session_identity.go | 错误处理/transport按现有对象获取日志；HTTP、认证、Cookie/Store语义不变，不串入新身份owner |
| amazon/api、temu/api/client、temu/api/api.go及product/query现有构造点 | 技术依赖贯通；具体不受影响的constructor不为形式修改。平台合同全保留，仅TEMU两条value参数例外 |
| 当前命令/runtime及上述包的直接原测试 | 只校准受日志签名/注入影响的fixture；生产根必须显式注入，不用测试no-op冒充当前运行接线 |

可在上述消费包内新增最小私有logger_port.go及有实际断言价值的原测试增量；不新增shared日志协议、adapter平台、DTO/config持久化字段或统一治理工具。调查时还定位19个跨包构造候选（含部分不需改动者）和14个跨包测试调用候选，另有消费包内原fixture；实现按真实日志依赖收敛，最终实际文件清单写PR。只改编译错误不等于完成，必须沿上述运行入口检查实际注入；若发现表外owner/新的领域边界或必须改冻结KEEP，先报告，不以“完整闭包”授权无关重构。

```text
Legacy decision: EXTRACT | RETIRE
Reusable behavior: 有效Logrus组件日志交消费端既有owner，runtime/Platform继续提供manager/配置/输出；不复制facade
Current owner: 原业务消费端 + 当前app/runtime/CLI装配 + platform/logging；SHEIN/Store原owner
Cutover/deletion condition: §1–§7/§6.1获复核和用户确认/Ready；全17包及上/下游日志接线切换，旧facade/helper/全局获取退出，原检查有效，无运行资源操作
```

## 6. 必要验证与证据

设计阶段只读源码/图、矩阵和依赖元数据，跑原文档/索引检查及稳定候选原PR CI，核对真实设计diff。源码未授权前不写正式port/helper、不开真实服务、不以设计CI/评审代替实施/运行证明。

确认后：原退休/导入guard先RED捕获旧根/helper/全部默认和build-tag/debug导入及SHEIN全局捕获，再整批切换得GREEN。日志依赖选择、配置继承与Cookie敏感参数用原测试先捕获目标差异；复用Logrus/标准库内存sink、现有fake/mock、任务临时目录，不建runner/验收平台。仅覆盖真实新风险：当前factory被用、component/级别/字段和原raw logger引用、nil禁用依赖不触发全局初始化/文件输出、两TEMU敏感值不进日志；不为79文件制造79套重复测试。

保留原Platform singleton/lazy/concurrency/输出/context/13字段测试；只删纯compat身份相等测试，不删有效日志行为测试。复用现有provider logger/SDK/SSA、Phase2导入方向/非增长ceilings/CURRENT准确性、#868数字字段禁入及退休/文档检查。业务自检只跑受影响的原纯规则/合成/loopback测试；不运行有真实浏览器、下载、安装、provider、文件持久化或PG/Redis前置的旧构造器。需要文件时只用任务自有临时目录，未授权真实运行保留NOT_RUN。三个当前入口、相关CLI和debug模块做适用编译/依赖元数据，不执行产物。稳定候选交原完整CI，不重复人工全量矩阵。

最终交付检查一次完整diff/真实调用路径/保留能力/证据，修复只复核增量，finding按现有规则先分类。准确HEAD、原CI、复核和批准留PR；不把未变化证据冒充新SHA直接运行。合并与实际merge SHA push/main确认另授权；运行/用户验收仍由用户或指定验证者决定。

### 6.1 原检查、CURRENT inventory、文档与workflow精确处置

以下为用户确认的**同批准确实施矩阵**；CURRENT 严格 17 删/1 增，其它对象按下列边界执行：

| 对象 | 同批准确处置 | 不得改变 |
| --- | --- | --- |
| tests/listingkit_backend_retirement_test.go | 在原AST/decoded-import设施加core/logger整根及专属helper源缺席/全追踪导入禁入；保护业务不通过platform/logging/appenv取新全局门面、SHEIN包级捕获退出、当前入口旧依赖退出 | 原所有退休/KEEP约束、错误必须fail；不missing-skip、不加dummy或旧alias |
| 同文件 TestNumericContextAndValueContractsAreRetired | 对compat.go“必须读文件且不得有旧数字字段”转为整个logger根缺席/禁止全部消费者；可复用本批helper | platform/logging/helpers.go 原数字字段/Task/Store helper禁止及所有其它值/hash/规则断言原样保持；不能只忽略文件不存在 |
| 敏感词 adapter 的原 import/退休检查（用户已另行确认的准确例外） | 仅 TestPublishingSheinSubmitPrepUsesOnlySensitiveWordAdapter 允许 content 与 Logrus 类型、恰好一个 func(string) *logrus.Entry 参数且原样转交；tests/listingkit_backend_retirement_test.go 两项原退休 KEEP token 从零参数改为 (logs) | 仍强制直接返回原 NewSensitiveWordServiceInMemory；不允许其它依赖/参数/逻辑/全局获取/持久化选择，所有其它检查与生产行为不变 |
| 原integration_provider_logger_contract、s3、SDK/SSA/phase2/import检查 | 除上行敏感词 adapter 的技术签名准确例外外，保持实现/原禁止列表；运行适用原检查证明新接线不新增concrete依赖、provider或隐藏global getter | 不改logger method-set/constructor inventory、SSA/decoded parser/ceilings或SDK豁免，不为过CI改规则 |
| 原日志/各消费包测试 | 使用现有Logrus值/fake和临时sink，校准技术构造参数，必要添加上述真实行为断言；pure compat测试随根退出 | 不删有效业务/安全/隔离/日志行为测试，不运行真实外部入口 |
| .golangci.yml / Code Health / deps | **零改**；退根由原Go guard加强，现有Platform/业务方向和provider规则继续约束 | 无lint豁免/新允许项、无升级/tidy/锁变化、无ceiling或accuracy逻辑调整 |
| CURRENT inventory | 仅下方17删/1增，逐行精确更新实际消费者关系 | 整个其它文本、历史计数/表、ceilings/映射和检查逻辑不改，不能删除仍存在core/config或workerpool行 |
| Project Boundaries 当前段、Architecture README/Legacy Register及本文 | 记录新确认的facade退出/消费端技术依赖和runtime owner；Phase2历史facade说明加窄supersession，不改历史原文 | 其它owner、当前产品/安全authority及历史登记行不变；新logger端口不是扩大legacy权限 |
| phase2-runtime-inventory / current-refactoring-status 相关历史适用 | 只有上方CURRENT行编辑；历史迁移债务由本文/当前索引窄supersession说明 | 不重写历史状态、计数、PASS/失败或新增运行验收条件 |
| worker/CLI/debug/config/部署 | 只§5.3明确的代码日志参数/签名与直接fixture | 不改命令目标/flags、私密JSON/config/schema/env、部署样例和依赖，不能操作运行资源 |
| 全部 workflow/保护规则 | **零改**，原Backend/Frontend/Required/Security/Admission/CodeHealth均保留 | 不删/跳过检查，不强推/管理员绕过 |

准确CURRENT删除：下面16条以 core/logger 为target的行，以及一条 platform/logging → core/logger。新增且仅新增一条 platform/logging → pkg/appenv。app/httpapi属于现有有意装配排除，不新增业务→Platform消费者；其日志从当前HTTP/runtime传入。

```text
DELETE target task-processor/internal/core/logger:
  task-processor/internal/amazon/api
  task-processor/internal/crawler/alibaba1688
  task-processor/internal/crawler/alibaba1688/extractor
  task-processor/internal/crawler/amazon
  task-processor/internal/crawler/amazon/browser
  task-processor/internal/crawler/amazon/extractor
  task-processor/internal/crawler/amazon/variations
  task-processor/internal/crawler/shared/browser
  task-processor/internal/localagent
  task-processor/internal/pkg/appenv
  task-processor/internal/prompt
  task-processor/internal/publishing/shein
  task-processor/internal/shein/client
  task-processor/internal/shein/content
  task-processor/internal/sheinlogin
  task-processor/internal/temu/api/client
DELETE:
| `task-processor/internal/platform/logging` | `task-processor/internal/core/logger` | compatibility facade | owning domains / Phases 3-8 |
ADD:
| `task-processor/internal/platform/logging` | `task-processor/internal/pkg/appenv` | runtime-support wiring | app runtime / current logging owner |
```

Platform直接内部importer保持原有3个总量：移除core/logger、加入appenv，其它仍为core/config及platform/workerpool，未提高历史ceiling9；新appenv关系必须登记，不能以“runtime helper”名字擅自扩大排除规则。core/logger ceiling82和其它历史值不改；归零由精确退休guard保护，不靠改历史counter。

## 7. 准入、停止与父目标关闭边界

1. 冻结设计 cace5c1f4 的独立复核/原 CI 已完成，用户确认具体§1–§7/§6.1（含17删/1增及两条Cookie值日志）；#301/PR Reuse Existing Architecture / Ready已读回后才进入同一PR整批实施。设计/旧批次证据不替代准确实现 HEAD 的原 CI、最终交付检查或合并授权。
2. 同一Writer连续完成，形成稳定最终候选前核对实际main与消费者；不为保持最新持续merge-main或重复全量评审。对已确认未变化证据明确复用，不冒充新HEAD直接执行。
3. 若真实消费者需要当前日志之外的新管理/配置/持久化/恢复/权限/平台副作用，或需要表外owner/冻结KEEP变更，先报告具体当前用户操作、最小改动和成本；不把旧facade搬家或用global setter/fallback“解决编译”。
4. 若仅是表中owner内的纯日志传参/fixture细化，在同批完成并维护实际清单；发现新的外部可观察兼容义务或当前Must的真实BLOCKER时停止对应部分并报告。不能由Reviewer自行新增compatibility、通用框架、运行验收矩阵或试用前置。
5. 原CI失败先读准确日志、分类并查同根因兄弟路径；非BLOCKER在实现/测试收敛，不能改workflow、历史ceilings/accuracy或扩大敏感日志/业务改造范围绕过。矩阵外必要修订先给精确提案，不能自行改冻结KEEP。
6. 源级退休不等于物理日志/旧文件/会话/数据清理、provider资格或运行验收；它们UNKNOWN/NOT_RUN。#301保持OPEN，#29不重开。每次更新前新读Issue，只改当前段，63542字符历史后缀及勾选逐字保留、正文不超过65536。合并、部署和关单另行授权。

