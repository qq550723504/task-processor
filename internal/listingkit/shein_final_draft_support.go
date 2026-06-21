package listingkit

import (
	sheinpub "task-processor/internal/publishing/shein"
	sheinproduct "task-processor/internal/shein/api/product"
)

func sheinPreviewSKCImagesCoverDraft(existing []sheinproduct.ImageDetail, draft *sheinpub.ImageDraft) bool {
	return sheinpub.PreviewSKCImagesCoverDraft(existing, draft)
}

func orderSheinImages(existing []string, order []string, deleted map[string]struct{}) []string {
	return sheinpub.OrderFinalDraftImages(existing, order, deleted)
}

func sheinFinalDraftFallbackImages(pkg *sheinpub.Package, main string, deleted map[string]struct{}) []string {
	return sheinpub.FinalDraftFallbackImages(pkg, main, deleted)
}

func sheinPackageSKCMainImage(pkg *sheinpub.Package, index int, supplierCode string) string {
	return sheinpub.PackageSKCMainImage(pkg, index, supplierCode)
}

func sheinRequestSKCMainImage(skc *sheinpub.SKCRequestDraft) string {
	return sheinpub.RequestSKCMainImage(skc)
}

func sheinImageDraftHasImages(info *sheinpub.ImageDraft) bool {
	return sheinpub.ImageDraftHasImages(info)
}

func sheinGalleryWithoutMain(images []string, main string) []string {
	return sheinpub.GalleryWithoutMain(images, main)
}

func sheinRequestDraftSKCByIndexOrCode(draft *sheinpub.RequestDraft, index int, supplierCode string) *sheinpub.SKCRequestDraft {
	return sheinpub.RequestDraftSKCByIndexOrCode(draft, index, supplierCode)
}

func sheinPreviewSKCSupplierCode(skc *sheinproduct.SKC) string {
	return sheinpub.PreviewSKCSupplierCode(skc)
}

func sheinProductImageInfoFromDraft(info *sheinpub.ImageDraft, roles map[string]string) *sheinproduct.ImageInfo {
	return sheinpub.ProductImageInfoFromDraft(info, roles)
}

func reorderSheinProductImages(info *sheinproduct.ImageInfo, order []string, main string, deleted map[string]struct{}, roles map[string]string) {
	sheinpub.ReorderFinalDraftProductImages(info, order, main, deleted, roles)
}

func normalizeImageRoleOverrides(input map[string]string) map[string]string {
	return sheinpub.NormalizeImageRoleOverrides(input)
}
