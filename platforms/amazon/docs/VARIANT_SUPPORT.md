# Amazon 变体产品支持

## 📋 功能概述

实现了对 Amazon 变体产品（Variations）的完整支持，可以将1688的多规格产品自动转换为 Amazon 的变体产品。

## 🎯 什么是变体产品？

变体产品是指同一产品的不同规格版本，例如：
- 不同颜色的T恤（红色、蓝色、绿色）
- 不同尺寸的鞋子（38、39、40、41）
- 不同颜色和尺寸的组合（红色-M、蓝色-L）

在 Amazon 上，变体产品由一个**父产品**和多个**子变体**组成。

## 🔄 数据流程

```
1688多规格产品
    ↓
VariantExtractor (提取变体信息)
    ↓
检测变体主题 (SizeColor/Size/Color)
    ↓
VariantHandler (处理变体)
    ↓
创建父产品 (容器)
    ↓
创建子变体 (每个规格)
    ↓
设置价格和库存
    ↓
✅ 变体产品上架完成
```

## 📦 核心组件

### 1. VariantExtractor (变体提取器)
**文件**: `platforms/amazon/utils/variant_extractor.go`

**功能**: 从1688数据中提取变体信息

**方法**:
```go
// 提取变体数据
func (e *VariantExtractor) ExtractVariants(
    productData map[string]interface{},
) (*VariantData, error)

// 构建子变体列表
func (e *VariantExtractor) BuildVariantChildren(
    variantData *VariantData,
    parentSKU string,
) ([]VariantChildData, error)
```

**支持的1688字段**:
- `skuInfos` - SKU信息列表
- `skuList` - SKU列表
- `productSKUPropertyList` - 产品SKU属性列表
- `specAttrs` - 规格属性
- `skuProps` - SKU属性

### 2. VariantHandler (变体处理器)
**文件**: `platforms/amazon/handlers/variant_handler.go`

**功能**: 协调变体产品的创建流程

**处理步骤**:
1. 提取变体信息
2. 判断是否为变体产品
3. 创建父产品（作为容器）
4. 创建所有子变体
5. 为每个子变体设置价格和库存

### 3. 变体数据模型
**文件**: `platforms/amazon/models_variant.go`

**核心结构**:
```go
// 变体产品
type VariantProduct struct {
    ParentSKU      string
    VariationTheme string  // 变体主题
    ParentData     map[string]interface{}
    Children       []VariantChild
}

// 子变体
type VariantChild struct {
    SKU           string
    VariationData map[string]string  // 如: {color: "Red", size: "M"}
    Price         float64
    Quantity      int
    Images        []string
}
```

## 🎨 支持的变体主题

### 1. SizeColor (尺寸+颜色)
**适用**: 服装、鞋类
**属性**: size, color
**示例**: 红色-M, 蓝色-L, 绿色-XL

### 2. Size (仅尺寸)
**适用**: 单色服装、配件
**属性**: size
**示例**: S, M, L, XL

### 3. Color (仅颜色)
**适用**: 单尺寸产品
**属性**: color
**示例**: 红色, 蓝色, 绿色

### 4. Style (款式)
**适用**: 其他类型变体
**属性**: style_name
**示例**: 款式A, 款式B

## 📝 1688 数据示例

### 输入 (1688多规格产品)

```json
{
  "subject": "时尚T恤",
  "skuInfos": [
    {
      "specAttrs": [
        {"name": "颜色", "value": "红色"},
        {"name": "尺码", "value": "M"}
      ],
      "price": "29.90",
      "quantity": 100,
      "image": "https://example.com/red-m.jpg"
    },
    {
      "specAttrs": [
        {"name": "颜色", "value": "蓝色"},
        {"name": "尺码", "value": "L"}
      ],
      "price": "29.90",
      "quantity": 150,
      "image": "https://example.com/blue-l.jpg"
    }
  ]
}
```

### 输出 (Amazon变体结构)

```
父产品: 1688-123456
├── 子变体1: 1688-123456-V1
│   ├── color: Red
│   ├── size: M
│   ├── price: $5.46
│   └── quantity: 100
└── 子变体2: 1688-123456-V2
    ├── color: Blue
    ├── size: L
    ├── price: $5.46
    └── quantity: 150
```

## 🔧 使用方式

### 在 Pipeline 中使用

```go
// 创建变体处理器
variantHandler := handlers.NewVariantHandler(apiClient)

// 添加到Pipeline（在Listing创建之后）
pipeline.AddHandler(listingHandler)
pipeline.AddHandler(variantHandler)  // 自动检测并处理变体

// 执行
ctx := amazon.NewTaskContext()
ctx.SetData("raw_product_data", productData)
pipeline.Process(ctx)

// 检查结果
isVariant, _ := ctx.GetData("is_variant_product")
if isVariant.(bool) {
    childrenCount, _ := ctx.GetData("variant_children_count")
    fmt.Printf("创建了 %d 个子变体\n", childrenCount)
}
```

### 手动处理变体

```go
// 1. 提取变体信息
extractor := utils.NewVariantExtractor()
variantData, err := extractor.ExtractVariants(productData)

if variantData != nil {
    // 2. 这是变体产品
    fmt.Printf("变体主题: %s\n", variantData.Theme)
    fmt.Printf("变体数量: %d\n", len(variantData.SKUs))
    
    // 3. 构建子变体
    children, _ := extractor.BuildVariantChildren(variantData, "PARENT-SKU")
    
    // 4. 创建每个子变体
    for _, child := range children {
        fmt.Printf("SKU: %s, 属性: %v\n", child.SKU, child.VariationData)
    }
}
```

## 🎯 变体主题检测逻辑

```go
// 自动检测变体主题
func determineVariationTheme(attributes []string) string {
    hasColor := false
    hasSize := false
    
    for _, attr := range attributes {
        if contains(attr, "color", "颜色") {
            hasColor = true
        }
        if contains(attr, "size", "尺码", "尺寸") {
            hasSize = true
        }
    }
    
    if hasColor && hasSize {
        return "SizeColor"  // 颜色+尺寸
    } else if hasSize {
        return "Size"       // 仅尺寸
    } else if hasColor {
        return "Color"      // 仅颜色
    }
    
    return "Style"  // 默认款式
}
```

## 📊 SKU 命名规则

- **父产品**: `1688-{ProductID}`
- **子变体**: `1688-{ProductID}-V{序号}`

示例:
- 父产品: `1688-123456`
- 子变体1: `1688-123456-V1`
- 子变体2: `1688-123456-V2`
- 子变体3: `1688-123456-V3`

## ⚙️ 价格和库存处理

### 价格策略
每个子变体独立定价：
1. 从1688 SKU中提取价格
2. 货币转换（CNY → USD，汇率 1:0.14）
3. 应用加价策略（+30%）

### 库存管理
每个子变体独立库存：
- 从1688 SKU中提取库存数量
- 直接设置到对应的子变体

## 🚨 注意事项

### 1. 父产品特点
- 父产品不能单独销售
- 父产品不设置价格和库存
- 父产品包含所有共同属性

### 2. 子变体特点
- 每个子变体有独立的SKU
- 每个子变体有独立的价格和库存
- 子变体必须包含变体属性值

### 3. 变体限制
- Amazon 最多支持 2000 个子变体
- 建议每个父产品不超过 100 个子变体
- 变体主题一旦设置不能更改

### 4. 图片要求
- 父产品需要主图
- 每个子变体最好有独立的图片
- 图片应清晰展示变体差异

## 🔍 调试和日志

```
[VariantHandler] 开始处理变体产品
[VariantExtractor] 从字段 skuInfos 提取到 3 个SKU
[VariantExtractor] 提取到 3 个变体，主题: SizeColor
[VariantHandler] 检测到变体产品，主题: SizeColor
[VariantHandler] 创建父产品: 1688-123456
[VariantHandler] 父产品创建成功
[VariantHandler] 创建子变体 [1/3]: 1688-123456-V1
[VariantHandler] 创建子变体 [2/3]: 1688-123456-V2
[VariantHandler] 创建子变体 [3/3]: 1688-123456-V3
[VariantHandler] 变体处理完成，成功创建 3/3 个子变体
```

## 📚 相关文档

- [Amazon Variations Guide](https://sellercentral.amazon.com/gp/help/201958220)
- [Variation Themes](https://sellercentral.amazon.com/gp/help/201958220)
- [COMPLETE_SUMMARY.md](./COMPLETE_SUMMARY.md)

## 🎉 总结

变体产品支持功能已完成，可以自动：
- ✅ 检测1688多规格产品
- ✅ 提取变体信息
- ✅ 确定变体主题
- ✅ 创建父产品和子变体
- ✅ 设置独立的价格和库存

支持服装、鞋类等多规格产品的自动上架！
