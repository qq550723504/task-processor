# SHEIN 敏感词过滤系统集成指南

## 集成状态 ✅

新的敏感词过滤系统已成功集成到 SHEIN 产品发布流程中。

## 工作原理

### 1. 双层过滤机制

系统采用**双层过滤**策略：

```
第一层：硬编码敏感词检查（拦截模式）
   ↓ 如果包含硬编码敏感词 → 直接拦截，不发布
   ↓ 如果通过检查
第二层：配置文件敏感词清理（清理模式）
   ↓ 自动清理可配置的敏感词
   ↓ 继续发布流程
```

### 2. 集成位置

在 Pipeline 中的位置：

```
构建最终发品数据 (BuildSpuHandler)
    ↓
清理敏感词 (CleanSensitiveWordsHandler) ← 集成点
    ├─ 硬编码敏感词检查（新增）
    └─ 配置文件敏感词清理（原有）
    ↓
发布产品 (PublishProductHandler)
```

## 代码集成详情

### 1. Pipeline 初始化

文件：`platforms/shein/pipeline.go`

```go
// 清理敏感词（集成硬编码敏感词检查）
sensitiveFilter, err := NewSensitiveWordsFilter("config/sensitive_words_shein.json")
if err != nil {
    logrus.WithError(err).Warn("初始化敏感词过滤器失败，将使用默认清理模式")
    pipeline.AddHandler(modules.NewCleanSensitiveWordsHandler())
} else {
    pipeline.AddHandler(modules.NewCleanSensitiveWordsHandlerWithFilter(sensitiveFilter))
}
```

### 2. Handler 增强

文件：`platforms/shein/modules/clean_sensitive_words_handler.go`

```go
// 如果配置了新的敏感词过滤器，先进行检查（硬编码敏感词拦截）
if h.sensitiveWordsFilter != nil && ctx.AmazonProduct != nil {
    title := ctx.AmazonProduct.Title
    description := ctx.AmazonProduct.Description
    languages := []string{"en", "zh", "es", "fr", "de"}

    hasSensitive, foundWords := h.sensitiveWordsFilter.CheckProduct(title, description, languages)
    if hasSensitive {
        // 拦截发布
        return NewFilteredError(fmt.Sprintf("产品包含硬编码敏感词: %v", foundWords))
    }
}
```

## 硬编码敏感词列表

以下敏感词已硬编码，**无法通过配置文件修改**：

### 英语 (en)
- `(?i)925\s*sterling` - 925 Sterling Silver
- `(?i)\bfake\b` - 假货
- `(?i)\bcounterfeit\b` - 仿冒品
- `(?i)\breplica\b` - 复制品

### 中文 (zh)
- `假货`
- `仿品`

## 配置文件敏感词

配置文件：`config/sensitive_words_shein.json`

可以通过配置文件添加更多敏感词，这些敏感词会被**自动清理**而不是拦截。

## 行为差异

| 敏感词类型 | 检测到后的行为 | 可否修改 | 配置位置 |
|-----------|--------------|---------|---------|
| 硬编码敏感词 | **拦截发布** | ❌ 需要修改代码 | `sensitive_words_filter.go` |
| 配置文件敏感词 | **自动清理** | ✅ 可热更新 | `config/sensitive_words_shein.json` |

## 日志示例

### 拦截日志

```
⚠️ 产品包含硬编码敏感词，拦截发布
asin: B08XYZ123
title: 925 Sterling Silver Ring
sensitive_words: map[en:[(?i)925\s*sterling]]
```

### 清理日志

```
✅ 敏感词清理处理完成，共处理了 3 个字段
```

## 测试验证

运行测试确保集成正常：

```bash
# 测试敏感词过滤器
go test ./platforms/shein -run TestSensitiveWordsFilter -v

# 测试完整 Pipeline
go test ./platforms/shein -v
```

## 监控指标

建议监控以下指标：

1. **拦截率**：因硬编码敏感词被拦截的产品数量
2. **清理率**：被自动清理敏感词的产品数量
3. **误拦截率**：需要人工审核的拦截案例

## 故障排查

### 问题：敏感词过滤器初始化失败

**症状**：
```
初始化敏感词过滤器失败，将使用默认清理模式
```

**原因**：
- 配置文件不存在或格式错误
- 配置文件路径不正确

**解决方案**：
1. 检查 `config/sensitive_words_shein.json` 是否存在
2. 验证 JSON 格式是否正确
3. 查看详细错误日志

### 问题：产品被意外拦截

**症状**：
```
产品包含硬编码敏感词: map[en:[(?i)925\s*sterling]]
```

**原因**：
- 产品标题或描述包含硬编码敏感词

**解决方案**：
1. 检查产品信息是否确实包含敏感词
2. 如果是误判，需要修改硬编码规则
3. 考虑将该词从硬编码移到配置文件（改为清理模式）

## 扩展开发

### 添加新的硬编码敏感词

编辑 `platforms/shein/sensitive_words_filter.go`：

```go
func initHardcodedWords() map[string][]string {
    return map[string][]string{
        "en": {
            "(?i)925\\s*sterling",
            "(?i)\\bfake\\b",
            // 添加新的敏感词
            "(?i)\\bnew_sensitive_word\\b",
        },
    }
}
```

### 添加新的配置文件敏感词

编辑 `config/sensitive_words_shein.json` 或使用 API：

```go
filter.ReloadConfig() // 热更新
```

## 相关文档

- [敏感词过滤系统详细文档](./SENSITIVE_WORDS_README.md)
- [Pipeline 架构说明](./pipeline.go)
- [测试用例](./sensitive_words_filter_test.go)
