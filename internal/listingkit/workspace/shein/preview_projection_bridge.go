package shein

import (
	sheinmarketplace "task-processor/internal/marketplace/shein/workspace"
	sheinproduct "task-processor/internal/shein/api/product"
)

type FinalReviewSKU = sheinmarketplace.FinalReviewSKU
type FinalReviewImage = sheinmarketplace.FinalReviewImage

func BuildPreviewReviewSummary(pkg *Package) (bool, []string) {
	return sheinmarketplace.BuildPreviewReviewSummary(pkg)
}

func BuildFinalReviewSKUs(draft *RequestDraft) []FinalReviewSKU {
	return sheinmarketplace.BuildFinalReviewSKUs(draft)
}

func BuildFinalReviewSKU(supplierCode string, sku SKUDraft) FinalReviewSKU {
	return sheinmarketplace.BuildFinalReviewSKU(supplierCode, sku)
}

func BuildFinalReviewImages(draft *RequestDraft, finalDraft *FinalDraft, product *sheinproduct.Product) []FinalReviewImage {
	return sheinmarketplace.BuildFinalReviewImages(draft, finalDraft, product)
}
