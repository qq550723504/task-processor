package shein

import (
	"fmt"
	"strings"

	sheinproduct "task-processor/internal/shein/api/product"
)

type submitProductValidationInput struct {
	ProductData              *sheinproduct.Product
	AllowPrimaryOnlyMultiSKU bool
}

type submitProductValidator struct{}

func (v submitProductValidator) preValidate(input submitProductValidationInput) error {
	if input.ProductData == nil {
		return fmt.Errorf("产品数据为空")
	}

	issues := make([]string, 0)
	if err := v.validateBasicProductInfo(input); err != nil {
		issues = append(issues, fmt.Sprintf("基本信息: %v", err))
	}
	if err := v.validateSKCAndSKUData(input); err != nil {
		issues = append(issues, fmt.Sprintf("SKC/SKU数据: %v", err))
	}
	if err := v.validateResolvedMappings(input); err != nil {
		issues = append(issues, fmt.Sprintf("属性映射: %v", err))
	}
	if len(issues) > 0 {
		return fmt.Errorf("发现%d个严重问题，无法继续发布: %s", len(issues), summarizeSubmitValidationIssues(issues, 2))
	}
	return nil
}

func (v submitProductValidator) validateBasicProductInfo(input submitProductValidationInput) error {
	product := input.ProductData
	if len(product.MultiLanguageNameList) == 0 {
		return fmt.Errorf("缺少产品名称")
	}
	if len(product.MultiLanguageDescList) == 0 {
		return fmt.Errorf("缺少产品描述")
	}
	if product.CategoryID == 0 {
		return fmt.Errorf("缺少分类ID")
	}
	return nil
}

func (v submitProductValidator) validateSKCAndSKUData(input submitProductValidationInput) error {
	product := input.ProductData
	if len(product.SKCList) == 0 {
		return fmt.Errorf("缺少SKC数据")
	}

	totalIssues := make([]string, 0)
	for skcIndex, skc := range product.SKCList {
		if len(skc.SKUS) == 0 {
			totalIssues = append(totalIssues, fmt.Sprintf("SKC[%d]缺少SKU数据", skcIndex))
			continue
		}
		for skuIndex, sku := range skc.SKUS {
			if sku.SupplierSKU == "" {
				totalIssues = append(totalIssues, fmt.Sprintf("SKC[%d] SKU[%d]缺少SupplierSKU", skcIndex, skuIndex))
			}
			if sku.CostInfo == nil || sku.CostInfo.CostPrice == "" {
				totalIssues = append(totalIssues, fmt.Sprintf("SKC[%d] SKU[%d]缺少成本价格信息", skcIndex, skuIndex))
			}
			if len(sku.PriceInfoList) == 0 {
				totalIssues = append(totalIssues, fmt.Sprintf("SKC[%d] SKU[%d]缺少价格信息", skcIndex, skuIndex))
			}
			if len(sku.StockInfoList) == 0 {
				totalIssues = append(totalIssues, fmt.Sprintf("SKC[%d] SKU[%d]缺少库存信息", skcIndex, skuIndex))
			}
			if issue := validateQuantityTypeAndValue(sku, skcIndex, skuIndex); issue != "" {
				totalIssues = append(totalIssues, issue)
			}
		}
	}

	if len(totalIssues) > 0 {
		return fmt.Errorf("发现%d个SKC/SKU数据问题: %s", len(totalIssues), strings.Join(totalIssues, "; "))
	}
	return nil
}

func (v submitProductValidator) validateResolvedMappings(input submitProductValidationInput) error {
	if err := validateResolvedAttributes(input); err != nil {
		return err
	}
	if err := validateResolvedSaleAttributes(input); err != nil {
		return err
	}
	return nil
}

func validateResolvedAttributes(input submitProductValidationInput) error {
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

func validateResolvedSaleAttributes(input submitProductValidationInput) error {
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

		requireSKUSaleAttribute := len(skc.SKUS) > 1 && !input.AllowPrimaryOnlyMultiSKU
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

func validateResolvedSaleAttribute(scope string, index int, attr sheinproduct.SaleAttribute) string {
	if attr.AttributeID <= 0 || attr.AttributeValueID <= 0 {
		return fmt.Sprintf("%s[%d]缺少真实销售属性映射", scope, index)
	}
	return ""
}

func validateQuantityTypeAndValue(sku sheinproduct.SKU, skcIndex, skuIndex int) string {
	if sku.QuantityInfo == nil || sku.QuantityInfo.QuantityType == nil || sku.QuantityInfo.Quantity == nil {
		return ""
	}
	quantityType := *sku.QuantityInfo.QuantityType
	quantity := *sku.QuantityInfo.Quantity
	if err := validateSubmitQuantity(quantity, quantityType); err != nil {
		return fmt.Sprintf("SKC[%d] SKU[%d]数量配置错误: %v (quantityType=%d, quantity=%d)",
			skcIndex, skuIndex, err, quantityType, quantity)
	}
	return ""
}

func validateSubmitQuantity(quantity, quantityType int) error {
	switch quantityType {
	case 1:
		if quantity != 1 {
			return fmt.Errorf("单品类型(quantityType=1)的数量必须为1，当前为: %d", quantity)
		}
	case 2, 4:
		if quantity < 2 {
			return fmt.Errorf("多件/多套类型(quantityType=%d)的数量必须大于等于2，当前为: %d", quantityType, quantity)
		}
	case 3:
		if quantity != 1 {
			return fmt.Errorf("单套类型(quantityType=3)的数量必须为1，当前为: %d", quantity)
		}
	default:
		return fmt.Errorf("不支持的数量类型: %d", quantityType)
	}
	return nil
}

func summarizeSubmitValidationIssues(issues []string, limit int) string {
	if len(issues) == 0 {
		return ""
	}
	if limit <= 0 || limit > len(issues) {
		limit = len(issues)
	}
	summary := strings.Join(issues[:limit], "; ")
	if len(issues) > limit {
		return fmt.Sprintf("%s; 另有%d项问题", summary, len(issues)-limit)
	}
	return summary
}
