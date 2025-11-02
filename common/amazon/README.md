# Amazon爬虫模块

这是从SHEIN项目迁移过来的Amazon爬虫模块，现在作为共享组件供TEMU和SHEIN平台使用。

## 功能特性

- **多浏览器池支持**: 支持浏览器池和单浏览器模式
- **反检测机制**: 内置反检测功能，避免被Amazon识别为机器人
- **多地区支持**: 支持Amazon各个地区站点（美国、日本、英国、德国等）
- **邮编设置**: 自动设置邮编获取准确的价格和配送信息
- **批量处理**: 支持批量处理多个产品，提高效率
- **错误恢复**: 自动检测和恢复被风控的浏览器实例

## 核心组件

### 1. AmazonProcessor
主要的处理器类，负责协调整个爬取流程。

```go
processor := amazon.NewAmazonProcessor(&config.Amazon)
defer processor.Shutdown()

product, err := processor.Process(url, zipcode)
if err != nil {
    log.Printf("处理失败: %v", err)
}
```

### 2. BrowserPool
浏览器池管理，支持多个浏览器实例并发处理。

```go
poolConfig := amazon.DefaultBrowserPoolConfig()
poolConfig.PoolSize = 3
browserPool := amazon.NewBrowserPool(cfg, poolConfig)
```

### 3. BrowserManager
单个浏览器实例管理，包含反检测和语言设置。

```go
manager := amazon.NewBrowserManager(cfg)
err := manager.Install()
err = manager.Launch()
page, err := manager.NewPage()
```

### 4. ZipcodeSetter
邮编设置器，支持各种Amazon站点的邮编设置。

```go
setter := amazon.NewZipcodeSetter(manager)
err := setter.SetAndVerifyZipcode(page, "10001")
```

## 配置说明

```yaml
amazon:
  enabled: true
  headless: true
  browserPath: "./chrome/chrome.exe"
  poolSize: 3
  zipcodes:
    US: "10001"
    JP: "100-0001"
    UK: "SW1A 1AA"
  viewportWidth: 1920
  viewportHeight: 1080
  proxyServer: ""  # 可选的代理服务器
```

## 使用示例

### 单个产品处理

```go
import "task-processor/common/amazon"

// 创建处理器
processor := amazon.NewAmazonProcessor(&config.Amazon)
defer processor.Shutdown()

// 处理单个产品
url := "https://www.amazon.com/dp/B08N5WRWNW"
zipcode := "10001"

product, err := processor.Process(url, zipcode)
if err != nil {
    log.Printf("处理失败: %v", err)
    return
}

log.Printf("产品标题: %s", product.Title)
log.Printf("价格: %.2f %s", product.FinalPrice, product.Currency)
```

### 批量处理

```go
// 准备批量请求
requests := []amazon.ProductRequest{
    {URL: "https://www.amazon.com/dp/B08N5WRWNW", Zipcode: "10001"},
    {URL: "https://www.amazon.com/dp/B07XJ8C8F5", Zipcode: "10001"},
}

// 批量处理
results := processor.ProcessBatch(requests)

for i, result := range results {
    if result.Error != nil {
        log.Printf("产品 %d 处理失败: %v", i, result.Error)
        continue
    }
    
    log.Printf("产品 %d: %s - %.2f %s", 
        i, result.Product.Title, result.Product.FinalPrice, result.Product.Currency)
}
```

## 支持的Amazon站点

- **美国**: amazon.com (USD)
- **日本**: amazon.co.jp (JPY)
- **英国**: amazon.co.uk (GBP)
- **德国**: amazon.de (EUR)
- **法国**: amazon.fr (EUR)
- **意大利**: amazon.it (EUR)
- **西班牙**: amazon.es (EUR)
- **加拿大**: amazon.ca (CAD)
- **澳大利亚**: amazon.com.au (AUD)

## 错误处理

爬虫包含完善的错误处理机制：

1. **网络错误**: 自动重试和超时处理
2. **风控检测**: 自动检测被风控的实例并重建
3. **页面错误**: 处理各种页面异常情况
4. **邮编设置失败**: 多种邮编设置策略

## 性能优化

1. **浏览器池**: 复用浏览器实例，减少启动开销
2. **邮编缓存**: 避免重复设置相同邮编
3. **批量处理**: 使用同一实例处理多个产品
4. **健康检查**: 定期检查和维护浏览器实例

## 注意事项

1. **合规使用**: 请遵守Amazon的robots.txt和使用条款
2. **频率控制**: 避免过于频繁的请求，建议添加适当延迟
3. **资源管理**: 确保正确关闭浏览器实例，避免资源泄漏
4. **错误监控**: 建议添加监控和告警机制

## 迁移说明

从SHEIN项目迁移到统一架构的主要变化：

1. **包路径**: 从 `task-processor/processor/modules/amazon` 改为 `task-processor/common/amazon`
2. **配置结构**: 使用统一的配置结构 `config.AmazonConfig`
3. **依赖简化**: 移除了一些SHEIN特定的依赖
4. **接口统一**: 提供了更简洁的API接口

## 扩展开发

如果需要添加新的提取器或功能：

1. 在相应的extractor文件中添加新的提取逻辑
2. 更新CompositeExtractor以包含新的提取器
3. 更新Product结构体以包含新的字段
4. 添加相应的测试用例

## 故障排除

常见问题和解决方案：

1. **浏览器启动失败**: 检查browserPath配置和Chrome安装
2. **邮编设置失败**: 检查目标站点和邮编格式
3. **提取数据为空**: 检查页面选择器是否正确
4. **频繁被风控**: 调整请求频率和使用代理