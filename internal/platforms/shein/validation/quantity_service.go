// Package modules 提供SHEIN平台数量信息验证工具
package validation

import (
	"fmt"
)

// QuantityValidator 数量信息验证器
type QuantityValidator struct{}

// NewQuantityValidator 创建数量信息验证器
func NewQuantityValidator() *QuantityValidator {
	return &QuantityValidator{}
}

// ValidateQuantityMapping 验证数量类型和单位类型的映射关系
// 根据SHEIN业务规则：
// - quantityType 为单品=1、同款多件=2、单套=3、多套=4
// - quantityUnit 单位类型 件=1，双=2，套=3
// - 当quantityType为3(单套)或4(多套)时，quantityUnit必须为3(套)
func (v *QuantityValidator) ValidateQuantityMapping(quantityType, quantityUnit int) error {
	switch quantityType {
	case 1: // 单品
		if quantityUnit != 1 && quantityUnit != 2 {
			return fmt.Errorf("单品类型(quantityType=1)的单位只能是件(1)或双(2)，当前为: %d", quantityUnit)
		}
	case 2: // 同款多件
		if quantityUnit != 1 {
			return fmt.Errorf("同款多件类型(quantityType=2)的单位只能是件(1)，当前为: %d", quantityUnit)
		}
	case 3: // 单套
		if quantityUnit != 3 {
			return fmt.Errorf("单套类型(quantityType=3)的单位只能是套(3)，当前为: %d", quantityUnit)
		}
	case 4: // 多套
		if quantityUnit != 3 {
			return fmt.Errorf("多套类型(quantityType=4)的单位只能是套(3)，当前为: %d", quantityUnit)
		}
	default:
		return fmt.Errorf("不支持的数量类型: %d", quantityType)
	}
	return nil
}

// GetCorrectQuantityUnit 根据数量类型获取正确的单位类型
func (v *QuantityValidator) GetCorrectQuantityUnit(quantityType int) (int, error) {
	switch quantityType {
	case 1: // 单品 - 默认使用件
		return 1, nil
	case 2: // 同款多件 - 必须使用件
		return 1, nil
	case 3: // 单套 - 必须使用套
		return 3, nil
	case 4: // 多套 - 必须使用套
		return 3, nil
	default:
		return 0, fmt.Errorf("不支持的数量类型: %d", quantityType)
	}
}

// ValidateQuantity 验证数量值的合理性
func (v *QuantityValidator) ValidateQuantity(quantity, quantityType int) error {
	switch quantityType {
	case 1: // 单品
		if quantity != 1 {
			return fmt.Errorf("单品类型(quantityType=1)的数量必须为1，当前为: %d", quantity)
		}
	case 2, 4: // 同款多件、多套
		if quantity < 2 {
			return fmt.Errorf("多件/多套类型(quantityType=%d)的数量必须大于等于2，当前为: %d", quantityType, quantity)
		}
	case 3: // 单套
		if quantity != 1 {
			return fmt.Errorf("单套类型(quantityType=3)的数量必须为1，当前为: %d", quantity)
		}
	default:
		return fmt.Errorf("不支持的数量类型: %d", quantityType)
	}
	return nil
}
