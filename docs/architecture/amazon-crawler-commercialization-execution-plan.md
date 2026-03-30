# Amazon Crawler Commercialization Execution Plan

本文档把 Amazon 爬虫商用化清单进一步拆成可以执行的任务计划。

目标不是一次性把所有能力做完，而是优先把最影响稳定性的部分补齐，尽快把 `amazon-crawler-api` 提升到“可商用第一阶段”。

建议按 3 个阶段推进：

1. 第一阶段：先把稳定性底盘补齐
2. 第二阶段：把风控和代理治理做实
3. 第三阶段：把运维和灰度能力补全

相关背景文档：

- [amazon-crawler-commercialization-checklist.md](D:/code/task-processor/docs/architecture/amazon-crawler-commercialization-checklist.md)
- [centralized-deployment-plan.md](D:/code/task-processor/docs/architecture/centralized-deployment-plan.md)

## 1. 第一阶段

目标：

- 让多节点 crawler API 在共享 Redis 下稳定运行
- 补齐最基础的业务监控、结果校验和失败留痕
- 先把“能不能稳定跑”解决掉

### 1.1 抓取结果完整性校验

任务：

- 增加商品关键字段校验器
- 对空标题、空价格、空主图、异常变体做统一判定
- 对低质量结果返回明确错误类型

建议改动模块：

- [processor.go](D:/code/task-processor/internal/crawler/amazon/processor.go)
- [product_checker.go](D:/code/task-processor/internal/crawler/amazon/product_checker.go)
- [crawler_service.go](D:/code/task-processor/internal/crawler/amazon/crawler_service.go)
- `internal/crawler/amazon/result_validator.go`（建议新增）

验收标准：

- 缺关键字段的抓取结果不会被当成成功结果返回
- 日志里能区分“抓取失败”和“结果不完整”
- 至少覆盖标题、价格、主图三类必填字段

### 1.2 失败样本留存

任务：

- 抓取失败时保留截图
- 可选保留 HTML 片段或文本摘要
- 记录 URL、region、实例 ID、错误类型

建议改动模块：

- [pool_manager.go](D:/code/task-processor/internal/crawler/amazon/browser/pool_manager.go)
- [processor.go](D:/code/task-processor/internal/crawler/amazon/processor.go)
- `internal/crawler/amazon/failure_artifact_store.go`（建议新增）

验收标准：

- 任意一次失败可以在本地文件或对象存储中找到对应样本
- 样本文件名或元数据中能定位 URL / region / time / error type

### 1.3 业务监控指标

任务：

- 增加抓取成功率
- 增加抓取失败率
- 增加 captcha 命中率
- 增加 timeout 比例
- 增加抓取耗时统计
- 增加去重命中率

建议改动模块：

- [api_service.go](D:/code/task-processor/internal/crawler/amazon/api_service.go)
- [crawler_service.go](D:/code/task-processor/internal/crawler/amazon/crawler_service.go)
- [health.go](D:/code/task-processor/internal/infra/httpx/health.go)
- `internal/crawler/amazon/metrics.go`（建议新增）

验收标准：

- 可以通过 HTTP 或日志看到成功率、失败率、P95
- 可以区分正常失败和 captcha/timeout 失败

### 1.4 去重参数运营化

任务：

- 已完成的 `productDedupe` 配置进一步接入监控
- 记录锁命中率、等待超时次数、共享结果复用率

建议改动模块：

- [crawler_service.go](D:/code/task-processor/internal/crawler/amazon/crawler_service.go)
- [config-amazon-crawler-api.yaml](D:/code/task-processor/config/config-amazon-crawler-api.yaml)

验收标准：

- 可以看到“并发合并是否有效”
- 可以根据数据调 `lockTTL` / `resultTTL` / `waitTimeout`

### 1.5 第一阶段并行建议

可以并行做的任务：

- `结果校验`
- `失败样本留存`
- `业务监控`

建议最后再合并：

- `去重监控`

原因：

- 去重监控依赖失败分类和成功统计

## 2. 第二阶段

目标：

- 把最核心的商用短板补上，也就是代理和风控治理

### 2.1 代理池

任务：

- 从单 `proxyServer` 演进到代理池
- 支持多代理配置
- 支持按 region 选代理
- 支持代理失效摘除
- 支持冷却恢复

建议改动模块：

- [browser_pool.go](D:/code/task-processor/internal/crawler/amazon/browser/browser_pool.go)
- `internal/crawler/shared/browser/proxy_provider.go`（建议新增）
- `internal/crawler/shared/browser/proxy_pool.go`（建议新增）
- [config-amazon-crawler-api.yaml](D:/code/task-processor/config/config-amazon-crawler-api.yaml)

验收标准：

- 不同请求可以命中不同代理
- 单代理故障不会拖垮整体服务
- 可以按 region 强制走指定代理池

### 2.2 风控退避策略

任务：

- captcha 命中后切代理
- captcha 命中后切实例
- 连续命中风控时对代理/region 退避
- 重试次数和退避时长配置化

建议改动模块：

- [captcha_handler.go](D:/code/task-processor/internal/crawler/amazon/captcha_handler.go)
- [product_checker.go](D:/code/task-processor/internal/crawler/amazon/product_checker.go)
- [pool_manager.go](D:/code/task-processor/internal/crawler/amazon/browser/pool_manager.go)
- `internal/crawler/amazon/risk_backoff.go`（建议新增）

验收标准：

- captcha 命中不会只是原地等待刷新
- 被风控的实例/代理会进入冷却
- 连续高风控时成功率不会快速塌陷

### 2.3 限流

任务：

- 按 region 限流
- 按代理限流
- 按实例并发限制
- 热门商品请求打平

建议改动模块：

- [crawler_service.go](D:/code/task-processor/internal/crawler/amazon/crawler_service.go)
- `internal/crawler/amazon/rate_limiter.go`（建议新增）
- Redis 共享状态（可选）

验收标准：

- 高并发下不会把浏览器池和代理同时打爆
- 热门商品不会造成请求雪崩

### 2.4 第二阶段并行建议

可以并行做的任务：

- `代理池`
- `限流`

依赖后再接入的任务：

- `风控退避`

原因：

- 风控退避策略需要依赖代理池和限流策略一起工作才有意义

## 3. 第三阶段

目标：

- 把 crawler service 做成真正可运维、可灰度、可持续演进的商用服务

### 3.1 会话与指纹隔离

任务：

- 代理和浏览器上下文绑定
- 指纹策略稳定化
- 高风险实例隔离
- 会话污染隔离

建议改动模块：

- [browser_pool.go](D:/code/task-processor/internal/crawler/amazon/browser/browser_pool.go)
- [manager.go](D:/code/task-processor/internal/crawler/amazon/browser/manager.go)
- `internal/crawler/shared/browser/session_profile.go`（建议新增）

### 3.2 配置热更新

任务：

- 代理池动态更新
- 退避参数动态更新
- 限流参数动态更新
- 黑白名单动态更新

建议改动模块：

- [config-amazon-crawler-api.yaml](D:/code/task-processor/config/config-amazon-crawler-api.yaml)
- `internal/crawler/amazon/runtime_config.go`（建议新增）
- Redis / 管理端配置同步模块

### 3.3 灰度发布与回滚

任务：

- 新旧版本并行
- 小流量灰度
- 快速回滚
- 节点排空

建议改动模块：

- 部署脚本
- LB/网关配置
- 健康检查与 readiness 标准

### 3.4 节点自动摘除

任务：

- 连续失败节点自动标记不健康
- readiness 返回失败
- LB 自动摘流量
- 恢复后再放回

建议改动模块：

- [api_service.go](D:/code/task-processor/internal/crawler/amazon/api_service.go)
- [health.go](D:/code/task-processor/internal/infra/httpx/health.go)
- `internal/crawler/amazon/node_health.go`（建议新增）

## 4. 模块归属建议

为了避免后面多人并行开发时互相打架，建议按职责拆模块归属。

### 4.1 抓取核心组

负责：

- 结果校验
- captcha 处理
- 错误分级
- 失败样本留存

主改文件：

- `internal/crawler/amazon/*`

### 4.2 浏览器与代理组

负责：

- 浏览器池
- 代理池
- 会话隔离
- 风控退避

主改文件：

- `internal/crawler/amazon/browser/*`
- `internal/crawler/shared/browser/*`

### 4.3 服务治理组

负责：

- 业务监控
- readiness / health
- 配置热更新
- 节点摘除

主改文件：

- `internal/crawler/amazon/api_service.go`
- `internal/crawler/amazon/crawler_service.go`
- `internal/infra/httpx/*`

## 5. 验收里程碑

建议按下面 3 个里程碑验收。

### M1 可商用基础版

完成：

- 结果完整性校验
- 失败样本留存
- 抓取成功率 / captcha / timeout 指标
- 去重命中统计

### M2 稳定运行版

完成：

- 代理池
- captcha 风控退避
- region/代理限流

### M3 可运维版

完成：

- 会话与指纹隔离
- 配置热更新
- 节点自动摘除
- 灰度与回滚

## 6. 建议排期

如果按较紧凑节奏推进，可以参考：

### 第 1 周

- 结果校验
- 失败样本留存
- 基础业务指标

### 第 2 周

- 去重指标
- 节点健康增强
- 第一阶段联调

### 第 3-4 周

- 代理池
- 限流
- 风控退避

### 第 5-6 周

- 会话隔离
- 配置热更新
- 灰度和摘流量

## 7. 结论

这份执行计划的核心思路是：

- 先补稳定性底盘
- 再补代理和风控治理
- 最后补运维和灰度能力

这样推进可以避免一上来做很大的体系化重构，同时能尽快把 Amazon crawler 提升到真正可商用的水平。
