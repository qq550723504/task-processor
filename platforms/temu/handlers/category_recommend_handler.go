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

	// 获取商品名称
	var goodsName string
	if ctx.AmazonProduct != nil && ctx.AmazonProduct.Title != "" {
		goodsName = ctx.AmazonProduct.Title
	} else if ctx.TemuProduct.GoodsBasic.GoodsName != "" {
		goodsName = ctx.TemuProduct.GoodsBasic.GoodsName
	} else {
		return fmt.Errorf("商品名称为空")
	}

	// 执行分类推荐
	err := h.recommendCategory(ctx, goodsName)
	if err != nil {
		h.logger.Warnf("分类推荐警告: %v", err)
		// 分类推荐失败使用默认分类
		h.setDefaultCategory(ctx)
	}

	h.logger.Info("分类推荐完成")
	return nil
}

// recommendCategory 执行分类推荐逻辑
func (h *CategoryRecommendHandler) recommendCategory(ctx *pipeline.TaskContext, goodsName string) error {
	h.logger.Infof("为商品推荐分类: %s", goodsName)

	// 这里应该调用TEMU API进行分类推荐
	// 构造请求体（示例）
	// requestBody := CategoryRecommendRequest{
	//     GoodsName: goodsName,
	// }

	// 为了简化，我们模拟推荐结果
	response := &CategoryRecommendResponse{
		Success: true,
		Result: ResultData{
			CategoryTreeList: []Category{
				{
					CatID:        30469,
					Cate1ID:      1001,
					Cate1Name:    "服装",
					Cate2ID:      2001,
					Cate2Name:    "女装",
					Cate3ID:      3001,
					Cate3Name:    "连衣裙",
					CateNameList: []string{"服装", "女装", "连衣裙"},
					CateType:     1,
					Level:        3,
				},
			},
		},
	}

	if !response.Success || len(response.Result.CategoryTreeList) == 0 {
		return fmt.Errorf("分类推荐失败或无推荐结果")
	}

	// 获取第一个推荐分类
	category := response.Result.CategoryTreeList[0]

	// 设置分类信息到产品
	ctx.TemuProduct.GoodsBasic.CatID = category.CatID
	ctx.TemuProduct.GoodsBasic.CatIDs = []int{category.Cate1ID, category.Cate2ID, category.Cate3ID}

	// 设置分类树信息
	ctx.TemuProduct.GoodsBasic.CategoryTree = types.CategoryTree{
		Level:        category.Level,
		CateType:     category.CateType,
		CatID:        category.CatID,
		Cate1ID:      category.Cate1ID,
		Cate1Name:    category.Cate1Name,
		Cate2ID:      category.Cate2ID,
		Cate2Name:    category.Cate2Name,
		Cate3ID:      category.Cate3ID,
		Cate3Name:    category.Cate3Name,
		CateNameList: category.CateNameList,
	}

	// 如果有更深层级的分类，也要设置
	if category.Cate4ID != nil {
		ctx.TemuProduct.GoodsBasic.CatIDs = append(ctx.TemuProduct.GoodsBasic.CatIDs, *category.Cate4ID)
		ctx.TemuProduct.GoodsBasic.CategoryTree.Cate4ID = *category.Cate4ID
		if category.Cate4Name != nil {
			ctx.TemuProduct.GoodsBasic.CategoryTree.Cate4Name = *category.Cate4Name
		}
	}

	if category.Cate5ID != nil {
		ctx.TemuProduct.GoodsBasic.CatIDs = append(ctx.TemuProduct.GoodsBasic.CatIDs, *category.Cate5ID)
		ctx.TemuProduct.GoodsBasic.CategoryTree.Cate5ID = *category.Cate5ID
		if category.Cate5Name != nil {
			ctx.TemuProduct.GoodsBasic.CategoryTree.Cate5Name = *category.Cate5Name
		}
	}

	h.logger.Infof("分类推荐成功: CatID=%d, 分类路径=%v",
		category.CatID, category.CateNameList)
	return nil
}

// setDefaultCategory 设置默认分类
func (h *CategoryRecommendHandler) setDefaultCategory(ctx *pipeline.TaskContext) {
	h.logger.Info("设置默认分类")

	// 设置默认分类（通用商品分类）
	ctx.TemuProduct.GoodsBasic.CatID = 30469
	ctx.TemuProduct.GoodsBasic.CatIDs = []int{1001, 2001, 3001}

	ctx.TemuProduct.GoodsBasic.CategoryTree = types.CategoryTree{
		Level:        3,
		CateType:     1,
		CatID:        30469,
		Cate1ID:      1001,
		Cate1Name:    "通用商品",
		Cate2ID:      2001,
		Cate2Name:    "其他商品",
		Cate3ID:      3001,
		Cate3Name:    "未分类商品",
		CateNameList: []string{"通用商品", "其他商品", "未分类商品"},
	}
}
