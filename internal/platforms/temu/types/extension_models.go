// Package types 提供TEMU平台的商品扩展信息数据结构定义
package types

import "encoding/json"

// ExtensionInfo 扩展信息
type ExtensionInfo struct {
	GoodsProperty             GoodsPropertys      `json:"goods_property"`
	CertificationInfo         CertificationInfo   `json:"certification_info"`
	GoodsTrademark            GoodsTrademark      `json:"goods_trademark,omitempty"`
	GoodsProductTaxCodeDetail interface{}         `json:"goods_product_tax_code_detail,omitempty"`
	GoodsOriginInfo           GoodsOriginInfo     `json:"goods_origin_info"`
	GoodsDesc                 string              `json:"goods_desc,omitempty"`
	BulletPoints              []string            `json:"bullet_points,omitempty"`
	SecondHand                interface{}         `json:"second_hand,omitempty"`
	GuideFileInfo             interface{}         `json:"guide_file_info,omitempty"`
	SizeChartImageInfo        *SizeChartImageInfo `json:"size_chart_image_info,omitempty"`
}

// SizeChartImageInfo 尺寸图表信息
type SizeChartImageInfo struct {
	SizeChartImageList []ImageInfo `json:"size_chart_image_list"`
}

// MarshalJSON 实现自定义JSON序列化
func (s *SizeChartImageInfo) MarshalJSON() ([]byte, error) {
	// 如果指针为 nil 或 SizeChartImageList 为空，返回 null
	if s == nil || len(s.SizeChartImageList) == 0 {
		return []byte("null"), nil
	}

	// 否则使用标准序列化
	type Alias SizeChartImageInfo
	return json.Marshal((*Alias)(s))
}

// GoodsProperty 商品属性
type GoodsPropertys struct {
	GoodsBrandProperties []interface{}      `json:"goods_brand_properties"`
	GoodsProperties      []PropertyItem     `json:"goods_properties"`
	GoodsSpecProperties  []GoodSpecProperty `json:"goods_spec_properties"`
}

// GoodsSpecProperty 商品规格属性
type GoodSpecProperty struct {
	Value            string `json:"value"`
	SpecID           string `json:"spec_id"`
	ParentSpecID     string `json:"parent_spec_id"`
	ParentSpecName   string `json:"parent_spec_name"`
	Feature          int    `json:"feature,omitempty"`
	Checked          bool   `json:"checked"`
	ControlType      int    `json:"control_type"`
	Disabled         bool   `json:"disabled"`
	Name             string `json:"name"`
	IsCustomized     int    `json:"is_customized"`
	Vid              int    `json:"vid,omitempty"`                // 添加vid字段
	TemplateModuleID int    `json:"template_module_id,omitempty"` // 添加模板模块ID
	TemplatePid      int    `json:"template_pid,omitempty"`       // 添加模板PID
}

// MarshalJSON 实现自定义JSON序列化
func (g GoodsPropertys) MarshalJSON() ([]byte, error) {
	result := make(map[string]interface{})

	// 只添加非空的字段
	if len(g.GoodsBrandProperties) > 0 {
		result["goods_brand_properties"] = g.GoodsBrandProperties
	}

	if len(g.GoodsProperties) > 0 {
		result["goods_properties"] = g.GoodsProperties
	}

	// GoodsSpecProperties 总是包含
	result["goods_spec_properties"] = g.GoodsSpecProperties

	return json.Marshal(result)
}

// PropertyItem 属性项
type PropertyItem struct {
	RefPid           int    `json:"ref_pid"`
	Pid              int    `json:"pid"`
	TemplatePid      int    `json:"template_pid"`
	Value            string `json:"value"`
	Vid              int    `json:"vid"`
	ValueUnit        string `json:"value_unit,omitempty"`
	TemplateModuleID int    `json:"template_module_id,omitempty"`
	NumberInputValue string `json:"number_input_value,omitempty"`
}

// ExtraInfo 额外信息
type ExtraInfo struct {
	CreateEmptyGoods bool        `json:"create_empty_goods"`
	VersionType      interface{} `json:"version_type"`
	Tab              int         `json:"tab"`
	MinSkuImageSize  int         `json:"min_sku_image_size"`
	MaxSkuImageSize  int         `json:"max_sku_image_size"`
	CurrentTab       int         `json:"current_tab"`
}
