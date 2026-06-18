package shein

import (
	"context"
	"strings"

	sheincategoryapi "task-processor/internal/shein/api/category"
)

type categoryTreeFallback interface {
	SelectCategoryID(ctx context.Context, query string, tree *sheincategoryapi.CategoryTreeResponse) (int, error)
}

type categoryTreeManager interface {
	SelectCategoryIDByTree(ctx context.Context, query string, tree *sheincategoryapi.CategoryTreeResponse) (int, error)
}

type aiCategoryTreeFallback struct {
	manager categoryTreeManager
}

func newAICategoryTreeFallback(selector CategoryAISelector) categoryTreeFallback {
	if selector == nil {
		return nil
	}
	return &aiCategoryTreeFallback{
		manager: newLegacyCategoryManager(selector),
	}
}

func (f *aiCategoryTreeFallback) SelectCategoryID(ctx context.Context, query string, tree *sheincategoryapi.CategoryTreeResponse) (int, error) {
	if f == nil || f.manager == nil || tree == nil || strings.TrimSpace(query) == "" {
		return 0, nil
	}
	return f.manager.SelectCategoryIDByTree(ctx, query, tree)
}
