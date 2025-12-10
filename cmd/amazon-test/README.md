# Amazon 商品上传测试工具

## 快速开始

### 1. 配置 Amazon SP-API 凭证

编辑 `config/config-dev.yaml`，添加以下配置：

```yaml
amazon:
  spapi:
    enabled: true
    region: "us-east-1"
    marketplaceID: "ATVPDKIKX0DER"
    clientID: "amzn1.application-oa2-client.xxxxx"
    clientSecret: "xxxxxxxxxxxxxxxxxxxxx"
    refreshToken: "Atzr|IwEBIxxxxxxxxxxxxxx"
    defaultFulfillmentType: "FBM"
    defaultCondition: "New"
```

### 2. 运行测试

**Windows:**
```cmd
scripts\test-amazon-upload.bat
```

**或直接运行:**
```cmd
go run cmd/amazon-test/main.go
```

**自定义参数:**
```cmd
go run cmd/amazon-test/main.go -sku "MY-SKU-001" -title "我的商品" -brand "MyBrand" -price 29.99
```

### 3. 查看结果

成功输出示例：
```
INFO[2025-12-06] === Amazon 商品上传测试 ===
INFO[2025-12-06] 🔍 检查配置...
INFO[2025-12-06] ✅ 配置检查通过
INFO[2025-12-06] 📋 当前配置:
INFO[2025-12-06]    启用状态: true
INFO[2025-12-06]    区域: us-east-1
INFO[2025-12-06]    市场ID: ATVPDKIKX0DER
INFO[2025-12-06] ✅ API客户端创建成功
INFO[2025-12-06] 🔐 测试认证...
INFO[2025-12-06] ✅ 认证成功
INFO[2025-12-06] 📦 准备商品数据...
INFO[2025-12-06] 🚀 开始上传商品...
INFO[2025-12-06] ✅ 商品上传成功！
INFO[2025-12-06]    SKU: TEST-SKU-001
INFO[2025-12-06]    状态: ACCEPTED
INFO[2025-12-06] === 测试完成 ===
```

## 命令行参数

| 参数 | 说明 | 默认值 |
|------|------|--------|
| `-sku` | 产品 SKU | `TEST-SKU-001` |
| `-title` | 商品标题 | `测试商品` |
| `-brand` | 品牌名称 | `TestBrand` |
| `-price` | 商品价格 | `19.99` |

**注意：** 配置文件路径由环境变量 `TASK_PROCESSOR_ENV` 控制，默认加载 `config/config-dev.yaml`

## 常见问题

### Q: 如何获取 Amazon SP-API 凭证？

1. 登录 [Amazon Seller Central](https://sellercentral.amazon.com/)
2. 进入 **设置 > 用户权限 > 开发者中心**
3. 创建新应用程序
4. 获取 Client ID、Client Secret 和 Refresh Token

详细步骤：https://developer-docs.amazon.com/sp-api/docs/registering-your-application

### Q: 认证失败怎么办？

检查以下几点：
- Client ID 和 Client Secret 是否正确
- Refresh Token 是否有效
- 网络连接是否正常
- 是否有 API 访问权限

### Q: 上传失败怎么办？

查看错误信息：
- `Missing required attribute`: 缺少必填属性
- `Invalid attribute value`: 属性值无效
- `Rate limit exceeded`: API 速率限制

### Q: 如何测试不同市场？

修改配置文件中的 `marketplaceID` 和 `region`：

```yaml
# 美国市场
marketplaceID: "ATVPDKIKX0DER"
region: "us-east-1"

# 英国市场
marketplaceID: "A1F83G8C2ARO7P"
region: "eu-west-1"

# 日本市场
marketplaceID: "A1VC38T7YXB528"
region: "us-west-2"
```

## 下一步

测试成功后，可以：

1. 集成到完整的上架流程
2. 添加图片上传功能
3. 支持变体产品
4. 实现批量上架

详细文档：`platforms/amazon/docs/TESTING_GUIDE.md`
