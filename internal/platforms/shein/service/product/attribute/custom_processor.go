// Package attribute 提供SHEIN平台的自定义属性处理功能
package attribute

import (
	"strings"

	"task-processor/internal/platforms/shein/api/attribute"
	"task-processor/internal/platforms/shein/model"
	"task-processor/internal/platforms/shein/utils"

	"github.com/sirupsen/logrus"
)

// CustomAttributeProcessor 自定义属性处理器，负责创建和验证自定义属性值
type CustomAttributeProcessor struct{}

// NewCustomAttributeProcessor 创建新的自定义属性处理器
// 返回值:
//   - *CustomAttributeProcessor: 自定义属性处理器实例
func NewCustomAttributeProcessor() *CustomAttributeProcessor {
	return &CustomAttributeProcessor{}
}

// ProcessCustomAttributeValue 处理自定义属性值的统一函数
// 参数:
//   - ctx: 任务上下文，包含必要的客户端和数据
//   - attrID: 属性ID
//   - attrValue: 属性值
//   - isRequired: 是否为必需属性（主要属性通常是必需的，次要属性可能不是）
//
// 返回值:
//   - CustomAttributeResult: 处理结果，包含成功状态、新属性值ID等信息
func (p *CustomAttributeProcessor) ProcessCustomAttributeValue(
	ctx *model.TaskContext,
	attrID int,
	attrValue string,
	isRequired bool,
) model.CustomAttributeResult {

	logrus.Infof("处理自定义属性值: 属性ID %d, 原始值 %s, 必需: %v", attrID, attrValue, isRequired)

	// 0. 清理属性值中的特殊字符
	sanitizedValue := utils.SanitizeForSheinAttribute(attrValue)
	if sanitizedValue != attrValue {
		logrus.Infof("属性值已清理: 原始值 '%s' -> 清理后 '%s'", attrValue, sanitizedValue)
	}

	// 检查清理后的值是否有效
	if !utils.IsValidForSheinAttribute(sanitizedValue) {
		logrus.Errorf("清理后的属性值仍然无效: %s", sanitizedValue)
		return model.CustomAttributeResult{
			Success:        false,
			ShouldContinue: !isRequired,
		}
	}

	// 1. 验证自定义属性值（使用清理后的值）
	validateResponse, err := ctx.AttributeAPI.ValidateCustomAttributeValue(attrID, sanitizedValue, ctx.ProductData.CategoryID, ctx.AmazonProduct.Title)
	if err != nil {
		logrus.Errorf("验证自定义属性值失败: %v", err)
		return model.CustomAttributeResult{
			Success:        false,
			ShouldContinue: !isRequired, // 非必需属性失败时继续，必需属性失败时不继续
		}
	}

	if validateResponse.Data.AttributeID == 0 {
		logrus.Errorf("验证自定义属性值失败，属性ID为0")
		return model.CustomAttributeResult{
			Success:        false,
			ShouldContinue: !isRequired,
		}
	}

	// 修复 Shein 接口翻译导致的中文逗号问题
	p.fixTranslatedCommas(&validateResponse.Data.AttributeValueNameMultis)

	// 转换多语言名称
	nameMultis := p.convertToAttributeValueNameMultis(validateResponse.Data.AttributeValueNameMultis)

	// 验证多语言名称不为空
	if len(nameMultis) == 0 {
		logrus.Errorf("验证响应中的多语言名称为空，属性值: %s", sanitizedValue)
		return model.CustomAttributeResult{
			Success:        false,
			ShouldContinue: !isRequired,
		}
	}

	logrus.Debugf("多语言名称数量: %d", len(nameMultis))
	for i, nm := range nameMultis {
		logrus.Debugf("  [%d] 语言: %s, 名称: %s, 警告类型: %d", i, nm.Language, nm.AttributeValueName, nm.WarningType)
	}

	// 2. 添加自定义属性值（使用清理后的值）
	addResponse, err := ctx.AttributeAPI.AddCustomAttributeValue(&attribute.AddCustomAttributeValueRequest{
		CategoryID: ctx.ProductData.CategoryID,
		PreAttributeValueList: []attribute.PreAttributeValue{
			{
				AttributeID:              attrID,
				AttributeValue:           sanitizedValue, // 使用清理后的值
				PreAttributeValueID:      int64(validateResponse.Data.PreAttributeValueID),
				AttributeValueNameMultis: nameMultis,
			},
		},
	})

	if err != nil {
		logrus.Errorf("添加自定义属性值失败: %v", err)
		return model.CustomAttributeResult{
			Success:        false,
			ShouldContinue: !isRequired,
		}
	}

	// 3. 处理响应结果
	if len(addResponse.Info.Data.CustomAttributeRelation) > 0 {
		newValueID := int(addResponse.Info.Data.CustomAttributeRelation[0].AttributeValueID)
		logrus.Infof("成功添加自定义属性值，新的属性值ID: %d", newValueID)

		return model.CustomAttributeResult{
			Success:        true,
			NewValueID:     newValueID,
			Relations:      addResponse.Info.Data.CustomAttributeRelation,
			ShouldContinue: true,
		}
	}

	logrus.Errorf("添加自定义属性值失败，没有返回属性值ID")
	return model.CustomAttributeResult{
		Success:        false,
		ShouldContinue: !isRequired,
	}
}

// convertToAttributeValueNameMultis 转换属性值名称多语言结构
func (p *CustomAttributeProcessor) convertToAttributeValueNameMultis(source []struct {
	Language                string `json:"language"`
	AttributeValueNameMulti string `json:"attribute_value_name_multi"`
	WarningType             int    `json:"warning_type"`
}) []attribute.AttributeValueNameMulti {
	if len(source) == 0 {
		logrus.Warn("源多语言名称列表为空")
		return []attribute.AttributeValueNameMulti{}
	}

	result := make([]attribute.AttributeValueNameMulti, 0, len(source))
	for i, item := range source {
		// 验证必填字段
		if item.Language == "" {
			logrus.Warnf("跳过第 %d 个多语言名称：语言代码为空", i)
			continue
		}
		if item.AttributeValueNameMulti == "" {
			logrus.Warnf("跳过第 %d 个多语言名称：属性值名称为空 (语言: %s)", i, item.Language)
			continue
		}

		result = append(result, attribute.AttributeValueNameMulti{
			Language:           item.Language,
			AttributeValueName: item.AttributeValueNameMulti,
			WarningType:        item.WarningType,
		})

		logrus.Debugf("转换多语言名称 [%d]: 语言=%s, 名称=%s", i, item.Language, item.AttributeValueNameMulti)
	}

	if len(result) == 0 {
		logrus.Warn("转换后的多语言名称列表为空，所有条目都被过滤")
	}

	return result
}

// fixTranslatedCommas 修复 Shein 接口翻译导致的中文逗号问题
// Shein 接口会将英文逗号翻译为中文逗号，需要转换回来
func (p *CustomAttributeProcessor) fixTranslatedCommas(nameMultis *[]struct {
	Language                string `json:"language"`
	AttributeValueNameMulti string `json:"attribute_value_name_multi"`
	WarningType             int    `json:"warning_type"`
}) {
	if nameMultis == nil || len(*nameMultis) == 0 {
		return
	}

	for i := range *nameMultis {
		original := (*nameMultis)[i].AttributeValueNameMulti
		// 将中文逗号替换为英文逗号
		fixed := strings.ReplaceAll(original, "，", ",")

		if fixed != original {
			logrus.Infof("修复翻译后的逗号: 语言=%s, 原始='%s' -> 修复后='%s'",
				(*nameMultis)[i].Language, original, fixed)
			(*nameMultis)[i].AttributeValueNameMulti = fixed
		}
	}
}
