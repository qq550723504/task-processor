package productenrich

import "context"

type VariantGenerator interface {
	GenerateSpecs(ctx context.Context, analysis *ProductAnalysis) (*ProductSpecs, error)
	GenerateVariants(ctx context.Context, analysis *ProductAnalysis) ([]ProductVariant, error)
	ExtractDimensions(ctx context.Context, text string) (*Dimensions, error)
	ExtractWeight(ctx context.Context, text string) (*Weight, error)
}
