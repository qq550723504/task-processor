// Package temu 提供TEMU平台产品API功能
package temu

import (
	"fmt"

	"github.com/sirupsen/logrus"
)

// ProductAPI 产品API管理器
type ProductAPI struct {
	client APIClientInterface
	logger *logrus.Entry
}

// NewProductAPI 创建新的产品API管理器
func NewProductAPI(client APIClientInterface, logger *logrus.Entry) *ProductAPI {
	return &ProductAPI{
		client: client,
		logger: logger,
	}
}

// ProductListRequest 产品列表请求参数
type ProductListRequest struct {
	PageSize              int    `json:"page_size"`
	PageNo                int    `json:"page_no"`
	OrderType             int    `json:"order_type"`  // 0=降序, 1=升序
	OrderField            string `json:"order_field"` // gmt_create=创建时间
	EnableBatchSearchText bool   `json:"enable_batch_search_text"`
	SkuSearchType         int    `json:"sku_search_type"` // 2=全部
}

// ProductListResponse 产品列表响应
type ProductListResponse struct {
	Success   bool `json:"success"`
	ErrorCode int  `json:"error_code"`
	Result    struct {
		PageNum   int                   `json:"page_num"`
		Total     int                   `json:"total"`
		GoodsList []TemuProductResponse `json:"goods_list"`
	} `json:"result"`
}

// TemuProductResponse TEMU 产品响应
type TemuProductResponse struct {
	ListingCommitID       string   `json:"listing_commit_id"`
	GoodsCommitID         string   `json:"goods_commit_id"`
	GoodsID               string   `json:"goods_id"`
	GoodsName             string   `json:"goods_name"`
	SpecName              string   `json:"spec_name"`
	ThumbURL              string   `json:"thumb_url"`
	SkuPreviewURL         string   `json:"sku_preview_url"`
	MallID                string   `json:"mall_id"`
	OutGoodsSn            string   `json:"out_goods_sn"`
	Status4Vo             int      `json:"status4_vo"`     // 3=已上架
	SubStatus4Vo          int      `json:"sub_status4_vo"` // 子状态
	ClosedTypeList        []any    `json:"closed_type_list"`
	Currency              string   `json:"currency"`
	MarketPrice           string   `json:"market_price"`
	MarketPriceVo         PriceVO  `json:"market_price_vo"`
	ListPrice             PriceVO  `json:"list_price"`
	ListPriceVo           PriceVO  `json:"list_price_vo"`
	OutSkuSnList          []string `json:"out_sku_sn_list"`
	SkuIDList             []string `json:"sku_id_list"`
	Price                 float64  `json:"price"`
	PriceVo               PriceVO  `json:"price_vo"`
	Quantity              int      `json:"quantity"`
	VariationsCount       int      `json:"variations_count"`
	CrtTime               string   `json:"crt_time"`
	StatusUpdateTime      string   `json:"status_update_time"`
	SupplierPrice         float64  `json:"supplier_price"`
	GoodsAllowSiteList    []any    `json:"goods_allow_site_list"`
	CatType               int      `json:"cat_type"`
	CatID                 int64    `json:"cat_id"`
	CatNameList           []string `json:"cat_name_list"`
	MultiSiteGoods        bool     `json:"multi_site_goods"`
	ShowSubStatus4Vo      int      `json:"show_sub_status4_vo"`
	PersonalizationStatus int      `json:"personalization_status"`
	PunishTags            int      `json:"punish_tags"`
	StockDisplayTag       int      `json:"stock_display_tag"`
	LowTrafficTag         int      `json:"low_traffic_tag"`
	RestrictedTrafficTag  int      `json:"restricted_traffic_tag"`
	OrdinaryStock         int      `json:"ordinary_stock"`
	ShippingMode          int      `json:"shipping_mode"`
	EasyGainsTag          int      `json:"easy_gains_tag"`
	IsBooks               bool     `json:"is_books"`
}

// ListProducts 获取产品列表
// 参考接口: POST https://seller.temu.com/mms/marigold/sku/v2/search
func (p *ProductAPI) ListProducts(pageNo, pageSize int) (*ProductListResponse, error) {
	request := map[string]any{
		"method": "POST",
		"url":    "/mms/marigold/sku/v2/search",
		"headers": map[string]string{
			"content-type":       "application/json;charset=UTF-8",
			"x-document-referer": "https://seller.temu.com/",
		},
		"body": ProductListRequest{
			PageSize:              pageSize,
			PageNo:                pageNo,
			OrderType:             0,            // 降序
			OrderField:            "gmt_create", // 按创建时间排序
			EnableBatchSearchText: true,
			SkuSearchType:         2, // 全部
		},
	}

	var result ProductListResponse
	// 通过认证管理器发送请求
	authManager := NewAuthManager(p.client.GetCookieManager(), p.logger)
	if err := authManager.SendRequestWithAuth(p.client, request, &result); err != nil {
		return nil, fmt.Errorf("调用产品列表 API 失败: %w", err)
	}

	if !result.Success {
		return nil, fmt.Errorf("API 返回错误: error_code=%d", result.ErrorCode)
	}

	p.logger.WithFields(logrus.Fields{
		"page_no":   pageNo,
		"page_size": pageSize,
		"total":     result.Result.Total,
		"count":     len(result.Result.GoodsList),
	}).Info("成功获取 TEMU 产品列表")

	return &result, nil
}

// ListOnShelfProducts 获取已上架的产品列表
func (p *ProductAPI) ListOnShelfProducts(pageNo, pageSize int) ([]TemuProductResponse, error) {
	response, err := p.ListProducts(pageNo, pageSize)
	if err != nil {
		return nil, err
	}

	// 过滤出已上架的产品 (status4_vo == 3)
	var onShelfProducts []TemuProductResponse
	for _, product := range response.Result.GoodsList {
		if product.Status4Vo == 3 {
			onShelfProducts = append(onShelfProducts, product)
		}
	}

	p.logger.WithFields(logrus.Fields{
		"total":    len(response.Result.GoodsList),
		"on_shelf": len(onShelfProducts),
	}).Info("过滤已上架产品")

	return onShelfProducts, nil
}

// GetProduct 获取单个产品详情
func (p *ProductAPI) GetProduct(goodsID string) (*TemuProductResponse, error) {
	// 先获取产品列表，然后查找指定的产品
	// TEMU 可能有专门的产品详情接口，这里暂时使用列表接口
	response, err := p.ListProducts(1, 100)
	if err != nil {
		return nil, err
	}

	for _, product := range response.Result.GoodsList {
		if product.GoodsID == goodsID {
			return &product, nil
		}
	}

	return nil, fmt.Errorf("未找到产品: %s", goodsID)
}
