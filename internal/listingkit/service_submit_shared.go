package listingkit

import (
	"fmt"
	"strings"
	"time"

	sheinpub "task-processor/internal/publishing/shein"
	sheinproduct "task-processor/internal/shein/api/product"
)

func sheinProductAttributesReadyForSubmit(attrs []sheinproduct.ProductAttribute) bool {
	if len(attrs) == 0 {
		return false
	}
	for _, attr := range attrs {
		if attr.AttributeID <= 0 {
			return false
		}
		if attr.AttributeValueID == nil && strings.TrimSpace(attr.AttributeExtraValue) == "" {
			return false
		}
	}
	return true
}

func normalizedSubmitIdempotencyKey(req *SubmitTaskRequest) string {
	if req == nil {
		return ""
	}
	if value := strings.TrimSpace(req.IdempotencyKey); value != "" {
		return value
	}
	return strings.TrimSpace(req.RequestID)
}

func derivedSheinSubmitRequestID(taskID, action string, requestedAt time.Time) string {
	taskID = strings.TrimSpace(taskID)
	action = strings.ToLower(strings.TrimSpace(action))
	if action == "" {
		action = "publish"
	}
	timestamp := requestedAt.UTC().Format("20060102T150405.000000000Z")
	if taskID == "" {
		taskID = "unknown-task"
	}
	return fmt.Sprintf("temporal:%s:%s:%s", taskID, action, timestamp)
}

func repairSheinSubmitSaleAttributes(pkg *SheinPackage) {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	if !sheinSubmitSaleAttributesNeedRepair(pkg) {
		return
	}
	sheinpub.ApplySaleAttributeResolution(pkg, pkg.SaleAttributeResolution)
	preview := sheinpub.BuildPreviewProduct(pkg)
	sheinpub.SetPreviewPayload(pkg, preview)
	sheinpub.NormalizePackageSemanticFields(pkg)
}

func sheinSubmitSaleAttributesNeedRepair(pkg *SheinPackage) bool {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.DraftPayload == nil || pkg.SaleAttributeResolution == nil {
		return false
	}
	if strings.TrimSpace(pkg.SaleAttributeResolution.Status) != "resolved" {
		return false
	}
	if len(pkg.DraftPayload.SKCList) == 0 {
		return false
	}
	if len(pkg.SaleAttributeResolution.SKCAttributes) == 0 && len(pkg.SaleAttributeResolution.SKUAttributes) == 0 {
		return false
	}
	for _, skc := range pkg.DraftPayload.SKCList {
		if skc.SaleAttribute == nil || skc.SaleAttribute.AttributeID <= 0 || skc.SaleAttribute.AttributeValueID == nil || *skc.SaleAttribute.AttributeValueID <= 0 {
			return true
		}
		for _, sku := range skc.SKUList {
			if len(pkg.SaleAttributeResolution.SKUAttributes) == 0 {
				continue
			}
			if len(sku.SaleAttributes) == 0 {
				return true
			}
			for _, attr := range sku.SaleAttributes {
				if attr.AttributeID <= 0 || attr.AttributeValueID == nil || *attr.AttributeValueID <= 0 {
					return true
				}
			}
		}
	}
	return false
}

func isSupportedSubmitAction(action string) bool {
	return action == "publish" || action == "save_draft"
}

func unsupportedSubmitActionError(action string) error {
	return fmt.Errorf("unsupported submit action: %s", action)
}

func sheinSubmitSupplierCode(product *sheinproduct.Product, pkg *SheinPackage) string {
	if product != nil {
		if value := strings.TrimSpace(product.SupplierCode); value != "" {
			return value
		}
		for i := range product.SKCList {
			if product.SKCList[i].SupplierCode == nil {
				continue
			}
			if value := strings.TrimSpace(*product.SKCList[i].SupplierCode); value != "" {
				return value
			}
		}
	}
	if pkg != nil {
		for _, skc := range pkg.SkcList {
			if value := strings.TrimSpace(skc.SupplierCode); value != "" {
				return value
			}
		}
	}
	if product != nil && strings.TrimSpace(product.SPUName) != "" {
		return strings.TrimSpace(product.SPUName)
	}
	return ""
}
