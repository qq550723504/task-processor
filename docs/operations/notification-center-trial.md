# 通知中心使用与启动说明

Refs #608、PR #609。设计依据为 [通知中心 v1](../architecture/notification-center-v1.md)，入口为当前 Console 的 `/workbench/notifications` 和右上角通知按钮。

通知中心展示当前账号有权读取的官方消息和业务事实。通知已读只保存阅读收据；待确认方案、人工 Review、资源欠量、支付核实等仍由原业务页面处理。它不执行模型、支付、平台发布、邀请重发或业务恢复。

## 准备独立存储与当前应用

复用现有 current-application 的身份认证、授权、业务 owner 与私有 manifest。数据库 owner 先为本次全新安装准备独立 PostgreSQL 数据库和 `notification_center_runtime` 登录角色：该角色没有 superuser、建库、建角色、replication、bypassrls、其他角色成员关系、schema/table 所有权、schema CREATE 或其他业务表权限。数据库与其他 owner 的 host/port/database 组合必须不同。密码只保存在私有配置中。

只在首次安装、目标为空且操作已获授权时，用 owner 连接显式初始化；初始化不会创建登录角色、公告或业务数据，服务启动也不会自动建表或迁移：

```powershell
$env:NOTIFICATION_CENTER_SCHEMA_DSN = '<private owner PostgreSQL DSN for the dedicated notification database>'
go run ./cmd/notification-center-schema-init --runtime-role notification_center_runtime
Remove-Item Env:NOTIFICATION_CENTER_SCHEMA_DSN
```

Docker 部署复用现有 `Dockerfile.account-compose` 的 `runtime` 和 `schema-init` 镜像；后者包含相同的通知初始化命令。它不随 account-bootstrap 或应用启动自动执行。获准初始化专用空库、且已准备受限角色后，使用私有 env 文件传入 `NOTIFICATION_CENTER_SCHEMA_DSN`，显式运行：

```powershell
docker build -f deployments/docker/Dockerfile.account-compose --target schema-init -t listingkit-notification-schema-init .
docker run --rm --network '<approved database network>' --env-file C:\private\notification-schema.env --entrypoint /usr/local/bin/notification-center-schema-init listingkit-notification-schema-init --runtime-role notification_center_runtime
```

env 文件与 DSN 只保存在私有位置；数据库网络及连接地址使用本次获准实例的实际配置。应用使用 `runtime` 镜像，Console 使用现有 `Dockerfile.listingkit-ui`；继续沿用原实例的身份、业务 owner 和持久化配置，并设置下方通知库字段及前端启用标志。

在现有完整私有 manifest 中增加以下字段；这是配置片段，不是可独立启动的完整 manifest：

```json
{
  "notificationCenterDatabase": {
    "host": "127.0.0.1",
    "port": 5432,
    "user": "notification_center_runtime",
    "password": "<private runtime password>",
    "database": "notification_center",
    "maxConnections": 4
  }
}
```

用正常启动路径加载绝对路径私有配置：

```powershell
go run ./cmd/current-application -config C:\private\current-application.json
```

启动会验证通知库权限。当前业务来源取决于原 owner 是否已经配置并可用；通知模块不会代替它们建库、授予业务权限或启用 provider。数据库 owner 应将新增 `notification_center` schema 纳入已有备份与恢复安排。

Console 保持已有 Auth.js/ZITADEL 配置，设置服务端 `LISTINGKIT_SERVICE_API_BASE` 为 current-application 的 `/api/v1` 地址，并保持 `LISTINGKIT_PUBLIC_BASE_URL` 与浏览器实际访问 origin 一致。启用通知入口后，按正常命令启动前端：

```powershell
# 在 web/listingkit-ui 中执行；API 地址按当前实例配置。
$env:LISTINGKIT_NOTIFICATION_CENTER_ENABLED = 'true'
pnpm.cmd dev
```

服务未配置通知库时不注册通知 API；前端未设启用标志时入口明确显示尚未启用。两边均启用才具备试用条件。

## 用户操作

1. 正常登录，在通知按钮或 `/workbench/notifications` 打开通知中心。查看“硕米官方通知”或“商家经营触发通知”。切换企业会清除旧上下文内容；企业上下文不可用时，官方消息和本人消息仍可读取。
2. 官方消息可以筛选未读，业务消息可以筛选待处理。数字来自服务器完整授权集合；来源不完整时数字显示已知下限，并列出不可用来源，不显示伪造的精确零。
3. 打开详情查看正文、实际发生时间与原业务入口。没有可靠原始时间的业务事实显示时间未记录。详情阅读成功才更新已读；业务“待处理”不会因已读消失。
4. “全部已读”覆盖点击时当前类别的完整可见集合，包括其他页和被筛选隐藏的消息。服务器保存最长五分钟的引用快照，并在提交时重新检查原资源授权和版本；不可用、失权或旧版本会整批拒绝。新消息和新版本继续未读。
5. 请求结果未知时，沿界面提示核实原操作；界面保留原操作键，先查询收据或重放原快照，不自动创建新操作扩大已读范围。快照失效且原操作未提交时刷新列表再操作。

阅读收据按已验证 realm、账号及具体来源事实版本保存；同一条本人消息跨企业上下文共享已读状态。公告与已读数据保存在通知数据库，业务事实仍由原 owner 保存。正常停止/重启应用保留这些数据；删除数据库或 Docker volume 是另外的破坏性操作。

## 官方消息维护

由服务器验证的平台管理员使用原 bearer 身份调用以下 API；普通企业管理员或浏览器自填角色不能发布公告。每次写入使用一个 UUID `Idempotency-Key`，同键同载荷重放，同键不同载荷拒绝。

- `POST /api/v1/platform/notifications/official`，JSON 字段为 `category`（`PRODUCT`、`SYSTEM`、`ACTIVITY`、`POLICY`）、`title`、`summary`、`paragraphs`（纯文本数组）、`target`（例如 `{"kind":"home","id":""}`）。响应 `resultId` 是公告 ID。
- `POST /api/v1/platform/notifications/official/{id}/withdraw`，JSON 为 `{"revision":1}`。撤回后不再可见；正文不可原地修改，纠正内容使用撤回后新发公告。

凭据仅在私有客户端或服务器环境使用，不写入文档、浏览器组件或日志。公告读者为同一配置 issuer/realm 下的正常登录用户。后台发布界面不在本批范围。

## 覆盖与限制

已接入当前模型中的 Workbench 方案与任务、标题 Review、1688 获取、店铺连接/服务、企业与本人资源/月度额度、支付与充值、邀请/成员操作、Knowledge、企业/个人认证及本人推广收益调整/提现。读取使用原 owner 的当前授权和纯事实接口；未配置或暂时失败的当前来源显示不可用。只有原 owner 可见的事实才参与消息、数字及全部已读。

库存、广告、机会、报表、商家聊天和履约尚缺正式引擎，明确显示依赖未开放，未伪造通知或新建这些业务功能。公告与普通已读无需等待这些未来引擎；当前应存在的业务来源读取失败会阻止该类别的全部已读。

开发验证使用隔离 PostgreSQL、受控业务事实和 UI fixture。1440px/390px 深浅色布局预览、单元/集成测试与 CI 是开发证据；真实登录、真实业务流程和用户试用仍须在获准目标实例独立确认，本说明不声明已部署或产品验收通过。
