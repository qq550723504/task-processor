# SHEIN 敏感词过滤 - 快速开始

## ✅ 已完成集成

新的敏感词过滤系统已经集成到 SHEIN 产品发布流程中，**无需额外配置即可使用**。

## 工作流程

```
产品发布流程
    ↓
到达"清理敏感词"步骤
    ↓
1️⃣ 检查硬编码敏感词（如 "925 Sterling"）
    ├─ 发现 → ❌ 拦截发布
    └─ 未发现 → 继续
    ↓
2️⃣ 清理配置文件敏感词
    ↓
3️⃣ 继续发布
```

## 硬编码敏感词（会拦截发布）

### 英语
- `925 Sterling` / `925Sterling` - 925纯银
- `fake` - 假货
- `counterfeit` - 仿冒品
- `replica` - 复制品

### 中文
- `假货`
- `仿品`

## 配置文件敏感词（会自动清理）

位置：`config/sensitive_words_shein.json`

当前已配置的敏感词会被自动清理，不会拦截发布。

## 查看运行日志

### 拦截日志示例
```
⚠️ 产品包含硬编码敏感词，拦截发布
asin: B08XYZ123
title: 925 Sterling Silver Ring
sensitive_words: map[en:[(?i)925\s*sterling]]
```

### 正常清理日志
```
✅ 敏感词清理处理完成，共处理了 3 个字段
```

## 常见问题

### Q: 如何添加新的拦截敏感词？
A: 编辑 `platforms/shein/sensitive_words_filter.go` 中的 `initHardcodedWords()` 函数，然后重新编译。

### Q: 如何添加新的清理敏感词？
A: 编辑 `config/sensitive_words_shein.json`，支持热更新，无需重启。

### Q: 如何测试敏感词过滤？
A: 运行 `go test ./platforms/shein -run TestSensitiveWordsFilter -v`

## 更多文档

- [完整功能文档](./SENSITIVE_WORDS_README.md)
- [集成指南](./INTEGRATION_GUIDE.md)
- [使用示例](./sensitive_words_example.go)
