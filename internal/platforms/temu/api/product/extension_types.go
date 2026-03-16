package product

import "encoding/json"

// ExtensionInfo 扩展信息
type ExtensionInfo struct {
	GoodsProperty             GoodsPropertys      `json:"goods_property"`
	CertificationInfo         CertificationInfo   `json:"certification_info"`
	GoodsTrademark            GoodsTrademark      `json:"goods_trademark,omitempty"`
	GoodsProductTaxCodeDetail any         `json:"goods_product_tax_code_detail,omitempty"`
	GoodsOriginInfo           GoodsOriginInfo     `json:"goods_origin_info"`
	GoodsDesc                 string              `json:"goods_desc,omitempty"`
	BulletPoints              []string            `json:"bullet_points,omitempty"`
	SecondHand                any         `json:"second_hand,omitempty"`
	GuideFileInfo             any         `json:"guide_file_info,omitempty"`
	SizeChartImageInfo        *SizeChartImageInfo `json:"size_chart_image_info,omitempty"`
}

// GoodsPropertys 商品属性集合
type GoodsPropertys struct {
	GoodsBrandProperties []any      `json:"goods_brand_properties"`
	GoodsProperties      []PropertyItem     `json:"goods_properties"`
	GoodsSpecProperties  []GoodSpecProperty `json:"goods_spec_properties"`
}

func (g GoodsPropertys) MarshalJSON() ([]byte, error) {
	result := make(map[string]any)
	if len(g.GoodsBrandProperties) > 0 {
		result["goods_brand_properties"] = g.GoodsBrandProperties
	}
	if len(g.GoodsProperties) > 0 {
		result["goods_properties"] = g.GoodsProperties
	}
	result["goods_spec_properties"] = g.GoodsSpecProperties
	return json.Marshal(result)
}

// GoodSpecProperty 商品规格属性
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
	Vid              int    `json:"vid,omitempty"`
	TemplateModuleID int    `json:"template_module_id,omitempty"`
	TemplatePid      int    `json:"template_pid,omitempty"`
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

// CertificationInfo 认证信息
type CertificationInfo struct {
	CertificateInfo map[string]any `json:"certificate_info"`
	ExtraTemplate   ExtraTemplate  `json:"extra_template"`
	ActualPhoto     ActualPhoto    `json:"actual_photo,omitempty"`
	GpsrInfo        GpsrInfo       `json:"gpsr_info,omitempty"`
	RepInfo         RepInfo        `json:"rep_info,omitempty"`
}

func (c CertificationInfo) MarshalJSON() ([]byte, error) {
	result := make(map[string]any)
	if len(c.CertificateInfo) > 0 {
		result["certificate_info"] = c.CertificateInfo
	}
	result["extra_template"] = c.ExtraTemplate
	if len(c.ActualPhoto.Ext) > 0 || len(c.ActualPhoto.ActualPhotoInfoList) > 0 {
		result["actual_photo"] = c.ActualPhoto
	}
	if len(c.GpsrInfo.Ext) > 0 {
		result["gpsr_info"] = c.GpsrInfo
	}
	if len(c.RepInfo.Ext) > 0 {
		result["rep_info"] = c.RepInfo
	}
	return json.Marshal(result)
}

// ExtraTemplate 额外模板
type ExtraTemplate struct {
	ExtraTemplateDetailList []ExtraTemplateDetail `json:"extra_template_detail_list"`
}

// ExtraTemplateDetail 额外模板详情
type ExtraTemplateDetail struct {
	TemplateID int              `json:"template_id"`
	Properties map[string][]int `json:"properties"`
	InputText  map[string]any   `json:"input_text"`
}

// ActualPhoto 实际照片
type ActualPhoto struct {
	Ext                 map[string]any `json:"ext"`
	ActualPhotoInfoList map[string]any `json:"actual_photo_info_list"`
}

// GpsrInfo GPSR信息
type GpsrInfo struct {
	Ext map[string]any `json:"ext"`
}

// RepInfo REP信息
type RepInfo struct {
	Ext map[string]any `json:"ext"`
}

// SizeChartImageInfo 尺寸图表信息
type SizeChartImageInfo struct {
	SizeChartImageList []ImageInfo `json:"size_chart_image_list"`
}

func (s *SizeChartImageInfo) MarshalJSON() ([]byte, error) {
	if s == nil || len(s.SizeChartImageList) == 0 {
		return []byte("null"), nil
	}
	type Alias SizeChartImageInfo
	return json.Marshal((*Alias)(s))
}
