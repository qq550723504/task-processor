package shein

import (
	"context"
	"strings"

	sheincategoryselector "task-processor/internal/shein/category"
)

type categorySuggestFallback interface {
	SelectCategoryID(ctx context.Context, input CategoryCoreItemInput, api CategoryAPI) (int, error)
}

type CategoryAIConfig struct {
	Selector         CategoryAISelector
	SemanticVerifier TextGenerator
}

type aiCategorySuggestFallback struct {
	manager *sheincategoryselector.CategoryManager
}

func newAICategorySuggestFallback(selector CategoryAISelector) categorySuggestFallback {
	if selector == nil {
		return nil
	}
	return &aiCategorySuggestFallback{
		manager: sheincategoryselector.NewCategoryManager(categoryAISelectorAdapter{selector: selector}),
	}
}

func (f *aiCategorySuggestFallback) SelectCategoryID(ctx context.Context, input CategoryCoreItemInput, api CategoryAPI) (int, error) {
	if f == nil || f.manager == nil || api == nil || strings.TrimSpace(input.Title) == "" && strings.TrimSpace(input.ProductType) == "" && len(input.CategoryPath) == 0 {
		return 0, nil
	}
	return f.manager.GetCategoryIDBySuggest(ctx, toLegacyCategoryCoreItemInput(input), api, nil)
}
