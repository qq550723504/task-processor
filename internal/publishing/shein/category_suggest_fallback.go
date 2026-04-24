package shein

import (
	"context"
	"strings"

	openaiclient "task-processor/internal/infra/clients/openai"
	sheincategoryselector "task-processor/internal/shein/category"
)

type categorySuggestFallback interface {
	SelectCategoryID(input sheincategoryselector.CoreItemInput, api CategoryAPI) (int, error)
}

type aiCategorySuggestFallback struct {
	manager *sheincategoryselector.CategoryManager
}

func newAICategorySuggestFallback(client openaiclient.ChatCompleter) categorySuggestFallback {
	if client == nil {
		return nil
	}
	selector := sheincategoryselector.NewOpenAISelector(client)
	return &aiCategorySuggestFallback{
		manager: sheincategoryselector.NewCategoryManager(selector),
	}
}

func (f *aiCategorySuggestFallback) SelectCategoryID(input sheincategoryselector.CoreItemInput, api CategoryAPI) (int, error) {
	if f == nil || f.manager == nil || api == nil || strings.TrimSpace(input.Title) == "" && strings.TrimSpace(input.ProductType) == "" && len(input.CategoryPath) == 0 {
		return 0, nil
	}
	return f.manager.GetCategoryIDBySuggest(context.Background(), input, api, nil)
}
