# 数据缓存逻辑重构

## 概述

将 Amazon 爬虫抓取的数据缓存逻辑从各平台特定代码提取到公共模块，实现统一的缓存管理。

## 改进内容

### 1. 公共缓存逻辑 (`common/product/fetcher.go`)

新增两个公共方法：

- **`CacheProduct()`** - 缓存单个产品数据到服务器
  - 自动检查服务器是否已有缓存
  - 避免重复保存
  - 失败不影响主流程

- **`CacheVariants()`** - 批量缓存变体数据到服务器
  - 支持批量处理多个变体
  - 自动跳过已缓存的变体
  - 统计成功/失败/跳过数量

### 2. Temu 平台

#### 新增处理器

- `platforms/temu/handlers/cache_product_handler.go` - 缓存产品数据处理器
- `platforms/temu/handlers/cache_variants_handler.go` - 缓存变体数据处理器

#### Pipeline 更新

在 `platforms/temu/pipeline.go` 中添加缓存步骤：

```go
// 4. 获取原始JSON数据
AddHandler(handlers.NewRawJsonDataHandlerV2(...))
// 5. 缓存产品数据到服务器 ✨ 新增
AddHandler(handlers.NewCacheProductHandler(...))

// 8. 获取变体JSON数据
AddHandler(handlers.NewVariantJsonDataHandler(...))
// 9. 缓存变体数据到服务器 ✨ 新增
AddHandler(handlers.NewCacheVariantsHandler(...))
```

### 3. Shein 平台

#### 处理器重构

更新 `platforms/shein/modules/submit_raw_json_data_handler.go`：

- `SubmitRawJsonDataHandler` - 使用公共 `CacheProduct()` 方法
- `SubmitVariantRawJsonDataHandler` - 使用公共 `CacheVariants()` 方法

#### Pipeline 更新

在 `platforms/shein/pipeline.go` 中传入必要参数：

```go
// 提交原始JSON数据到服务器缓存（使用公共缓存逻辑）
pipeline.AddHandler(modules.NewSubmitRawJsonDataHandler(
    processor.managementClientMgr.GetRawJsonDataClient(), 
    &cfg.Amazon, 
    processor.amazonProcessor
))

// 提交变体原始JSON数据到服务器缓存（使用公共缓存逻辑）
pipeline.AddHandler(modules.NewSubmitVariantRawJsonDataHandler(
    processor.managementClientMgr.GetRawJsonDataClient(), 
    &cfg.Amazon, 
    processor.amazonProcessor
))
```

## 优势

### 1. 代码复用
- 缓存逻辑统一在 `ProductFetcher` 中
- 避免各平台重复实现相同功能
- 减少代码维护成本

### 2. 一致性
- 所有平台使用相同的缓存策略
- 统一的日志格式和错误处理
- 统一的重复检查逻辑

### 3. 可维护性
- 缓存逻辑集中管理
- 修改一处，所有平台受益
- 更容易添加新功能（如缓存过期、更新策略等）

### 4. 容错性
- 缓存失败不影响主流程
- 自动跳过已缓存的数据
- 详细的日志记录便于排查问题

## 缓存流程

```
1. 获取产品数据（从API或爬虫）
   ↓
2. 检查服务器是否已有缓存
   ↓
3. 如果没有缓存，保存到服务器
   ↓
4. 记录缓存结果（成功/失败/跳过）
```

## 日志示例

```
💾 开始缓存产品数据到服务器: ProductID=B08XYZ123
⏭️ 服务器已有产品数据缓存，跳过: ProductID=B08XYZ123
✅ 产品数据已缓存: ProductID=B08XYZ123

💾 开始批量缓存变体数据到服务器: 数量=5
⏭️ 服务器已有变体数据缓存，跳过: ASIN=B08ABC456
💾 保存到服务器: ProductID=B08DEF789
✅ 保存成功: ProductID=B08DEF789, ID=12345
✅ 变体数据缓存完成: 成功=3, 失败=0, 跳过=2, 总数=5
```

## 注意事项

1. **缓存失败不影响主流程** - 缓存操作失败只记录警告，不会导致任务失败
2. **自动去重** - 在保存前会检查服务器是否已有数据，避免重复保存
3. **批量处理** - 变体数据支持批量缓存，提高效率
4. **类型适配** - 自动处理不同平台的数据类型差异（Temu 使用 `[]*amazon.Product`，Shein 使用 `[]amazon.Product`）

## 未来改进方向

1. 添加缓存过期机制
2. 支持缓存更新策略
3. 添加缓存统计和监控
4. 支持缓存预热
