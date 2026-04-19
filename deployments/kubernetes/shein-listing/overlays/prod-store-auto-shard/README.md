# SHEIN Store Auto Shard

这个 overlay 演示如何把 `shein-listing` 部署成“自动分片”的 store shard 节点。

与手工 `ownedStores` 的区别：

- 不再设置 `TASK_PROCESSOR_RABBITMQ_NODE_OWNED_STORES`
- 由 `task-processor` 自己定时拉店铺列表
- 按 `TASK_PROCESSOR_RABBITMQ_AUTO_SHARD_CANDIDATE_NODES` 自动写 Redis owner
- 每个 shard Pod 继续通过动态归属热更新消费自己的 `tasks.store.{storeId}`

当前示例节点：

- `shein-listing-store-a`
- `shein-listing-store-b`
- `shein-listing-store-c`
- `shein-listing-store-d`

关键环境变量：

- `TASK_PROCESSOR_RABBITMQ_NODE_NODE_ID`
- `TASK_PROCESSOR_RABBITMQ_NODE_USE_STORE_QUEUES=true`
- `TASK_PROCESSOR_RABBITMQ_AUTO_SHARD_ENABLED=true`
- `TASK_PROCESSOR_RABBITMQ_AUTO_SHARD_PLATFORM=shein`
- `TASK_PROCESSOR_RABBITMQ_AUTO_SHARD_CANDIDATE_NODES=shein-listing-store-a,shein-listing-store-b,shein-listing-store-c,shein-listing-store-d`
- `TASK_PROCESSOR_REDIS_DB=9`

使用前请至少确认两件事：

1. 四个 Deployment 的镜像都换成包含自动分片代码的新 tag
2. `shein-listing-secret` 里 Redis 与 management API 配置可用，且自动分片写入的 Redis DB 要和 `yudao-listing` 读取 `listing:queue:*` 的 DB 保持一致

示例应用：

```bash
kubectl apply -k deployments/kubernetes/shein-listing/overlays/prod-store-auto-shard
```

建议上线步骤：

1. 先保留 4 个 shard 节点，验证 Redis owner 是否自动生成
2. 看 `rabbitmq_detail` / 日志中 `dynamic store assignments changed`
3. 确认新店铺不再需要手工写 `ownedStores`
4. 稳定后再继续扩成更多 shard 节点
