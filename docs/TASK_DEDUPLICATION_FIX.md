# Amazon爬虫重复打开相同链接问题修复

## 问题描述
Amazon爬虫会重复打开相同的产品链接，导致资源浪费和任务重复处理。

## 根本原因分析

### 1. 缺少内存去重机制
任务获取器（`UnifiedTaskFetcher`）从API获取任务后，直接分发到worker队列，没有检查该任务是否已经在队列中。

### 2. 缺少API状态同步
任务提交到队列后，没有立即更新API端的任务状态为"处理中"，导致：
- 任务在API端仍然是 `pending` 状态
- 下次获取任务时，又会拿到相同的任务
- 即使有内存去重，任务处理完成后从内存移除，下次还是会重复获取

## 解决方案

### 1. 添加内存去重机制（`common/task/fetcher.go`）

#### 1.1 添加任务跟踪
```go
type UnifiedTaskFetcher struct {
    // ... 其他字段
    processingTasks  map[string]time.Time // taskID -> 提交时间，用于去重
    tasksMutex       sync.RWMutex         // 保护 processingTasks
}
```

#### 1.2 提交前检查重复
```go
// 去重检查：跳过已在处理中的任务
f.tasksMutex.RLock()
submitTime, isProcessing := f.processingTasks[taskID]
f.tasksMutex.RUnlock()

if isProcessing {
    logrus.Debugf("⏭️ 跳过重复任务: TaskID=%s (已在队列中，提交时间: %v)", taskID, submitTime)
    continue
}
```

#### 1.3 提交成功后标记
```go
// 提交成功后，标记任务为处理中
f.tasksMutex.Lock()
f.processingTasks[internalTask.ID] = time.Now()
f.tasksMutex.Unlock()
```

#### 1.4 定期清理过期记录
```go
// 每5分钟清理超过30分钟的任务记录
func (f *UnifiedTaskFetcher) cleanupExpiredTasks() {
    ticker := time.NewTicker(5 * time.Minute)
    defer ticker.Stop()

    for range ticker.C {
        // 清理超过30分钟的任务记录
        for taskID, submitTime := range f.processingTasks {
            if now.Sub(submitTime) > 30*time.Minute {
                delete(f.processingTasks, taskID)
            }
        }
    }
}
```

### 2. 添加API状态同步

#### 2.1 扩展接口（`common/task/interfaces.go`）
```go
type ImportTaskClient interface {
    GetPendingAndRetryTasks(maxTasks int, userID int64, storeIDs []int64) ([]TaskDTO, error)
    UpdateTaskStatus(taskID int64, status int16, errorMessage string) error // 新增
}
```

#### 2.2 实现适配器（`common/task/adapters.go`）
```go
func (a *ImportTaskClientAdapter) UpdateTaskStatus(taskID int64, status int16, errorMessage string) error {
    req := &api.ProductImportTaskUpdateReqDTO{
        ID:           taskID,
        Status:       status,
        ErrorMessage: errorMessage,
    }
    return a.client.UpdateTaskStatus(req)
}
```

#### 2.3 任务提交后立即更新状态（`common/task/fetcher.go`）
```go
// 提交成功后，立即更新API端任务状态为"处理中"
f.updateTaskStatusToProcessing(apiTask.ID)

// 异步更新，避免阻塞任务分发
func (f *UnifiedTaskFetcher) updateTaskStatusToProcessing(taskID int64) {
    go func() {
        importTaskClient := f.managementClient.GetImportTaskClient()
        if err := importTaskClient.UpdateTaskStatus(taskID, common.TaskStatusProcessing.Int16(), ""); err != nil {
            logrus.Warnf("更新任务状态为处理中失败: TaskID=%d, Error=%v", taskID, err)
        } else {
            logrus.Debugf("✅ 任务状态已更新为处理中: TaskID=%d", taskID)
        }
    }()
}
```

## 修改的文件

1. **common/task/fetcher.go**
   - 添加 `processingTasks` map 和 `tasksMutex`
   - 添加去重检查逻辑
   - 添加 `cleanupExpiredTasks()` 方法
   - 添加 `updateTaskStatusToProcessing()` 方法

2. **common/task/interfaces.go**
   - 在 `ImportTaskClient` 接口添加 `UpdateTaskStatus` 方法

3. **common/task/adapters.go**
   - 在 `ImportTaskClientAdapter` 实现 `UpdateTaskStatus` 方法

## 工作流程

```
1. 从API获取待处理任务
   ↓
2. 检查任务是否已在内存中（去重）
   ↓ 否
3. 提交任务到worker队列
   ↓ 成功
4. 标记任务到内存（防止重复获取）
   ↓
5. 异步更新API状态为"处理中"（防止下次获取）
   ↓
6. Worker处理任务
   ↓
7. 处理完成后更新API状态为"已完成"/"草稿箱"/"已终止"
   ↓
8. 定期清理过期的内存记录（30分钟）
```

## 效果

- ✅ 防止同一任务在短时间内被重复获取
- ✅ 防止同一任务在API端被重复拉取
- ✅ 自动清理过期的任务记录，避免内存泄漏
- ✅ 异步更新状态，不阻塞任务分发流程

## 测试建议

1. 启动程序，观察日志中是否有"⏭️ 跳过重复任务"的提示
2. 检查API端任务状态是否在提交后立即变为"处理中"
3. 观察是否还有重复打开相同链接的情况
4. 检查内存中的任务记录是否会定期清理

## 注意事项

- 内存去重只在单个进程内有效，如果有多个进程，需要使用Redis等分布式缓存
- 任务状态更新失败不会影响任务处理，只会记录警告日志
- 过期任务清理时间设置为30分钟，可根据实际情况调整
