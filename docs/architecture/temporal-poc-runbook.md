# ListingKit Temporal PoC Runbook

## 目标

这份 runbook 只覆盖当前 PoC：

- 平台：`shein`
- 动作：`publish`
- 编排层：Temporal
- 业务规则：继续复用 ListingKit 现有提交逻辑

当前实现支持两种形态：

- `product-listing-api` 进程内启动 worker
- 独立进程 `listingkit-temporal-worker`

只有开启 PoC 开关时，SHEIN `publish` 才会切到 Temporal 编排。

## 启动前提

### 1. 启动本地 Temporal

本地开发可以先起一个 Temporal dev server。

如果机器上已经安装了 Temporal CLI，可以直接使用官方开发模式：

```bash
temporal server start-dev
```

如果本机没有 `temporal` CLI，可以直接用仓库里的 Docker 脚本启动。脚本底层按官方 `temporalio/temporal` 镜像执行 `server start-dev --ip 0.0.0.0`：

```powershell
.\scripts\start-temporal-dev.ps1
```

默认映射：

- gRPC Frontend: `localhost:7233`
- Temporal UI: `http://localhost:8233/`

停止命令：

```powershell
.\scripts\stop-temporal-dev.ps1
```

如果你们本地已经有单独的 Temporal 或 Docker 编排环境，也可以直接复用，只要应用能连到对应地址即可。

### 2. 设置 PoC 环境变量

当前 PoC 通过环境变量控制：

```bash
LISTINGKIT_TEMPORAL_ENABLED=true
LISTINGKIT_TEMPORAL_ADDRESS=localhost:7233
LISTINGKIT_TEMPORAL_NAMESPACE=default
LISTINGKIT_TEMPORAL_START_WORKER=true
```

说明：

- `LISTINGKIT_TEMPORAL_ENABLED=true`：启用 SHEIN `publish` Temporal PoC
- `LISTINGKIT_TEMPORAL_ADDRESS`：Temporal Frontend 地址
- `LISTINGKIT_TEMPORAL_NAMESPACE`：Temporal namespace；默认 `default`
- `LISTINGKIT_TEMPORAL_START_WORKER`：是否在 `product-listing-api` 当前进程内启动 worker；默认 `true`

不设置 `LISTINGKIT_TEMPORAL_ENABLED=true` 时，系统保持原来的同步提交流程。

## 启动方式

### 方式 A：API 进程内同时跑 client + worker

```bash
go run ./cmd/product-listing-api -config config/config-dev.yaml
```

启用 PoC 后，`product-listing-api` 在构建 ListingKit 模块时会：

1. dial Temporal client
2. 把 Temporal workflow client 注入 ListingKit service
3. 在 `LISTINGKIT_TEMPORAL_START_WORKER` 未显式关闭时启动 in-process worker

启动成功后，日志里会出现类似信息：

- `connected listingkit shein publish temporal client`
- `started listingkit shein publish temporal worker`

### 方式 B：API 和 worker 分离

API 进程只负责接请求和发起 workflow：

```bash
LISTINGKIT_TEMPORAL_ENABLED=true \
LISTINGKIT_TEMPORAL_START_WORKER=false \
go run ./cmd/product-listing-api -config config/config-dev.yaml
```

独立 worker 进程负责消费 Temporal task queue：

```bash
LISTINGKIT_TEMPORAL_ENABLED=true \
go run ./cmd/listingkit-temporal-worker -config config/config-dev.yaml
```

说明：

- API 进程里把 `LISTINGKIT_TEMPORAL_START_WORKER=false` 关掉，是为了避免 API 自己起 worker。
- 独立 worker 入口现在**不再读取这个开关来决定是否启动**；只要 `LISTINGKIT_TEMPORAL_ENABLED=true` 且能连上 Temporal，就会真正启动 worker。

这个形态下，建议看两类日志：

- API:
  `connected listingkit shein publish temporal client`
- worker:
  `started listingkit shein publish temporal worker`

## 提交流程验证

### 1. 发起 SHEIN publish

```bash
curl -X POST http://localhost:3000/api/v1/listing-kits/tasks/<task_id>/submit \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: temporal-poc-123" \
  -d '{"platform":"shein","action":"publish"}'
```

预期：

- HTTP 返回 `200`
- 不再直接在请求线程里执行完整 publish 编排
- Temporal 中出现 workflow：`shein-submit:<task_id>:publish`

### 2. 观察当前阶段

当前 PoC 没有额外开放 Temporal query HTTP 接口，建议通过已有 ListingKit 预览和事件接口看状态：

```bash
curl "http://localhost:3000/api/v1/listing-kits/tasks/<task_id>/preview?platform=shein"
curl "http://localhost:3000/api/v1/listing-kits/tasks/<task_id>/submission-events"
```

重点看：

- `submission_state.current_phase`
- `submission_state.current_request_id`
- `submission_state.last_status`
- `submission_state.last_error`
- 兼容字段 `submission.*` 仍会保留一段迁移窗口
- `submission_events[*].phase`

如果需要强制刷新远端确认结果，可以继续使用已有接口：

```bash
curl -X POST http://localhost:3000/api/v1/listing-kits/tasks/<task_id>/submission-status/refresh
```

### 3. 重复提交验证

重复用同一个 `task_id + publish` 发起请求：

- 如果 workflow 仍在跑，应该映射成 `ErrSubmitInProgress`
- HTTP 层表现为 `409`
- 不应该重复调用远端 publish

## 手工验证清单

### 用例 1：正常发起 publish

1. 启动 Temporal
2. 启动 `product-listing-api`
3. 对可提交的 SHEIN task 调用 `/submit`
4. 通过 `/preview` 或 `/submission-events` 确认阶段推进

通过标准：

- workflow 被创建
- phase 能推进到 `submit_remote` / `confirm_remote`
- 最终状态回到任务结果里，而不是只存在 Temporal history 中

### 用例 2：观察阶段可见性

1. 触发一个会经过多个 phase 的 publish
2. 在执行中轮询 `/preview` 和 `/submission-events`

通过标准：

- 可以看到 `prepare_product`
- 可以看到 `pre_validate`
- 可以看到 `submit_remote`
- 可以看到 `confirm_remote`

### 用例 3：worker 中断恢复

1. 触发 publish
2. 在 workflow 运行中停止 worker 进程
3. 重新启动 worker 进程
4. 观察 workflow 是否继续推进

通过标准：

- workflow 不会因为进程重启丢失
- 恢复后能继续活动执行

### 用例 4：避免重复远端提交

1. 对同一个 task 发起 publish
2. 在 workflow 进行中重复提交
3. 检查远端 publish 调用次数

通过标准：

- 相同 workflow ID 只保留一个运行中的 workflow
- API 返回冲突而不是再次提交远端

## 代码级验证命令

当前 PoC 建议至少执行下面几组验证：

```bash
go test ./internal/listingkit/temporal -run "PublishWorkflow|ConcreteActivities|Client|WorkflowID" -v
go test ./internal/listingkit ./internal/listingkit/api -run "SubmitTask|SubmitHandler|SheinPublishActivityHost" -v
go test ./internal/app/httpapi -run "TestBuild|TestConfig|TestApp|TestHTTP|TestModules|TestListingKit" -v
```

如果本地已经启动了真实 Temporal server，还可以跑一条可选的恢复集成测试：

```bash
TEMPORAL_E2E_ADDRESS=127.0.0.1:7233 \
go test ./internal/listingkit/temporal -run TestPublishWorkflowResumesAfterWorkerRestartAgainstRealTemporal -v
```

这条测试会：

1. 用 `worker-1` 执行 workflow 到 `persist_result`
2. 主动停止 `worker-1`
3. 用 `worker-2` 接着执行 `confirm_remote`
4. 断言 workflow 最终完成，且收尾 activity 由重启后的 worker 执行

注意：

- 这条测试依赖真实 Temporal server，不会默认进 CI
- worker 停掉后，Temporal query handler 在恢复间隙不一定能稳定响应，所以该测试只验证恢复和完成，不把 query 作为恢复断言

准备做更大范围回归时再跑：

```bash
go test ./...
```

## 当前 PoC 边界

当前实现明确不覆盖：

- `save_draft` Temporal 化
- 非 SHEIN 平台提交
- crawler / scheduler / 通用任务迁移
- 专门的 workflow query API 暴露

这些都留在下一阶段。
