# 产品不存在错误不应重试问题修复

## 问题描述

从日志中发现，当遇到"产品页面缺少必要元素"这类明确的产品不存在错误时，系统仍然将其标记为 `pending_retry`（可重试），导致无效重试。

### 错误日志示例

```
❌ 产品不存在或无法访问，标记为不可重试: Amazon爬虫抓取失败: 抓取失败: ❌ 产品页面缺少必要元素 
(URL: https://www.amazon.com/dp/B084KP3NG6?th=1&psc=1&language=en_US, Title: Amazon.com: Amazon Secured Card)

错误类型: *fmt.wrapError, 错误值: 处理器 原始JSON数据处理器V2 执行失败: 产品不存在或无法访问: 
❌ 产品页面缺少必要元素, 是否可重试: true

任务状态同步更新成功: TaskID=2051937, Status=pending_retry
```

## 问题根因

1. **handlers 包中定义了独立的 `NonRetryableError`**，但 `platforms/temu/errors.go` 中的 `IsRetryableError` 函数无法识别这个类型
2. **错误内容判断不完整**：`isRetryableByContent` 函数缺少对"产品页面缺少必要元素"等关键词的判断

## 修复方案

### 1. 增强错误类型识别

在 `platforms/temu/errors.go` 中添加 `isHandlersNonRetryableError` 函数，通过类型名称判断是否为 handlers 包中的 `NonRetryableError`：

```go
// isHandlersNonRetryableError 检查是否为handlers包中的NonRetryableError
func isHandlersNonRetryableError(err error) bool {
	if err == nil {
		return false
	}
	
	typeName := fmt.Sprintf("%T", err)
	return typeName == "*handlers.NonRetryableError"
}
```

### 2. 完善错误内容判断

在 `isRetryableByContent` 函数的 `validationErrors` 列表中添加产品不存在相关的关键词：

```go
validationErrors := []string{
	// ... 原有错误 ...
	"产品不存在",
	"产品页面不存在",
	"产品页面缺少必要元素",
	"page not found",
	"asin无效",
	"产品已下架",
}
```

## 修改文件

1. `platforms/temu/errors.go`
   - 添加 `isHandlersNonRetryableError` 函数
   - 在 `IsRetryableError` 中调用该函数
   - 增强 `isRetryableByContent` 的错误关键词列表

2. `platforms/temu/errors_test.go`（新增）
   - 添加完整的单元测试
   - 验证各种产品不存在错误场景

## 测试结果

所有测试用例通过：

```
=== RUN   TestIsRetryableError_ProductNotFound
    --- PASS: TestIsRetryableError_ProductNotFound/产品不存在错误_-_不可重试
    --- PASS: TestIsRetryableError_ProductNotFound/产品页面缺少必要元素_-_不可重试
    --- PASS: TestIsRetryableError_ProductNotFound/产品页面不存在_-_不可重试
    --- PASS: TestIsRetryableError_ProductNotFound/Page_Not_Found_-_不可重试
    --- PASS: TestIsRetryableError_ProductNotFound/网络超时_-_可重试
    --- PASS: TestIsRetryableError_ProductNotFound/NonRetryableError类型_-_不可重试
    --- PASS: TestIsRetryableError_ProductNotFound/RetryableError类型_-_可重试
PASS
```

## 预期效果

修复后，当遇到以下错误时，任务将被标记为 `terminated`（终止）而不是 `pending_retry`（等待重试）：

- ❌ 产品页面缺少必要元素
- 产品不存在
- 产品页面不存在
- Page Not Found
- ASIN无效
- 产品已下架
- 变体数量过多
- 变体ASIN数量过多

这将避免无效重试，提高系统效率。

## 额外修复

### 1. TaskContext Data Map 未初始化导致 Panic

**问题**：在 `price_handler.go` 的 `CalculateVariantPrice` 方法中，直接创建 `TaskContext` 结构体而没有初始化 `Data` map，导致后续调用 `SetData` 时 panic。

**修复**：在创建 `TaskContext` 时初始化 `Data` map：

```go
tempCtx := &pipeline.TaskContext{
    Task:          ctx.Task,
    AmazonProduct: variant,
    StoreInfo:     ctx.StoreInfo,
    Data:          make(map[string]interface{}), // 初始化Data map
}
```

### 2. 变体数量限制检查

在 `product_exists_check_handler.go` 中添加变体数量限制（500个），超过限制时返回不可重试错误。

## 日期

2025-11-24
