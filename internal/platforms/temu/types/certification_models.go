// Package types 提供TEMU平台的认证信息数据结构定义
package types

import "encoding/json"

// CertificationInfo 认证信息
type CertificationInfo struct {
	CertificateInfo map[string]any `json:"certificate_info"`
	ExtraTemplate   ExtraTemplate  `json:"extra_template"`
	ActualPhoto     ActualPhoto    `json:"actual_photo,omitempty"`
	GpsrInfo        GpsrInfo       `json:"gpsr_info,omitempty"`
	RepInfo         RepInfo        `json:"rep_info,omitempty"`
}

// MarshalJSON 实现自定义JSON序列化
func (c CertificationInfo) MarshalJSON() ([]byte, error) {
	result := make(map[string]interface{})

	// 只添加非空的字段
	if len(c.CertificateInfo) > 0 {
		result["certificate_info"] = c.CertificateInfo
	}

	// ExtraTemplate 总是包含（因为它是必需的）
	result["extra_template"] = c.ExtraTemplate

	// 检查 ActualPhoto 是否为空
	if !c.isActualPhotoEmpty() {
		result["actual_photo"] = c.ActualPhoto
	}

	// 检查 GpsrInfo 是否为空
	if !c.isGpsrInfoEmpty() {
		result["gpsr_info"] = c.GpsrInfo
	}

	// 检查 RepInfo 是否为空
	if !c.isRepInfoEmpty() {
		result["rep_info"] = c.RepInfo
	}

	return json.Marshal(result)
}

// isActualPhotoEmpty 检查 ActualPhoto 是否为空
func (c CertificationInfo) isActualPhotoEmpty() bool {
	return len(c.ActualPhoto.Ext) == 0 &&
		len(c.ActualPhoto.ActualPhotoInfoList) == 0
}

// isGpsrInfoEmpty 检查 GpsrInfo 是否为空
func (c CertificationInfo) isGpsrInfoEmpty() bool {
	return len(c.GpsrInfo.Ext) == 0
}

// isRepInfoEmpty 检查 RepInfo 是否为空
func (c CertificationInfo) isRepInfoEmpty() bool {
	return len(c.RepInfo.Ext) == 0
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
	Ext                 map[string]interface{} `json:"ext"`
	ActualPhotoInfoList map[string]interface{} `json:"actual_photo_info_list"`
}

// GpsrInfo GPSR信息
type GpsrInfo struct {
	Ext map[string]interface{} `json:"ext"`
}

// RepInfo REP信息
type RepInfo struct {
	Ext map[string]interface{} `json:"ext"`
}
