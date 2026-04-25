package productimage

import "task-processor/internal/productimage/providers"

type TaskSubmitter interface {
	Submit(taskID string) error
}

type SourceParser = providers.SourceParser
type ProductContextAnalyzer = providers.ProductContextAnalyzer
type ImageInspector = providers.ImageInspector
type ImageRanker = providers.ImageRanker
type SubjectExtractor = providers.SubjectExtractor
type ImageCleaner = providers.ImageCleaner
type WhiteBackgroundRenderer = providers.WhiteBackgroundRenderer
type AssetPublisher = providers.AssetPublisher
type SceneRenderer = providers.SceneRenderer
type MarketplaceValidator = providers.MarketplaceValidator
type QualityAssessor = providers.QualityAssessor
type ReviewAssessor = providers.ReviewAssessor
