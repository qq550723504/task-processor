# ListingKit 本地复验（复用线上数据库）

这个方案适合验证 `ListingKit Workbench` / `product-listing-api` 的页面与业务逻辑，
避免每次都先发版再看结果。

核心思路：

1. 本地启动 `product-listing-api`
2. 通过 `kubectl port-forward` 把线上 PostgreSQL 转到本地
3. 本地 `listingkit-ui` 指向本地 API

注意：

- 这是“本地程序 + 线上数据库”的联调方式，不是完整离线环境。
- 本地操作会直接读取线上任务数据；如果你点击保存/提交，也会真正写入线上库。
- 建议优先用于页面逻辑、只读检查、按钮可用性和提交前校验复验。

## 一键转发

在仓库根目录执行：

```powershell
.\scripts\start-listingkit-local-portforward.ps1
```

如果你还需要复用线上 Redis（例如 SHEIN cookie / 某些本地登录态依赖），可以加：

```powershell
.\scripts\start-listingkit-local-portforward.ps1 -IncludeRedis
```

脚本会：

- 转发 `yudao-cloud/postgresql-v18` 到 `127.0.0.1:15432`
- 可选转发 `yudao-cloud/redis-master` 到 `127.0.0.1:16379`
- 打印本地 API / UI 的建议启动命令
- 保持窗口常驻，直到你 `Ctrl+C`

## 启动本地 API

脚本启动后，另开一个 PowerShell：

```powershell
$env:TASK_PROCESSOR_DATABASE_HOST='127.0.0.1'
$env:TASK_PROCESSOR_DATABASE_PORT='15432'
$env:TASK_PROCESSOR_DATABASE_USER='root'
$env:TASK_PROCESSOR_DATABASE_NAME='ruoyi-vue-pro'
$env:TASK_PROCESSOR_DATABASE_PASSWORD='<fill-real-password>'
go run ./cmd/product-listing-api -config config/config-dev.yaml -port 8085 -log-level info
```

如果你用了 `-IncludeRedis`，也可以继续覆盖：

```powershell
$env:TASK_PROCESSOR_REDIS_HOST='127.0.0.1'
$env:TASK_PROCESSOR_REDIS_PORT='16379'
$env:TASK_PROCESSOR_SHEIN_COOKIE_REDIS_HOST='127.0.0.1'
$env:TASK_PROCESSOR_SHEIN_COOKIE_REDIS_PORT='16379'
```

## 启动本地 UI

再开一个 PowerShell：

```powershell
Set-Location web/listingkit-ui
$env:LISTINGKIT_API_BASE='http://localhost:8085/api/v1/listing-kits'
$env:LISTINGKIT_SERVICE_API_BASE='http://localhost:8085/api/v1'
$env:LISTINGKIT_UI_BYPASS_AUTH_GATE='1'
npm run dev
```

打开：

```text
http://localhost:3000
```

## 适合复验的内容

- 最终确认页按钮是否可点击
- readiness / blocker 文案是否正确
- 页面映射、价格显示、草稿视图等前端逻辑
- 本地 API 是否按预期读写 `listingkit` 相关数据

## 不建议直接依赖的内容

- 完整线上鉴权链路
- 依赖线上部署环境注入的全部 secret
- 需要多服务协同的真正发布链路回归

如果后面要进一步减少线上依赖，下一步最值得做的是再补一层：

- `product-listing-api` 本地直连线上库
- 但把真正 `submit remote` 调用切到可控的 mock / dry-run 开关

这样页面和数据库能真联调，发布动作又不会误打到线上。
