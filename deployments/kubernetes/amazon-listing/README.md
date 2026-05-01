# Amazon Listing Kubernetes 部署说明

这套清单用于部署 RabbitMQ 驱动的 Amazon 上架消费者，对应程序入口：

- [main.go](/D:/code/task-processor/cmd/amazon-listing/main.go)

## 部署顺序

Amazon 上架现在依赖集群内的外部 crawler LB，所以顺序必须是：

1. 先部署 [amazon-crawler-external-lb](/D:/code/task-processor/deployments/kubernetes/amazon-crawler-external-lb/README.md)
2. 验证 `amazon-crawler-external-lb.task-processor.svc.cluster.local:8080` 可访问
3. 再部署 `amazon-listing`

## 目录结构

- `base/`
- `overlays/staging`
- `overlays/prod`

## 关键配置

主配置在：

- [configmap.yaml](/D:/code/task-processor/deployments/kubernetes/amazon-listing/base/configmap.yaml)

当前默认值说明：

- `rabbitmq.node.useStoreQueues: false`
  默认走平台共享队列
- `rabbitmq.consumer.queues`
  同时监听 `amazon.tasks` 和 `amazon.tasks.store.*`
- `amazon.remoteAPI.enabled: true`
- `amazon.remoteAPI.baseURL: http://amazon-crawler-external-lb.task-processor.svc.cluster.local:8080`

这意味着 Amazon 上架默认会通过 external-lb 去访问外部 crawler 节点。

## Secret 准备

生产 overlay 已经预置了模板：

- [external-secret.yaml](/D:/code/task-processor/deployments/kubernetes/amazon-listing/overlays/prod/external-secret.yaml)

上线前至少替换这些值：

- `TASK_PROCESSOR_MANAGEMENT_CLIENT_SECRET`
- `TASK_PROCESSOR_RABBITMQ_URL`
- `TASK_PROCESSOR_OPENAI_API_KEY`
- `TASK_PROCESSOR_AMAZON_SPAPI_CLIENT_ID`
- `TASK_PROCESSOR_AMAZON_SPAPI_CLIENT_SECRET`
- `TASK_PROCESSOR_AMAZON_SPAPI_REFRESH_TOKEN`
- `TASK_PROCESSOR_REDIS_PASSWORD`（如果有）

## 部署命令

Staging：

```bash
kubectl apply -k deployments/kubernetes/amazon-listing/overlays/staging
```

Prod：

```bash
kubectl apply -k deployments/kubernetes/amazon-listing/overlays/prod
```

## 部署后检查

```bash
kubectl -n task-processor get deploy,pod,svc | grep amazon-listing
kubectl -n task-processor logs deploy/amazon-listing --tail=200
kubectl -n task-processor port-forward deploy/amazon-listing 8081:8081 8082:8082
```

然后检查：

```bash
curl http://127.0.0.1:8081/health
curl http://127.0.0.1:8081/stats
curl http://127.0.0.1:8081/api/v1/management/tasks/health
```

如果要重点验证 external-lb 依赖，可以再起一个临时 Pod：

```bash
kubectl -n task-processor run curl-test --rm -it --image=curlimages/curl -- \
  curl -sS http://amazon-crawler-external-lb.task-processor.svc.cluster.local:8080/health
```

## 建议副本数

起步建议：

- `staging`: 1 副本
- `prod`: 2 到 3 副本

等 external crawler 稳定后，再根据 RabbitMQ 积压和 Amazon API 限流情况调大。
