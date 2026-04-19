# SHEIN Listing 监控清单

这套清单面向 `kube-prometheus-stack` / Prometheus Operator 环境，提供：

- `PodMonitor`
- `PrometheusRule`
- 2 份 Grafana dashboard `ConfigMap`

## 应用方式

```bash
kubectl apply -k deployments/kubernetes/monitoring/shein-listing
```

## 默认监控内容

- 每 pod 的上架完成量、失败量、重试量
- 认证过期、Cookie 加载失败、每日限额、上架额度耗尽、SKU 重复等原因趋势
- 平均等待时长、平均处理时长
- 业务看板：按上架结果、问题恢复趋势、异常原因聚合观察业务波动
- 店铺视角：支持受控的 Top 店铺榜，并可按异常原因切换
- 排障入口：在看板里直接给出 `/stats` 查询模板
- 业务告警：回补率过低、重试压力过高、认证过期激增

## 说明

- `PodMonitor` 会抓 `task-processor` 命名空间里 `app=shein-listing` 的 pod
- dashboard 默认使用 Prometheus 数据源 `prometheus`
- `stats` 更适合排障明细，Grafana 主要消费 `/metrics`
- 店铺榜指标只导出受控的 Top N 店铺，不会给所有指标直接挂 `store_id` 标签
- 告警注解已经内置 Grafana 看板链接和常用 `/stats` runbook 链接，便于企业微信里直接跳转排障
