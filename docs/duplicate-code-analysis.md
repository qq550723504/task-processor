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
if r := recover(); r != nil {
    return fmt.Errorf("处理时发生panic: %v", r)
}
```

**统计数据:**
- 约 40+ 处类似的 panic 恢复代码
- 约 100+ 处错误包装代码

**建议解决方案:**
1. 创建统一的 panic 恢复辅助函数
2. 考虑使用 `errors.Wrap()` 替代 `fmt.Errorf()`

**预期收益:**
- 统一错误处理模式
- 减少样板代码

**优先级:** 中
**预计工作量:** 3-4 小时

---

### 5. 日志初始化模式 🟢 低优先级

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
2. 🔴 JSON 解析重复 - 待处理

### 中优先级（建议近期处理）
3. 🟡 HTTP 客户端创建重复
4. 🟡 错误包装模式重复

### 低优先级（可选优化）
5. 🟢 考虑提取更多的业务逻辑公共代码

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
- 消除工具函数重复，减少约 80 行代码

**待处理:**
- JSON 解析重复（高优先级）：预计减少 200+ 行代码
- HTTP 客户端重复（中优先级）：预计减少 10+ 行代码
- 错误处理重复（中优先级）：预计减少 50+ 行代码

**总预期收益:**
- 减少重复代码约 340+ 行
- 提高代码一致性和可维护性
- 降低 bug 风险

---

## 参考文档

- [重构进度报告](./refactoring-progress.md)
- [重构计划](./refactoring-plan.md)
