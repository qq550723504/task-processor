package listingkit

import (
	"strings"

	"task-processor/internal/productenrich"
	"task-processor/internal/productimage"
)

func toProductGenerateRequest(task *Task) *productenrich.GenerateRequest {
	if task == nil || task.Request == nil {
		return &productenrich.GenerateRequest{}
	}
	return &productenrich.GenerateRequest{
		ImageURLs:  append([]string(nil), task.Request.ImageURLs...),
		Text:       task.Request.Text,
		ProductURL: task.Request.ProductURL,
	}
}

func toImageProcessRequest(task *Task) *productimage.ImageProcessRequest {
	if task == nil || task.Request == nil {
		return &productimage.ImageProcessRequest{}
	}
	marketplace := detectImageMarketplace(task.Request)
	var scene *productimage.SceneGenerationOptions
	if task.Request.Options != nil {
		scene = task.Request.Options.Scene.Clone()
	}
	return &productimage.ImageProcessRequest{
		ProductURL:  task.Request.ProductURL,
		ImageURLs:   append([]string(nil), task.Request.ImageURLs...),
		Text:        task.Request.Text,
		Marketplace: marketplace,
		Country:     task.Request.Country,
		Scene:       scene,
	}
}

func shouldProcessImages(req *GenerateRequest) bool {
	if shouldUseSDSCatalogSource(req) {
		return false
	}
	return req != nil && req.Options != nil && req.Options.ProcessImages &&
		(len(req.ImageURLs) > 0 || strings.TrimSpace(req.ProductURL) != "")
}

func shouldGenerateAssets(req *GenerateRequest) bool {
	if shouldUseSDSCatalogSource(req) {
		return false
	}
	return req != nil && req.Options != nil && req.Options.ProcessImages
}

func shouldUseSDSCatalogSource(req *GenerateRequest) bool {
	if req == nil || req.Options == nil || req.Options.SDS == nil {
		return false
	}
	sds := req.Options.SDS
	return shouldSyncSDS(req) &&
		(strings.TrimSpace(sds.ProductName) != "" ||
			strings.TrimSpace(sds.ProductSKU) != "" ||
			strings.TrimSpace(sds.VariantSKU) != "" ||
			len(sds.CategoryPath) > 0)
}

func shouldSyncSDS(req *GenerateRequest) bool {
	return req != nil &&
		req.Options != nil &&
		req.Options.SDS != nil &&
		(req.Options.SDS.VariantID > 0 || len(req.Options.SDS.Variants) > 0)
}

func detectImageMarketplace(req *GenerateRequest) string {
	if req == nil {
		return "amazon"
	}
	platforms := normalizePlatforms(req.Platforms)
	if len(platforms) == 0 {
		return "amazon"
	}
	return platforms[0]
}
