> #301 / #937 **USER_CONFIRMED**：[五组闲置支持门面完整退休闭包](legacy-idle-support-facade-closure-retirement-boundary.md) §1–§7（含§6.1）已获用户确认，Reuse Existing Architecture / Ready与Issue/PR读回6386195192/6386203704先于源码修改。仅替代闲置httpclient.ClientConfig/Build/buildTLSConfig、builder两文件及NewSimple的KEEP。旧平台消费者已退出；当前Client/Config/New/Do/NewSimpleWithTimeout、TLS/retry实际consumer、JPEG/PNG/WebP生产解码及其它当前能力保持。只替代准确旧KEEP；以下完整原Git正文/既有顶注/数字/勾选/批准/历史证据逐字保留。workflow/完整inventory/依赖/保护与既有实例/数据/凭据/进程零改，不增试用前置，保留独立SHEIN登录与Store同步，#301 OPEN。

# #301 旧宿主、HTTP 与图像辅助完整退休闭包设计

Refs [#301](https://github.com/qq550723504/shuomi-ai-commerce/issues/301)。#301 保持 OPEN，#29 不重开。

- Status: **USER_CONFIRMED / SOURCE_IMPLEMENTED**。用户已确认准确设计 HEAD `156162acf813cb3d49baf0f8dc10938661914777` 的 §1–§7（含 §6.1），授权同一 #915 整批实施。当前待原 CI 与最终交付复核，不代表产品验收或合并授权；准确实现 HEAD/证据留主要 PR。
- Design Basis：实施 **Reuse Existing Architecture / Ready**；[接单与 Ready 准入6368596913](https://github.com/qq550723504/shuomi-ai-commerce/issues/301#issuecomment-6368596913)及 Issue/PR 读回先于源码修改。原纯文档设计 N/A 的 CI/只读复核证据留 #915，不冒充实现证据；合并与运行验收仍需分别授权。
- 调查基线：main **de6b2cb349e9b1398269674b91a94d2d902fc96f**，tree **25ed7530cd1237df675647cd8e92be4aa6c54fa4**。前批 #914 的准确 push/main CI39965667687 整 run SUCCESS，仅作基线证据。
- 唯一 Writer 沿原独占 worktree，分支 `codex/issue301-host-image-helper-closure-design`；不新增 Writer，不按包、文件或检查机械拆批。

## 1. 用户结果、产品依据与完整范围

使用者是维护当前采集、HTTP、图像、登录和 Store 源码的维护者。结果是一份四组完整删除/保留矩阵，清除无当前需求与消费者、仍被历史 KEEP 保活的辅助分支，同时保留同包真实行为。没有当前核心用户操作因此阻塞；本次整理不增加内部试用前置，不建设新的日志、HTTP 或图片工具产品。

一个 Delivery Batch / 一个主要 PR 覆盖：① appenv 旧日志配置/实例/平台辅助；② HTTP 两项孤立 helper；③ imagex 旧编码/格式辅助及完整专属类型、常量和测试调用尾部；④ 孤立 ServerConfig。原退休 guard 和直接冲突的六份旧 KEEP 文档同批收齐。设计阶段仅交付本文及两个索引；用户确认后的实施严格限 §6.1 的十五工件。

Product Authority 为本轮用户明确范围、[Greenfield 决定](../product/greenfield-no-legacy-migration.md)、[Hard-Cut Policy](../refactoring/legacy-hard-cut-policy.md)、[Legacy Register](../refactoring/legacy-register.md)、[Issue 驱动规则](../engineering/issue-driven-development.md) 和当前消费 owner。UI/IA/Figma N/A，不改页面/交互。既有代码、历史 KEEP 和旧测试不自动构成当前产品需求；新矩阵经确认后才精确替代 §2 指定对象。

Must：四组完整闭包；当前日志注入/版本输出、HTTP/TLS/重试、图像解码/metadata 与 JPEG/PNG/WebP 注册不减；保留采集、原浏览器/debug、平台 API/DTO/有效规则、共享合同及独立 SHEIN 登录/Store 同步。Should：复用原检查、标准库和现役组件，无新 EXTRACT/library/wrapper。Out of Scope：其它业务退休、搬包、统一配置/HTTP/图像框架、provider 调用、凭据/数据迁移/兼容、workflow/inventory/依赖/锁/保护规则变化、真实实例/数据/凭据/进程操作。Accepted Risks 无新增；既有真实环境状态 UNKNOWN，运行/产品验收 NOT_RUN。

Threat Model 限误删混合文件的当前行为、同名符号全局禁词、因删编码 imports 丢失生产解码注册、测试导入 decoder 掩盖注册缺陷、误改请求重试/TLS 或现有 owner。此前被用户撤回的“全部 Cookie API 退休”不恢复。

## 2. 根因、Design Basis 与旧 KEEP 替代

旧宿主与数字刊登消费者已经退出，包级广泛 KEEP 和一条声明存在性检查仍保留其闲置尾部。图像包还存在隐式依赖：`image/jpeg`、`image/png` 的导入既支持旧编码函数，也执行 `image.Decode` 所需的注册。根因处理是删除准确旧分支并显式保留注册，不为 helper 安排新消费者、复制算法或整体删包。

| 前批依据 | 本次获确认后拟替代的准确对象 | 仍保持的边界 |
| --- | --- | --- |
| [TEMU Store init](legacy-temu-store-init-retirement-boundary.md) | appenv.GetInstanceID 的独立消费者/KEEP 及其原存在性 token；现在仅剩包内实例链 | 其它同名 GetInstanceInfo/GetInstanceID、Store/API/身份与原断言不动 |
| [TEMU 图片处理](legacy-temu-image-processing-retirement-boundary.md) | imagex 宽泛编码/辅助 KEEP 中 §5 的闲置声明 | 当前解码、图像处理/填充/上传、平台合同和有效规则不减 |
| [水印退休](legacy-watermark-config-closure-retirement-boundary.md) | imagex 整体暂留中 §5 的闲置部分 | 成熟图像库、当前风险/provider 参数、ImageAgent 与有效共享层保留 |
| [Amazon 刊登整链](legacy-amazon-listing-closure-retirement-boundary.md) | imagex/httpclient 广泛 KEEP 中本矩阵的孤立声明 | 当前 API、原浏览器/Processor/debug、共享合同不动 |
| [预览与下载退休](legacy-listing-preview-download-closure-retirement-boundary.md) | 共享编码/辅助保留中本矩阵的闲置部分 | 当前 HTTP/TLS、解码/metadata、Product/Asset owner 保留 |
| [宿主运行层](legacy-host-runtime-closure-retirement-boundary.md) | 共享辅助暂留中本矩阵的闲置宿主/HTTP/ServerConfig 分支 | 当前 HTTP server/health/ready、日志、HTTP/Retry 与其它能力保持 |

拟实施分类为 **Reuse Existing Architecture**：这是现有辅助层在原 owner 内的有界 Hard-Cut，没有新增能力、领域合同或 owner。调用链见 §3；权限/租户、事实、状态、事务、幂等、retry/UNKNOWN、恢复和外部副作用均不改变。四个生产工件仅删除准确声明、整理专属 imports/comment，并保留 decoder 注册；无新生产实现。原 HTTP server 是 `app/httpapi` 构造的 `net/http.Server`，无需从孤立类型抽出另一配置源。

本次不跨三个独立业务子系统、不跨持久化一致性边界；十五个拟实施工件未命中文件阈值，没有新增生产行数/状态/权限/事实 owner 风险。若实际发现当前消费者、外部可观察合同或持久化要求，停止相关删除，报告最小缺口，必要时升级 Independent Architecture；不得在实现中发明 fallback/迁移。

## 3. 实际消费者与当前调用链

核对范围包括全部 tracked Go、根模块与 `hack/debug` 独立 module、导入 alias、包内互调、原测试及文档/脚本引用；默认 cmd 零 importer 本身不作为退休理由。

| 工件/分支 | 当前真实消费者或仅剩引用 | 结论 |
| --- | --- | --- |
| appenv.SetupLogger、LoggingConfig、ApplyLoggingConfig | 无生产/debug 调用或构造；SetupLogger 仅转发活跃的 SetupLoggerWithLevel | 精确 RETIRE；不是删除 core/config.LoggingConfig 或 platform/logging |
| appenv.InstanceInfo/GetInstanceID/GetPodName/GetNamespace/GetInstanceInfo | 仅本包实例链互调；GetInstanceID 另被原退休 guard 和旧 KEEP 文档保活 | 连完整宿主实例链 RETIRE，不读取主机/env 来验收 |
| appenv.ParsePlatformList/ContainsPlatform | 声明外无 Go 调用 | RETIRE；不删除当前平台枚举/API/规则 |
| appenv.ComponentLoggers | 当前 HTTP/runtime、1688 agent、installer、SHEIN runtime、原 browser/debug 等装配传入组件日志回调 | KEEP 原 getter/注入，不让业务重新查全局 manager |
| appenv.SetupLoggerWithLevel | image-agent-temporal-worker、shein-login-worker、listingkitschemamigrate | 原原始 Logrus/级别行为 KEEP |
| appenv.VersionInfo/PrintVersionInfo | listingkitschemamigrate 原版本输出 | KEEP 原字段、日志和启动路径 |
| httpclient.DefaultConfig/NewWithTransport | 无包内/包外生产、debug 或原测试调用 | 两 helper RETIRE；不全局禁用同名方法 |
| httpclient.Config/Client/New/Client.Do | shared/browser.ChromeDownloader 明确构造并执行下载请求 | 原 timeout/default/retry/context/response/log 行为 KEEP |
| httpclient.NewSimple/NewSimpleWithTimeout | Amazon API auth/client；shared/browser.ip_location | 原标准 http.Client 返回值与超时保持 |
| httpclient.ClientConfig/Build/buildTLSConfig | TEMU API client 的 req/v3 接线；原 TLS/request 测试 | 完整 KEEP，仍复用现役组件，不更换 HTTP 库 |
| imagex 的 §5 闲置辅助 | 生产/debug 无包外调用；ToBase64 两 wrapper、FromBase64→FromBytes、OptimalFormat→HasTransparency 为包内闭包；Size 仅原 WebP 测试调用 | 同批 RETIRE，转换该测试调用，不删有效断言 |
| imagex.ImageInfo/FromBytesWithFormat/Inspect | SHEIN api/image 直接解码；ImageAgent generation_output/runtime、Temporal artifacts 和 DurableArtifactStore 检查字节/metadata，原 objectstore 测试用 ImageInfo | KEEP 原 fields/types/错误/数值；解码依赖实际生产注册 |
| core/config.ServerConfig | 仅 type_server.go 声明；根 Config 无字段、无 loader/default/env/配置样例或生产/debug/test 构造消费 | 整文件 RETIRE；当前 HTTP server/timeout 不受此类型控制 |

原保留路径：`current-application → app/httpapi → http.Server`；`image-agent-temporal-worker → app/worker/imageagent + imageagent/temporal/objectstore → imagex.Inspect`；`shein/api/image → imagex.FromBytesWithFormat`；原 Amazon browser/debug → shared/browser.ChromeDownloader/Processor。独立登录仍为 `cmd/shein-login-worker → app/runtime/sheinloginworker → sheinlogin/bootstrap/worker`，Store 持久化归 `storecenter`，不恢复旧 managed/digital owner。

## 4. 配置、持久化与副作用 owner

| 边界 | 原 owner 与本次不变量 |
| --- | --- |
| 旧实例/平台 helper | 仅读取 os.Hostname/POD_NAME/POD_NAMESPACE 或纯字符串，无 durable 事实；删除函数不授权读取/修改现有环境、Pod、文件或进程 |
| 日志 | appenv 只留当前装配 helper；platform/logging 与 core/config.LoggingConfig、日志文件/hook/rotation 及当前注入保持。删除无调用 ApplyLoggingConfig 不执行重配置，也不新增日志 owner |
| HTTP | 原 net/http/req client/平台调用方仍持有 timeout、重试、TLS 和请求副作用。只删无 caller 的两个 helper；不改变错误/响应丢失/取消/重试语义，不发真实请求 |
| 图像 | ImageInfo 与 decoder 是内存值/解析；ArtifactStore、ApprovedAsset、SHEIN 官方 effect/receipt/UNKNOWN 各守原 owner。无新编码、文件写入、上传、批准或资产迁移 |
| ServerConfig | 不是根配置的持久化输入；删除不改变当前 Viper/加载/环境覆盖/schema 或 app/httpapi server timeout，不新增配置拒绝/兼容行为 |
| 独立 worker/Store | 原授权、PG/Redis/attempt/Profile/持久化、租户隔离、fence、UNKNOWN 与 Store 同步均零改。无自动接管、凭据复制、旧 ID 映射或第二事实源 |

没有新事务、状态机、恢复协议或外部 mutation；原失败/响应丢失/重试/重启/取消/并发结果不变。本批不取消 #301 其它验收或把源码缺席写成运行/产品 PASS；#301 保持 OPEN。

## 5. 删除、保留与抽取矩阵

### 5.1 四个生产工件

| 文件 | 精确 RETIRE | 必须 KEEP |
| --- | --- | --- |
| internal/pkg/appenv/apputil.go | SetupLogger；LoggingConfig/ApplyLoggingConfig；InstanceInfo/GetInstanceID/GetPodName/GetNamespace/GetInstanceInfo；ParsePlatformList/ContainsPlatform；专属 os/strings imports、注释/旧 package 描述 | ComponentLoggers、SetupLoggerWithLevel、VersionInfo/PrintVersionInfo 的完整实现/字段/值，现役日志 imports；不改 logging_test.go |
| internal/pkg/httpclient/httpclient.go | DefaultConfig、NewWithTransport 与专属注释 | Config/Client/New/Client.Do/NewSimple/NewSimpleWithTimeout 全部实现/字段/默认值/日志/context/重试保持；builder.go/builder_test.go 逐字不变 |
| internal/pkg/imagex/imageutil.go | Format；FormatJPEG/FormatPNG/FormatAuto；ToBase64/ToBase64JPEG/ToBase64PNG/FromBase64/FromBytes/ToBytes/FromReader/Encode/Size/FormatFromBytes/HasTransparency/OptimalFormat 及专属 imports/comment | ImageInfo 的 Width/Height/Format/Size 原字段类型；FromBytesWithFormat、Inspect 完整实现/错误/metadata；生产解码注册见 §5.2 |
| internal/core/config/type_server.go | 整文件 ServerConfig | 根 Config、HTTPClientConfig/RetryConfig/LoggingConfig、当前 server 构造/timeout/所有实际配置保持，原 type_rabbitmq.go 实际承载通用 Retry，不因文件名删除 |

EXTRACT：无新业务实现或替代工具。活跃行为已在正确 owner；删除无消费者的编码 helper 不等于取消平台/当前产品的编码/填充/上传能力，其各自现役实现不在删除矩阵。不是全仓 Size/Format/LoggingConfig/ServerConfig/NewWithTransport 禁词；不得删 ImageInfo.Size/Format 或 acquisition/a1688 的同名受控注入方法。

### 5.2 图像解码注册是保留合同

删除 jpeg/png.Encode 调用后，**将生产 imageutil.go 的 image/jpeg、image/png imports 改为显式空白导入，保留已有 golang.org/x/image/webp 空白导入**。三项注册仍由同一生产包初始化负责；不依赖测试、其它偶然 import 或调用方帮它注册，不新增注册器/library/Init。

当前 FromBytesWithFormat 对空输入/坏图像的拒绝，Inspect 的 Width/Height/Format/Size=len(data) 均不变；不扩 GIF 等新格式，不改内容检测/像素上限/当前图像风险策略。删除 FormatFromBytes 的 GIF 魔数分支不是删除当前已开放 GIF 解码能力的证据或授权。

## 6. 实施顺序与必要验证

仅设计确认后同一主要 PR 实施：先写 Ready 并读回；扩展 §6.1 原退休 guard，在旧声明仍存在时确认准确 RED；再一次退休四源码工件、转换原图像测试并收齐六旧依据顶注至 GREEN。不要为未变化的业务规则制造替代实现或形式化 RED。

保留混合文件的完整当前函数/字段/默认值；复用现有 diff/AST 检查确认 KEEP 代码语义不变、禁止项零变更、六旧文档原正文精确 suffix。设计阶段仅改三文档；获确认后按 §6.1 转换原检查/测试，不运行 constructor/main/浏览器或访问真实服务。

实施必要自检：原两项退休 guard 与 Architecture README 四项检查；appenv 原装配 getter 测试；httpclient 原 TLS/请求设置回归（仅原 loopback fixture）；imagex 注册/解码回归和原 ImageAgent/objectstore/平台图像适用测试；原 ChromeDownloader/installer timeout、独立 SHEIN 登录与 Store 的适用合成 fixture 原测试。只执行与改动直接相关的选定原测试，不手工重复原完整 CI。

必要入口仅编译：current-application、image-agent-temporal-worker、shein-login-worker、product-listing-api-schema-migrate、1688-local-agent、1688-batch-import 及 hack/debug 独立 module 的原 test-amazon；不执行产物。开发自检只用任务自有临时文件、原内存/loopback fixture，不新建 PG/Redis/容器/浏览器环境或读取既有配置/凭据；依赖真实条件者如实 NOT_RUN。稳定候选交原必需 CI。

### 6.1 原检查转换与完整文档尾部

确认后的整批实施为 **15 工件**：§5.1 四生产文件 + 一个原图像测试文件 + 一个原退休 guard 文件 + 下列六旧文档 + 本文/README/Legacy Register 三当前文档。不制造空壳或新 guard/test harness；未列检查零改。

| 文件/原函数 | 唯一允许的准确变更 | 必须保持 |
| --- | --- | --- |
| tests/listingkit_backend_retirement_test.go / TestLegacyRuntimeSkeletonAndIdleConfigClosureIsRetired | 加 type_server.go 的 fail-closed Lstat 缺席；core/config 仅加 ServerConfig 类型禁入；沿原 owner 限定 TypeSpec/FuncDecl/ValueSpec 扫描增加 appenv/httpclient/imagex 及 §5 全部精确退休声明；增加原当前 helper/ImageInfo 字段/decoder 保留断言 | 原 kernel/HTTP/config/日志/timeout/goroutine/jsonx 的全部声明、字段、decoded import、解析/读取失败、原缺席与 KEEP 断言；不全局禁词或禁用三个仍活跃包 |
| 同文件 / TestListingKitTaskBackendSourcesAreRetired | 仅将 appenv/apputil.go 的 GetInstanceID 存在性 token 换为当前 ComponentLoggers/SetupLoggerWithLevel/VersionInfo/PrintVersionInfo 的准确声明保护 | 全部其它文件/平台/Store/安全/退休断言不变；同名别的 owner 不受影响 |
| internal/pkg/imagex/imageutil_test.go | 原 TestFromBytesWithFormat_DecodesWebP 的 Size(img) 改为直接用 img.Bounds() 验证尺寸，保留原 WebP fixture/解码/格式/非空/正尺寸断言；同文件补 JPEG/PNG 固定合成字节的 FromBytesWithFormat/Inspect 正向与 metadata 回归 | 测试不得导入 image/jpeg、image/png 或 webp 包替生产注册，不使用编码器动态造图来掩盖缺失注册；只用本地合成常量，所有当前解码行为保持 |

原退休 guard 对 imageutil.go 沿现有 AST/decoded-import 能力确认三个准确 decoder imports 在生产文件且 alias 为 `_`；失败不 skip。原活跃 HTTP/日志 helper、ImageInfo 四字段与两个解码方法在 owner 内正向保留。HTTP/Retry 其它配置同名类型、current server 和 acquisition/a1688.NewWithTransport 原样；不删除检查以换绿。

六旧文档只在首部加窄 superseding 注记，链接本文并精确替代 §2 对象，原全部正文逐字作为后缀保留：

1. legacy-temu-store-init-retirement-boundary.md
2. legacy-temu-image-processing-retirement-boundary.md
3. legacy-watermark-config-closure-retirement-boundary.md
4. legacy-amazon-listing-closure-retirement-boundary.md
5. legacy-listing-preview-download-closure-retirement-boundary.md
6. legacy-host-runtime-closure-retirement-boundary.md

README/Legacy Register 只维护本批当前入口/状态，原正文/历史/计数保留。无需修改旧 Phase2 inventory：当前 logging→core/config、logging→pkg/appenv、openai→core/config、httpimage→imageagent 四行仍真实；**workflow/完整 inventory/CURRENT/历史/计数/ceilings 及 inventory 检查逻辑均零改**。根包 guard 只允许上表两函数修改，其它函数逐字保持；其余测试除指定 imageutil_test.go 外不改。

若原 CI 暴露表外孤立尾部或冻结 KEEP 冲突，先给具体失败、真实消费者和准确最小修订，取得用户决定后再应用，不能借“完整闭包”扩大范围或绕过原门禁。

## 7. 交接、复核与停止条件

### 7.1 设计交付

设计阶段仅本文、Architecture README 和 Legacy Register 三文档。完成原适用文档检查、准确设计 HEAD 原 CI 与一次适用独立只读检查；记录实际审核 SHA/结论/finding 分类，设计交用户确认。生产代码/原测试实施前仍须用户确认 §1–§7（含 §6.1），再记录拟实施 Reuse Existing Architecture / Ready 并读回，不用纯文档 N/A 授权生产删除。

### 7.2 实施与证据

实现期先分类 finding，再在同一 PR 收敛；非 Blocker 不重开全局架构。最终交付检查完整真实 diff、同名/兄弟消费路径、生产 decoder 注册、保留合同和现有证据；修复后只复核对应增量，未变代码证据明确复用。不以源码缺席、可编译、CI 或 AI 报告签发产品验收。

### 7.3 评审与权限边界

同一 HEAD 不重复触发正在运行的 GitHub 复核；只有明确启动失败才作一次受控重试，保留失败/UNKNOWN，不把空 reviews 或 Completed 标签当实际 PASS。**未经用户本轮授权不默认新增 AI 互验线程**；旧 #914 子代理授权不覆盖本设计/源码。备用只读设计或最终复核需给准确 HEAD、只读范围和现有失败证据，先交用户决定，不自行启动新 Reviewer/Writer。

设计确认、生产实施、合并、运行/产品验收分开记录；合并/部署、既有实例/数据/凭据/进程操作和 #301 关单 NOT_AUTHORIZED。本批不加试用前置，#301 保持 OPEN。若后续获准确 HEAD 正常合并授权，只用实际 merge SHA 的 event=push/head_branch=main 整 run completed/SUCCESS 确认，不能以 PR CI、单独 Gate 或其它 main SHA 替代。
