# 通用工具包 (Common Utilities)

本目录包含项目中通用的工具函数包，用于消除代码重复，提高代码质量。

## 包列表

### mathutil - 数学工具包
提供常用的数学计算函数。

**函数列表：**
- `Abs(float64) float64` - 浮点数绝对值
- `AbsInt(int) int` - 整数绝对值
- `AbsInt64(int64) int64` - int64绝对值
- `Min(int, int) int` - 返回较小值
- `Max(int, int) int` - 返回较大值
- `MinInt64(int64, int64) int64` - int64最小值
- `MaxInt64(int64, int64) int64` - int64最大值

**使用示例：**
```go
import "task-processor/internal/pkg/mathutil"

// 计算绝对值
result := mathutil.Abs(-5.5) // 返回 5.5

// 获取最小值
min := mathutil.Min(10, 20) // 返回 10
```

### ptrutil - 指针工具包
提供各种类型的指针转换函数。

**函数列表：**
- `IntPtr(int) *int` - int指针
- `Int16Ptr(int16) *int16` - int16指针
- `Int32Ptr(int32) *int32` - int32指针
- `Int64Ptr(int64) *int64` - int64指针
- `StringPtr(string) *string` - string指针
- `Float32Ptr(float32) *float32` - float32指针
- `Float64Ptr(float64) *float64` - float64指针
- `BoolPtr(bool) *bool` - bool指针

**使用示例：**
```go
import "task-processor/internal/pkg/ptrutil"

// 创建指针
name := ptrutil.StringPtr("test")
age := ptrutil.IntPtr(25)
price := ptrutil.Float64Ptr(99.99)
```

### strutil - 字符串工具包
提供常用的字符串处理函数。

**函数列表：**
- `ContainsIgnoreCase(string, string) bool` - 忽略大小写包含检查
- `FindSubstring(string, string) bool` - 查找子字符串
- `TruncateString(string, int) string` - 截断字符串（支持UTF-8）
- `CleanWhitespace(string) string` - 清理多余空白
- `Contains(string, string) bool` - 包含检查
- `ToLower(string) string` - 转小写
- `ToUpper(string) string` - 转大写

**使用示例：**
```go
import "task-processor/internal/pkg/strutil"

// 忽略大小写查找
found := strutil.ContainsIgnoreCase("Hello World", "hello") // 返回 true

// 截断字符串（正确处理中文）
short := strutil.TruncateString("这是一个测试", 3) // 返回 "这是一"

// 清理空白
clean := strutil.CleanWhitespace("  hello   world  ") // 返回 "hello world"
```

## 设计原则

1. **单一职责**：每个包只负责一类功能
2. **简单易用**：函数命名清晰，参数简单
3. **完整测试**：每个函数都有单元测试
4. **性能优先**：避免不必要的内存分配
5. **UTF-8支持**：正确处理多字节字符

## 测试

每个包都有完整的单元测试，运行测试：

```bash
# 测试所有工具包
go test ./internal/pkg/... -v

# 测试单个包
go test ./internal/pkg/mathutil -v
go test ./internal/pkg/ptrutil -v
go test ./internal/pkg/strutil -v
```

## 贡献指南

### 添加新函数
1. 在相应的包中添加函数实现
2. 添加完整的文档注释
3. 编写单元测试
4. 确保测试通过
5. 更新本README文档

### 命名规范
- 函数名使用大驼峰（PascalCase）
- 参数名使用小驼峰（camelCase）
- 布尔返回值函数以Is/Has/Can开头

### 测试规范
- 测试函数名：Test + 函数名
- 覆盖正常情况和边界情况
- 使用表驱动测试（table-driven tests）

## 迁移指南

如果你在代码中发现重复的工具函数，可以按以下步骤迁移：

1. **检查是否已存在**：先查看工具包是否已有类似函数
2. **添加到工具包**：如果没有，添加到合适的工具包
3. **编写测试**：确保新函数有完整测试
4. **渐进迁移**：保留原函数作为包装器，标记为废弃
5. **更新引用**：逐步更新代码使用新函数
6. **清理废弃**：在后续版本中移除废弃函数

## 版本历史

### v1.0.0 (2026-03-06)
- 初始版本
- 添加 mathutil 包
- 添加 ptrutil 包
- 添加 strutil 包
- 完成第一阶段工具函数统一

## 相关文档

- [重复代码检测报告](../../docs/重复代码检测报告.md)
- [第一阶段完成总结](../../docs/第一阶段完成总结.md)
- [第一阶段工具函数统一进度](../../docs/第一阶段工具函数统一进度.md)
