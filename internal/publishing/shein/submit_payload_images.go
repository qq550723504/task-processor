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
	detailImages := submitDetailImages(images)
	if len(detailImages) == 0 {
		return []sheinproduct.SiteDetailImageInfo{}
	}
	return []sheinproduct.SiteDetailImageInfo{{
		SiteAbbrList:  []string{},
		ImageInfoList: detailImages,
	}}
}

// NormalizeSubmitGalleryImages normalizes gallery image types, ordering, square image, and optional color block.
func NormalizeSubmitGalleryImages(images []sheinproduct.ImageDetail, includeColorBlock bool) []sheinproduct.ImageDetail {
	source := DedupeImagesByURL(images)
	if len(source) == 0 {
		return nil
	}
	colorBlockSource := source[0]
	for _, image := range source {
		if image.ImageType == 6 && !image.SizeImgFlag && strings.TrimSpace(image.ImageURL) != "" {
			colorBlockSource = image
			break
		}
	}
	gallerySource := make([]sheinproduct.ImageDetail, 0, len(source))
	for _, image := range source {
		if image.ImageType == 6 && !image.SizeImgFlag {
			continue
		}
		gallerySource = append(gallerySource, image)
	}
	if len(gallerySource) == 0 {
		gallerySource = []sheinproduct.ImageDetail{source[0]}
	}
	extraCapacity := 1
	if includeColorBlock {
		extraCapacity = 2
	}
	normalized := make([]sheinproduct.ImageDetail, 0, len(gallerySource)+extraCapacity)
	for index, image := range gallerySource {
		image.ImageType = 2
		if index == 0 {
			image.ImageType = 1
		}
		image.ImageSort = index + 1
		image.MarketingMainImage = false
		image.SizeImgFlag = false
		image.TransformCVSizeImage = false
		if image.PSTypes == nil {
			image.PSTypes = []string{}
		}
		normalized = append(normalized, image)
	}
	square := gallerySource[0]
	square.ImageType = 5
	square.ImageSort = len(normalized) + 1
	square.MarketingMainImage = false
	square.SizeImgFlag = false
	square.TransformCVSizeImage = false
	if square.PSTypes == nil {
		square.PSTypes = []string{}
	}
	normalized = append(normalized, square)
	if !includeColorBlock {
		return normalized
	}
	colorBlock := colorBlockSource
	colorBlock.ImageType = 6
	colorBlock.ImageSort = len(normalized) + 1
	colorBlock.MarketingMainImage = false
	colorBlock.SizeImgFlag = false
	colorBlock.TransformCVSizeImage = false
	if colorBlock.PSTypes == nil {
		colorBlock.PSTypes = []string{}
	}
	normalized = append(normalized, colorBlock)
	return normalized
}

// DedupeImagesByURL keeps the first non-empty image for each image URL.
func DedupeImagesByURL(images []sheinproduct.ImageDetail) []sheinproduct.ImageDetail {
	return sheinmarketpub.DedupeImagesByURL(images)
}

func submitDetailImages(images []sheinproduct.ImageDetail) []sheinproduct.DetailImage {
	primary := make([]sheinproduct.DetailImage, 0, len(images))
	fallback := make([]sheinproduct.DetailImage, 0, len(images))
	seen := map[string]bool{}
	for _, image := range images {
		url := strings.TrimSpace(image.ImageURL)
		if url == "" || seen[url] {
			continue
		}
		seen[url] = true
		detail := sheinproduct.DetailImage{
			ImageURL:    url,
			ImageItemID: image.ImageItemID,
		}
		if image.ImageType == 2 && !image.SizeImgFlag {
			primary = append(primary, detail)
			continue
		}
		if image.ImageType == 1 || image.ImageType == 5 || image.ImageType == 6 {
			fallback = append(fallback, detail)
		}
	}
	detailImages := primary
	if len(detailImages) < 2 {
		for _, image := range fallback {
			if len(detailImages) >= 2 {
				break
			}
			detailImages = append(detailImages, image)
		}
	}
	for index := range detailImages {
		detailImages[index].ImageSort = index + 1
	}
	return detailImages
}
