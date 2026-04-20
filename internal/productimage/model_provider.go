package productimage

import "context"

type GenerationMetadata struct {
	Provider         string
	ModelFamily      string
	GenerationMode   string
	PromptRef        string
	PromptKey        string
	PromptSource     string
	PromptVersion    string
	ReviewConfidence float64
}

func (m *GenerationMetadata) Clone() *GenerationMetadata {
	if m == nil {
		return nil
	}
	cloned := *m
	return &cloned
}

type FaithfulEditRequest struct {
	SourceAsset    *ImageAsset
	ProductContext *ProductContext
	Operation      string
	PromptRef      string
}

type FaithfulEditResult struct {
	Asset    *ImageAsset
	Metadata *GenerationMetadata
}

type SceneGenerationRequest struct {
	SourceAsset     *ImageAsset
	ProductContext  *ProductContext
	PromptRef       string
	SceneIntent     string
	SceneCategory   string
	SceneStyle      string
	BackgroundTone  string
	Composition     string
	PropsLevel      string
	AudienceHint    string
	CustomSceneHint string
}

type SceneGenerationResult struct {
	Assets   []ImageAsset
	Metadata *GenerationMetadata
}

type ReviewModelRequest struct {
	Source  *SourceBundle
	Result  *ImageProcessResult
	Context *ProductContext
}

type ReviewModelResult struct {
	Decision   *ReviewDecision
	Confidence float64
}

type FaithfulEditor interface {
	Edit(ctx context.Context, req *FaithfulEditRequest) (*FaithfulEditResult, error)
}

type SceneGenerator interface {
	GenerateScene(ctx context.Context, req *SceneGenerationRequest) (*SceneGenerationResult, error)
}

type ImageReviewModel interface {
	Review(ctx context.Context, req *ReviewModelRequest) (*ReviewModelResult, error)
}

type ProductImageModelProvider interface {
	FaithfulEditor() FaithfulEditor
	SceneGenerator() SceneGenerator
	ReviewModel() ImageReviewModel
}
