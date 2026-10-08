# 生态服务 v1 运行交接

适用 #603 / Draft PR #604，[冻结设计](../architecture/ecosystem-services-v1.md)。这是全新安装的运行说明，开发检查不代表用户验收或真实渠道开放。

## 用户入口与正常操作

在现有 Console 域名下使用正常登录与企业选择：

- `/workbench/services/market`：按类别筛选实际已发布服务，提交需求。
- `/workbench/services/mine`：客户跟进需求、确认报价、获取原微信付款码、检查交付并确认精确版本；服务商报价、开始服务、提交交付和协商退款。
- `/workbench/services/join`：企业提交资质，平台批准后接受合作协议，再提交原企业微信商户申请。法人按渠道要求完成账户验证与签约；只有原申请 `FINISH + SIGNED` 才可发布服务。
- `/workbench/services/review`：既有平台管理员审阅原资质、双方同版本退款协议及临期订单；使用全局平台权限，不要求选择企业。已付款订单可读取原资金事实及核实指定日期的手续费流水。

平台抽佣10%，渠道手续费由平台承担，第三方服务不产生个人推广佣金。客户确认原报价和规则后一次付款；客户验收原交付版本后，渠道分佣金并解冻服务商份额。受理、付款、分账、退款各自保存实际状态，页面不把请求成功当资金到账。

开始前取消全额原路退款；开始后由双方确认同一退款金额/版本，再由平台审核。已分账的先回退相应佣金，退款从原子商户发起；资金不足保留等待状态，不垫付。实际手续费及退费由原平台手续费账户账单核实，分别保存不可变流水，不改服务商份额。

临期与超期订单保留人工审核（AR1）。时间到期不改变验收状态。后台只核实原交易待分余额；差额无法解释时暂停后续金融操作，不推算“已自动解冻”。原已派发操作先回读其事实，不能把未落账的在途效果误判成解冻。人工漏处理导致渠道自行解冻的风险仍按已确认产品决定接受。

## 全新安装与启动

沿现有 current-application 私有 manifest、身份服务、canonical commercial/money owner 和私有 S3 安装方式。不得在共享或生产实例试运行这些初始化命令，也不复用旧服务 schema。此说明不是执行真实数据操作的授权。

E 使用独立 `ecoservices` 数据库和预先创建的 `ecoservices_runtime` 角色。该角色不拥有表，无建库/建角色/复制/RLS绕过/公共schema CREATE/数据库 CREATE或TEMP权，无成员继承；只授本模块必要 SELECT/INSERT，以及操作进度表必要 UPDATE。持久原intent、receipt、版本和商户绑定不授 UPDATE/DELETE。管理员须先按现有 owner provisioning 规则创建数据库及角色。

通过私密环境设置 `ECOSERVICES_SCHEMA_DSN`（schema owner，数据库必须是 `ecoservices`），在仓库根目录运行一次：

```powershell
go run ./cmd/ecoservices-schema-init
```

命令同事务安装 E 表并给预建 runtime 角色授予最小权限，不创建企业、申请、订单或付款。B/M 复用现有 owner 初始化入口，显式传入两个 schema owner 私有配置：

```powershell
go run ./cmd/commercial-owner-schema-migrate -config '<private-commercial-schema.json>' -money-config '<private-money-schema.json>'
```

本批的 service purchase 表和 M 资金/费用字段已包含在该入口中。运行服务本身只做 schema/role preflight，不自动安装。schema owner 凭据不能放入 serving manifest。

现有 current-application 私有 manifest 增加 `ecoservices`：

```json
{
  "enabled": true,
  "database": {
    "host": "<same-instance-as-source-owner>",
    "port": 5432,
    "user": "ecoservices_runtime",
    "password": "<private-runtime-password>",
    "database": "ecoservices",
    "maxConnections": 4
  },
  "payloadKey": "<base64-encoded-32-random-bytes>",
  "storage": {
    "mode": "aws",
    "region": "<region>",
    "bucket": "<private-immutable-bucket>",
    "endpoint": "<HTTPS-or-private-loopback-S3-endpoint>",
    "accessKeyId": "<private-access-key>",
    "secretAccessKey": "<private-secret>"
  },
  "payments": {
    "profile": {
      "Version": "<immutable-channel-profile-version>",
      "Environment": "PRODUCTION",
      "PlatformMerchantID": "<platform-merchant>",
      "AppID": "<original-platform-app>",
      "FreezeDays": 180
    },
    "newMerchantApplications": false,
    "newPayments": false,
    "productQualified": false,
    "platformPaysFees": false,
    "privateKey": "<private-merchant-PEM>",
    "serialNumber": "<merchant-certificate-serial>",
    "apiV3Key": "<private-32-byte-v3-key>",
    "publicKeyId": "<PUB_KEY_ID_...>",
    "publicKey": "<official-WeChat-public-PEM>",
    "notifyUrl": "https://<public-service-domain>/api/v1/payments/ecoservices/wechat/notify"
  }
}
```

示例只展示 `ecoservices` 对象。manifest 还须保留当前有效 identity/sourceAccountDatabase/commercialOwnerDatabase/moneyOwnerDatabase 配置；新进件与新付款要求当前 identity tenantDirectoryToken 的实时精确授权。COS 使用现有 immutable bucket policy 显式准入字段；AWS/私有 S3 沿既有 immutable object 合同。保护密钥与原渠道 profile 必须保留至原在途订单完成，不能换 profile 处理旧订单。

关闭 `newPayments` 或 `newMerchantApplications` 后保留已有事实读取、原商户查询及退款恢复；关闭整个模块则不注册生态服务接口。配置和运行依赖不全时启动拒绝，不使用默认DB、钱包支付或假渠道兜底。

后端从仓库根目录正常启动：

```powershell
go run ./cmd/current-application -config '<absolute-private-current-application.json>'
```

Console 继续使用现有正常 OAuth/session 配置，并在服务端设置：

```text
LISTINGKIT_ECOSERVICES_ENABLED=true
LISTINGKIT_SERVICE_API_BASE=https://<current-application-origin>/api/v1
LISTINGKIT_PUBLIC_BASE_URL=https://<console-origin>
```

在 `web/listingkit-ui` 用现有 pnpm 运行 `pnpm build`、`pnpm start`。新开关只控制本实例页面/导航入口，不授予权限或证明渠道资格。未启用时显示“尚未开放”。平台审核入口由后端既有 platform-admin 真实权限核实，企业自定义 module role 无法获得该权限。

## 保存、停止与开放边界

申请、服务条目、需求、原命令与私有文件元数据保存在 E；订单/原外部操作及验签 inbox 在 B；付款、分配、预留、实际费用和 effect receipt 在 M。文件以原 file ID immutable key 存在私有对象存储，所有下载重新核实原父关系；商户敏感资料及验证观察加密保存，浏览器不持久保存原明文。

按既有进程停止方式或 `current-application -shutdown-file <private-trigger>` 优雅停止。重启复用同一数据库、bucket、原保护密钥及原渠道配置；不要重新安装、重新seed或删除 volumes。恢复继续原订单/原外部编号，UNKNOWN 不换号重发。

当前代码自检覆盖域规则、HTTP/BFF、signed original query、独立 PG runtime role/并发原intent、恢复轮转、精确版本同意及原未知命令重试。尚未执行真实微信进件、账户验证、付款、分账/回退/退款、真实FEES账单，也未完成正常登录浏览器的完整产品验收。

真实开放前需在原平台收付通账号确认产品/经营场景资格、Native支付与子商户关系、平台承担手续费账户、实际冻结/退款/回退时限和公网验签通知。这些是现有设计开放条件；不将本地fixture或CI写作通过，也不要求新增验收平台。

实现依据：[原交易剩余待分金额](https://pay.wechatpay.cn/doc/v3/partner/4012477751)、[资金变动原因/分账回退说明](https://pay.wechatpay.cn/doc/v3/partner/4012525463)、[原服务商资金账单与FEES账户](https://pay.wechatpay.cn/doc/v3/partner/4012760136)、[账单字段说明](https://pay.wechatpay.cn/doc/v3/merchant/4013071249)。未找到关联流水或验签/摘要/字段失败保留未知，不登记零费用或最终净收益。
