// Package types 提供TEMU平台模板相关的类型定义
package types

// ValueUnit 值单位结构体
type ValueUnit struct {
	ValueUnit   string `json:"value_unit"`
	ValueUnitID string `json:"value_unit_id"`
}

// Group 分组结构体
type Group struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// SubGroup 子分组结构体
type SubGroup struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// PropertyValue 属性值结构体
type PropertyValue struct {
	VID        int       `json:"vid"`
	Value      string    `json:"value"`
	Group      *Group    `json:"group,omitempty"`
	SubGroup   *SubGroup `json:"sub_group,omitempty"`
	SpecID     string    `json:"spec_id,omitempty"`
	ExtendInfo string    `json:"extend_info,omitempty"`
	ParentVIDs []int     `json:"parent_vids,omitempty"`
}

// ShowCondition 显示条件结构体
type ShowCondition struct {
	ParentRefPID int   `json:"parent_ref_pid"`
	ParentVIDs   []int `json:"parent_vids"`
}

// GroupValues 分组值结构体
type GroupValues struct {
	Name           string                    `json:"name"`
	Values         []PropertyValue           `json:"values"`
	SubGroupValues map[string]SubGroupValues `json:"sub_group_values,omitempty"`
}

// SubGroupValues 子分组值结构体
type SubGroupValues struct {
	SubGroupName string          `json:"sub_group_name"`
	Values       []PropertyValue `json:"values"`
}

// TemplatePropertyValueParent 模板属性值父级结构体
type TemplatePropertyValueParent struct {
	VIDs       []int `json:"vids"`
	ParentVIDs []int `json:"parent_vids"`
}

// GoodsProperty 商品属性结构体
type GoodsProperty struct {
	PID               int                    `json:"pid,omitempty"`
	TemplateModuleID  int                    `json:"template_module_id,omitempty"`
	TemplatePID       int                    `json:"template_pid,omitempty"`
	RefPID            int                    `json:"ref_pid,omitempty"`
	Name              string                 `json:"name,omitempty"`
	PropertyValueType int                    `json:"property_value_type,omitempty"`
	ValueUnit         []string               `json:"value_unit,omitempty"`
	Values            []PropertyValue        `json:"values,omitempty"`
	Group2Values      map[string]GroupValues `json:"group2_values,omitempty"`
	ChooseMaxNum      int                    `json:"choose_max_num"`
	MaxValue          string                 `json:"max_value,omitempty"`
	MinValue          string                 `json:"min_value,omitempty"`
	ValuePrecision    int                    `json:"value_precision,omitempty"`
	Required          bool                   `json:"required,omitempty"`
	IsSale            bool                   `json:"is_sale,omitempty"`
	ParentSpecID      string                 `json:"parent_spec_id,omitempty"`
	MainSale          bool                   `json:"main_sale,omitempty"`
	Feature           int                    `json:"feature,omitempty"`
	ValueRule         int                    `json:"value_rule,omitempty"`
	ControlType       int                    `json:"control_type,omitempty"`
	ShowType          int                    `json:"show_type,omitempty"`
	ParentTemplatePID int                    `json:"parent_template_pid,omitempty"`
}

// TemplateGoodsProperty 模板商品属性结构体
type TemplateRespGoodsProperty struct {
	PID                             int                           `json:"pid,omitempty"`
	TemplateModuleID                int                           `json:"template_module_id,omitempty"`
	TemplatePID                     int                           `json:"template_pid,omitempty"`
	RefPID                          int                           `json:"ref_pid,omitempty"`
	Name                            string                        `json:"name,omitempty"`
	PropertyValueType               int                           `json:"property_value_type,omitempty"`
	ValueUnit                       []string                      `json:"value_unit,omitempty"`
	ValueUnitDTOList                []ValueUnit                   `json:"value_unit_dtolist,omitempty"`
	Values                          []PropertyValue               `json:"values,omitempty"`
	ChooseMaxNum                    int                           `json:"choose_max_num,omitempty"`
	MaxValue                        string                        `json:"max_value,omitempty"`
	MinValue                        string                        `json:"min_value,omitempty"`
	ValuePrecision                  int                           `json:"value_precision,omitempty"`
	ShowCondition                   []ShowCondition               `json:"show_condition,omitempty"`
	Required                        bool                          `json:"required,omitempty"`
	IsSale                          bool                          `json:"is_sale,omitempty"`
	Feature                         int                           `json:"feature,omitempty"`
	ParentSpecID                    string                        `json:"parent_spec_id,omitempty"`
	MainSale                        bool                          `json:"main_sale,omitempty"`
	PropertyChooseTitle             string                        `json:"property_choose_title,omitempty"`
	NumberInputTitle                string                        `json:"number_input_title,omitempty"`
	ValueRule                       int                           `json:"value_rule,omitempty"`
	ControlType                     int                           `json:"control_type,omitempty"`
	ShowType                        int                           `json:"show_type,omitempty"`
	ParentTemplatePID               int                           `json:"parent_template_pid,omitempty"`
	TemplatePropertyValueParentList []TemplatePropertyValueParent `json:"template_property_value_parent_list,omitempty"`
}

// TemplateRespGoodsSpecProperty 模板响应商品规格属性
type TemplateRespGoodsSpecProperty struct {
	PID               int                    `json:"pid,omitempty"`
	TemplateModuleID  int                    `json:"template_module_id,omitempty"`
	TemplatePID       int                    `json:"template_pid,omitempty"`
	RefPID            int                    `json:"ref_pid,omitempty"`
	Name              string                 `json:"name,omitempty"`
	PropertyValueType int                    `json:"property_value_type,omitempty"`
	ValueUnit         []string               `json:"value_unit,omitempty"`
	Values            []PropertyValue        `json:"values,omitempty"`
	Group2Values      map[string]GroupValues `json:"group2_values,omitempty"`
	MaxValue          string                 `json:"max_value,omitempty"`
	MinValue          string                 `json:"min_value,omitempty"`
	ValuePrecision    int                    `json:"value_precision,omitempty"`
	Required          bool                   `json:"required,omitempty"`
	IsSale            bool                   `json:"is_sale,omitempty"`
	ParentSpecID      string                 `json:"parent_spec_id,omitempty"`
	MainSale          bool                   `json:"main_sale,omitempty"`
	Feature           int                    `json:"feature,omitempty"`
	ControlType       int                    `json:"control_type,omitempty"`
}

// GoodsSpecProperty 商品规格属性结构体
type GoodsSpecProperty struct {
	PID               int                    `json:"pid"`
	TemplateModuleID  int                    `json:"template_module_id"`
	TemplatePID       int                    `json:"template_pid"`
	RefPID            int                    `json:"ref_pid"`
	Name              string                 `json:"name"`
	PropertyValueType int                    `json:"property_value_type"`
	ValueUnit         []string               `json:"value_unit"`
	Values            []PropertyValue        `json:"values"`
	Group2Values      map[string]GroupValues `json:"group2_values,omitempty"`
	MaxValue          string                 `json:"max_value"`
	MinValue          string                 `json:"min_value"`
	ValuePrecision    int                    `json:"value_precision"`
	Required          bool                   `json:"required"`
	IsSale            bool                   `json:"is_sale"`
	ParentSpecID      string                 `json:"parent_spec_id,omitempty"`
	MainSale          bool                   `json:"main_sale"`
	Feature           int                    `json:"feature"`
	ControlType       int                    `json:"control_type"`
}

// UserInputParentSpec 用户输入父规格结构体
type UserInputParentSpec struct {
	ParentSpecID   string `json:"parent_spec_id"`
	ParentSpecName string `json:"parent_spec_name"`
}

// PubConfig 发布配置结构体
type PubConfig struct {
	Currency                  string `json:"currency"`
	CurrencySymbol            string `json:"currency_symbol"`
	VolumeUnit                string `json:"volume_unit"`
	WeightUnit                string `json:"weight_unit"`
	IsSymbolAfterPrice        bool   `json:"is_symbol_after_price"`
	RetailPriceCurrency       string `json:"retail_price_currency"`
	RetailPriceCurrencySymbol string `json:"retail_price_currency_symbol"`
	RetailIsSymbolAfterPrice  bool   `json:"retail_is_symbol_after_price"`
}

// TemplateInfo 模板信息结构体
type TemplateInfo struct {
	TemplateID          int                             `json:"template_id"`
	GoodsProperties     []TemplateRespGoodsProperty     `json:"goods_properties"`
	GoodsSpecProperties []TemplateRespGoodsSpecProperty `json:"goods_spec_properties"`
}

// TemplateQueryRequest 模板查询请求结构体
type TemplateQueryRequest struct {
	ListingCommitID      string `json:"listing_commit_id"`
	GoodsCommitID        string `json:"goods_commit_id"`
	GoodsID              string `json:"goods_id"`
	CatID                int    `json:"cat_id"`
	ListingCommitVersion string `json:"listing_commit_version"`
	ClickType            string `json:"click_type"`
}

// TemplateQueryResult 模板查询结果结构体
type TemplateQueryResult struct {
	InputMaxSpecNum         int                   `json:"input_max_spec_num"`
	SingleSpecValueNum      int                   `json:"single_spec_value_num"`
	AuthenticationLinkURL   string                `json:"authentication_link_url"`
	NeedMultiOriginRegion   bool                  `json:"need_multi_origin_region"`
	PubConfig               PubConfig             `json:"pub_config"`
	UserInputParentSpecList []UserInputParentSpec `json:"user_input_parent_spec_list"`
	TemplateInfo            TemplateInfo          `json:"template_info"`
}

// TemplateQueryResponse 模板查询响应结构体
type TemplateQueryResponse struct {
	Success   bool                `json:"success"`
	ErrorCode int                 `json:"error_code"`
	Result    TemplateQueryResult `json:"result"`
}
