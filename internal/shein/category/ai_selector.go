package category

import (
	"fmt"
	openaiClient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/shein"
)

// AICategorySelectorHandler AI分类选择处理器
type AICategorySelectorHandler struct {
	openaiClient *openaiClient.Client
}

// NewAICategorySelectorHandler 创建新的AI分类选择处理器
func NewAICategorySelectorHandler(config *openaiClient.ClientConfig) *AICategorySelectorHandler {
	return &AICategorySelectorHandler{
		openaiClient: openaiClient.NewClient(config),
	}
}

// Name 返回处理器名称
func (h *AICategorySelectorHandler) Name() string {
	return "AI分类选择"
}

// Handle 执行AI分类选择处理
func (h *AICategorySelectorHandler) Handle(ctx *shein.TaskContext) error {

	productTitle := ctx.AmazonProduct.Title

	// 创建AI选择器
	aiSelector := NewOpenAISelector(h.openaiClient)

	// 创建分类管理器
	categoryManager := NewCategoryManager(aiSelector)

	// 使用AI选择分类
	selectedCategoryID, err := categoryManager.GetCategoryIDByTitleWithTree(ctx.Context, productTitle, ctx.CategoryTree)
	if err != nil {
		return fmt.Errorf("AI选择分类失败: %w", err)
	}

	ctx.ProductData.CategoryID = selectedCategoryID
	idList, TopCategoryID, productTypeId, err := categoryManager.GetCategoryIDListWithTree(ctx, selectedCategoryID)
	if err != nil {
		return fmt.Errorf("获取分类ID列表失败: %w", err)
	}
	ctx.ProductData.CategoryIDList = idList
	ctx.ProductData.TopCategoryID = TopCategoryID
	ctx.ProductData.ProductTypeID = &productTypeId

	return nil
}
