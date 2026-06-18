package sheinmanaged

import (
	"context"

	openaiclient "task-processor/internal/infra/clients/openai"
	sheinpub "task-processor/internal/publishing/shein"
	sheincategoryselector "task-processor/internal/shein/category"
)

type categorySelectorAdapter struct {
	selector sheincategoryselector.AISelector
}

func newCategorySelectorAdapter(llm openaiclient.ChatCompleter) sheinpub.CategoryAISelector {
	if llm == nil {
		return nil
	}
	return categorySelectorAdapter{selector: sheincategoryselector.NewOpenAISelector(llm)}
}

func (a categorySelectorAdapter) SelectLevelOneCategoryByAI(ctx context.Context, title string, levelOneIDs []int, levelOneMap map[int]string) (int, error) {
	return a.selector.SelectLevelOneCategoryByAI(ctx, title, levelOneIDs, levelOneMap)
}

func (a categorySelectorAdapter) SelectCategoryByAI(ctx context.Context, title string, leafIDs []int, leafMap map[int]string) (int, error) {
	return a.selector.SelectCategoryByAI(ctx, title, leafIDs, leafMap)
}

func (a categorySelectorAdapter) ExtractCoreItemByAI(ctx context.Context, input sheinpub.CategoryCoreItemInput) (string, error) {
	return a.selector.ExtractCoreItemByAI(ctx, sheincategoryselector.CoreItemInput{
		Title:        input.Title,
		ProductType:  input.ProductType,
		CategoryPath: append([]string(nil), input.CategoryPath...),
		Attributes:   cloneCategoryAttributes(input.Attributes),
	})
}

func cloneCategoryAttributes(attributes map[string]string) map[string]string {
	if len(attributes) == 0 {
		return nil
	}
	clone := make(map[string]string, len(attributes))
	for key, value := range attributes {
		clone[key] = value
	}
	return clone
}
