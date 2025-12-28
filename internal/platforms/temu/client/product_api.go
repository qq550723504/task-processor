// Package client 提供TEMU平台的产品API功能
package client

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
	ListingCommitID       string        `json:"listing_commit_id"`
	GoodsCommitID         string        `json:"goods_commit_id"`
	GoodsID               string        `json:"goods_id"`
	GoodsName             string        `json:"goods_name"`
	SpecName              string        `json:"spec_name"`
	ThumbURL              string        `json:"thumb_url"`
	SkuPreviewURL         string        `json:"sku_preview_url"`
	MallID                string        `json:"mall_id"`
	OutGoodsSn            string        `json:"out_goods_sn"`
	Status4Vo             int           `json:"status4_vo"`     // 3=已上架
	SubStatus4Vo          int           `json:"sub_status4_vo"` // 子状态
	ClosedTypeList        []interface{} `json:"closed_type_list"`
	Currency              string        `json:"currency"`
	MarketPrice           string        `json:"market_price"`
	MarketPriceVo         PriceVO       `json:"market_price_vo"`
	ListPrice             PriceVO       `json:"list_price"`
	ListPriceVo           PriceVO       `json:"list_price_vo"`
	OutSkuSnList          []string      `json:"out_sku_sn_list"`
	SkuIDList             []string      `json:"sku_id_list"`
	Price                 float64       `json:"price"`
	PriceVo               PriceVO       `json:"price_vo"`
	Quantity              int           `json:"quantity"`
	VariationsCount       int           `json:"variations_count"`
	CrtTime               string        `json:"crt_time"`
	StatusUpdateTime      string        `json:"status_update_time"`
	SupplierPrice         float64       `json:"supplier_price"`
	GoodsAllowSiteList    []interface{} `json:"goods_allow_site_list"`
	CatType               int           `json:"cat_type"`
	CatID                 int64         `json:"cat_id"`
	CatNameList           []string      `json:"cat_name_list"`
	MultiSiteGoods        bool          `json:"multi_site_goods"`
	ShowSubStatus4Vo      int           `json:"show_sub_status4_vo"`
	PersonalizationStatus int           `json:"personalization_status"`
	PunishTags            int           `json:"punish_tags"`
	StockDisplayTag       int           `json:"stock_display_tag"`
	LowTrafficTag         int           `json:"low_traffic_tag"`
	RestrictedTrafficTag  int           `json:"restricted_traffic_tag"`
	OrdinaryStock         int           `json:"ordinary_stock"`
	ShippingMode          int           `json:"shipping_mode"`
	EasyGainsTag          int           `json:"easy_gains_tag"`
	IsBooks               bool          `json:"is_books"`
}

// ProductAPIHandler 产品API处理器
type ProductAPIHandler struct {
	requestSender *RequestSender
}

// NewProductAPIHandler 创建产品API处理器
func NewProductAPIHandler(requestSender *RequestSender) *ProductAPIHandler {
	return &ProductAPIHandler{
		requestSender: requestSender,
	}
}

// ListProducts 获取产品列表
// 参考接口: POST https://seller.temu.com/mms/marigold/sku/v2/search
func (p *ProductAPIHandler) ListProducts(pageNo, pageSize int) (*ProductListResponse, error) {
	request := map[string]interface{}{
		"method": "POST",
		"url":    "/mms/marigold/sku/v2/search",
		"headers": map[string]string{
			"content-type": "application/json;charset=UTF-8",
		},
		"body": ProductListRequest{
			PageSize:              pageSize,
			PageNo:                pageNo,
			OrderType:             0,
			OrderField:            "gmt_create",
			EnableBatchSearchText: false,
			SkuSearchType:         2,
		},
	}

	var response ProductListResponse
	err := p.requestSender.SendTEMURequest(request, &response)
	if err != nil {
		return nil, err
	}

	return &response, nil
}
