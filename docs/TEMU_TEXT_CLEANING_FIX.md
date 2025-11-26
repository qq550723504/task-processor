# TEMU文本清理修复

## 问题描述

从日志中发现两个TEMU API错误：

1. **错误 10000019**: "Chinese characters not allowed in text"
   - 产品数据中包含中文字符，TEMU API拒绝接受

2. **错误 10000138**: "Product name should not have a space before the mark ','"
   - 产品名称中逗号前有空格，不符合TEMU格式要求

## 根本原因

`common/utils/text_cleaner.go` 中的 `cleanSpecialCharacters` 函数使用了 `\p{L}` 正则表达式，这会匹配所有Unicode字母包括中文字符，导致中文字符过滤失效。

同时 `fixCommaSpacing` 函数只处理了逗号后的空格，没有处理逗号前的空格。

## 修复方案

### 1. 修复中文字符过滤

将 `cleanSpecialCharacters` 中的正则表达式从：
```go
re := regexp.MustCompile(`[^\p{L}\p{N}\s.,!?()\-_+=/:;"'\[\]{}|]+`)
```

改为：
```go
re := regexp.MustCompile(`[^a-zA-Z0-9\s.,!?()\-_+=/:;"'\[\]{}|]+`)
```

这样只保留ASCII字母和数字，确保中文字符被移除。

### 2. 修复逗号空格问题

更新 `fixCommaSpacing` 函数，添加移除逗号前空格的逻辑：
```go
// 1. 移除逗号前的空格
re := regexp.MustCompile(`\s+,`)
text = re.ReplaceAllString(text, ",")

// 2. 确保逗号后有空格
re = regexp.MustCompile(`,(\S)`)
text = re.ReplaceAllString(text, ", $1")

// 3. 清理逗号后的多余空格
re = regexp.MustCompile(`,\s{2,}`)
text = re.ReplaceAllString(text, ", ")
```

## 测试验证

创建了完整的单元测试 `common/utils/text_cleaner_test.go`，覆盖以下场景：
- 移除中文字符
- 移除逗号前的空格
- 处理日志中的复杂案例
- 移除表情符号
- 混合中英文处理

所有测试通过 ✅

## 影响范围

此修复会影响所有通过 `utils.CleanProductTitle()` 清理的产品标题，包括：
- TEMU产品提交
- 产品名称验证
- 所有使用该工具函数的地方

## 预期效果

修复后，产品标题将：
1. 完全移除中文字符，避免错误 10000019
2. 自动修正逗号前的空格，避免错误 10000138
3. 保持其他格式要求（括号前空格等）
