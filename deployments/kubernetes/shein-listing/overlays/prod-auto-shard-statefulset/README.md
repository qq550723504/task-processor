# SHEIN auto shard StatefulSet

这套清单把 `shein-listing` 部署为共享 worker 池，而不是单店独享 Deployment。

特点：

- 用 `StatefulSet` 保证每个 Pod 有稳定唯一的 `node_id`
- `TASK_PROCESSOR_RABBITMQ_NODE_NODE_ID` 直接取 `metadata.name`
- 自动分片候选节点固定为：
  - `shein-listing-shard-0`
  - `shein-listing-shard-1`
  - `shein-listing-shard-2`
  - ...
  - `shein-listing-shard-19`

当前副本数是 `20`，候选节点范围为 `shein-listing-shard-0` 到 `shein-listing-shard-19`。

如果后面要扩到更多 shard worker，必须同时修改两处：

1. `spec.replicas`
2. `TASK_PROCESSOR_RABBITMQ_AUTO_SHARD_CANDIDATE_NODES`
