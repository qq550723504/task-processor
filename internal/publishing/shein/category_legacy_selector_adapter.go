package shein

import (
	"context"

	sheincategoryselector "task-processor/internal/shein/category"
)

type categoryAISelectorAdapter struct {
	selector CategoryAISelector
}

func (a categoryAISelectorAdapter) SelectLevelOneCategoryByAI(ctx context.Context, title string, levelOneIDs []int, levelOneMap map[int]string) (int, error) {
	return a.selector.SelectLevelOneCategoryByAI(ctx, title, levelOneIDs, levelOneMap)
}

func (a categoryAISelectorAdapter) SelectCategoryByAI(ctx context.Context, title string, leafIDs []int, leafMap map[int]string) (int, error) {
	return a.selector.SelectCategoryByAI(ctx, title, leafIDs, leafMap)
}

func (a categoryAISelectorAdapter) ExtractCoreItemByAI(ctx context.Context, input sheincategoryselector.CoreItemInput) (string, error) {
	return a.selector.ExtractCoreItemByAI(ctx, CategoryCoreItemInput{
		Title:        input.Title,
		ProductType:  input.ProductType,
		CategoryPath: append([]string(nil), input.CategoryPath...),
		Attributes:   cloneCategoryAttributes(input.Attributes),
	})
}

func toLegacyCategoryCoreItemInput(input CategoryCoreItemInput) sheincategoryselector.CoreItemInput {
	return sheincategoryselector.CoreItemInput{
		Title:        input.Title,
		ProductType:  input.ProductType,
		CategoryPath: append([]string(nil), input.CategoryPath...),
		Attributes:   cloneCategoryAttributes(input.Attributes),
	}
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
