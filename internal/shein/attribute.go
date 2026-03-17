// Package shein 提供 SHEIN 属性类型（通过 shein/context 包定义）
package shein

import sheinctx "task-processor/internal/shein/context"

// AttributeValue 属性值信息
type AttributeValue = sheinctx.AttributeValue

// AttributePriorityConfig 属性优先级配置
type AttributePriorityConfig = sheinctx.AttributePriorityConfig

// AttributeValueIDManager 属性值ID管理器
type AttributeValueIDManager = sheinctx.AttributeValueIDManager

// ResultSaleAttribute 结果销售规格
type ResultSaleAttribute = sheinctx.ResultSaleAttribute

// VariantInfo 变体信息
type VariantInfo = sheinctx.VariantInfo

// Variant 变体
type Variant = sheinctx.Variant

// AttributeImportanceCalculator 属性重要性计算器
type AttributeImportanceCalculator = sheinctx.AttributeImportanceCalculator

// AttributeMetadata 属性元数据
type AttributeMetadata = sheinctx.AttributeMetadata

// AttributeConfig 属性配置
type AttributeConfig = sheinctx.AttributeConfig

// ImportanceRules 重要性评分规则
type ImportanceRules = sheinctx.ImportanceRules

// ResultAttribute 结果属性
type ResultAttribute = sheinctx.ResultAttribute

// SaleAttribute 销售属性
type SaleAttribute = sheinctx.SaleAttribute

// AttributeData 属性数据
type AttributeData = sheinctx.AttributeData

// AttributeImportanceResult 属性重要性计算结果
type AttributeImportanceResult = sheinctx.AttributeImportanceResult

// GenerationRequest AI生成请求
type GenerationRequest = sheinctx.GenerationRequest

// AttributeNameMapping 属性名称映射
type AttributeNameMapping = sheinctx.AttributeNameMapping

// ProductVariantData 产品变体数据
type ProductVariantData = sheinctx.ProductVariantData

// GenerateAttributeValue 生成的属性值
type GenerateAttributeValue = sheinctx.GenerateAttributeValue

// GenerateAttribute 生成的属性
type GenerateAttribute = sheinctx.GenerateAttribute

// BuildAttributeInfo 构建的属性信息
type BuildAttributeInfo = sheinctx.BuildAttributeInfo

// EnhancedGenerateAttribute 增强的属性生成结构
type EnhancedGenerateAttribute = sheinctx.EnhancedGenerateAttribute

// AttributeStrategy 属性策略结构体
type AttributeStrategy = sheinctx.AttributeStrategy

// CustomAttributeResult 自定义属性处理结果
type CustomAttributeResult = sheinctx.CustomAttributeResult

// NewAttributeValueIDManager 创建属性值ID管理器
var NewAttributeValueIDManager = sheinctx.NewAttributeValueIDManager

// NewAttributeImportanceCalculator 创建属性重要性计算器
var NewAttributeImportanceCalculator = sheinctx.NewAttributeImportanceCalculator

// NewAttributeImportanceCalculatorWithRules 创建带自定义规则的属性重要性计算器
var NewAttributeImportanceCalculatorWithRules = sheinctx.NewAttributeImportanceCalculatorWithRules