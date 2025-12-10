# 敏感词处理器使用指南

## 概述

统一的敏感词处理器,支持三种处理模式:
- **拦截模式** - 发现敏感词直接拦截,阻止发布
- **清理模式** - 发现敏感词自动替换,继续发布
- **警告模式** - 发现敏感词仅记录日志,不影响发布

## 使用方式

### 1. 拦截模式 (推荐用于前期过滤)

```go
// 创建敏感词过滤器
filter, err := NewSensitiveWordsFilter("config/sensitive_words_shein.json")
if err != nil {
    return err
}

// 创建拦截模式处理器
handler := modules.NewSensitiveWordsBlockHandler(filter)
pipeline.AddHandler(handler)
```

**行为**: 检测到敏感词 → 返回 `FilteredError` → 任务标记为已过滤

### 2. 清理模式 (推荐用于自动修复)

```go
// 创建清理模式处理器
handler := modules.NewSensitiveWordsCleanHandler(filter)
pipeline.AddHandler(handler)
```

**行为**: 检测到敏感词 → 自动替换 → 继续处理

### 3. 警告模式 (推荐用于监控)

```go
// 创建警告模式处理器
handler := modules.NewSensitiveWordsWarnHandler(filter)
pipeline.AddHandler(handler)
```

**行为**: 检测到敏感词 → 记录日志 → 继续处理

## 高级用法

### 自定义处理模式

```go
// 直接指定模式
handler := modules.NewSensitiveWordsProcessor(modules.ModeClean, filter)
```

### 组合使用

```go
// 先拦截硬编码敏感词
pipeline.AddHandler(modules.NewSensitiveWordsBlockHandler(hardcodedFilter))

// 再清理配置文件中的敏感词
pipeline.AddHandler(modules.NewSensitiveWordsCleanHandler(configFilter))
```

## 配置文件格式

```json
{
  "version": "1.0.0",
  "platform": "shein",
  "static_words": [
    "925 Sterling",
    "Real Gold"
  ],
  "dynamic_patterns": [
    {
      "pattern": "\\d+K Gold",
      "replacement": "Gold Tone"
    }
  ]
}
```

## 处理流程

```
产品数据
    ↓
检查敏感词
    ↓
┌───────────┬───────────┬───────────┐
│ 拦截模式   │ 清理模式   │ 警告模式   │
├───────────┼───────────┼───────────┤
│ 返回错误   │ 自动替换   │ 记录日志   │
│ 阻止发布   │ 继续发布   │ 继续发布   │
└───────────┴───────────┴───────────┘
```

## 优势

1. **统一接口** - 一个处理器支持多种模式
2. **灵活配置** - 根据需求选择处理策略
3. **易于维护** - 代码集中,逻辑清晰
4. **向后兼容** - 提供便捷构造函数

## 迁移指南

### 旧代码
```go
// 旧的拦截处理器
handler := modules.NewSensitiveWordsHandler(filter)

// 旧的清理处理器
handler := modules.NewCleanSensitiveWordsHandler()
```

### 新代码
```go
// 新的拦截处理器
handler := modules.NewSensitiveWordsBlockHandler(filter)

// 新的清理处理器
handler := modules.NewSensitiveWordsCleanHandler(filter)
```

## 注意事项

1. 清理模式需要 `ProductData` 已准备好
2. 拦截模式会返回 `FilteredError`,需要正确处理
3. 警告模式不影响流程,适合监控和统计
