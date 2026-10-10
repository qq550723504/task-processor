# 供应市场 v1 正常程序接线

Issue #622 / PR #625。设计依据为已冻结的
[供应市场 v1](../architecture/supply-market-v1.md)。本接线使用现有
`cmd/current-application`，Product/Collection、Asset、Submission 和 Temporal owner。

## 新安装与配置

只对预先创建、明确确认的空 Product 数据库执行已有初始化命令。初始化私密 JSON
沿现有 `schemaVersion: 1` / `database` 格式，追加以下开关；已有供应链实例同时保留
`supplyChain: true`。现有数据库会被拒绝，不提供修复或旧数据迁移路径。

```json
{"collections": true, "supplyMarket": true, "pod": true}
```

```powershell
go run ./cmd/product-acquisition-init -config C:/private/product-init.json -confirm-empty-database product
go run ./cmd/supply-asset-init -dsn-file C:/private/asset-owner.dsn -confirm-database assets -install-empty-schema
```

部署 owner 预先配置 `source_acquisition_runtime` 和 `supply_asset_runtime` 窄角色。
Product initializer 安装 Collection 的 market/sds_template/sds_finished 约束、市场、
当前 Review 读取所需 schema，以及 POD/Submission schema；授权不包含市场/Collection
删除或 Review 写入。POD fence 仅因精确终态释放和行锁需要 DELETE/UPDATE。
Asset initializer 复用当前批准图案 schema，不新建第二 Asset owner。

当前程序的私密 manifest 继续使用已有 identity、Product 和其他实际启用的 owner 配置，
并设 `productCollections: true`。新增部分示例（占位值须由部署 owner 替换）：

```json
{
  "supplyMarket": {
    "storage": {
      "region": "REGION",
      "bucket": "PRIVATE_QUALIFICATION_BUCKET",
      "accessKeyId": "PRIVATE_KEY_ID",
      "secretAccessKey": "PRIVATE_KEY",
      "mode": "aws"
    }
  },
  "pod": {
    "assetDatabase": {
      "host": "127.0.0.1", "port": 5432,
      "database": "assets", "user": "supply_asset_runtime",
      "password": "PRIVATE_PASSWORD", "maxConnections": 4
    },
    "temporalAddress": "127.0.0.1:7233",
    "temporalNamespace": "default",
    "credentialFile": "C:/private/platform-account.private.json",
    "ossHosts": ["QUALIFIED_OSS_HOST"]
  }
}
```

资质 bucket 必须为私有 immutable S3/COS 存储，无 PublicBase。COS 模式还必须明确
设置现有 `cosImmutableNonVersionedBucketPolicy: true`。S3-compatible 测试服务可使用
`endpoint` 指向本机或私有网络；外部 endpoint 必须 HTTPS。不得使用公开商品图片桶
分发资质资料。普通启动不创建 bucket、schema、角色或业务记录。

POD 的 Asset 目标须与已启用 Image Agent、Product Agent、供应链的 canonical Asset
目标相同；已有供应链时复用其窄 Asset pool 和相同 Temporal client。其他 owner
数据库不得与 Asset 共用。`ossHosts` 必须来自已经完成的实际协议 qualification，
不能填写任意上传域名。省略 `pod` 时市场及人工申请可独立使用，SDS 定制保持关闭。

凭据文件使用当前 `sds-platform-session-v1` 格式及 UUID revision，仅服务器读取。
程序使用固定 HTTPS 商户接口核实当前凭据的实际 merchant ID，拒绝 redirects、cookie
jar、重复 JSON 字段、越界/过期/不可读凭据及身份不符；不自动登录或刷新，不读取旧
Redis/auth_state fallback。文件必须为当前用户私密文件，Windows 校验 ACL。

```powershell
go run ./cmd/current-application -config C:/private/current-application.json
```

正常生命周期启动 `product-pod-current` worker，停止 worker 后才关闭 Temporal/DB。
固定 workflow ID 为 `pod-design/<operation-id>`，恢复沿已有 original intent、
SendPermit/attempt、merchant/template fence 及 read-only observation，不重复发送。
启动依赖或 schema/权限验证失败时服务拒绝启动，并清理已打开资源。

## 前端与使用入口

前端连接同一正常 API，设置 `LISTINGKIT_SERVICE_API_BASE=<API origin>/api/v1`。
实际后台已准入后，启用 `LISTINGKIT_PRODUCT_COLLECTIONS_ENABLED=true`、
`LISTINGKIT_SUPPLY_MARKET_ENABLED=true`；仅在 POD account、Asset、Temporal 和已核定
OSS hosts 都完成配置后才设置 `LISTINGKIT_SDS_POD_ENABLED=true`。沿既有 HTTPS/登录
配置启动 UI；这些变量不代替后端依赖检查或权限。

企业成员登录并选择企业后可进入：

- `/workbench/supply/official`、`/selected`：查看真实已发布商品，选择并保存到本人
  Collection。空市场显示空状态，不 seed 假发布。
- `/workbench/supply/catalogs`：沿已有 1688 采集、SDS 模板及人工货盘申请入口。
- `/workbench/supply/applications`：本人申请、资料补充和进度。优选申请继续要求
  本人自有商品及实际 Apply lineage。
- `/workbench/supply/catalogs/sds`：选模板/款式和本人已批准图案，配置设计；核实
  原 operation 的效果图和已保存成品后再选择入本人 Collection。

市场和已核实 SDS 成品批次可以沿既有“加入我的供应链”转入准备流程，保留原
source kind、publication 和 operation；未定制 SDS 模板须先生成并核实成品。
混合手动批次包含未定制模板时，转入会明确拒绝并回滚，先将成品单独分组。
新安装的 preparation schema 只接收 acquisition/own/market/sds_finished；启动
检查拒绝仍只有 acquisition/own 的旧约束，不在运行期修改既有数据库。

首次发送前的临时读取失败仅在原 Kernel 确认 OSS attempt 不存在时，在同一
固定工作流内等待重查，最长15分钟。已有 attempt 或读取结果不明确时不重试。
前置等待超时/工作流关闭后显示 UNKNOWN，保留原操作和 fence，不新建工作流。

平台专员通过 `/workbench/supply/operator` 及原申请 UUID 详情执行人工评估、发布。
该入口独立使用现有 server session 身份，后台每次核对 verified global platform
权限，不要求成为客户企业成员。导航仅在原平台读取成功后展示专员入口；企业
角色/自定义模块、浏览器 body/header 不能授予平台权限。成员页面仍保留企业 gate。

所有事实保存在现有 Product/Asset 数据库，资质 bytes 留在私有对象存储。保留命名
volume/数据库并按原配置 stop/start；不执行 `down -v`。UNKNOWN/UNCONFIRMED 保存
原 key/body/account revision 和 fence，先核实原操作；不另起操作覆盖或重发。

## 当前证据与交付边界

本轮有受限角色的 disposable PostgreSQL 空库安装、市场/POD HTTP composition、
精确 route admission、额外删除权限拒绝、凭据身份校验、运行资源清理及 UI/BFF
开发自检。这些是接线检查，不是用户验收。

本轮未改变保留中的 31544 实例/数据卷，未追加真实 SDS 素材/成品写入，也未重试
旧未确认 operation。正常实际 runtime、真实浏览器1688/SDS 路径和用户试用仍为
`NOT_RUN`；实际环境的私有资质存储、OSS qualification host 及外部依赖须按部署
owner 配置后再试用。本 PR 不授权合并、共享/生产部署、Issue 关闭或真实数据操作。
