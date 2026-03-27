package productenrich

import "context"

type ProductUnderstanding interface {
	AnalyzeProduct(ctx context.Context, input *ParsedInput) (*ProductAnalysis, error)
	AnalyzeImage(ctx context.Context, imagePath string) (*ImageAttributes, error)
	ExtractTextAttributes(ctx context.Context, text string) (*TextAttributes, error)
	FuseMultimodal(ctx context.Context, imageAttr *ImageAttributes, textAttr *TextAttributes) (*ProductRepresentation, error)
}
