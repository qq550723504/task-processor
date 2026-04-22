package publish

import (
	"fmt"
	"strings"

	apiproduct "task-processor/internal/shein/api/product"
)

func (v *PublishProductValidator) validateResolvedAttributes(input *ValidationInput) error {
	product := input.ProductData
	if product == nil {
		return fmt.Errorf("产品数据为空")
	}

	issues := make([]string, 0)
	for attrIndex, attr := range product.ProductAttributeList {
		if attr.AttributeID <= 0 {
			issues = append(issues, fmt.Sprintf("ProductAttribute[%d]缺少真实 attribute_id", attrIndex))
			continue
		}
		if attr.AttributeValueID == nil && strings.TrimSpace(attr.AttributeExtraValue) == "" {
			issues = append(issues, fmt.Sprintf("ProductAttribute[%d]缺少属性值", attrIndex))
		}
	}

	if len(issues) > 0 {
		return fmt.Errorf("发现%d个属性映射问题: %s", len(issues), strings.Join(issues, "; "))
	}
	return nil
}

func (v *PublishProductValidator) validateResolvedSaleAttributes(input *ValidationInput) error {
	product := input.ProductData
	if product == nil {
		return fmt.Errorf("产品数据为空")
	}
	if len(product.SKCList) == 0 {
		return nil
	}

	issues := make([]string, 0)
	requireSKCSaleAttribute := len(product.SKCList) > 1

	for skcIndex, skc := range product.SKCList {
		if requireSKCSaleAttribute {
			if issue := validateResolvedSaleAttribute("SKC", skcIndex, skc.SaleAttribute); issue != "" {
				issues = append(issues, issue)
			}
		}

		requireSKUSaleAttribute := len(skc.SKUS) > 1
		for skuIndex, sku := range skc.SKUS {
			if !requireSKUSaleAttribute && len(sku.SaleAttributeList) == 0 {
				continue
			}
			if len(sku.SaleAttributeList) == 0 {
				issues = append(issues, fmt.Sprintf("SKC[%d] SKU[%d]缺少销售属性", skcIndex, skuIndex))
				continue
			}
			for saleAttrIndex, saleAttr := range sku.SaleAttributeList {
				if saleAttr.AttributeID <= 0 || saleAttr.AttributeValueID <= 0 {
					issues = append(issues, fmt.Sprintf("SKC[%d] SKU[%d] SaleAttribute[%d]缺少真实 attribute_id/value_id", skcIndex, skuIndex, saleAttrIndex))
				}
			}
		}
	}

	if len(issues) > 0 {
		return fmt.Errorf("发现%d个销售属性映射问题: %s", len(issues), strings.Join(issues, "; "))
	}
	return nil
}

func validateResolvedSaleAttribute(scope string, index int, attr apiproduct.SaleAttribute) string {
	if attr.AttributeID <= 0 || attr.AttributeValueID <= 0 {
		return fmt.Sprintf("%s[%d]缺少真实销售属性映射", scope, index)
	}
	return ""
}
