package shein

import (
	"context"
	"strings"

	sheincategoryapi "task-processor/internal/shein/api/category"
	sheincategoryselector "task-processor/internal/shein/category"
)

type categoryTreeFallback interface {
	SelectCategoryID(ctx context.Context, query string, tree *sheincategoryapi.CategoryTreeResponse) (int, error)
}

type aiCategoryTreeFallback struct {
	manager *sheincategoryselector.CategoryManager
}

func newAICategoryTreeFallback(selector sheincategoryselector.AISelector) categoryTreeFallback {
	if selector == nil {
		return nil
	}
	return &aiCategoryTreeFallback{
		manager: sheincategoryselector.NewCategoryManager(selector),
	}
}

func (f *aiCategoryTreeFallback) SelectCategoryID(ctx context.Context, query string, tree *sheincategoryapi.CategoryTreeResponse) (int, error) {
	if f == nil || f.manager == nil || tree == nil || strings.TrimSpace(query) == "" {
		return 0, nil
	}
	return f.manager.GetCategoryIDByTitleWithTree(ctx, query, tree, nil)
}
