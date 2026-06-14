package shein

import (
	"strings"

	sheinproduct "task-processor/internal/shein/api/product"
)

func ResolveSubmissionSupplierCode(product *sheinproduct.Product, pkg *Package) string {
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

func PrepareSubmissionPersistenceInput(pkg *Package, action, requestID string, product *sheinproduct.Product, snapshot *SubmitSnapshot) (string, *SubmitSnapshot) {
	supplierCode := ResolveSubmissionSupplierCode(product, pkg)
	if snapshot == nil {
		snapshot = BuildSubmitSnapshot(product)
	}
	ApplySubmissionPersistenceInput(pkg, action, requestID, supplierCode, nil, snapshot)
	return supplierCode, snapshot
}
