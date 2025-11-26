# 任务完成后清理修复

## 问题描述

程序出现任务卡住的情况：
- 任务失败后（如 Category unavailable 错误）
- 任务ID一直保留在 `processingTasks` 中
- 后续获取到相同任务时被跳过（认为还在处理中）
- 导致任务永远无法重试

## 根本原因

`UnifiedTaskFetcher` 在任务提交时将 TaskID 加入 `processingTasks`，但任务完成（成功或失败）后没有移除。

## 解决方案

### 1. 添加任务完成通知机制

**接口定义** (`common/processor/processor.go`):
```go
type TaskCompletionNotifier interface {
    OnTaskCompleted(taskID string)
}
```

**WorkerPool接口扩展**:
```go
type WorkerPool interface {
    // ... 其他方法
    SetCompletionNotifier(notifier TaskCompletionNotifier)
}
```

### 2. WorkerPool实现通知

**Pool结构** (`common/worker/pool.go`):
- 添加 `completionNotifier` 字段
- 在任务处理完成后（defer）调用通知器

### 3. UnifiedTaskFetcher实现接口

**实现** (`common/task/fetcher.go`):
```go
func (f *UnifiedTaskFetcher) OnTaskCompleted(taskID string) {
    f.RemoveProcessingTask(taskID)
}
```

### 4. 启动时注册通知器

**Server启动** (`cmd/temu-web/server/server.go`):
```go
if temuPool := s.temuProcessor.GetWorkerPool(); temuPool != nil {
    temuPool.SetCompletionNotifier(unifiedFetcher)
}
if sheinPool := s.sheinProcessor.GetWorkerPool(); sheinPool != nil {
    sheinPool.SetCompletionNotifier(unifiedFetcher)
}
```

## 效果

- ✅ 任务完成后立即从 `processingTasks` 移除
- ✅ 失败任务可以正常重试
- ✅ 避免任务永久卡住
- ✅ 保留30分钟超时清理作为兜底机制

## 测试验证

启动程序后观察日志：
1. 任务提交：`任务已提交到工作池`
2. 任务完成：`任务处理完成` 或 `处理任务失败`
3. 清理通知：`✅ 任务已从处理队列移除: TaskID=xxx`
