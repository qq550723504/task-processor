# Secret Management

生产凭据不得写入仓库，包括 `.env`、`config/*.yaml`、`Secret` manifest、临时脚本和调试输出。

## 配置约定

- 本地开发：使用 `.env.example`、`.env.prod` 模板、`config/*example*` 或未提交的本地覆盖文件。
- Kubernetes：使用 `ExternalSecret` 生成运行时 `Secret`，业务 Deployment 继续通过现有 `secretRef`/`envFrom` 读取变量。
- 变量名保持稳定，例如 `TASK_PROCESSOR_OPENAI_API_KEY`、`TASK_PROCESSOR_RABBITMQ_URL`，避免应用代码感知密钥来源变化。
- ListingKit Go API 运行时配置现在统一进入核心配置对象；`listingkit.zitadel.*`
  可以来自 YAML，也可以来自绑定 env。Next.js UI 仍直接读取
  `ZITADEL_*` 浏览器端 OIDC 变量。

## External Secrets 约定

- `secretStoreRef.name`: `task-processor-secrets`
- `secretStoreRef.kind`: `ClusterSecretStore`
- 远端 key 路径格式：`task-processor/<env>/<app>`
- 每个 `ExternalSecret` 的 `target.name` 必须与 Deployment 当前引用的 Secret 名称一致。

## 上线前必须完成

1. 轮转 RabbitMQ、OpenAI、PostgreSQL、Redis、对象存储等现有凭据。
2. 在集群中创建或更新 `ClusterSecretStore` 与远端 key。
3. 失效旧的明文 Kubernetes Secret 来源。
4. 评估并执行 git 历史清理。

## ListingKit / ZITADEL 额外约定

- `ZITADEL_ISSUER_URL`、`ZITADEL_CLIENT_ID`、`ZITADEL_CLIENT_SECRET`、
  redirect URI 这类值继续保留在 Secret，因为 UI 仍直接读取它们。
- Go API 的 ZITADEL issuer/client secret 与 allowlist 继续通过
  `ZITADEL_*`、`LISTINGKIT_ZITADEL_ALLOWED_*` 或对应的
  `listingkit.zitadel.*` 配置项注入；ListingKit 鉴权不再提供运行时关闭开关。
- 迁移步骤与验收项见
  [listingkit-config-migration-checklist.md](/D:/code/task-processor/docs/development/listingkit-config-migration-checklist.md)。
