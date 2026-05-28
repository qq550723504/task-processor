# ListingKit 本地复验（复用线上数据库）

这个方案适合验证 `ListingKit Workbench` / `product-listing-api` 的页面与业务逻辑，
避免每次都先发版再看结果。

核心思路：

1. 本地启动 `product-listing-api`
2. 通过 `kubectl port-forward` 把线上 PostgreSQL 转到本地
3. 同时把线上 Redis 转到本地
4. 本地 `listingkit-ui` 指向本地 API

注意：

- 这是“本地程序 + 线上数据库”的联调方式，不是完整离线环境。
- 本地操作会直接读取线上任务数据；如果你点击保存/提交，也会真正写入线上数据库和 Redis。
- 建议优先用于页面逻辑、只读检查、按钮可用性和提交前校验复验。
- 本地联调统一只保留一套固定端口：
  `PostgreSQL=15432`、`Redis=16379`、`product-listing-api=8085`、`listingkit-ui=3000`。
- 不要同时启动多个 `port-forward` 窗口、多个本地 API、多个本地 UI，也不要临时改成本轮专用端口。
  否则很容易出现“浏览器连的是 A、API 连的是 B、Redis 连的是 C”的混乱状态。
- 对 ListingKit / SHEIN / SDS 的真实联调，Redis 不再建议省略。
  原因是任务状态、登录态、SHEIN cookie、部分缓存和恢复链路都会依赖 Redis；
  只连数据库不连 Redis，容易出现“数据库是线上、cookie 和缓存却是本地旧数据”的假象。

## 一键转发

在仓库根目录执行：

```powershell
.\scripts\start-listingkit-local-portforward.ps1
```

脚本会：

- 转发 `yudao-cloud/postgresql-v18` 到 `127.0.0.1:15432`
- 转发 `yudao-cloud/redis-master` 到 `127.0.0.1:16379`
- 打印本地 API / UI 的建议启动命令
- 保持窗口常驻，直到你 `Ctrl+C`

固定端口约定：

- 数据库固定用 `15432`
- Redis 固定用 `16379`
- 本地 API 固定用 `8085`
- 本地 UI 固定用 `3000`

每次联调都复用这一组固定端口，不再另外开第二套 `8086`、`3001`、`16380` 之类的临时实例。

如果你这次只是看纯静态页面，也可以显式跳过 Redis：

```powershell
.\scripts\start-listingkit-local-portforward.ps1 -SkipRedis
```

但除非你非常确定这轮只看纯前端静态展示，否则不建议这么做。
也就是说，默认命令就是推荐命令。

如果你真的要跳过 Redis，那么下面这些内容都不应该在这轮里复验：

- SHEIN 手工搜类目
- SHEIN / SDS 登录态恢复
- SHEIN cookie 读取与刷新
- Resolution cache / 发布前缓存命中
- 任务状态恢复、重试、阻断项判断

## 启动本地 API

脚本启动后，另开一个 PowerShell，直接执行：

```powershell
.\scripts\start-listingkit-local-api.ps1
```

这个脚本会固定做 4 件事：

1. 清掉当前占用 `8085` 的旧进程，避免误连到上一次残留实例
2. 重新构建本地 `product-listing-api`
3. 用固定端口 `8085` 启动新实例，并把日志写到 `.local/tmp/listingkit-local-api/logs/`
4. 对这次本地进程强制注入 `TASK_PROCESSOR_SHEIN_IGNORE_STORE_PROXY=1`

第 4 点是这轮联调里非常关键的一步。
原因不是“兼容”，而是根因上本地开发机通常访问不到店铺在远端环境里配置的代理地址；
如果本地复验还沿用 `storeInfo.Proxy`，就会出现：

- `preview` 看起来正常
- 但真正发 SHEIN 请求时在 TCP 层直接连不上代理
- 最终把“本地网络不可达”误判成“类目搜索/联调逻辑有问题”

本地 API 启动脚本默认就是为了避免这种假失败。

如果你想显式指定参数，也可以：

```powershell
.\scripts\start-listingkit-local-api.ps1 -Port 8085 -ConfigPath config/config-dev.yaml -LogLevel info
```

这里仍然建议把数据库、Redis、cookie Redis 这些连接信息固定写在仓库根目录的 [`.env`](/D:/code/task-processor/.env)，
不要每次在当前 PowerShell 窗口里临时敲一遍环境变量。
原因是 `product-listing-api` 本身就会自动加载仓库根目录 `.env`；
统一落到文件里，才能保证每次联调都命中同一套固定配置。

## 为什么数据库和 Redis 要一起连

这套本地复验里，至少有 3 类状态不是只放数据库：

1. SHEIN cookie
   `product-listing-api` 会从 `platforms.shein.cookieRedis` 读取 `shein:cookie:<tenant>:<store>`。
   如果这里还指向本地 Redis，就会出现：
   “任务是线上数据，但 cookie 是本地旧值或空值”。

2. 通用运行态 / 登录态
   一些恢复、登录、暂停状态和运行态判断会同时依赖数据库与 Redis。
   只连库不连 Redis，页面看到的阻断项可能不是真实线上状态。

3. 缓存与联调结论
   如果你正在复验缓存是否生效，数据库和 Redis 必须来自同一套远端环境，
   否则很容易把“本地缓存命中/未命中”误判成真实逻辑结果。

## 最小自检

启动本地 API 后，建议先做一轮最小自检，再打开页面：

```powershell
Invoke-WebRequest -Uri http://localhost:8085/health -UseBasicParsing
```

然后确认 API 启动日志里至少满足两点：

- 数据库连接目标已经是 `127.0.0.1:15432`
- Redis / SHEIN cookie Redis 已经是 `127.0.0.1:16379`
- 当前进程已经显式忽略 SHEIN 店铺代理
- 当前机器上只有这一套本地联调进程在占用 `15432 / 16379 / 8085 / 3000`

如果你机器装了 `redis-cli`，也建议再做一次连通性检查：

```powershell
redis-cli -h 127.0.0.1 -p 16379 PING
redis-cli -h 127.0.0.1 -p 16379 -n 9 KEYS "shein:cookie:*"
```

预期：

- `PING` 返回 `PONG`
- `DB 9` 下能看到远端同步过来的 `shein:cookie:*` key

如果 `DB 9` 为空，不要继续做 SHEIN 类目、属性、发布相关联调，
先排查是不是没有把远端 Redis 正确转发到本地。

## 启动本地 UI

再开一个 PowerShell：

```powershell
Set-Location web/listingkit-ui
$env:LISTINGKIT_API_BASE='http://localhost:8085/api/v1/listing-kits'
$env:LISTINGKIT_SERVICE_API_BASE='http://localhost:8085/api/v1'
npm run dev
```

打开：

```text
http://localhost:3000
```

本地 UI 现在也必须走真实 ZITADEL 登录；
真正决定“读的是本地还是远端”的仍然是上面的 API 环境变量。

## 适合复验的内容

- 最终确认页按钮是否可点击
- readiness / blocker 文案是否正确
- 页面映射、价格显示、草稿视图等前端逻辑
- 本地 API 是否按预期读写 `listingkit` 相关数据库与 Redis 状态
- SHEIN cookie 阻断是否会在更早阶段暴露
- 手工搜类目、缓存命中、重试子任务等依赖 Redis 的链路

## 不建议直接依赖的内容

- 完整线上鉴权链路
- 依赖线上部署环境注入的全部 secret
- 需要多服务协同的真正发布链路回归
- 任何会真实改写线上店铺登录态、发布状态的高风险操作

如果后面要进一步减少线上依赖，下一步最值得做的是再补一层：

- `product-listing-api` 本地直连线上库
- 但把真正 `submit remote` 调用切到可控的 mock / dry-run 开关

这样页面和数据库能真联调，发布动作又不会误打到线上。
