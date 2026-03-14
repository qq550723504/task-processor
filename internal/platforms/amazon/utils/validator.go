package amazonutil

import (
	"fmt"
	"strings"
)

// Validator 数据验证器
type Validator struct{}

// NewValidator 创建验证器
func NewValidator() *Validator {
	return &Validator{}
}

// ValidateTitle 验证标题
func (v *Validator) ValidateTitle(title string) error {
	title = strings.TrimSpace(title)

	if title == "" {
		return fmt.Errorf("标题不能为空")
	}

	if len(title) > 200 {
		return fmt.Errorf("标题长度不能超过200个字符")
	}

	return nil
}

// ValidateDescription 验证描述
func (v *Validator) ValidateDescription(desc string) error {
	desc = strings.TrimSpace(desc)

	if desc == "" {
		return fmt.Errorf("描述不能为空")
	}

	if len(desc) > 2000 {
		return fmt.Errorf("描述长度不能超过2000个字符")
	}

	return nil
}

// ValidatePrice 验证价格
func (v *Validator) ValidatePrice(price float64) error {
	if price <= 0 {
		return fmt.Errorf("价格必须大于0")
	}

	if price > 999999.99 {
		return fmt.Errorf("价格不能超过999999.99")
	}

	return nil
}

// ValidateQuantity 验证库存数量
func (v *Validator) ValidateQuantity(quantity int) error {
	if quantity < 0 {
		return fmt.Errorf("库存数量不能为负数")
	}

	return nil
}

// ValidateImages 验证图片列表
func (v *Validator) ValidateImages(images []string) error {
	if len(images) == 0 {
		return fmt.Errorf("至少需要1张图片")
	}

	if len(images) > 9 {
		return fmt.Errorf("图片数量不能超过9张")
	}

	for i, img := range images {
		if !v.isValidImageURL(img) {
			return fmt.Errorf("第%d张图片URL无效", i+1)
		}
	}

	return nil
}

// isValidImageURL 验证图片URL
func (v *Validator) isValidImageURL(url string) bool {
	url = strings.ToLower(strings.TrimSpace(url))
	return strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://")
}
