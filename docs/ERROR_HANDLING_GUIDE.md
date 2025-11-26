# 错误处理指南

## 标准错误定义

### TEMU 平台标准错误

```go
// platforms/temu/errors.go
var (
    ErrProductNotFound     = errors.New("产品不存在")
    ErrProductOffline      = errors.New("产品已下架")
    ErrAuthExpired         = errors.New("认证已过期")
    ErrTooManyVariants     = errors.New("变体数量过多")
    ErrInvalidASIN         = errors.New("ASIN无效")
    ErrDuplicateProduct    = errors.New("产品重复")
    ErrPageNotFound        = errors.New("页面不存在")
    ErrMissingPageElements = errors.New("页面缺少必要元素")
)
```

## 错误类型

### 1. 可重试错误 (RetryableError)

用于临时性错误，系统会自动重试：

```go
// 创建可重试错误
err := NewRetryableError("网络连接失败", originalErr)

// 判断是否可重试
if IsRetryableError(err) {
    // 重试逻辑
}
```

**适用场景**：
- 网络超时
- 连接被拒绝
- 临时服务不可用
- 速率限制

### 2. 不可重试错误 (NonRetryableError)

用于永久性错误，不会重试：

```go
// 创建不可重试错误
err := NewNonRetryableError("产品不存在", originalErr)

// 或使用标准错误
err := fmt.Errorf("查询产品失败: %w", ErrProductNotFound)
```

**适用场景**：
- 产品不存在
- 数据验证失败
- 权限不足
- 重复提交

### 3. 认证过期错误

特殊处理，任务会被暂停：

```go
// 检查认证过期
if IsAuthExpiredError(err) {
    // 暂停任务，等待Cookie更新
}
```

## 使用标准错误

### ✅ 推荐做法

```go
// 1. 使用标准错误
if err != nil {
    return fmt.Errorf("获取产品失败: %w", ErrProductNotFound)
}

// 2. 使用 errors.Is 判断
if errors.Is(err, ErrProductNotFound) {
    // 处理产品不存在的情况
}

// 3. 使用 errors.As 获取错误详情
var nonRetryableErr *NonRetryableError
if errors.As(err, &nonRetryableErr) {
    // 处理不可重试错误
}
```

### ❌ 不推荐做法

```go
// 不要使用字符串比较
if err.Error() == "产品不存在" {  // ❌
    // ...
}

// 不要使用字符串包含判断
if strings.Contains(err.Error(), "not found") {  // ❌
    // ...
}
```

## Handler 中的错误处理

### 返回标准错误

```go
func (h *SomeHandler) Handle(ctx *pipeline.TaskContext) error {
    // 检查 Context 是否取消
    select {
    case <-ctx.Context.Done():
        return fmt.Errorf("操作被取消: %w", ctx.Context.Err())
    default:
    }
    
    // 业务逻辑
    product, err := h.getProduct(productID)
    if err != nil {
        // 包装标准错误
        return fmt.Errorf("获取产品失败: %w", ErrProductNotFound)
    }
    
    return nil
}
```

### 创建自定义错误

```go
// 不可重试错误
if isDuplicate {
    return NewNonRetryableError("SKU重复", nil)
}

// 可重试错误
if isTimeout {
    return NewRetryableError("请求超时", err)
}
```

## 错误判断流程

```
错误发生
    ↓
是否为认证过期？ → 是 → 暂停任务
    ↓ 否
是否为标准不可重试错误？ → 是 → 终止任务
    ↓ 否
是否为 NonRetryableError？ → 是 → 终止任务
    ↓ 否
是否为 RetryableError？ → 是 → 重试任务
    ↓ 否
根据错误内容判断 → 可重试/不可重试
```

## 错误日志规范

### 日志级别

```go
// Debug: 详细的错误分析
h.logger.Debugf("错误分析: 类型=%T, 可重试=%t", err, isRetryable)

// Info: 关键状态变更
h.logger.Infof("任务处理完成: ID=%s", taskID)

// Warn: 警告信息（可恢复）
h.logger.Warnf("⏸️ 任务因认证过期而暂停: ID=%s", taskID)

// Error: 错误信息（需要关注）
h.logger.Errorf("❌ 任务处理失败: ID=%s, Error=%v", taskID, err)
```

### 错误上下文

```go
// ✅ 包含足够的上下文信息
return fmt.Errorf("获取产品 %s 失败: %w", productID, err)

// ❌ 缺少上下文
return fmt.Errorf("获取失败: %w", err)
```

## 最佳实践

1. **优先使用标准错误**
   - 定义常见错误为包级变量
   - 使用 `errors.Is` 和 `errors.As` 判断

2. **错误要包含上下文**
   - 使用 `fmt.Errorf` 包装错误
   - 添加必要的参数信息

3. **明确错误类型**
   - 可重试 vs 不可重试
   - 临时性 vs 永久性

4. **合理的日志级别**
   - Debug: 调试信息
   - Info: 关键操作
   - Warn: 可恢复的问题
   - Error: 需要关注的错误

5. **避免 panic**
   - 使用 defer + recover 保护
   - 返回错误而不是 panic

## 示例代码

### 完整的 Handler 示例

```go
type ProductFetchHandler struct {
    client ProductClient
    logger *logrus.Entry
}

func (h *ProductFetchHandler) Handle(ctx *pipeline.TaskContext) error {
    // 1. 检查 Context
    select {
    case <-ctx.Context.Done():
        return fmt.Errorf("操作被取消: %w", ctx.Context.Err())
    default:
    }
    
    // 2. 获取参数
    productID := ctx.GetProductID()
    
    // 3. 执行业务逻辑
    product, err := h.client.GetProduct(productID)
    if err != nil {
        // 判断错误类型
        if isNotFound(err) {
            return fmt.Errorf("产品 %s 不存在: %w", productID, ErrProductNotFound)
        }
        if isTimeout(err) {
            return NewRetryableError(fmt.Sprintf("获取产品 %s 超时", productID), err)
        }
        return fmt.Errorf("获取产品 %s 失败: %w", productID, err)
    }
    
    // 4. 保存结果
    ctx.SetData("product", product)
    
    h.logger.Debugf("产品获取成功: %s", productID)
    return nil
}
```
