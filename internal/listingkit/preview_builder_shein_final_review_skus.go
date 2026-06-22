package listingkit

import sheinworkspace "task-processor/internal/marketplace/shein/workspace"

func buildSheinFinalReviewSKUs(draft *SheinRequestDraft) []SheinFinalReviewSKU {
	return sheinworkspace.BuildFinalReviewSKUs(draft)
}

func buildSheinFinalReviewSKU(supplierCode string, sku SheinSKUDraft) SheinFinalReviewSKU {
	return sheinworkspace.BuildFinalReviewSKU(supplierCode, sku)
}
