package handlers

import (
	"fmt"
	"task-processor/common/pipeline"
	"task-processor/platforms/temu/types"

	"github.com/sirupsen/logrus"
)

// CategoryRecommendHandler 分类推荐处理器
type CategoryRecommendHandler struct {
	logger *logrus.Entry
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

// Handle 处理任务
func (h *CategoryRecommendHandler) Handle(ctx *pipeline.TaskContext) error {
	h.logger.Info("开始执行分类推荐")

	// 检查任务上下文中的必要数据
	if ctx.Task == nil {
		return fmt.Errorf("任务信息为空")
	}

	if ctx.TemuProduct == nil {
		return fmt.Errorf("TEMU产品信息为空")
	}

	// 检查是否已经有分类信息
	if ctx.TemuProduct.GoodsBasic.CatID != 0 {
		h.logger.Infof("产品已有分类信息: CatID=%d，跳过分类推荐", ctx.TemuProduct.GoodsBasic.CatID)
		return nil
	}

	// 获取商品名称
	var goodsName string
	if ctx.AmazonProduct != nil && ctx.AmazonProduct.Title != "" {
		goodsName = ctx.AmazonProduct.Title
	}

	// 执行分类推荐
	err := h.recommendCategory(ctx, goodsName)
	if err != nil {
		return fmt.Errorf("分类推荐错误")
	}

	h.logger.Info("分类推荐处理完成")
	return nil
}

// recommendCategory 执行分类推荐逻辑
func (h *CategoryRecommendHandler) recommendCategory(ctx *pipeline.TaskContext, goodsName string) error {
	h.logger.Infof("为商品推荐分类: %s", goodsName)

	// 检查API客户端
	if ctx.APIClient == nil {
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

	// 发送API请求（Cookie检查和重试逻辑已在API客户端中处理）
	response := &CategoryRecommendResponse{}
	err := ctx.APIClient.SendTEMURequest(apiReq, response)
	if err != nil {
		h.logger.Errorf("分类推荐API调用失败: %v", err)
		return fmt.Errorf("分类推荐API调用失败: %w", err)
	}

	if !response.Success || len(response.Result.CategoryTreeList) == 0 {
		return fmt.Errorf("分类推荐失败或无推荐结果")
	}

	// 获取第一个推荐分类（通常API会返回按相关度排序的分类列表）
	category := response.Result.CategoryTreeList[0]

	// 验证分类数据的完整性
	if category.CatID == 0 || category.Cate1ID == 0 {
		return fmt.Errorf("推荐的分类数据不完整: CatID=%d, Cate1ID=%d", category.CatID, category.Cate1ID)
	}

	// 构建分类ID层级列表，并找到最深层级的分类ID
	ctx.TemuProduct.GoodsBasic.CatIDs = []int{category.Cate1ID}
	lastLevelCatID := category.Cate1ID // 默认使用第一级

	if category.Cate2ID != 0 {
		ctx.TemuProduct.GoodsBasic.CatIDs = append(ctx.TemuProduct.GoodsBasic.CatIDs, category.Cate2ID)
		lastLevelCatID = category.Cate2ID
	}
	if category.Cate3ID != 0 {
		ctx.TemuProduct.GoodsBasic.CatIDs = append(ctx.TemuProduct.GoodsBasic.CatIDs, category.Cate3ID)
		lastLevelCatID = category.Cate3ID
	}

	// 设置分类树信息
	ctx.TemuProduct.GoodsBasic.CategoryTree = types.CategoryTree{
		Level:        category.Level,
		CateType:     category.CateType,
		CatID:        lastLevelCatID, // 使用最深层级的分类ID
		Cate1ID:      category.Cate1ID,
		Cate1Name:    category.Cate1Name,
		Cate2ID:      category.Cate2ID,
		Cate2Name:    category.Cate2Name,
		Cate3ID:      category.Cate3ID,
		Cate3Name:    category.Cate3Name,
		CateNameList: category.CateNameList,
	}

	// 如果有更深层级的分类，也要设置，并更新最深层级ID
	if category.Cate4ID != nil && *category.Cate4ID != 0 {
		ctx.TemuProduct.GoodsBasic.CatIDs = append(ctx.TemuProduct.GoodsBasic.CatIDs, *category.Cate4ID)
		ctx.TemuProduct.GoodsBasic.CategoryTree.Cate4ID = *category.Cate4ID
		lastLevelCatID = *category.Cate4ID
		if category.Cate4Name != nil {
			ctx.TemuProduct.GoodsBasic.CategoryTree.Cate4Name = *category.Cate4Name
		}
	}

	if category.Cate5ID != nil && *category.Cate5ID != 0 {
		ctx.TemuProduct.GoodsBasic.CatIDs = append(ctx.TemuProduct.GoodsBasic.CatIDs, *category.Cate5ID)
		ctx.TemuProduct.GoodsBasic.CategoryTree.Cate5ID = *category.Cate5ID
		lastLevelCatID = *category.Cate5ID
		if category.Cate5Name != nil {
			ctx.TemuProduct.GoodsBasic.CategoryTree.Cate5Name = *category.Cate5Name
		}
	}

	// 添加对更深层级分类的支持，并持续更新最深层级ID
	if category.Cate6ID != nil && *category.Cate6ID != 0 {
		ctx.TemuProduct.GoodsBasic.CatIDs = append(ctx.TemuProduct.GoodsBasic.CatIDs, *category.Cate6ID)
		lastLevelCatID = *category.Cate6ID
	}
	if category.Cate7ID != nil && *category.Cate7ID != 0 {
		ctx.TemuProduct.GoodsBasic.CatIDs = append(ctx.TemuProduct.GoodsBasic.CatIDs, *category.Cate7ID)
		lastLevelCatID = *category.Cate7ID
	}
	if category.Cate8ID != nil && *category.Cate8ID != 0 {
		ctx.TemuProduct.GoodsBasic.CatIDs = append(ctx.TemuProduct.GoodsBasic.CatIDs, *category.Cate8ID)
		lastLevelCatID = *category.Cate8ID
	}
	if category.Cate9ID != nil && *category.Cate9ID != 0 {
		ctx.TemuProduct.GoodsBasic.CatIDs = append(ctx.TemuProduct.GoodsBasic.CatIDs, *category.Cate9ID)
		lastLevelCatID = *category.Cate9ID
	}
	if category.Cate10ID != nil && *category.Cate10ID != 0 {
		ctx.TemuProduct.GoodsBasic.CatIDs = append(ctx.TemuProduct.GoodsBasic.CatIDs, *category.Cate10ID)
		lastLevelCatID = *category.Cate10ID
	}

	// 设置最深层级的分类ID作为主要CatID
	ctx.TemuProduct.GoodsBasic.CatID = lastLevelCatID

	return nil
}
