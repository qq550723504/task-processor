package shein

import (
	"context"
)

type CategoryCoreItemInput struct {
	Title        string
	ProductType  string
	CategoryPath []string
	Attributes   map[string]string
}

type CategoryAISelector interface {
	SelectLevelOneCategoryByAI(ctx context.Context, title string, levelOneIDs []int, levelOneMap map[int]string) (int, error)
	SelectCategoryByAI(ctx context.Context, title string, leafIDs []int, leafMap map[int]string) (int, error)
	ExtractCoreItemByAI(ctx context.Context, input CategoryCoreItemInput) (string, error)
}
