# 用户输入规格验证修复

## 问题描述

从日志中发现SKU映射处理器报警告：
```
WARN[2025-11-23 12:05:36] ❌ SKU[0] parent_spec_id 1001 不存在于模板中，跳过该规格
WARN[2025-11-23 12:05:36] ❌ SKU[0] parent_spec_id 3001 不存在于模板中，跳过该规格
INFO[2025-11-23 12:05:36] 构建了0个parent_spec_id的有效spec_id映射
```

## 根本原因

当使用 `user_input_parent_spec_list`（用户自定义规格）时：

1. `convertUserInputSpecsToGoodsSpecProperties` 函数将用户输入规格转换为 `GoodsSpecProperty`
2. 但转换时 `Values` 数组为空（因为用户输入规格没有预定义值）
3. `validateAndFixAIResponse` 函数在构建 `parentSpecToValidSpecIDs` 映射时，因为 `Values` 为空而无法添加任何有效的 `spec_id`
4. 结果导致所有SKU的规格都被标记为"不存在于模板中"并被跳过

## 修复方案

在 `validateAndFixAIResponse` 函数中添加对用户输入规格的特殊处理：

### 1. 识别用户输入规格
```go
userInputSpecs := make(map[string]bool) // 标记哪些是用户输入规格

for _, specProp := range temuSpecProperties {
    if specProp.ParentSpecID != "" {
        // 如果Values为空，说明这是用户输入规格
        if len(specProp.Values) == 0 {
            userInputSpecs[specProp.ParentSpecID] = true
            continue
        }
        // ... 正常处理有预定义值的规格
    }
}
```

### 2. 跳过用户输入规格的spec_id验证
```go
for _, spec := range sku.Spec {
    // 如果是用户输入规格，直接接受AI提供的spec_id和spec_name
    if userInputSpecs[spec.ParentSpecID] {
        validSpecs = append(validSpecs, types.SpecInfo{
            SpecID:         spec.SpecID,
            SpecName:       spec.SpecName,
            ParentSpecID:   spec.ParentSpecID,
            ParentSpecName: parentSpecNames[spec.ParentSpecID],
        })
        continue
    }
    // ... 正常验证有预定义值的规格
}
```

## 逻辑说明

对于用户输入规格（如自定义的颜色、尺寸等）：
- 没有预定义的 `spec_id` 列表
- AI会根据Amazon变体数据生成合适的 `spec_id` 和 `spec_name`
- 这些值应该被直接接受，不需要与模板进行验证

对于模板预定义规格：
- 有固定的 `spec_id` 列表
- 需要验证AI生成的 `spec_id` 是否在允许的列表中
- 如果不在，尝试通过 `spec_name` 匹配

## 影响范围

此修复影响所有使用 `user_input_parent_spec_list` 的产品，特别是：
- 没有标准模板的类目
- 使用自定义规格的产品
- 需要灵活规格定义的场景

## 预期效果

修复后：
1. 用户输入规格的 `parent_spec_id` 将被正确识别
2. AI生成的 `spec_id` 和 `spec_name` 将被接受
3. SKU规格不会被错误地跳过
4. 产品可以正常提交到TEMU
