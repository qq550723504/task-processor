package shein

import (
	"strings"

	sheinmarketpub "task-processor/internal/marketplace/shein/publishing"
	sheinproduct "task-processor/internal/shein/api/product"
)

// NormalizeSubmitImages normalizes SHEIN submit product, SKC, and SKU image payload fields.
func NormalizeSubmitImages(product *sheinproduct.Product) {
	if product == nil {
		return
	}
	if product.ImageInfo != nil {
		product.ImageInfo.ImageInfoList = NormalizeSubmitSPUImages(product.ImageInfo.ImageInfoList)
		if product.ImageInfo.OriginalImageInfoList == nil {
			empty := []any{}
			product.ImageInfo.OriginalImageInfoList = &empty
		}
	}
	product.Extra.SwitchToSPUPic = false
	for skcIndex := range product.SKCList {
		skc := &product.SKCList[skcIndex]
		NormalizeSubmitSKCImages(skc)
		NormalizeSubmitSKUImages(skc)
	}
}

// NormalizeSubmitSPUImages normalizes top-level SPU gallery images for SHEIN submit.
func NormalizeSubmitSPUImages(images []sheinproduct.ImageDetail) []sheinproduct.ImageDetail {
	normalized := NormalizeSubmitGalleryImages(images, false)
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}

// NormalizeSubmitSKUImages normalizes SKU image payloads and repairs missing SKU images from SKC fallback.
func NormalizeSubmitSKUImages(skc *sheinproduct.SKC) {
	if skc == nil {
		return
	}
	var fallback sheinproduct.ImageDetail
	hasFallback := false
	if len(skc.ImageInfo.ImageInfoList) > 0 {
		fallback = skc.ImageInfo.ImageInfoList[0]
		hasFallback = strings.TrimSpace(fallback.ImageURL) != ""
	}
	for skuIndex := range skc.SKUS {
		info := skc.SKUS[skuIndex].ImageInfo
		if info == nil || len(info.ImageInfoList) == 0 {
			if !hasFallback {
				continue
			}
			skc.SKUS[skuIndex].ImageInfo = &sheinproduct.ImageInfo{
				ImageInfoList: []sheinproduct.ImageDetail{NormalizeSubmitSKUImageDetail(fallback)},
			}
			empty := []any{}
			skc.SKUS[skuIndex].ImageInfo.OriginalImageInfoList = &empty
			continue
		}
		info.ImageInfoList = DedupeImagesByURL(info.ImageInfoList)
		if len(info.ImageInfoList) > 0 {
			info.ImageInfoList = []sheinproduct.ImageDetail{NormalizeSubmitSKUImageDetail(info.ImageInfoList[0])}
		}
		if info.OriginalImageInfoList == nil {
			empty := []any{}
			info.OriginalImageInfoList = &empty
		}
	}
}

// NormalizeSubmitSKUImageDetail normalizes one SKU image detail for SHEIN submit.
func NormalizeSubmitSKUImageDetail(image sheinproduct.ImageDetail) sheinproduct.ImageDetail {
	return sheinmarketpub.NormalizeSubmitSKUImageDetail(image)
}

// NormalizeSubmitSKCImages normalizes SKC gallery and site detail images for SHEIN submit.
func NormalizeSubmitSKCImages(skc *sheinproduct.SKC) {
	if skc == nil {
		return
	}
	if len(skc.ImageInfo.ImageInfoList) == 0 {
		skc.SiteDetailImageInfoList = []sheinproduct.SiteDetailImageInfo{}
		return
	}
	skc.ImageInfo.ImageInfoList = NormalizeSubmitGalleryImages(skc.ImageInfo.ImageInfoList, true)
	if skc.ImageInfo.OriginalImageInfoList == nil {
		empty := []any{}
		skc.ImageInfo.OriginalImageInfoList = &empty
	}
	skc.SiteDetailImageInfoList = BuildSubmitSiteDetailImageInfoList(skc.ImageInfo.ImageInfoList)
}

// BuildSubmitSiteDetailImageInfoList builds SHEIN site detail image groups from normalized images.
func BuildSubmitSiteDetailImageInfoList(images []sheinproduct.ImageDetail) []sheinproduct.SiteDetailImageInfo {
	return sheinmarketpub.BuildSubmitSiteDetailImageInfoList(images)
}

// NormalizeSubmitGalleryImages normalizes gallery image types, ordering, square image, and optional color block.
func NormalizeSubmitGalleryImages(images []sheinproduct.ImageDetail, includeColorBlock bool) []sheinproduct.ImageDetail {
	return sheinmarketpub.NormalizeSubmitGalleryImages(images, includeColorBlock)
}

// DedupeImagesByURL keeps the first non-empty image for each image URL.
func DedupeImagesByURL(images []sheinproduct.ImageDetail) []sheinproduct.ImageDetail {
	return sheinmarketpub.DedupeImagesByURL(images)
}
