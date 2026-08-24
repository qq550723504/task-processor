# 监控运维说明

本目录定义受版本控制的监控目标状态；它不表示任一集群已经安装了这些组件。生产变更前必须先完成本文件的 preflight，并获得单独的生产写入授权。

监控能力由这几部分组成：

- `kube-prometheus-stack`
- `amazon-crawler-api` 指标、告警和 Grafana 看板
- `shein-listing` 指标、告警和 Grafana 看板
- 企业微信告警适配器
- Grafana 域名入口

## 访问与凭证

本次 TaskEvent V2 发布不创建公网 Grafana 或 Alertmanager Ingress。安装后仅通过已获授权的内部访问方式或临时 `kubectl -n monitoring port-forward service/monitoring-grafana 3000:80` 访问 Grafana。管理员凭证只保留在 Kubernetes Secret 中，不在 Git、终端记录或发布证据中输出。

## 当前部署清单

- `deployments/kubernetes/monitoring/kube-prometheus-stack-values.yaml`
- `deployments/kubernetes/monitoring/amazon-crawler-api`
- `deployments/kubernetes/monitoring/shein-listing`
- `deployments/kubernetes/monitoring/alertmanager-wecom`
- `deployments/kubernetes/monitoring/grafana-ingress`
- `deployments/kubernetes/cert-manager/letsencrypt-prod`

## 固定版本与部署顺序

- Chart: `oci://ghcr.io/prometheus-community/charts/kube-prometheus-stack`
- Version: `88.3.0`
- OCI digest: `sha256:2b2f2c5c6f76ff661bdb34e6ae3410e01258f39632bffa52b94c0d42a1da0be6`
- Storage: default `local-path`; Prometheus 使用 30Gi RWO PVC，保留 21 天。

安装前必须确认存储类和节点资源：

```bash
kubectl get storageclass local-path
kubectl top node
```

在已获生产授权后执行：

```bash
helm upgrade --install monitoring oci://ghcr.io/prometheus-community/charts/kube-prometheus-stack \
  --version 88.3.0 \
  --namespace monitoring \
  --create-namespace \
  --atomic \
  --timeout 10m \
  -f deployments/kubernetes/monitoring/kube-prometheus-stack-values.yaml

kubectl apply -k deployments/kubernetes/monitoring/amazon-crawler-api
kubectl apply -k deployments/kubernetes/monitoring/shein-listing
```

企业微信告警和公网 Grafana Ingress 是独立的既有运维决策，不属于 TaskEvent V2 发布；需要时应另行审批和应用对应清单。

## 企业微信告警

企业微信告警通过 `alertmanager-wecom` 适配器转发。

部署前需要先创建 Secret：

```bash
kubectl -n monitoring create secret generic alertmanager-wecom-secret \
  --from-literal=webhook-url='https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=YOUR_KEY'
```

当前默认只转发：

- `service=amazon-crawler-api`
- `service=shein-listing`

## 当前监控覆盖

- 每个 `amazon-crawler-api` pod 的抓取吞吐
- 每个 pod 的失败率
- 每个 pod 的并发占用
- 按错误类型拆分失败趋势
- `region_guard` 阻断趋势
- 浏览器池初始化失败、实例重建成功/失败、活跃重建数量
- 浏览器池当前实例数与配置池大小对比、`processor_unavailable` 快速定位
- 任务提交失败和 `dedupe` 等待超时
- 每个 `shein-listing` pod 的上架成功、失败、重试
- `shein-listing` 的认证过期、Cookie 加载失败、每日限额、上架额度耗尽、SKU 重复
- `shein-listing` 的平均等待时长、平均处理时长
- 企业微信中文告警

## Amazon Crawler 当前分层部署

当前 `amazon-crawler-api` 已拆成两套 `DaemonSet`，但 pod 仍保留统一标签 `app=amazon-crawler-api`，所以现有 `Service`、`PodMonitor`、告警规则和 Grafana 看板不需要额外修改：

- `amazon-crawler-api-lite`：运行在 `2C4G` 节点
- `amazon-crawler-api-heavy`：运行在 `4C8G` 节点

监控上仍然会按 pod 维度展示；如果要区分规格层级，可以后续在看板里追加 `crawler-tier` 维度。

## 当前已知注意点

- Grafana 使用 `local-path` 持久卷，因此 values 里关闭了 `initChownData`，避免滚动升级时权限修复失败
- `amazon-crawler-api` 监控依赖 Prometheus Operator CRD 和 Grafana dashboard sidecar
- TaskEvent V2 兼容性移除前，必须同时满足两个消费者发布周期完成、所有活跃消费者均为 V2 镜像，并且 `increase(task_event_decoded_total{schema_version="legacy",kubernetes_namespace="task-processor"}[14d]) == 0` 持续完整 14 天；任一抓取中断、legacy 事件或回滚都会重置窗口。
