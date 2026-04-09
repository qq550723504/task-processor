# Amazon Crawler Commercialization Checklist

本文档用于梳理 `amazon-crawler-api` 从“可运行”走向“可商用稳定运行”还缺少的关键能力。

这里的“商用”不只是指功能可用，而是指：

- 可以长期稳定运行
- 可以承受多节点部署和持续流量
- 出现风控、验证码、代理失效、页面变更时有恢复能力
- 有足够的监控、告警和排障信息

## 1. 当前现状

当前代码已经具备一些不错的基础能力：

- 独立 `amazon-crawler-api` 入口
- 独立配置文件
- 浏览器池
- 基础健康检查
- 页面错误检测
- 验证码检测与简单绕过
- 同商品并发请求的 Redis 去重

相关位置：

- [main.go](D:/code/task-processor/cmd/amazon-crawler-api/main.go)
- [config-amazon-crawler-api.yaml](D:/code/task-processor/config/config-amazon-crawler-api.yaml)
- [browser_pool.go](D:/code/task-processor/internal/crawler/amazon/browser/browser_pool.go)
- [pool_manager.go](D:/code/task-processor/internal/crawler/amazon/browser/pool_manager.go)
- [captcha_handler.go](D:/code/task-processor/internal/crawler/amazon/captcha_handler.go)
- [crawler_service.go](D:/code/task-processor/internal/crawler/amazon/crawler_service.go)
- [amazon-crawler-runtime-flow.md](D:/code/task-processor/docs/architecture/amazon-crawler-runtime-flow.md)

但从商用标准看，当前仍然更接近“内部生产试跑”而不是“高稳定商用服务”。

## 2. 必须做

这些能力我认为是商用前必须具备的。

### 2.1 代理池

当前 `browser.proxyServer` 更像单代理配置，不足以支撑商用。

至少需要：

- 多代理池
- 按 region 分配代理
- 代理失败自动摘除
- 代理冷却恢复
- 代理可用率统计

否则会出现：

- 单代理故障导致整批请求失败
- 某 region 被封后没有绕行能力
- 高并发时风控集中打到同一出口

### 2.2 风控治理

当前验证码处理主要是：

- 等待
- 刷新
- 点击空白区域

这适合开发期，不适合商用高稳定。

商用至少需要：

- captcha 命中后切代理
- captcha 命中后切实例
- 按代理/region 触发退避
- 连续命中风控时临时熔断

否则同类请求会持续打在已被风控的出口上。

### 2.3 业务监控与告警

只有 `/health` 和 `/ready` 不够。

至少应补：

- 抓取成功率
- 抓取失败率
- captcha 命中率
- 超时率
- 平均耗时 / P95 / P99
- 各 region 成功率
- 共享去重命中率
- 代理失败率

并配合告警：

- 成功率持续下降
- captcha 比例突增
- 某个 region 全量失败
- 某个代理池失败率过高

### 2.4 限流与退避

商用不能只靠浏览器池大小来控制流量。

至少需要：

- 按 region 限流
- 按代理限流
- 按实例限流
- 连续失败退避
- 热门商品防击穿

否则会出现请求高峰时：

- 浏览器池耗尽
- 同类请求堆积
- 风控命中率升高

### 2.5 抓取结果完整性校验

当前已有页面错误检测，但还不够业务化。

至少需要检查：

- 标题是否为空
- 价格是否缺失
- 主图是否缺失
- 变体是否异常
- 详情字段是否明显不完整

对于缺关键字段的结果，要支持：

- 自动重抓
- 标记低质量结果
- 返回明确错误类型

### 2.6 节点故障摘除

多台 crawler API 部署后，必须支持异常节点自动摘流量。

至少需要：

- 连续失败阈值
- 自动标记不健康
- 被负载均衡器摘除
- 恢复后再进入流量

否则某台有问题的节点会持续拖垮整体成功率。

## 3. 应该做

这些能力不是“没做就不能跑”，但很影响商用可维护性和稳定性。

### 3.1 会话与指纹隔离

建议逐步做到：

- 代理和浏览器上下文绑定
- 指纹和会话稳定化，而不是只随机
- 不同流量来源隔离
- 高风险实例黑名单

目标是减少“不同请求互相污染”的问题。

### 3.2 配置热更新

以下参数最好支持动态调整，而不是只改 YAML 重启：

- 代理池
- 黑白名单
- region 开关
- 退避参数
- 限流参数
- 去重 TTL

这样线上调优效率会高很多。

### 3.3 错误分级处理

当前错误检测已经有基础，但商用建议进一步分层：

- captcha
- timeout
- network
- proxy failure
- product not found
- page structure changed
- server side transient error

不同错误走不同恢复策略，而不是统一重试。

### 3.4 样本留存与排障材料

失败时建议至少支持保留：

- 页面截图
- HTML 片段
- URL
- region
- proxy 标识
- 浏览器实例 ID
- 错误类型

否则页面变更时排查会很慢。

### 3.5 灰度发布

商用版本升级建议支持：

- 小流量灰度
- 新旧版本并行
- 回滚快捷

避免 crawler 逻辑一变，全量节点一起出问题。

## 4. 可以后做

这些能力有价值，但不必作为第一波商用门槛。

### 4.1 第三方验证码识别或人工打码

当前阶段可先通过代理和退避降低 captcha 压力。

### 4.2 代理评分系统

后续可以做：

- 成功率评分
- 响应时间评分
- captcha 风险评分
- region 适配评分

### 4.3 多地区爬虫池

如果后面规模继续增大，可以进一步演进到：

- region 专属 crawler 池
- 多机房部署
- 就近出口策略

### 4.4 结果质量评分

可以对抓取结果做自动质量打分：

- 完整性
- 一致性
- 与历史结果差异

### 4.5 运营后台

后续可以提供可视化面板，用于观察：

- 节点状态
- 代理状态
- 成功率
- captcha 率
- region 热点

## 5. 推荐执行顺序

如果按投入产出比排序，我建议优先级如下：

1. 代理池
2. 风控退避策略
3. 业务指标和告警
4. 限流与退避
5. 结果完整性校验
6. 会话/指纹隔离
7. 配置热更新
8. 灰度发布

## 6. 近期建议目标

如果只规划一轮“商用化第一阶段”，建议目标定为：

- 多代理池
- Redis 去重继续保留
- region/代理限流
- captcha/timeout 统计
- 抓取成功率和 P95 指标
- 关键字段完整性校验
- 失败样本留存

做到这一步后，`amazon-crawler-api` 会比较接近“可以稳定商用”的基础版本。

## 7. 结论

当前 Amazon crawler 已经具备：

- 独立部署能力
- 基础浏览器池
- 基础健康检查
- 商品级去重

但距离真正商用，还差的核心不是“还能不能抓到页面”，而是：

- 风控治理
- 代理治理
- 可观测性
- 限流与恢复策略

这几层补齐后，系统的稳定性、扩展性和可运维性才会真正上一个台阶。
