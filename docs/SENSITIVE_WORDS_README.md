# SHEIN 敏感词过滤系统

## 概述

SHEIN 敏感词过滤系统支持**硬编码敏感词**与**配置文件敏感词**结合使用，提供灵活且安全的敏感词检测能力。

## 特性

- ✅ **硬编码敏感词**：优先级最高，无法通过配置文件修改
- ✅ **配置文件敏感词**：支持静态词汇和动态正则表达式
- ✅ **多语言支持**：支持英语、中文、西班牙语、法语、德语等
- ✅ **热更新**：支持运行时重新加载配置文件
- ✅ **线程安全**：使用读写锁保证并发安全

## 架构设计

### 1. 敏感词优先级

```
硬编码敏感词（最高优先级）
    ↓
配置文件静态敏感词
    ↓
配置文件动态正则表达式
```

### 2. 文件结构

```
platforms/shein/
├── sensitive_words_filter.go          # 核心过滤器实现
├── sensitive_words_filter_test.go     # 单元测试
├── sensitive_words_example.go         # 使用示例
└── modules/
    └── sensitive_words_handler.go     # Pipeline Handler
```

## 使用方法

### 1. 初始化过滤器

```go
import "task-processor/platforms/shein"

// 初始化过滤器
filter, err := shein.NewSensitiveWordsFilter("config/sensitive_words_shein.json")
if err != nil {
    log.Fatal(err)
}
```

### 2. 检查单个文本

```go
text := "This is a 925 Sterling Silver ring"
hasSensitive, words := filter.CheckText(text, "en")
if hasSensitive {
    fmt.Printf("发现敏感词: %v\n", words)
}
```

### 3. 检查产品信息

```go
title := "Beautiful 925 Sterling Silver Necklace"
description := "High quality sterling silver jewelry"
languages := []string{"en", "zh"}

hasSensitive, foundWords := filter.CheckProduct(title, description, languages)
if hasSensitive {
    fmt.Printf("产品包含敏感词: %v\n", foundWords)
}
```

### 4. 在 Pipeline 中使用

```go
import (
    "task-processor/platforms/shein"
    "task-processor/platforms/shein/modules"
)

// 初始化过滤器
filter, _ := shein.NewSensitiveWordsFilter("config/sensitive_words_shein.json")

// 创建 Handler
handler := modules.NewSensitiveWordsHandler(filter)

// 添加到 Pipeline
pipeline.AddHandler(handler)
```

### 5. 热更新配置

```go
// 重新加载配置文件
if err := filter.ReloadConfig(); err != nil {
    log.Printf("重新加载配置失败: %v", err)
}
```

## 配置文件格式

配置文件位置：`config/sensitive_words_shein.json`

```json
{
  "static_words": {
    "en": ["brand", "trademark", "logo"],
    "zh": ["品牌", "商标"]
  },
  "dynamic_words": {
    "en": [
      "(?i)\\b(nike|adidas)\\b",
      "(?i)925\\s*sterling"
    ]
  },
  "last_updated": "2025-12-03T10:00:00Z",
  "version": "1.0.0",
  "platform": "shein"
}
```

## 硬编码敏感词

以下敏感词已硬编码到系统中，**无法通过配置文件修改**：

### 英语 (en)
- `(?i)925\s*sterling` - 匹配 "925 Sterling"、"925Sterling" 等
- `(?i)\bfake\b` - 匹配 "fake"
- `(?i)\bcounterfeit\b` - 匹配 "counterfeit"
- `(?i)\breplica\b` - 匹配 "replica"

### 中文 (zh)
- `假货`
- `仿品`

## 添加新的硬编码敏感词

编辑 `platforms/shein/sensitive_words_filter.go` 中的 `initHardcodedWords()` 函数：

```go
func initHardcodedWords() map[string][]string {
    return map[string][]string{
        "en": {
            "(?i)925\\s*sterling",
            "(?i)\\bfake\\b",
            // 添加新的敏感词
            "(?i)\\bnew_word\\b",
        },
        "zh": {
            "假货",
            "仿品",
            // 添加新的中文敏感词
            "新敏感词",
        },
    }
}
```

## 运行测试

```bash
# 运行所有测试
go test ./platforms/shein -v

# 运行特定测试
go test ./platforms/shein -run TestSensitiveWordsFilter_CheckText -v
```

## 性能优化

1. **正则表达式预编译**：所有正则表达式在加载时预编译，避免运行时编译开销
2. **读写锁**：使用 `sync.RWMutex` 支持高并发读取
3. **缓存机制**：编译后的正则表达式缓存在内存中

## 注意事项

1. 硬编码敏感词优先级最高，会首先检查
2. 配置文件支持热更新，但硬编码敏感词需要重新编译
3. 正则表达式使用 `(?i)` 前缀表示不区分大小写
4. 建议定期审查和更新敏感词列表

## 常见问题

### Q: 如何添加临时敏感词？
A: 在配置文件的 `static_words` 中添加，然后调用 `ReloadConfig()` 热更新。

### Q: 硬编码敏感词可以删除吗？
A: 不可以通过配置文件删除，需要修改代码并重新编译。

### Q: 支持哪些语言？
A: 目前支持 en、zh、es、fr、de 等，可以在配置文件中扩展。

### Q: 如何查看当前配置？
A: 使用 `filter.GetConfig()` 方法获取当前配置。
