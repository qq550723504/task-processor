# 日志使用指南

## 统一日志规范

本项目统一使用 `logrus` 的全局方法记录日志，确保所有日志都能正确输出到控制台和文件。

## 使用方法

### 导入包
```go
import "github.com/sirupsen/logrus"
```

### 记录日志
```go
// 信息日志
logrus.Info("这是一条信息日志")
logrus.Infof("处理产品: %s", productID)

// 警告日志
logrus.Warn("这是一条警告日志")
logrus.Warnf("库存不足: %d", stock)

// 错误日志
logrus.Error("这是一条错误日志")
logrus.Errorf("处理失败: %v", err)

// 调试日志
logrus.Debug("这是一条调试日志")
logrus.Debugf("变量值: %+v", data)
```

### 带字段的日志
```go
logrus.WithFields(logrus.Fields{
    "task_id": taskID,
    "product_id": productID,
    "platform": "TEMU",
}).Info("开始处理任务")
```

## 日志配置

### 环境变量

- **LOG_LEVEL**: 设置日志级别
  - `DEBUG`: 调试级别，输出所有日志
  - `INFO`: 信息级别（默认），输出INFO及以上级别
  - `WARN`: 警告级别，输出WARN及以上级别
  - `ERROR`: 错误级别，只输出ERROR级别

- **LOG_FILE**: 设置日志文件路径
  - 默认: `logs/app.log`
  - 示例: `LOG_FILE=logs/custom.log`

- **LOG_FORMAT**: 设置日志格式
  - `text`: 文本格式（默认）
  - `json`: JSON格式

### 示例配置

Windows (PowerShell):
```powershell
$env:LOG_LEVEL="DEBUG"
$env:LOG_FILE="logs/debug.log"
```

Windows (CMD):
```cmd
set LOG_LEVEL=DEBUG
set LOG_FILE=logs/debug.log
```

Linux/Mac:
```bash
export LOG_LEVEL=DEBUG
export LOG_FILE=logs/debug.log
```

## 日志输出

### 控制台输出
- 带颜色的日志，便于阅读
- 实时显示日志信息

### 文件输出
- 纯文本格式，无颜色代码
- 默认路径: `logs/app.log`
- 自动创建日志目录
- 追加模式，不会覆盖旧日志

## 注意事项

1. **统一使用全局方法**: 不要创建新的logger实例，统一使用 `logrus.Info()` 等全局方法
2. **避免使用 fmt.Println**: 使用 `logrus.Info()` 代替 `fmt.Println()`
3. **合理使用日志级别**: 
   - DEBUG: 详细的调试信息
   - INFO: 一般信息，如任务开始、完成
   - WARN: 警告信息，如配置缺失、使用默认值
   - ERROR: 错误信息，如处理失败、异常情况
4. **敏感信息**: 不要在日志中输出密码、token等敏感信息

## 最佳实践

### 任务处理日志
```go
logrus.Infof("[TEMU] 开始处理任务: ID=%s, ProductID=%s", task.ID, task.ProductID)
// ... 处理逻辑
logrus.Infof("[TEMU] 任务处理完成: ID=%s", task.ID)
```

### 错误处理日志
```go
if err != nil {
    logrus.Errorf("处理失败: %v", err)
    return fmt.Errorf("处理失败: %w", err)
}
```

### 性能监控日志
```go
startTime := time.Now()
// ... 处理逻辑
duration := time.Since(startTime)
logrus.Infof("处理耗时: %v", duration)
```

## 日志查看

### 实时查看日志
Windows (PowerShell):
```powershell
Get-Content logs/app.log -Wait -Tail 50
```

Windows (CMD):
```cmd
type logs\app.log
```

Linux/Mac:
```bash
tail -f logs/app.log
```

### 搜索日志
Windows (PowerShell):
```powershell
Select-String -Path logs/app.log -Pattern "ERROR"
```

Linux/Mac:
```bash
grep "ERROR" logs/app.log
```
