# 不可重试错误处理

## 概述

某些TEMU API错误是不可重试的，即使重试也无法成功。系统会自动识别这些错误并标记任务为失败，避免无意义的重试。

## 不可重试错误类型

### 1. SKU重复错误

**错误码：** `10000103`

**错误消息示例：**
```
Contribution SKU duplicated with another product (Goods ID: 603076614269339)
```

**原因：**
- 产品的SKU编码（out_sku_sn）与平台上已存在的其他产品重复
- TEMU平台要求每个SKU编码必须唯一

**处理方式：**
- 系统识别到此错误后，会立即标记任务为失败
- 不会尝试保存到草稿箱
- 不会进行重试
- 返回错误消息：`NONRETRYABLE: 产品提交失败(error_code=10000103): ...`

**解决方案：**
- 检查SKU生成策略配置
- 确保SKU编码的唯一性
- 可以修改店铺配置中的前缀/后缀来避免重复

### 2. 商品已存在

**错误码：** `10000104`, `10000105`

**原因：**
- 商品ID或其他唯一标识符与已存在的商品冲突

**处理方式：**
- 同SKU重复错误的处理方式

## 识别机制

系统通过两种方式识别不可重试错误：

### 1. 错误码匹配

```go
nonRetryableErrorCodes := map[int]string{
    10000103: "SKU重复错误",
    10000104: "商品已存在",
    10000105: "商品ID重复",
}
```

### 2. 错误消息关键词匹配

```go
nonRetryableKeywords := []string{
    "duplicated",
    "duplicate", 
    "already exists",
    "重复",
    "已存在",
}
```

如果错误消息中包含这些关键词，也会被识别为不可重试错误。

## 日志输出

当检测到不可重试错误时，会输出以下日志：

```
ERROR TEMU API响应失败: success=false, error_code=10000103, error_message=Contribution SKU duplicated...
ERROR 完整响应: {...}
ERROR ❌ 检测到不可重试错误(error_code=10000103): Contribution SKU duplicated...
ERROR ❌ 此错误无法通过重试解决，任务将被标记为失败
```

## 错误返回格式

不可重试错误会返回特殊格式的错误消息：

```
NONRETRYABLE: 产品提交失败(error_code=10000103): Contribution SKU duplicated with another product
```

错误消息以 `NONRETRYABLE:` 开头，上层系统可以通过检查这个前缀来识别不可重试错误。

## 与可重试错误的区别

### 可重试错误
- 网络超时
- 临时服务不可用
- 限流错误
- 其他临时性错误

**处理方式：**
1. 尝试保存到草稿箱
2. 返回错误，允许重试
3. 可能在下次重试时成功

### 不可重试错误
- SKU重复
- 商品已存在
- 数据验证失败（永久性）
- 权限不足

**处理方式：**
1. 立即标记为失败
2. 不保存到草稿箱
3. 不允许重试
4. 需要人工介入或修改配置

## 扩展

如果需要添加新的不可重试错误类型，修改 `product_submit_handler.go` 中的 `isNonRetryableError` 方法：

```go
func (h *ProductSubmitHandler) isNonRetryableError(errorCode int, errorMessage string) bool {
    // 添加新的错误码
    nonRetryableErrorCodes := map[int]string{
        10000103: "SKU重复错误",
        10000XXX: "新的错误类型", // 添加这里
    }
    
    // 添加新的关键词
    nonRetryableKeywords := []string{
        "duplicated",
        "新的关键词", // 添加这里
    }
    
    // ...
}
```

## 最佳实践

1. **监控不可重试错误**
   - 定期检查日志中的不可重试错误
   - 分析错误原因，优化配置

2. **SKU唯一性**
   - 使用合适的SKU生成策略
   - 添加唯一的前缀/后缀
   - 考虑使用时间戳或随机数

3. **错误处理**
   - 不要对不可重试错误进行重试
   - 及时通知相关人员
   - 记录错误详情供后续分析

## 相关文件

- `platforms/temu/handlers/product_submit_handler.go` - 错误识别和处理逻辑
- `common/worker/pool.go` - 任务重试逻辑（需要识别NONRETRYABLE前缀）
