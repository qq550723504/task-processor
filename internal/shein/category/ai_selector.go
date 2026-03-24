package category

import (
	"fmt"
	"task-processor/internal/core/logger"
	openaiClient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/shein"
)

// AICategorySelectorHandler AI分类选择处理器
type AICategorySelectorHandler struct {
	openaiClient openaiClient.ChatCompleter
}

// NewAICategorySelectorHandler 创建新的AI分类选择处理器
func NewAICategorySelectorHandler(client openaiClient.ChatCompleter) *AICategorySelectorHandler {
	return &AICategorySelectorHandler{
		openaiClient: client,
	}
}

// Name 返回处理器名称
func (h *AICategorySelectorHandler) Name() string {
	return "AI分类选择"
}

// Handle 执行AI分类选择处理
func (h *AICategorySelectorHandler) Handle(ctx *shein.TaskContext) error {
	productTitle := ctx.AmazonProduct.Title

	// 创建AI选择器和分类管理器
	aiSelector := NewOpenAISelector(h.openaiClient)
	categoryManager := NewCategoryManager(aiSelector)

	// 优先尝试：AI提取核心物品 -> SuggestCategoryByText
	suggestedID, err := categoryManager.GetCategoryIDBySuggest(ctx.Context, productTitle, ctx.CategoryAPI, ctx.AICache)
	if err != nil {
		logger.GetGlobalLogger("shein/category").Warnf("SuggestCategory流程失败，降级到AI分类树选择: %v", err)
	}

	var selectedCategoryID int
	if suggestedID > 0 {
		selectedCategoryID = suggestedID
	} else {
		// Fallback：原有两级AI分类树选择
		logger.GetGlobalLogger("shein/category").Infof("SuggestCategory无结果，使用AI分类树选择: title=%q", productTitle)
		selectedCategoryID, err = categoryManager.GetCategoryIDByTitleWithTree(ctx.Context, productTitle, ctx.CategoryTree, ctx.AICache)
		if err != nil {
			return fmt.Errorf("AI选择分类失败: %w", err)
		}
	}

	ctx.ProductData.CategoryID = selectedCategoryID
	idList, topCategoryID, productTypeID, err := categoryManager.GetCategoryIDListWithTree(ctx, selectedCategoryID)
	if err != nil {
		return fmt.Errorf("获取分类ID列表失败: %w", err)
	}
	ctx.ProductData.CategoryIDList = idList
	ctx.ProductData.TopCategoryID = topCategoryID
	ctx.ProductData.ProductTypeID = &productTypeID

	return nil
}
