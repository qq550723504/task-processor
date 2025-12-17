package modules

import (
	"fmt"
	"strings"
	"task-processor/internal/common/shein/api/attribute"

	"github.com/sirupsen/logrus"
)

// ValidateRepairSaleAttributeHandler 验证修复销售属性处理器
type ValidateRepairSaleAttributeHandler struct {
}

// NewValidateRepairSaleAttributeHandler 创建新的验证修复销售属性处理器
func NewValidateRepairSaleAttributeHandler() *ValidateRepairSaleAttributeHandler {
	return &ValidateRepairSaleAttributeHandler{}
}

// Name 返回处理器名称
func (h *ValidateRepairSaleAttributeHandler) Name() string {
	return "验证修复销售属性"
}

// Handle 执行验证修复销售属性处理
func (h *ValidateRepairSaleAttributeHandler) Handle(ctx *TaskContext) error {
	// 检查是否已获取产品数据
	if ctx.ProductData == nil {
		return fmt.Errorf("产品数据未获取，请先执行获取产品数据步骤")
	}

	// 确保 ctx.SaleSpecResult 不为 nil
	if ctx.SaleSpecResult == nil {
		return fmt.Errorf("销售规格结果未获取，请先执行相关步骤")
	}

	// 修复属性值ID
	h.fixAttributeValueIDs(ctx.SaleSpecResult, ctx.AttributeTemplates)

	logrus.Println("销售属性验证修复完成")

	return nil
}

// 如果对应不上则修改为负数递增
func (h *ValidateRepairSaleAttributeHandler) fixAttributeValueIDs(saleAttributeData *ResultSaleAttribute, attributeTemplates *attribute.AttributeTemplateInfo) *ResultSaleAttribute {
	logrus.Info("🔧 开始修复属性值ID，确保与SHEIN平台一致")

	// 构建SHEIN平台属性值映射表
	platformValueMap := h.buildPlatformAttributeValueMap(attributeTemplates)

	// 用于生成负数ID的计数器
	negativeIDCounter := -1

	// 处理每个销售属性
	for attrIndex := range saleAttributeData.SaleAttributes {
		attr := &saleAttributeData.SaleAttributes[attrIndex]
		attrID := attr.AttrID

		logrus.Infof("处理属性ID %d 的属性值，数量: %d", attrID, len(attr.AttrValue))

		// 获取该属性在平台中的可用值
		platformValues, exists := platformValueMap[attrID]
		if !exists {
			logrus.Warnf("属性ID %d 在平台模板中不存在，将所有属性值设为负数ID", attrID)
			// 如果属性不存在，将所有值设为负数ID
			for valueIndex := range attr.AttrValue {
				attr.AttrValue[valueIndex].ID = FlexibleID(negativeIDCounter)
				logrus.Debugf("属性值 '%s' 设置为负数ID: %d", attr.AttrValue[valueIndex].Value, negativeIDCounter)
				negativeIDCounter--
			}
			continue
		}

		// 处理每个属性值
		for valueIndex := range attr.AttrValue {
			attrValue := &attr.AttrValue[valueIndex]
			originalValue := strings.TrimSpace(attrValue.Value)

			// 尝试在平台值中找到精确匹配
			if platformID, found := h.findExactMatch(originalValue, platformValues); found {
				attrValue.ID = FlexibleID(platformID)
				logrus.Debugf("✓ 属性值 '%s' 找到精确匹配，平台ID: %d", originalValue, platformID)
				continue
			}

			// 尝试模糊匹配
			if platformID, found := h.findFuzzyMatch(originalValue, platformValues); found {
				attrValue.ID = FlexibleID(platformID)
				logrus.Debugf("✓ 属性值 '%s' 找到模糊匹配，平台ID: %d", originalValue, platformID)
				continue
			}

			// 如果都没有匹配，设置为负数ID
			attrValue.ID = FlexibleID(negativeIDCounter)
			logrus.Debugf("✗ 属性值 '%s' 未找到匹配，设置为负数ID: %d", originalValue, negativeIDCounter)
			negativeIDCounter--
		}
	}

	return saleAttributeData
}

// buildPlatformAttributeValueMap 构建SHEIN平台属性值映射表
func (h *ValidateRepairSaleAttributeHandler) buildPlatformAttributeValueMap(attributeTemplates *attribute.AttributeTemplateInfo) map[int]map[string]int {
	platformValueMap := make(map[int]map[string]int)

	if attributeTemplates == nil || len(attributeTemplates.Data) == 0 {
		logrus.Warn("属性模板为空，无法构建平台属性值映射")
		return platformValueMap
	}

	for _, template := range attributeTemplates.Data {
		for _, attrInfo := range template.AttributeInfos {
			attrID := attrInfo.AttributeID
			valueMap := make(map[string]int)

			for _, valueInfo := range attrInfo.AttributeValueInfoList {
				// 使用英文值作为主要匹配键
				if valueInfo.AttributeValueEn != "" {
					valueMap[strings.TrimSpace(valueInfo.AttributeValueEn)] = valueInfo.AttributeValueID
				}
				// 同时使用中文值作为备用匹配键
				if valueInfo.AttributeValue != "" {
					valueMap[strings.TrimSpace(valueInfo.AttributeValue)] = valueInfo.AttributeValueID
				}
			}

			if len(valueMap) > 0 {
				platformValueMap[attrID] = valueMap
				logrus.Debugf("构建属性ID %d 的平台值映射，包含 %d 个值", attrID, len(valueMap))
			}
		}
	}

	logrus.Infof("构建平台属性值映射完成，包含 %d 个属性", len(platformValueMap))
	return platformValueMap
}

// findExactMatch 查找精确匹配的平台属性值ID
func (h *ValidateRepairSaleAttributeHandler) findExactMatch(value string, platformValues map[string]int) (int, bool) {
	// 直接匹配
	if id, exists := platformValues[value]; exists {
		return id, true
	}

	// 忽略大小写匹配
	valueLower := strings.ToLower(value)
	for platformValue, id := range platformValues {
		if strings.ToLower(platformValue) == valueLower {
			return id, true
		}
	}

	return 0, false
}

// findFuzzyMatch 模糊匹配平台属性值ID
func (h *ValidateRepairSaleAttributeHandler) findFuzzyMatch(value string, platformValues map[string]int) (int, bool) {
	valueLower := strings.ToLower(strings.TrimSpace(value))

	for platformValue, id := range platformValues {
		platformValueLower := strings.ToLower(strings.TrimSpace(platformValue))

		// 只进行基本的大小写忽略匹配，不做过度标准化
		if valueLower == platformValueLower {
			return id, true
		}

		// 移除过度的模糊匹配逻辑，避免不同的Amazon属性值被错误地映射到相同ID
		// 如果需要更精确的匹配，应该通过自定义属性值创建来处理
	}

	return 0, false
}
