// Package attribute 提供SHEIN平台的属性映射功能，包括属性值映射、平台值匹配等功能
package attribute

import (
	"fmt"
	"task-processor/internal/pkg/types"
	"task-processor/internal/platforms/shein"
	"task-processor/internal/platforms/shein/api/attribute"

	"github.com/sirupsen/logrus"
)

// AttributeMapper 属性映射器，负责将Amazon属性值映射到SHEIN平台属性值ID
type AttributeMapper struct {
	valueMatcher *AttributeValueMatcher
	processor    *CustomAttributeProcessor
}

// NewAttributeMapper 创建新的属性映射器
// 返回值:
//   - *AttributeMapper: 属性映射器实例
func NewAttributeMapper() *AttributeMapper {
	return &AttributeMapper{
		valueMatcher: NewAttributeValueMatcher(),
		processor:    NewCustomAttributeProcessor(),
	}
}

// MapAttributeValuesToSheinIDs 将Amazon属性值映射到SHEIN平台属性值ID
// 参数:
//   - ctx: 任务上下文，包含必要的客户端和数据
//   - strategy: 属性策略，包含主要和次要属性信息
//
// 返回值:
//   - []attribute.CustomAttributeRelation: 自定义属性关系列表
//   - error: 错误信息
func (m *AttributeMapper) MapAttributeValuesToSheinIDs(ctx *shein.TaskContext, strategy *shein.AttributeStrategy) ([]attribute.CustomAttributeRelation, error) {
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
func (m *AttributeMapper) mapSingleAttributeValues(ctx *shein.TaskContext, attr *shein.ResultAttribute, isRequired bool) ([]attribute.CustomAttributeRelation, error) {
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
	platformValues := m.valueMatcher.GetPlatformAttributeValues(attr.AttrID, ctx.AttributeTemplates)
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
		if platformID := m.valueMatcher.FindMatchingPlatformValue(attrValue.Value, platformValues); platformID > 0 {
			attr.AttrValue[i].ID = types.FlexibleID(platformID)
			logrus.Infof("✓ 属性值 '%s' 映射到平台ID: %d", attrValue.Value, platformID)
			continue
		}
		logrus.Debugf("✗ 属性值 '%s' 在平台中未找到匹配", attrValue.Value)

		// 如果没有找到匹配，需要创建自定义属性值
		logrus.Infof("属性值 %s 在平台中不存在，需要创建自定义值", attrValue.Value)
		result := m.processor.ProcessCustomAttributeValue(ctx,
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
		attr.AttrValue[i].ID = types.FlexibleID(result.NewValueID)
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
