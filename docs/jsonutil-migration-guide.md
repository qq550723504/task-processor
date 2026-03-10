# JSON工具包迁移指南

## 概述

为了消除项目中60+处重复的JSON解析代码，我们创建了 `internal/pkg/jsonutil` 包。

本文档说明如何将现有代码迁移到新的工具包。

---

## 新工具包功能

### 1. UnmarshalString - 从JSON字符串解析

```go
func UnmarshalString[T any](jsonStr string, target *T, errorPrefix string) error
```

### 2. UnmarshalBytes - 从JSON字节数组解析

```go
func UnmarshalBytes[T any](data []byte, target *T, errorPrefix string) error
```

### 3. MustUnmarshalString - 必须成功解析（用于测试）

```go
func MustUnmarshalString[T any](jsonStr string) T
```

---

## 迁移示例

### 示例 1: 基本用法

**迁移前:**
```go
var task model.Task
if err := json.Unmarshal([]byte(taskData), &task); err != nil {
    return fmt.Errorf("解析任务数据失败: %w", err)
}
```

**迁移后:**
```go
var task model.Task
if err := jsonutil.UnmarshalString(taskData, &task, "解析任务数据失败"); err != nil {
    return err
}
```

### 示例 2: 从字节数组解析

**迁移前:**
```go
var config Config
if err := json.Unmarshal(data, &config); err != nil {
    return fmt.Errorf("解析配置文件失败: %w", err)
}
```

**迁移后:**
```go
var config Config
if err := jsonutil.UnmarshalBytes(data, &config, "解析配置文件失败"); err != nil {
    return err
}
```

### 示例 3: 无错误前缀

**迁移前:**
```go
var result Response
if err := json.Unmarshal([]byte(content), &result); err != nil {
    return fmt.Errorf("JSON解析失败: %w", err)
}
```

**迁移后:**
```go
var result Response
if err := jsonutil.UnmarshalString(content, &result, ""); err != nil {
    return err
}
```

---

## 需要迁移的文件列表

### 高优先级（核心业务逻辑）

#### TEMU平台
1. `internal/platforms/temu/task_submitter.go` - 1处
2. `internal/platforms/temu/processor.go` - 1处
3. `internal/platforms/temu/services/business_service/inventory_sync_helper.go` - 4处
4. `internal/platforms/temu/services/business_service/inventory_sync_updater.go` - 2处
5. `internal/platforms/temu/services/business_service/inventory_sync_record.go` - 2处
6. `internal/platforms/temu/services/business_service/inventory_sync_api.go` - 2处

#### SHEIN平台
7. `internal/platforms/shein/service/pipeline/processor_service.go` - 1处
8. `internal/platforms/shein/service/pipeline/submitter_service.go` - 1处
9. `internal/platforms/shein/service/business_service/inventory_sync.go` - 1处
10. `internal/platforms/shein/service/business_service/inventory_sync_api.go` - 2处
11. `internal/platforms/shein/service/business_service/inventory_sync_helper.go` - 4处
12. `internal/platforms/shein/service/business_service/inventory_sync_record.go` - 2处
13. `internal/platforms/shein/service/business_service/product_data_helper.go` - 3处

### 中优先级（Handler层）

14. `internal/platforms/temu/handlers/sku/ai_mapping_single_processor.go` - 1处
15. `internal/platforms/temu/handlers/filter/sensitive_words_filter.go` - 1处
16. `internal/platforms/temu/handlers/filter/prohibited_items_config.go` - 1处
17. `internal/platforms/temu/handlers/ai/service.go` - 1处
18. `internal/platforms/temu/handlers/ai/content_rewriter.go` - 1处

19. `internal/platforms/shein/service/publish/error_handler_service.go` - 1处
20. `internal/platforms/shein/service/product/skc/translation_service.go` - 2处
21. `internal/platforms/shein/service/product/attribute/selector_service.go` - 2处
22. `internal/platforms/shein/service/product/attribute/sale/json_parser_service.go` - 1处

### 低优先级（其他模块）

23. 其他零散文件 - 约10+处

---

## 迁移步骤

### 阶段一：核心业务逻辑（推荐优先）
1. 迁移 TEMU 平台的 inventory_sync 相关文件
2. 迁移 SHEIN 平台的 inventory_sync 相关文件
3. 迁移任务提交和处理相关文件
4. 验证编译和测试

### 阶段二：Handler层
1. 迁移 TEMU handlers
2. 迁移 SHEIN service
3. 验证编译和测试

### 阶段三：其他模块
1. 迁移剩余文件
2. 最终验证

---

## 迁移检查清单

每个文件迁移后需要检查：

- [ ] 添加 `jsonutil` 导入
- [ ] 替换所有 `json.Unmarshal` 调用
- [ ] 移除不必要的 `json` 导入（如果没有其他使用）
- [ ] 验证错误消息格式
- [ ] 运行 `go build` 确保编译通过
- [ ] 运行相关测试

---

## 注意事项

1. **保持错误消息一致性**
   - 使用有意义的错误前缀
   - 保持与原代码相同的错误信息

2. **不要过度迁移**
   - 如果代码有特殊的错误处理逻辑，保持原样
   - 如果需要在解析前后做额外处理，考虑保持原样

3. **测试覆盖**
   - 迁移后运行相关测试
   - 特别注意错误处理路径

---

## 预期收益

- **代码减少**: 约200+行重复代码
- **一致性**: 统一的JSON解析错误格式
- **可维护性**: 集中管理JSON解析逻辑
- **可读性**: 更简洁的代码

---

## 示例PR

建议分批提交，每个PR包含：
- 5-10个相关文件的迁移
- 完整的编译验证
- 相关测试通过

---

## 参考

- jsonutil包位置: `internal/pkg/jsonutil/unmarshal.go`
- 重复代码分析: `docs/duplicate-code-analysis.md`
