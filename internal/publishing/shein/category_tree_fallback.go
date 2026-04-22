package shein

import (
	"context"
	"strings"

	openaiclient "task-processor/internal/infra/clients/openai"
	sheincategoryapi "task-processor/internal/shein/api/category"
	sheincategoryselector "task-processor/internal/shein/category"
)

type categoryTreeFallback interface {
	SelectCategoryID(query string, tree *sheincategoryapi.CategoryTreeResponse) (int, error)
}

type aiCategoryTreeFallback struct {
	manager *sheincategoryselector.CategoryManager
}

func newAICategoryTreeFallback(client openaiclient.ChatCompleter) categoryTreeFallback {
	if client == nil {
		return nil
	}
	selector := sheincategoryselector.NewOpenAISelector(client)
	return &aiCategoryTreeFallback{
		manager: sheincategoryselector.NewCategoryManager(selector),
	}
}

func (f *aiCategoryTreeFallback) SelectCategoryID(query string, tree *sheincategoryapi.CategoryTreeResponse) (int, error) {
	if f == nil || f.manager == nil || tree == nil || strings.TrimSpace(query) == "" {
		return 0, nil
	}
	return f.manager.GetCategoryIDByTitleWithTree(context.Background(), query, tree, nil)
}

