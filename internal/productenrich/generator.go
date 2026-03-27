package productenrich

import "context"

type JSONGenerator interface {
	GenerateJSON(ctx context.Context, analysis *ProductAnalysis, variantGen VariantGenerator, skipVariants bool) (*ProductJSON, error)
}
