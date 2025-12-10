# Amazon 平台上架模块

## 概述

Amazon 平台上架模块提供了完整的 Amazon Seller Central 产品上架功能，支持通过 Amazon SP-API 创建、更新和管理产品 listing。

## 功能特性

- ✅ 产品 Listing 创建和更新
- ✅ 库存管理
- ✅ 价格管理
- ✅ 产品数据验证
- ✅ 多市场支持（北美、欧洲、远东）
- ✅ 错误重试机制
- ✅ 任务状态跟踪

## 项目结构

```
platforms/amazon/
├── api/                    # Amazon SP-API 客户端
│   ├── client.go          # API 客户端基础
│   ├── listings.go        # Listing 相关 API
│   ├── inventory.go       # 库存相关 API
│   └── pricing.go         # 价格相关 API
├── handlers/              # 处理步骤
│   ├── store_info_handler.go
│   ├── product_data_handler.go
│   ├── validation_handler.go
│   ├── listing_handler.go
│   ├── inventory_handler.go
│   └── pricing_handler.go
├── service/               # 业务逻辑层
│   ├── listing_service.go
│   ├── inventory_service.go
│   └── pricing_service.go
├── utils/                 # 工具类
│   ├── converter.go
│   └── validator.go
├── models.go              # 数据模型
├── processor.go           # 主处理器
├── task_handler.go        # 任务处理器
├── pipeline.go            # 处理管道
├── context.go             # 任务上下文
└── errors.go              # 错误定义
```

## 配置说明

在 `config/config-dev.yaml` 中添加 Amazon 配置：

```yaml
# Amazon 平台配置
amazon:
  enabled: true
  region: "us-east-1"           # AWS 区域
  marketplaceID: "ATVPDKIKX0DER" # 美国市场
  clientID: "your-client-id"
  clientSecret: "your-client-secret"
  refreshToken: "your-refresh-token"
  
  # 自动核价配置
  autoPricing:
    enabled: false
    interval: 300  # 5分钟
    batchSize: 100
```

## 使用示例

### 1. 创建 Amazon 处理器

```go
import (
    "task-processor/common/config"
    "task-processor/platforms/amazon"
    "github.com/sirupsen/logrus"
)

// 加载配置
cfg := config.LoadConfig("config/config-dev.yaml")

// 创建处理器
logger := logrus.New()
processor := amazon.NewAmazonProcessor(cfg, logger)

// 启动处理器
ctx := context.Background()
if err := processor.Start(ctx); err != nil {
    log.Fatal(err)
}

// 优雅关闭
defer processor.Close()
```

### 2. 处理上架任务

```go
task := types.Task{
    ID:        "12345",
    ProductID: "B08N5WRWNW",
    StoreID:   556,
    TenantID:  1,
}

err := processor.ProcessTask(ctx, task)
if err != nil {
    log.Printf("任务处理失败: %v", err)
}
```

### 3. 使用 API 客户端

```go
import "task-processor/platforms/amazon/api"

// 创建 API 客户端
apiClient := api.NewClient(&api.Config{
    Region:        "us-east-1",
    MarketplaceID: "ATVPDKIKX0DER",
    ClientID:      "your-client-id",
    ClientSecret:  "your-client-secret",
    RefreshToken:  "your-refresh-token",
})

// 创建 listing
req := &api.ListingRequest{
    SKU:         "MY-SKU-001",
    ProductType: "PRODUCT",
    Attributes: map[string]interface{}{
        "item_name": "My Product",
        "brand":     "My Brand",
    },
}

resp, err := apiClient.CreateListing(ctx, req)
```

## 处理流程

Amazon 平台的上架流程包含以下步骤：

1. **获取店铺信息** - 验证店铺配置
2. **获取产品数据** - 从管理系统获取原始数据
3. **验证产品数据** - 检查必填字段和格式
4. **创建 Listing** - 调用 Amazon SP-API 创建产品
5. **设置库存** - 更新产品库存数量
6. **设置价格** - 更新产品售价
7. **保存结果** - 记录上架结果

## 错误处理

模块支持以下错误类型：

- `ErrInvalidASIN` - ASIN 无效
- `ErrInvalidSKU` - SKU 无效
- `ErrProductNotFound` - 产品未找到
- `ErrAPIRateLimit` - API 速率限制
- `ErrAuthenticationFailed` - 认证失败
- `ErrInsufficientInventory` - 库存不足
- `ErrCategoryRestricted` - 分类受限

可重试错误会自动重试，不可重试错误会标记任务为终止状态。

## Amazon SP-API 文档

- [SP-API 开发者指南](https://developer-docs.amazon.com/sp-api/)
- [Listings Items API](https://developer-docs.amazon.com/sp-api/docs/listings-items-api-v2021-08-01-reference)
- [Product Pricing API](https://developer-docs.amazon.com/sp-api/docs/product-pricing-api-v0-reference)
- [FBA Inventory API](https://developer-docs.amazon.com/sp-api/docs/fba-inventory-api-v1-reference)

## 注意事项

1. **API 限流**：Amazon SP-API 有严格的速率限制，需要合理控制请求频率
2. **认证令牌**：需要定期刷新 LWA (Login with Amazon) 访问令牌
3. **市场 ID**：不同国家/地区使用不同的 Marketplace ID
4. **产品类型**：不同产品类型需要不同的属性模板
5. **图片要求**：主图需要白色背景，尺寸至少 1000x1000 像素

## 开发计划

- [ ] 完善 Amazon SP-API 集成
- [ ] 添加变体产品支持
- [ ] 实现批量上架功能
- [ ] 添加产品监控功能
- [ ] 支持 FBA 库存管理
- [ ] 添加订单同步功能
