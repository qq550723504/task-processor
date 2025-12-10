# Amazon SP-API 沙盒模式常见问题

## ❓ 沙盒模式创建的产品在哪里可以看到？

### 简短回答

**沙盒模式创建的产品无法在 Seller Central 中看到。**

### 详细解释

沙盒环境是一个**完全独立的测试环境**，它的特点是：

#### 🔍 沙盒环境的本质

1. **模拟环境**
   - 沙盒不是真实的 Amazon 系统
   - 它只是一个模拟 API 响应的测试服务器
   - 不会创建真实的数据库记录

2. **数据不持久化**
   - 沙盒中"创建"的产品不会被保存
   - 每次调用都返回预设的模拟响应
   - 不会在任何地方显示

3. **无法查看**
   - ❌ Seller Central 中看不到
   - ❌ Amazon 前台看不到
   - ❌ 任何管理后台都看不到
   - ✅ 只能通过 API 响应确认

## 📊 沙盒 vs 生产环境对比

| 特性 | 沙盒模式 | 生产环境 |
|------|---------|---------|
| 创建真实产品 | ❌ 否 | ✅ 是 |
| Seller Central 可见 | ❌ 否 | ✅ 是 |
| Amazon 前台可见 | ❌ 否 | ✅ 是 |
| 数据持久化 | ❌ 否 | ✅ 是 |
| API 响应 | 模拟数据 | 真实数据 |
| 用途 | 测试代码逻辑 | 正式上架 |

## 🧪 沙盒模式的正确用途

### 1. 验证 API 调用是否正确

```go
// 测试 API 调用格式
resp, err := apiClient.CreateListing(ctx, req)
if err != nil {
    // 验证错误处理逻辑
    log.Printf("错误: %v", err)
} else {
    // 验证成功响应处理
    log.Printf("成功: SKU=%s, Status=%s", resp.SKU, resp.Status)
}
```

**沙盒会返回：**
```json
{
  "sku": "TEST-SKU-001",
  "status": "ACCEPTED"
}
```

### 2. 测试错误处理

```go
// 测试无效数据
req := &api.ListingRequest{
    SKU: "",  // 空 SKU
}

_, err := apiClient.CreateListing(ctx, req)
// 验证是否正确处理错误
```

### 3. 验证数据格式

```go
// 测试不同的属性组合
req := &api.ListingRequest{
    SKU: "TEST-001",
    Attributes: map[string]interface{}{
        "item_name": "测试商品",
        "brand": "TestBrand",
        // ... 其他属性
    },
}
```

### 4. 调试业务流程

- 验证完整的上架流程
- 测试重试机制
- 验证日志记录
- 测试监控告警

## ✅ 如何验证沙盒测试成功

### 方法 1: 查看 API 响应

```
INFO[2025-12-06] ✅ 商品上传成功！
INFO[2025-12-06]    SKU: TEST-SKU-001
INFO[2025-12-06]    状态: ACCEPTED
```

如果看到这个输出，说明：
- ✅ API 调用格式正确
- ✅ 认证成功
- ✅ 数据格式正确
- ✅ 代码逻辑正确

### 方法 2: 查看日志

```
time="2025-12-06" level=info msg="⚠️  沙盒模式：所有操作仅用于测试"
time="2025-12-06" level=info msg="✅ 认证成功"
time="2025-12-06" level=info msg="🚀 开始上传商品..."
time="2025-12-06" level=info msg="✅ 商品上传成功！"
```

### 方法 3: 检查错误处理

故意使用错误数据测试：
```go
req := &api.ListingRequest{
    SKU: "",  // 空 SKU 应该返回错误
}
```

如果正确返回错误，说明错误处理逻辑正确。

## 🔄 从沙盒切换到生产环境

### 步骤 1: 确认沙盒测试通过

- ✅ API 调用成功
- ✅ 错误处理正确
- ✅ 数据格式正确
- ✅ 业务流程完整

### 步骤 2: 切换到生产环境

编辑 `config/config-dev.yaml`：

```yaml
amazon:
  spapi:
    sandbox: false  # 切换到生产环境
```

### 步骤 3: 小规模测试

```cmd
# 使用测试 SKU
go run ./cmd/amazon-test -sku "REAL-TEST-001" -title "真实测试商品"
```

### 步骤 4: 在 Seller Central 查看

1. 登录 [Amazon Seller Central](https://sellercentral.amazon.com/)
2. 进入：**库存 > 管理库存**
3. 搜索你的 SKU：`REAL-TEST-001`
4. 应该能看到刚创建的产品

## 📝 沙盒模式的典型响应

### 成功响应示例

```json
{
  "sku": "TEST-SKU-001",
  "status": "ACCEPTED",
  "submissionId": "sandbox-submission-12345",
  "issues": []
}
```

### 错误响应示例

```json
{
  "sku": "TEST-SKU-001",
  "status": "INVALID",
  "issues": [
    {
      "code": "8541",
      "message": "Missing required attribute 'brand'",
      "severity": "ERROR"
    }
  ]
}
```

## 💡 最佳实践

### 1. 开发阶段（沙盒）

```yaml
sandbox: true
```

**目标：**
- 验证代码逻辑
- 测试错误处理
- 调试数据格式
- 完善业务流程

**验证方式：**
- 查看 API 响应
- 检查日志输出
- 验证错误处理

### 2. 测试阶段（生产）

```yaml
sandbox: false
```

**目标：**
- 创建 1-2 个测试产品
- 验证真实上架流程
- 确认数据准确性

**验证方式：**
- Seller Central 查看产品
- Amazon 前台搜索
- 确认所有信息正确

### 3. 生产部署（生产）

```yaml
sandbox: false
```

**目标：**
- 批量上架商品
- 监控错误率
- 优化性能

**验证方式：**
- 批量检查 Seller Central
- 监控系统告警
- 分析错误日志

## ⚠️ 常见误区

### 误区 1: 沙盒产品应该在 Seller Central 显示

❌ **错误理解**：沙盒创建的产品应该在某个地方显示

✅ **正确理解**：沙盒只是模拟 API 响应，不创建真实数据

### 误区 2: 沙盒需要单独的凭证

❌ **错误理解**：沙盒和生产需要不同的 Client ID/Secret

✅ **正确理解**：使用相同的凭证，通过配置切换环境

### 误区 3: 沙盒可以测试真实业务

❌ **错误理解**：沙盒可以测试库存、价格等真实业务

✅ **正确理解**：沙盒只能测试 API 调用格式和代码逻辑

## 🎯 总结

### 沙盒模式的价值

1. **零风险测试**
   - 不会创建真实产品
   - 不会影响账号
   - 可以无限次测试

2. **快速验证**
   - 验证 API 调用
   - 测试错误处理
   - 调试代码逻辑

3. **开发效率**
   - 不需要清理测试数据
   - 不消耗 API 配额
   - 快速迭代开发

### 何时切换到生产环境

当满足以下条件时：
- ✅ 沙盒测试全部通过
- ✅ 错误处理完善
- ✅ 数据格式正确
- ✅ 业务流程完整
- ✅ 准备好监控和回滚

### 生产环境验证

切换到生产环境后：
1. 使用测试 SKU 小规模验证
2. 在 Seller Central 查看产品
3. 确认所有信息正确
4. 逐步扩大规模

## 📚 相关文档

- [沙盒模式详细指南](./SANDBOX_MODE.md)
- [测试指南](./TESTING_GUIDE.md)
- [快速开始](./QUICK_START.md)

---

**记住：沙盒是用来测试代码的，不是用来创建产品的！** 🎯
