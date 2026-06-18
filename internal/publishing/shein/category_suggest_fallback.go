package shein

import (
	"context"
	"strings"
)

type categorySuggestFallback interface {
	SelectCategoryID(ctx context.Context, input CategoryCoreItemInput, api CategoryAPI) (int, error)
}

type CategoryAIConfig struct {
	Selector         CategoryAISelector
	SemanticVerifier TextGenerator
}

type categorySuggestManager interface {
	SelectCategoryIDBySuggest(ctx context.Context, input CategoryCoreItemInput, api CategoryAPI) (int, error)
}

type aiCategorySuggestFallback struct {
	manager categorySuggestManager
}

func newAICategorySuggestFallback(selector CategoryAISelector) categorySuggestFallback {
	if selector == nil {
		return nil
	}
	return &aiCategorySuggestFallback{
		manager: newLegacyCategoryManager(selector),
	}
}

func (f *aiCategorySuggestFallback) SelectCategoryID(ctx context.Context, input CategoryCoreItemInput, api CategoryAPI) (int, error) {
	if f == nil || f.manager == nil || api == nil || strings.TrimSpace(input.Title) == "" && strings.TrimSpace(input.ProductType) == "" && len(input.CategoryPath) == 0 {
		return 0, nil
	}
	return f.manager.SelectCategoryIDBySuggest(ctx, input, api)
}
