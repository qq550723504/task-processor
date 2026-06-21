package listingkit

import (
	"fmt"
	"strings"

	sheinworkspace "task-processor/internal/listingkit/workspace/shein"
	sheinpub "task-processor/internal/publishing/shein"
)

type sheinBuildValidation struct {
	categoryReady        bool
	categoryReviewReady  bool
	categoryMessage      string
	attributeReady       bool
	attributeMessage     string
	saleAttributeReady   bool
	saleAttributeMessage string
	submitPayloadReady   bool
	submitPayloadMessage string
}

func ValidateSheinPackageAgainstTemplates(pkg *SheinPackage) sheinBuildValidation {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	result := sheinBuildValidation{
		categoryReady:        pkg != nil && isSheinCategoryResolved(pkg) && pkg.CategoryID > 0 && pkg.ProductTypeID != nil && *pkg.ProductTypeID > 0,
		categoryReviewReady:  !sheinCategoryReviewPending(pkg),
		categoryMessage:      "类目、类目层级和 product_type_id 需要确认；如当前类目被建议复核，也不能直接进入提交态",
		attributeReady:       isSheinAttributeResolved(pkg) && !sheinHasBlockingPendingAttributes(pkg),
		attributeMessage:     "普通属性还没有全部映射到真实 attribute_id / attribute_value_id，或仍存在模板必填/重要属性未确认",
		saleAttributeReady:   sheinSaleAttributesReadyForSubmit(pkg),
		saleAttributeMessage: "销售属性主副规格还没有稳定映射到真实 sale attribute/value，或当前类目/规格组合仍需复核",
		submitPayloadReady:   true,
		submitPayloadMessage: "发布载荷结构需要满足 SHEIN 提交要求，包括 SKC 图片、方形图、SKU 数量/包装/仓库/尺寸字段",
	}
	if err := validatePreparedSheinSubmitPayload(pkg); err != nil {
		result.submitPayloadReady = false
		result.submitPayloadMessage = err.Error()
	}
	return result
}

func appendSheinBuildValidationChecks(checks []sheinworkspace.ReadinessCheckSpec, validation sheinBuildValidation) []sheinworkspace.ReadinessCheckSpec {
	return append(checks, sheinworkspace.BuildSubmitPayloadValidationReadinessChecks(sheinworkspace.SubmitPayloadValidationReadinessInput{
		Ready:   validation.submitPayloadReady,
		Message: validation.submitPayloadMessage,
	})...)
}

func sheinHasBlockingPendingAttributes(pkg *SheinPackage) bool {
	return sheinpub.HasBlockingPendingAttributes(pkg)
}

func sheinCategoryReviewPending(pkg *SheinPackage) bool {
	if pkg == nil || pkg.SaleAttributeResolution == nil {
		return false
	}
	return pkg.SaleAttributeResolution.RecommendCategoryReview
}

func sheinSaleAttributeReviewPending(pkg *SheinPackage) bool {
	return sheinpub.SaleAttributeReviewPending(pkg)
}

func sheinSaleAttributeStatusResolved(pkg *SheinPackage) bool {
	return sheinpub.SaleAttributeStatusResolved(pkg)
}

func sheinSaleAttributesReadyForSubmit(pkg *SheinPackage) bool {
	return sheinpub.SaleAttributesReadyForSubmit(pkg)
}

func sheinSaleAttributesReadinessFailureReasons(pkg *SheinPackage) []string {
	return sheinpub.SaleAttributesReadinessFailureReasons(pkg)
}

func sheinResolvedSaleAttributeReady(attr *SheinResolvedSaleAttribute) bool {
	return sheinpub.ResolvedSaleAttributeReady(attr)
}

func sheinResolvedSaleAttributeValueReady(attr SheinResolvedSaleAttribute) bool {
	return sheinpub.ResolvedSaleAttributeValueReady(attr)
}

func validatePreparedSheinSubmitPayload(pkg *SheinPackage) error {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.PreviewPayload == nil {
		return nil
	}
	product, err := cloneSheinProductForSubmit(pkg.PreviewPayload)
	if err != nil {
		return err
	}
	prepareSheinProductForNewSubmit(product)
	if err := validateSheinProductPublishPayload(product); err != nil {
		return err
	}
	for skcIndex, skc := range product.SKCList {
		for skuIndex, sku := range skc.SKUS {
			if sku.QuantityInfo == nil || sku.QuantityInfo.Quantity == nil || sku.QuantityInfo.QuantityType == nil || sku.QuantityInfo.QuantityUnit == nil {
				return fmt.Errorf("SHEIN publish blocked: SKC[%d] SKU[%d] is missing quantity_info", skcIndex, skuIndex)
			}
			if sku.PackageType == 0 {
				return fmt.Errorf("SHEIN publish blocked: SKC[%d] SKU[%d] is missing package_type", skcIndex, skuIndex)
			}
			if len(sku.StockInfoList) == 0 {
				return fmt.Errorf("SHEIN publish blocked: SKC[%d] SKU[%d] is missing stock_info_list", skcIndex, skuIndex)
			}
			if strings.TrimSpace(sku.Length) == "" || strings.TrimSpace(sku.Width) == "" || strings.TrimSpace(sku.Height) == "" || strings.TrimSpace(sku.LengthUnit) == "" {
				return fmt.Errorf("SHEIN publish blocked: SKC[%d] SKU[%d] is missing package dimensions", skcIndex, skuIndex)
			}
		}
	}
	return nil
}
