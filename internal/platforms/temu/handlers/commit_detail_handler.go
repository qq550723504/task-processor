package handlers

import (
	"fmt"
	"task-processor/internal/common/pipeline"

	"github.com/sirupsen/logrus"
)

// CommitDetailHandler 提交详情查询处理器
type CommitDetailHandler struct {
	logger *logrus.Entry
}

// CommitDetailRequest 提交详情查询请求结构体
type CommitDetailRequest struct {
	ListingCommitID      string `json:"listing_commit_id"`
	GoodsCommitID        string `json:"goods_commit_id"`
	GoodsID              string `json:"goods_id"`
	ListingCommitVersion string `json:"listing_commit_version"`
	ClickType            string `json:"click_type"`
}

// CommitDetailResponse 提交详情查询响应结构体
type CommitDetailResponse struct {
	Success   bool                `json:"success"`
	ErrorCode int                 `json:"error_code,omitempty"`
	Message   string              `json:"message,omitempty"`
	Result    *CommitDetailResult `json:"result,omitempty"`
}

// CommitDetailResult 提交详情结果数据
type CommitDetailResult struct {
	GoodsBasic            *CommitDetailGoodsBasic     `json:"goods_basic,omitempty"`
	GoodsSaleInfo         *CommitDetailGoodsSaleInfo  `json:"goods_sale_info,omitempty"`
	GoodsServicePromise   *CommitDetailServicePromise `json:"goods_service_promise,omitempty"`
	GoodsExtensionInfo    *CommitDetailExtensionInfo  `json:"goods_extension_info,omitempty"`
	Extra                 *CommitDetailExtra          `json:"extra,omitempty"`
	CanSave               bool                        `json:"can_save"`
	SupportMaxRetailPrice bool                        `json:"support_max_retail_price"`
	PlatformExpressBill   bool                        `json:"platform_express_bill"`
}

// CommitDetailGoodsBasic 商品基础信息
type CommitDetailGoodsBasic struct {
	GoodsID                 string                          `json:"goods_id"`
	ListingCommitID         string                          `json:"listing_commit_id"`
	ListingCommitVersion    string                          `json:"listing_commit_version"`
	GoodsName               string                          `json:"goods_name"`
	GoodsCreateTime         int64                           `json:"goods_create_time"`
	GoodsCommitID           string                          `json:"goods_commit_id"`
	Lang                    string                          `json:"lang"`
	AllowSite               []int                           `json:"allow_site"`
	CatID                   int                             `json:"cat_id"`
	CatIDs                  []int                           `json:"cat_ids"`
	CategoryTree            *CommitDetailCategoryTree       `json:"category_tree,omitempty"`
	CategoryDisclaimer      *CommitDetailCategoryDisclaimer `json:"category_disclaimer,omitempty"`
	GoodsType               int                             `json:"goods_type"`
	HdThumbURL              string                          `json:"hd_thumb_url"`
	GoodsGallery            map[string]interface{}          `json:"goods_gallery,omitempty"`
	IsOnSale                int                             `json:"is_on_sale"`
	CatType                 int                             `json:"cat_type"`
	IsClothes               bool                            `json:"is_clothes"`
	IsBooks                 bool                            `json:"is_books"`
	CanSkipRequiredProperty bool                            `json:"can_skip_required_property"`
	IsShop                  bool                            `json:"is_shop"`
	FromCopy                bool                            `json:"from_copy"`
	HasSubmitted            bool                            `json:"has_submitted"`
	Source                  int                             `json:"source"`
	OutGoodsSn              string                          `json:"out_goods_sn"`
	ListPriceRequired       bool                            `json:"list_price_required"`
	ListPriceDocuments      bool                            `json:"list_price_documents"`
	NeedAccessoryInfo       bool                            `json:"need_accessory_info"`
	AccessoryInfoRequired   bool                            `json:"accessory_info_required"`
	Customized              bool                            `json:"customized"`
	SecondHand              bool                            `json:"second_hand"`
	SupportCustomizedGoods  bool                            `json:"support_customized_goods"`
	RecommendURLPrice       bool                            `json:"recommend_url_price"`
	AgreeMaxRetailPrice     bool                            `json:"agree_max_retail_price"`
	CanEditSecondHand       bool                            `json:"can_edit_second_hand"`
	MadeToOrder             bool                            `json:"made_to_order"`
}

// CommitDetailCategoryTree 分类树结构
type CommitDetailCategoryTree struct {
	Level        int      `json:"level"`
	CateType     int      `json:"cate_type"`
	CatID        int      `json:"cat_id"`
	Cate1ID      int      `json:"cate1_id"`
	Cate1Name    string   `json:"cate1_name"`
	Cate2ID      int      `json:"cate2_id"`
	Cate2Name    string   `json:"cate2_name"`
	Cate3ID      int      `json:"cate3_id"`
	Cate3Name    string   `json:"cate3_name"`
	Cate4ID      int      `json:"cate4_id"`
	Cate4Name    string   `json:"cate4_name"`
	Cate5ID      int      `json:"cate5_id"`
	Cate5Name    string   `json:"cate5_name"`
	Cate6ID      int      `json:"cate6_id"`
	Cate6Name    string   `json:"cate6_name"`
	CateNameList []string `json:"cate_name_list"`
}

// CommitDetailCategoryDisclaimer 分类免责声明
type CommitDetailCategoryDisclaimer struct {
	PromptList []string `json:"prompt_list"`
}

// CommitDetailGoodsSaleInfo 商品销售信息
type CommitDetailGoodsSaleInfo struct {
	GoodsPattern int `json:"goods_pattern"`
}

// CommitDetailServicePromise 服务承诺
type CommitDetailServicePromise struct {
	// 根据实际数据结构添加字段
}

// CommitDetailExtensionInfo 扩展信息
type CommitDetailExtensionInfo struct {
	// 根据实际数据结构添加字段
}

// CommitDetailExtra 额外信息
type CommitDetailExtra struct {
	Tab             int `json:"tab"`
	MinSkuImageSize int `json:"min_sku_image_size"`
	MaxSkuImageSize int `json:"max_sku_image_size"`
}

// NewCommitDetailHandler 创建新的提交详情查询处理器
func NewCommitDetailHandler() *CommitDetailHandler {
	return &CommitDetailHandler{
		logger: logrus.WithField("handler", "CommitDetailHandler"),
	}
}

// Name 返回处理器名称
func (h *CommitDetailHandler) Name() string {
	return "提交详情查询处理器"
}

// Handle 处理任务
func (h *CommitDetailHandler) Handle(ctx *pipeline.TaskContext) error {
	h.logger.Info("开始查询提交详情")

	// 检查任务上下文中的必要数据
	if ctx.Task == nil {
		return fmt.Errorf("任务信息为空")
	}

	if ctx.TemuProduct == nil {
		return fmt.Errorf("TEMU产品信息为空")
	}

	// 查询提交详情
	err := h.queryCommitDetail(ctx)
	if err != nil {
		h.logger.WithError(err).Error("查询提交详情失败")
		return fmt.Errorf("查询提交详情失败: %w", err)
	}

	h.logger.Info("提交详情查询完成")
	return nil
}

// validateCommitInfo 验证提交信息
func (h *CommitDetailHandler) validateCommitInfo(ctx *pipeline.TaskContext) error {
	basic := ctx.TemuProduct.GoodsBasic

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
func (h *CommitDetailHandler) queryCommitDetail(ctx *pipeline.TaskContext) error {
	// 检查API客户端
	if ctx.APIClient == nil {
		return fmt.Errorf("API客户端未初始化")
	}

	basic := ctx.TemuProduct.GoodsBasic

	// 构造查询请求体
	requestBody := CommitDetailRequest{
		ListingCommitID:      basic.ListingCommitID,
		GoodsCommitID:        basic.GoodsCommitID,
		GoodsID:              basic.GoodsID,
		ListingCommitVersion: basic.ListingCommitVersion,
		ClickType:            "8", // 默认点击类型
	}

	h.logger.WithFields(logrus.Fields{
		"listingCommitID":      requestBody.ListingCommitID,
		"goodsCommitID":        requestBody.GoodsCommitID,
		"goodsID":              requestBody.GoodsID,
		"listingCommitVersion": requestBody.ListingCommitVersion,
	}).Info("发送提交详情查询请求")

	// 构造API请求
	apiReq := map[string]interface{}{
		"method": "POST",
		"url":    "/mms/marigold/query/commit/query_commit_detail",
		"headers": map[string]string{
			"accept":             "application/json, text/plain, */*",
			"accept-language":    "zh-CN,zh;q=0.9,en;q=0.8,en-GB;q=0.7,en-US;q=0.6",
			"content-type":       "application/json;charset=UTF-8",
			"priority":           "u=1, i",
			"sec-ch-ua":          "\"Microsoft Edge\";v=\"141\", \"Not?A_Brand\";v=\"8\", \"Chromium\";v=\"141\"",
			"sec-ch-ua-mobile":   "?0",
			"sec-ch-ua-platform": "\"Windows\"",
			"sec-fetch-dest":     "empty",
			"sec-fetch-mode":     "cors",
			"sec-fetch-site":     "same-origin",
		},
		"body": requestBody,
	}

	// 发送API请求
	response := &CommitDetailResponse{}
	err := ctx.APIClient.SendTEMURequest(apiReq, response)
	if err != nil {
		h.logger.WithError(err).Error("查询提交详情API调用失败")
		return fmt.Errorf("查询提交详情API调用失败: %w", err)
	}

	h.logger.WithFields(logrus.Fields{
		"success":              response.Success,
		"listingCommitID":      requestBody.ListingCommitID,
		"goodsCommitID":        requestBody.GoodsCommitID,
		"goodsID":              requestBody.GoodsID,
		"listingCommitVersion": requestBody.ListingCommitVersion,
	}).Info("提交详情查询API响应")

	if !response.Success {
		errorMsg := "查询提交详情失败，API返回失败状态"
		if response.Message != "" {
			errorMsg = fmt.Sprintf("%s: %s", errorMsg, response.Message)
		}
		if response.ErrorCode != 0 {
			errorMsg = fmt.Sprintf("%s (错误码: %d)", errorMsg, response.ErrorCode)
		}
		return fmt.Errorf("%s", errorMsg)
	}

	// 解析并更新产品数据
	if response.Result != nil {
		err := h.updateProductFromCommitDetail(ctx, response.Result)
		if err != nil {
			h.logger.WithError(err).Warn("更新产品数据失败，但继续执行")
		}

		h.logger.Info("提交详情数据已存储到上下文")
	}

	h.logger.Info("提交详情查询成功")
	return nil
}

// updateProductFromCommitDetail 从提交详情更新产品数据
func (h *CommitDetailHandler) updateProductFromCommitDetail(ctx *pipeline.TaskContext, result *CommitDetailResult) error {
	if result.GoodsBasic == nil {
		return fmt.Errorf("商品基础信息为空")
	}

	basic := result.GoodsBasic

	// 更新商品基础信息
	if basic.GoodsName != "" {
		ctx.TemuProduct.GoodsBasic.GoodsName = basic.GoodsName
		h.logger.Infof("更新商品名称: %s", basic.GoodsName)
	}

	if basic.CatID > 0 {
		ctx.TemuProduct.GoodsBasic.CatID = basic.CatID
		h.logger.Infof("更新分类ID: %d", basic.CatID)
	}

	if len(basic.CatIDs) > 0 {
		ctx.TemuProduct.GoodsBasic.CatIDs = basic.CatIDs
		h.logger.Infof("更新分类ID列表: %v", basic.CatIDs)
	}

	// 更新分类树信息
	if basic.CategoryTree != nil {
		h.updateCategoryTree(ctx, basic.CategoryTree)
	}

	// 更新分类免责声明
	if basic.CategoryDisclaimer != nil && len(basic.CategoryDisclaimer.PromptList) > 0 {
		ctx.TemuProduct.GoodsBasic.CategoryDisclaimer.PromptList = basic.CategoryDisclaimer.PromptList
		h.logger.Infof("更新分类免责声明: %d条", len(basic.CategoryDisclaimer.PromptList))
	}

	// 更新商品类型信息
	ctx.TemuProduct.GoodsBasic.GoodsType = basic.GoodsType
	ctx.TemuProduct.GoodsBasic.IsClothes = basic.IsClothes
	ctx.TemuProduct.GoodsBasic.IsBooks = basic.IsBooks
	ctx.TemuProduct.GoodsBasic.Customized = basic.Customized
	ctx.TemuProduct.GoodsBasic.SecondHand = basic.SecondHand
	ctx.TemuProduct.GoodsBasic.MadeToOrder = basic.MadeToOrder

	// 更新外部商品编号
	if basic.OutGoodsSn != "" {
		ctx.TemuProduct.GoodsBasic.OutGoodsSN = basic.OutGoodsSn
		h.logger.Infof("更新外部商品编号: %s", basic.OutGoodsSn)
	}

	// 更新销售信息
	if result.GoodsSaleInfo != nil {
		ctx.TemuProduct.GoodsSaleInfo.GoodsPattern = result.GoodsSaleInfo.GoodsPattern
	}

	// 更新额外信息
	if result.Extra != nil {
		ctx.TemuProduct.Extra.Tab = result.Extra.Tab
		ctx.TemuProduct.Extra.MinSkuImageSize = result.Extra.MinSkuImageSize
		ctx.TemuProduct.Extra.MaxSkuImageSize = result.Extra.MaxSkuImageSize
	}

	// 更新支持标志
	ctx.TemuProduct.CanSave = &result.CanSave
	ctx.TemuProduct.SupportMaxRetailPrice = &result.SupportMaxRetailPrice
	ctx.TemuProduct.PlatformExpressBill = &result.PlatformExpressBill

	h.logger.WithFields(logrus.Fields{
		"goodsName":             ctx.TemuProduct.GoodsBasic.GoodsName,
		"catID":                 ctx.TemuProduct.GoodsBasic.CatID,
		"goodsType":             ctx.TemuProduct.GoodsBasic.GoodsType,
		"isClothes":             ctx.TemuProduct.GoodsBasic.IsClothes,
		"canSave":               ctx.TemuProduct.CanSave,
		"supportMaxRetailPrice": ctx.TemuProduct.SupportMaxRetailPrice,
	}).Info("产品数据更新完成")

	return nil
}

// updateCategoryTree 更新分类树信息
func (h *CommitDetailHandler) updateCategoryTree(ctx *pipeline.TaskContext, tree *CommitDetailCategoryTree) {
	// 更新分类层级信息
	ctx.TemuProduct.GoodsBasic.CategoryTree.Level = tree.Level
	ctx.TemuProduct.GoodsBasic.CategoryTree.CateType = tree.CateType
	ctx.TemuProduct.GoodsBasic.CategoryTree.CatID = tree.CatID

	// 更新各级分类信息
	if tree.Cate1ID > 0 {
		ctx.TemuProduct.GoodsBasic.CategoryTree.Cate1ID = tree.Cate1ID
		ctx.TemuProduct.GoodsBasic.CategoryTree.Cate1Name = tree.Cate1Name
	}
	if tree.Cate2ID > 0 {
		ctx.TemuProduct.GoodsBasic.CategoryTree.Cate2ID = tree.Cate2ID
		ctx.TemuProduct.GoodsBasic.CategoryTree.Cate2Name = tree.Cate2Name
	}
	if tree.Cate3ID > 0 {
		ctx.TemuProduct.GoodsBasic.CategoryTree.Cate3ID = tree.Cate3ID
		ctx.TemuProduct.GoodsBasic.CategoryTree.Cate3Name = tree.Cate3Name
	}
	if tree.Cate4ID > 0 {
		ctx.TemuProduct.GoodsBasic.CategoryTree.Cate4ID = tree.Cate4ID
		ctx.TemuProduct.GoodsBasic.CategoryTree.Cate4Name = tree.Cate4Name
	}
	if tree.Cate5ID > 0 {
		ctx.TemuProduct.GoodsBasic.CategoryTree.Cate5ID = tree.Cate5ID
		ctx.TemuProduct.GoodsBasic.CategoryTree.Cate5Name = tree.Cate5Name
	}

	// 更新分类名称列表
	if len(tree.CateNameList) > 0 {
		ctx.TemuProduct.GoodsBasic.CategoryTree.CateNameList = tree.CateNameList
	}

	h.logger.WithFields(logrus.Fields{
		"level":        tree.Level,
		"cateType":     tree.CateType,
		"cateNameList": tree.CateNameList,
	}).Info("分类树信息更新完成")
}

// GetCommitDetailFromContext 从上下文中获取提交详情
func GetCommitDetailFromContext(ctx *pipeline.TaskContext) (interface{}, bool) {
	if data, exists := ctx.GetData("commit_detail"); exists {
		return data, true
	}
	return nil, false
}
