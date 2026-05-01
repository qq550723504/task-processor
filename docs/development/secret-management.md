# Secret Management

生产凭据不得写入仓库，包括 `.env`、`config/*.yaml`、`Secret` manifest、临时脚本和调试输出。

## 配置约定

- 本地开发：使用 `.env.example`、`.env.prod` 模板、`config/*example*` 或未提交的本地覆盖文件。
- Kubernetes：使用 `ExternalSecret` 生成运行时 `Secret`，业务 Deployment 继续通过现有 `secretRef`/`envFrom` 读取变量。
- 变量名保持稳定，例如 `TASK_PROCESSOR_OPENAI_API_KEY`、`TASK_PROCESSOR_RABBITMQ_URL`，避免应用代码感知密钥来源变化。

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
