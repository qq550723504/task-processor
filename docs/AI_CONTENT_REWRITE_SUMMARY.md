# AI内容重构功能 - 快速指南

## 功能说明

为TEMU产品添加了AI内容重构功能，自动优化产品标题、描述和要点。

## 核心特性

### 1. 避免儿童产品描述
- 系统会自动过滤所有儿童相关词汇
- 即使原Amazon产品包含儿童关键词，也会改写为通用场景
- 确保所有产品文案聚焦于成人或通用使用场景

### 2. 智能内容优化
- **标题**：20-200字符，包含核心关键词
- **描述**：200-2000字符，结构清晰
- **要点**：3-6个，每个15-120字符

### 3. 自动降级
- 如果OpenAI未配置或调用失败，自动跳过AI重构
- 不影响整个产品发布流程

## 配置方法

在配置文件中添加OpenAI设置：

```yaml
openai:
  api_key: "your-api-key"
  model: "gpt-4"  # 或 gpt-3.5-turbo
  base_url: "https://api.openai.com/v1"
  timeout: 60
```

## Pipeline位置

```
18. 模板查询
19. AI内容重构 ← 新增
20. 产品名称验证和清理
21. 产品要点验证和优化
22. 产品描述验证和优化
23. 构建SPU
```

## 示例效果

### 原Amazon产品（包含儿童关键词）
```
标题: Kids Building Blocks Set 100 Pieces for Children Ages 3+
```

### AI重构后（移除儿童相关描述）
```
标题: 创意积木套装 100块 益智拼装玩具 多功能组合
描述: 这款创意积木套装包含100块色彩鲜艳的积木...
      适合家庭娱乐、创意拼装，帮助培养空间想象力...
要点:
- 优质材质：采用环保ABS塑料，安全耐用
- 多样玩法：100块积木可自由组合，发挥创造力
- 家庭娱乐：适合家庭聚会、休闲时光
```

## 相关文件

- `platforms/temu/handlers/ai_content_rewriter.go` - 核心实现
- `platforms/temu/pipeline.go` - Pipeline配置
- `docs/AI_CONTENT_REWRITE.md` - 详细文档

## 测试

```bash
go test -v ./platforms/temu/handlers -run TestAIContentRewriter
```
