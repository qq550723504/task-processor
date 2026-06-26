package shein

import (
	"encoding/json"
	"fmt"
	"strings"

	sheinmarketpub "task-processor/internal/marketplace/shein/publishing"
	sheinproduct "task-processor/internal/shein/api/product"
)

func CloneProductForSubmit(product *sheinproduct.Product) (*sheinproduct.Product, error) {
	if product == nil {
		return nil, nil
	}
	data, err := json.Marshal(product)
	if err != nil {
		return nil, fmt.Errorf("clone shein product: %w", err)
	}
	var cloned sheinproduct.Product
	if err := json.Unmarshal(data, &cloned); err != nil {
		return nil, fmt.Errorf("clone shein product: %w", err)
	}
	return &cloned, nil
}

func ProductImageURLCount(product *sheinproduct.Product) int {
	if product == nil {
		return 0
	}
	count := ImageInfoURLCount(product.ImageInfo)
	for i := range product.SKCList {
		count += ImageInfoURLCount(&product.SKCList[i].ImageInfo)
		for j := range product.SKCList[i].SKUS {
			count += ImageInfoURLCount(product.SKCList[i].SKUS[j].ImageInfo)
		}
	}
	return count
}

func ProductPendingImageUploadCount(product *sheinproduct.Product) int {
	if product == nil {
		return 0
	}
	count := ImageInfoPendingUploadCount(product.ImageInfo)
	for i := range product.SKCList {
		count += ImageInfoPendingUploadCount(&product.SKCList[i].ImageInfo)
		for j := range product.SKCList[i].SKUS {
			count += ImageInfoPendingUploadCount(product.SKCList[i].SKUS[j].ImageInfo)
		}
	}
	return count
}

func ImageInfoURLCount(info *sheinproduct.ImageInfo) int {
	if info == nil {
		return 0
	}
	count := 0
	for _, image := range info.ImageInfoList {
		if strings.TrimSpace(image.ImageURL) != "" {
			count++
		}
	}
	return count
}

func ImageInfoPendingUploadCount(info *sheinproduct.ImageInfo) int {
	if info == nil {
		return 0
	}
	count := 0
	for _, image := range info.ImageInfoList {
		url := strings.TrimSpace(image.ImageURL)
		if url != "" && !IsUploadedImageURL(url) {
			count++
		}
	}
	return count
}

func IsUploadedImageURL(url string) bool {
	return sheinmarketpub.IsUploadedImageURL(url)
}

func IsSDSImageURL(url string) bool {
	return sheinmarketpub.IsSDSImageURL(url)
}

func CloneImageUploadCache(input map[string]string) map[string]string {
	return sheinmarketpub.CloneImageUploadCache(input)
}
