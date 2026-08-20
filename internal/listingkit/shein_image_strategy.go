package listingkit

import "strings"

const (
	sheinImageStrategyAIGenerated = "ai_generated"
	sheinImageStrategySDSOfficial = "sds_official"
	sheinImageStrategyHybrid      = "hybrid"
)

func resolveSheinImageStrategy(req *GenerateRequest) string {
	if req == nil || req.Options == nil {
		return sheinImageStrategySDSOfficial
	}
	return normalizeSheinImageStrategy(req.Options.ImageStrategy)
}

func normalizeSheinImageStrategy(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case sheinImageStrategyAIGenerated:
		return sheinImageStrategyAIGenerated
	case sheinImageStrategyHybrid:
		return sheinImageStrategyHybrid
	default:
		return sheinImageStrategySDSOfficial
	}
}

func shouldUseSDSOfficialImages(req *GenerateRequest) bool {
	strategy := resolveSheinImageStrategy(req)
	return strategy == sheinImageStrategySDSOfficial || strategy == sheinImageStrategyHybrid
}

func shouldRenderSheinSizeImagesWithSDS(req *GenerateRequest) bool {
	if shouldUseSDSOfficialImages(req) {
		return true
	}
	return req != nil &&
		req.Options != nil &&
		req.Options.SheinStudio != nil &&
		req.Options.SheinStudio.RenderSizeImagesWithSDS
}

func shouldUseSheinStudioAIImages(req *GenerateRequest) bool {
	strategy := resolveSheinImageStrategy(req)
	return strategy == sheinImageStrategyAIGenerated || strategy == sheinImageStrategyHybrid
}
