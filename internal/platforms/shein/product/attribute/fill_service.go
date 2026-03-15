package attribute

import (
	"fmt"
	"task-processor/internal/platforms/shein/api/attribute"
	"task-processor/internal/platforms/shein/api/product"
	"task-processor/internal/platforms/shein"

	"github.com/sirupsen/logrus"
)

// FillAttributeHandler 填充属性处理器
type FillAttributeHandler struct {
}

// NewFillAttributeHandler 创建新的填充属性处理器
func NewFillAttributeHandler() *FillAttributeHandler {
	return &FillAttributeHandler{}
}

// Name 返回处理器名称
func (h *FillAttributeHandler) Name() string {
	return "填充属性"
}

// Handle 执行填充属性处理
func (h *FillAttributeHandler) Handle(ctx *shein.TaskContext) error {
	// 检查是否已获取生成的属性数据
	if ctx.GenerateAttribute == nil {
		return fmt.Errorf("生成的属性数据未获取，请先执行AI属性选择步骤")
	}

	// 检查是否已获取产品数据
	if ctx.ProductData == nil {
		return fmt.Errorf("产品数据未获取，请先执行获取产品数据步骤")
	}

	// 执行属性填充逻辑
	if err := h.fillProductAttributes(ctx); err != nil {
		return fmt.Errorf("填充属性失败: %w", err)
	}

	logrus.Println("属性填充完成")

	return nil
}

// fillProductAttributes 填充产品属性
func (h *FillAttributeHandler) fillProductAttributes(ctx *shein.TaskContext) error {
	productAttributeList := []product.ProductAttribute{}

	// 定义需要跳过非必填的特殊属性ID列表
	skipNonRequiredIDs := map[int]bool{
		62:      true, // 成分
		1001675: true, // 电池使用方式
		8:       true, // 包含电池
		1000462: true, // 敏感类别
		147:     true, // 电源
		1002019: true, // 抽屉数量
		1000101: true, // 电压
		164:     true, // 电压 (V)
		1001466: true, // 插头(电压)
		1000100: true, // 插电规格
		1000088: true, // 连接方式
		1002027: true, // 激光等级
		1000099: true, // 插电类产品认证
		1000105: true, // 涂层
	}

	// 创建属性ID到必填状态的映射
	requiredAttributeMap := make(map[int]bool)
	if ctx.AttributeTemplates != nil && len(ctx.AttributeTemplates.Data) > 0 {
		for _, attribute := range ctx.AttributeTemplates.Data[0].AttributeInfos {
			requiredAttributeMap[attribute.AttributeID] = h.isAttributeRequired(attribute)
		}
	}

	// 处理产品属性
	for _, attribute := range ctx.GenerateAttribute.AttributeData {
		// 跳过没有属性值的属性
		if len(attribute.AttrValue) == 0 {
			continue
		}

		// 对于指定的特殊属性ID，只有非必填时才跳过
		if skipNonRequiredIDs[attribute.AttrID] {
			if isRequired, exists := requiredAttributeMap[attribute.AttrID]; exists && !isRequired {
				continue // 跳过指定ID的非必填属性
			}
		}
		// 对于其他属性ID，无论是否必填都处理

		// 使用最后一个属性值
		lastAttrValue := attribute.AttrValue[len(attribute.AttrValue)-1]
		valueIDInt := lastAttrValue.ID.Int()
		valueID := &valueIDInt
		extraValue := ""

		// 处理attribute_value_id=0的手填值情况
		if valueIDInt == 0 {
			// 当ID为0时，说明是自定义值，将Value放入ExtraValue
			extraValue = lastAttrValue.Value
		}

		// 处理特殊属性ID的额外值
		switch attribute.AttrID {
		case 1000411: // 数量
			if valueIDInt != 0 {
				extraValue = "1"
			}
		case 62: // 成分
			if valueIDInt != 0 {
				extraValue = "100"
			}
		case 1000546: // 产品型号
			extraValue = "/"
			valueID = nil
		case 1002189, 1002188: // 主料克重（g/m²）
			extraValue = "150"
			valueID = nil
		case 1000078: // 里衬成分
			extraValue = "100"
		case 1000105: //无涂层
			if valueIDInt != 0 {
				extraValue = "100"
			}
		}

		productAttributeList = append(productAttributeList, product.ProductAttribute{
			AttributeID:         attribute.AttrID,
			AttributeValueID:    valueID,
			CVSuggestType:       "",
			AttributeExtraValue: extraValue,
		})
	}

	// 将填充好的属性列表保存到上下文中
	ctx.ProductData.ProductAttributeList = productAttributeList

	return nil
}

// isAttributeRequired 基于模板数据判断属性是否必填
func (h *FillAttributeHandler) isAttributeRequired(attribute attribute.AttributeInfo) bool {
	// 判断必填的优先级逻辑（基于实际模板数据）
	switch {
	case len(attribute.AttributeRemarkList) > 0:
		// 有备注列表的属性通常是必填的
		return true
	case attribute.AttributeLabel == 1:
		// AttributeLabel为1表示必填
		return true
	case attribute.AttributeStatus == 3:
		// 状态为3的属性通常是必填的
		return true
	case attribute.AttributeIsShow == 1:
		// 显示标记为1且有其他必填特征
		//return len(attribute.AttributeValueInfoList) > 0 && attribute.AttributeSource == 1
		return false
	default:
		return false
	}
}


