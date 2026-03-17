package shein

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
	ShouldContinue bool // 是否应该继续处理（用于决定是continue还是return error）
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

type AttributePriorityConfig struct {
	SKCPrimaryAttributePriority   []int          `json:"skc_primary_attribute_priority"`   // SKC主要属性的优先级顺序（按重要性从高到低）
	SKUSecondaryAttributePriority []int          `json:"sku_secondary_attribute_priority"` // SKU次要属性的优先级顺序
	DefaultSKCAttributeID         int            `json:"default_skc_attribute_id"`         // 默认的SKC属性ID（如果找不到任何匹配的主要属性）
	AttributeNameToID             map[string]int `json:"attribute_name_to_id"`             // 属性名称映射到ID的配置
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

type ResultSaleAttribute struct {
	SaleAttributes []ResultAttribute
	Variants       []Variant
}

type VariantInfo struct {
	Variant   Variant
	AttrID    int
	ValueID   int
	AttrValue string // 添加属性值用于追踪和去重
}

type Variant struct {
	Attributes   map[string]string    `json:"attributes"`
	Length       types.FlexibleString `json:"length"`
	Width        types.FlexibleString `json:"width"`
	Height       types.FlexibleString `json:"height"`
	Weight       types.FlexibleString `json:"weight"`
	LengthUnit   string               `json:"lengthUnit"`
	ASIN         string               `json:"asin"`
	Price        float64              `json:"price"`
	QuantityType int                  `json:"quantityType"` // 单品、同款多件、单套、多套
	UnitType     int                  `json:"unitType"`     // 单位类型: "件" 、"双"、"套"
	Quantity     int                  `json:"quantity"`     // 数量
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
	return &AttributeImportanceCalculator{
		Rules: rules,
	}
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
	VariantName     string                   `json:"variantName"` // 用于变体中的属性名称
}

// AttributeConfig 属性配置
type AttributeConfig struct {
	// 通用英文名称到属性ID的映射
	CommonNameToAttrID map[string]int
	// 属性重要性计算规则
	ImportanceRules ImportanceRules
}

// ImportanceRules 重要性评分规则
type ImportanceRules struct {
	RemarkListScore int // 有备注列表的分数
	RequiredScore   int // 必填属性的分数
	SampleScore     int // 示例属性的分数
	ActiveScore     int // 活跃属性的分数
	DisplayScore    int // 显示属性的分数
}

// TODO:有地方使用attr_Id,attr_Value，已修改，待观察
type ResultAttribute struct {
	AttrID    int              `json:"attrId"`
	AttrValue []AttributeValue `json:"attrValue"`
}

// SaleAttribute 销售属性（ResultAttribute的别名）
type SaleAttribute = ResultAttribute
type AttributeData struct {
	AttributeData []ResultAttribute
}

// AttributeImportanceResult 属性重要性计算结果
type AttributeImportanceResult struct {
	Importance        int  // 总重要性评分
	HasRemarkList     bool // 是否有备注列表
	IsLabelAttribute  bool // 是否为标签属性
	IsSampleAttribute bool // 是否为示例属性
	IsActiveStatus    bool // 是否为活跃状态
	IsKeyPrimary      bool // 是否为关键主属性
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
	AttrID    int                      `json:"attrId"`    //如果AI生成的属性有问题就是这里attr_Id
	AttrValue []GenerateAttributeValue `json:"attrValue"` //如果AI生成的属性有问题就是这里attr_Value
	Required  bool                     `json:"required"`
	Type      int                      `json:"type"`
}

// BuildAttributeInfo 构建的属性信息
type BuildAttributeInfo struct {
	AttributeData     []GenerateAttribute `json:"attribute_data"`
	SaleAttributeData []GenerateAttribute `json:"sale_attribute_data"`
}

// EnhancedGenerateAttribute 增强的属性生成结构，包含模板信息
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
	Importance                  int                      `json:"importance"`          // 重要性评分
	HasRemarkList               bool                     `json:"has_remark_list"`     // 是否有备注列表
	IsLabelAttribute            bool                     `json:"is_label_attribute"`  // 是否为标签属性
	IsSampleAttribute           bool                     `json:"is_sample_attribute"` // 是否为示例属性
	IsActiveStatus              bool                     `json:"is_active_status"`    // 是否为活跃状态
	HasDependency               bool                     `json:"has_dependency"`      // 是否有依赖关系
}

