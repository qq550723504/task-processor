// Package query 提供TEMU平台查询相关的API和数据结构
package query

import (
	"fmt"
	"strings"
	"task-processor/internal/platforms/temu/api/client"
	"task-processor/internal/platforms/temu/types"

	"github.com/sirupsen/logrus"
)

// --- 数据模型 ---

// TextCheckRequest 文本检查请求
type TextCheckRequest struct {
	Content string `json:"content"`
	Type    int    `json:"type"`
}

// TextCheckResponse 文本检查响应
type TextCheckResponse struct {
	Success   bool `json:"success"`
	ErrorCode int  `json:"error_code"`
	Result    struct {
		Success bool `json:"success"`
	} `json:"result"`
}

// TemplateQueryRequest 模板查询请求
type TemplateQueryRequest struct {
	CatID int `json:"cat_id"`
}

// TemplateQueryResponse 模板查询响应
type TemplateQueryResponse struct {
	Success   bool `json:"success"`
	ErrorCode int  `json:"error_code"`
	Result    struct {
		Templates []TemplateInfo `json:"templates"`
	} `json:"result"`
}

// TemplateInfo 模板信息
type TemplateInfo struct {
	TemplateID   int    `json:"template_id"`
	TemplateName string `json:"template_name"`
	CategoryID   int    `json:"category_id"`
}

// SpecQueryRequest 规格查询请求
type SpecQueryRequest struct {
	GoodsID       string   `json:"goods_id"`
	ChildSpecName string   `json:"child_spec_name"`
	ParentSpecID  string   `json:"parent_spec_id"`
	ExistSpecList []string `json:"exist_spec_list"`
}

// SpecQueryResponse 规格查询响应
type SpecQueryResponse struct {
	Success   bool             `json:"success"`
	ErrorCode int              `json:"error_code"`
	Result    *SpecQueryResult `json:"result"`
}

// SpecQueryResult 规格查询结果
type SpecQueryResult struct {
	SpecID string `json:"spec_id"`
}

// SkuSnCheckRequest SKU编码检查请求
type SkuSnCheckRequest struct {
	GoodsID   string         `json:"goods_id"`
	OutSnList []OutSkuSnItem `json:"out_sn_list"`
}

// OutSkuSnItem SKU编码项
type OutSkuSnItem struct {
	OutSkuSn string `json:"out_sku_sn"`
}

// SkuSnCheckResponse SKU编码检查响应
type SkuSnCheckResponse struct {
	Success   bool                   `json:"success"`
	ErrorCode int                    `json:"error_code"`
	Result    *OutGoodsSnCheckResult `json:"result,omitempty"`
	Message   string                 `json:"error_msg,omitempty"`
}

// OutGoodsSnCheckResult SKU编码检查结果
type OutGoodsSnCheckResult struct {
	FailList []OutSkuSnFailItem `json:"fail_list"`
}

// OutSkuSnFailItem 失败的SKU编码项
type OutSkuSnFailItem struct {
	OutSkuSn    string `json:"out_sku_sn"`
	UsedGoodsID string `json:"used_goods_id"`
	UsedSkuID   string `json:"used_sku_id"`
	FailReason  string `json:"fail_reason"`
}

// CostTemplateRequest 成本模板查询请求
type CostTemplateRequest struct {
	ListingCommitID      string `json:"listing_commit_id"`
	GoodsCommitID        string `json:"goods_commit_id"`
	GoodsID              string `json:"goods_id"`
	CatID                int    `json:"cat_id"`
	ListingCommitVersion string `json:"listing_commit_version"`
	ClickType            string `json:"click_type"`
	QueryAll             bool   `json:"query_all"`
}

// CostTemplateResponse 成本模板查询响应
type CostTemplateResponse struct {
	Success   bool                `json:"success"`
	ErrorCode int                 `json:"error_code"`
	Result    *CostTemplateResult `json:"result,omitempty"`
	Message   string              `json:"message,omitempty"`
}

// CostTemplateResult 成本模板结果数据
type CostTemplateResult struct {
	CostTemplateList []CostTemplateItem `json:"cost_template_list"`
	CostTemplateURL  string             `json:"cost_template_url"`
}

// CostTemplateItem 成本模板项
type CostTemplateItem struct {
	CostTemplateID  string `json:"cost_template_id"`
	TemplateName    string `json:"template_name"`
	Disabled        bool   `json:"disabled"`
	DefaultTemplate bool   `json:"default_template"`
}

// CommitDetailRequest 提交详情查询请求
type CommitDetailRequest struct {
	ListingCommitID      string `json:"listing_commit_id"`
	GoodsCommitID        string `json:"goods_commit_id"`
	GoodsID              string `json:"goods_id"`
	ListingCommitVersion string `json:"listing_commit_version"`
	ClickType            string `json:"click_type"`
}

// CommitDetailResponse 提交详情查询响应
type CommitDetailResponse struct {
	Success   bool                `json:"success"`
	ErrorCode int                 `json:"error_code,omitempty"`
	Message   string              `json:"message,omitempty"`
	Result    *CommitDetailResult `json:"result,omitempty"`
}

// CommitDetailResult 提交详情结果数据
type CommitDetailResult struct {
	GoodsBasic            *CommitDetailGoodsBasic    `json:"goods_basic,omitempty"`
	GoodsSaleInfo         *CommitDetailGoodsSaleInfo `json:"goods_sale_info,omitempty"`
	GoodsServicePromise   map[string]interface{}     `json:"goods_service_promise,omitempty"`
	GoodsExtensionInfo    map[string]interface{}     `json:"goods_extension_info,omitempty"`
	Extra                 *CommitDetailExtra         `json:"extra,omitempty"`
	CanSave               bool                       `json:"can_save"`
	SupportMaxRetailPrice bool                       `json:"support_max_retail_price"`
	PlatformExpressBill   bool                       `json:"platform_express_bill"`
}

// CommitDetailGoodsBasic 商品基础信息
type CommitDetailGoodsBasic struct {
	GoodsID              string                          `json:"goods_id"`
	ListingCommitID      string                          `json:"listing_commit_id"`
	ListingCommitVersion string                          `json:"listing_commit_version"`
	GoodsName            string                          `json:"goods_name"`
	GoodsCommitID        string                          `json:"goods_commit_id"`
	CatID                int                             `json:"cat_id"`
	CatIDs               []int                           `json:"cat_ids"`
	CategoryTree         *CommitDetailCategoryTree       `json:"category_tree,omitempty"`
	CategoryDisclaimer   *CommitDetailCategoryDisclaimer `json:"category_disclaimer,omitempty"`
	GoodsType            int                             `json:"goods_type"`
	IsClothes            bool                            `json:"is_clothes"`
	IsBooks              bool                            `json:"is_books"`
	Customized           bool                            `json:"customized"`
	SecondHand           bool                            `json:"second_hand"`
	MadeToOrder          bool                            `json:"made_to_order"`
	OutGoodsSn           string                          `json:"out_goods_sn"`
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

// CommitDetailExtra 额外信息
type CommitDetailExtra struct {
	Tab             int `json:"tab"`
	MinSkuImageSize int `json:"min_sku_image_size"`
	MaxSkuImageSize int `json:"max_sku_image_size"`
}

// SkuQueryRequest SKU查询请求
type SkuQueryRequest struct {
	CommitID             string `json:"commit_id"`
	GoodsID              string `json:"goods_id"`
	SourceTypeOfSkuQuery int    `json:"source_type_of_sku_query"`
	Source               int    `json:"source"`
}

// SkuQueryResponse SKU查询响应
type SkuQueryResponse struct {
	Success   bool `json:"success"`
	ErrorCode int  `json:"error_code"`
	Result    struct {
		Total   int              `json:"total"`
		SkuList []SkuQueryResult `json:"sku_list"`
	} `json:"result"`
}

// SkuQueryResult SKU查询结果项
type SkuQueryResult struct {
	GoodsName           string                 `json:"goods_name"`
	SpecName            string                 `json:"spec_name"`
	SpecList            []SkuSpecInfo          `json:"spec_list"`
	ThumbURL            string                 `json:"thumb_url"`
	GoodsID             string                 `json:"goods_id"`
	GoodsCommitID       string                 `json:"goods_commit_id"`
	ListingCommitID     string                 `json:"listing_commit_id"`
	MallID              string                 `json:"mall_id"`
	SkuID               string                 `json:"sku_id"`
	SkuSN               string                 `json:"sku_sn"`
	Stock               int                    `json:"stock"`
	Price               float64                `json:"price"`
	SupplierPrice       float64                `json:"supplier_price"`
	Currency            string                 `json:"currency"`
	SkcID               string                 `json:"skc_id"`
	OutGoodsSN          string                 `json:"out_goods_sn"`
	ShippingMode        int                    `json:"shipping_mode"`
	OrdinaryStock       int                    `json:"ordinary_stock"`
	CheckPriceAuditInfo map[string]interface{} `json:"check_price_audit_info"`
}

// SkuSpecInfo SKU规格信息
type SkuSpecInfo struct {
	SpecID         string `json:"spec_id"`
	SpecName       string `json:"spec_name"`
	ParentSpecName string `json:"parent_spec_name"`
}

// --- API ---

// API 查询API管理器
type API struct {
	client client.APIClientInterface
	logger *logrus.Entry
}

// NewAPI 创建查询API管理器
func NewAPI(c client.APIClientInterface, logger *logrus.Entry) *API {
	return &API{client: c, logger: logger}
}

func (a *API) defaultHeaders() map[string]string {
	return map[string]string{
		"accept":             "application/json, text/plain, */*",
		"accept-language":    "zh-CN,zh;q=0.9",
		"sec-fetch-dest":     "empty",
		"sec-fetch-mode":     "cors",
		"sec-fetch-site":     "same-origin",
		"x-document-referer": "https://seller.temu.com/product-add.html?is_back=1",
	}
}

// CheckText 检查文本内容
func (a *API) CheckText(request *TextCheckRequest) (*TextCheckResponse, error) {
	if request == nil || request.Content == "" {
		return nil, fmt.Errorf("检查文本不能为空")
	}

	req := map[string]any{
		"method":  "POST",
		"url":     "/mms/marigold/query/commit/check_text",
		"headers": a.defaultHeaders(),
		"body":    request,
	}

	var result TextCheckResponse
	if err := a.client.SendTEMURequest(req, &result); err != nil {
		return nil, fmt.Errorf("文本检查请求失败: %w", err)
	}
	if !result.Success {
		return nil, fmt.Errorf("文本检查失败: errorCode=%d", result.ErrorCode)
	}
	if !result.Result.Success {
		return nil, fmt.Errorf("文本检查未通过")
	}
	return &result, nil
}

// QueryTemplate 查询模板信息
func (a *API) QueryTemplate(request *TemplateQueryRequest) (*TemplateQueryResponse, error) {
	if request == nil {
		return nil, fmt.Errorf("查询请求不能为空")
	}

	req := map[string]any{
		"method": "POST",
		"url":    "/mms/marigold/query/commit/query_template",
		"body":   request,
	}

	var result TemplateQueryResponse
	if err := a.client.SendTEMURequest(req, &result); err != nil {
		return nil, fmt.Errorf("模板查询请求失败: %w", err)
	}
	if !result.Success {
		return nil, fmt.Errorf("模板查询失败: errorCode=%d", result.ErrorCode)
	}
	return &result, nil
}

// QueryTemplateAdvanced 查询模板信息（支持完整 types 结构）
func (a *API) QueryTemplateAdvanced(request *types.TemplateQueryRequest) (*types.TemplateQueryResponse, error) {
	if request == nil {
		return nil, fmt.Errorf("查询请求不能为空")
	}

	req := map[string]any{
		"method":  "POST",
		"url":     "/mms/marigold/query/commit/query_template",
		"headers": a.defaultHeaders(),
		"body":    request,
	}

	var result types.TemplateQueryResponse
	if err := a.client.SendTEMURequest(req, &result); err != nil {
		return nil, fmt.Errorf("模板查询请求失败: %w", err)
	}
	if !result.Success {
		return nil, fmt.Errorf("模板查询失败: errorCode=%d", result.ErrorCode)
	}
	return &result, nil
}

// QuerySpec 查询规格信息
func (a *API) QuerySpec(request *SpecQueryRequest) (*SpecQueryResponse, error) {
	if request == nil {
		return nil, fmt.Errorf("查询请求不能为空")
	}

	req := map[string]any{
		"method": "POST",
		"url":    "/mms/marigold/edit/commit/spec_query",
		"body":   request,
	}

	var result SpecQueryResponse
	if err := a.client.SendTEMURequest(req, &result); err != nil {
		return nil, fmt.Errorf("规格查询请求失败: %w", err)
	}
	if !result.Success {
		return nil, fmt.Errorf("规格查询失败: errorCode=%d", result.ErrorCode)
	}
	return &result, nil
}

// CheckSkuSn 检查SKU编码
func (a *API) CheckSkuSn(request *SkuSnCheckRequest) (*SkuSnCheckResponse, error) {
	if request == nil {
		return nil, fmt.Errorf("检查请求不能为空")
	}

	req := map[string]any{
		"method": "POST",
		"url":    "/mms/marigold/query/commit/out_sku_sn_batch_check",
		"body":   request,
	}

	var result SkuSnCheckResponse
	if err := a.client.SendTEMURequest(req, &result); err != nil {
		return nil, fmt.Errorf("SKU编码检查请求失败: %w", err)
	}
	if !result.Success {
		return nil, fmt.Errorf("SKU编码检查失败: errorCode=%d", result.ErrorCode)
	}
	return &result, nil
}

// QueryCostTemplate 查询成本模板
func (a *API) QueryCostTemplate(request *CostTemplateRequest) (*CostTemplateResponse, error) {
	if request == nil {
		return nil, fmt.Errorf("查询请求不能为空")
	}

	req := map[string]any{
		"method": "POST",
		"url":    "/mms/marigold/query/commit/query_cost_template",
		"body":   request,
	}

	var result CostTemplateResponse
	if err := a.client.SendTEMURequest(req, &result); err != nil {
		return nil, fmt.Errorf("成本模板查询请求失败: %w", err)
	}
	if !result.Success {
		return nil, fmt.Errorf("成本模板查询失败: errorCode=%d", result.ErrorCode)
	}
	return &result, nil
}

// QueryCommitDetail 查询提交详情
func (a *API) QueryCommitDetail(request *CommitDetailRequest) (*CommitDetailResponse, error) {
	if request == nil {
		return nil, fmt.Errorf("查询请求不能为空")
	}

	req := map[string]any{
		"method": "POST",
		"url":    "/mms/marigold/query/commit/query_commit_detail",
		"body":   request,
	}

	var result CommitDetailResponse
	if err := a.client.SendTEMURequest(req, &result); err != nil {
		return nil, fmt.Errorf("提交详情查询请求失败: %w", err)
	}
	if !result.Success {
		return nil, fmt.Errorf("提交详情查询失败: errorCode=%d", result.ErrorCode)
	}
	return &result, nil
}

// QuerySkuPriceAndStock 查询SKU价格与库存
func (a *API) QuerySkuPriceAndStock(commitID, goodsID string) (*SkuQueryResponse, error) {
	if commitID == "" || goodsID == "" {
		return nil, fmt.Errorf("commitID 和 goodsID 不能为空")
	}

	req := map[string]any{
		"method": "POST",
		"url":    "/mms/marigold/sku/query",
		"headers": map[string]string{
			"x-document-referer": "https://seller.temu.com/products.html",
		},
		"body": &SkuQueryRequest{
			CommitID:             commitID,
			GoodsID:              goodsID,
			SourceTypeOfSkuQuery: 1,
			Source:               0,
		},
	}

	var result SkuQueryResponse
	authManager := client.NewAuthManager(a.logger)
	if err := authManager.SendRequestWithAuth(a.client, req, &result); err != nil {
		return nil, fmt.Errorf("调用SKU查询API失败: %w", err)
	}
	if !result.Success {
		return nil, fmt.Errorf("API返回错误: error_code=%d", result.ErrorCode)
	}
	return &result, nil
}

func isTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "timeout") ||
		strings.Contains(s, "deadline exceeded") ||
		strings.Contains(s, "Client.Timeout exceeded")
}

// SkuQueryOptions SKU查询选项
type SkuQueryOptions struct {
	CommitID             string
	GoodsID              string
	SourceTypeOfSkuQuery int
	Source               int
}

// NewSkuQueryOptions 创建SKU查询选项
func NewSkuQueryOptions(commitID, goodsID string) SkuQueryOptions {
	return SkuQueryOptions{
		CommitID:             commitID,
		GoodsID:              goodsID,
		SourceTypeOfSkuQuery: 1,
		Source:               0,
	}
}

// QuerySkuPriceAndStockWithOptions 使用选项查询SKU价格与库存
func (a *API) QuerySkuPriceAndStockWithOptions(options SkuQueryOptions) (*SkuQueryResponse, error) {
	return a.QuerySkuPriceAndStock(options.CommitID, options.GoodsID)
}
