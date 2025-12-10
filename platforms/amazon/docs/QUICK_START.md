# Amazon 商品上传快速开始指南

## 📋 前置要求

1. **Amazon 卖家账号**：已注册并激活的 Amazon Seller Central 账号
2. **SP-API 访问权限**：已申请并获得 SP-API 开发者权限
3. **Go 环境**：Go 1.19 或更高版本

## 🚀 5 分钟快速测试

### 步骤 1: 获取 SP-API 凭证

1. 登录 [Amazon Seller Central](https://sellercentral.amazon.com/)
2. 导航到：**设置 > 用户权限 > 开发者中心**
3. 点击 **添加新的开发者应用程序**
4. 填写应用信息并提交
5. 获取以下凭证：
   - **Client ID**: `amzn1.application-oa2-client.xxxxx`
   - **Client Secret**: `xxxxxxxxxxxxxxxxxxxxx`
   - **Refresh Token**: `Atzr|IwEBIxxxxxxxxxxxxxx`

### 步骤 2: 配置凭证

**方法 A: 使用配置向导（推荐）**
```cmd
scripts\setup-amazon-config.bat
```

**方法 B: 手动编辑配置文件**

编辑 `config/config-dev.yaml`：

```yaml
amazon:
  spapi:
    enabled: true
    region: "us-east-1"
    marketplaceID: "ATVPDKIKX0DER"
    clientID: "你的 Client ID"
    clientSecret: "你的 Client Secret"
    refreshToken: "你的 Refresh Token"
    defaultFulfillmentType: "FBM"
    defaultCondition: "New"
```

### 步骤 3: 运行测试

**Windows:**
```cmd
scripts\test-amazon-upload.bat
```

**或使用 Go 命令:**
```cmd
go run cmd/amazon-test/main.go
```

### 步骤 4: 查看结果

成功输出：
```
✅ 商品上传成功！
   SKU: TEST-SKU-001
   状态: ACCEPTED
```

## 📝 自定义测试

### 测试不同的商品

```cmd
go run cmd/amazon-test/main.go ^
  -sku "MY-PRODUCT-001" ^
  -title "我的测试商品" ^
  -brand "MyBrand" ^
  -price 39.99
```

### 测试不同的市场

修改配置文件中的市场设置：

**美国市场:**
```yaml
region: "us-east-1"
marketplaceID: "ATVPDKIKX0DER"
```

**英国市场:**
```yaml
region: "eu-west-1"
marketplaceID: "A1F83G8C2ARO7P"
```

**日本市场:**
```yaml
region: "us-west-2"
marketplaceID: "A1VC38T7YXB528"
```

## 🔧 故障排查

### 问题 1: 认证失败

**错误信息:**
```
❌ 认证失败: invalid_client
```

**解决方法:**
1. 检查 Client ID 和 Client Secret 是否正确
2. 确认 Refresh Token 未过期
3. 验证应用程序状态是否为"已发布"

### 问题 2: 配置未找到

**错误信息:**
```
❌ 配置检查失败: amazon.spapi.clientID 未配置
```

**解决方法:**
1. 确认配置文件路径正确
2. 检查 YAML 格式是否正确（注意缩进）
3. 确认所有必填字段都已填写

### 问题 3: API 速率限制

**错误信息:**
```
❌ 上传失败: API rate limit exceeded
```

**解决方法:**
1. 等待 1-2 分钟后重试
2. 减少请求频率
3. 查看 Amazon SP-API 速率限制文档

### 问题 4: 属性验证失败

**错误信息:**
```
⚠️  存在以下问题:
   1. [ERROR] Missing required attribute 'manufacturer'
```

**解决方法:**
1. 查看错误信息中缺少的属性
2. 参考产品类型定义文档
3. 补充必填属性

## 📚 市场 ID 参考

### 北美市场
| 国家 | Marketplace ID | Region |
|------|----------------|--------|
| 美国 | ATVPDKIKX0DER | us-east-1 |
| 加拿大 | A2EUQ1WTGCTBG2 | us-east-1 |
| 墨西哥 | A1AM78C64UM0Y8 | us-east-1 |

### 欧洲市场
| 国家 | Marketplace ID | Region |
|------|----------------|--------|
| 英国 | A1F83G8C2ARO7P | eu-west-1 |
| 德国 | A1PA6795UKMFR9 | eu-west-1 |
| 法国 | A13V1IB3VIYZZH | eu-west-1 |
| 意大利 | APJ6JRA9NG5V4 | eu-west-1 |
| 西班牙 | A1RKKUPIHCS9HS | eu-west-1 |

### 远东市场
| 国家 | Marketplace ID | Region |
|------|----------------|--------|
| 日本 | A1VC38T7YXB528 | us-west-2 |
| 澳大利亚 | A39IBJ37TRP1C6 | us-west-2 |
| 新加坡 | A19VAU5U5O7RUS | us-west-2 |

## 🎯 下一步

测试成功后，你可以：

### 1. 集成到完整流程

```go
import "task-processor/platforms/amazon"

processor := amazon.NewAmazonProcessor(cfg, logger)
processor.Start(ctx)

task := types.Task{
    ID:        "12345",
    ProductID: "B08N5WRWNW",
    StoreID:   556,
}

err := processor.ProcessTask(ctx, task)
```

### 2. 添加图片上传

参考：`platforms/amazon/handlers/image_handler.go`

### 3. 支持变体产品

参考：`platforms/amazon/handlers/variant_handler.go`

### 4. 实现批量上架

参考：`platforms/amazon/ROADMAP.md` 中的批量处理计划

## 📖 相关文档

- [完整测试指南](./TESTING_GUIDE.md)
- [开发路线图](../ROADMAP.md)
- [Amazon SP-API 官方文档](https://developer-docs.amazon.com/sp-api/)
- [Listings Items API](https://developer-docs.amazon.com/sp-api/docs/listings-items-api-v2021-08-01-reference)

## 💡 提示

1. **测试环境**：建议先在测试环境测试，避免影响生产数据
2. **SKU 命名**：使用有意义的 SKU 命名规则，便于管理
3. **日志记录**：保留测试日志，便于问题排查
4. **速率限制**：注意 Amazon API 的速率限制，避免被限流
5. **数据备份**：上传前备份重要数据

## 🆘 获取帮助

遇到问题？

1. 查看 [故障排查](#-故障排查) 部分
2. 查看 [Amazon SP-API 文档](https://developer-docs.amazon.com/sp-api/)
3. 查看项目 Issues
4. 联系技术支持

---

**祝你测试顺利！** 🎉
