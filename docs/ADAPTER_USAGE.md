# 适配器使用说明

## 适配器模式的作用

### 问题
`UnifiedTaskFetcher` 需要使用管理客户端，但我们希望：
1. **解耦** - 不直接依赖具体的 `management.ClientManager` 实现
2. **可测试** - 可以轻松 mock 接口进行单元测试
3. **灵活** - 未来可以替换不同的实现

### 解决方案：适配器模式

## 调用链

```
Server (server.go)
  │
  ├─ managementClient (*management.ClientManager)
  │
  ├─ WrapManagementClient(managementClient)
  │    │
  │    └─ 返回 ManagementClientProvider 接口
  │         │
  │         └─ DirectManagementClientProvider (适配器)
  │
  └─ NewUnifiedTaskFetcher(cfg, managementProvider, submitters)
       │
       └─ UnifiedTaskFetcher
            │
            ├─ managementClient (接口类型)
            │
            └─ fetchAndDispatchTasks()
                 │
                 ├─ managementClient.GetImportTaskClient()
                 │    │
                 │    └─ ImportTaskClientAdapter (适配器)
                 │         │
                 │         └─ 调用真实的 ImportTaskAPIClientImpl
                 │
                 └─ managementClient.GetStoreClient()
                      │
                      └─ StoreClientAdapter (适配器)
                           │
                           └─ 调用真实的 StoreAPIClientImpl
```

## 代码位置

### 1. 接口定义
**文件**: `common/task/interfaces.go`
```go
// ManagementClientProvider 管理客户端提供者接口
type ManagementClientProvider interface {
    GetImportTaskClient() ImportTaskClient
    GetStoreClient() StoreClient
}

// ImportTaskClient 导入任务API客户端接口
type ImportTaskClient interface {
    GetPendingAndRetryTasks(maxTasks int, userID int64, storeIDs []int64) ([]TaskDTO, error)
}

// StoreClient 店铺API客户端接口
type StoreClient interface {
    GetStore(storeID int64) (*StoreDTO, error)
}
```

### 2. 适配器实现
**文件**: `common/task/adapters.go`
```go
// DirectManagementClientProvider 适配器
type DirectManagementClientProvider struct {
    *management.ClientManager
}

// 实现接口方法
func (p *DirectManagementClientProvider) GetImportTaskClient() ImportTaskClient {
    return &ImportTaskClientAdapter{
        client: p.ClientManager.GetImportTaskClient(),
    }
}

func (p *DirectManagementClientProvider) GetStoreClient() StoreClient {
    return &StoreClientAdapter{
        client: p.ClientManager.GetStoreClient(),
    }
}

// 便捷包装函数
func WrapManagementClient(clientManager *management.ClientManager) ManagementClientProvider {
    if clientManager == nil {
        return nil
    }
    return &DirectManagementClientProvider{
        ClientManager: clientManager,
    }
}
```

### 3. 使用位置
**文件**: `cmd/temu-web/server/server.go`
```go
func (s *Server) startTaskProcessor() {
    // ...
    
    // 使用适配器包装管理客户端
    managementProvider := task.WrapManagementClient(s.managementClient)
    
    // 传递接口而不是具体实现
    unifiedFetcher := task.NewUnifiedTaskFetcher(s.cfg, managementProvider, submitters)
    go unifiedFetcher.Start(s.processorCtx)
    
    // ...
}
```

### 4. 消费位置
**文件**: `common/task/fetcher.go`
```go
type UnifiedTaskFetcher struct {
    config           *config.Config
    managementClient ManagementClientProvider  // 接口类型
    submitters       map[string]TaskSubmitter
    interval         time.Duration
}

func (f *UnifiedTaskFetcher) fetchAndDispatchTasks() {
    // 通过接口调用
    importTaskClient := f.managementClient.GetImportTaskClient()
    apiTasks, err := importTaskClient.GetPendingAndRetryTasks(...)
    
    storeClient := f.managementClient.GetStoreClient()
    storeInfo, err := storeClient.GetStore(apiTask.StoreID)
}
```

## 优势

### 1. 依赖倒置原则 (DIP)
```
之前:
UnifiedTaskFetcher → management.ClientManager (具体实现)

现在:
UnifiedTaskFetcher → ManagementClientProvider (接口)
                            ↑
                     DirectManagementClientProvider (适配器)
                            ↓
                     management.ClientManager (具体实现)
```

### 2. 易于测试
```go
// 可以轻松创建 mock 实现
type MockManagementClient struct {}

func (m *MockManagementClient) GetImportTaskClient() ImportTaskClient {
    return &MockImportTaskClient{}
}

func (m *MockManagementClient) GetStoreClient() StoreClient {
    return &MockStoreClient{}
}

// 测试时使用 mock
fetcher := NewUnifiedTaskFetcher(cfg, &MockManagementClient{}, submitters)
```

### 3. 灵活扩展
```go
// 未来可以添加新的实现，无需修改 UnifiedTaskFetcher
type CachedManagementClient struct {
    cache map[int64]*StoreDTO
    real  ManagementClientProvider
}

func (c *CachedManagementClient) GetStoreClient() StoreClient {
    return &CachedStoreClient{
        cache: c.cache,
        real:  c.real.GetStoreClient(),
    }
}
```

## 数据流

### 获取任务流程
```
1. UnifiedTaskFetcher.fetchAndDispatchTasks()
   ↓
2. managementClient.GetImportTaskClient()
   ↓
3. DirectManagementClientProvider.GetImportTaskClient()
   ↓
4. 返回 ImportTaskClientAdapter
   ↓
5. ImportTaskClientAdapter.GetPendingAndRetryTasks()
   ↓
6. 调用真实的 impl.ImportTaskAPIClientImpl.GetPendingAndRetryTasks()
   ↓
7. 返回 []api.ProductImportTaskRespDTO
   ↓
8. 适配器转换为 []TaskDTO
   ↓
9. 返回给 UnifiedTaskFetcher
```

### 获取店铺信息流程
```
1. UnifiedTaskFetcher.fetchAndDispatchTasks()
   ↓
2. managementClient.GetStoreClient()
   ↓
3. DirectManagementClientProvider.GetStoreClient()
   ↓
4. 返回 StoreClientAdapter
   ↓
5. StoreClientAdapter.GetStore(storeID)
   ↓
6. 调用真实的 impl.StoreAPIClientImpl.GetStore()
   ↓
7. 返回 api.StoreRespDTO
   ↓
8. 适配器转换为 StoreDTO
   ↓
9. 返回给 UnifiedTaskFetcher
```

## 为什么需要适配器？

### 问题场景
如果不使用适配器，直接依赖具体实现：
```go
// ❌ 紧耦合
type UnifiedTaskFetcher struct {
    managementClient *management.ClientManager  // 具体类型
}

// 问题:
// 1. 难以测试 - 必须创建真实的 ClientManager
// 2. 难以扩展 - 无法替换实现
// 3. 违反 DIP - 高层模块依赖低层模块
```

### 使用适配器后
```go
// ✅ 松耦合
type UnifiedTaskFetcher struct {
    managementClient ManagementClientProvider  // 接口类型
}

// 优势:
// 1. 易于测试 - 可以 mock 接口
// 2. 易于扩展 - 可以添加缓存、日志等装饰器
// 3. 符合 DIP - 依赖抽象而不是具体实现
```

## 总结

适配器模式在这里的作用：
1. ✅ **解耦** - UnifiedTaskFetcher 不依赖具体的 ClientManager
2. ✅ **可测试** - 可以轻松 mock 接口
3. ✅ **灵活** - 可以添加缓存、日志等功能
4. ✅ **符合 SOLID 原则** - 依赖倒置原则 (DIP)

虽然增加了一些代码，但带来了更好的架构和可维护性。
