# SHEIN Store Shards Example

这个 overlay 演示如何把 `shein-listing` 拆成多个按店铺归属消费的 shard Deployment。

当前示例包含：

- `shein-listing-store-a`
- `shein-listing-store-b`

每个 Deployment 通过环境变量覆盖节点配置：

- `TASK_PROCESSOR_RABBITMQ_NODE_NODE_ID`
- `TASK_PROCESSOR_RABBITMQ_NODE_USE_STORE_QUEUES=true`
- `TASK_PROCESSOR_RABBITMQ_NODE_OWNED_STORES`

使用前请先把示例店铺列表改成真实店铺：

- `deployment-store-a.yaml`
- `deployment-store-b.yaml`

同时把占位 Secret 改成真实值：

- `secret.example.yaml`

示例应用：

```bash
kubectl apply -k deployments/kubernetes/shein-listing/overlays/prod-store-shards-example
```

建议迁移步骤：

1. 先灰度少量店铺到 `tasks.store.{storeId}`
2. 只让这些店铺进入对应 shard 的 `ownedStores`
3. 观察 409、ready 状态、RabbitMQ backlog
4. 稳定后再继续增加 shard 或切到 Redis 动态归属
