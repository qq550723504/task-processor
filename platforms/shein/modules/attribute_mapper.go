package modules

import (
	"fmt"
	"strings"
	"task-processor/common/shein/api/attribute"

	"github.com/sirupsen/logrus"
)

// AttributeMapper 属性映射器
type AttributeMapper struct{}

// NewAttributeMapper 创建新的属性映射器
func NewAttributeMapper() *AttributeMapper {
	return &AttributeMapper{}
}

// MapAttributeValuesToSheinIDs 将Amazon属性值映射到SHEIN平台属性值ID
func (m *AttributeMapper) MapAttributeValuesToSheinIDs(ctx *TaskContext, strategy *AttributeStrategy) ([]attribute.CustomAttributeRelation, error) {
	logrus.Infof("🔄 === 开始属性值ID映射流程 ===")
	
	var allRelations []attribute.CustomAttributeRelation

	// 处理主要属性值
	logrus.Infof("📊 步骤1: 处理主要属性值...")
	logrus.Infof("  - 主要属性ID: %d", strategy.PrimaryAttribute.AttrID)
	logrus.Infof("  - 主要属性值数量: %d", len(strategy.PrimaryAttribute.AttrValue))
	
	// 打印映射前的主要属性值状态
	for i, attrValue := range strategy.PrimaryAttribute.AttrValue {
		logrus.Infof("  - 主要属性值[%d]: %s (映射前ID: %d)", i+1, attrValue.Value, attrValue.ID.Int())
	}
	
	relations, err := m.mapSingleAttributeValues(ctx, &strategy.PrimaryAttribute, true)
	if err != nil {
		logrus.Errorf("❌ 映射主要属性值失败: %v", err)
		return nil, fmt.Errorf("映射主要属性值失败: %w", err)
	}
	allRelations = append(allRelations, relations...)
	logrus.Infof("✅ 主要属性值映射完成，创建了 %d 个关系", len(relations))

	// 处理次要属性值（如果存在）
	if strategy.SecondaryAttribute.AttrID > 0 && len(strategy.SecondaryAttribute.AttrValue) > 0 {
		logrus.Infof("📊 步骤2: 处理次要属性值...")
		logrus.Infof("  - 次要属性ID: %d", strategy.SecondaryAttribute.AttrID)
		logrus.Infof("  - 次要属性值数量: %d", len(strategy.SecondaryAttribute.AttrValue))
		
		// 打印映射前的次要属性值状态
		for i, attrValue := range strategy.SecondaryAttribute.AttrValue {
			logrus.Infof("  - 次要属性值[%d]: %s (映射前ID: %d)", i+1, attrValue.Value, attrValue.ID.Int())
		}
		
		relations, err := m.mapSingleAttributeValues(ctx, &strategy.SecondaryAttribute, false)
		if err != nil {
			logrus.Errorf("❌ 映射次要属性值失败: %v", err)
			return nil, fmt.Errorf("映射次要属性值失败: %w", err)
		}
		allRelations = append(allRelations, relations...)
		logrus.Infof("✅ 次要属性值映射完成，创建了 %d 个关系", len(relations))
	} else {
		logrus.Infof("⏭️ 跳过次要属性值处理（无次要属性）")
	}

	// 调试：记录映射后的整体状态
	logrus.Infof("📋 === 映射后整体状态 ===")
	logrus.Infof("主要属性ID: %d, 属性值数量: %d", strategy.PrimaryAttribute.AttrID, len(strategy.PrimaryAttribute.AttrValue))
	validPrimaryCount := 0
	for i, attrValue := range strategy.PrimaryAttribute.AttrValue {
		status := "❌"
		if attrValue.ID.Int() > 0 {
			status = "✅"
			validPrimaryCount++
		}
		logrus.Infof("  主要属性值[%d]: %s (映射后ID: %d) %s", i+1, attrValue.Value, attrValue.ID.Int(), status)
	}
	logrus.Infof("主要属性值有效率: %d/%d (%.1f%%)", validPrimaryCount, len(strategy.PrimaryAttribute.AttrValue), 
		float64(validPrimaryCount)*100/float64(len(strategy.PrimaryAttribute.AttrValue)))
	
	if strategy.SecondaryAttribute.AttrID > 0 {
		logrus.Infof("次要属性ID: %d, 属性值数量: %d", strategy.SecondaryAttribute.AttrID, len(strategy.SecondaryAttribute.AttrValue))
		validSecondaryCount := 0
		for i, attrValue := range strategy.SecondaryAttribute.AttrValue {
			status := "❌"
			if attrValue.ID.Int() > 0 {
				status = "✅"
				validSecondaryCount++
			}
			logrus.Infof("  次要属性值[%d]: %s (映射后ID: %d) %s", i+1, attrValue.Value, attrValue.ID.Int(), status)
		}
		logrus.Infof("次要属性值有效率: %d/%d (%.1f%%)", validSecondaryCount, len(strategy.SecondaryAttribute.AttrValue), 
			float64(validSecondaryCount)*100/float64(len(strategy.SecondaryAttribute.AttrValue)))
	}

	logrus.Infof("🎉 属性值映射完成，总共创建了 %d 个自定义属性关系", len(allRelations))
	return allRelations, nil
}

// mapSingleAttributeValues 映射单个属性的所有属性值
func (m *AttributeMapper) mapSingleAttributeValues(ctx *TaskContext, attr *ResultAttribute, isRequired bool) ([]attribute.CustomAttributeRelation, error) {
	if attr.AttrID <= 0 || len(attr.AttrValue) == 0 {
		return nil, nil
	}

	var relations []attribute.CustomAttributeRelation

	logrus.Infof("映射属性ID %d 的属性值，数量: %d", attr.AttrID, len(attr.AttrValue))

	// 调试：记录映射前的属性值ID状态
	for i, attrValue := range attr.AttrValue {
		logrus.Debugf("映射前 - 属性值[%d]: %s (ID: %d)", i, attrValue.Value, attrValue.ID.Int())
	}

	// 获取SHEIN平台该属性的所有可用属性值
	platformValues := m.getPlatformAttributeValues(attr.AttrID, ctx.AttributeTemplates)
	logrus.Infof("平台属性ID %d 的可用值数量: %d", attr.AttrID, len(platformValues))

	// 调试：打印前几个平台值
	count := 0
	for value, id := range platformValues {
		if count < 5 {
			logrus.Debugf("平台值示例: %s -> %d", value, id)
			count++
		} else {
			break
		}
	}

	for i := 0; i < len(attr.AttrValue); i++ {
		attrValue := &attr.AttrValue[i]

		// 跳过已经有有效ID的属性值
		if attrValue.ID.Int() > 0 {
			logrus.Debugf("属性值 %s 已有有效ID: %d，跳过", attrValue.Value, attrValue.ID.Int())
			continue
		}

		// 尝试在平台已有值中找到匹配
		logrus.Debugf("尝试为属性值 '%s' 查找平台匹配", attrValue.Value)
		if platformID := m.findMatchingPlatformValue(attrValue.Value, platformValues); platformID > 0 {
			attr.AttrValue[i].ID = FlexibleID(platformID)
			logrus.Infof("✓ 属性值 '%s' 映射到平台ID: %d", attrValue.Value, platformID)
			continue
		}
		logrus.Debugf("✗ 属性值 '%s' 在平台中未找到匹配", attrValue.Value)

		// 如果没有找到匹配，需要创建自定义属性值
		logrus.Infof("属性值 %s 在平台中不存在，需要创建自定义值", attrValue.Value)
		result := m.processCustomAttributeValue(ctx,
			attr.AttrID,
			attrValue.Value,
			isRequired,
		)

		if !result.Success {
			if !result.ShouldContinue {
				return nil, fmt.Errorf("创建自定义属性值失败: %s", attrValue.Value)
			}
			logrus.Warnf("创建自定义属性值失败但继续: %s", attrValue.Value)
			// 对于失败的情况，保持原有的负数ID，后续会被跳过
			continue
		}

		// 更新为新创建的属性值ID
		attr.AttrValue[i].ID = FlexibleID(result.NewValueID)
		relations = append(relations, result.Relations...)
		logrus.Infof("成功创建自定义属性值: %s -> ID: %d", attrValue.Value, result.NewValueID)
	}

	// 调试：记录映射后的属性值ID状态
	logrus.Debugf("映射完成 - 属性ID %d 的最终状态:", attr.AttrID)
	for i, attrValue := range attr.AttrValue {
		logrus.Debugf("映射后 - 属性值[%d]: %s (ID: %d)", i, attrValue.Value, attrValue.ID.Int())
	}

	return relations, nil
}

// findMatchingPlatformValue 在平台值中查找匹配的属性值
func (m *AttributeMapper) findMatchingPlatformValue(value string, platformValues map[string]int) int {
	if value == "" {
		return 0
	}

	// 1. 精确匹配（优先级最高）
	if id, exists := platformValues[value]; exists {
		logrus.Debugf("✓ 属性值 '%s' 找到精确匹配，平台ID: %d", value, id)
		return id
	}

	// 2. 忽略大小写匹配
	lowerValue := strings.ToLower(strings.TrimSpace(value))
	if id, exists := platformValues[lowerValue]; exists {
		logrus.Debugf("✓ 属性值 '%s' 找到大小写匹配，平台ID: %d", value, id)
		return id
	}

	// 3. 清理特殊字符后匹配
	sanitizedValue := SanitizeForSheinAttribute(value)
	if sanitizedValue != value {
		logrus.Debugf("尝试使用清理后的值进行匹配: '%s' -> '%s'", value, sanitizedValue)

		// 使用清理后的值进行精确匹配
		if id, exists := platformValues[sanitizedValue]; exists {
			logrus.Debugf("✓ 属性值 '%s' 清理后找到精确匹配，平台ID: %d", value, id)
			return id
		}

		// 使用清理后的值进行大小写匹配
		lowerSanitized := strings.ToLower(strings.TrimSpace(sanitizedValue))
		if id, exists := platformValues[lowerSanitized]; exists {
			logrus.Debugf("✓ 属性值 '%s' 清理后找到大小写匹配，平台ID: %d", value, id)
			return id
		}
	}

	// 4. 模糊匹配（仅去除特殊字符，不做颜色标准化）
	normalizedValue := m.normalizeValueForMatching(value)
	for platformValue, id := range platformValues {
		if m.normalizeValueForMatching(platformValue) == normalizedValue {
			logrus.Debugf("✓ 属性值 '%s' 找到模糊匹配，平台ID: %d", value, id)
			return id
		}
	}

	// 移除颜色特殊处理，保持Amazon原始属性值的完整性
	logrus.Debugf("✗ 属性值 '%s' 在平台中未找到任何匹配", value)
	return 0
}

// normalizeValueForMatching 标准化属性值用于匹配
func (m *AttributeMapper) normalizeValueForMatching(value string) string {
	// 只做最基本的标准化：转小写和去除首尾空格
	normalized := strings.ToLower(strings.TrimSpace(value))
	// 只去除多余的空格，保留其他字符以维持属性值的唯一性
	normalized = strings.Join(strings.Fields(normalized), " ")
	return normalized
}

// getPlatformAttributeValues 获取平台属性的所有可用值
func (m *AttributeMapper) getPlatformAttributeValues(attrID int, attributeTemplates *attribute.AttributeTemplateInfo) map[string]int {
	platformValues := make(map[string]int)

	if attributeTemplates == nil || len(attributeTemplates.Data) == 0 {
		logrus.Warnf("属性模板为空，无法获取平台属性值")
		return platformValues
	}

	// 遍历属性模板找到对应的属性
	for _, template := range attributeTemplates.Data {
		for _, attrInfo := range template.AttributeInfos {
			if attrInfo.AttributeID == attrID {
				// 提取属性值列表
				for _, valueInfo := range attrInfo.AttributeValueInfoList {
					if valueInfo.AttributeValueID > 0 && valueInfo.AttributeValue != "" {
						// 支持多种匹配方式
						platformValues[strings.ToLower(strings.TrimSpace(valueInfo.AttributeValue))] = valueInfo.AttributeValueID
						// 也存储原始值
						platformValues[valueInfo.AttributeValue] = valueInfo.AttributeValueID
						// 如果有英文值，也存储
						if valueInfo.AttributeValueEn != "" {
							platformValues[strings.ToLower(strings.TrimSpace(valueInfo.AttributeValueEn))] = valueInfo.AttributeValueID
							platformValues[valueInfo.AttributeValueEn] = valueInfo.AttributeValueID
						}
					}
				}
				logrus.Debugf("属性ID %d 找到 %d 个平台属性值", attrID, len(platformValues)/2) // 除以2因为每个值存储了两次
				return platformValues
			}
		}
	}

	logrus.Warnf("在属性模板中未找到属性ID %d", attrID)
	return platformValues
}

// processCustomAttributeValue 处理自定义属性值的统一函数
func (m *AttributeMapper) processCustomAttributeValue(
	ctx *TaskContext,
	attrID int,
	attrValue string,
	isRequired bool, // 是否为必需属性（主要属性通常是必需的，次要属性可能不是）
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

	// 2. 添加自定义属性值（使用清理后的值）
	addResponse, err := ctx.ShopClient.AddCustomAttributeValue(&attribute.AddCustomAttributeValueRequest{
		CategoryID: ctx.ProductData.CategoryID,
		PreAttributeValueList: []attribute.PreAttributeValue{
			{
				AttributeID:              attrID,
				AttributeValue:           sanitizedValue, // 使用清理后的值
				PreAttributeValueID:      int64(validateResponse.Data.PreAttributeValueID),
				AttributeValueNameMultis: convertToAttributeValueNameMultis(validateResponse.Data.AttributeValueNameMultis),
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
func convertToAttributeValueNameMultis(source []struct {
	Language                string `json:"language"`
	AttributeValueNameMulti string `json:"attribute_value_name_multi"`
	WarningType             int    `json:"warning_type"`
}) []attribute.AttributeValueNameMulti {
	result := make([]attribute.AttributeValueNameMulti, len(source))
	for i, item := range source {
		result[i] = attribute.AttributeValueNameMulti{
			Language:           item.Language,
			AttributeValueName: item.AttributeValueNameMulti,
			WarningType:        item.WarningType,
		}
	}
	return result
}
