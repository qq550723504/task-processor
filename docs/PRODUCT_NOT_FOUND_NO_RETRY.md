# 产品不存在错误不重试优化

## 问题描述

当Amazon产品不存在时（如ASIN无效、产品已下架），系统会：
1. 检测到 "Page Not Found"
2. 返回 `ProductNotFoundError`
3. 但错误被包装为 `RetryableError`（可重试错误）
4. 任务被重新入队，反复重试

**日志示例：**
```
INFO 错误类型: *modules.retryableError, 错误值: Amazon爬虫抓取失败: 抓取Amazon产品失败: 产品页面缺少必要元素 (URL: https://www.amazon.com/dp/B00BEXTWP33?th=1&psc=1&language=en_US, Title: Page Not Found), 是否可重试: true
```

**问题：**
- 产品确实不存在，重试也不会成功
- 浪费系统资源和时间
- 任务队列被无效任务占用

## 解决方案

### 修改文件：`platforms/shein/modules/raw_json_data_handler.go`

#### 1. 新增 `isProductNotFoundError` 函数

检查错误是否为产品不存在类型：

```go
func isProductNotFoundError(err error) bool {
    // 检查是否为 ProductNotFoundError 类型
    if _, ok := err.(*amazon.ProductNotFoundError); ok {
        return true
    }

    // 检查错误信息中是否包含产品不存在的关键词
    errorStr := err.Error()
    productNotFoundPatterns := []string{
        "产品页面不存在",
        "产品页面缺少必要元素",
        "Page Not Found",
        "产品不存在",
    }

    for _, pattern := range productNotFoundPatterns {
        if strings.Contains(errorStr, pattern) {
            return true
        }
    }

    return false
}
```

#### 2. 修改 `Handle` 方法的错误处理

**修改前：**
```go
amazonProduct, err = h.fetchFromAmazonCrawlerDirectly(ctx)
if err != nil {
    return NewRetryableError("Amazon爬虫抓取失败", err)
}
```

**修改后：**
```go
amazonProduct, err = h.fetchFromAmazonCrawlerDirectly(ctx)
if err != nil {
    // 检查是否为产品不存在错误
    if isProductNotFoundError(err) {
        logrus.Warnf("产品不存在，不需要重试: ProductID=%s, Error=%v", ctx.Task.ProductID, err)
        return NewNonRetryableError("Amazon产品不存在", err)
    }
    // 其他错误（如超时、网络错误）可以重试
    return NewRetryableError("Amazon爬虫抓取失败", err)
}
```

## 错误分类

### 不可重试错误（NonRetryableError）
- 产品不存在（Page Not Found）
- 产品页面缺少必要元素
- ASIN无效
- 产品已下架

### 可重试错误（RetryableError）
- 网络超时
- 浏览器崩溃
- 页面加载失败
- 风控验证码
- 临时性服务器错误（502, 503, 504）

## 效果对比

### 修改前
```
INFO[14:26:25] 检测到产品不存在: Page Not Found
ERRO[14:26:25] 任务处理失败: Amazon爬虫抓取失败
INFO[14:26:25] 错误类型: *modules.retryableError, 是否可重试: true
WARN[14:26:25] 任务重新入队，等待重试
... (反复重试)
```

### 修改后
```
INFO[14:26:25] 检测到产品不存在: Page Not Found
WARN[14:26:25] 产品不存在，不需要重试: ProductID=B00BEXTWP33
ERRO[14:26:25] 任务处理失败: Amazon产品不存在
INFO[14:26:25] 错误类型: *modules.nonRetryableError, 是否可重试: false
INFO[14:26:25] 任务标记为失败，不再重试 ✅
```

## 优势

1. **减少无效重试** - 产品不存在的任务不会反复重试
2. **节省资源** - 不浪费浏览器实例和网络带宽
3. **提高效率** - 任务队列不被无效任务占用
4. **快速失败** - 立即标记失败，用户可以及时处理

## 相关文件

- `common/amazon/models.go` - `ProductNotFoundError` 定义
- `common/amazon/processor.go` - `checkProductExists` 产品存在性检查
- `platforms/shein/modules/raw_json_data_handler.go` - 错误分类处理

## 注意事项

1. 确保 `checkProductExists` 方法准确识别产品不存在的情况
2. 避免将临时性错误误判为产品不存在
3. 如果产品确实存在但被误判，可以手动重试或调整检测逻辑
