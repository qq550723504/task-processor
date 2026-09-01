package httpapi

import "context"

const productListingRuntimeAutoMigrateFlag = "product-listing-runtime-auto-migrate"

type BoolEvaluator interface {
	Bool(context.Context, string, bool, map[string]any) bool
}

func shouldAutoMigrateProductListingAPIRuntime(ctx context.Context, evaluator BoolEvaluator) bool {
	if evaluator == nil {
		return true
	}
	return evaluator.Bool(ctx, productListingRuntimeAutoMigrateFlag, true, nil)
}
