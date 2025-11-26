# 产品数据获取器迁移完成

## 迁移总结

已成功将TEMU和SHEIN的产品数据获取逻辑迁移到公共模块 `common/product/fetcher.go`。

## 修改的文件

### 1. 新增文件

- **`common/product/fetcher.go`** - 公共产品数据获取器
  - `ProductFetcher` - 核心获取器类
  - `FetchProduct()` - 统一获取方法
  - `ParseAmazonProduct()` - JSON解析
  - `GetAmazonDomainByRegion()` - 地区到域名映射
  - `GetZipcodeForRegion()` - 地区到邮编映射

### 2. 修改的文件

#### TEMU平台

- **`platforms/temu/handlers/raw_json_data_handler.go`**
  - ✅ 移除重复的获取逻辑（~150行代码）
  - ✅ 使用 `ProductFetcher` 替代
  - ✅ 简化 `Handle()` 方法
  - ✅ 移除 `shouldUseAmazonCrawler()`
  - ✅ 移除 `fetchFromAmazonCrawler()`
  - ✅ 移除 `saveToServer()`
  - ✅ 移除 `parseAmazonProduct()`
  - ✅ 移除 `getAmazonDomainByRegion()`
  - ✅ 移除 `getZipcodeForRegion()`

- **`platforms/temu/handlers/variant_json_data_handler.go`**
  - ✅ 使用公共函数 `product.GetAmazonDomainByRegion()`
  - ✅ 使用公共函数 `product.GetZipcodeForRegion()`
  - ✅ 移除重复的地区映射函数（~50行代码）

#### SHEIN平台

- **`platforms/shein/modules/raw_json_data_handler.go`**
  - ✅ 移除重复的获取逻辑（~200行代码）
  - ✅ 使用 `ProductFetcher` 替代
  - ✅ 添加适配器 `rawJsonDataClientAdapter`（SHEIN不需要保存数据）
  - ✅ 简化 `Handle()` 方法
  - ✅ 移除 `shouldUseAmazonCrawler()`
  - ✅ 移除 `fetchFromAmazonCrawler()`
  - ✅ 移除 `fetchFromAmazonCrawlerDirectly()`
  - ✅ 移除 `convertAmazonProduct()`
  - ✅ 移除 `ParseAmazonProduct()`（使用公共版本）
  - ✅ 移除 `GetAmazonDomainByRegion()`（使用公共版本）
  - ✅ 移除 `GetZipcodeForRegion()`（使用公共版本）
  - ✅ 移除 `getMapKeys()`

## 代码减少统计

| 平台 | 文件 | 删除行数 | 新增行数 | 净减少 |
|------|------|---------|---------|--------|
| TEMU | raw_json_data_handler.go | ~150 | ~20 | ~130 |
| TEMU | variant_json_data_handler.go | ~50 | ~5 | ~45 |
| SHEIN | raw_json_data_handler.go | ~200 | ~30 | ~170 |
| **总计** | | **~400** | **~55** | **~345** |

**净减少约 345 行重复代码！**

## 功能对比

### 迁移前

```go
// TEMU - 每个handler都有自己的实现
func (h *RawJsonDataHandlerV2) Handle(ctx *pipeline.TaskContext) error {
    // 1. 检查服务器数据
    rawJsonData, err := h.rawJsonDataClient.GetRawJsonData(req)
    if err == nil && rawJsonData != nil {
        amazonProduct, parseErr := h.parseAmazonProduct(rawJsonData.RawJSONData)
        if parseErr == nil {
            ctx.AmazonProduct = amazonProduct
            return nil
        }
    }
    
    // 2. 使用爬虫抓取
    if h.shouldUseAmazonCrawler(ctx) && h.amazonProcessor != nil {
        amazonProduct, err = h.fetchFromAmazonCrawler(ctx)
        if err != nil {
            return fmt.Errorf("amazon爬虫抓取失败: %w", err)
        }
    }
    
    ctx.AmazonProduct = amazonProduct
    return nil
}

// SHEIN - 类似的实现，但有细微差异
func (h *RawJsonDataHandler) Handle(ctx *TaskContext) error {
    // 类似的逻辑，但代码重复...
}
```

### 迁移后

```go
// TEMU - 使用公共ProductFetcher
func (h *RawJsonDataHandlerV2) Handle(ctx *pipeline.TaskContext) error {
    req := &product.FetchRequest{
        TenantID:   ctx.Task.TenantID,
        Platform:   ctx.Task.Platform,
        Region:     ctx.Task.Region,
        ProductID:  ctx.Task.ProductID,
        StoreID:    ctx.Task.StoreID,
        CategoryID: ctx.Task.CategoryID,
        Creator:    ctx.Task.Creator,
    }

    amazonProduct, err := h.fetcher.FetchProduct(req)
    if err != nil {
        return fmt.Errorf("获取产品数据失败: %w", err)
    }

    ctx.AmazonProduct = amazonProduct
    return nil
}

// SHEIN - 完全相同的使用方式
func (h *RawJsonDataHandler) Handle(ctx *TaskContext) error {
    req := &product.FetchRequest{
        TenantID:   ctx.Task.TenantID,
        Platform:   ctx.Task.Platform,
        Region:     ctx.Task.Region,
        ProductID:  ctx.Task.ProductID,
        StoreID:    ctx.Task.StoreID,
        CategoryID: ctx.Task.CategoryID,
        Creator:    ctx.Task.Creator,
    }

    amazonProduct, err := h.fetcher.FetchProduct(req)
    if err != nil {
        // SHEIN特定的错误处理
        if isProductNotFoundError(err) {
            return NewNonRetryableError("Amazon产品不存在", err)
        }
        return NewRetryableError("获取产品数据失败", err)
    }

    ctx.AmazonProduct = amazonProduct
    return nil
}
```

## 关键改进

### 1. 统一的日志格式

所有产品获取操作现在使用统一的emoji日志：

```
🔍 开始获取产品数据: ProductID=B001, Platform=amazon, Region=US
✅ 服务器有历史数据: ProductID=B001, 数据长度=1234
✅ 成功解析服务器数据: ProductID=B001, Title=Product Name
🌐 服务器无数据，使用Amazon爬虫抓取: ProductID=B001
🚀 开始爬取: URL=https://www.amazon.com/dp/B001, Zipcode=10001
✅ 爬取成功: ProductID=B001, Title=Product Name
💾 保存到服务器: ProductID=B001
✅ 保存成功: ProductID=B001, ID=123
```

### 2. 统一的地区映射

所有平台使用相同的地区到域名/邮编映射：

```go
// 支持的地区
US, USA, United States -> amazon.com, 10001
UK, GB, United Kingdom -> amazon.co.uk, SW1A 1AA
DE, Germany -> amazon.de, 10115
FR, France -> amazon.fr, 75001
IT, Italy -> amazon.it, 00118
ES, Spain -> amazon.es, 28001
CA, Canada -> amazon.ca, M5H 2N2
JP, Japan -> amazon.co.jp, 100-0001
AU, Australia -> amazon.com.au, 2000
```

### 3. 配置优先级

邮编获取支持配置优先级：

```go
// 1. 优先使用配置文件中的邮编
if configZipcodes != nil {
    if zipcode, exists := configZipcodes[region]; exists {
        return zipcode
    }
}

// 2. 使用默认邮编映射
return defaultZipcode
```

### 4. SHEIN适配器

为SHEIN创建了适配器，因为SHEIN不需要保存数据到服务器：

```go
type rawJsonDataClientAdapter struct {
    client interface {
        GetRawJsonData(req *api.RawJsonDataReqDTO) (*api.RawJsonDataRespDTO, error)
        ConfirmProductVariants(req *api.ProductVariantConfirmationReqDTO) (bool, error)
    }
}

func (a *rawJsonDataClientAdapter) CreateRawJsonData(req *api.RawJsonDataCreateReqDTO) (int64, error) {
    // SHEIN不需要保存数据到服务器
    logrus.Debug("[SHEIN] CreateRawJsonData 被调用，但SHEIN不需要保存数据")
    return 0, nil
}
```

## 测试建议

### 1. TEMU测试

```bash
# 测试主产品获取
curl -X POST http://localhost:8081/api/test/temu/product \
  -H "Content-Type: application/json" \
  -d '{"productId": "B001", "region": "US"}'

# 测试变体获取
curl -X POST http://localhost:8081/api/test/temu/variants \
  -H "Content-Type: application/json" \
  -d '{"productId": "B001", "region": "US"}'
```

### 2. SHEIN测试

```bash
# 测试主产品获取
curl -X POST http://localhost:8080/api/test/shein/product \
  -H "Content-Type: application/json" \
  -d '{"productId": "B001", "region": "US"}'
```

### 3. 日志检查

查看日志确认使用了公共模块：

```bash
# 应该看到这些日志
grep "🔍 开始获取产品数据" logs/app.log
grep "✅ 服务器有历史数据" logs/app.log
grep "🌐 服务器无数据，使用Amazon爬虫抓取" logs/app.log
grep "✅ 爬取成功" logs/app.log
```

## 后续优化建议

1. **添加缓存层** - 在ProductFetcher中添加内存缓存
2. **批量获取** - 支持批量获取多个产品
3. **重试机制** - 内置重试逻辑
4. **指标收集** - 收集获取成功率、耗时等指标
5. **单元测试** - 为ProductFetcher添加完整的单元测试

## 总结

✅ 成功消除了 ~345 行重复代码
✅ 统一了TEMU和SHEIN的产品获取逻辑
✅ 提供了统一的日志格式
✅ 支持配置优先级
✅ 易于测试和维护
✅ 为未来扩展打下基础
