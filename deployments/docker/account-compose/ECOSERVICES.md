# 生态服务非支付接入

设计依据：`docs/architecture/ecoservices-qualification-runtime.md`，已独立准入。它在现有统一程序后安装新的空 Ecoservices owner，不导入旧模拟试用的数据，也不重装 Product/native owners。

在 base、knowledge、data-services、modules 四份 Compose 后追加 `docker-compose.ecoservices.yml`。例如在本目录使用原项目私密 env：

```powershell
$composeFiles = @('--env-file', '<private-compose.env>', '-f', 'docker-compose.yml', '-f', 'docker-compose.knowledge.yml', '-f', 'docker-compose.data-services.yml', '-f', 'docker-compose.modules.yml', '-f', 'docker-compose.ecoservices.yml')
docker compose @composeFiles build bootstrap ecoservices-init current-application listingkit-ui
docker compose @composeFiles up -d --wait
```

若使用原保留实例的 immutable image overlay，必须在这个 overlay 内更新当前 application/UI 和新的 ecoservices-init 镜像；后置镜像覆盖优先。实例交接的 start/stop 脚本需要携带所有五份 Compose 和原 image overlay，保留同一 project/env/named volumes。首次 Ecoservices 安装拒绝已有同名数据库及中断 marker，不支持在此模式接管全支付实例。完成后的 restart 复用原配置/密钥；输入 module manifest 摘要变化会明确拒绝，需检查后另行决定，不静默重新装配。不要删 marker、重 seed、执行 `down -v` 或用旧模拟 credentials。

统一程序的原登录和企业选择保持。申请负责人进入 `/workbench/services/join` 上传私有机构材料并提交/更正；原平台审核身份进入 `/workbench/services/review` 审核原申请；批准后申请企业在原申请确认当前协议。市场 `/workbench/services/market` 与我的服务 `/workbench/services/mine` 查询实际记录，空结果保持为空。原审核记录、附件、CAS/幂等和原生授权保留。

本模式只有资质路径开放。商户进件、草稿/发布、服务需求、报价/交付/验收、支付/退款/分账及金融审核关闭。平台批准及协议确认不激活服务商，不替代微信签约。准备商户配置后的正式渠道接入属于后续明确范围，不能直接删除 `nonPaymentOnly` 或补假配置开启；原完整渠道合同及原 B/M 所有者验证继续适用。

数据库仅使用现有 `ecoservices-schema-init` 和受限 `ecoservices_runtime`；业务服务器不做 DDL、不挂 SQL installer/owner secrets。私有对象 server 只有自己的 root secret 与 data；storage init 只有该 root 和专用 access/secret keys，不读完整统一 manifest。bucket 匿名访问关闭，runtime policy 仅 GetObject/PutObject 于当前 E 文件前缀。存储使用当前 immutable S3 adapter，无公开下载或覆盖重用文件身份。

数据存于该项目原 business-db 和新的独立 ecosystem object/config/installer volumes。停止和重启均保留数据，销毁须单独授权。正常运行/开发测试不等于用户业务验收；真实微信/provider、资金操作和用户验收须分别记录为 NOT_RUN。
