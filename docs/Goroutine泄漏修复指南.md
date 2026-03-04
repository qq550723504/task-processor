# Goroutine 泄漏修复指南

## 📋 问题概述

在代码审查中发现 50+ 处 `go func()` 调用，存在以下潜在风险：
1. 缺少 context 取消机制
2. 无限期运行的 goroutine
3. 为每个操作创建新的 goroutine（资源浪费）
4. 缺少超时控制

## 🔍 已识别的问题类型

### 类型 1: 无限制创建 Goroutine

**问题代码：**
```go
// 每次过期检查都创建新的 goroutine
if time.Now().After(item.ExpiresAt) {
    go func() {
        c.mutex.Lock()
        delete(c.featureCache, key)
        c.mutex.Unlock()
    }()
    return PropertyFeature{}, false
}
```

**问题：**
- 高频调用时会创建大量 goroutine
- 可能导致内存和 CPU 资源耗尽

**修复方案：**
使用工作队列模式，单个 goroutine 处理所有清理任务：

```go
type InMemoryPropertyCache struct {
    cleanupQueue  chan func()
    stopCleanup   chan struct{}
}

func (c *InMemoryPropertyCache) startCleanupWorker() {
    go func() {
        for {
            select {
            case cleanupFn := <-c.cleanupQueue:
                cleanupFn()
            case <-c.stopCleanup:
                return
            }
        }
    }()
}

// 使用时
select {
case c.cleanupQueue <- func() {
    c.mutex.Lock()
    delete(c.featureCache, key)
    c.mutex.Unlock()
}:
default:
    // 队列满了，同步清理
}
```

### 类型 2: 缺少 Context 取消机制

**问题代码：**
```go
go func() {
    ticker := time.NewTicker(5 * time.Minute)
    defer ticker.Stop()
    
    for {
        <-ticker.C
        performHealthCheck()
    }
}()
```

**问题：**
- goroutine 永远不会停止
- 程序退出时可能导致资源泄漏

**修复方案：**
使用 context 控制生命周期：

```go
func (hc *HealthChecker) StartHealthCheckRoutine(ctx context.Context) {
    go func() {
        ticker := time.NewTicker(5 * time.Minute)
        defer ticker.Stop()
        
        for {
            select {
            case <-ctx.Done():
                logrus.Info("健康检查例程停止")
                return
            case <-ticker.C:
                hc.performHealthCheck()
            }
        }
    }()
}
```

### 类型 3: 缺少超时控制

**问题代码：**
```go
go func() {
    result := longRunningOperation()
    resultChan <- result
}()
```

**问题：**
- 操作可能永远不会完成
- goroutine 永远不会退出

**修复方案：**
使用带超时的 context：

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

go func() {
    select {
    case <-ctx.Done():
        resultChan <- Result{Error: ctx.Err()}
    case result := <-operationChan:
        resultChan <- result
    }
}()
```

### 类型 4: 异步操作缺少错误处理

**问题代码：**
```go
go func() {
    updateStatus() // 可能失败但无人知道
}()
```

**问题：**
- 错误被静默忽略
- 难以调试

**修复方案：**
使用 AsyncTask 包装器：

```go
task := utils.NewAsyncTask(ctx, "update-status", func(ctx context.Context) error {
    return updateStatus()
})

// 可以选择等待结果
if err := task.Wait(); err != nil {
    logger.Errorf("更新状态失败: %v", err)
}
```

## ✅ 已修复的文件

### 1. internal/platforms/temu/handlers/property_cache.go

**修复内容：**
- 使用工作队列替代无限制创建 goroutine
- 添加 Close() 方法停止清理工作协程
- 队列满时降级为同步清理

**影响：**
- 减少 goroutine 数量从 N（缓存项数）到 1
- 避免高频调用时的资源耗尽

## 🛠️ 新增工具

### SafeGo - 安全的 Goroutine 启动

```go
utils.SafeGo(ctx, "task-name", func(ctx context.Context) {
    // 自动 panic 恢复
    // 自动日志记录
})
```

### GoroutinePool - Goroutine 池

```go
pool := utils.NewGoroutinePool(ctx, 10, logger)
defer pool.Close()

pool.Submit("task-1", func(ctx context.Context) error {
    // 限制并发数
    // 自动资源管理
    return nil
})
```

### AsyncTask - 异步任务包装器

```go
task := utils.NewAsyncTask(ctx, "task-name", func(ctx context.Context) error {
    return doWork()
})

// 等待结果
err := task.WaitWithTimeout(30 * time.Second)
```

### PeriodicTask - 周期性任务

```go
task := utils.NewPeriodicTask(ctx, "cleanup", 5*time.Minute, func(ctx context.Context) error {
    return cleanup()
}, logger)

task.Start()
defer task.Stop()
```

## 📊 修复优先级

### 高优先级（立即修复）

1. ✅ **property_cache.go** - 无限制创建 goroutine
2. ⏳ **health_checker.go** - 长期运行的 goroutine 缺少 context
3. ⏳ **shop_pause_manager.go** - 清理任务缺少 context
4. ⏳ **config_service.go** - 异步保存配置缺少错误处理

### 中优先级（逐步修复）

5. ⏳ **inventory_sync_*.go** - 批量处理缺少超时控制
6. ⏳ **image_upload_worker.go** - 并发上传缺少资源限制
7. ⏳ **status_service.go** - 异步更新状态缺少错误处理

### 低优先级（优化改进）

8. ⏳ **parallel_handler.go** - 已有 WaitGroup，但可以改用 GoroutinePool
9. ⏳ **parallel_processor.go** - 已有基本控制，可以增强错误处理

## 🎯 最佳实践

### 1. 始终使用 Context

```go
// ✅ 好的做法
func StartWorker(ctx context.Context) {
    go func() {
        for {
            select {
            case <-ctx.Done():
                return
            case work := <-workChan:
                process(work)
            }
        }
    }()
}

// ❌ 不好的做法
func StartWorker() {
    go func() {
        for {
            work := <-workChan
            process(work)
        }
    }()
}
```

### 2. 限制并发数

```go
// ✅ 好的做法
pool := utils.NewGoroutinePool(ctx, 10, logger)
for _, item := range items {
    pool.Submit("process", func(ctx context.Context) error {
        return process(item)
    })
}
pool.Wait()

// ❌ 不好的做法
for _, item := range items {
    go process(item) // 可能创建数千个 goroutine
}
```

### 3. 设置超时

```go
// ✅ 好的做法
ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
defer cancel()

err := utils.SafeGoWithTimeout(ctx, "task", 30*time.Second, func(ctx context.Context) error {
    return longRunningTask()
})

// ❌ 不好的做法
go longRunningTask() // 可能永远不会完成
```

### 4. 处理 Panic

```go
// ✅ 好的做法
utils.SafeGo(ctx, "task", func(ctx context.Context) {
    // 自动 panic 恢复和日志记录
    riskyOperation()
})

// ❌ 不好的做法
go func() {
    riskyOperation() // panic 会导致程序崩溃
}()
```

### 5. 优雅关闭

```go
// ✅ 好的做法
type Service struct {
    ctx    context.Context
    cancel context.CancelFunc
    wg     sync.WaitGroup
}

func (s *Service) Start() {
    s.wg.Add(1)
    go func() {
        defer s.wg.Done()
        for {
            select {
            case <-s.ctx.Done():
                return
            case work := <-s.workChan:
                s.process(work)
            }
        }
    }()
}

func (s *Service) Stop() {
    s.cancel()
    s.wg.Wait()
}
```

## 🧪 测试建议

### 1. 使用 Goroutine 泄漏检测

```go
import "go.uber.org/goleak"

func TestMain(m *testing.M) {
    goleak.VerifyTestMain(m)
}
```

### 2. 测试超时场景

```go
func TestTimeout(t *testing.T) {
    ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
    defer cancel()
    
    err := service.Process(ctx)
    if err != context.DeadlineExceeded {
        t.Errorf("Expected timeout error, got: %v", err)
    }
}
```

### 3. 测试取消场景

```go
func TestCancel(t *testing.T) {
    ctx, cancel := context.WithCancel(context.Background())
    
    go func() {
        time.Sleep(50 * time.Millisecond)
        cancel()
    }()
    
    err := service.Process(ctx)
    if err != context.Canceled {
        t.Errorf("Expected canceled error, got: %v", err)
    }
}
```

## 📈 性能影响

### 修复前
- 高频缓存访问：每秒可能创建 1000+ goroutine
- 内存使用：不可预测，可能持续增长
- CPU 使用：goroutine 调度开销大

### 修复后
- 固定数量的 worker goroutine
- 内存使用：稳定可控
- CPU 使用：显著降低

## 🔄 持续改进

1. 定期使用 pprof 检查 goroutine 数量
2. 监控 goroutine 泄漏指标
3. 代码审查时重点关注 `go func()` 使用
4. 使用静态分析工具检测潜在问题

## 📚 参考资料

- [Go Concurrency Patterns](https://go.dev/blog/pipelines)
- [Context Package](https://pkg.go.dev/context)
- [Goroutine Leaks](https://www.ardanlabs.com/blog/2018/11/goroutine-leaks-the-forgotten-sender.html)

---

**文档版本：** v1.0  
**最后更新：** 2024-03-04  
**维护者：** 开发团队
