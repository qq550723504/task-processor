# SHEIN Listing Kubernetes 部署说明

这套清单用于部署 RabbitMQ 驱动的 SHEIN 上架消费者，对应程序入口：

- [main.go](D:/code/task-processor/cmd/shein-listing/main.go)

## 目录结构

- `base/`
  - `ConfigMap`
  - `Deployment`
  - `Service`
  - `HPA`
  - `secret.example.yaml`
- `overlays/staging`
- `overlays/prod`
- `overlays/prod-store-shards-example`

## 镜像构建

通用 Dockerfile：

- [Dockerfile.listing](D:/code/task-processor/deployments/docker/Dockerfile.listing)

SHEIN 专用推送脚本：

- [push-shein-listing-dockerhub.ps1](D:/code/task-processor/scripts/push-shein-listing-dockerhub.ps1)
- [build-push-deploy-shein-listing.ps1](D:/code/task-processor/scripts/build-push-deploy-shein-listing.ps1)

示例：

```powershell
pwsh ./scripts/push-shein-listing-dockerhub.ps1 -DockerHubUser xuwei190 -Tag v20260402
```

一键构建、推送并部署到 K3S：

```powershell
pwsh ./scripts/build-push-deploy-shein-listing.ps1 -DockerHubUser xuwei190 -Tag v20260402
```

单店专属 pod：

```powershell
pwsh ./scripts/deploy-single-store-listing.ps1 `
  -StoreId 801 `
  -OwnerNodeId ser376996273941-c4e40df6 `
  -Tier heavy `
  -ExcludeNode ser376996273941-c4e40df6 `
  -Apply
```

说明：

- `OwnerNodeId`
  - RabbitMQ 店铺队列归属节点标识，必须和店铺当前 owner 绑定保持一致，或配合后台切换 owner 后再部署
- `Tier`
  - 默认复用对应 `shein-listing-lite` / `shein-listing-heavy` 的线上镜像和资源档位
- `ExcludeNode`
  - 可选；用于避开故障节点，但仍沿用原 owner 节点名接管队列

如果你不用脚本，也可以直接构建：

```bash
docker build \
  --build-arg SERVICE_CMD=./cmd/shein-listing/main.go \
  -f deployments/docker/Dockerfile.listing \
  -t xuwei190/task-processor-shein-listing:latest .
```

## 部署前必须确认

1. `yudao-cloud` 已开启 RabbitMQ 自动投递
2. RabbitMQ / Redis 已在集群内可访问
3. 生产不会再启动 `cmd/task`
4. SHEIN 店铺所需的管理端凭证、OpenAI、Redis、RabbitMQ 地址已准备好
5. `amazon-crawler-external-lb` 已先部署并可从集群内访问

## 关键配置

主配置在：

- [configmap.yaml](D:/code/task-processor/deployments/kubernetes/shein-listing/base/configmap.yaml)

当前默认值说明：

- `rabbitmq.node.useStoreQueues: false`
  - 默认走平台共享队列
- `rabbitmq.node.ownedStores: []`
  - 可通过环境变量 `TASK_PROCESSOR_RABBITMQ_NODE_OWNED_STORES` 为每个 Deployment 单独覆盖
- `rabbitmq.node.nodeID: ""`
  - 可通过环境变量 `TASK_PROCESSOR_RABBITMQ_NODE_NODE_ID` 指定固定节点标识
- `rabbitmq.consumer.queues`
  - 同时监听 `shein.tasks` 和 `shein.tasks.store.*`
- `platforms.shein.enabled: true`
- `platforms.temu.enabled: false`
- `amazon.remoteAPI.enabled: true`
- `amazon.remoteAPI.baseURL: http://amazon-crawler-external-lb.task-processor.svc.cluster.local:8080`

这意味着这套 Deployment 是专门跑 SHEIN 上架消费，不会去处理 TEMU/Amazon 目标平台任务，但会通过 external-lb 获取 Amazon 源商品数据。

## Secret 准备

基础模板：

- [secret.example.yaml](D:/code/task-processor/deployments/kubernetes/shein-listing/base/secret.example.yaml)

生产 overlay 已经预置了可编辑模板：

- [secret.yaml](D:/code/task-processor/deployments/kubernetes/shein-listing/overlays/prod/secret.yaml)
- [secret.example.yaml](D:/code/task-processor/deployments/kubernetes/shein-listing/overlays/prod/secret.example.yaml)

现在只需要直接修改 `secret.yaml` 里的占位值即可，不需要再额外改 `kustomization.yaml`。

最低需要替换：

- `TASK_PROCESSOR_MANAGEMENT_CLIENT_SECRET`
- `TASK_PROCESSOR_RABBITMQ_URL`
- `TASK_PROCESSOR_OPENAI_API_KEY`
- `TASK_PROCESSOR_AMAZON_REMOTE_API_BASE_URL`
- `TASK_PROCESSOR_REDIS_PASSWORD`（如果有）

## 部署命令

Staging：

```bash
kubectl apply -k deployments/kubernetes/shein-listing/overlays/staging
```

Prod：

```bash
kubectl apply -k deployments/kubernetes/shein-listing/overlays/prod
```

按店铺分片示例：

```bash
kubectl apply -k deployments/kubernetes/shein-listing/overlays/prod-store-shards-example
```

## 部署后检查

```bash
kubectl -n task-processor get deploy,pod,svc | grep shein-listing
kubectl -n task-processor logs deploy/shein-listing --tail=200
kubectl -n task-processor port-forward deploy/shein-listing 8081:8081 8082:8082
```

然后检查：

```bash
curl http://127.0.0.1:8081/health
curl http://127.0.0.1:8081/stats
curl http://127.0.0.1:8081/api/v1/management/tasks/health
```

通过标准：

- `/health` 返回就绪
- `/stats` 有 `ServiceManager` 和 RabbitMQ 运行态
- `/api/v1/management/tasks/health` 能拿到 management task RPC 的摘要

## 建议副本数

起步建议：

- `staging`: 1 副本
- `prod`: 2 到 3 副本

后面再根据：

- SHEIN backlog
- RabbitMQ 积压
- API 限流
- 每日额度命中情况

逐步调高。
