// Package modules 提供SHEIN平台的自定义属性处理功能
package modules

import (
	"task-processor/internal/common/shein/api/attribute"

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
	ctx *TaskContext,
	attrID int,
	attrValue string,
	isRequired bool,
) CustomAttributeResult {

	logrus.Infof("处理自定义属性值: 属性ID %d, 原始值 %s, 必需: %v", attrID, attrValue, isRequired)

	// 0. 清理属性值中的特殊字符
	sanitizedValue := SanitizeForSheinAttribute(attrValue)
	if sanitizedValue != attrValue {
		logrus.Infof("属性值已清理: 原始值 '%s' -> 清理后 '%s'", attrValue, sanitizedValue)
	}

	// 检查清理后的值是否有效
	if !IsValidForSheinAttribute(sanitizedValue) {
		logrus.Errorf("清理后的属性值仍然无效: %s", sanitizedValue)
		return CustomAttributeResult{
			Success:        false,
			ShouldContinue: !isRequired,
		}
	}

	// 1. 验证自定义属性值（使用清理后的值）
	validateResponse, err := ctx.ShopClient.ValidateCustomAttributeValue(attrID, sanitizedValue, ctx.ProductData.CategoryID, ctx.AmazonProduct.Title)
	if err != nil {
		logrus.Errorf("验证自定义属性值失败: %v", err)
		return CustomAttributeResult{
			Success:        false,
			ShouldContinue: !isRequired, // 非必需属性失败时继续，必需属性失败时不继续
		}
	}

	if validateResponse.Data.AttributeID == 0 {
		logrus.Errorf("验证自定义属性值失败，属性ID为0")
		return CustomAttributeResult{
			Success:        false,
			ShouldContinue: !isRequired,
		}
	}

	// 记录验证响应的详细信息
	logrus.Debugf("验证响应: AttributeID=%d, PreAttributeValueID=%d, NameMultis数量=%d",
		validateResponse.Data.AttributeID,
		validateResponse.Data.PreAttributeValueID,
		len(validateResponse.Data.AttributeValueNameMultis))

	for i, nm := range validateResponse.Data.AttributeValueNameMultis {
		logrus.Debugf("  验证响应[%d]: 语言=%s, 名称=%s, 警告=%d",
			i, nm.Language, nm.AttributeValueNameMulti, nm.WarningType)
	}

	// 转换多语言名称
	nameMultis := p.convertToAttributeValueNameMultis(validateResponse.Data.AttributeValueNameMultis)

	// 验证多语言名称不为空
	if len(nameMultis) == 0 {
		logrus.Errorf("验证响应中的多语言名称为空，属性值: %s", sanitizedValue)
		return CustomAttributeResult{
			Success:        false,
			ShouldContinue: !isRequired,
		}
	}

	logrus.Debugf("多语言名称数量: %d", len(nameMultis))
	for i, nm := range nameMultis {
		logrus.Debugf("  [%d] 语言: %s, 名称: %s, 警告类型: %d", i, nm.Language, nm.AttributeValueName, nm.WarningType)
	}

	// 2. 添加自定义属性值（使用清理后的值）
	addResponse, err := ctx.ShopClient.AddCustomAttributeValue(&attribute.AddCustomAttributeValueRequest{
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
		return CustomAttributeResult{
			Success:        false,
			ShouldContinue: !isRequired,
		}
	}

	// 3. 处理响应结果
	if len(addResponse.Info.Data.CustomAttributeRelation) > 0 {
		newValueID := int(addResponse.Info.Data.CustomAttributeRelation[0].AttributeValueID)
		logrus.Infof("成功添加自定义属性值，新的属性值ID: %d", newValueID)

		return CustomAttributeResult{
			Success:        true,
			NewValueID:     newValueID,
			Relations:      addResponse.Info.Data.CustomAttributeRelation,
			ShouldContinue: true,
		}
	}

	logrus.Errorf("添加自定义属性值失败，没有返回属性值ID")
	return CustomAttributeResult{
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
