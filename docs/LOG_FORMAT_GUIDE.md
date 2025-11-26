# 日志格式统一指南

## 问题

项目中存在两种日志格式：

```go
// 格式1 (旧格式 - logrus.Infof)
logrus.Infof("加载配置文件: %s", filename)
// 输出: time="2025-11-19T13:02:44+08:00" level=info msg="加载配置文件: config.yaml"

// 格式2 (新格式 - logrus.Infof)
logrus.Infof("加载配置文件: %s", filename)
// 输出: INFO[2025-11-19 13:02:44] 加载配置文件: config.yaml
```

## 解决方案

### 短期方案：统一使用 logrus 标准方法

将所有 `logrus.Infof` 替换为对应的级别方法：

```go
// 替换规则
logrus.Infof(...)  → logrus.Infof(...)
logrus.Infof(...)     → logrus.Infof(...)
```

### 长期方案：使用统一的日志包装器

创建统一的日志工具函数（已在 `common/utils/logger.go` 中实现）：

```go
// 使用结构化日志
utils.WithTaskContext(logger, taskID, productID, platform).Info("处理任务")
utils.WithProcessorContext(logger, "TEMU", "WorkerPool").Info("启动")
```

## 批量替换命令

### 方法1: 使用 sed (Linux/Mac)
```bash
find . -name "*.go" -type f -exec sed -i 's/logrus\.Printf/logrus.Infof/g' {} \;
```

### 方法2: 使用 PowerShell (Windows)
```powershell
Get-ChildItem -Path . -Include *.go -Recurse | ForEach-Object {
    $content = Get-Content $_.FullName -Raw
    $newContent = $content -replace 'logrus\.Printf', 'logrus.Infof'
    if ($content -ne $newContent) {
        Set-Content -Path $_.FullName -Value $newContent -NoNewline
        Write-Host "Updated: $($_.FullName)"
    }
}
```

### 方法3: 手动替换（推荐）

在 IDE 中使用全局搜索替换：
1. 搜索: `logrus\.Printf`
2. 替换为: `logrus.Infof`
3. 全部替换

## 日志级别使用指南

```go
// DEBUG - 详细的调试信息
logrus.Debugf("变量值: %v", value)

// INFO - 一般信息
logrus.Infof("任务开始处理: %s", taskID)

// WARN - 警告信息（不影响主流程）
logrus.Warnf("配置项缺失，使用默认值: %v", defaultValue)

// ERROR - 错误信息（影响功能但不致命）
logrus.Errorf("处理失败: %v", err)

// FATAL - 致命错误（程序无法继续）
logrus.Fatalf("初始化失败: %v", err)
```

## 结构化日志示例

```go
// 带上下文的日志
logger.WithFields(logrus.Fields{
    "task_id":    taskID,
    "product_id": productID,
    "platform":   "TEMU",
}).Info("开始处理任务")

// 使用辅助函数
utils.WithTaskContext(logger, taskID, productID, "TEMU").Info("开始处理任务")
```

## 当前状态

已修复的文件：
- ✅ `common/config/config.go`

待修复的文件（17个）：
- ⏳ `platforms/temu/handlers/text_check_handler.go`
- ⏳ `platforms/temu/handlers/internal/downloader/image_downloader.go`
- ⏳ `platforms/shein/modules/*.go` (多个文件)
- ⏳ `common/worker/pool.go`
- ⏳ `common/util.go`
- ⏳ `common/management/impl/image_downloader.go`

## 优先级

1. **P0 (立即)**: 核心启动流程
   - ✅ `common/config/config.go`

2. **P1 (近期)**: 主要业务逻辑
   - `common/worker/pool.go`
   - `platforms/*/modules/*.go`

3. **P2 (长期)**: 辅助功能
   - 其他文件

## 注意事项

1. 替换时注意保留原有的格式化参数
2. 确保日志级别选择正确
3. 考虑使用结构化日志提高可读性
4. 测试替换后的日志输出是否正常
