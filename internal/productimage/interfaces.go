package productimage

import "context"

type TaskSubmitter interface {
	Submit(taskID string) error
}

type SourceParser interface {
	Parse(ctx context.Context, req *ImageProcessRequest) (*SourceBundle, error)
}

type ProductContextAnalyzer interface {
	Analyze(ctx context.Context, source *SourceBundle) (*ProductContext, error)
}

type ImageInspector interface {
	Inspect(ctx context.Context, source *SourceBundle, imageURL string) (*ImageAudit, error)
}

type ImageRanker interface {
	Select(ctx context.Context, source *SourceBundle, audits []ImageAudit, context *ProductContext) (*ImageCandidateSet, error)
}

type SubjectExtractor interface {
	Extract(ctx context.Context, imageURL string, context *ProductContext) (*ImageAsset, error)
}

type ImageCleaner interface {
	Clean(ctx context.Context, asset *ImageAsset, context *ProductContext) (*ImageAsset, error)
}

type WhiteBackgroundRenderer interface {
	Render(ctx context.Context, asset *ImageAsset, context *ProductContext) (*ImageAsset, error)
}

type AssetPublisher interface {
	Publish(ctx context.Context, req *ImageProcessRequest, result *ImageProcessResult) error
}

type SceneRenderer interface {
	Render(ctx context.Context, asset *ImageAsset, context *ProductContext) ([]ImageAsset, error)
}

type MarketplaceValidator interface {
	Validate(ctx context.Context, req *ImageProcessRequest, result *ImageProcessResult) (*ComplianceReport, error)
}

type QualityAssessor interface {
	Assess(ctx context.Context, source *SourceBundle, audits []ImageAudit, candidates *ImageCandidateSet, result *ImageProcessResult) (*QualityAssessment, error)
}

type ReviewAssessor interface {
	Assess(ctx context.Context, source *SourceBundle, audits []ImageAudit, candidates *ImageCandidateSet, result *ImageProcessResult) (*ReviewDecision, error)
}
