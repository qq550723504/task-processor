# JSON解析调试日志增强

## 问题描述

遇到以下错误日志，但缺少足够的信息来诊断问题：
```
WARN[2025-11-21 13:44:09] 解析服务器历史数据失败: 解析JSON数据失败: json: cannot unmarshal object into Go value of type []amazon.Product
```

问题：
- 不知道JSON数据的实际内容
- 不知道是什么导致解析失败
- 无法快速定位问题

## 解决方案

### 修改文件：`platforms/shein/modules/raw_json_data_handler.go`

#### 1. 增强 `Handle` 方法的日志

**新增日志：**
- JSON数据长度
- JSON数据前200字符预览
- 解析成功时的产品标题
- 解析失败时的详细错误和数据预览

**修改前：**
```go
logrus.Infof("服务器有历史数据，直接使用: ProductID=%s", ctx.Task.ProductID)
amazonProduct, parseErr := ParseAmazonProduct(rawJsonData.RawJSONData)
if parseErr == nil {
    ctx.AmazonProduct = amazonProduct
    return nil
}
logrus.Warnf("解析服务器历史数据失败: %v", parseErr)
```

**修改后：**
```go
logrus.Infof("服务器有历史数据，直接使用: ProductID=%s, 数据长度=%d", ctx.Task.ProductID, len(rawJsonData.RawJSONData))

// 打印JSON数据的前200个字符用于调试
jsonPreview := rawJsonData.RawJSONData
if len(jsonPreview) > 200 {
    jsonPreview = jsonPreview[:200] + "..."
}
logrus.Debugf("JSON数据预览: %s", jsonPreview)

amazonProduct, parseErr := ParseAmazonProduct(rawJsonData.RawJSONData)
if parseErr == nil {
    ctx.AmazonProduct = amazonProduct
    logrus.Infof("成功解析服务器历史数据: ProductID=%s, Title=%s", ctx.Task.ProductID, amazonProduct.Title)
    return nil
}
logrus.Warnf("解析服务器历史数据失败: %v, JSON数据前200字符: %s", parseErr, jsonPreview)
```

#### 2. 增强 `ParseAmazonProduct` 函数的日志

**新增功能：**
- 检测JSON数据类型（对象 vs 数组）
- 记录每个解析步骤的结果
- 解析失败时打印数据类型信息
- 成功时记录产品基本信息

**关键改进：**
```go
// 检测JSON数据的类型
if trimmedData[i] == '{' {
    logrus.Debug("检测到JSON对象格式，尝试解析为单个产品")
} else if trimmedData[i] == '[' {
    logrus.Debug("检测到JSON数组格式，尝试解析为产品数组")
}

// 解析成功时的日志
logrus.Debugf("成功解析为单个产品对象: Title=%s, ASIN=%s", product.Title, product.Asin)

// 解析失败时的详细日志
logrus.Debugf("解析为单个对象失败: %v", err)
logrus.Errorf("解析为数组也失败: %v", err)

// 打印JSON数据的实际类型
var rawData interface{}
if parseErr := json.Unmarshal([]byte(jsonData), &rawData); parseErr == nil {
    logrus.Debugf("JSON数据类型: %T", rawData)
}
```

## 新增日志示例

### 成功解析时：
```
INFO[13:44:09] 服务器有历史数据，直接使用: ProductID=B08XYZ123, 数据长度=5432
DEBUG[13:44:09] JSON数据预览: {"title":"Example Product","asin":"B08XYZ123",...
DEBUG[13:44:09] 检测到JSON对象格式，尝试解析为单个产品
DEBUG[13:44:09] 成功解析为单个产品对象: Title=Example Product, ASIN=B08XYZ123
INFO[13:44:09] 成功解析服务器历史数据: ProductID=B08XYZ123, Title=Example Product
```

### 解析失败时：
```
INFO[13:44:09] 服务器有历史数据，直接使用: ProductID=B08XYZ123, 数据长度=5432
DEBUG[13:44:09] JSON数据预览: {"error":"not found","status":404}...
DEBUG[13:44:09] 检测到JSON对象格式，尝试解析为单个产品
DEBUG[13:44:09] 解析为单个对象失败: json: cannot unmarshal string into Go struct field Product.final_price of type float64
DEBUG[13:44:09] 解析为数组也失败: json: cannot unmarshal object into Go value of type []amazon.Product
DEBUG[13:44:09] JSON数据类型: map[string]interface {}
ERROR[13:44:09] 解析为数组也失败: json: cannot unmarshal object into Go value of type []amazon.Product
WARN[13:44:09] 解析服务器历史数据失败: 解析JSON数据失败: json: cannot unmarshal object into Go value of type []amazon.Product, JSON数据前200字符: {"error":"not found","status":404}...
```

## 调试步骤

当遇到JSON解析错误时，按以下步骤排查：

1. **查看数据长度**
   - 如果为0，说明服务器返回空数据
   
2. **查看JSON预览**
   - 检查数据格式是否正确
   - 是否包含错误信息（如 `{"error": "..."}`）
   
3. **查看数据类型检测**
   - 确认是对象 `{}` 还是数组 `[]`
   
4. **查看具体错误**
   - `cannot unmarshal string into float64` → 字段类型不匹配
   - `cannot unmarshal object into []Type` → 期望数组但收到对象
   
5. **查看实际数据类型**
   - `map[string]interface {}` → 对象
   - `[]interface {}` → 数组

## 日志级别说明

- **INFO**: 关键流程节点（有数据、解析成功）
- **DEBUG**: 详细调试信息（数据预览、类型检测、解析步骤）
- **WARN**: 解析失败但可能有备用方案
- **ERROR**: 严重错误，所有解析尝试都失败

## 产品页面检查增强（新增）

### 问题
当遇到"产品页面缺少必要元素"错误时，缺少足够的信息来判断：
- 页面是否加载成功？
- 页面显示的是什么内容？
- 为什么找不到标题元素？

### 解决方案

#### 修改文件：`common/amazon/processor.go` - `checkProductExists` 方法

**新增日志：**
1. 当前页面URL
2. 页面标题（HTML title）
3. 检查每个选择器的结果
4. 找到标题时显示标题文本
5. 未找到标题时显示页面HTML前500字符

**日志示例：**

成功找到产品：
```
INFO[14:02:09] 开始检查产品页面有效性: URL=https://www.amazon.com/dp/B08XYZ123
INFO[14:02:09] 页面标题: Example Product - Amazon.com
DEBUG[14:02:09] 检查产品不存在标识...
DEBUG[14:02:09] 检查产品标题元素...
INFO[14:02:09] 找到产品标题元素: selector=#productTitle, text=Example Product
INFO[14:02:09] 产品页面有效性检查通过: selector=#productTitle
```

产品不存在：
```
INFO[14:02:09] 开始检查产品页面有效性: URL=https://www.amazon.com/dp/INVALID
INFO[14:02:09] 页面标题: Page Not Found
DEBUG[14:02:09] 检查产品不存在标识...
WARN[14:02:09] 检测到产品不存在或页面异常: selector=text=Page Not Found, URL=https://www.amazon.com/dp/INVALID, PageTitle=Page Not Found
```

页面异常（找不到标题）：
```
INFO[14:02:09] 开始检查产品页面有效性: URL=https://www.amazon.com/dp/B08XYZ123
INFO[14:02:09] 页面标题: Amazon.com
DEBUG[14:02:09] 检查产品不存在标识...
DEBUG[14:02:09] 检查产品标题元素...
DEBUG[14:02:09] 查询选择器失败: selector=#productTitle, error=timeout
WARN[14:02:09] 未找到产品标题元素，页面可能异常
WARN[14:02:09] 页面URL: https://www.amazon.com/dp/B08XYZ123
WARN[14:02:09] 页面标题: Amazon.com
WARN[14:02:09] 页面HTML前500字符: <div id="main">...</div>
WARN[14:02:09] 尝试的选择器: [#productTitle #title h1[id*='title']]
```

## 注意事项

1. JSON预览限制在200字符，避免日志过大
2. 使用DEBUG级别记录详细信息，生产环境可关闭
3. 敏感数据（如价格、库存）会在预览中显示，注意日志安全
4. 页面HTML预览限制在500字符，用于诊断页面结构问题
