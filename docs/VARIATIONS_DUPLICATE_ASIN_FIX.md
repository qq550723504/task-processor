# Variations 重复 ASIN 修复

## 问题描述

在产品 B08937KYGJ 的 variations 数据中存在重复的 ASIN，同一个 ASIN 被匹配到多个不同的颜色组合。

### 问题根源

在 `common/amazon/variations/matcher.go` 的 `ValuesMatch` 方法中，使用了包含匹配（`strings.Contains`），导致：

- "Black" 错误地匹配到 "Light Brown & Black"
- 同一个 ASIN 被多个不同的属性组合匹配

### 示例场景

```go
// 错误的匹配逻辑
ValuesMatch("Black", "Light Brown & Black") // 返回 true（错误！）

// 导致以下问题：
组合1: {color: "Black", size: "L=38.3\"(2pcs)"} -> 匹配到 B08937KYGJ（错误）
组合2: {color: "Light Brown & Black", size: "L=38.3\"(2pcs)"} -> 匹配到 B08937KYGJ（正确）
```

## 修复方案

参考 amazon-crawler-go 项目的修复方案，将包含匹配改为精确匹配。

### 修改文件

`go/task-processor/common/amazon/variations/matcher.go`

### 关键修改

#### 1. ValuesMatch 方法

**修改前：**
```go
func (m *Matcher) ValuesMatch(value1, value2 string) bool {
    norm1 := strings.ToLower(strings.TrimSpace(value1))
    norm2 := strings.ToLower(strings.TrimSpace(value2))

    // 直接匹配
    if norm1 == norm2 {
        return true
    }

    // 包含匹配（问题所在）
    if strings.Contains(norm1, norm2) || strings.Contains(norm2, norm1) {
        return true
    }

    // 移除特殊字符后匹配
    clean1 := strings.ReplaceAll(strings.ReplaceAll(norm1, "-", ""), " ", "")
    clean2 := strings.ReplaceAll(strings.ReplaceAll(norm2, "-", ""), " ", "")

    return clean1 == clean2 || strings.Contains(clean1, clean2) || strings.Contains(clean2, clean1)
}
```

**修改后：**
```go
func (m *Matcher) ValuesMatch(value1, value2 string) bool {
    norm1 := strings.ToLower(strings.TrimSpace(value1))
    norm2 := strings.ToLower(strings.TrimSpace(value2))

    // 精确匹配
    if norm1 == norm2 {
        return true
    }

    // 移除特殊字符后精确匹配（不使用包含匹配）
    clean1 := strings.ReplaceAll(strings.ReplaceAll(norm1, "-", ""), " ", "")
    clean2 := strings.ReplaceAll(strings.ReplaceAll(norm2, "-", ""), " ", "")

    return clean1 == clean2
}
```

#### 2. AttributesMatch 方法优化

**修改前：**
```go
// 直接匹配键名
if asinValue, exists := asinAttrs[key]; exists {
    if m.ValuesMatch(valueStr, asinValue) {
        matchCount++
    }
} else {
    // 尝试模糊匹配
    for _, asinValue := range asinAttrs {
        if m.ValuesMatch(valueStr, asinValue) {
            matchCount++
            break
        }
    }
}
```

**修改后：**
```go
// 尝试直接键名匹配
if asinValue, exists := asinAttrs[key]; exists {
    if m.ValuesMatch(valueStr, asinValue) {
        matchCount++
        continue
    }
}

// 如果直接匹配失败，尝试通过值匹配（用于attribute_1, attribute_2等通用键名）
// 但要确保值是精确匹配的，避免"Black"匹配"Light Brown & Black"
for _, asinValue := range asinAttrs {
    if m.ValuesMatch(valueStr, asinValue) {
        matchCount++
        break
    }
}
```

## 测试验证

创建了 `matcher_test.go` 测试文件，包含以下测试用例：

### 1. TestVariationsDuplicateASINFix
验证修复后不会出现重复的 ASIN 匹配：

```go
// 测试数据基于产品 B08937KYGJ
asinMapping := map[string]map[string]string{
    "B0DHXZD7RD": {"size": "L=17.4\" (1pcs)", "color": "Black"},
    "B082D965XB": {"size": "L=17.4\" (1pcs)", "color": "Light Brown & Black"},
    "B089373LC7": {"size": "L=38.3\"(2pcs)", "color": "Black"},
    "B08937KYGJ": {"size": "L=38.3\"(2pcs)", "color": "Light Brown & Black"},
}

// 验证每个组合只匹配一个唯一的 ASIN
```

### 2. TestAttributesMatchPrecision
验证属性匹配的精确性：
- 完全匹配 ✅
- 部分匹配返回 false ✅
- ASIN 有额外属性但 combo 的所有属性都匹配 ✅

### 3. TestValuesMatchPrecision
验证值匹配的精确性：
- 精确匹配 ✅
- 大小写不敏感 ✅
- **"Black" 不匹配 "Light Brown & Black"** ✅（关键修复）
- 特殊字符处理 ✅

## 测试结果

```bash
$ go test -v ./common/amazon/variations/matcher_test.go
=== RUN   TestVariationsDuplicateASINFix
    ✅ 组合1正确匹配: Black + L=38.3"(2pcs) -> B089373LC7
    ✅ 组合2正确匹配: Light Brown & Black + L=38.3"(2pcs) -> B08937KYGJ
    ✅ 组合3正确匹配: Black + L=17.4" (1pcs) -> B0DHXZD7RD
    ✅ 所有ASIN都是唯一匹配，没有重复
--- PASS: TestVariationsDuplicateASINFix (0.00s)

=== RUN   TestAttributesMatchPrecision
    ✅ 完全匹配测试通过
    ✅ 部分匹配测试通过（正确返回false）
    ✅ 额外属性测试通过
--- PASS: TestAttributesMatchPrecision (0.00s)

=== RUN   TestValuesMatchPrecision
    ✅ 精确匹配测试通过
    ✅ 大小写不敏感测试通过
    ✅ 包含匹配正确返回false（关键修复验证）
    ✅ 特殊字符处理测试通过
    ✅ 不同值测试通过
--- PASS: TestValuesMatchPrecision (0.00s)

PASS
```

## 影响范围

此修复影响所有使用 variations 匹配逻辑的地方：

1. **Amazon 产品变体提取**
   - `common/amazon/variations/extractor.go`
   - `common/amazon/variations/matcher.go`

2. **Temu 平台处理**
   - `platforms/temu/handlers/sku_ai_mapping.go`
   - `platforms/temu/handlers/variant_json_data_handler.go`

3. **Shein 平台处理**
   - `platforms/shein/modules/sale_attribute_preparation.go`
   - `platforms/shein/modules/has_spu_record_handler.go`

## 预期效果

修复后：
- ✅ 每个属性组合只匹配一个唯一的 ASIN
- ✅ "Black" 不会错误匹配到 "Light Brown & Black"
- ✅ 变体数据不会出现重复的 ASIN
- ✅ AI 生成的规格数据更加准确

## 相关参考

- amazon-crawler-go 项目的相同修复：`internal/extractor/variations.go`
- 测试文件：`internal/extractor/variations_fix_test.go`
