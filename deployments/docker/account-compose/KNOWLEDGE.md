# 当前企业知识库（#557 Slice A）

依据 [冻结架构](../../../docs/architecture/agent-knowledge-context-v1.md) 和 [#557](https://github.com/qq550723504/task-processor/issues/557)。

本切片交付企业知识库与资料管理：创建、改名、停用、上传、版本替换、处理状态和正文预览。
每个知识库最多 4 份 ACTIVE 资料；TXT、Markdown 直接规范化，文本 PDF/DOCX 使用固定 Tika 4.0.0 提取。
不包含 OCR、模板、向量检索、Agent 选择或模型调用，资料不会自动用于 AI。

#558 的冻结执行上下文、Citation 与本地 dispatch permit 接口见
[Knowledge context 消费说明](../../../docs/engineering/knowledge-context-consumption.md)。
它仍不开放 Agent/模型入口；#559 负责后续接线。新 schema 仅用于新的空项目，
已初始化的 #557 试用项目继续使用其匹配镜像，不能就地追加本切片表或重建该保留实例。

## 启动与入口

沿用 [账户中心启动说明](README.md) 中的环境变量、独立端口、首次初始化和登录方式。
必须使用新的空 Compose project，同时加载两个文件：

```powershell
$env:COMPOSE_PROJECT_NAME = "knowledge-v1-local"
$env:ACCOUNT_IDENTITY_PORT = "29441"
$env:ACCOUNT_APPLICATION_PORT = "29442"
$env:ACCOUNT_MAIL_PORT = "29443"
docker compose -f deployments/docker/account-compose/docker-compose.yml -f deployments/docker/account-compose/docker-compose.knowledge.yml up -d --build
```

首次启动会生成私密凭据。按账户中心 README 的方式取得初始登录凭据；不要将卷中的密码复制到 Issue、PR 或聊天。
登录后在当前企业打开 `https://localhost:29442/workbench/ai/knowledge`。
MinIO / mc 从[官方 MinIO Release](https://github.com/minio/minio/releases/tag/RELEASE.2025-04-22T22-12-26Z)和
[官方 mc Release](https://github.com/minio/mc/releases/tag/RELEASE.2025-04-16T18-13-26Z)的固定二进制构建，
Dockerfile 校验各架构已固定的官方 SHA256；首次构建需要访问 GitHub Release，提供 amd64 / arm64。
本轮实际构建运行的是 amd64，arm64 构建未执行。
原 Quay 固定镜像与 DockerHub 同版本镜像在开发验证时无法拉取，不作为启动依赖。
本地 bootstrap 管理员允许管理；常规 `listingkit_admin/platform_admin` 可管理，
`listingkit_operator` 只读且页面隐藏管理控件，`listingkit_viewer` 不可读取。每次读取和写入都重新验证企业授权。

1. 创建知识库，打开详情。
2. 选择一份文件并确认资料名称；请求总大小不超过 10 MiB、文件名不超过 255 UTF-8 字节、资料名最多 120 字符。
   浏览器为有界 multipart 元数据保守预留 4 KiB；临界文件须缩小后提交，API 总请求上限仍为 10 MiB。
3. 查看真实处理状态；AVAILABLE/PARTIAL 可预览纯文本，PARTIAL 会显示原因。
4. 重新上传会建立新版本。新版本处理或失败时仍可预览旧的可读版本。
5. 停用资料释放一个有效资料名额；停用资料或整个知识库立即禁止预览，V1 不提供恢复或删除。
6. 若写请求结果不确定，点击“重试同一次操作”。页面保留原键与原命令；不要关闭页面后另起同一创建操作。
   原请求未确认时，重试的权限拒绝不能证明前次未发生；页面保留原键，恢复访问后再确认同一次操作。
7. 当前企业读取权限撤销或读取返回授权错误时，页面隐藏缓存名称、文件名和正文并取消旧读取；授权错误保持到点击“重新确认”并重新取数。未确认写操作的原键继续保留。

## 数据与运行边界

- 应用 PostgreSQL 实例中的独立 `knowledge` database 保存事实、解析正文和租约。
- `knowledge_owner` 只用于显式 Goose schema-init；运行角色 `knowledge_runtime` 最多 4 连接，
  只有所需 SELECT/INSERT/可变列 UPDATE，无 DDL、TEMP、跨业务库 CONNECT 或角色继承。
- 原文件保存在私有 MinIO 卷的 `knowledge/knowledge/*` 命名空间，运行凭据只允许该前缀 GetObject/PutObject。
  重复写入复用确定对象身份并先检查摘要与大小；浏览器没有对象 URL 或 key。
  合成集成检查已用实际 bootstrap/policy 的非 root 运行凭据验证缺失对象、首次上传、不可覆盖、读回、禁止删除与前缀外写入。
  本文件 policy 对应固定 MinIO；若单独配置 AWS S3，必须按[官方 HeadObject 权限要求](https://docs.aws.amazon.com/AmazonS3/latest/API/API_HeadObject.html)
  为专用私有 bucket 配置 `s3:ListBucket` 以区分缺失对象的 404 与拒绝访问的 403。生产 S3 仍为 NOT_RUN。
  对象 endpoint 留空使用 AWS 默认 HTTPS；自定义外部地址必须 HTTPS。HTTP 仅允许固定 `knowledge-objects` 服务、localhost、loopback 或私网 IP。
  知识库专属 HTTP 客户端禁止所有重定向，且不使用环境代理；配置错误须修正原 endpoint，不能自动转发签名请求或正文。
- Tika 只连接 internal Docker network，无公开端口、凭据或宿主目录；非 root、只读 rootfs、256 MiB 临时盘、
  2 CPU/2 GiB/128 PID，仅 PDF/OOXML parser，关闭 OCR/嵌入文档/请求配置。
  [官方配置说明](https://tika.apache.org/docs/4.0.x/configuration/index.html)与[服务端边界](https://tika.apache.org/docs/4.0.x/using-tika/server/index.html)。
- 后台每 5 秒最多领取 2 个立即可运行任务（合同上限 4），租约 30 秒、处理截止 20 秒、最多 3 次解析尝试。
  重启从数据库恢复；缺失对象标记 UPLOAD_INCOMPLETE，相同上传键与字节可恢复原 Revision。
- 企业切换时清空旧企业数据、请求与预览。结果不确定的写操作先用原键收敛后才能切换企业。
- 打开详情每 5 秒同步知识库与资料状态，其他管理员的改名/停用会反映在页面；停用后移除预览与管理控件。

停止/重启保留资料与凭据：

```powershell
docker compose -f deployments/docker/account-compose/docker-compose.yml -f deployments/docker/account-compose/docker-compose.knowledge.yml stop
docker compose -f deployments/docker/account-compose/docker-compose.yml -f deployments/docker/account-compose/docker-compose.knowledge.yml up -d
```

保留项目须继续使用相同 overlay。现有未启用知识库的项目不能就地追加该 schema/凭据；请创建新的空项目。
此启动命令不授权部署或改动现有共享实例。卷删除与停止是不同操作。

## 开发检查与验收边界

开发检查使用隔离 PostgreSQL、合成文件和私有 Tika；不使用企业真实文档。

```powershell
go test ./internal/knowledge/... ./internal/integration/documentparser/tika ./internal/integration/s3 ./internal/authz
go test -tags=integration ./internal/integration/persistence/knowledge ./internal/app/httpapi -run "TestKnowledge|TestCurrentKnowledge" -count=1
go test -tags=integration ./internal/integration/s3 -run TestKnowledgeMinIORuntimeMissingObjectAndImmutableRoundTrip -count=1
# 可选：仅指向独立 localhost Tika 开发实例
$env:KNOWLEDGE_TIKA_TEST_URL = "http://127.0.0.1:29998"
go test -tags=integration ./internal/integration/documentparser/tika -run TestTikaPinned -count=1
cd web/listingkit-ui
pnpm.cmd typecheck
pnpm.cmd test src/lib/server/knowledge-proxy.test.ts src/components/workbench/knowledge/knowledge-page.test.tsx
pnpm.cmd build
```

实现自检与 CI 不等于用户验收。真实企业文档、生产 S3/Tika、完整登录浏览器用户试用和 Agent/模型消费均为 NOT_RUN；
用户或指定独立验证者确认实际使用效果后才可接受本切片。PR 不自动合并或关闭 Issue。
