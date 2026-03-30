# Amazon Crawler API

`amazon-crawler-api` 是一个独立的 Amazon 爬虫服务程序。

它的职责很单一：

- 提供 HTTP API
- 接收商品抓取请求
- 使用本地浏览器池执行 Amazon 抓取
- 把抓取结果返回给 `task` 程序或其他调用方

这个程序不依赖 RabbitMQ 才能工作，适合单独部署成 crawler service。

## 配置文件

默认配置文件是：

`config/config-amazon-crawler-api.yaml`

这份配置已经和 task 程序拆开，专门给爬虫 API 使用。

重点配置项：

- `browser.*`：浏览器路径、无头模式、池大小
- `amazon.crawlTimeout`：单次抓取超时
- `amazon.zipcodes`：不同 region 默认邮编
- `amazon.productDedupe.*`：去重锁、共享结果缓存、等待超时、轮询间隔
- `amazon.failureArtifacts.*`：抓取失败时的截图、HTML 和上下文留存
- `amazon.riskControl.*`：针对 `captcha / timeout / browser_crash` 等风险错误的实例摘除阈值
- `amazon.regionGuard.*`：按 region 做站点级短期熔断，防止某个站点连续异常时继续放量
- `amazon.qualityControl.*`：对字段不完整这类质量异常做一次轻量自动重抓
- `redis.*`：可选，用于多台 crawler API 之间做全局去重
- `logging.*`：日志输出

补充说明：

- 这个程序也会读取仓库根目录的 `.env`
- 现在它会优先读取专用环境文件 [config/.env.config-amazon-crawler-api](D:/code/task-processor/config/.env.config-amazon-crawler-api)，然后再用仓库根目录的 `.env` 补齐其他变量
- 如果共享 `.env` 里打开了 `TASK_PROCESSOR_PLATFORM_1688_ENABLED=true`，这个专用环境文件会先把 crawler API 需要的 `platform` 开关钉住，避免被共享配置误伤

## 启动方式

在仓库根目录运行：

```bash
go run ./cmd/amazon-crawler-api -config config/config-amazon-crawler-api.yaml
```

也可以直接运行编译后的程序：

```bash
./amazon-crawler-api -config config/config-amazon-crawler-api.yaml -port 8080
```

默认端口是 `8080`。

Docker 部署文件：

- `deployments/docker/Dockerfile.amazon-crawler-api`
- `deployments/docker/docker-compose.amazon-crawler-api.yml`

## 主要接口

健康检查：

- `GET /health`
- `GET /ready`
- `GET /metrics`

同步抓取单个商品：

- `POST /api/v1/products/fetch`

请求示例：

```json
{
  "asin": "B001234567",
  "region": "us"
}
```

也可以直接传 URL：

```json
{
  "url": "https://www.amazon.com/dp/B001234567"
}
```

返回示例：

```json
{
  "success": true,
  "message": "抓取成功",
  "data": {
    "url": "https://www.amazon.com/dp/B001234567",
    "product": {
      "asin": "B001234567",
      "title": "Demo Product"
    }
  }
}
```

兼容保留的异步任务接口：

- `POST /api/v1/crawl`
- `GET /api/v1/tasks/{id}`
- `GET /api/v1/tasks`
- `DELETE /api/v1/tasks/{id}`
- `GET /api/v1/stats`

## 与 Task 程序配合

如果 `task` 程序要通过 HTTP 调这个 crawler service，需要在它自己的配置里开启：

```yaml
amazon:
  remoteAPI:
    enabled: true
    baseURL: "http://amazon-crawler-api:8080"
    timeout: 300
```

对应的 task 配置文件现在是：

`config/config-task.yaml`

## 多实例去重

如果只部署 1 台 `amazon-crawler-api`，不接 Redis 也能工作。

但如果部署多台 crawler API，建议配置 Redis：

```yaml
redis:
  host: "localhost"
  port: 6379
  password: ""
  db: 0
  pool_size: 10
```

启用后，同一个 `asin + region` 的并发请求会尽量合并为一次真实抓取：

- 首个请求拿到分布式锁并执行抓取
- 其他请求等待共享结果
- 抓取成功后结果会短暂写入 Redis

这样可以显著减少重复抓取、浏览器池争抢和风控压力。

## 指标接口

`GET /metrics` 会以 Prometheus 文本格式输出当前 crawler API 的核心指标。

当前重点包括：

- `crawler_fetch_total`
- `crawler_fetch_success_total`
- `crawler_fetch_failure_total`
- `crawler_retryable_failure_total`
- `crawler_dedupe_shared_hit_total`
- `crawler_failure_by_type{key="..."}`
- `crawler_success_by_region{key="us|uk|jp|..."}`
- `crawler_failure_by_region{key="us|uk|jp|..."}`
- `crawler_failure_by_region_type{region="...",type="..."}`
- `crawler_region_guard_open_total`
- `crawler_region_guard_block_total`
- `crawler_region_guard_open_by_region{key="..."}`
- `crawler_region_guard_block_by_region{key="..."}`
- `crawler_success_by_mode{key="sync_api|async_task"}`
- `crawler_failure_by_mode{key="sync_api|async_task"}`
- `crawler_concurrency_waiting_total`
- `crawler_concurrency_global_inflight`
- `crawler_concurrency_global_limit`
- `crawler_concurrency_region_inflight_by_region{key="..."}`
- `crawler_concurrency_region_limit_by_region{key="..."}`
- `crawler_proxy_failure_by_server{key="..."}`
- `crawler_proxy_cooldown_by_server{key="..."}`
- `crawler_proxy_health_score_by_server{key="..."}`

默认可调参数：

```yaml
amazon:
  productDedupe:
    lockTTLSeconds: 300
    resultTTLSeconds: 600
    waitTimeoutSeconds: 120
    pollIntervalMillis: 500
```

含义：

- `lockTTLSeconds`：单次抓取持锁时间
- `resultTTLSeconds`：共享抓取结果在 Redis 中保留多久
- `waitTimeoutSeconds`：并发请求最多等待首个抓取结果多久
- `pollIntervalMillis`：等待共享结果时的轮询间隔

失败样本留存配置：

```yaml
amazon:
  failureArtifacts:
    enabled: true
    directory: "./tmp/amazon-failure-artifacts"
    captureHTML: true
    maxHTMLBytes: 262144
```

含义：

- `enabled`：是否在抓取失败时保留排查样本
- `directory`：样本输出目录，程序会按日期分子目录保存
- `captureHTML`：是否额外保存失败页面 HTML
- `maxHTMLBytes`：单个失败页面最多保留多少 HTML 字节

风控处置配置：

```yaml
amazon:
  riskControl:
    captchaRecreateThreshold: 1
    authenticationRecreateThreshold: 1
    browserCrashRecreateThreshold: 1
    timeoutRecreateThreshold: 3
    networkRecreateThreshold: 2
    serverErrorRecreateThreshold: 3
```

含义：

- `captcha/authentication/browserCrash` 默认一次命中就摘实例并重建
- `timeout` 默认连续 3 次才摘实例，避免偶发慢请求把池打抖
- `network` 默认连续 2 次重建
- `serverError` 默认连续 3 次重建

站点级风控配置：

```yaml
amazon:
  regionGuard:
    enabled: true
    failureThreshold: 3
    evaluationWindowSeconds: 300
    cooldownSeconds: 180
```

含义：

- 某个 `region` 在统计窗口内连续出现高风险错误超过阈值时，临时打开站点级熔断
- 熔断打开后，这个 `region` 的新请求会直接返回 `region_circuit_open`
- 冷却时间结束后自动恢复，不需要手工干预

当前会计入站点级风控的错误主要包括：

- `captcha`
- `authentication`
- `browser_crash`
- `timeout`
- `network`
- `server_error`

结果质量重抓配置：

```yaml
amazon:
  qualityControl:
    retryOnValidationFailure: true
    validationRetryMaxAttempts: 2
```

含义：

- 当页面抓取成功，但结果因为缺标题、缺主图、缺价格、变体载荷不完整等原因未通过质量校验时，自动再抓一次
- 默认最多尝试 2 次，避免因为单次页面抖动把脏数据直接传给下游
- 质量异常最终会归类成 `product_quality`

## 设计约定

- 抓变体不提供单独接口
- 变体抓取由调用方循环调用 `/api/v1/products/fetch`
- crawler API 只负责“抓数据”，不负责上架、调度、任务编排

这样可以保持 crawler service 足够简单，也方便单独扩容。

## 告警建议

下面这组阈值比较适合作为第一版线上告警基线，先以 `5 分钟` 窗口观察，再按你的真实流量慢慢调。

### P1 告警

- `crawler_failure_rate > 0.30` 持续 `5 分钟`
  说明整体失败率已经明显异常，优先排查页面结构变化、代理池和站点风控。

- `crawler_failure_by_type{key="captcha"}` 在 `5 分钟` 内持续快速上升
  如果同时看到 `crawler_proxy_failure_by_server` 也在升高，通常优先怀疑代理质量或站点风控升级。

- `crawler_region_guard_block_total` 在短时间内明显增加
  说明某个 region 已经频繁进入熔断，通常代表该站点正在被打抖。

- `crawler_concurrency_waiting_total >= 20` 持续 `3 分钟`
  说明当前流量已经开始堆积，请求在 Service 层明显排队，接下来大概率会出现 `system_busy`。

- `crawler_failure_by_type{key="system_busy"}` 在 `5 分钟` 内持续增加
  说明集群容量不够或限流阈值过低，需要看是否扩容 crawler 节点或调大并发上限。

### P2 告警

- `crawler_proxy_health_score_by_server{key="..."}` 持续显著低于其他代理
  用于发现“没完全挂，但明显拖后腿”的代理。

- `crawler_proxy_cooldown_by_server{key="..."}` 在 `10 分钟` 内反复增加
  说明某个代理频繁进入冷却，建议从代理池中临时摘除。

- `crawler_concurrency_global_inflight / crawler_concurrency_global_limit > 0.8` 持续 `5 分钟`
  说明整体容量已经逼近上限，短期内很容易因为流量小波动触发排队和 `429`。

- `crawler_concurrency_region_inflight_by_region{key="us"} / crawler_concurrency_region_limit_by_region{key="us"} > 0.8`
  用于发现某个 region 单独过热，避免误以为是全局容量问题。

- `crawler_retryable_failure_total` 或 `crawler_retryable_failure_by_type` 持续升高
  说明系统还没完全坏，但已经进入“反复重试”的不稳定状态。

### 观察建议

- `failure_rate` 高，同时 `system_busy` 高：
  优先看并发配置和节点数。

- `failure_rate` 高，同时 `captcha` 高：
  优先看代理池、站点风控和 IP 质量。

- `region_guard_block_total` 高，但全局并发不高：
  更像是某个 region 被打坏了，不一定是整体容量问题。

- `proxy_health_score_by_server` 某几个代理持续明显偏低：
  可以直接从代理池里先摘掉这些代理，再观察整体成功率是否恢复。
