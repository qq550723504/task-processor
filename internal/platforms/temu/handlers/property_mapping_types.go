package handlers

// =============================================================================
// AI属性映射数据结构
// =============================================================================

// PropertyMappingData AI属性映射数据结构
type PropertyMappingData struct {
	AmazonProduct  AmazonProductData    `json:"amazon_product"`
	TemuProperties []TemuPropertyOption `json:"temu_properties"`
}

// AmazonProductData Amazon产品数据（简化版）
type AmazonProductData struct {
	Title             string              `json:"title"`
	Brand             string              `json:"brand"`
	Description       string              `json:"description"`
	Features          []string            `json:"features"`
	ProductDetails    []ProductDetailData `json:"product_details"`
	ProductDimensions string              `json:"product_dimensions"`
	ItemWeight        string              `json:"item_weight"`
	ModelNumber       string              `json:"model_number"`
	Department        string              `json:"department"`
	Manufacturer      string              `json:"manufacturer"`
	Categories        []string            `json:"categories"`
}

// ProductDetailData 产品详情数据
type ProductDetailData struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

// TemuPropertyOption TEMU属性选项
type TemuPropertyOption struct {
	PID               int                   `json:"pid"`
	RefPID            int                   `json:"ref_pid"`
	TemplatePID       int                   `json:"template_pid"`
	TemplateModuleID  int                   `json:"template_module_id"`
	Name              string                `json:"name"`
	PropertyValueType int                   `json:"property_value_type"` // 1:选择 2:数字 3:文本
	Required          bool                  `json:"required"`
	ChooseMaxNum      int                   `json:"choose_max_num"` // 最大选择数量，1表示单选，>1表示多选
	Values            []PropertyValueOption `json:"values,omitempty"`
	ValueUnit         []string              `json:"value_unit,omitempty"`
	MinValue          string                `json:"min_value,omitempty"`
	MaxValue          string                `json:"max_value,omitempty"`
	ShowCondition     []ShowCondition       `json:"show_condition,omitempty"` // 显示条件
}

// PropertyValueOption 属性值选项
type PropertyValueOption struct {
	VID   int    `json:"vid"`
	Value string `json:"value"`
}
