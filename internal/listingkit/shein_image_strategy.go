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
	switch strings.ToLower(strings.TrimSpace(req.Options.ImageStrategy)) {
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

func shouldUseSheinStudioAIImages(req *GenerateRequest) bool {
	strategy := resolveSheinImageStrategy(req)
	return strategy == sheinImageStrategyAIGenerated || strategy == sheinImageStrategyHybrid
}
