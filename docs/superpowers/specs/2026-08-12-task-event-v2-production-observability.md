# TaskEvent V2 生产发布与可观测性设计

**状态：已确认；仓库内实施完成，生产执行待单独授权**
**日期：2026-08-12**

## 背景与目标

`TaskEventV2` 已在代码中完成生产者迁移、兼容解码和 legacy 指标。生产集群尚未运行包含该改动的 SHEIN 消费者镜像，也没有 Prometheus Operator、`PodMonitor` 或 `PrometheusRule` CRD。因此，当前不能证明所有实际队列均已升级，也不能开始 14 天的 legacy 归零观察。

本次发布的目标是：

1. 让所有**正在运行**的 SHEIN task 消费者运行同一份含 V2 兼容解码的镜像。
2. 用持久化 Prometheus 连续保存至少 21 天数据，可靠计算 14 天 legacy 事件窗口。
3. 以一个 StatefulSet 副本为灰度起点；只有指标、消费和健康检查都通过后才滚动剩余实例。
4. 为 legacy 事件再次出现、抓取失败和存储风险建立可操作告警。

不在范围内：移除 legacy 解码分支、修改 Listing Control Plane 的 ID-only dispatch、启动停用的 `shein-listing-store-976`，或改变 RabbitMQ 的 ack/claim 顺序。

## 已核实的生产基线

- 集群只有一个可调度节点；Prometheus Operator CRD 和 `monitoring` 命名空间均不存在。
- 节点约 32 CPU、31 GiB 内存，当前内存使用约 70%；根文件系统约 610 GiB 可用。
- 当前活跃消费者是 `shein-listing-shard`（10 个 ready 副本）和 `shein-listing-store-883`（1 个 ready 副本）。`shein-listing-store-976` 为 0 副本，不参与本次发布。
- 仓库已有 SHEIN `PodMonitor` / `PrometheusRule`，但依赖未安装的 Operator；其 7 天保留期不足，Alertmanager 的固定 node affinity 也已失效。
- `store-883` 没有 `shein-listing-role=service` 标签，故现有 PodMonitor 只会抓取 shard。为避免漏观测，新增一个明确选择 `app=shein-listing-store-883` 的 PodMonitor，而不依赖手工补标签。

## 方案选择

采用开源的 `kube-prometheus-stack`，而不是自建抓取器或本地轮询：它提供 Prometheus Operator、Grafana、告警规则和 PVC 生命周期管理，且仓库现有清单已针对该接口编写。

### 监控安装

- 固定 chart 版本并将该版本写入仓库部署说明；实际安装前用 `helm show chart` 再校验版本与 Kubernetes 1.36 的兼容性。
- 在 `monitoring` namespace 安装 release；不创建公网 Ingress。Grafana 仅通过受控 port-forward / 已有内部访问方式使用。
- Prometheus 使用 `local-path` RWO PVC，申请 30 GiB，保留期 `21d`。不设置 `retentionSize`，防止容量阈值先于时间阈值淘汰 14 天证据。
- 配置明确的 requests/limits，使 Prometheus、Grafana、Alertmanager 和 kube-state-metrics 的总常态内存预算不超过约 4 GiB；安装前再次核对节点可用内存。
- 删除过期的固定 Alertmanager node affinity；单节点环境让调度器自行选择当前节点。
- 保持 30 秒 scrape/evaluation 间隔。所有 PrometheusRule/PodMonitor 采用显式、与 Helm release 无关的 selector，确保仓库资源能被发现。

### 指标与告警

保留 `task_event_decoded_total{schema_version="legacy"}` 的低基数标签设计。新增以下规则：

| 规则 | 条件 | 用途 |
| --- | --- | --- |
| `TaskEventV2LegacyDecodeObserved` | `increase(task_event_decoded_total{schema_version="legacy"}[15m]) > 0` | 新 legacy 输入立即可见；会重置迁移观察窗口。 |
| `TaskEventV2MetricsScrapeMissing` | 11 个预期活跃消费者中任一抓取目标连续 5 分钟未 healthy（包括零目标发现） | 防止把“未抓到指标”误判成零 legacy。 |
| `PrometheusPVCNearFull` | Prometheus PVC 可用率低于 20%，持续 15 分钟 | 保证 21 天观察窗口不会因磁盘压力失真。 |

现有 SHEIN 业务告警继续保留。Grafana 增加一个 V2 迁移面板：legacy decode 的 15 分钟速率、14 天 increase、按 pod 的 `up`，以及当前活跃消费者库存。

### 抓取目标

1. 保留现有 `shein-listing` PodMonitor，抓取有 `shein-listing-role=service` 的 shard，端口 `metrics`、路径 `/metrics`。
2. 增加 `shein-listing-store-883` PodMonitor，匹配 `app=shein-listing-store-883`，使用同一端口、重标标签和 30 秒间隔。
3. 用 PromQL 中 `up` 的实际 target 数与发布库存交叉验证：灰度时为 1 个新 shard；全量时应为 11 个活跃消费者（10 shard + store-883）。

### V2 灰度与回滚

1. 以当前提交构建不可变 SHEIN listing 镜像 tag，并记录 digest；先在本地执行完整 Go 测试、镜像构建和 manifest server-side dry-run。
2. 先安装并验证监控：CRD 就绪、两个 PodMonitor 均被 Prometheus 发现、Grafana 可查询 `/metrics` 的基础 target、PVC 已绑定。
3. 将 `shein-listing-shard` StatefulSet 设置为 partitioned rollout，仅更新一个副本；等待 ready、查看该 pod 的 `/metrics` 含 V2 metric，并用 Prometheus 确认 target `up=1`。
4. 观察至少一个 scrape interval 后，核对消费者健康、RabbitMQ queue/claim 行为、错误率和 legacy decode 计数。任何指标抓取缺失、崩溃、持续积压或业务错误恶化，立即把该 workload image 回退到发布前 digest，停止后续滚动。
5. 灰度通过后，滚动余下 9 个 shard，再滚动 `store-883`；每一阶段等待 rollout 完成并复核 target、ready、错误率和 legacy 计数。
6. `store-883` 当前没有可信的 Git 声明式完整资源，且其历史 last-applied 配置的 replicas 值与当前运行状态不一致。本次仅对该既有 Deployment 做受控 image patch / rollback，绝不 apply 旧完整清单；将其完整 IaC 回收列为后续独立工作，避免误缩容。

## 验收与迁移门槛

发布完成只说明 V2 兼容消费者已上线，不代表可以删除 legacy 分支。删除前必须同时满足：

1. 连续两个消费者发布周期使用 V2 镜像；
2. 活跃 workload 清单中 `legacy_task_event_consumers = 0`；
3. 在所有 task/crawler 队列上，`increase(task_event_decoded_total{schema_version="legacy"}[14d]) == 0` 连续完整 14 天，且这一期间 scrape 健康。

每次检查保存 workload image digest、replica 数、PromQL 结果和告警状态。任何 legacy 事件、消费者回退或指标采集中断都会重新开始 14 天窗口。

## 实施前置审批

本文获批后才会：修改部署/监控清单、创建版本化发布计划、执行 Helm 安装、推送镜像或写入生产集群。每个阶段的实际集群变更均会在执行前重新检查当前 Git、CI、镜像 digest、workload 状态和可用容量。
