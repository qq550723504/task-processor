# Amazon Crawler API 监控清单

这套清单面向 `kube-prometheus-stack` 一类 Prometheus Operator 环境，提供：

- `PodMonitor`
- `PrometheusRule`
- Grafana dashboard `ConfigMap`

## 适用场景

推荐用于当前 `amazon-crawler-api` 的 `DaemonSet` 部署，因为需要保留每个 pod 的独立视角。

相比 `ServiceMonitor`，这里默认使用 `PodMonitor`，原因是：

- 更容易按 pod 对比成功率和失败率
- 更适合观察每台 agent 机器上的 crawler 差异
- 不会被 Service 负载均衡掩盖单 pod 异常

## 部署前提

集群中需要已经安装：

- Prometheus Operator / `kube-prometheus-stack`
- Grafana sidecar dashboard 自动发现能力

常见要求：

- `monitoring.coreos.com/v1` CRD 已存在
- Grafana sidecar 会加载带 `grafana_dashboard: "1"` 标签的 `ConfigMap`

## 应用方式

```bash
kubectl apply -k deployments/kubernetes/monitoring/amazon-crawler-api
```

## 默认监控内容

- 每 pod 抓取吞吐
- 每 pod 失败率
- 每 pod 并发占用
- 按错误类型拆分失败趋势
- `region_guard` 阻断情况
- 浏览器池当前实例数、配置池大小和活跃重建数
- 最近 10-30 分钟的 `processor_unavailable`、任务提交失败、`dedupe` 等待超时
- Pod 视角的稳定性汇总表，方便对比池缩容和重建失败

## 默认告警

- 某 pod 5 分钟失败率持续超过 80%
- 某 pod 15 分钟内重复重启
- 某 pod 并发占用持续超过 80%
- 某 pod 的 `region_guard_block_total` 快速增加
- 某 pod 在最近 10 分钟内出现 `processor_unavailable`
- 某 pod 在最近 10 分钟内出现浏览器池初始化失败
- 某 pod 在最近 15 分钟内出现浏览器实例重建失败
- 某 pod 的浏览器池实例数持续低于配置池大小
- 某 pod 连续 10 分钟没有可用浏览器实例
- 某 pod 长时间存在未完成的实例重建
- 某 pod 在最近 10 分钟内出现任务提交失败
- 某 pod 在最近 10 分钟内 `dedupe` 等待超时快速增加

## 说明

- `PodMonitor` 会抓 `task-processor` 命名空间里 `app=amazon-crawler-api` 的 pod
- dashboard 默认使用 Prometheus 数据源变量 `${DS_PROMETHEUS}`
- 如果你的 Grafana sidecar 只监听 `monitoring` 命名空间，这套清单可以直接用
