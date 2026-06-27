# TEMU Listing Kubernetes 部署说明

这套清单用于部署 RabbitMQ 驱动的 TEMU 上架消费者，对应程序入口：

- [main.go](/D:/code/task-processor/cmd/temu-listing/main.go)

## 部署顺序

TEMU 上架会通过公共 `ProductFetcher` 获取 Amazon 源商品数据，所以顺序建议是：

1. 先部署 [amazon-crawler-external-lb](/D:/code/task-processor/deployments/kubernetes/amazon-crawler-external-lb/README.md)
2. 验证 `amazon-crawler-external-lb.task-processor.svc.cluster.local:8080` 可访问
3. 再部署 `temu-listing`

## 目录结构

- `base/`
- `overlays/staging`
- `overlays/prod`

## 关键配置

主配置在：

- [configmap.yaml](/D:/code/task-processor/deployments/kubernetes/temu-listing/base/configmap.yaml)

当前默认值说明：

- `rabbitmq.node.useStoreQueues: false`
  默认走平台共享队列
- `rabbitmq.consumer.queues`
  同时监听 `temu.tasks` 和 `temu.tasks.store.*`
- `platforms.temu.enabled: true`
- `platforms.shein.enabled: false`
- `amazon.remoteAPI.enabled: true`
- `amazon.remoteAPI.baseURL: http://amazon-crawler-external-lb.task-processor.svc.cluster.local:8080`

这意味着这套 Deployment 是专门跑 TEMU 上架消费，不会去处理 SHEIN/Amazon 目标平台任务，但会通过 external-lb 获取 Amazon 源商品数据。

## Secret 准备

基础模板：

- [secret.example.yaml](/D:/code/task-processor/deployments/kubernetes/temu-listing/base/secret.example.yaml)

生产 overlay 已经预置了可编辑模板：

- [external-secret.yaml](/D:/code/task-processor/deployments/kubernetes/temu-listing/overlays/prod/external-secret.yaml)

上线前至少替换这些值：

- `TASK_PROCESSOR_RABBITMQ_URL`
- `TASK_PROCESSOR_OPENAI_API_KEY`
- `TASK_PROCESSOR_AMAZON_REMOTE_API_BASE_URL`
- `TASK_PROCESSOR_REDIS_PASSWORD`（如果有）

## 部署命令

Staging：

```bash
kubectl apply -k deployments/kubernetes/temu-listing/overlays/staging
```

Prod：

```bash
kubectl apply -k deployments/kubernetes/temu-listing/overlays/prod
```

## 部署后检查

```bash
kubectl -n task-processor get deploy,pod,svc | grep temu-listing
kubectl -n task-processor logs deploy/temu-listing --tail=200
kubectl -n task-processor port-forward deploy/temu-listing 8081:8081 8082:8082
```

然后检查：

```bash
curl http://127.0.0.1:8081/health
curl http://127.0.0.1:8081/stats
curl http://127.0.0.1:8081/api/v1/management/tasks/health
```

## 建议副本数

起步建议：

- `staging`: 1 副本
- `prod`: 2 到 3 副本

等 external crawler 稳定后，再根据 TEMU backlog、RabbitMQ 积压和平台限流情况调大。
