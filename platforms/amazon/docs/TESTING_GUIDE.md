# Amazon 商品上传测试指南

## 前提条件

### 1. 配置 Amazon SP-API 凭证

在 `config/config-dev.yaml` 中配置 Amazon SP-API：

```yaml
amazon:
  spapi:
    enabled: true                      # 启用 SP-API
    region: "us-east-1"                # AWS 区域
    marketplaceID: "ATVPDKIKX0DER"     # 美国市场ID
    clientID: "your-client-id"         # 替换为你的 Client ID
    clientSecret: "your-client-secret" # 替换为你的 Client Secret
    refreshToken: "your-refresh-token" # 替换为你的 Refresh Token
    defaultFulfillmentType: "FBM"      # 配送方式
    defaultCondition: "New"            # 产品状态
```

### 2. 获取 Amazon SP-API 凭证

1. 登录 [Amazon Seller Central](https://sellercentral.amazon.com/)
2. 进入 **设置 > 用户权限 > 开发者中心**
3. 创建新的应用程序
4. 获取 Client ID 和 Client Secret
5. 生成 Refresh Token

详细步骤参考：[Amazon SP-API 文档](https://developer-docs.amazon.com/sp-api/docs/registering-your-application)

## 测试方法

### 方法 1: 使用测试程序（推荐）

```bash
# 使用默认参数测试
go run cmd/amazon-test/main.go

# 自定义参数测试
go run cmd/amazon-test/main.go \
  -sku "MY-TEST-SKU-001" \
  -title "我的测试商品" \
  -brand "MyBrand" \
  -price 29.99
```

**参数说明：**
- `-config`: 配置文件路径（默认: config/config-dev.yaml）
- `-sku`: 产品 SKU（默认: TEST-SKU-001）
- `-title`: 商品标题（默认: 测试商品）
- `-brand`: 品牌名称（默认: TestBrand）
- `-price`: 商品价格（默认: 19.99）

### 方法 2: 使用 API 客户端

```go
package main

import (
    "context"
    "task-processor/platforms/amazon/api"
    "github.com/sirupsen/logrus"
)

func main() {
    // 创建 API 客户端
    client := api.NewClient(&api.Config{
        Region:        "us-east-1",
        MarketplaceID: "ATVPDKIKX0DER",
        ClientID:      "your-client-id",
        ClientSecret:  "your-client-secret",
        RefreshToken:  "your-refresh-token",
    })

    // 创建商品
    ctx := context.Background()
    req := &api.ListingRequest{
        SKU:         "TEST-SKU-001",
        ProductType: "PRODUCT",
        Requirements: "LISTING",
        Attributes: map[string]interface{}{
            "item_name": []map[string]string{
                {
                    "value":         "测试商品",
                    "language_tag":  "en_US",
                    "marketplace_id": "ATVPDKIKX0DER",
                },
            },
            "brand": []map[string]string{
                {
                    "value":         "TestBrand",
                    "language_tag":  "en_US",
                    "marketplace_id": "ATVPDKIKX0DER",
                },
            },
        },
    }

    resp, err := client.CreateListing(ctx, req)
    if err != nil {
        logrus.Fatalf("上传失败: %v", err)
    }

    logrus.Infof("上传成功: SKU=%s, Status=%s", resp.SKU, resp.Status)
}
```

## 预期结果

### 成功情况

```
INFO[2025-12-06] === Amazon 商品上传测试 ===
INFO[2025-12-06] ✅ 配置加载成功
INFO[2025-12-06]    区域: us-east-1
INFO[2025-12-06]    市场: ATVPDKIKX0DER
INFO[2025-12-06] ✅ API客户端创建成功
INFO[2025-12-06] 🔐 测试认证...
INFO[2025-12-06] ✅ 认证成功，Token: Atza|IwEBIJK1234567...
INFO[2025-12-06] 📦 准备商品数据...
INFO[2025-12-06]    SKU: TEST-SKU-001
INFO[2025-12-06]    标题: 测试商品
INFO[2025-12-06]    品牌: TestBrand
INFO[2025-12-06]    价格: $19.99
INFO[2025-12-06] 🚀 开始上传商品...
INFO[2025-12-06] ✅ 商品上传成功！
INFO[2025-12-06]    SKU: TEST-SKU-001
INFO[2025-12-06]    状态: ACCEPTED
INFO[2025-12-06] === 测试完成 ===
```

### 失败情况

#### 1. 认证失败
```
FATAL[2025-12-06] ❌ 认证失败: invalid_client
```
**解决方法：** 检查 Client ID 和 Client Secret 是否正确

#### 2. 凭证未配置
```
FATAL[2025-12-06] ❌ Amazon SP-API 凭证未配置
```
**解决方法：** 在配置文件中设置 clientID 和 clientSecret

#### 3. API 速率限制
```
ERROR[2025-12-06] ❌ 上传失败: API rate limit exceeded
```
**解决方法：** 等待一段时间后重试

#### 4. 属性验证失败
```
WARN[2025-12-06] ⚠️  存在以下问题:
WARN[2025-12-06]    1. [ERROR] 8541: Missing required attribute 'manufacturer'
```
**解决方法：** 补充缺失的必填属性

## 常见问题

### Q1: 如何获取不同市场的 Marketplace ID？

**北美市场：**
- 美国: `ATVPDKIKX0DER`
- 加拿大: `A2EUQ1WTGCTBG2`
- 墨西哥: `A1AM78C64UM0Y8`

**欧洲市场：**
- 英国: `A1F83G8C2ARO7P`
- 德国: `A1PA6795UKMFR9`
- 法国: `A13V1IB3VIYZZH`

**远东市场：**
- 日本: `A1VC38T7YXB528`
- 澳大利亚: `A39IBJ37TRP1C6`

### Q2: 如何测试不同的产品类型？

修改 `ProductType` 字段：
```go
ProductType: "LUGGAGE"  // 行李箱
ProductType: "SHOES"    // 鞋类
ProductType: "APPAREL"  // 服装
```

不同产品类型需要不同的属性，参考 [Product Type Definitions API](https://developer-docs.amazon.com/sp-api/docs/product-type-definitions-api-v2020-09-01-reference)

### Q3: 如何添加图片？

```go
Attributes: map[string]interface{}{
    "main_product_image_locator": []map[string]string{
        {
            "media_location": "https://example.com/image.jpg",
            "marketplace_id": "ATVPDKIKX0DER",
        },
    },
}
```

### Q4: 如何设置库存？

商品创建后，使用库存 API 设置：
```go
inventoryClient := api.NewClient(config)
err := inventoryClient.UpdateInventory(ctx, &api.InventoryRequest{
    SKU:      "TEST-SKU-001",
    Quantity: 100,
})
```

## 下一步

测试成功后，可以：

1. **集成到完整流程**：使用 `AmazonProcessor` 处理完整的上架任务
2. **添加图片上传**：使用 `ImageHandler` 处理图片
3. **支持变体产品**：使用 `VariantHandler` 处理变体
4. **批量上架**：使用批量处理功能

## 参考资料

- [Amazon SP-API 文档](https://developer-docs.amazon.com/sp-api/)
- [Listings Items API](https://developer-docs.amazon.com/sp-api/docs/listings-items-api-v2021-08-01-reference)
- [产品类型定义](https://developer-docs.amazon.com/sp-api/docs/product-type-definitions-api-v2020-09-01-reference)
