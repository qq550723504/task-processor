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

type CoreItemInput struct {
	Title        string
	ProductType  string
	CategoryPath []string
	Attributes   map[string]string
}

var childrenRelatedKeywords = []string{
	"儿童", "童装", "童鞋", "婴儿", "宝宝", "幼儿", "小孩", "孩子", "童", "婴", "幼",
	"children", "child", "kids", "kid", "baby", "infant", "toddler", "youth", "teen", "school bags",
}

// AISelector AI选择器接口
type AISelector interface {
	// SelectLevelOneCategoryByAI 通过AI选择一级分类
	SelectLevelOneCategoryByAI(ctx context.Context, title string, levelOneIDs []int, levelOneMap map[int]string) (int, error)
	// SelectCategoryByAI 通过AI选择最终分类
	SelectCategoryByAI(ctx context.Context, title string, leafIDs []int, leafMap map[int]string) (int, error)
	// ExtractCoreItemByAI 通过AI从商品上下文中提取核心物品描述（用于SuggestCategoryByText）
	ExtractCoreItemByAI(ctx context.Context, input CoreItemInput) (string, error)
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
	systemPrompt, err := prompt.GetTenantFromContextWithGlobalFallback(ctx, prompt.KSheinCategorySelectorSelectCategorySystem)
	if err != nil {
		return 0, fmt.Errorf("读取租户分类系统提示词失败: %w", err)
	}

	userPrompt, err := prompt.RenderTenantFromContextWithGlobalFallback(ctx, prompt.KSheinCategorySelectorSelectCategoryUser, map[string]any{
		"title":        title,
		"categoryInfo": categoryInfo,
	})
	if err != nil {
		return 0, fmt.Errorf("渲染租户分类用户提示词失败: %w", err)
	}

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

// ExtractCoreItemByAI 通过AI从商品上下文中提取适合 SuggestCategoryByText 的核心检索词。
func (s *OpenAISelector) ExtractCoreItemByAI(ctx context.Context, input CoreItemInput) (string, error) {
	systemPrompt, err := prompt.GetTenantFromContextWithGlobalFallback(ctx, prompt.KSheinCategorySelectorExtractCoreItemSystem)
	if err != nil {
		return "", fmt.Errorf("读取租户核心类目提示词失败: %w", err)
	}

	userPrompt := buildCoreItemPromptInput(input)

	temperature := float32(0.1)
	seed := 42
	req := &openaiClient.ChatCompletionRequest{
		Model: s.openaiClient.GetDefaultModel(),
		Messages: []openaiClient.ChatCompletionMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
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

func buildCoreItemPromptInput(input CoreItemInput) string {
	var builder strings.Builder
	if title := strings.TrimSpace(input.Title); title != "" {
		builder.WriteString("商品标题: ")
		builder.WriteString(title)
		builder.WriteString("\n")
	}
	if productType := strings.TrimSpace(input.ProductType); productType != "" {
		builder.WriteString("产品类别: ")
		builder.WriteString(productType)
		builder.WriteString("\n")
	}
	if len(input.CategoryPath) > 0 {
		path := make([]string, 0, len(input.CategoryPath))
		for _, item := range input.CategoryPath {
			item = strings.TrimSpace(item)
			if item == "" {
				continue
			}
			path = append(path, item)
		}
		if len(path) > 0 {
			builder.WriteString("来源类目: ")
			builder.WriteString(strings.Join(path, " > "))
			builder.WriteString("\n")
		}
	}
	if len(input.Attributes) > 0 {
		attrOrder := []string{"产品类别", "品类", "category", "空间", "用途", "材质", "style"}
		written := 0
		for _, key := range attrOrder {
			value := strings.TrimSpace(input.Attributes[key])
			if value == "" {
				continue
			}
			builder.WriteString(key)
			builder.WriteString(": ")
			builder.WriteString(value)
			builder.WriteString("\n")
			written++
			if written >= 4 {
				break
			}
		}
	}
	return strings.TrimSpace(builder.String())
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
	leafNodes = filterNonChildrenLeafNodes(leafNodes)
	if len(leafNodes) == 0 {
		return 0, fmt.Errorf("未找到非儿童类目叶子节点")
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
func (m *CategoryManager) GetCategoryIDBySuggest(ctx context.Context, input CoreItemInput, categoryAPI interface {
	SuggestCategoryByText(productInfo string) (*category.SuggestCategoryResponse, error)
	GetCategory(categoryID int) (*category.CategoryInfo, error)
}, cache *aicache.Cache) (int, error) {
	// 1. 用AI提取核心物品描述
	aiCtx, cancel := timeout.WithAIShortTimeout(ctx)
	defer cancel()

	coreItem, err := m.aiSelector.ExtractCoreItemByAI(aiCtx, input)
	if err != nil {
		return 0, fmt.Errorf("AI提取核心物品失败: %w", err)
	}
	logger.GetGlobalLogger("shein/category").Infof("AI提取核心物品: title=%q, productType=%q, categoryPath=%q -> coreItem=%q",
		input.Title, input.ProductType, strings.Join(input.CategoryPath, " > "), coreItem)

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

	for _, item := range resp.Data {
		categoryID, parseErr := strconv.Atoi(item.CategoryID)
		if parseErr != nil {
			return 0, fmt.Errorf("SuggestCategoryByText返回的categoryId无法解析为整数: %q", item.CategoryID)
		}
		if categoryAPI == nil {
			logger.GetGlobalLogger("shein/category").Infof("SuggestCategoryByText推荐分类: coreItem=%q, categoryID=%d", coreItem, categoryID)
			if cache != nil {
				cache.Set(aicache.TypeCategory, cacheKey, categoryID)
			}
			return categoryID, nil
		}
		info, infoErr := categoryAPI.GetCategory(categoryID)
		if infoErr != nil {
			return 0, fmt.Errorf("获取SuggestCategory候选类目详情失败: %w", infoErr)
		}
		if !isChildrenRelatedCategoryInfo(info) {
			logger.GetGlobalLogger("shein/category").Infof("SuggestCategoryByText推荐分类: coreItem=%q, categoryID=%d", coreItem, categoryID)
			if cache != nil {
				cache.Set(aicache.TypeCategory, cacheKey, categoryID)
			}
			return categoryID, nil
		}
	}

	logger.GetGlobalLogger("shein/category").Infof("SuggestCategoryByText候选均为儿童相关类目: coreItem=%q", coreItem)
	return 0, nil
}

func filterNonChildrenLeafNodes(nodes []category.CategoryTreeNode) []category.CategoryTreeNode {
	filtered := make([]category.CategoryTreeNode, 0, len(nodes))
	for _, node := range nodes {
		if isChildrenRelatedCategoryPath([]string{buildFullCategoryPath(node)}) {
			continue
		}
		filtered = append(filtered, node)
	}
	return filtered
}

func isChildrenRelatedCategoryInfo(info *category.CategoryInfo) bool {
	if info == nil {
		return false
	}
	path := make([]string, 0, 4)
	for _, name := range []string{
		info.LevelOneCategoryName,
		info.LevelTwoCategoryName,
		info.LevelThreeCategoryName,
	} {
		if strings.TrimSpace(name) != "" {
			path = append(path, name)
		}
	}
	if info.LevelFourCategoryName != nil && strings.TrimSpace(*info.LevelFourCategoryName) != "" {
		path = append(path, *info.LevelFourCategoryName)
	}
	return isChildrenRelatedCategoryPath(path)
}

func isChildrenRelatedCategoryPath(path []string) bool {
	for _, segment := range path {
		normalized := strings.ToLower(strings.TrimSpace(segment))
		if normalized == "" {
			continue
		}
		for _, keyword := range childrenRelatedKeywords {
			if strings.Contains(normalized, strings.ToLower(keyword)) {
				return true
			}
		}
	}
	return false
}
