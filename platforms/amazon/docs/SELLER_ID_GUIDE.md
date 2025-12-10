# 如何获取 Amazon Seller ID

## ❌ 当前错误

```
Amazon API错误 [InvalidInput]: Invalid 'marketplaceIds' provided.
```

这个错误通常是因为 API 路径中的 Seller ID 不正确。

## 📋 什么是 Seller ID？

Seller ID（也叫 Merchant ID）是你的 Amazon 卖家账号的唯一标识符。

**格式示例：**
- `A1234567890ABC`
- `A2ABCDEFGHIJK`

## 🔍 如何获取 Seller ID

### 方法 1: 从 Seller Central 获取（推荐）

1. 登录 [Amazon Seller Central](https://sellercentral.amazon.com/)

2. 点击右上角的 **设置** 图标

3. 选择 **账户信息**

4. 在"商家令牌"部分，你会看到：
   ```
   商家令牌（Merchant Token）: A1234567890ABC
   ```
   这就是你的 Seller ID

### 方法 2: 从 URL 获取

1. 登录 Seller Central

2. 查看浏览器地址栏的 URL：
   ```
   https://sellercentral.amazon.com/home?merchantId=A1234567890ABC
   ```

3. `merchantId=` 后面的值就是你的 Seller ID

### 方法 3: 使用 SP-API 获取

可以调用 Sellers API 获取：

```go
// GET /sellers/v1/marketplaceParticipations
```

## ⚙️ 配置 Seller ID

### 更新配置结构

编辑 `common/config/config.go`，在 `SPAPIConfig` 中添加 `SellerID`：

```go
type SPAPIConfig struct {
    Enabled                bool   `yaml:"enabled"`
    Sandbox                bool   `yaml:"sandbox"`
    Region                 string `yaml:"region"`
    MarketplaceID          string `yaml:"marketplaceID"`
    SellerID               string `yaml:"sellerID"`  // 新增
    ClientID               string `yaml:"clientID"`
    ClientSecret           string `yaml:"clientSecret"`
    RefreshToken           string `yaml:"refreshToken"`
    DefaultFulfillmentType string `yaml:"defaultFulfillmentType"`
    DefaultCondition       string `yaml:"defaultCondition"`
}
```

### 更新配置文件

编辑 `config/config-dev.yaml`：

```yaml
amazon:
  spapi:
    enabled: true
    sandbox: true
    region: "us-east-1"
    marketplaceID: "ATVPDKIKX0DER"
    sellerID: "A1234567890ABC"  # 新增：填写你的 Seller ID
    clientID: "你的 Client ID"
    clientSecret: "你的 Client Secret"
    refreshToken: "你的 Refresh Token"
```

## 🔧 临时解决方案

如果暂时无法获取 Seller ID，可以尝试以下方法：

### 方法 1: 使用 Marketplace ID（可能不工作）

某些情况下可以使用 Marketplace ID 代替：

```go
path := fmt.Sprintf("/listings/2021-08-01/items/%s/%s?marketplaceIds=%s",
    c.marketplaceID,  // 使用 marketplaceID
    req.SKU,
    c.marketplaceID)
```

### 方法 2: 从 Refresh Token 解析

Refresh Token 中可能包含 Seller ID 信息（需要解码）

### 方法 3: 调用 API 获取

```go
// 首先调用 Sellers API 获取 Seller ID
GET /sellers/v1/marketplaceParticipations
```

## 📝 完整示例

### 1. 获取 Seller ID

从 Seller Central 获取：`A1234567890ABC`

### 2. 更新配置

```yaml
amazon:
  spapi:
    sellerID: "A1234567890ABC"
```

### 3. 更新代码

```go
// 在 Client 结构中添加 sellerID
type Client struct {
    httpClient    *http.Client
    baseURL       string
    authManager   *AuthManager
    region        string
    marketplaceID string
    sellerID      string  // 新增
    logger        *logrus.Entry
}

// 在 NewClient 中设置
func NewClient(cfg *Config) *Client {
    return &Client{
        // ...
        sellerID:      cfg.SellerID,  // 新增
        // ...
    }
}

// 在 CreateListing 中使用
path := fmt.Sprintf("/listings/2021-08-01/items/%s/%s?marketplaceIds=%s",
    c.sellerID,  // 使用 sellerID
    req.SKU,
    c.marketplaceID)
```

## ⚠️ 注意事项

1. **Seller ID 是必需的**
   - Listings Items API 需要 Seller ID
   - 不能省略或使用错误的值

2. **每个账号唯一**
   - 每个卖家账号有唯一的 Seller ID
   - 不同市场使用相同的 Seller ID

3. **格式要求**
   - 通常以 `A` 开头
   - 后跟 13 位字母数字组合
   - 例如：`A1234567890ABC`

## 🆘 常见问题

### Q: 找不到 Seller ID 怎么办？

A: 尝试以下方法：
1. 联系 Amazon 卖家支持
2. 查看账户设置中的所有信息
3. 使用 Sellers API 获取

### Q: Seller ID 和 Marketplace ID 有什么区别？

A:
- **Seller ID**: 你的卖家账号 ID（例如：`A1234567890ABC`）
- **Marketplace ID**: 市场 ID（例如：`ATVPDKIKX0DER` 代表美国市场）

### Q: 沙盒环境需要真实的 Seller ID 吗？

A: 是的，即使在沙盒环境中，也需要使用真实的 Seller ID。

## 📚 相关文档

- [Amazon SP-API Sellers API](https://developer-docs.amazon.com/sp-api/docs/sellers-api-v1-reference)
- [Listings Items API](https://developer-docs.amazon.com/sp-api/docs/listings-items-api-v2021-08-01-reference)

---

**下一步：** 获取你的 Seller ID 并更新配置文件。
