# 产品数据获取器重构

## 概述

将TEMU和SHEIN中重复的产品数据获取逻辑提取到公共模块 `common/product/fetcher.go`。

## 问题

TEMU和SHEIN的 `RawJsonDataHandler` 中有大量重复代码：
- 从管理系统API获取产品数据
- 从Amazon爬虫抓取产品数据
- 解析JSON数据
- 保存数据到服务器
- 地区到Amazon域名/邮编的映射

## 解决方案

创建统一的 `ProductFetcher`，提供以下功能：

### 1. 核心功能

```go
// 创建获取器
fetcher := product.NewProductFetcher(
    rawJsonDataClient,
    amazonConfig,
    amazonProcessor,
)

// 获取产品数据
req := &product.FetchRequest{
    TenantID:   task.TenantID,
    Platform:   task.Platform,
    Region:     task.Region,
    ProductID:  task.ProductID,
    StoreID:    task.StoreID,
    CategoryID: task.CategoryID,
    Creator:    task.Creator,
}

amazonProduct, err := fetcher.FetchProduct(req)
```

### 2. 工作流程

```
1. 检查服务器是否有历史数据
   ↓ 有
2. 解析并返回
   ↓ 没有
3. 判断是否为Amazon平台
   ↓ 是
4. 使用Amazon爬虫抓取
   ↓
5. 保存到服务器
   ↓
6. 返回产品数据
```

### 3. 公共工具函数

- `ParseAmazonProduct(jsonData string)` - 解析JSON数据
- `GetAmazonDomainByRegion(region string)` - 获取Amazon域名
- `GetZipcodeForRegion(region, configZipcodes)` - 获取邮编

## 使用示例

### TEMU Handler

```go
package handlers

import (
    "task-processor/common/product"
    "task-processor/common/pipeline"
)

type RawJsonDataHandler struct {
    fetcher *product.ProductFetcher
}

func NewRawJsonDataHandler(
    rawJsonDataClient api.RawJsonDataAPI,
    amazonConfig *config.AmazonConfig,
    amazonProcessor *amazon.AmazonProcessor,
) *RawJsonDataHandler {
    return &RawJsonDataHandler{
        fetcher: product.NewProductFetcher(
            rawJsonDataClient,
            amazonConfig,
            amazonProcessor,
        ),
    }
}

func (h *RawJsonDataHandler) Handle(ctx *pipeline.TaskContext) error {
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
        return err
    }

    ctx.AmazonProduct = amazonProduct
    return nil
}
```

### SHEIN Handler

```go
// 完全相同的使用方式
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
        return err
    }

    ctx.AmazonProduct = amazonProduct
    return nil
}
```

## 优势

1. **代码复用** - 消除TEMU和SHEIN之间的重复代码
2. **统一逻辑** - 产品获取逻辑在一个地方维护
3. **易于测试** - 公共模块更容易编写单元测试
4. **易于扩展** - 添加新平台时可以直接使用
5. **统一日志** - 所有产品获取操作使用统一的日志格式

## 迁移步骤

### 1. TEMU迁移

修改 `platforms/temu/handlers/raw_json_data_handler.go`:

```go
// 替换原有的实现
type RawJsonDataHandlerV2 struct {
    logger  *logrus.Entry
    fetcher *product.ProductFetcher
}

func NewRawJsonDataHandlerV2(...) *RawJsonDataHandlerV2 {
    return &RawJsonDataHandlerV2{
        logger:  logrus.WithField("handler", "RawJsonDataHandlerV2"),
        fetcher: product.NewProductFetcher(rawJsonDataClient, amazonConfig, amazonProcessor),
    }
}

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
        return err
    }

    ctx.AmazonProduct = amazonProduct
    return nil
}
```

### 2. SHEIN迁移

修改 `platforms/shein/modules/raw_json_data_handler.go`:

```go
// 替换原有的实现
type RawJsonDataHandler struct {
    fetcher *product.ProductFetcher
}

func NewRawJsonDataHandler(...) *RawJsonDataHandler {
    return &RawJsonDataHandler{
        fetcher: product.NewProductFetcher(rawJsonDataClient, amazonConfig, amazonProcessor),
    }
}

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
        return err
    }

    ctx.AmazonProduct = amazonProduct
    return nil
}
```

### 3. 变体处理器迁移

`VariantJsonDataHandler` 也可以使用相同的 `ProductFetcher`:

```go
// 批量获取变体
for _, asin := range variantAsins {
    req := &product.FetchRequest{
        TenantID:   ctx.Task.TenantID,
        Platform:   ctx.Task.Platform,
        Region:     ctx.Task.Region,
        ProductID:  asin,
        StoreID:    ctx.Task.StoreID,
        CategoryID: ctx.Task.CategoryID,
        Creator:    ctx.Task.Creator,
    }

    variant, err := h.fetcher.FetchProduct(req)
    if err != nil {
        logrus.Warnf("获取变体失败: ASIN=%s, Error=%v", asin, err)
        continue
    }

    variants = append(variants, variant)
}
```

## 注意事项

1. **接口兼容性** - `RawJsonDataClient` 接口需要实现 `GetRawJsonData` 和 `CreateRawJsonData` 方法
2. **错误处理** - 调用方需要根据错误类型决定是否重试
3. **日志格式** - 使用统一的emoji日志格式便于追踪
4. **配置优先级** - 邮编优先使用配置文件中的值，然后使用默认值

## 测试

```go
func TestProductFetcher(t *testing.T) {
    // Mock clients
    mockClient := &MockRawJsonDataClient{}
    mockProcessor := &MockAmazonProcessor{}
    
    fetcher := product.NewProductFetcher(mockClient, &config.AmazonConfig{}, mockProcessor)
    
    req := &product.FetchRequest{
        Platform:  "amazon",
        Region:    "US",
        ProductID: "B001",
    }
    
    product, err := fetcher.FetchProduct(req)
    assert.NoError(t, err)
    assert.NotNil(t, product)
}
```

## 未来扩展

1. **缓存层** - 添加内存缓存减少API调用
2. **批量获取** - 支持批量获取多个产品
3. **重试机制** - 内置重试逻辑
4. **指标收集** - 收集获取成功率、耗时等指标
