package product

import (
	"fmt"
	"task-processor/internal/core/logger"
	"task-processor/internal/pipeline"
	temuapi "task-processor/internal/platforms/temu/api"
	temucontext "task-processor/internal/platforms/temu/context"

	"github.com/sirupsen/logrus"
)

// CommitDetailHandler 提交详情查询处理器
type CommitDetailHandler struct {
	logger *logrus.Entry
}

// NewCommitDetailHandler 创建新的提交详情查询处理器
func NewCommitDetailHandler() *CommitDetailHandler {
	return &CommitDetailHandler{
		logger: logger.GetGlobalLogger("temu.handlers.commit_detail").WithField("handler", "CommitDetailHandler"),
	}
}

// Name 返回处理器名称
func (h *CommitDetailHandler) Name() string {
	return "提交详情查询处理器"
}

// Handle 处理任务（兼容pipeline.Handler接口）
func (h *CommitDetailHandler) Handle(ctx pipeline.TaskContext) error {
	// 类型断言为强类型上下文
	temuCtx, ok := ctx.(*temucontext.TemuTaskContext)
	if !ok {
		return fmt.Errorf("上下文类型错误，期望TemuTaskContext")
	}
	return h.HandleTemu(temuCtx)
}

// HandleTemu 处理任务（强类型上下文）
func (h *CommitDetailHandler) HandleTemu(temuCtx *temucontext.TemuTaskContext) error {
	h.logger.Info("开始查询提交详情")

	// 获取任务信息
	task := temuCtx.GetTask()
	if task == nil {
		return fmt.Errorf("任务信息为空")
	}

	// 从上下文获取TEMU产品信息
	if temuCtx.TemuProduct == nil {
		return fmt.Errorf("TEMU产品信息为空")
	}

	temuProduct := temuCtx.TemuProduct

	// 查询提交详情
	err := h.queryCommitDetail(temuCtx, temuProduct)
	if err != nil {
		h.logger.WithError(err).Error("查询提交详情失败")
		return fmt.Errorf("查询提交详情失败: %w", err)
	}

	h.logger.Info("提交详情查询完成")
	return nil
}

// validateCommitInfo 验证提交信息
func (h *CommitDetailHandler) validateCommitInfo(temuProduct *temuapi.Product) error {
	basic := &temuProduct.GoodsBasic

	if basic.ListingCommitID == "" {
		return fmt.Errorf("ListingCommitID不能为空")
	}

	if basic.GoodsCommitID == "" {
		return fmt.Errorf("GoodsCommitID不能为空")
	}

	if basic.GoodsID == "" {
		return fmt.Errorf("GoodsID不能为空")
	}

	if basic.ListingCommitVersion == "" {
		return fmt.Errorf("ListingCommitVersion不能为空")
	}

	h.logger.WithFields(logrus.Fields{
		"listingCommitID":      basic.ListingCommitID,
		"goodsCommitID":        basic.GoodsCommitID,
		"goodsID":              basic.GoodsID,
		"listingCommitVersion": basic.ListingCommitVersion,
	}).Info("提交信息验证通过")

	return nil
}

// queryCommitDetail 查询提交详情
func (h *CommitDetailHandler) queryCommitDetail(temuCtx *temucontext.TemuTaskContext, temuProduct *temuapi.Product) error {
	// 获取API客户端
	if temuCtx.APIClient == nil {
		return fmt.Errorf("API客户端未初始化")
	}

	basic := &temuProduct.GoodsBasic

	// 创建QueryAPI实例
	queryAPI := temuapi.NewQueryAPI(temuCtx.APIClient, h.logger)

	// 构造查询请求体
	request := &temuapi.CommitDetailRequest{
		ListingCommitID:      basic.ListingCommitID,
		GoodsCommitID:        basic.GoodsCommitID,
		GoodsID:              basic.GoodsID,
		ListingCommitVersion: basic.ListingCommitVersion,
		ClickType:            "8", // 默认点击类型
	}

	h.logger.WithFields(logrus.Fields{
		"listingCommitID":      request.ListingCommitID,
		"goodsCommitID":        request.GoodsCommitID,
		"goodsID":              request.GoodsID,
		"listingCommitVersion": request.ListingCommitVersion,
	}).Info("发送提交详情查询请求")

	// 发送API请求
	response, err := queryAPI.QueryCommitDetail(request)
	if err != nil {
		h.logger.WithError(err).Error("查询提交详情API调用失败")
		return fmt.Errorf("查询提交详情API调用失败: %w", err)
	}

	h.logger.WithFields(logrus.Fields{
		"success":              response.Success,
		"listingCommitID":      request.ListingCommitID,
		"goodsCommitID":        request.GoodsCommitID,
		"goodsID":              request.GoodsID,
		"listingCommitVersion": request.ListingCommitVersion,
	}).Info("提交详情查询API响应")

	// 解析并更新产品数据
	if response.Result != nil {
		err := h.updateProductFromCommitDetail(temuProduct, response.Result)
		if err != nil {
			h.logger.WithError(err).Warn("更新产品数据失败，但继续执行")
		}

		h.logger.Info("提交详情数据已存储到上下文")
	}

	h.logger.Info("提交详情查询成功")
	return nil
}

// updateProductFromCommitDetail 从提交详情更新产品数据
func (h *CommitDetailHandler) updateProductFromCommitDetail(temuProduct *temuapi.Product, result *temuapi.CommitDetailResult) error {
	if result.GoodsBasic == nil {
		return fmt.Errorf("商品基础信息为空")
	}

	basic := result.GoodsBasic

	// 更新商品基础信息
	if basic.GoodsName != "" {
		temuProduct.GoodsBasic.GoodsName = basic.GoodsName
		h.logger.Infof("更新商品名称: %s", basic.GoodsName)
	}

	if basic.CatID > 0 {
		temuProduct.GoodsBasic.CatID = basic.CatID
		h.logger.Infof("更新分类ID: %d", basic.CatID)
	}

	if len(basic.CatIDs) > 0 {
		temuProduct.GoodsBasic.CatIDs = basic.CatIDs
		h.logger.Infof("更新分类ID列表: %v", basic.CatIDs)
	}

	// 更新分类树信息
	if basic.CategoryTree != nil {
		h.updateCategoryTree(temuProduct, basic.CategoryTree)
	}

	// 更新分类免责声明
	if basic.CategoryDisclaimer != nil && len(basic.CategoryDisclaimer.PromptList) > 0 {
		temuProduct.GoodsBasic.CategoryDisclaimer.PromptList = basic.CategoryDisclaimer.PromptList
		h.logger.Infof("更新分类免责声明: %d条", len(basic.CategoryDisclaimer.PromptList))
	}

	// 更新商品类型信息
	temuProduct.GoodsBasic.GoodsType = basic.GoodsType
	temuProduct.GoodsBasic.IsClothes = basic.IsClothes
	temuProduct.GoodsBasic.IsBooks = basic.IsBooks
	temuProduct.GoodsBasic.Customized = basic.Customized
	temuProduct.GoodsBasic.SecondHand = basic.SecondHand
	temuProduct.GoodsBasic.MadeToOrder = basic.MadeToOrder

	// 更新外部商品编号
	if basic.OutGoodsSn != "" {
		temuProduct.GoodsBasic.OutGoodsSN = basic.OutGoodsSn
		h.logger.Infof("更新外部商品编号: %s", basic.OutGoodsSn)
	}

	// 更新销售信息
	if result.GoodsSaleInfo != nil {
		temuProduct.GoodsSaleInfo.GoodsPattern = result.GoodsSaleInfo.GoodsPattern
	}

	// 更新额外信息
	if result.Extra != nil {
		temuProduct.Extra.Tab = result.Extra.Tab
		temuProduct.Extra.MinSkuImageSize = result.Extra.MinSkuImageSize
		temuProduct.Extra.MaxSkuImageSize = result.Extra.MaxSkuImageSize
	}

	// 更新支持标志
	temuProduct.CanSave = &result.CanSave
	temuProduct.SupportMaxRetailPrice = &result.SupportMaxRetailPrice
	temuProduct.PlatformExpressBill = &result.PlatformExpressBill

	h.logger.WithFields(logrus.Fields{
		"goodsName":             temuProduct.GoodsBasic.GoodsName,
		"catID":                 temuProduct.GoodsBasic.CatID,
		"goodsType":             temuProduct.GoodsBasic.GoodsType,
		"isClothes":             temuProduct.GoodsBasic.IsClothes,
		"canSave":               temuProduct.CanSave,
		"supportMaxRetailPrice": temuProduct.SupportMaxRetailPrice,
	}).Info("产品数据更新完成")

	return nil
}

// updateCategoryTree 更新分类树信息
func (h *CommitDetailHandler) updateCategoryTree(temuProduct *temuapi.Product, tree *temuapi.CommitDetailCategoryTree) {
	// 更新分类层级信息
	temuProduct.GoodsBasic.CategoryTree.Level = tree.Level
	temuProduct.GoodsBasic.CategoryTree.CateType = tree.CateType
	temuProduct.GoodsBasic.CategoryTree.CatID = tree.CatID

	// 更新各级分类信息
	if tree.Cate1ID > 0 {
		temuProduct.GoodsBasic.CategoryTree.Cate1ID = tree.Cate1ID
		temuProduct.GoodsBasic.CategoryTree.Cate1Name = tree.Cate1Name
	}
	if tree.Cate2ID > 0 {
		temuProduct.GoodsBasic.CategoryTree.Cate2ID = tree.Cate2ID
		temuProduct.GoodsBasic.CategoryTree.Cate2Name = tree.Cate2Name
	}
	if tree.Cate3ID > 0 {
		temuProduct.GoodsBasic.CategoryTree.Cate3ID = tree.Cate3ID
		temuProduct.GoodsBasic.CategoryTree.Cate3Name = tree.Cate3Name
	}
	if tree.Cate4ID > 0 {
		temuProduct.GoodsBasic.CategoryTree.Cate4ID = tree.Cate4ID
		temuProduct.GoodsBasic.CategoryTree.Cate4Name = tree.Cate4Name
	}
	if tree.Cate5ID > 0 {
		temuProduct.GoodsBasic.CategoryTree.Cate5ID = tree.Cate5ID
		temuProduct.GoodsBasic.CategoryTree.Cate5Name = tree.Cate5Name
	}

	// 更新分类名称列表
	if len(tree.CateNameList) > 0 {
		temuProduct.GoodsBasic.CategoryTree.CateNameList = tree.CateNameList
	}

	h.logger.WithFields(logrus.Fields{
		"level":        tree.Level,
		"cateType":     tree.CateType,
		"cateNameList": tree.CateNameList,
	}).Info("分类树信息更新完成")
}

// GetCommitDetailFromContext 从强类型上下文中获取提交详情
func GetCommitDetailFromContext(temuCtx *temucontext.TemuTaskContext) (any, bool) {
	// 优先从强类型上下文字段获取
	if temuCtx.CommitDetail != nil {
		return temuCtx.CommitDetail, true
	}

	// 兼容性：从基础上下文的GetData方法获取
	if data, exists := temuCtx.DefaultTaskContext.GetData("commit_detail"); exists {
		return data, true
	}
	return nil, false
}
