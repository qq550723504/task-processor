# 重复代码分析报告

生成时间: 2026-03-10

---

## 概述

本报告分析了项目中存在的重复代码模式，并提供了优化建议。

---

## 已消除的重复代码

### 1. 工具函数重复 ✅

**问题描述:**
- IntPtr, StringPtr, Float64Ptr 等指针工具函数在多个地方重复实现
- Abs, Min, Max 等数学函数在多个地方重复实现
- parsePrice, parseStock 等解析函数在多个地方重复实现

**解决方案:**
- 统一使用 `internal/pkg/ptrutil` 包
- 统一使用 `internal/pkg/mathutil` 包
- 创建 `internal/pkg/strutil/parse.go` 统一解析函数

**收益:**
- 删除重复代码约 80 行
- 提高代码一致性
- 简化维护

---

## 发现的重复代码模式

### 2. JSON 解析重复 🔴 高优先级

**问题描述:**
在整个项目中发现了大量重复的 JSON 解析代码模式：

```go
var data SomeType
if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
    return fmt.Errorf("解析xxx失败: %w", err)
}
```

**统计数据:**
- TEMU 平台: 约 20+ 处重复
- SHEIN 平台: 约 30+ 处重复
- 其他模块: 约 10+ 处重复
- 总计: 60+ 处重复

**影响范围:**
- `internal/platforms/temu/services/business_service/*.go`
- `internal/platforms/shein/service/business_service/*.go`
- `internal/platforms/temu/handlers/**/*.go`
- `internal/platforms/shein/service/**/*.go`

**建议解决方案:**
创建 `internal/pkg/jsonutil` 包提供泛型辅助函数：

```go
// 使用前
var task model.Task
if err := json.Unmarshal([]byte(taskData), &task); err != nil {
    return fmt.Errorf("解析任务数据失败: %w", err)
}

// 使用后
var task model.Task
if err := jsonutil.UnmarshalString(taskData, &task, "解析任务数据失败"); err != nil {
    return err
}
```

**预期收益:**
- 减少约 200+ 行重复代码
- 统一错误处理格式
- 提高代码可读性

**优先级:** 高
**预计工作量:** 4-6 小时

---

### 3. HTTP 客户端创建重复 🟡 中优先级

**问题描述:**
虽然已有 `internal/pkg/utils/http_client.go` 提供工具函数，但仍有部分代码直接创建 HTTP 客户端：

```go
client := &http.Client{
    Timeout: 30 * time.Second,
}
```

**发现位置:**
- `internal/crawler/shared/browser/utils.go`
- `internal/app/messaging/result_reporter.go`

**建议解决方案:**
统一使用 `utils.CreateSimpleHTTPClientWithTimeout()`

**预期收益:**
- 统一 HTTP 客户端配置
- 便于全局调整超时策略

**优先级:** 中
**预计工作量:** 1 小时

---

### 4. 错误包装模式重复 🟡 中优先级

**问题描述:**
大量使用 `fmt.Errorf("xxx: %v", err)` 模式包装错误，特别是在 panic 恢复场景：

```go
defer func() {
    if r := recover(); r != nil {
        return fmt.Errorf("处理时发生panic: %v", r)
    }
}()
```

**统计数据:**
- 约 60+ 处 panic 恢复代码（defer func() { if r := recover()）
- 约 100+ 处错误包装代码

**影响范围:**
- 几乎所有的 goroutine 启动处
- 所有的并发处理代码
- 大量的业务逻辑处理

**建议解决方案:**
1. 创建统一的 panic 恢复辅助函数
2. 考虑使用 `errors.Wrap()` 替代 `fmt.Errorf()`

**预期收益:**
- 统一错误处理模式
- 减少样板代码约 100+ 行

**优先级:** 中
**预计工作量:** 3-4 小时

---

### 5. Context 超时创建重复 🟡 中优先级

**问题描述:**
大量重复的 `context.WithTimeout()` 调用模式：

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
```

**统计数据:**
- 约 50+ 处重复的超时上下文创建
- 常见超时值：10s, 30s, 60s, 2min, 5min

**影响范围:**
- AI 调用处（通常 30s 或 60s）
- HTTP 请求处（通常 10s）
- 任务处理处（通常 2min 或更长）

**建议解决方案:**
虽然已有 `utils.WithTimeout()`，但使用率低。可以：
1. 创建预定义的超时常量
2. 创建特定场景的辅助函数（如 `WithAITimeout`, `WithHTTPTimeout`）

**预期收益:**
- 统一超时配置
- 便于全局调整超时策略
- 减少约 50 行重复代码

**优先级:** 中
**预计工作量:** 2-3 小时

---

### 6. 时间格式化重复 🟢 低优先级

**问题描述:**
多处使用相同的时间格式化字符串：

```go
time.Now().Format("2006-01-02")           // 日期格式，约 10+ 处
time.Now().Format("20060102_150405")      // 文件名时间戳，约 10+ 处
time.Now().Format("2006-01-02 15:04:05")  // 完整时间，约 5+ 处
time.Now().Format(time.RFC3339)           // RFC3339 格式，约 5+ 处
```

**建议解决方案:**
创建时间格式化常量或辅助函数：

```go
const (
    DateFormat     = "2006-01-02"
    TimestampFormat = "20060102_150405"
    DateTimeFormat = "2006-01-02 15:04:05"
)

func FormatDate(t time.Time) string { return t.Format(DateFormat) }
func FormatTimestamp(t time.Time) string { return t.Format(TimestampFormat) }
```

**预期收益:**
- 统一时间格式
- 避免格式字符串错误
- 减少约 20 行重复代码

**优先级:** 低
**预计工作量:** 1 小时

---

### 7. 日志初始化模式 🟢 低优先级

**问题描述:**
大量使用 `logger.GetGlobalLogger()` 初始化日志，这是正常的模式，不算重复。

**结论:**
这是合理的设计模式，无需优化。

---

### 6. 字符串拼接模式 🟢 低优先级

**问题描述:**
使用 `strings.Join()` 拼接字符串，这是标准库的正确用法。

**结论:**
这是合理的使用方式，无需优化。

---

## 优化建议优先级

### 高优先级（建议立即处理）
1. ✅ 工具函数重复 - 已完成
2. 🔴 JSON 解析重复 - 进行中（已创建工具包和迁移指南）

### 中优先级（建议近期处理）
3. ✅ HTTP 客户端创建重复 - 已完成
4. 🟡 错误包装和 panic 恢复模式重复
5. 🟡 Context 超时创建重复

### 低优先级（可选优化）
6. 🟢 时间格式化重复
7. 🟢 考虑提取更多的业务逻辑公共代码

---

## 下一步行动计划

### 阶段一：JSON 解析统一（推荐）
1. 创建 `internal/pkg/jsonutil` 包 ✅
2. 逐步替换 TEMU 平台的 JSON 解析代码
3. 逐步替换 SHEIN 平台的 JSON 解析代码
4. 逐步替换其他模块的 JSON 解析代码
5. 验证编译和测试

### 阶段二：HTTP 客户端统一
1. 替换直接创建 HTTP 客户端的代码
2. 验证功能正常

### 阶段三：错误处理优化
1. 分析 panic 恢复模式
2. 创建统一的辅助函数
3. 逐步替换

---

## 总结

**已完成:**
- 消除工具函数重复，减少约 80 行代码 ✅
- 统一 HTTP 客户端创建 ✅
- 创建 JSON 工具包和迁移指南 ✅

**进行中:**
- JSON 解析重复（高优先级）：已创建工具包，待迁移 60+ 处

**待处理:**
- panic 恢复模式重复（中优先级）：预计减少 100+ 行代码
- Context 超时创建重复（中优先级）：预计减少 50+ 行代码
- 时间格式化重复（低优先级）：预计减少 20+ 行代码

**总预期收益:**
- 已减少重复代码约 90 行
- 待减少重复代码约 370+ 行
- 总计可减少约 460+ 行重复代码
- 提高代码一致性和可维护性
- 降低 bug 风险

---

## 参考文档

- [重构进度报告](./refactoring-progress.md)
- [重构计划](./refactoring-plan.md)
