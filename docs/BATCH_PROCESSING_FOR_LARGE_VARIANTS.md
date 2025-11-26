# 大量变体分批处理优化

## 问题背景

当产品有大量变体（>20个）时，AI生成SKU映射会遇到以下问题：

1. **Token限制**：Gemini 2.0 Flash的输出限制是8192 tokens，大量变体会导致响应被截断
2. **规格维度超限**：TEMU平台限制最多2个销售规格，但AI可能提取3个或更多维度
3. **数据完整性**：简化输入数据会导致物流信息（重量、尺寸）不准确

## 解决方案

### TEMU平台优化

#### 1. 分批处理机制

**文件**: `platforms/temu/handlers/sku_ai_mapping.go`

- 当变体数量 > 20时，自动分批处理
- 每批最多处理20个变体
- 分批完成后统一规格维度

```go
const maxVariantsPerBatch = 20

if len(variants) > maxVariantsPerBatch {
    return sb.generateAISkuMappingInBatches(ctx, variants, maxVariantsPerBatch)
}
```

#### 2. 动态MaxTokens计算

根据变体数量动态计算所需tokens：

```go
estimatedTokensPerSku := 300
estimatedTokens := len(variants)*estimatedTokensPerSku + 1000

const maxOutputTokens = 8000
maxTokens := estimatedTokens
if maxTokens > maxOutputTokens {
    maxTokens = maxOutputTokens
}
```

#### 3. 规格维度自动过滤

**文件**: `platforms/temu/handlers/sku_ai_mapping.go`

`enforceSpecCountLimit`方法按优先级自动选择最重要的2个规格维度：

- 优先级：颜色 > 尺寸 > 其他
- 统计所有SKU使用的规格维度
- 如果超过2个，自动过滤保留最重要的2个

#### 4. AI Prompt优化

强化prompt中的规格限制说明：

```
1. **规格数量限制（最重要）**: 每个SKU的spec数组**严格限制为最多2个规格**
   - **所有SKU必须使用相同的规格维度组合**
   - **如果Amazon产品有超过2个变体维度，必须只选择最重要的2个**
   - **优先级顺序**: 颜色 > 尺寸 > 其他属性
```

### SHEIN平台优化

#### 1. 分批处理机制

**文件**: `platforms/shein/modules/sale_attribute_handler.go`

与TEMU类似，添加分批处理逻辑：

```go
const maxVariantsPerBatch = 20

if variantCount > maxVariantsPerBatch {
    return h.callGPTAPIInBatches(ctx, request, maxVariantsPerBatch)
}
```

#### 2. MaxTokens限制

为`createChatCompletionRequest`添加MaxTokens参数：

```go
estimatedTokensPerVariant := 300
estimatedTokens := variantCount*estimatedTokensPerVariant + 1000

const maxOutputTokens = 8000
maxTokens := estimatedTokens
if maxTokens > maxOutputTokens {
    maxTokens = maxOutputTokens
}
```

#### 3. 响应截断检测

添加响应截断检测和错误处理：

```go
if response.Choices[0].FinishReason == "length" {
    logrus.Errorf("❌ GPT响应被截断（达到token限制）")
    return ResultSaleAttribute{}
}
```

## 技术细节

### 分批处理流程

1. **检查变体数量**：如果 > 20，启动分批处理
2. **计算批次数量**：`totalBatches = (len(variants) + batchSize - 1) / batchSize`
3. **逐批处理**：
   - 创建批次请求（包含部分变体）
   - 调用AI API
   - 收集结果
4. **合并结果**：将所有批次的结果合并
5. **统一规格**：对合并结果执行规格维度统一（仅TEMU）

### Token估算

- **每个SKU/变体**：约300 tokens
- **基础开销**：约1000 tokens
- **总估算**：`变体数 × 300 + 1000`
- **最大限制**：8000 tokens（Gemini 2.0 Flash）

### 规格优先级（TEMU）

```go
func (sb *SkuBuilder) getSpecPriority(specName string) int {
    if isColorSpec(specName) {
        return 1  // 颜色：最高优先级
    }
    if isSizeSpec(specName) {
        return 2  // 尺寸：次优先级
    }
    return 3      // 其他：最低优先级
}
```

## 优势

### ✅ 支持大量变体
- 最多支持100个变体
- 自动分批，无需手动干预

### ✅ 避免Token限制
- 动态计算MaxTokens
- 单批不超过8000 tokens
- 检测并处理响应截断

### ✅ 保证数据完整性
- 不简化输入数据
- 保留完整的物流信息
- 保留description和features

### ✅ 规格维度控制（TEMU）
- 自动限制为2个规格
- 智能选择最重要的维度
- 统一所有SKU的规格组合

### ✅ 代码质量
- 编译通过，无警告
- 遵循Go最佳实践
- 完善的日志记录

## 使用示例

### TEMU平台

```go
// 自动处理，无需额外配置
aiResponse, err := sb.generateAISkuMapping(ctx, variants)
// 如果变体数 > 20，会自动分批处理
```

### SHEIN平台

```go
// 自动处理，无需额外配置
saleAttributeData := h.callGPTAPI(ctx, request)
// 如果变体数 > 20，会自动分批处理
```

## 日志示例

### 分批处理日志

```
INFO[2025-11-22] 🔄 变体数量(23)超过单批限制(20)，将分批处理
INFO[2025-11-22] 📦 开始分批处理: 总变体数=23, 批次大小=20, 总批次=2
INFO[2025-11-22] 📦 处理批次 1/2: 变体[0-19]
INFO[2025-11-22] 设置MaxTokens=7000 (变体数=20, 估算需要=7000)
INFO[2025-11-22] ✅ 批次 1/2 完成，生成20个SKU
INFO[2025-11-22] 📦 处理批次 2/2: 变体[20-22]
INFO[2025-11-22] 设置MaxTokens=1900 (变体数=3, 估算需要=1900)
INFO[2025-11-22] ✅ 批次 2/2 完成，生成3个SKU
INFO[2025-11-22] ✅ 所有批次处理完成，共生成23个SKU
INFO[2025-11-22] 🔄 对合并结果执行规格维度统一...
```

### 规格过滤日志（TEMU）

```
WARN[2025-11-22] ⚠️ 检测到3个规格维度，超过TEMU限制(2个)，将自动选择最重要的2个
INFO[2025-11-22] ✅ 选择规格维度[1]: 颜色 (parent_spec_id=1001, 使用次数=23)
INFO[2025-11-22] ✅ 选择规格维度[2]: 尺寸 (parent_spec_id=3001, 使用次数=23)
WARN[2025-11-22] ⚠️ 忽略规格维度: 数量 (parent_spec_id=15998553, 使用次数=6)
INFO[2025-11-22] ✅ 规格数量限制强制执行完成，所有SKU现在使用2个规格维度
```

## 配置

无需额外配置，系统会自动：
- 检测变体数量
- 决定是否分批
- 计算合适的MaxTokens
- 统一规格维度（TEMU）

## 限制

- **最大变体数**：100个（硬性限制）
- **单批大小**：20个变体
- **MaxTokens**：8000 tokens（Gemini 2.0 Flash限制）
- **规格数量**：2个（TEMU平台限制）

## 相关文件

### TEMU平台
- `platforms/temu/handlers/sku_ai_mapping.go` - AI映射生成和分批处理
- `platforms/temu/handlers/sku_skc_builder.go` - 规格维度统一

### SHEIN平台
- `platforms/shein/modules/sale_attribute_handler.go` - 销售属性生成和分批处理

## 测试

编译测试通过：
```bash
go build -o task-processor.exe ./cmd/temu-web
# Exit Code: 0
```

## 总结

通过分批处理和智能规格过滤，系统现在可以稳定处理大量变体（最多100个），同时保证数据完整性和平台规则合规性。
