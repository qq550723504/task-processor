// Package category 提供SHEIN平台的分类管理功能，包括AI智能分类选择等
package category

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"task-processor/internal/core/logger"
	openaiClient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/pkg/jsonx"
	"task-processor/internal/pkg/timeout"
	"task-processor/internal/prompt"
	"task-processor/internal/shein"
	"task-processor/internal/shein/aicache"
	"task-processor/internal/shein/api/category"
)

// CategorySelectionResult 分类选择结果
type CategorySelectionResult struct {
	CategoryID int    `json:"category_id"`
	Reason     string `json:"reason"`
}

// AISelector AI选择器接口
type AISelector interface {
	// SelectLevelOneCategoryByAI 通过AI选择一级分类
	SelectLevelOneCategoryByAI(ctx context.Context, title string, levelOneIDs []int, levelOneMap map[int]string) (int, error)
	// SelectCategoryByAI 通过AI选择最终分类
	SelectCategoryByAI(ctx context.Context, title string, leafIDs []int, leafMap map[int]string) (int, error)
	// ExtractCoreItemByAI 通过AI从Amazon标题中提取核心物品描述（用于SuggestCategoryByText）
	ExtractCoreItemByAI(ctx context.Context, title string) (string, error)
}

// OpenAISelector OpenAI选择器实现
type OpenAISelector struct {
	openaiClient openaiClient.ChatCompleter
}

// NewOpenAISelector 创建新的OpenAI选择器
func NewOpenAISelector(client openaiClient.ChatCompleter) *OpenAISelector {
	return &OpenAISelector{
		openaiClient: client,
	}
}

// SelectLevelOneCategoryByAI 通过AI选择一级分类
func (s *OpenAISelector) SelectLevelOneCategoryByAI(ctx context.Context, title string, levelOneIDs []int, levelOneMap map[int]string) (int, error) {
	categoryInfo := s.buildCategoryInfo(levelOneIDs, levelOneMap, "可用一级分类列表：\n", func(id int, name string) string {
		return fmt.Sprintf("分类ID: %d, 分类名称: %s\n", id, name)
	})

	return s.selectCategoryByAI(ctx, title, levelOneIDs, levelOneMap, categoryInfo, "一级分类")
}

// SelectCategoryByAI 通过AI选择最终分类
func (s *OpenAISelector) SelectCategoryByAI(ctx context.Context, title string, leafIDs []int, leafMap map[int]string) (int, error) {
	categoryInfo := s.buildCategoryInfo(leafIDs, leafMap, "可选的最终分类列表：\n", func(id int, path string) string {
		return fmt.Sprintf("分类ID: %d, 分类路径: %s\n", id, path)
	})

	return s.selectCategoryByAI(ctx, title, leafIDs, leafMap, categoryInfo, "最终分类")
}

// selectCategoryByAI 通用的AI分类选择方法（提取重复逻辑）
func (s *OpenAISelector) selectCategoryByAI(
	ctx context.Context,
	title string,
	categoryIDs []int,
	categoryMap map[int]string,
	categoryInfo string,
	categoryType string,
) (int, error) {
	// 从 prompt registry 获取系统提示词
	systemPrompt := prompt.GlobalRegistry.Get(prompt.KSheinCategorySelectorSelectCategorySystem,
		`你是一个专业的电商产品分类专家。根据产品标题，从给定的分类列表中选择最合适的分类ID。

分析原则：
1. 仔细分析产品标题中的关键词
2. 理解产品的类型、用途、材质等特征
3. 从分类路径中找到最精确匹配的分类
4. 优先选择更具体、更精准的分类
5. 考虑产品的主要功能和用途

返回格式：
请返回JSON格式，包含选中的分类ID和选择理由：
{
  "category_id": 12345,
  "reason": "选择该分类的详细理由"
}

注意：category_id必须是给定列表中的有效ID。`)

	// 从 prompt registry 渲染用户提示词
	userPrompt, _ := prompt.GlobalRegistry.Render(prompt.KSheinCategorySelectorSelectCategoryUser, map[string]any{
		"title":        title,
		"categoryInfo": categoryInfo,
	}, fmt.Sprintf("产品标题：%s\n\n%s\n\n请分析产品标题，选择最合适的分类ID（必须从上述列表中选择）：", title, categoryInfo))

	// 设置seed确保结果一致性
	seed := 42
	temperature := float32(0.1)

	// 调用OpenAI API
	messages := []openaiClient.ChatCompletionMessage{
		{
			Role:    "system",
			Content: systemPrompt,
		},
		{
			Role:    "user",
			Content: userPrompt,
		},
	}

	req := &openaiClient.ChatCompletionRequest{
		Model:       s.openaiClient.GetDefaultModel(),
		Messages:    messages,
		Temperature: &temperature,
		Seed:        &seed,
	}

	resp, err := s.openaiClient.CreateChatCompletion(ctx, req)
	if err != nil {
		return 0, fmt.Errorf("调用OpenAI API失败: %w", err)
	}

	if len(resp.Choices) == 0 {
		return 0, fmt.Errorf("OpenAI API返回空响应")
	}

	// 解析响应
	result, err := s.parseOpenAIResponse(resp)
	if err != nil {
		return 0, err
	}

	// 验证选择的ID是否在可选范围内
	if !slices.Contains(categoryIDs, result.CategoryID) {
		return 0, fmt.Errorf("选择的分类ID %d 不在可选范围内", result.CategoryID)
	}

	logger.GetGlobalLogger("shein/category").Infof("AI成功选择%s: ID=%d, 名称/路径=%s, 理由=%s\n",
		categoryType, result.CategoryID, categoryMap[result.CategoryID], result.Reason)
	return result.CategoryID, nil
}

// buildCategoryInfo 构建分类信息字符串
func (s *OpenAISelector) buildCategoryInfo(ids []int, data map[int]string, header string, formatFunc func(int, string) string) string {
	var categoryInfo strings.Builder
	categoryInfo.WriteString(header)
	for _, id := range ids {
		categoryInfo.WriteString(formatFunc(id, data[id]))
	}
	return categoryInfo.String()
}

// parseOpenAIResponse 解析OpenAI响应
func (s *OpenAISelector) parseOpenAIResponse(resp *openaiClient.ChatCompletionResponse) (*CategorySelectionResult, error) {
	// 解析响应中的分类ID
	content := strings.TrimSpace(resp.Choices[0].Message.Content)
	// 处理可能的代码块格式
	if after, ok := strings.CutPrefix(content, "```json"); ok {
		content = after
		content = strings.TrimSuffix(content, "```")
		content = strings.TrimSpace(content)
	}

	// 解析JSON响应
	var result CategorySelectionResult
	if err := jsonx.UnmarshalString(content, &result, "解析AI分类选择结果失败"); err != nil {
		logger.GetGlobalLogger("shein/category").Infof("解析AI分类选择结果失败: %v, 内容: %s\n", err, content)
		return nil, err
	}

	return &result, nil
}

// ExtractCoreItemByAI 通过AI从Amazon标题中提取核心物品的简短描述，用于SuggestCategoryByText。
func (s *OpenAISelector) ExtractCoreItemByAI(ctx context.Context, title string) (string, error) {
	systemPrompt := prompt.GlobalRegistry.Get(prompt.KSheinCategorySelectorExtractCoreItemSystem,
		`你是一个电商产品分析专家。请从Amazon产品标题中提取出该商品的核心物品描述。
要求：
1. 用简洁的中文描述核心物品，例如"环保塑料杯套装"、"不锈钢保温水壶"、"儿童棉质连衣裙"
2. 保留关键材质、用途、人群等修饰词，但去掉品牌名、数量、尺寸、颜色等无关信息
3. 长度控制在5-20个汉字
4. 只返回描述文本，不要任何解释`)

	temperature := float32(0.1)
	seed := 42
	req := &openaiClient.ChatCompletionRequest{
		Model: s.openaiClient.GetDefaultModel(),
		Messages: []openaiClient.ChatCompletionMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: title},
		},
		Temperature: &temperature,
		Seed:        &seed,
	}

	resp, err := s.openaiClient.CreateChatCompletion(ctx, req)
	if err != nil {
		return "", fmt.Errorf("调用OpenAI API失败: %w", err)
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("OpenAI API返回空响应")
	}

	return strings.TrimSpace(resp.Choices[0].Message.Content), nil
}

// CategoryManager 分类管理器
type CategoryManager struct {
	aiSelector AISelector
}

// NewCategoryManager 创建新的分类管理器
func NewCategoryManager(aiSelector AISelector) *CategoryManager {
	return &CategoryManager{
		aiSelector: aiSelector,
	}
}

// GetCategoryIDListWithTree 通过SHEIN接口获取分类ID列表
func (m *CategoryManager) GetCategoryIDListWithTree(ctx *shein.TaskContext, categoryID int) ([]int, int, int, error) {
	// 调用API获取分类信息
	categoryInfo, err := ctx.CategoryAPI.GetCategory(categoryID)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("获取分类信息失败: %w", err)
	}

	// 按层级顺序组装
	var idList []int

	if categoryInfo.LevelOneCategoryID > 0 {
		idList = append(idList, categoryInfo.LevelOneCategoryID)
	}

	if categoryInfo.LevelTwoCategoryID > 0 {
		idList = append(idList, categoryInfo.LevelTwoCategoryID)
	}

	if categoryInfo.LevelThreeCategoryID > 0 {
		idList = append(idList, categoryInfo.LevelThreeCategoryID)
	}

	if categoryInfo.LevelFourCategoryID != nil && *categoryInfo.LevelFourCategoryID > 0 {
		idList = append(idList, *categoryInfo.LevelFourCategoryID)
	}

	return idList, categoryInfo.LevelOneCategoryID, categoryInfo.ProductTypeID, nil
}

// GetCategoryIDByTitleWithTree 用产品标题获取分类ID，优先读 AI 缓存。
func (m *CategoryManager) GetCategoryIDByTitleWithTree(ctx context.Context, title string, categoryTree *category.CategoryTreeResponse, cache *aicache.Cache) (int, error) {
	// 查缓存
	if cache != nil {
		cacheKey := aicache.HashKey(title)
		var cached int
		if cache.Get(aicache.TypeCategory, cacheKey, &cached) {
			logger.GetGlobalLogger("shein/category").Infof("AI分类选择命中缓存: title=%s, categoryID=%d", title, cached)
			return cached, nil
		}
	}

	// 1. 获取所有一级分类节点
	levelOneNodes := getLevelOneCategories(categoryTree.Data)
	levelOneIDs := make([]int, 0, len(levelOneNodes))
	levelOneMap := make(map[int]string)
	for _, node := range levelOneNodes {
		levelOneIDs = append(levelOneIDs, node.CategoryID)
		levelOneMap[node.CategoryID] = node.CategoryName
	}

	// 2. AI选择一级分类 - 使用传入的context，添加超时控制
	aiCtx, cancel := timeout.WithAIShortTimeout(ctx)
	defer cancel()

	selectedLevelOneID, err := m.aiSelector.SelectLevelOneCategoryByAI(aiCtx, title, levelOneIDs, levelOneMap)
	if err != nil {
		logger.GetGlobalLogger("shein/category").Infof("AI选择一级分类失败: %v\n", err)
		return 0, fmt.Errorf("AI选择一级分类失败且无可用分类: %w", err)
	}

	// 3. 获取该一级分类下所有叶子节点
	leafNodes := getLeafNodesUnderCategory(selectedLevelOneID, categoryTree.Data)
	if len(leafNodes) == 0 {
		return 0, fmt.Errorf("未找到一级分类下的叶子节点")
	}
	leafIDs := make([]int, 0, len(leafNodes))
	leafMap := make(map[int]string)
	for _, node := range leafNodes {
		leafIDs = append(leafIDs, node.CategoryID)
		leafMap[node.CategoryID] = buildFullCategoryPath(node)
	}

	// 4. AI在这些叶子节点中做最终选择 - 使用传入的context，添加超时控制
	aiCtx2, cancel2 := timeout.WithAIShortTimeout(ctx)
	defer cancel2()

	selectedCategoryID, err := m.aiSelector.SelectCategoryByAI(aiCtx2, title, leafIDs, leafMap)
	if err != nil {
		return 0, fmt.Errorf("AI选择最终分类失败: %w", err)
	}

	// 写缓存
	if cache != nil {
		cacheKey := aicache.HashKey(title)
		cache.Set(aicache.TypeCategory, cacheKey, selectedCategoryID)
	}

	return selectedCategoryID, nil
}

// getLevelOneCategories 获取所有一级分类节点
func getLevelOneCategories(nodes []category.CategoryTreeNode) []category.CategoryTreeNode {
	return append([]category.CategoryTreeNode{}, nodes...)
}

// getLeafNodesUnderCategory 获取指定分类下的所有叶子节点
func getLeafNodesUnderCategory(categoryID int, nodes []category.CategoryTreeNode) []category.CategoryTreeNode {
	for _, node := range nodes {
		if node.CategoryID == categoryID {
			return collectLeafNodes(node)
		}
	}
	return nil
}

// collectLeafNodes 收集叶子节点
func collectLeafNodes(node category.CategoryTreeNode) []category.CategoryTreeNode {
	if len(node.Children) == 0 {
		return []category.CategoryTreeNode{node}
	}
	var result []category.CategoryTreeNode
	for _, child := range node.Children {
		result = append(result, collectLeafNodes(child)...)
	}
	return result
}

// buildFullCategoryPath 构建分类路径（递归向上）
func buildFullCategoryPath(node category.CategoryTreeNode) string {
	return node.CategoryName
}

// GetCategoryIDBySuggest 通过SuggestCategoryByText接口推荐分类，返回第一个有效的分类ID。
// categoryAPI 为 nil 或接口返回空结果时返回 0, nil（调用方应 fallback）。
func (m *CategoryManager) GetCategoryIDBySuggest(ctx context.Context, title string, categoryAPI interface {
	SuggestCategoryByText(productInfo string) (*category.SuggestCategoryResponse, error)
}, cache *aicache.Cache) (int, error) {
	// 1. 用AI提取核心物品描述
	aiCtx, cancel := timeout.WithAIShortTimeout(ctx)
	defer cancel()

	coreItem, err := m.aiSelector.ExtractCoreItemByAI(aiCtx, title)
	if err != nil {
		return 0, fmt.Errorf("AI提取核心物品失败: %w", err)
	}
	logger.GetGlobalLogger("shein/category").Infof("AI提取核心物品: title=%q -> coreItem=%q", title, coreItem)

	// 2. 查缓存（以 coreItem 为 key）
	cacheKey := aicache.HashKey("suggest:" + coreItem)
	if cache != nil {
		var cached int
		if cache.Get(aicache.TypeCategory, cacheKey, &cached) {
			logger.GetGlobalLogger("shein/category").Infof("SuggestCategory命中缓存: coreItem=%q, categoryID=%d", coreItem, cached)
			return cached, nil
		}
	}

	// 3. 调用 SuggestCategoryByText
	resp, err := categoryAPI.SuggestCategoryByText(coreItem)
	if err != nil {
		return 0, fmt.Errorf("SuggestCategoryByText调用失败: %w", err)
	}
	if resp == nil || len(resp.Data) == 0 {
		logger.GetGlobalLogger("shein/category").Infof("SuggestCategoryByText返回空结果: coreItem=%q", coreItem)
		return 0, nil
	}

	categoryID, err := strconv.Atoi(resp.Data[0].CategoryID)
	if err != nil {
		return 0, fmt.Errorf("SuggestCategoryByText返回的categoryId无法解析为整数: %q", resp.Data[0].CategoryID)
	}
	logger.GetGlobalLogger("shein/category").Infof("SuggestCategoryByText推荐分类: coreItem=%q, categoryID=%d", coreItem, categoryID)

	// 4. 写缓存
	if cache != nil {
		cache.Set(aicache.TypeCategory, cacheKey, categoryID)
	}

	return categoryID, nil
}
