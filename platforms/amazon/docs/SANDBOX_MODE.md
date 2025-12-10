# Amazon SP-API 沙盒模式指南

## 📋 什么是沙盒模式？

Amazon SP-API 提供了沙盒环境，允许你在不影响真实数据的情况下测试 API 调用。

### 沙盒环境特点

✅ **优点：**
- 不会创建真实的产品 listing
- 不会影响库存和价格
- 可以无限次测试
- 不消耗 API 配额
- 适合开发和调试

⚠️ **限制：**
- 返回的是模拟数据
- 某些 API 功能可能不完整
- 不能验证真实的业务逻辑

## ⚙️ 如何启用沙盒模式

### 方法 1: 配置文件（推荐）

编辑 `config/config-dev.yaml`：

```yaml
amazon:
  spapi:
    enabled: true
    sandbox: true  # 设置为 true 启用沙盒模式
    region: "us-east-1"
    marketplaceID: "ATVPDKIKX0DER"
    clientID: "你的 Client ID"
    clientSecret: "你的 Client Secret"
    refreshToken: "你的 Refresh Token"
```

### 方法 2: 代码中指定

```go
apiClient := api.NewClient(&api.Config{
    Region:        "us-east-1",
    MarketplaceID: "ATVPDKIKX0DER",
    ClientID:      "your-client-id",
    ClientSecret:  "your-client-secret",
    RefreshToken:  "your-refresh-token",
    Sandbox:       true,  // 启用沙盒模式
})
```

## 🌍 沙盒环境端点

### 北美市场
- **生产环境**: `https://sellingpartnerapi-na.amazon.com`
- **沙盒环境**: `https://sandbox.sellingpartnerapi-na.amazon.com`

### 欧洲市场
- **生产环境**: `https://sellingpartnerapi-eu.amazon.com`
- **沙盒环境**: `https://sandbox.sellingpartnerapi-eu.amazon.com`

### 远东市场
- **生产环境**: `https://sellingpartnerapi-fe.amazon.com`
- **沙盒环境**: `https://sandbox.sellingpartnerapi-fe.amazon.com`

## 🧪 测试示例

### 1. 在沙盒模式下测试

```cmd
# 确保配置文件中 sandbox: true
go run ./cmd/amazon-test
```

输出会显示：
```
⚠️  沙盒模式：所有操作仅用于测试，不会影响真实数据
```

### 2. 切换到生产环境

修改配置文件：
```yaml
sandbox: false  # 切换到生产环境
```

**⚠️ 警告：** 生产环境的操作会影响真实数据！

## 📊 沙盒模式 vs 生产环境

| 特性 | 沙盒模式 | 生产环境 |
|------|---------|---------|
| 数据真实性 | 模拟数据 | 真实数据 |
| 影响真实业务 | ❌ 否 | ✅ 是 |
| API 配额消耗 | ❌ 否 | ✅ 是 |
| 测试次数限制 | ❌ 无限制 | ✅ 有限制 |
| 适用场景 | 开发、测试 | 生产部署 |
| 推荐用途 | 功能验证 | 正式上架 |

## 🔄 开发流程建议

### 阶段 1: 沙盒测试（开发阶段）
```yaml
sandbox: true
```
- 验证 API 调用是否正确
- 测试错误处理逻辑
- 调试数据格式
- 验证业务流程

### 阶段 2: 小规模生产测试
```yaml
sandbox: false
```
- 使用测试 SKU 进行真实上架
- 验证完整流程
- 确认数据准确性
- 测试 1-2 个商品

### 阶段 3: 正式部署
```yaml
sandbox: false
```
- 批量上架商品
- 监控错误率
- 优化性能

## ⚠️ 注意事项

### 1. 凭证要求

沙盒环境和生产环境使用**相同的凭证**：
- Client ID
- Client Secret
- Refresh Token

### 2. 数据差异

沙盒环境返回的数据是模拟的，可能与生产环境不同：
- 产品 ASIN 可能不存在
- 价格和库存是模拟值
- 某些验证规则可能不同

### 3. API 限制

某些 API 在沙盒环境中可能：
- 返回固定的响应
- 不支持某些参数
- 行为与生产环境略有不同

### 4. 切换环境

切换环境时需要：
1. 修改配置文件
2. 重启应用程序
3. 验证端点是否正确

## 🧪 测试用例示例

### 测试 1: 创建 Listing（沙盒）

```go
// 配置沙盒模式
cfg.Amazon.SPAPI.Sandbox = true

// 创建测试商品
req := &api.ListingRequest{
    SKU:         "TEST-SANDBOX-001",
    ProductType: "PRODUCT",
    Attributes: map[string]interface{}{
        "item_name": []map[string]string{
            {"value": "沙盒测试商品"},
        },
    },
}

resp, err := apiClient.CreateListing(ctx, req)
// 沙盒环境会返回模拟响应
```

### 测试 2: 验证错误处理（沙盒）

```go
// 故意使用无效数据测试错误处理
req := &api.ListingRequest{
    SKU: "",  // 空 SKU 应该返回错误
}

_, err := apiClient.CreateListing(ctx, req)
if err != nil {
    // 验证错误处理逻辑
    log.Printf("预期的错误: %v", err)
}
```

## 📚 相关文档

- [Amazon SP-API 沙盒指南](https://developer-docs.amazon.com/sp-api/docs/the-selling-partner-api-sandbox)
- [测试指南](./TESTING_GUIDE.md)
- [快速开始](./QUICK_START.md)
- [配置指南](../../config/amazon-spapi-setup.md)

## 💡 最佳实践

1. **始终先在沙盒测试**
   - 新功能开发时使用沙盒
   - 验证代码逻辑正确性
   - 确保错误处理完善

2. **小规模生产验证**
   - 使用少量测试数据
   - 监控错误和异常
   - 验证数据准确性

3. **生产环境谨慎操作**
   - 充分测试后再部署
   - 设置监控和告警
   - 准备回滚方案

4. **环境隔离**
   - 开发环境使用沙盒
   - 测试环境可选沙盒或生产
   - 生产环境禁用沙盒

## 🔍 调试技巧

### 查看当前环境

```cmd
go run ./cmd/amazon-test -debug
```

输出会显示：
```
SPAPI 配置:
  Sandbox: true  ← 当前使用沙盒模式
```

### 日志标识

程序会在日志中标识当前环境：
```
⚠️  使用沙盒环境 - 所有操作不会影响真实数据
```

---

**建议：** 在开发和测试阶段始终使用沙盒模式，确保功能正确后再切换到生产环境。
