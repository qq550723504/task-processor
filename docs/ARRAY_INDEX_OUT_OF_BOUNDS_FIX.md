# 数组越界Panic修复

## 问题描述

程序在处理任务时发生panic：

```
ERRO 工作协程 2 发生 panic: runtime error: index out of range [7] with length 7
ERRO Panic 发生在任务: TaskID=1553537, ProductID=B0D2D4MKN4
```

**堆栈跟踪显示：**
```
task-processor/platforms/temu/handlers/sku_skc_builder.go:139
```

## 根本原因

在 `sku_skc_builder.go` 第139行：

```go
for i, variant := range variants {
    aiSku := aiMapping.SkuList[i]  // ❌ 没有边界检查
    // ...
}
```

**问题：**
- `variants` 有11个元素（索引0-10）
- `aiMapping.SkuList` 只有7个元素（索引0-6）
- 当 `i=7` 时，访问 `aiMapping.SkuList[7]` 导致越界panic

**为什么会出现数量不匹配？**
1. AI生成的映射数量少于变体数量
2. 补充映射的逻辑可能失败
3. 或者在某些步骤中过滤掉了部分映射

## 解决方案

### 1. 添加边界检查（`sku_skc_builder.go`）

**修改前：**
```go
for i, variant := range variants {
    aiSku := aiMapping.SkuList[i]  // ❌ 直接访问，可能越界
    // ...
}
```

**修改后：**
```go
for i, variant := range variants {
    // 边界检查：防止数组越界
    if i >= len(aiMapping.SkuList) {
        sb.logger.Errorf("❌ 变体索引[%d]超出AI映射范围(长度=%d)，跳过该变体: ASIN=%s", 
            i, len(aiMapping.SkuList), variant.Asin)
        continue
    }
    
    aiSku := aiMapping.SkuList[i]  // ✅ 安全访问
    // ...
}
```

### 2. 增强Panic日志（`common/worker/pool.go`）

添加堆栈跟踪，方便定位问题：

```go
defer func() {
    if r := recover() {
        logrus.Errorf("工作协程 %d 发生 panic: %v", w.id, r)
        
        // 打印堆栈跟踪
        buf := make([]byte, 4096)
        n := runtime.Stack(buf, false)
        logrus.Errorf("堆栈跟踪:\n%s", string(buf[:n]))
        
        // 记录任务信息
        var task types.Task
        if err := json.Unmarshal([]byte(job.TaskData), &task); err == nil {
            logrus.Errorf("Panic 发生在任务: TaskID=%s, ProductID=%s", task.ID, task.ProductID)
        }
    }
}()
```

## 效果对比

### 修改前
```
ERRO 工作协程 2 发生 panic: runtime error: index out of range [7] with length 7
ERRO Panic 发生在任务: TaskID=1553537, ProductID=B0D2D4MKN4
→ 程序崩溃，任务失败，没有详细信息
```

### 修改后
```
ERRO ❌ 变体索引[7]超出AI映射范围(长度=7)，跳过该变体: ASIN=B0XXXXX
ERRO ❌ 变体索引[8]超出AI映射范围(长度=7)，跳过该变体: ASIN=B0YYYYY
ERRO ❌ 变体索引[9]超出AI映射范围(长度=7)，跳过该变体: ASIN=B0ZZZZZ
ERRO ❌ 变体索引[10]超出AI映射范围(长度=7)，跳过该变体: ASIN=B0WWWWW
→ 程序继续运行，跳过问题变体，处理其他变体
```

## 根本解决方案

虽然添加了边界检查可以防止panic，但根本问题是AI映射数量与变体数量不匹配。需要：

### 1. 检查AI映射生成逻辑

确保AI为所有变体生成映射：

```go
// 在 sku_builder.go 中
if len(aiMapping.SkuList) != len(variants) {
    sb.logger.Warnf("⚠️ AI映射数量(%d)与变体数量(%d)不匹配", 
        len(aiMapping.SkuList), len(variants))
    
    // 补充缺失的映射
    if err := sb.supplementMissingMappings(aiMapping, variants); err != nil {
        return nil, fmt.Errorf("补充缺失映射失败: %w", err)
    }
}
```

### 2. 检查补充映射逻辑

确保 `supplementMissingMappings` 方法正确工作：

```go
func (sb *SkuBuilder) supplementMissingMappings(aiMapping *AISkuMappingResponse, variants []*amazon.Product) error {
    // 创建已映射的ASIN集合
    mappedAsins := make(map[string]bool)
    for _, sku := range aiMapping.SkuList {
        mappedAsins[sku.Asin] = true
    }
    
    // 为未映射的变体创建默认映射
    for _, variant := range variants {
        if !mappedAsins[variant.Asin] {
            // 创建默认映射
            defaultSku := AISkuInfo{
                Asin: variant.Asin,
                // ... 其他字段
            }
            aiMapping.SkuList = append(aiMapping.SkuList, defaultSku)
        }
    }
    
    return nil
}
```

## 相关文件

- `platforms/temu/handlers/sku_skc_builder.go` - 添加边界检查
- `platforms/temu/handlers/sku_builder.go` - AI映射数量检查和补充
- `common/worker/pool.go` - 增强panic日志

## 预防措施

在所有数组访问前添加边界检查：

```go
// ❌ 不安全
item := array[i]

// ✅ 安全
if i < len(array) {
    item := array[i]
} else {
    // 处理越界情况
}
```
