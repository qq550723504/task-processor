package listingkit

import (
	"strings"

	listingworkflow "task-processor/internal/listingkit/workflow"
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
	return listingworkflow.ShouldProcessImages(buildWorkflowRequestPolicyInput(req))
}

func shouldGenerateAssets(req *GenerateRequest) bool {
	return listingworkflow.ShouldGenerateAssets(buildWorkflowRequestPolicyInput(req))
}

func shouldUseSDSCatalogSource(req *GenerateRequest) bool {
	return listingworkflow.ShouldUseSDSCatalogSource(buildWorkflowRequestPolicyInput(req))
}

func shouldSyncSDS(req *GenerateRequest) bool {
	return listingworkflow.ShouldSyncSDS(buildWorkflowRequestPolicyInput(req))
}

func shouldRunStudioInline(req *GenerateRequest) bool {
	return listingworkflow.ShouldRunStudioInline(buildWorkflowRequestPolicyInput(req))
}

func shouldRunRemoteSDSDesignSync(req *GenerateRequest) bool {
	return listingworkflow.ShouldRunRemoteSDSDesignSync(buildWorkflowRequestPolicyInput(req))
}

func buildWorkflowRequestPolicyInput(req *GenerateRequest) listingworkflow.RequestPolicyInput {
	if req == nil || req.Options == nil {
		return listingworkflow.RequestPolicyInput{}
	}

	input := listingworkflow.RequestPolicyInput{
		ProcessImages:                req.Options.ProcessImages,
		ImageURLs:                    append([]string(nil), req.ImageURLs...),
		ProductURL:                   strings.TrimSpace(req.ProductURL),
		Platforms:                    append([]string(nil), req.Platforms...),
		UseSheinStudioAIImages:       shouldUseSheinStudioAIImages(req),
		RenderSheinSizeImagesWithSDS: shouldRenderSheinSizeImagesWithSDS(req),
	}
	if req.Options.SDS != nil {
		input.SDS = listingworkflow.SDSPolicyInput{
			VariantID:    req.Options.SDS.VariantID,
			VariantCount: len(req.Options.SDS.Variants),
			ProductName:  req.Options.SDS.ProductName,
			ProductSKU:   req.Options.SDS.ProductSKU,
			VariantSKU:   req.Options.SDS.VariantSKU,
			CategoryPath: append([]string(nil), req.Options.SDS.CategoryPath...),
		}
	}
	return input
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
