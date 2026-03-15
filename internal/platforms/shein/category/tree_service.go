package category

import (
	"fmt"
	"task-processor/internal/platforms/shein/model"

	"github.com/sirupsen/logrus"
)

// GetCategoryTreeHandler 获取分类树处理器
type GetCategoryTreeHandler struct {
}

// NewGetCategoryTreeHandler 创建新的获取分类树处理器
func NewGetCategoryTreeHandler() *GetCategoryTreeHandler {
	return &GetCategoryTreeHandler{}
}

// Name 返回处理器名称
func (h *GetCategoryTreeHandler) Name() string {
	return "获取分类树"
}

// Handle 执行获取分类树处理
func (h *GetCategoryTreeHandler) Handle(ctx *model.TaskContext) error {
	// 调用API获取分类树
	categoryTree, err := ctx.CategoryAPI.GetCategoryTree()
	if err != nil {
		return fmt.Errorf("获取分类树失败: %w", err)
	}

	ctx.CategoryTree = categoryTree

	logrus.Infof("成功获取分类树，共 %d 个分类\n", len(categoryTree.Data))
	return nil
}
