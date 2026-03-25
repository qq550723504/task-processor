package category

import (
	"fmt"

	"task-processor/internal/core/logger"
	"task-processor/internal/shein"
)

type GetCategoryTreeHandler struct{}

func NewGetCategoryTreeHandler() *GetCategoryTreeHandler {
	return &GetCategoryTreeHandler{}
}

func (h *GetCategoryTreeHandler) Name() string {
	return "get_category_tree"
}

func (h *GetCategoryTreeHandler) Handle(ctx *shein.TaskContext) error {
	categoryTree, err := ctx.CategoryAPI.GetCategoryTree()
	if err != nil {
		return fmt.Errorf("get category tree failed: %w", err)
	}

	ctx.SetCategoryTree(categoryTree)
	logger.GetGlobalLogger("shein/category").Infof("loaded category tree: total=%d", len(categoryTree.Data))
	return nil
}
