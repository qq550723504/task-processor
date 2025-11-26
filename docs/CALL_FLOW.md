# 完整调用流程

## 启动流程

```
main.go
  │
  ├─ SetupLogger()
  ├─ LoadConfig("temu")
  ├─ ValidateConfig()
  └─ InitializeWithClientCredentials()
       │
       └─ Server.autoStartProcessor()
            │
            ├─ initializeTaskProcessor()
            │    │
            │    ├─ 创建 managementClient
            │    ├─ 创建 TemuProcessor (内部创建 WorkerPool)
            │    └─ 创建 SheinProcessor (内部创建 WorkerPool)
            │
            └─ startTaskProcessor()
                 │
                 ├─ TemuProcessor.Start()
                 │    └─ WorkerPool.Start()
                 │
                 ├─ SheinProcessor.Start()
                 │    └─ WorkerPool.Start()
                 │
                 └─ 启动 UnifiedTaskFetcher
                      │
                      ├─ WrapManagementClient() ← 适配器在这里
                      │    └─ 返回 ManagementClientProvider
                      │
                      └─ UnifiedTaskFetcher.Start()
```

## 任务处理流程

```
UnifiedTaskFetcher (每30秒)
  │
  └─ fetchAndDispatchTasks()
       │
       ├─ 1. 获取任务
       │    │
       │    ├─ managementClient.GetImportTaskClient() ← 通过适配器
       │    │    │
       │    │    └─ ImportTaskClientAdapter
       │    │         └─ impl.ImportTaskAPIClientImpl
       │    │              └─ HTTP API 调用
       │    │
       │    └─ GetPendingAndRetryTasks(5个任务)
       │         └─ 返回 []TaskDTO
       │
       ├─ 2. 查询店铺平台
       │    │
       │    ├─ managementClient.GetStoreClient() ← 通过适配器
       │    │    │
       │    │    └─ StoreClientAdapter
       │    │         └─ impl.StoreAPIClientImpl
       │    │              └─ HTTP API 调用
       │    │
       │    └─ GetStore(storeID)
       │         └─ 返回 StoreDTO (包含 Platform)
       │
       ├─ 3. 根据平台分发
       │    │
       │    ├─ Platform = "TEMU"
       │    │    └─ TemuTaskSubmitter.SubmitTask()
       │    │         └─ TemuProcessor.WorkerPool.Submit()
       │    │              └─ 任务进入队列
       │    │
       │    └─ Platform = "SHEIN"
       │         └─ SheinTaskSubmitter.SubmitTask()
       │              └─ SheinProcessor.WorkerPool.Submit()
       │                   └─ 任务进入队列
       │
       └─ 4. 输出统计
            └─ "✅ 任务分发完成: {TEMU:3, SHEIN:2}"
```

## Worker 处理流程

```
WorkerPool (TEMU 或 SHEIN)
  │
  ├─ Worker 1
  │    │
  │    └─ 循环等待任务
  │         │
  │         ├─ 从队列获取任务
  │         │
  │         ├─ 解析 TaskData
  │         │
  │         ├─ Processor.ProcessTask()
  │         │    │
  │         │    ├─ TEMU: TemuProcessor.ProcessTask()
  │         │    │    └─ Pipeline 处理
  │         │    │
  │         │    └─ SHEIN: SheinProcessor.ProcessTask()
  │         │         └─ Pipeline 处理
  │         │
  │         └─ 记录结果
  │
  ├─ Worker 2 (如果 concurrency > 1)
  │
  └─ Worker N
```

## 适配器的位置

```
Server
  │
  ├─ managementClient: *management.ClientManager (具体实现)
  │
  └─ startTaskProcessor()
       │
       ├─ WrapManagementClient(managementClient)
       │    │
       │    └─ 创建 DirectManagementClientProvider (适配器)
       │         │
       │         └─ 包装 ClientManager
       │              └─ 实现 ManagementClientProvider 接口
       │
       └─ NewUnifiedTaskFetcher(cfg, managementProvider, ...)
            │
            └─ UnifiedTaskFetcher
                 │
                 └─ managementClient: ManagementClientProvider (接口)
                      │
                      ├─ GetImportTaskClient()
                      │    └─ 返回 ImportTaskClient 接口
                      │         └─ ImportTaskClientAdapter (适配器)
                      │              └─ 包装 ImportTaskAPIClientImpl
                      │
                      └─ GetStoreClient()
                           └─ 返回 StoreClient 接口
                                └─ StoreClientAdapter (适配器)
                                     └─ 包装 StoreAPIClientImpl
```

## 数据转换流程

```
API 响应
  │
  ├─ ProductImportTaskRespDTO (API 层)
  │    │
  │    └─ ImportTaskClientAdapter.GetPendingAndRetryTasks()
  │         │
  │         └─ 转换为 TaskDTO (通用层)
  │              │
  │              └─ UnifiedTaskFetcher
  │                   │
  │                   └─ 转换为 types.Task (内部层)
  │                        │
  │                        └─ TaskSubmitter.SubmitTask()
  │                             │
  │                             └─ WorkerPool.Submit()
  │
  └─ StoreRespDTO (API 层)
       │
       └─ StoreClientAdapter.GetStore()
            │
            └─ 转换为 StoreDTO (通用层)
                 │
                 └─ UnifiedTaskFetcher (用于判断平台)
```

## 配置流程

```
config-temu-dev.yaml
  │
  ├─ worker:
  │    ├─ concurrency: 1
  │    ├─ bufferSize: 5
  │    └─ taskInterval: 30
  │
  ├─ management:
  │    ├─ baseURL
  │    ├─ clientID
  │    ├─ clientSecret
  │    └─ storeIDs: [508]
  │
  └─ LoadConfig()
       │
       └─ ValidateConfig()
            │
            ├─ 验证 concurrency > 0
            ├─ 验证 bufferSize > 0
            ├─ 验证 clientID 不为空
            └─ 验证 tenantID 不为空
```

## 优雅关闭流程

```
用户按 Ctrl+C
  │
  └─ setupGracefulShutdown()
       │
       ├─ 收到 SIGINT 信号
       │
       └─ Server.StopProcessor()
            │
            ├─ processorCancel() (取消 context)
            │    │
            │    └─ 通知所有组件停止
            │         │
            │         ├─ UnifiedTaskFetcher 停止获取
            │         ├─ TemuProcessor 停止
            │         └─ SheinProcessor 停止
            │
            ├─ TemuProcessor.Close()
            │    │
            │    └─ WorkerPool.Stop()
            │         │
            │         ├─ 关闭队列
            │         ├─ 等待 Worker 完成
            │         └─ 清理资源
            │
            └─ SheinProcessor.Close()
                 │
                 └─ WorkerPool.Stop()
                      └─ (同上)
```

## 总结

### 适配器的作用
- **位置**: Server 和 UnifiedTaskFetcher 之间
- **目的**: 将具体的 `ClientManager` 适配为接口
- **好处**: 解耦、可测试、灵活

### 关键组件
1. **Server** - 协调所有组件
2. **UnifiedTaskFetcher** - 统一获取和分发任务
3. **适配器** - 连接具体实现和接口
4. **TaskSubmitter** - 提交任务到对应平台
5. **WorkerPool** - 管理 Worker 和任务队列
6. **Processor** - 实际处理任务

### 数据流向
```
API → 适配器 → UnifiedTaskFetcher → TaskSubmitter → WorkerPool → Worker → Processor
```
