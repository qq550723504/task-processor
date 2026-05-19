package workflow

import "strings"

type SDSPolicyInput struct {
	VariantID    int64
	VariantCount int
	ProductName  string
	ProductSKU   string
	VariantSKU   string
	CategoryPath []string
}

type RequestPolicyInput struct {
	ProcessImages               bool
	ImageURLs                   []string
	ProductURL                  string
	Platforms                   []string
	UseSheinStudioAIImages      bool
	RenderSheinSizeImagesWithSDS bool
	SDS                         SDSPolicyInput
}

func ShouldSyncSDS(input RequestPolicyInput) bool {
	return input.SDS.VariantID > 0 || input.SDS.VariantCount > 0
}

func ShouldUseSDSCatalogSource(input RequestPolicyInput) bool {
	return ShouldSyncSDS(input) &&
		(strings.TrimSpace(input.SDS.ProductName) != "" ||
			strings.TrimSpace(input.SDS.ProductSKU) != "" ||
			strings.TrimSpace(input.SDS.VariantSKU) != "" ||
			len(input.SDS.CategoryPath) > 0)
}

func ShouldProcessImages(input RequestPolicyInput) bool {
	if ShouldUseSDSCatalogSource(input) {
		return false
	}
	return input.ProcessImages &&
		(len(input.ImageURLs) > 0 || strings.TrimSpace(input.ProductURL) != "")
}

func ShouldGenerateAssets(input RequestPolicyInput) bool {
	if ShouldUseSDSCatalogSource(input) {
		return false
	}
	return input.ProcessImages
}

func ShouldUseStudioProductFallback(input RequestPolicyInput) bool {
	return ShouldSyncSDS(input) && len(input.ImageURLs) > 0
}

func ShouldUseStudioCatalogCanonical(input RequestPolicyInput) bool {
	return ShouldSyncSDS(input)
}

func ShouldRunStudioInline(input RequestPolicyInput) bool {
	if input.ProcessImages {
		return false
	}
	if !ShouldSyncSDS(input) && !input.UseSheinStudioAIImages {
		return false
	}
	if len(input.ImageURLs) == 0 || len(input.Platforms) != 1 {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(input.Platforms[0]), "shein")
}

func ShouldRunRemoteSDSDesignSync(input RequestPolicyInput) bool {
	if input.ProcessImages {
		return false
	}
	if !ShouldSyncSDS(input) || !input.RenderSheinSizeImagesWithSDS {
		return false
	}
	return len(input.ImageURLs) > 0
}
