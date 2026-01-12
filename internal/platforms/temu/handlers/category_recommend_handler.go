package handlers

import (
	"fmt"
	"strings"
	"task-processor/internal/platforms/temu/api/models"
	temucontext "task-processor/internal/platforms/temu/context"

	"github.com/sirupsen/logrus"
)

// CategoryRecommendHandler 分类推荐处理器
type CategoryRecommendHandler struct {
	logger *logrus.Entry
}

// 儿童相关关键词列表
var childrenRelatedKeywords = []string{
	"儿童", "童装", "童鞋", "玩具", "婴儿", "宝宝", "幼儿", "小孩",
	"children", "child", "kids", "kid", "baby", "infant", "toddler", "toy",
	"童", "婴", "幼", "小朋友", "儿", "孩子",
}

// Category 分类信息
type Category struct {
	CatID        int      `json:"cat_id"`
	Cate1ID      int      `json:"cate1_id"`
	Cate1Name    string   `json:"cate1_name"`
	Cate2ID      int      `json:"cate2_id"`
	Cate2Name    string   `json:"cate2_name"`
	Cate3ID      int      `json:"cate3_id"`
	Cate3Name    string   `json:"cate3_name"`
	Cate4ID      *int     `json:"cate4_id"`
	Cate4Name    *string  `json:"cate4_name"`
	Cate5ID      *int     `json:"cate5_id"`
	Cate5Name    *string  `json:"cate5_name"`
	Cate6ID      *int     `json:"cate6_id"`
	Cate6Name    *string  `json:"cate6_name"`
	Cate7ID      *int     `json:"cate7_id"`
	Cate7Name    *string  `json:"cate7_name"`
	Cate8ID      *int     `json:"cate8_id"`
	Cate8Name    *string  `json:"cate8_name"`
	Cate9ID      *int     `json:"cate9_id"`
	Cate9Name    *string  `json:"cate9_name"`
	Cate10ID     *int     `json:"cate10_id"`
	Cate10Name   *string  `json:"cate10_name"`
	CateNameList []string `json:"cate_name_list"`
	CateType     int      `json:"cate_type"`
	Level        int      `json:"level"`
}

// CategoryRecommendRequest 分类推荐请求结构体
type CategoryRecommendRequest struct {
	GoodsName string `json:"goods_name"`
}

// CategoryRecommendResponse 分类推荐响应结构体
type CategoryRecommendResponse struct {
	Success bool       `json:"success"`
	Result  ResultData `json:"result"`
}

type ResultData struct {
	CategoryTreeList []Category `json:"category_tree_list"`
}

// NewCategoryRecommendHandler 创建新的分类推荐处理器
func NewCategoryRecommendHandler() *CategoryRecommendHandler {
	return &CategoryRecommendHandler{
		logger: logrus.WithField("handler", "CategoryRecommendHandler"),
	}
}

// Name 返回处理器名称
func (h *CategoryRecommendHandler) Name() string {
	return "分类推荐处理器"
}

// HandleTemu 处理任务（强类型上下文）
func (h *CategoryRecommendHandler) HandleTemu(temuCtx *temucontext.TemuTaskContext) error {
	h.logger.Info("开始执行分类推荐")

	// 检查任务上下文中的必要数据
	task := temuCtx.GetTask()
	if task == nil {
		return fmt.Errorf("任务信息为空")
	}

	// 获取TEMU产品信息
	if temuCtx.TemuProduct == nil {
		return fmt.Errorf("TEMU产品信息为空")
	}

	temuProduct := temuCtx.TemuProduct

	// 检查是否已经有分类信息
	if temuProduct.GoodsBasic.CatID != 0 {
		h.logger.Infof("产品已有分类信息: CatID=%d，跳过分类推荐", temuProduct.GoodsBasic.CatID)
		return nil
	}
	// 获取商品名称
	var goodsName string
	amazonProduct := temuCtx.GetAmazonProduct()
	if amazonProduct != nil && amazonProduct.Title != "" {
		goodsName = amazonProduct.Title
	}

	if goodsName == "" {
		// 尝试从temuProduct获取商品名称
		if temuProduct.GoodsBasic.GoodsName != "" {
			goodsName = temuProduct.GoodsBasic.GoodsName
		}
	}

	if goodsName == "" {
		return fmt.Errorf("无法获取商品名称")
	}

	// 执行分类推荐
	err := h.recommendCategory(temuCtx, goodsName)
	if err != nil {
		return fmt.Errorf("分类推荐错误: %w", err)
	}

	h.logger.Info("分类推荐处理完成")
	return nil
}

// recommendCategory 执行分类推荐逻辑
func (h *CategoryRecommendHandler) recommendCategory(temuCtx *temucontext.TemuTaskContext, goodsName string) error {
	h.logger.Infof("为商品推荐分类: %s", goodsName)

	// 检查API客户端
	if temuCtx.APIClient == nil {
		return fmt.Errorf("API客户端未初始化")
	}

	// 构造请求体
	requestBody := CategoryRecommendRequest{
		GoodsName: goodsName,
	}

	// 构造API请求
	apiReq := map[string]interface{}{
		"method": "POST",
		"url":    "/mms/marigold/category/recommend",
		"headers": map[string]string{
			"accept":             "application/json, text/plain, */*",
			"accept-language":    "zh-CN,zh;q=0.9",
			"content-type":       "application/json;charset=UTF-8",
			"priority":           "u=1, i",
			"sec-ch-ua":          "\"Chromium\";v=\"140\", \"Not=A?Brand\";v=\"24\", \"Google Chrome\";v=\"140\"",
			"sec-ch-ua-mobile":   "?0",
			"sec-ch-ua-platform": "\"Windows\"",
			"sec-fetch-dest":     "empty",
			"sec-fetch-mode":     "cors",
			"sec-fetch-site":     "same-origin",
			"x-document-referer": "https://seller.temu.com/product-add.html?is_back=1",
		},
		"body": requestBody,
	}

	// 类型断言获取TEMU API客户端
	temuAPIClient, ok := interface{}(temuCtx.APIClient).(interface {
		SendTEMURequest(apiReq map[string]interface{}, response interface{}) error
	})
	if !ok {
		return fmt.Errorf("API客户端不支持TEMU请求")
	}

	// 发送API请求（Cookie检查和重试逻辑已在API客户端中处理）
	response := &CategoryRecommendResponse{}
	err := temuAPIClient.SendTEMURequest(apiReq, response)
	if err != nil {
		h.logger.Errorf("分类推荐API调用失败: %v", err)
		return fmt.Errorf("分类推荐API调用失败: %w", err)
	}

	if !response.Success || len(response.Result.CategoryTreeList) == 0 {
		return fmt.Errorf("分类推荐失败或无推荐结果")
	}

	// 选择合适的分类（避免儿童相关类目）
	selectedCategory, err := h.selectNonChildrenCategory(response.Result.CategoryTreeList)
	if err != nil {
		return fmt.Errorf("无法选择合适的分类: %w", err)
	}

	// 验证分类数据的完整性
	if selectedCategory.CatID == 0 || selectedCategory.Cate1ID == 0 {
		return fmt.Errorf("推荐的分类数据不完整: CatID=%d, Cate1ID=%d", selectedCategory.CatID, selectedCategory.Cate1ID)
	}

	// 构建分类ID层级列表，并找到最深层级的分类ID
	// 从强类型上下文获取TEMU产品信息
	if temuCtx.TemuProduct == nil {
		return fmt.Errorf("无法获取TEMU产品信息")
	}

	temuProduct := temuCtx.TemuProduct

	temuProduct.GoodsBasic.CatIDs = []int{selectedCategory.Cate1ID}
	lastLevelCatID := selectedCategory.Cate1ID // 默认使用第一级

	if selectedCategory.Cate2ID != 0 {
		temuProduct.GoodsBasic.CatIDs = append(temuProduct.GoodsBasic.CatIDs, selectedCategory.Cate2ID)
		lastLevelCatID = selectedCategory.Cate2ID
	}
	if selectedCategory.Cate3ID != 0 {
		temuProduct.GoodsBasic.CatIDs = append(temuProduct.GoodsBasic.CatIDs, selectedCategory.Cate3ID)
		lastLevelCatID = selectedCategory.Cate3ID
	}

	// 设置分类树信息
	temuProduct.GoodsBasic.CategoryTree = models.CategoryTree{
		Level:        selectedCategory.Level,
		CateType:     selectedCategory.CateType,
		CatID:        lastLevelCatID, // 使用最深层级的分类ID
		Cate1ID:      selectedCategory.Cate1ID,
		Cate1Name:    selectedCategory.Cate1Name,
		Cate2ID:      selectedCategory.Cate2ID,
		Cate2Name:    selectedCategory.Cate2Name,
		Cate3ID:      selectedCategory.Cate3ID,
		Cate3Name:    selectedCategory.Cate3Name,
		CateNameList: selectedCategory.CateNameList,
	}

	// 如果有更深层级的分类，也要设置，并更新最深层级ID
	if selectedCategory.Cate4ID != nil && *selectedCategory.Cate4ID != 0 {
		temuProduct.GoodsBasic.CatIDs = append(temuProduct.GoodsBasic.CatIDs, *selectedCategory.Cate4ID)
		temuProduct.GoodsBasic.CategoryTree.Cate4ID = *selectedCategory.Cate4ID
		lastLevelCatID = *selectedCategory.Cate4ID
		if selectedCategory.Cate4Name != nil {
			temuProduct.GoodsBasic.CategoryTree.Cate4Name = *selectedCategory.Cate4Name
		}
	}

	if selectedCategory.Cate5ID != nil && *selectedCategory.Cate5ID != 0 {
		temuProduct.GoodsBasic.CatIDs = append(temuProduct.GoodsBasic.CatIDs, *selectedCategory.Cate5ID)
		temuProduct.GoodsBasic.CategoryTree.Cate5ID = *selectedCategory.Cate5ID
		lastLevelCatID = *selectedCategory.Cate5ID
		if selectedCategory.Cate5Name != nil {
			temuProduct.GoodsBasic.CategoryTree.Cate5Name = *selectedCategory.Cate5Name
		}
	}

	if selectedCategory.Cate6ID != nil && *selectedCategory.Cate6ID != 0 {
		temuProduct.GoodsBasic.CatIDs = append(temuProduct.GoodsBasic.CatIDs, *selectedCategory.Cate6ID)
		lastLevelCatID = *selectedCategory.Cate6ID
	}
	if selectedCategory.Cate7ID != nil && *selectedCategory.Cate7ID != 0 {
		temuProduct.GoodsBasic.CatIDs = append(temuProduct.GoodsBasic.CatIDs, *selectedCategory.Cate7ID)
		lastLevelCatID = *selectedCategory.Cate7ID
	}
	if selectedCategory.Cate8ID != nil && *selectedCategory.Cate8ID != 0 {
		temuProduct.GoodsBasic.CatIDs = append(temuProduct.GoodsBasic.CatIDs, *selectedCategory.Cate8ID)
		lastLevelCatID = *selectedCategory.Cate8ID
	}
	if selectedCategory.Cate9ID != nil && *selectedCategory.Cate9ID != 0 {
		temuProduct.GoodsBasic.CatIDs = append(temuProduct.GoodsBasic.CatIDs, *selectedCategory.Cate9ID)
		lastLevelCatID = *selectedCategory.Cate9ID
	}
	if selectedCategory.Cate10ID != nil && *selectedCategory.Cate10ID != 0 {
		temuProduct.GoodsBasic.CatIDs = append(temuProduct.GoodsBasic.CatIDs, *selectedCategory.Cate10ID)
		lastLevelCatID = *selectedCategory.Cate10ID
	}

	// 设置最深层级的分类ID作为主要CatID
	temuProduct.GoodsBasic.CatID = lastLevelCatID

	h.logger.Infof("成功设置分类信息: CatID=%d, 层级=%d", lastLevelCatID, len(temuProduct.GoodsBasic.CatIDs))
	return nil
}

// isChildrenRelatedCategory 检查分类是否与儿童相关
func (h *CategoryRecommendHandler) isChildrenRelatedCategory(category Category) bool {
	// 检查所有分类名称
	categoryNames := []string{
		category.Cate1Name,
		category.Cate2Name,
		category.Cate3Name,
	}

	// 添加可选的分类名称
	if category.Cate4Name != nil {
		categoryNames = append(categoryNames, *category.Cate4Name)
	}
	if category.Cate5Name != nil {
		categoryNames = append(categoryNames, *category.Cate5Name)
	}
	if category.Cate6Name != nil {
		categoryNames = append(categoryNames, *category.Cate6Name)
	}
	if category.Cate7Name != nil {
		categoryNames = append(categoryNames, *category.Cate7Name)
	}
	if category.Cate8Name != nil {
		categoryNames = append(categoryNames, *category.Cate8Name)
	}
	if category.Cate9Name != nil {
		categoryNames = append(categoryNames, *category.Cate9Name)
	}
	if category.Cate10Name != nil {
		categoryNames = append(categoryNames, *category.Cate10Name)
	}

	// 检查分类名称列表
	categoryNames = append(categoryNames, category.CateNameList...)

	// 检查是否包含儿童相关关键词
	for _, categoryName := range categoryNames {
		if categoryName == "" {
			continue
		}

		categoryNameLower := strings.ToLower(categoryName)
		for _, keyword := range childrenRelatedKeywords {
			if strings.Contains(categoryNameLower, strings.ToLower(keyword)) {
				h.logger.Warnf("检测到儿童相关分类: %s (包含关键词: %s)", categoryName, keyword)
				return true
			}
		}
	}

	return false
}

// selectNonChildrenCategory 选择非儿童相关的分类
func (h *CategoryRecommendHandler) selectNonChildrenCategory(categories []Category) (Category, error) {
	maxAttempts := 3

	// 检查前三个推荐分类
	for i := 0; i < len(categories) && i < maxAttempts; i++ {
		category := categories[i]

		if !h.isChildrenRelatedCategory(category) {
			h.logger.Infof("选择第%d个推荐分类 (非儿童相关): %s", i+1, category.Cate1Name)
			return category, nil
		}

		h.logger.Warnf("第%d个推荐分类包含儿童相关内容，跳过: %s", i+1, category.Cate1Name)
	}

	// 如果前三个都是儿童相关，终止任务
	h.logger.Errorf("前%d个推荐分类都包含儿童相关内容，终止任务", maxAttempts)
	return Category{}, fmt.Errorf("NONRETRYABLE: 前%d个推荐分类都包含儿童相关内容，无法继续处理", maxAttempts)
}
