// Package inventory 提供TEMU平台库存、上下架相关的API和数据结构
package inventory

import (
	"fmt"
	"task-processor/internal/temu/api/client"
	"task-processor/internal/temu/api/types"

	"github.com/sirupsen/logrus"
)

// --- 库存数据模型 ---

// StockEditRequest 库存修改请求
type StockEditRequest struct {
	GoodsID            string           `json:"goods_id"`
	SkuStockChangeList []SkuStockChange `json:"sku_stock_change_list"`
}

// SkuStockChange SKU库存变更信息
type SkuStockChange struct {
	SkuID                 string `json:"sku_id"`
	CurrentShippingMode   int    `json:"current_shipping_mode"`
	CurrentStockAvailable int    `json:"current_stock_available"`
	StockDiff             int    `json:"stock_diff"`
}

// StockEditResponse 库存修改响应
type StockEditResponse struct {
	Success   bool            `json:"success"`
	ErrorCode int             `json:"error_code"`
	Result    StockEditResult `json:"result"`
}

// StockEditResult 库存修改结果
type StockEditResult struct {
	Result bool `json:"result"`
}

// --- 上下架数据模型 ---

// OfflineRequest 下架请求
type OfflineRequest struct {
	GoodsID string   `json:"goods_id"`
	SkuIDs  []string `json:"sku_ids"`
}

// OfflineResponse 下架响应
type OfflineResponse struct {
	Success   bool          `json:"success"`
	ErrorCode int           `json:"error_code"`
	Result    OfflineResult `json:"result"`
}

// OfflineResult 下架结果
type OfflineResult struct {
	Result bool `json:"result"`
}

// OnlineRequest 上架请求
type OnlineRequest struct {
	GoodsID string   `json:"goods_id"`
	SkuIDs  []string `json:"sku_ids"`
}

// OnlineResponse 上架响应
type OnlineResponse struct {
	Success   bool         `json:"success"`
	ErrorCode int          `json:"error_code"`
	Result    OnlineResult `json:"result"`
}

// OnlineResult 上架结果
type OnlineResult struct {
	Result bool   `json:"result"`
	Msg    string `json:"msg"`
}

// RelistRequest 重新上架请求
type RelistRequest struct {
	GoodsID string   `json:"goods_id"`
	SkuIDs  []string `json:"sku_ids"`
}

// RelistResponse 重新上架响应
type RelistResponse struct {
	Success   bool `json:"success"`
	ErrorCode int  `json:"error_code"`
	Result    struct {
		Result bool `json:"result"`
	} `json:"result"`
}

// DelistRequest 下架请求（带操作来源）
type DelistRequest struct {
	GoodsID         string   `json:"goods_id"`
	SkuIDs          []string `json:"sku_ids"`
	OperationSource int      `json:"operation_source"`
}

// DelistResponse 下架响应
type DelistResponse struct {
	Success   bool     `json:"success"`
	ErrorCode int      `json:"error_code"`
	Result    struct{} `json:"result"`
}

// --- 已下架产品搜索数据模型 ---

// SearchRequest 已下架产品搜索请求
type SearchRequest struct {
	PageSize              int    `json:"page_size"`
	PageNo                int    `json:"page_no"`
	OrderType             int    `json:"order_type"`
	OrderField            string `json:"order_field"`
	EnableBatchSearchText bool   `json:"enable_batch_search_text"`
	SkuSearchType         int    `json:"sku_search_type"`
}

// SearchResponse 已下架产品搜索响应
type SearchResponse struct {
	Success   bool `json:"success"`
	ErrorCode int  `json:"error_code"`
	Result    struct {
		PageNum int    `json:"page_num"`
		Total   int    `json:"total"`
		SkuList []Item `json:"sku_list"`
	} `json:"result"`
}

// Item 已下架产品项
type Item struct {
	GoodsName                   string                    `json:"goods_name"`
	SpecName                    string                    `json:"spec_name"`
	SpecList                    []SpecInfo                `json:"spec_list"`
	ThumbURL                    string                    `json:"thumb_url"`
	GoodsID                     string                    `json:"goods_id"`
	GoodsCommitID               string                    `json:"goods_commit_id"`
	ListingCommitID             string                    `json:"listing_commit_id"`
	MallID                      string                    `json:"mall_id"`
	SkuID                       string                    `json:"sku_id"`
	SkuSN                       string                    `json:"sku_sn"`
	Stock                       int                       `json:"stock"`
	Price                       float64                   `json:"price"`
	PriceVO                     PriceVO                   `json:"price_vo"`
	CrtTime                     string                    `json:"crt_time"`
	Status4VO                   int                       `json:"status4_vo"`
	SubStatus4VO                int                       `json:"sub_status4_vo"`
	ClosedTypeList              []any                     `json:"closed_type_list"`
	SupplierPrice               float64                   `json:"supplier_price"`
	GoodsIsOnsale               int                       `json:"goods_is_onsale"`
	Currency                    string                    `json:"currency"`
	ListPrice                   PriceVO                   `json:"list_price"`
	ListPriceVO                 PriceVO                   `json:"list_price_vo"`
	StatusUpdateTime            string                    `json:"status_update_time"`
	CatType                     int                       `json:"cat_type"`
	LockInfo                    LockInfo                  `json:"lock_info"`
	MultiSiteGoods              bool                      `json:"multi_site_goods"`
	CheckPriceAuditInfo         map[string]any            `json:"check_price_audit_info"`
	AdjustPriceAuditInfo        AdjustPriceAuditInfo      `json:"adjust_price_audit_info"`
	ShowSubStatus4VO            int                       `json:"show_sub_status4_vo"`
	NewVersionApprovedLabelShow bool                      `json:"new_version_approved_label_show"`
	PersonalizationStatus       int                       `json:"personalization_status"`
	PunishTags                  int                       `json:"punish_tags"`
	CatNameList                 []string                  `json:"cat_name_list"`
	CatID                       int                       `json:"cat_id"`
	CategoryRectificationInfo   CategoryRectificationInfo `json:"category_rectification_info"`
	StockDisplayTag             int                       `json:"stock_display_tag"`
	LowTrafficTag               int                       `json:"low_traffic_tag"`
	RestrictedTrafficTag        int                       `json:"restricted_traffic_tag"`
	PreSaleStockInfo            PreSaleStockInfo          `json:"pre_sale_stock_info"`
	OrdinaryStock               int                       `json:"ordinary_stock"`
	ShippingMode                int                       `json:"shipping_mode"`
	OutGoodsSN                  string                    `json:"out_goods_sn"`
	EasyGainsTag                int                       `json:"easy_gains_tag"`
	IsBooks                     bool                      `json:"is_books"`
	StockSearchTags             []any                     `json:"stock_search_tags"`
}

// SpecInfo 规格信息
type SpecInfo = types.SpecInfo

// PriceVO 价格对象
type PriceVO = types.PriceVO

// LockInfo 锁定信息
type LockInfo = types.LockInfo

// AdjustPriceAuditInfo 调价审核信息
type AdjustPriceAuditInfo = types.AdjustPriceAuditInfo

// PreSaleStockInfo 预售库存信息
type PreSaleStockInfo = types.PreSaleStockInfo

// CategoryRectificationInfo 分类整改信息
type CategoryRectificationInfo struct {
	NeedRectification bool  `json:"need_rectification"`
	ExpectedTime      int64 `json:"expected_time"`
}

// --- API ---

// API 库存和上下架API管理器
type API struct {
	client client.ClientAPI
	logger *logrus.Entry
}

// NewAPI 创建库存API管理器
func NewAPI(c client.ClientAPI, logger *logrus.Entry) *API {
	return &API{client: c, logger: logger}
}

func (a *API) defaultHeaders() map[string]string {
	h := client.GetDefaultHeaders()
	h["content-type"] = "application/json;charset=UTF-8"
	h["x-document-referer"] = "https://seller.temu.com/products.html"
	return h
}

// EditStock 修改商品库存
func (a *API) EditStock(goodsID string, changes []SkuStockChange) (*StockEditResponse, error) {
	if goodsID == "" {
		return nil, fmt.Errorf("商品ID不能为空")
	}
	if len(changes) == 0 {
		return nil, fmt.Errorf("SKU库存变更列表不能为空")
	}

	req := map[string]any{
		"method":  "POST",
		"url":     "/mms/marigold/stock/edit",
		"headers": a.defaultHeaders(),
		"body":    &StockEditRequest{GoodsID: goodsID, SkuStockChangeList: changes},
	}

	var result StockEditResponse
	if err := a.client.SendTEMURequest(req, &result); err != nil {
		return nil, fmt.Errorf("修改库存失败: %w", err)
	}
	if !result.Success {
		return nil, fmt.Errorf("修改库存失败: errorCode=%d", result.ErrorCode)
	}
	return &result, nil
}

// SetStock 设置SKU库存（便捷方法）
func (a *API) SetStock(goodsID, skuID string, stock, stockDiff int) error {
	result, err := a.EditStock(goodsID, []SkuStockChange{
		{SkuID: skuID, CurrentShippingMode: 1, CurrentStockAvailable: stock, StockDiff: stockDiff},
	})
	if err != nil {
		return err
	}
	if !result.Result.Result {
		return fmt.Errorf("设置库存失败")
	}
	return nil
}

// postWithHeaders 使用 defaultHeaders 发送 POST 请求的通用方法
func (a *API) postWithHeaders(url string, body any, result any) error {
	req := map[string]any{
		"method":  "POST",
		"url":     url,
		"headers": a.defaultHeaders(),
		"body":    body,
	}
	return a.client.SendTEMURequest(req, result)
}

// Offline 下架产品
func (a *API) Offline(goodsID string, skuIDs []string) (*OfflineResponse, error) {
	if goodsID == "" {
		return nil, fmt.Errorf("商品ID不能为空")
	}
	if len(skuIDs) == 0 {
		return nil, fmt.Errorf("SKU ID列表不能为空")
	}
	var result OfflineResponse
	if err := a.postWithHeaders("/mms/marigold/sku/offline", &OfflineRequest{GoodsID: goodsID, SkuIDs: skuIDs}, &result); err != nil {
		return nil, fmt.Errorf("下架产品失败: %w", err)
	}
	if !result.Success {
		return nil, fmt.Errorf("下架产品失败: errorCode=%d", result.ErrorCode)
	}
	return &result, nil
}

// Online 上架产品
func (a *API) Online(goodsID string, skuIDs []string) (*OnlineResponse, error) {
	if goodsID == "" {
		return nil, fmt.Errorf("商品ID不能为空")
	}
	if len(skuIDs) == 0 {
		return nil, fmt.Errorf("SKU ID列表不能为空")
	}
	var result OnlineResponse
	if err := a.postWithHeaders("/mms/marigold/sku/online", &OnlineRequest{GoodsID: goodsID, SkuIDs: skuIDs}, &result); err != nil {
		return nil, fmt.Errorf("上架产品失败: %w", err)
	}
	if !result.Success {
		return nil, fmt.Errorf("上架产品失败: errorCode=%d", result.ErrorCode)
	}
	return &result, nil
}

// Relist 重新上架产品
func (a *API) Relist(goodsID string, skuIDs []string) (*RelistResponse, error) {
	if goodsID == "" {
		return nil, fmt.Errorf("商品ID不能为空")
	}
	if len(skuIDs) == 0 {
		return nil, fmt.Errorf("SKU ID列表不能为空")
	}
	var result RelistResponse
	if err := a.postWithHeaders("/mms/marigold/sku/online", &RelistRequest{GoodsID: goodsID, SkuIDs: skuIDs}, &result); err != nil {
		return nil, fmt.Errorf("重新上架产品失败: %w", err)
	}
	if !result.Success {
		return nil, fmt.Errorf("重新上架产品失败: errorCode=%d", result.ErrorCode)
	}
	return &result, nil
}

// SearchOffline 获取已下架产品列表
func (a *API) SearchOffline(pageNo, pageSize int) (*SearchResponse, error) {
	if pageSize > 200 {
		pageSize = 200
	}
	if pageSize <= 0 {
		pageSize = 50
	}
	if pageNo <= 0 {
		pageNo = 1
	}

	h := client.GetDefaultHeaders()
	h["content-type"] = "application/json;charset=UTF-8"
	h["x-document-referer"] = "https://seller.temu.com/"

	req := map[string]any{
		"method":  "POST",
		"url":     "/mms/marigold/sku/v2/search",
		"headers": h,
		"body": &SearchRequest{
			PageSize:              pageSize,
			PageNo:                pageNo,
			OrderType:             0,
			OrderField:            "gmt_create",
			EnableBatchSearchText: true,
			SkuSearchType:         3,
		},
	}

	var result SearchResponse
	if err := a.client.SendTEMURequest(req, &result); err != nil {
		return nil, fmt.Errorf("获取已下架产品列表失败: %w", err)
	}
	if !result.Success {
		return nil, fmt.Errorf("获取已下架产品列表失败: errorCode=%d", result.ErrorCode)
	}
	return &result, nil
}
