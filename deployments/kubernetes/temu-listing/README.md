# TEMU Listing Kubernetes 部署说明

这套清单用于部署 RabbitMQ 驱动的 TEMU 上架消费者，对应程序入口：

- `cmd/temu-listing/main.go`

## 当前状态

`cmd/temu-listing` 是当前受维护的正式 runtime 入口之一，但 TEMU 完整 ListingKit 工作台扩张当前仍是 deferred。

这份部署说明只表示：

- TEMU listing runtime 和 Kubernetes 资产仍被保留；
- 现有 TEMU 消费者可以按当前部署资产继续维护；
- 运行时正确性、队列消费和配置健康需要通过当前基线验证；
- 不应把该 runtime 的存在理解为 TEMU 已经具备与 SHEIN 主链路同等成熟的产品工作台、readiness、提交恢复或运营体验。

当前产品主线仍是 SHEIN 稳定化、Product Sourcing MVP 验证闭环，以及之后选择一个下一来源扩展。

## 部署顺序

TEMU 上架会通过公共 `ProductFetcher` 获取来源商品数据。具体来源服务和地址应以当前环境配置为准，不应依赖历史本地路径或已退役服务名称。

上线前需要确认：

1. 来源商品获取路径在目标环境可用。
2. `configmap.yaml` 中的 remote API / source API 地址与当前集群服务一致。
3. RabbitMQ、Redis、OpenAI、目标平台凭证和对象存储配置可用。
4. 当前 `master` 或 release commit 已通过后端测试、构建和必要 smoke。
5. TEMU 队列积压、平台限流和失败恢复边界已明确。

## 目录结构

- `base/`
- `overlays/staging`
- `overlays/prod`

## 关键配置

主配置在：

- `deployments/kubernetes/temu-listing/base/configmap.yaml`

当前默认值说明：

- `rabbitmq.node.useStoreQueues: false`
  默认走平台共享队列
- `rabbitmq.consumer.queues`
  同时监听 `temu.tasks` 和 `temu.tasks.store.*`
- `platforms.temu.enabled: true`
- `platforms.shein.enabled: false`
- `amazon.remoteAPI.enabled: true`
- `amazon.remoteAPI.baseURL`
  应指向当前环境实际可用的来源商品 API 服务

这意味着这套 Deployment 是专门跑 TEMU 上架消费，不会去处理 SHEIN/Amazon 目标平台任务。来源商品获取路径需要按当前集群和配置验证，不再把旧的 external crawler 服务名当作长期架构事实。

## Secret 准备

基础模板：

- `deployments/kubernetes/temu-listing/base/secret.example.yaml`

生产 overlay 已经预置了可编辑模板：

- `deployments/kubernetes/temu-listing/overlays/prod/external-secret.yaml`

上线前至少替换这些值：

- `TASK_PROCESSOR_RABBITMQ_URL`
- `TASK_PROCESSOR_OPENAI_API_KEY`
- `TASK_PROCESSOR_AMAZON_REMOTE_API_BASE_URL` 或当前来源商品 API 对应配置
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
- `prod`: 1 副本，除非当前 release 已有 TEMU 队列、限流、幂等和失败恢复验证证据

在调高副本数前，需要确认：

- 消费者幂等边界清晰；
- store queue / shared queue 策略符合当前运营模型；
- RabbitMQ backlog 和平台限流允许扩容；
- 重试不会重复提交或重复扣减库存；
- 最新 validation note 或发布记录覆盖过 TEMU runtime smoke。
