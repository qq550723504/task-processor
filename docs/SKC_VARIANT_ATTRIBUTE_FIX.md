# SKC构建失败修复 - 变体属性缺失问题

## 问题描述

当Amazon产品只有Size变体，但SHEIN平台要求该分类必须有Color作为主规格时，SKC构建会失败，错误信息：
```
❌ 未找到属性ID 27, 属性值 'Multi-Color' 的匹配变体
SKC列表构建结果为空
```

## 根本原因

1. **平台要求**：SHEIN分类2632要求Color (27)作为主规格属性
2. **产品实际情况**：Amazon产品只有Size变体（Small, Medium），没有Color变体
3. **AI正确行为**：AI生成了"Multi-Color"作为统一的颜色值（符合平台要求）
4. **问题所在**：AI生成的每个variant的`attributes`字段中只包含Size属性，没有Color属性

### 日志证据

```
17:04:17 - 源数据: Attributes=map[size:Small]  ✅ 只有尺寸
17:07:50 - 系统创建: "Multi-Color" (ID: 142201256) ✅ 主规格值
17:07:53 - 变体匹配失败: 
  变体[1] ASIN=B0FD3NGJTM: Size = 'Small'  ❌ 没有Color
  变体[2] ASIN=B0FD3H284C: Size = 'Medium' ❌ 没有Color
17:07:53 - 结果: 0个SKC被创建
```

## 解决方案

在SKC构建的多变体流程中，添加变体属性修复步骤：

### 修复逻辑

```go
// 在属性值ID映射完成后，修复变体属性
func (b *SKCBuilder) ensureVariantsHaveRequiredAttributes(ctx *TaskContext, strategy *AttributeStrategy) {
    // 1. 获取主规格和次规格的属性名
    primaryAttrName := b.getAttributeNameForVariant(strategy.PrimaryAttribute.AttrID)
    
    // 2. 检查每个变体
    for i := range ctx.SaleSpecResult.Variants {
        variant := &ctx.SaleSpecResult.Variants[i]
        
        // 3. 如果变体缺少主规格属性，添加默认值
        if !b.variantHasAttribute(variant, primaryAttrName) {
            if len(strategy.PrimaryAttribute.AttrValue) > 0 {
                defaultValue := strategy.PrimaryAttribute.AttrValue[0].Value
                variant.Attributes[primaryAttrName] = defaultValue
                logrus.Infof("✅ 为变体 ASIN=%s 添加主规格属性: %s = %s", 
                    variant.ASIN, primaryAttrName, defaultValue)
            }
        }
    }
}
```

### 修复时机

在`buildMultiVariantSKCList`函数中，在属性值ID映射完成后立即执行：

```go
// 1. 预处理属性值ID映射
mappingRelations, err := b.attributeMapper.MapAttributeValuesToSheinIDs(ctx, &strategy)
...

// 1.5 修复变体属性：确保所有变体都包含主规格和次规格属性
b.ensureVariantsHaveRequiredAttributes(ctx, &strategy)

// 2. 开始构建SKC列表
for i := 0; i < len(strategy.PrimaryAttribute.AttrValue); i++ {
    ...
}
```

## 修复效果

修复后，当遇到类似情况时：

1. **AI生成**：
   - saleAttributes: Color=["Multi-Color"], Size=["Small", "Medium"]
   - variants[0]: {ASIN: "B0FD3NGJTM", attributes: {Size: "Small"}}
   - variants[1]: {ASIN: "B0FD3H284C", attributes: {Size: "Medium"}}

2. **自动修复**：
   - variants[0]: {ASIN: "B0FD3NGJTM", attributes: {**Color: "Multi-Color"**, Size: "Small"}}
   - variants[1]: {ASIN: "B0FD3H284C", attributes: {**Color: "Multi-Color"**, Size: "Medium"}}

3. **变体匹配成功**：
   - 主规格Color="Multi-Color"可以匹配到所有2个变体
   - 次规格Size可以区分不同的SKU

4. **SKC构建成功**：
   - 创建1个SKC（Color=Multi-Color）
   - 包含2个SKU（Size=Small, Size=Medium）

## 适用场景

此修复适用于以下场景：

1. **平台强制属性**：平台要求某个属性作为主规格，但产品实际没有该属性的变体
2. **单一属性值**：所有变体共享同一个主规格属性值（如统一颜色）
3. **属性适配**：由于分类限制，原本的主规格被替换为另一个属性

## 相关文件

- `platforms/shein/modules/skc_builder.go` - SKC构建器，添加了变体属性修复逻辑
- `platforms/shein/modules/variant_matcher.go` - 变体匹配器，负责根据属性值查找变体

## 测试建议

测试以下场景：

1. ✅ 只有Size变体，平台要求Color主规格
2. ✅ 只有Color变体，平台要求Size主规格
3. ✅ 同时有Color和Size变体，正常情况
4. ✅ 单变体产品，平台要求多个属性

## 相关修复

### 修复2: 单变体SKU构建找不到产品信息

**问题**：单变体产品构建SKU时，如果ASIN在`ctx.Variants`中找不到，会直接报错。

**原因**：单变体产品可能没有`Variants`数据，或者ASIN不在`Variants`列表中。

**解决方案**：在`BuildSKUListForSingleVariant`中添加备选逻辑：
```go
// 如果在变体中找不到，使用主产品信息作为备选
if productInfo == nil {
    logrus.Warnf("在变体中未找到ASIN %s 的产品信息，使用主产品信息", variant.ASIN)
    if ctx.AmazonProduct != nil {
        productInfo = ctx.AmazonProduct
    } else {
        return nil, fmt.Errorf("未找到ASIN %s 对应的产品信息，且主产品信息也为空", variant.ASIN)
    }
}
```

**修复文件**：`platforms/shein/modules/sku_builder.go`

## 日期

2025-11-23
