# 工具市场 v1 运行接线交接

执行Issue #613；设计依据 [IMPLEMENTATION_READY](../architecture/tool-market-v1.md)。
本批唯一Writer交独立模块；协调方指定共享runtime Writer接导航/权限/当前安装组合。
本文件不表示共享安装已经开放或用户验收通过。

## 唯一注入路径

1. fresh空安装由schema owner显式调用
   `toolmarketpersistence.InstallSchema(db)`，或按同等owner流程安装内嵌schema.sql。
   拒绝既有schema，不迁移/覆盖。runtime startup仅调用`VerifySchema(ctx, db)`和`New(db)`。
2. 复用当前组织resolver、现有`*authz.ListingKitAuthorizer`：
   `toolmarketauth.New(resolver, authorizer)`；设置`Handler.Bind=a.Bind`和
   `Handler.Authorize=a.Authorize`。不得替换成基于浏览器权限数组或缓存角色的闭包。
   企业在每次授权及事务回调使用当前LiveWrite；manage额外限制listingkit_admin。
   平台verified-roles路由仅检查当前未过期平台identity与现有platform_admin策略，
   不传客户org，不要求客户成员关系。
3. 现有Casbin初始化纳入`authz.ToolMarketPolicies()`；WorkbenchPermissions纳入
   tools.read/manage/customize。既有受保护viewer获得read，operator获得read/customize，
   listingkit_admin获得三项。自定义模块只能纳入read/customize，不能manage或platform_admin。
   `tools`和`tools-custom`模块目录及当前菜单可用性由共享Writer一次接线，不新建角色体系。
4. 从当前已注入、实际开放的采集receiver与online模块推导`Handler.Readiness`。
   enterprise启用不是授权或能力存在的依据。下载仅在local receiver开放且包验证通过时可用。
   数据结果继续走当前`/workbench/data/mine`，保持现有collections安装门控。
5. 通过`httpapi.NewModule(handler)`注册；必须纳入当前application路由准入，复用
   `httpapi.ValidateDescriptor`，不能以任意通用route覆盖策略。缺依赖拒绝构建。
   模块企业Base `/api/v1/workbench/tool-market`；平台Base `/api/v1/admin/tool-market`。
   不得用普通CurrentIdentity替代平台CurrentIdentityWithVerifiedRoles。

```go
// Existing current-application composition performs all dependency validation.
repo, err := toolmarketpersistence.New(db)
// check err and VerifySchema(ctx, db)
admission, err := toolmarketauth.New(resolver, authorizer)
// check err
h := &toolmarkethttp.Handler{
    Repository: repo, Bind: admission.Bind, Authorize: admission.Authorize,
    Readiness: toolmarket.Readiness{LocalCapture: receiverReady, OnlineCapture: onlineReady},
}
// optional package is validated below; failure leaves download unavailable
m, err := toolmarkethttp.NewModule(h)
// check err, register m through current application and descriptor admission
```

runtime角色只需schema USAGE、activations SELECT/INSERT/UPDATE、requests SELECT/INSERT/UPDATE、
events SELECT/INSERT、commands SELECT/INSERT/UPDATE；无schema CREATE/DROP/ALTER或owner成员权。
所有新事实在同一个PG pool，原有Product/Commercial库和采集计价合同不变。

## 插件包

已有扩展依赖先按extensions/1688-capture文档安装。实际安装须使用自己的已验证HTTPS接收地址：

```powershell
./scripts/build-tool-market-plugin.ps1 -CaptureAppUrl 'https://你的应用域名/capture/1688' -OutputDirectory './.local/tool-market-release'
```

脚本只调用已有非fixture build和标准Compress-Archive，不创建第二采集器。
将zip、release.json及当前安装的完整CAPTURE_APP_URL作为同一次可信发布记录交接；
runtime读取record的sha256，将**当前安装接收地址**传入`Handler.ConfigurePackage`的PackageConfig。
不能仅凭manifest的hostname绑定地址，也不能把客户端参数作为地址/路径/sha来源。
当前安装地址与record.captureAppUrl必须完全相同，否则不得加载包。
ConfigurePackage失败保持下载不可用并显示当前安装未配置插件包。
Handler不暴露原始包bytes注入口，ConfigurePackage内部调用LoadPackage校验后保存。
本批开发检查用测试HTTPS地址编译，没有把测试包交付为用户安装包。

## 消费者和用户操作

- `/workbench/tools/official`：分类浏览；管理员启用商品采集插件；七项开发中工具无法启用。
- `/workbench/tools/mine`：企业真实清单；插件下载/安装步骤、当前1688在线采集和我的数据入口。
  停用仅改变清单，不撤销插件或既有采集。
- `/workbench/tools/custom`：提交需求、查询原需求和专员进度；线下报价付款。
- `/workbench/tools/custom/review`：硕米专员处理；平台API独立授权，无客户企业选择要求。
- 专属BFF `/api/tool-market/[...path]`使用既有serverAuth及服务端token，
  `LISTINGKIT_SERVICE_API_BASE`和可信公共Origin沿用当前安装，不传客户端Bearer。

共享导航激活三个现有菜单；不新增顶层专员菜单。
`workspace-app-shell.tsx`的`hasIndependentIdentityRoute`须纳入已开放安装的
`/workbench/tools/custom/review`，复用现有生态服务专员入口规则；否则零企业成员的合法
平台专员会被前端重定向到no-organization。专员页面仍依赖登录身份，API必须独立授权。
采集插件启用不授予Product/Sourcing业务权限。
需求stage推进依次：提交→评估→方案确认→开发联调→专员记录交付；同阶段追加进度或非终态关闭。
已交付/已关闭是终态，进度不是付款或Tool安装证明。

未知写响应保存在本浏览器会话的原actor+org隔离存储，刷新/返回原上下文可重试原命令。
浏览器无法读写恢复记录时暂停新提交；不清理用户浏览器数据来恢复操作。

## 验证界限

开发自检覆盖PG原子性/并发/隔离/撤权、HTTP严格边界、平台授权、BFF与UI原key恢复。
这些是本批代码证据。共享runtime接线、当前安装插件、真实1688/provider、用户试用均
需要独立实际执行；未执行保持NOT_RUN。无合并、关单、共享部署或真实数据授权。
