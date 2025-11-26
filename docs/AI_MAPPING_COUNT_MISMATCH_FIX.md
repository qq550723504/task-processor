# AI映射数量不匹配问题修复

## 问题描述

在处理变体产品时，AI生成的SKU映射数量可能与实际变体数量不匹配：

```
WARN 根据AI映射构建SKC失败: AI映射数量(25)多于变体数量(24)，无法处理，使用默认映射
WARN ⚠️ 使用默认SKC构建器（备用方案）
```

### 问题影响

1. **回退到默认构建器**：当AI映射数量不匹配时，系统会回退到默认构建器
2. **缺少规格信息**：默认构建器不生成规格信息，导致SKU没有spec
3. **提交失败风险**：没有规格的SKU可能无法提交到TEMU平台

### 根本原因

- AI在分批处理时可能重复生成某个变体的映射
- AI可能误解变体数量，生成了额外的映射
- 之前的代码对数量不匹配采取"零容忍"策略，直接返回错误

## 解决方案

### 1. 智能容错机制

修改 `buildSkcsFromAIMapping` 方法，增加智能处理逻辑：

```go
// 如果差异在可接受范围内（≤2个），尝试智能处理
if diff <= 2 {
    sb.logger.Infof("差异在可接受范围内，尝试去重和修复...")
    if err := sb.removeDuplicateOrExcessMappings(aiMapping, variants); err != nil {
        return nil, fmt.Errorf("移除多余映射失败: %w", err)
    }
    sb.logger.Infof("✅ 成功处理多余映射，当前映射数量: %d", len(aiMapping.SkuList))
}
```

### 2. 去重和清理方法

新增 `removeDuplicateOrExcessMappings` 方法，处理多余映射：

**处理策略：**

1. **识别重复ASIN**：统计每个ASIN出现次数，找出重复项
2. **识别无效ASIN**：找出不在变体列表中的ASIN
3. **去重处理**：对于重复的ASIN，只保留第一个映射
4. **移除无效映射**：移除不在变体列表中的映射
5. **截断多余项**：如果处理后仍有多余，移除末尾的映射

**代码逻辑：**

```go
func (sb *SkuBuilder) removeDuplicateOrExcessMappings(aiMapping *AISkuMappingResponse, variants []*amazon.Product) error {
    // 1. 创建有效ASIN集合
    validAsins := make(map[string]bool)
    for _, variant := range variants {
        validAsins[variant.Asin] = true
    }

    // 2. 统计ASIN出现次数，找出重复和无效的ASIN
    asinCount := make(map[string]int)
    for _, sku := range aiMapping.SkuList {
        asinCount[sku.Asin]++
    }

    // 3. 过滤：移除重复和无效的映射
    var filteredSkus []AIGeneratedSku
    seenAsins := make(map[string]bool)
    
    for _, sku := range aiMapping.SkuList {
        // 跳过无效ASIN
        if !validAsins[sku.Asin] {
            continue
        }
        // 跳过重复ASIN（保留第一个）
        if seenAsins[sku.Asin] {
            continue
        }
        filteredSkus = append(filteredSkus, sku)
        seenAsins[sku.Asin] = true
    }

    // 4. 如果仍有多余，截断
    if len(filteredSkus) > len(variants) {
        filteredSkus = filteredSkus[:len(variants)]
    }

    aiMapping.SkuList = filteredSkus
    return nil
}
```

### 3. 改进日志输出

更新默认构建器的警告信息，明确说明风险：

```go
sb.logger.Warn("⚠️ 使用默认SKC构建器（备用方案）")
sb.logger.Warn("⚠️ 注意：默认构建器不生成规格信息，可能导致TEMU提交失败")
```

## 修复效果

### 修复前

```
WARN AI映射数量(25)多于变体数量(24)，无法处理
WARN 使用默认SKC构建器（备用方案）
WARN 默认构建器无法生成spec，SKU将没有规格信息
```

→ **结果**：回退到默认构建器，SKU缺少规格，可能提交失败

### 修复后

```
WARN ⚠️ AI映射数量(25)与变体数量(24)不匹配
WARN ⚠️ AI映射数量多于变体数量，差异: 1个
INFO 差异在可接受范围内，尝试去重和修复...
WARN ⚠️ 检测到重复的ASIN: B0XXXXX (出现2次)
INFO 🗑️ 移除重复映射: ASIN=B0XXXXX
INFO ✅ 移除了1个多余/重复的映射，剩余24个映射
INFO ✅ 成功处理多余映射，当前映射数量: 24
INFO AI辅助构建完成，创建了24个SKC
```

→ **结果**：智能修复映射，保留AI生成的规格信息，正常提交

## 容错范围

- **差异 ≤ 2个**：自动去重和修复
- **差异 > 2个**：返回错误，回退到默认构建器
- **原因**：差异过大可能表示AI理解错误，不应强制修复

## 相关文件

- `platforms/temu/handlers/sku_builder.go` - 主要修复逻辑
- `platforms/temu/handlers/sku_default_builder.go` - 改进日志输出

## 测试建议

1. **正常场景**：AI映射数量与变体数量完全匹配
2. **少量多余**：AI映射多1-2个（触发去重逻辑）
3. **重复ASIN**：AI为同一个ASIN生成多个映射
4. **无效ASIN**：AI生成了不存在的ASIN映射
5. **大量差异**：AI映射多3个以上（应该报错）

## 后续优化建议

1. **分析AI重复原因**：检查分批处理逻辑，避免重复生成
2. **改进AI提示词**：明确要求AI为每个变体生成唯一映射
3. **增加验证步骤**：在AI生成后立即验证ASIN唯一性
