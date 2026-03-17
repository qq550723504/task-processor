// Package context 提供 SHEIN 属性相关类型定义
package context

import (
	"task-processor/internal/model"
	"task-processor/internal/pkg/types"
	"task-processor/internal/shein/api/attribute"
)

// CustomAttributeResult 自定义属性处理结果
type CustomAttributeResult struct {
	Success        bool
	NewValueID     int
	Relations      []attribute.CustomAttributeRelation
	ShouldContinue bool
}

// AttributeStrategy 属性策略结构体
type AttributeStrategy struct {
	PrimaryAttribute   ResultAttribute `json:"primary_attribute"`
	SecondaryAttribute ResultAttribute `json:"secondary_attribute"`
	StrategyType       string          `json:"strategy_type"`
}

// AttributeValue 属性值信息
type AttributeValue struct {
	ID    types.FlexibleID `json:"id"`
	Value string           `json:"value"`
}

// AttributePriorityConfig 属性优先级配置
type AttributePriorityConfig struct {
	SKCPrimaryAttributePriority   []int          `json:"skc_primary_attribute_priority"`
	SKUSecondaryAttributePriority []int          `json:"sku_secondary_attribute_priority"`
	DefaultSKCAttributeID         int            `json:"default_skc_attribute_id"`
	AttributeNameToID             map[string]int `json:"attribute_name_to_id"`
}

// AttributeValueIDManager 属性值ID管理器
type AttributeValueIDManager struct {
	usedIDs      map[int]bool
	nextCustomID int
}

// NewAttributeValueIDManager 创建属性值ID管理器
func NewAttributeValueIDManager() *AttributeValueIDManager {
	return &AttributeValueIDManager{
		usedIDs:      make(map[int]bool),
		nextCustomID: -1,
	}
}

// ResultSaleAttribute 结果销售规格
type ResultSaleAttribute struct {
	SaleAttributes []ResultAttribute
	Variants       []Variant
}

// VariantInfo 变体信息
type VariantInfo struct {
	Variant   Variant
	AttrID    int
	ValueID   int
	AttrValue string
}

// Variant 变体
type Variant struct {
	Attributes   map[string]string    `json:"attributes"`
	Length       types.FlexibleString `json:"length"`
	Width        types.FlexibleString `json:"width"`
	Height       types.FlexibleString `json:"height"`
	Weight       types.FlexibleString `json:"weight"`
	LengthUnit   string               `json:"lengthUnit"`
	ASIN         string               `json:"asin"`
	Price        float64              `json:"price"`
	QuantityType int                  `json:"quantityType"`
	UnitType     int                  `json:"unitType"`
	Quantity     int                  `json:"quantity"`
}

// AttributeImportanceCalculator 属性重要性计算器
type AttributeImportanceCalculator struct {
	Rules *ImportanceRules
}

// NewAttributeImportanceCalculator 创建新的属性重要性计算器
func NewAttributeImportanceCalculator() *AttributeImportanceCalculator {
	return &AttributeImportanceCalculator{
		Rules: &ImportanceRules{
			RemarkListScore: 100,
			RequiredScore:   80,
			SampleScore:     40,
			ActiveScore:     30,
			DisplayScore:    20,
		},
	}
}

// NewAttributeImportanceCalculatorWithRules 创建带自定义规则的属性重要性计算器
func NewAttributeImportanceCalculatorWithRules(rules *ImportanceRules) *AttributeImportanceCalculator {
	return &AttributeImportanceCalculator{Rules: rules}
}

// AttributeMetadata 属性元数据
type AttributeMetadata struct {
	AttrID          int                      `json:"attrId"`
	AttrValue       []GenerateAttributeValue `json:"attrValue"`
	Required        bool                     `json:"required"`
	Type            int                      `json:"type"`
	Importance      int                      `json:"importance"`
	AttributeName   string                   `json:"attributeName"`
	AttributeNameEn string                   `json:"attributeNameEn"`
	VariantName     string                   `json:"variantName"`
}

// AttributeConfig 属性配置
type AttributeConfig struct {
	CommonNameToAttrID map[string]int
	ImportanceRules    ImportanceRules
}

// ImportanceRules 重要性评分规则
type ImportanceRules struct {
	RemarkListScore int
	RequiredScore   int
	SampleScore     int
	ActiveScore     int
	DisplayScore    int
}

// ResultAttribute 结果属性
type ResultAttribute struct {
	AttrID    int              `json:"attrId"`
	AttrValue []AttributeValue `json:"attrValue"`
}

// SaleAttribute 销售属性（ResultAttribute 的别名）
type SaleAttribute = ResultAttribute

// AttributeData 属性数据
type AttributeData struct {
	AttributeData []ResultAttribute
}

// AttributeImportanceResult 属性重要性计算结果
type AttributeImportanceResult struct {
	Importance        int
	HasRemarkList     bool
	IsLabelAttribute  bool
	IsSampleAttribute bool
	IsActiveStatus    bool
	IsKeyPrimary      bool
}

// GenerationRequest AI生成请求
type GenerationRequest struct {
	ProductsData             []ProductVariantData    `json:"products_data"`
	VariationData            []model.Variation       `json:"variation_data"`
	VariationAttributeValues *[]model.VariationValue `json:"variations_values"`
	SaleAttributesData       []AttributeMetadata     `json:"sale_attributes_data"`
	AttributeMappings        []AttributeNameMapping  `json:"attribute_mappings"`
	RequiredVariantCount     int                     `json:"required_variant_count"`
}

// AttributeNameMapping 属性名称映射
type AttributeNameMapping struct {
	AttrID               int    `json:"attrId"`
	VariantAttributeName string `json:"variantAttributeName"`
}

// ProductVariantData 产品变体数据
type ProductVariantData struct {
	ASIN         string            `json:"asin"`
	Title        string            `json:"title"`
	BulletPoints string            `json:"bulletpoints,omitempty"`
	Attributes   map[string]string `json:"attributes"`
	Price        float64           `json:"price,omitempty"`
	Dimensions   string            `json:"dimensions,omitempty"`
	Weight       string            `json:"weight,omitempty"`
}

// GenerateAttributeValue 生成的属性值
type GenerateAttributeValue struct {
	ID    int    `json:"id"`
	Value string `json:"value"`
}

// GenerateAttribute 生成的属性
type GenerateAttribute struct {
	AttrID    int                      `json:"attrId"`
	AttrValue []GenerateAttributeValue `json:"attrValue"`
	Required  bool                     `json:"required"`
	Type      int                      `json:"type"`
}

// BuildAttributeInfo 构建的属性信息
type BuildAttributeInfo struct {
	AttributeData     []GenerateAttribute `json:"attribute_data"`
	SaleAttributeData []GenerateAttribute `json:"sale_attribute_data"`
}

// EnhancedGenerateAttribute 增强的属性生成结构
type EnhancedGenerateAttribute struct {
	AttrID                      int                      `json:"attrId"`
	AttrValue                   []GenerateAttributeValue `json:"attrValue"`
	Required                    bool                     `json:"required"`
	Type                        int                      `json:"type"`
	AttributeNameEn             string                   `json:"attribute_name_en"`
	AttributeName               string                   `json:"attribute_name"`
	AttributeDoc                *string                  `json:"attribute_doc"`
	CascadeAttributeID          int                      `json:"cascade_attribute_id"`
	CascadeAttributeValueIDList *string                  `json:"cascade_attribute_value_id_list"`
	SKCScope                    *bool                    `json:"skc_scope"`
	Importance                  int                      `json:"importance"`
	HasRemarkList               bool                     `json:"has_remark_list"`
	IsLabelAttribute            bool                     `json:"is_label_attribute"`
	IsSampleAttribute           bool                     `json:"is_sample_attribute"`
	IsActiveStatus              bool                     `json:"is_active_status"`
	HasDependency               bool                     `json:"has_dependency"`
}
