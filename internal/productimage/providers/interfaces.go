package providers

import (
	"context"

	"task-processor/internal/productimage/domain"
)

type SourceParser interface {
	Parse(ctx context.Context, req *domain.ImageProcessRequest) (*domain.SourceBundle, error)
}

type ProductContextAnalyzer interface {
	Analyze(ctx context.Context, source *domain.SourceBundle) (*domain.ProductContext, error)
}

type ImageInspector interface {
	Inspect(ctx context.Context, source *domain.SourceBundle, imageURL string) (*domain.ImageAudit, error)
}

type ImageRanker interface {
	Select(ctx context.Context, source *domain.SourceBundle, audits []domain.ImageAudit, context *domain.ProductContext) (*domain.ImageCandidateSet, error)
}

type SubjectExtractor interface {
	Extract(ctx context.Context, imageURL string, context *domain.ProductContext) (*domain.ImageAsset, error)
}

type ImageCleaner interface {
	Clean(ctx context.Context, asset *domain.ImageAsset, context *domain.ProductContext) (*domain.ImageAsset, error)
}

type WhiteBackgroundRenderer interface {
	Render(ctx context.Context, asset *domain.ImageAsset, context *domain.ProductContext) (*domain.ImageAsset, error)
}

type AssetPublisher interface {
	Publish(ctx context.Context, req *domain.ImageProcessRequest, result *domain.ImageProcessResult) error
}

type SceneRenderer interface {
	Render(ctx context.Context, asset *domain.ImageAsset, context *domain.ProductContext) ([]domain.ImageAsset, error)
}

type MarketplaceValidator interface {
	Validate(ctx context.Context, req *domain.ImageProcessRequest, result *domain.ImageProcessResult) (*domain.ComplianceReport, error)
}

type QualityAssessor interface {
	Assess(ctx context.Context, source *domain.SourceBundle, audits []domain.ImageAudit, candidates *domain.ImageCandidateSet, result *domain.ImageProcessResult) (*domain.QualityAssessment, error)
}

type ReviewAssessor interface {
	Assess(ctx context.Context, source *domain.SourceBundle, audits []domain.ImageAudit, candidates *domain.ImageCandidateSet, result *domain.ImageProcessResult) (*domain.ReviewDecision, error)
}
