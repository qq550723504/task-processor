# Logger - 统一日志管理系统

## 概述

这是一个为 task-processor 项目设计的统一日志管理系统，提供了完整的日志功能，包括日志轮转、Context 集成、辅助工具等。

## 特性

- ✅ 统一的日志管理接口
- ✅ 自动日志文件轮转和压缩
- ✅ 支持多输出目标（控制台 + 文件）
- ✅ Context 集成，支持链路追踪
- ✅ 丰富的辅助工具
- ✅ 标准化的字段命名
- ✅ 动态调整日志级别
- ✅ 完整的测试覆盖

## 快速开始

### 基础使用

```go
import "task-processor/internal/core/logger"

// 获取 logger
log := logger.GetGlobalLogger("my_service")

// 记录日志
log.Info("服务启动")
log.WithField("port", 8080).Info("监听端口")
log.WithError(err).Error("操作失败")
```

### 在结构体中使用

```go
type MyService struct {
    logger *logrus.Entry
    helper *logger.LoggerHelper
}

func NewMyService() *MyService {
    log := logger.GetGlobalLogger("my_service")
    return &MyService{
        logger: log,
        helper: logger.NewLoggerHelper(log),
    }
}

func (s *MyService) DoWork() error {
    // 自动记录操作耗时
    return s.helper.LogOperation("do_work", func() error {
        // 业务逻辑
        return nil
    })
}
```

### 使用标准字段

```go
log.WithFields(logrus.Fields{
    logger.FieldTaskID:    taskID,
    logger.FieldProductID: productID,
    logger.FieldTenantID:  tenantID,
    logger.FieldStoreID:   storeID,
}).Info("处理任务")
```

### Context 集成

```go
// 添加 logger 到 context
ctx = logger.WithLogger(ctx, log)
ctx = logger.WithTraceID(ctx, "trace-123")

// 从 context 获取 logger
log := logger.FromContext(ctx, "component")
```

## 配置

### 配置文件示例

```yaml
log:
  level: info              # 日志级别: debug, info, warn, error
  format: json             # 日志格式: json, text
  output_file: logs/app.log
  max_size: 100            # 单文件最大大小(MB)
  max_backups: 10          # 保留的备份文件数
  max_age: 30              # 保留天数
  compress: true           # 是否压缩旧文件
  console: true            # 是否输出到控制台
  report_caller: false     # 是否记录调用位置
```

### 代码配置

```go
import "task-processor/internal/core/logger"

config := &logger.LogConfig{
    Level:      "info",
    Format:     "json",
    OutputFile: "logs/app.log",
    MaxSize:    100,
    MaxBackups: 10,
    MaxAge:     30,
    Compress:   true,
    Console:    true,
}

logger.InitGlobalLogger(config)
```

## 辅助工具

### LoggerHelper

提供了一系列便捷的日志记录方法：

```go
helper := logger.NewLoggerHelper(log)

// 记录操作（自动计时）
helper.LogOperation("fetch_data", func() error {
    return fetchData()
})

// 记录进度
helper.LogProgress(50, 100, "处理中")

// 记录重试
helper.LogRetry("api_call", 1, 3, err)

// 记录任务生命周期
helper.LogTaskStart(taskID, productID)
helper.LogTaskComplete(taskID, duration)
helper.LogTaskFailed(taskID, err)

// 记录 API 调用
helper.LogAPICall("GET", url, 200, duration)

// 记录状态变更
helper.LogStateChange("task", taskID, "pending", "processing")
```

### 标准字段常量

```go
const (
    FieldComponent   = "component"
    FieldPlatform    = "platform"
    FieldTaskID      = "task_id"
    FieldProductID   = "product_id"
    FieldTenantID    = "tenant_id"
    FieldStoreID     = "store_id"
    FieldTraceID     = "trace_id"
    FieldRequestID   = "request_id"
    FieldDurationMs  = "duration_ms"
    FieldRetryCount  = "retry_count"
    FieldErrorCode   = "error_code"
    FieldErrorType   = "error_type"
    FieldOperation   = "operation"
    FieldStatus      = "status"
)
```

## 日志轮转

日志文件会自动轮转，无需手动管理：

- 当文件大小达到 `max_size` 时自动轮转
- 自动压缩旧日志文件（.gz）
- 自动清理超过 `max_backups` 数量的文件
- 自动清理超过 `max_age` 天数的文件

轮转后的文件命名格式：
```
app.log.20240304-150405.gz
```

## 文件结构

```
internal/core/logger/
├── manager.go           # 日志管理器
├── rotating_writer.go   # 日志轮转实现
├── context.go           # Context 集成
├── helpers.go           # 辅助工具
├── manager_test.go      # 管理器测试
├── helpers_test.go      # 辅助工具测试
└── README.md            # 本文档
```

## 最佳实践

### 1. 使用依赖注入

```go
type Service struct {
    logger *logrus.Entry
}

func NewService() *Service {
    return &Service{
        logger: logger.GetGlobalLogger("service"),
    }
}
```

### 2. 使用标准字段常量

```go
// ✅ 推荐
log.WithField(logger.FieldTaskID, taskID)

// ❌ 不推荐
log.WithField("taskId", taskID)
```

### 3. 错误日志使用 WithError

```go
// ✅ 推荐
log.WithError(err).Error("操作失败")

// ❌ 不推荐
log.Errorf("操作失败: %v", err)
```

### 4. 避免过多日志

```go
// ✅ 推荐：每100个记录一次
for i, item := range items {
    if (i+1)%100 == 0 {
        helper.LogProgress(i+1, len(items), "处理中")
    }
    processItem(item)
}

// ❌ 不推荐：每次都记录
for i, item := range items {
    log.Infof("处理 %d/%d", i+1, len(items))
    processItem(item)
}
```

### 5. 使用 Context 传递 Logger

```go
// ✅ 推荐
func ProcessTask(ctx context.Context, task Task) error {
    log := logger.GetGlobalLogger("processor").WithField("task_id", task.ID)
    ctx = logger.WithLogger(ctx, log)
    return s.service.Process(ctx, task)
}

func (s *Service) Process(ctx context.Context, task Task) error {
    log := logger.FromContext(ctx, "service")
    log.Info("处理中") // 自动包含 task_id
    return nil
}
```

## 性能考虑

1. **复用 Logger 实例**：在结构体中保存 logger，避免重复创建
2. **使用字段而非格式化**：使用 `WithField()` 而不是 `Infof()`
3. **日志采样**：对于高频日志，使用 `ShouldLog()` 进行采样
4. **异步轮转**：日志轮转和压缩在后台异步执行

## 测试

运行测试：

```bash
go test ./internal/core/logger/...
```

运行基准测试：

```bash
go test -bench=. ./internal/core/logger/...
```

## 相关文档

- [日志使用规范](../../../docs/日志使用规范.md)
- [日志迁移指南](../../../docs/日志迁移指南.md)
- [日志迁移示例](../../../docs/日志迁移示例.md)
- [日志系统改进总结](../../../docs/日志系统改进总结.md)

## 常见问题

### Q: 如何动态调整日志级别？
```go
logger.SetGlobalLogLevel("debug")
```

### Q: 如何在测试中使用？
```go
func TestMyFunction(t *testing.T) {
    logger.SetGlobalLogLevel("debug")
    log := logger.GetGlobalLogger("test")
    // 测试代码
}
```

### Q: 日志文件在哪里？
默认在 `logs/` 目录下，可以通过配置文件修改。

### Q: 如何查看轮转后的日志？
```bash
# 解压查看
gunzip -c logs/app.log.20240304-150405.gz | less

# 或者直接查看
zless logs/app.log.20240304-150405.gz
```

## 贡献

如果发现问题或有改进建议，请：
1. 查看现有文档
2. 运行测试确保功能正常
3. 提交 issue 或 PR

## 许可

与项目主许可证相同。
