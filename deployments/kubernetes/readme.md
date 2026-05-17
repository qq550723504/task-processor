# Kubernetes 部署清单

当前仓库内提供这些 K8s 清单：

- `cert-manager/letsencrypt-prod`
- `amazon-crawler-api`
- `amazon-crawler-external-lb`
- `monitoring/alertmanager-wecom`
- `monitoring/amazon-crawler-api`
- `zitadel/local`
- `shein-listing`
- `temu-listing`
- `amazon-listing`

## ZITADEL 生产说明

当前线上 `https://auth.shuomiai.com` 已不再使用 `zitadel` 命名空间内置的
`zitadel-postgres16`。

自 `2026-05-16` 起，线上 ZITADEL 数据库改为复用 `yudao-cloud` 命名空间内
现有的 PostgreSQL 18 实例：

- 实例 Service：`postgresql-v18.yudao-cloud.svc.cluster.local:5432`
- 独立数据库：`zitadel_auth`
- 独立用户：`zitadel`

这次切换没有把 ZITADEL 并进业务库 `ruoyi-vue-pro`。身份库和业务库仍然分
离，只是共用同一 PostgreSQL 实例。

旧资源：

- `StatefulSet/zitadel-postgres16`
- `Service/zitadel-postgres16`
- `Service/zitadel-postgres16-hl`
- `Secret/zitadel-postgres16`
- `PVC/data-zitadel-postgres16-0`

已经在 `2026-05-16` 清理完成。当前没有“切回旧 Pod/旧 PVC”的快速回滚点。
如果后续需要回退，只能基于外部备份、导出文件或重新创建独立实例后再恢复。

建议部署顺序：

1. `yudao-cloud`
2. RabbitMQ / Redis
3. `zitadel/local`（如果本地验证 ListingKit OIDC 登录）
4. `amazon-crawler-api`（如果 Amazon 上架依赖远程爬虫）
5. `shein-listing`
6. `temu-listing`
7. `amazon-listing`

示例：

```bash
kubectl apply -k deployments/kubernetes/shein-listing/overlays/staging
kubectl apply -k deployments/kubernetes/temu-listing/overlays/staging
kubectl apply -k deployments/kubernetes/amazon-listing/overlays/staging
```

## Amazon Crawler 节点分层

`amazon-crawler-api` 现在拆成两套 `DaemonSet`：

- `amazon-crawler-api-lite`：给 `2C4G` 节点
- `amazon-crawler-api-heavy`：给 `4C8G` 节点

当前仓库改成按节点标签调度，不再写死主机名：

- `task-processor/crawler-tier=lite`
- `task-processor/crawler-tier=heavy`

这样做是为了避免浏览器爬虫继续落到低规格节点上。最近线上采样里，单个 Pod 内存峰值已经接近 `2.5Gi`，继续全量铺到小机器上会触发 `OOMKilled`，并放大 `region_circuit_open` 和 `system_busy`。

建议规则：

- `2C2G`：不要调度 `amazon-crawler-api`
- `2C4G`：只跑 `lite`
- `4C8G`：跑 `heavy`

首次切换前，先给节点打标签：

```bash
# 2C4G 节点
kubectl label node <node-name> task-processor/crawler-tier=lite --overwrite

# 4C8G 节点
kubectl label node <node-name> task-processor/crawler-tier=heavy --overwrite
```

如果某台机器不想运行爬虫，删除这个标签即可：

```bash
kubectl label node <node-name> task-processor/crawler-tier-
```

推荐做法：

- `2C2G`：不打 `crawler-tier` 标签
- `2C4G`：打 `task-processor/crawler-tier=lite`
- `4C8G`：打 `task-processor/crawler-tier=heavy`

当前分层配置：

- `lite`: `browser.poolSize=2`，`maxInFlight=2`，`memory limit=2200Mi`
- `heavy`: `browser.poolSize=4`，`maxInFlight=4`，`memory limit=4Gi`
- `lite`: `browser.randomConfig.maxUsesPerInstance=12`
- `heavy`: `browser.randomConfig.maxUsesPerInstance=18`

轮换策略说明：

- 每个浏览器实例累计处理到阈值后，会在本次任务完成后异步重建
- 这样可以定期回收长生命周期 `BrowserContext`，降低 `Page crashed`、`Target crashed` 和 `OOMKilled` 的概率
- `lite` 机器更容易受内存影响，所以阈值更低；`heavy` 机器阈值稍高，减少频繁轮换带来的吞吐损失

注意：

- `base/secret.example.yaml` 只是示例，不要直接用于生产
- 生产前请替换镜像名、RabbitMQ 地址、管理端地址、OpenAI/SP-API 凭证
- 三个平台默认都走平台共享队列；只有显式启用 `useStoreQueues` 才走店铺队列
- `cert-manager/letsencrypt-prod` 用于固化 `letsencrypt-prod` 的 HTTP-01 solver 调度策略，避免 solver 被调度到和 `traefik` 不同的节点后挑战请求失败
- `monitoring/kube-prometheus-stack-values.yaml` 里固定了 Alertmanager 到 `vm-4-17-ubuntu`，这是当前集群为绕开跨节点 Pod 网络异常和本地卷绑定问题的兼容配置
- `monitoring/alertmanager-wecom` 需要先创建 `alertmanager-wecom-secret`
- `monitoring/amazon-crawler-api` 依赖 Prometheus Operator CRD 和 Grafana dashboard sidecar
- `amazon-crawler-api` 已移除 HPA，因为工作负载是 `DaemonSet`，此前的 HPA 会持续报 `deployments.apps "amazon-crawler-api" not found`

# 查看token
kubectl -n kubernetes-dashboard create token admin-user --duration=168h

curl -sfL https://get.k3s.io | \
K3S_URL=https://101.33.34.102:6443 \
K3S_TOKEN='K106d307a8203c82df970ad850a99a06c1a977c3047455b5ca9d554a93397e02347::server:9abe15a6701e84fbc441b166acfbcea1' \
sh -s - agent --node-external-ip <远程机器公网IP>
