// Package common 提供TEMU平台处理器的共享类型定义
package common

import (
	"strings"

	temutemplate "task-processor/internal/platforms/temu/api/template"

	"github.com/sirupsen/logrus"
)

// PropertyFeature 属性特征信息
type PropertyFeature struct {
	PID             int    // 属性ID
	Name            string // 属性名称
	IsRequired      bool   // 是否必填
	IsPercentageSum bool   // 是否为百分比总和属性
	IsSelection     bool   // 是否为选择类型
	HasNumericInput bool   // 是否有数值输入
	HasValueUnit    bool   // 是否有值单位
	ValueUnit       string // 值单位
	RequiredSum     int    // 要求的总和值
	ControlType     int    // 控制类型
}

// PropertyFeatureDetector 属性特征识别器
type PropertyFeatureDetector struct {
	logger *logrus.Entry
}

// NewPropertyFeatureDetector 创建属性特征识别器
func NewPropertyFeatureDetector(logger *logrus.Entry) *PropertyFeatureDetector {
	return &PropertyFeatureDetector{
		logger: logger,
	}
}

// DetectFeatures 识别属性特征
func (d *PropertyFeatureDetector) DetectFeatures(templateProp temutemplate.TemplateRespGoodsProperty) PropertyFeature {
	feature := PropertyFeature{
		PID:         templateProp.PID,
		Name:        templateProp.Name,
		IsRequired:  templateProp.Required,
		ControlType: templateProp.ControlType,
	}

	// 识别是否为选择类型
	feature.IsSelection = len(templateProp.Values) > 0

	// 识别是否需要数值输入
	feature.HasNumericInput = templateProp.ControlType == 16

	// 识别值单位
	if len(templateProp.ValueUnit) > 0 {
		feature.HasValueUnit = true
		feature.ValueUnit = templateProp.ValueUnit[0]
	} else if len(templateProp.ValueUnitDTOList) > 0 {
		feature.HasValueUnit = true
		feature.ValueUnit = templateProp.ValueUnitDTOList[0].ValueUnit
	}

	// 识别百分比总和属性
	// 规则：ControlType=16（数值输入选择类型）且 ValueUnit 包含 "%"
	if feature.HasNumericInput && d.isPercentageUnit(feature.ValueUnit) {
		feature.IsPercentageSum = true
		feature.RequiredSum = 100
		d.logger.Debugf("🔍 识别到百分比总和属性: %s (PID=%d)", templateProp.Name, templateProp.PID)
	}

	return feature
}

// isPercentageUnit 判断是否为百分比单位
func (d *PropertyFeatureDetector) isPercentageUnit(unit string) bool {
	unit = strings.TrimSpace(unit)
	return unit == "%" || unit == "percent" || unit == "percentage"
}

// DetectAllFeatures 批量识别属性特征
func (d *PropertyFeatureDetector) DetectAllFeatures(templateProps []temutemplate.TemplateRespGoodsProperty) map[int]PropertyFeature {
	features := make(map[int]PropertyFeature)

	for _, templateProp := range templateProps {
		feature := d.DetectFeatures(templateProp)
		features[templateProp.PID] = feature
	}

	d.logger.Debugf("✅ 完成 %d 个属性的特征识别", len(features))
	return features
}

// DimensionInfo 尺寸信息
type DimensionInfo struct {
	Length string // 长度（英寸）
	Width  string // 宽度（英寸）
	Height string // 高度（英寸）
}
