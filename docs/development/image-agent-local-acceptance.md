# Image Agent 本地 Manual Runtime 验收

这条链路用于验证本地 ZITADEL 身份、ListingKit 任务归属和 Image Agent
Temporal 接收边界。它不声称远程图片生成成功，也不配置或调用生产 COS、远程
模型或真实平台凭据。

## 前置条件

- Windows PowerShell、Go、Docker Desktop 和 Docker Compose 可用。
- 准备一个本地 ZITADEL 管理 PAT。PAT 只由操作者写入
  `.local/image-agent-acceptance/management-admin-token.txt`，命令不会打印它。
- 不需要手工复制 bearer token。登录后的 Auth.js JWT callback 会在服务端把加密 JWT
  中的 access token 写入验收目录；浏览器可见的 session 和自定义 session API 都不含
  access token 或 ID token，token 也不会进入聊天或日志。
- `SourceUrl` 必须是公开可访问的 HTTPS 图片 URL；`StyleUrl`（如果使用）同样如此。

所有生成文件和凭据都位于 `.local/image-agent-acceptance/`，该目录已被 git 忽略。
脚本和 UI token handoff 会拒绝符号链接/Windows reparse point；在 Windows 上，凭据
文件使用受保护 ACL，只允许当前用户、SYSTEM 和本机 Administrators 读取。ACL 无法
收紧或验证时链路会失败关闭。

## 操作顺序

### 1. 启动隔离运行时

```powershell
./scripts/image-agent-local-acceptance.ps1 start
```

该命令启动本地 ZITADEL 和固定 Compose 项目 `task-processor-image-agent-acceptance`，包含：

- `acceptance-postgres`，固定数据库 `image_agent_acceptance`；
- `acceptance-redis`，固定本地端口 `16379`；
- `acceptance-minio`，固定 S3 API 端口 `19000`，只用于本地 durable artifact；
- `temporal`，本地前端端口 `17233`；
- 本地 ListingKit API（`18085`）和 UI（`3000`）。worker 会在第 4 步拿到真实
  tenant allowlist 后启动。

它会生成本地数据库密码，并只在验收编排器中创建环境 marker；正式产品 schema
迁移不会创建这个验收表。DSN、marker、Compose project 和本地 ZITADEL issuer
会写入 runtime 文件。后续 seed 在连接数据库前会核对固定项目
`task-processor-image-agent-acceptance`、服务 `acceptance-postgres`、数据库
`image_agent_acceptance`、用户 `acceptance`、回环发布地址和端口 `15433`。此时 API 只验证 HTTP listener；由于
OIDC 应用尚未 provision，`/readyz` 暂不可用是预期状态。不要手工修改这些值。

验收 Compose、ZITADEL、API 和 UI 的实际监听地址都只绑定 `127.0.0.1`。API/UI 使用隔离启动
模式，不读取仓库 `.env`、不导入 Kubernetes 配置，并且遇到未知端口占用者会拒绝
启动，不会终止无关进程。

### 2. Provision 本地 ZITADEL 应用

```powershell
./scripts/image-agent-local-acceptance.ps1 provision `
  -ManagementTokenFile .local/image-agent-acceptance/management-admin-token.txt
```

该命令幂等创建/复用本地 API 应用和服务端 Web OIDC 应用，然后用真实 OIDC 配置重启
API 并验证 `/readyz`。应用只允许本地 issuer、
`http://localhost:3000/api/auth/callback/zitadel` 和 `http://localhost:3000`。
OIDC 应用与 Auth.js 明确使用相同的 `client_secret_basic` 机密客户端契约；复用旧的
public/no-auth 本地应用时会更新配置并轮换 client secret。
本地 provision 会按 `LISTINGKIT_ZITADEL_BOOTSTRAP_LOGIN_NAME`（默认
`zitadel-admin@zitadel.localhost`）查找默认人类账号并幂等授予
`listingkit_operator`，并记录该账号的 tenant ID 和 subject，因此不需要操作者提供
tenant ID 或 user ID。每次 provision 都会通过 ZITADEL 的 secret regeneration API
轮换已存在的 local API/OIDC client secret，并以原子方式写入受限 runtime 文件；这样
即使上一次在轮换后、写盘前中断，重试也不会复用失效的旧 secret。直接运行 Go
provision CLI 时也执行相同的 Windows ACL、Unix mode 和 reparse-point 校验。
后续 authorize 会 introspect 真实 bearer token，并要求 tenant 和 subject 与已记录的
bootstrap 身份完全一致，避免误授权另一个已登录账号。

### 3. 浏览器登录并完成服务端 token handoff

打开本地 UI，完成一次本地 ZITADEL 登录：

```text
http://localhost:3000
```

登录成功后打开 ListingKit 工作台即可触发 handoff。验收脚本会使用服务端写入的
`.local/image-agent-acceptance/user-token.txt`；文件只存在于本地验收目录并保持受限权限。
如果首次进入后文件尚未出现，刷新一次工作台页面即可。

### 4. Authorize 当前登录用户

```powershell
./scripts/image-agent-local-acceptance.ps1 authorize `
  -TokenFile .local/image-agent-acceptance/user-token.txt
```

命令先 introspect token，并核对 provision 记录的 bootstrap tenant/subject，再按
token 派生的真实 user/tenant 给本地项目授予
`listingkit_operator`，并把同一个 token 派生的 tenant 写入本地 worker allowlist。
随后启动 `image-agent-manual-v3` worker。需要管理页面验证时，才显式使用底层
authorize 命令的 `-grant-admin`；默认不授予管理员角色。

### 5. Seed 一条归属明确的 Image Agent 任务

```powershell
./scripts/image-agent-local-acceptance.ps1 seed `
  -TokenFile .local/image-agent-acceptance/user-token.txt `
  -SourceUrl https://public.example/image.png
```

Seed 会再次验证 Compose project、数据库名、marker 和 bearer token，然后创建
固定目标 `image-agent-acceptance-source` 的任务。重复执行同一 token 和 URL 应
得到同一确定性任务；更换用户、tenant 或目标 URL 必须被拒绝，且不能泄露其他
任务信息。

### 可选：建立双 Organization 授权验收状态

这一步不是普通 Image Agent 验收的组成部分。它会修改本地 ZITADEL：创建或复用
`ListingKit Acceptance Organization A` 和 `ListingKit Acceptance Organization B`，
把现有 ListingKit Project grant 给两者，并把 runtime 文件中记录的同一个 bootstrap
用户分别赋予 A 的 `listingkit_admin` 和 B 的 `listingkit_viewer`。当已有同名测试状态
的角色集合不同，命令会把 Project Grant 和 Role Assignment 更新为上述精确集合。

因此只有在操作者已明确授权修改可重置的本地 ZITADEL 测试数据后，才能运行：

```powershell
go run ./internal/zitadelprovision/cmd provision-multi-org-acceptance `
  -issuer-url http://localhost:8080 `
  -management-token-file .local/image-agent-acceptance/management-admin-token.txt `
  -runtime-file .local/image-agent-acceptance/runtime.env `
  -confirm-resettable-test-data
```

该子命令不提供远程覆盖，只接受 hostname 为 `localhost`、`127.0.0.1` 或 `::1` 的
issuer。Project ID 和 bootstrap user ID 只能从受保护的既有 runtime 文件读取；命令
只把后续验收需要的两个不透明 Organization ID 写回该文件，标准输出不会显示完整
用户 ID、Organization ID 或任何凭据。重复执行会复用同名 Organization、Project
Grant 和 Role Assignment。

provision 后仍需通过现有 Auth.js 流程真实登录，并由服务端持有的浏览器 token 调用
官方 v2 `ListAuthorizations`。这项读取验证与角色撤销验证是独立门槛；撤销或恢复任何
授权都需要另一项明确批准。未获批准时，两项真实环境状态都必须记录为 pending。

### 6. 查看状态和停止

```powershell
./scripts/image-agent-local-acceptance.ps1 status
./scripts/image-agent-local-acceptance.ps1 stop
```

`status` 只检查已知服务、API readiness 和 Temporal 前端连通性。普通 `stop`
会停止由验收目录记录且通过命令行/端口所有权校验的 API、UI、worker，以及验收
Compose（包含 MinIO）和本地 ZITADEL；它不删除卷，也不会终止未知进程。

需要清空验收数据库时必须显式预览并确认范围：

```powershell
./scripts/image-agent-local-acceptance.ps1 stop -Reset -WhatIf
./scripts/image-agent-local-acceptance.ps1 stop -Reset
```

Reset 只允许删除固定 Compose 项目的容器、`acceptance-postgres-data` 和
`acceptance-s3-data` 卷；不会
删除 ZITADEL 外部网络、其他 Compose 项目、工作区文件或生产数据。

本地 MinIO 的 S3 endpoint 是 `http://127.0.0.1:19000`。由于 Image Agent 对外发布
的候选 URL 仍要求公开 HTTPS，编排器为启动校验设置了专用的
`https://local.acceptance.invalid/image-agent-assets` metadata base；它不是可对外
访问的成功 URL。若执行阶段需要真实公开图片 URL，链路会按真实校验失败并记录错误，
不会回退成合成成功。

## 真实链路验收记录

验收记录应保留 HTTP 状态、任务 ID、run ID、目标和 Temporal task queue receipt，
但不要记录 bearer token、管理 PAT、client secret 或完整响应体。

Image Agent 执行阶段必须观察真实 provider/COS 配置结果：缺失或不支持的凭据应
进入 blocked/error 状态并暴露节点级原因，不得生成合成图片、伪造 URL 或回退到
Mock 成功。只有本地身份、任务创建和 Temporal 接收都实际成立，才能称为“真实链路
已到达执行边界”；不能把它表述成远程图片生成成功。
