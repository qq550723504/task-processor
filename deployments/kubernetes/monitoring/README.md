# 监控运维说明

当前仓库里的监控能力由这几部分组成：

- `kube-prometheus-stack`
- `amazon-crawler-api` 指标、告警和 Grafana 看板
- `shein-listing` 指标、告警和 Grafana 看板
- 企业微信告警适配器
- Grafana 域名入口

## 当前访问入口

- Grafana: [https://monitoring.shuomiai.com](https://monitoring.shuomiai.com)
- Dashboard 标题: `Amazon 爬虫 Pod 看板`

Grafana 管理员密码可通过下面命令查看：

```bash
kubectl -n monitoring get secret monitoring-grafana -o jsonpath="{.data.admin-password}" | base64 -d && echo
```

## 当前部署清单

- `deployments/kubernetes/monitoring/kube-prometheus-stack-values.yaml`
- `deployments/kubernetes/monitoring/amazon-crawler-api`
- `deployments/kubernetes/monitoring/shein-listing`
- `deployments/kubernetes/monitoring/alertmanager-wecom`
- `deployments/kubernetes/monitoring/grafana-ingress`
- `deployments/kubernetes/cert-manager/letsencrypt-prod`

## 建议部署顺序

```bash
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update
helm upgrade --install monitoring prometheus-community/kube-prometheus-stack \
  --namespace monitoring \
  --create-namespace \
  -f deployments/kubernetes/monitoring/kube-prometheus-stack-values.yaml

kubectl apply -k deployments/kubernetes/cert-manager/letsencrypt-prod
kubectl apply -k deployments/kubernetes/monitoring/amazon-crawler-api
kubectl apply -k deployments/kubernetes/monitoring/shein-listing
kubectl apply -k deployments/kubernetes/monitoring/alertmanager-wecom
kubectl apply -k deployments/kubernetes/monitoring/grafana-ingress
```

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

## 集群兼容性约束

当前 k3s 集群存在跨节点 Pod 网络访问异常或不稳定的现象，因此仓库里固化了两个兼容性配置：

- `letsencrypt-prod` 的 HTTP-01 solver 固定到 `vm-4-17-ubuntu`
- Alertmanager 固定到 `vm-4-17-ubuntu`

如果后续集群网络恢复正常，可以再评估是否移除这些节点固定策略。

## 当前已知注意点

- Grafana 使用 `local-path` 持久卷，因此 values 里关闭了 `initChownData`，避免滚动升级时权限修复失败
- `monitoring.shuomiai.com` 使用 `traefik + cert-manager(letsencrypt-prod)` 证书链路
- `amazon-crawler-api` 监控依赖 Prometheus Operator CRD 和 Grafana dashboard sidecar
