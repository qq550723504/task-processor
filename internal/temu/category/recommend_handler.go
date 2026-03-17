package category

import (
	"fmt"
	"strings"
	"task-processor/internal/core/logger"
	"task-processor/internal/temu/api"
	temucategory "task-processor/internal/temu/api/category"
	temuproduct "task-processor/internal/temu/api/product"
	temucontext "task-processor/internal/temu/context"

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

// NewCategoryRecommendHandler 创建新的分类推荐处理器
func NewCategoryRecommendHandler() *CategoryRecommendHandler {
	return &CategoryRecommendHandler{
		logger: logger.GetGlobalLogger("temu.handlers.category_recommend").WithField("handler", "CategoryRecommendHandler"),
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
		h.logger.WithField("cat_id", temuProduct.GoodsBasic.CatID).Info("产品已有分类信息，跳过分类推荐")
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
	h.logger.WithField("goods_name", goodsName).Info("为商品推荐分类")

	// 检查API客户端
	if temuCtx.APIClient == nil {
		return fmt.Errorf("API客户端未初始化")
	}

	// 创建CategoryAPI实例
	categoryAPI := api.NewCategoryAPI(temuCtx.APIClient, h.logger)

	// 构造请求体
	request := &temucategory.RecommendRequest{
		GoodsName: goodsName,
	}

	// 发送API请求
	response, err := categoryAPI.Recommend(request)
	if err != nil {
		h.logger.WithError(err).Error("分类推荐API调用失败")
		return fmt.Errorf("分类推荐API调用失败: %w", err)
	}

	if len(response.Result.CategoryTreeList) == 0 {
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
	temuProduct.GoodsBasic.CategoryTree = temuproduct.CategoryTree{
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

	h.logger.WithFields(map[string]any{
		"cat_id": lastLevelCatID,
		"level":  len(temuProduct.GoodsBasic.CatIDs),
	}).Info("成功设置分类信息")
	return nil
}

// isChildrenRelatedCategory 检查分类是否与儿童相关
func (h *CategoryRecommendHandler) isChildrenRelatedCategory(category temucategory.Category) bool {
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
				h.logger.WithFields(map[string]any{
					"category_name": categoryName,
					"keyword":       keyword,
				}).Warn("检测到儿童相关分类")
				return true
			}
		}
	}

	return false
}

// selectNonChildrenCategory 选择非儿童相关的分类
func (h *CategoryRecommendHandler) selectNonChildrenCategory(categories []temucategory.Category) (temucategory.Category, error) {
	maxAttempts := 3

	// 检查前三个推荐分类
	for i := 0; i < len(categories) && i < maxAttempts; i++ {
		category := categories[i]

		if !h.isChildrenRelatedCategory(category) {
			h.logger.WithFields(map[string]any{
				"index":     i + 1,
				"cate_name": category.Cate1Name,
			}).Info("选择推荐分类（非儿童相关）")
			return category, nil
		}

		h.logger.WithFields(map[string]any{
			"index":     i + 1,
			"cate_name": category.Cate1Name,
		}).Warn("推荐分类包含儿童相关内容，跳过")
	}

	// 如果前三个都是儿童相关，终止任务
	h.logger.WithField("max_attempts", maxAttempts).Error("推荐分类都包含儿童相关内容，终止任务")
	return temucategory.Category{}, fmt.Errorf("NONRETRYABLE: 前%d个推荐分类都包含儿童相关内容，无法继续处理", maxAttempts)
}
