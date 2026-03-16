// Package pricing 提供TEMU平台定价相关的API和数据结构
package pricing

import (
	"fmt"
	"strings"
	"task-processor/internal/platforms/temu/api/client"

	"github.com/sirupsen/logrus"
)

// --- 数据模型 ---

// PriceVO 价格对象
type PriceVO struct {
	Amount   string `json:"amount"`
	Currency string `json:"currency"`
}

// SpecInfo 规格信息
type SpecInfo struct {
	SpecID         string `json:"spec_id"`
	SpecName       string `json:"spec_name"`
	ParentSpecName string `json:"parent_spec_name"`
}

// LockInfo 锁定信息
type LockInfo struct {
	CloseListingMMS struct {
		AllowOperate       bool   `json:"allow_operate"`
		AllowShow          bool   `json:"allow_show"`
		AllowOperateReason string `json:"allow_operate_reason"`
	} `json:"close_listing_mms"`
	AllowEditPersonalizationInfo bool `json:"allow_edit_personalization_info"`
}

// AdjustPriceAuditInfo 调价审核信息
type AdjustPriceAuditInfo struct {
	Currency                string  `json:"currency"`
	PricingType             int     `json:"pricing_type"`
	PriceAuditOrderID       string  `json:"price_audit_order_id"`
	SuggestSupplierPrice    float64 `json:"suggest_supplier_price"`
	SourceSupplierPrice     int     `json:"source_supplier_price"`
	TargetSupplierPrice     int     `json:"target_supplier_price"`
	SupplierPrice           int     `json:"supplier_price"`
	CrtTime                 int64   `json:"crt_time"`
	UptTime                 int64   `json:"upt_time"`
	Status                  int     `json:"status"`
	PriceCommitVersion      int     `json:"price_commit_version"`
	GoodsCommitEdit         int     `json:"goods_commit_edit"`
	SuggestSupplierPriceStr string  `json:"suggest_supplier_price_str"`
	SourceSupplierPriceStr  string  `json:"source_supplier_price_str"`
	TargetSupplierPriceStr  string  `json:"target_supplier_price_str"`
	SupplierPriceStr        string  `json:"supplier_price_str"`
}

// PreSaleStockInfo 预售库存信息
type PreSaleStockInfo struct {
	PreSaleStockShowStatus         int `json:"pre_sale_stock_show_status"`
	MaximumPreSaleStock            int `json:"maximum_pre_sale_stock"`
	Version                        int `json:"version"`
	PreSaleEnableDays              int `json:"pre_sale_enable_days"`
	PreSaleStockReopenIntervalDate int `json:"pre_sale_stock_reopen_interval_date"`
}

// PendingListRequest 待核价列表请求
type PendingListRequest struct {
	PageSize int    `json:"page_size"`
	PageNo   int    `json:"page_no"`
	Scene    string `json:"scene"`
}

// PendingListResponse 待核价列表响应
type PendingListResponse struct {
	Success   bool `json:"success"`
	ErrorCode int  `json:"error_code"`
	Result    struct {
		PageNum             int               `json:"page_num"`
		Total               int               `json:"total"`
		SalesBoostGoodsList []SalesBoostGoods `json:"sales_boost_goods_list"`
	} `json:"result"`
}

// SalesBoostGoods 销量提升商品信息
type SalesBoostGoods struct {
	SalesBoostGoodsBasicInfo SalesBoostGoodsBasicInfo `json:"sales_boost_goods_basic_info"`
	SalesBoostSkuList        []SalesBoostSku          `json:"sales_boost_sku_list"`
	HoverInfo                HoverInfo                `json:"hover_info"`
}

// SalesBoostGoodsBasicInfo 商品基本信息
type SalesBoostGoodsBasicInfo struct {
	GoodsID     string   `json:"goods_id"`
	GoodsName   string   `json:"goods_name"`
	CatType     int      `json:"cat_type"`
	CatID       string   `json:"cat_id"`
	CatNameList []string `json:"cat_name_list"`
	HdThumbURL  string   `json:"hd_thumb_url"`
	IsBooks     bool     `json:"is_books"`
}

// SalesBoostSku 销量提升SKU信息
type SalesBoostSku struct {
	GoodsID               string                 `json:"goods_id"`
	SkcID                 string                 `json:"skc_id"`
	SkuID                 string                 `json:"sku_id"`
	SpecNames             []SpecInfo             `json:"spec_names"`
	SkuThumbURL           string                 `json:"sku_thumb_url"`
	OutSkuSN              string                 `json:"out_sku_sn"`
	CurrentSupplierPrice  PriceVO                `json:"current_supplier_price"`
	TargetSupplierPrice   PriceVO                `json:"target_supplier_price"`
	TargetRetailPrice     PriceVO                `json:"target_retail_price"`
	CurrentRetailPrice    PriceVO                `json:"current_retail_price"`
	RecRetailPriceTipInfo map[string]any `json:"rec_retail_price_tip_info"`
	CreateTime            int64                  `json:"create_time"`
	PriceSpreadRatio      string                 `json:"price_spread_ratio"`
	EasyGainsTag          int                    `json:"easy_gains_tag"`
	Active                int                    `json:"active"`
	HighPriceSpreadRatio  bool                   `json:"high_price_spread_ratio"`
	ProposalStatus        int                    `json:"proposal_status"`
	ActionInfo            ActionInfo             `json:"action_info"`
}

// ActionInfo 操作信息
type ActionInfo struct {
	AllowCreateAppealOrder bool `json:"allow_create_appeal_order"`
	AllowViewAppealOrder   bool `json:"allow_view_appeal_order"`
	ShowAppealBubble       bool `json:"show_appeal_bubble"`
}

// HoverInfo 悬浮信息
type HoverInfo struct {
	HoverSkuInfoList   []HoverSkuInfo `json:"hover_sku_info_list"`
	TotalSkuCount      int            `json:"total_sku_count"`
	NeedActionSkuCount int            `json:"need_action_sku_count"`
}

// HoverSkuInfo 悬浮SKU信息
type HoverSkuInfo struct {
	SkuID       string     `json:"sku_id"`
	SpecNames   []SpecInfo `json:"spec_names"`
	SkuThumbURL string     `json:"sku_thumb_url"`
	OutSkuSN    string     `json:"out_sku_sn"`
	NeedAction  int        `json:"need_action"`
}

// Sku 待核价商品项
type Sku struct {
	GoodsName                   string                 `json:"goods_name"`
	SpecName                    string                 `json:"spec_name"`
	SpecList                    []SpecInfo             `json:"spec_list"`
	ThumbURL                    string                 `json:"thumb_url"`
	GoodsID                     string                 `json:"goods_id"`
	GoodsCommitID               string                 `json:"goods_commit_id"`
	ListingCommitID             string                 `json:"listing_commit_id"`
	MallID                      string                 `json:"mall_id"`
	SkuID                       string                 `json:"sku_id"`
	SkuSN                       string                 `json:"sku_sn"`
	Stock                       int                    `json:"stock"`
	Price                       float64                `json:"price"`
	PriceVO                     PriceVO                `json:"price_vo"`
	CrtTime                     string                 `json:"crt_time"`
	Status4VO                   int                    `json:"status4_vo"`
	SubStatus4VO                int                    `json:"sub_status4_vo"`
	ClosedTypeList              []any          `json:"closed_type_list"`
	SupplierPrice               float64                `json:"supplier_price"`
	GoodsIsOnsale               int                    `json:"goods_is_onsale"`
	Currency                    string                 `json:"currency"`
	ListPrice                   PriceVO                `json:"list_price"`
	ListPriceVO                 PriceVO                `json:"list_price_vo"`
	StatusUpdateTime            string                 `json:"status_update_time"`
	CatType                     int                    `json:"cat_type"`
	LockInfo                    LockInfo               `json:"lock_info"`
	MultiSiteGoods              bool                   `json:"multi_site_goods"`
	CheckPriceAuditInfo         map[string]any `json:"check_price_audit_info"`
	AdjustPriceAuditInfo        AdjustPriceAuditInfo   `json:"adjust_price_audit_info"`
	ShowSubStatus4VO            int                    `json:"show_sub_status4_vo"`
	NewVersionApprovedLabelShow bool                   `json:"new_version_approved_label_show"`
	CheckedTime                 int64                  `json:"checked_time"`
	PersonalizationStatus       int                    `json:"personalization_status"`
	PunishTags                  int                    `json:"punish_tags"`
	CatNameList                 []string               `json:"cat_name_list"`
	CatID                       int                    `json:"cat_id"`
	StockDisplayTag             int                    `json:"stock_display_tag"`
	LowTrafficTag               int                    `json:"low_traffic_tag"`
	RestrictedTrafficTag        int                    `json:"restricted_traffic_tag"`
	PreSaleStockInfo            PreSaleStockInfo       `json:"pre_sale_stock_info"`
	OrdinaryStock               int                    `json:"ordinary_stock"`
	ShippingMode                int                    `json:"shipping_mode"`
	OutGoodsSN                  string                 `json:"out_goods_sn"`
	EasyGainsTag                int                    `json:"easy_gains_tag"`
	IsBooks                     bool                   `json:"is_books"`
}

// RejectRequest 拒绝平台报价请求
type RejectRequest struct {
	GoodsID         string   `json:"goods_id"`
	SkuIDs          []string `json:"sku_ids"`
	OperationSource int      `json:"operation_source"`
}

// RejectResponse 拒绝平台报价响应
type RejectResponse struct {
	Success   bool     `json:"success"`
	ErrorCode int      `json:"error_code"`
	Result    struct{} `json:"result"`
}

// ReappealRequest 重新报价请求
type ReappealRequest struct {
	GoodsID              string            `json:"goods_id"`
	AppealSource         int               `json:"appeal_source"`
	MerchantAppealReason []string          `json:"merchant_appeal_reason"`
	SkuInfoList          []ReappealSkuInfo `json:"sku_info_list"`
}

// ReappealSkuInfo 重新报价SKU信息
type ReappealSkuInfo struct {
	SkuID                       string `json:"sku_id"`
	SupplierPriceStr            string `json:"supplier_price_str"`
	RecommendedSupplierPriceStr string `json:"recommended_supplier_price_str"`
	TargetSupplierPriceStr      string `json:"target_supplier_price_str"`
	Currency                    string `json:"currency"`
}

// ReappealResponse 重新报价响应
type ReappealResponse struct {
	Success   bool     `json:"success"`
	ErrorCode int      `json:"error_code"`
	Result    struct{} `json:"result"`
}

// AcceptRequest 接受平台报价请求
type AcceptRequest struct {
	Scene   int             `json:"scene"`
	GoodsID string          `json:"goods_id"`
	SkuList []AcceptSkuInfo `json:"sku_list"`
}

// AcceptSkuInfo 接受报价SKU信息
type AcceptSkuInfo struct {
	SkuID                  string `json:"sku_id"`
	Currency               string `json:"currency"`
	TargetSupplierPriceStr string `json:"target_supplier_price_str"`
}

// AcceptResponse 接受平台报价响应
type AcceptResponse struct {
	Success   bool     `json:"success"`
	ErrorCode int      `json:"error_code"`
	Result    struct{} `json:"result"`
}

// DecisionAction 核价决策动作
type DecisionAction string

const (
	DecisionAccept   DecisionAction = "accept"
	DecisionReject   DecisionAction = "reject"
	DecisionReappeal DecisionAction = "reappeal"
	DecisionSkip     DecisionAction = "skip"
)

// Decision 核价决策结果
type Decision struct {
	Sku             *Sku
	Action          DecisionAction
	Reason          string
	ProfitMargin    float64
	TargetPrice     float64
	TargetMargin    float64
	MinMargin       float64
	AcceptablePrice float64
}

// Statistics 核价统计信息
type Statistics struct {
	TotalProcessed int
	AcceptCount    int
	RejectCount    int
	ReappealCount  int
	SkipCount      int
	SuccessCount   int
	FailCount      int
}

// --- API ---

// API 定价API管理器
type API struct {
	client client.ClientAPI
	logger *logrus.Entry
}

// NewAPI 创建定价API管理器
func NewAPI(c client.ClientAPI, logger *logrus.Entry) *API {
	return &API{client: c, logger: logger}
}

func (a *API) defaultHeaders() map[string]string {
	h := client.GetDefaultHeaders()
	h["content-type"] = "application/json;charset=UTF-8"
	h["x-document-referer"] = "https://seller.temu.com/"
	return h
}

// GetPendingList 获取待核价列表
func (a *API) GetPendingList(pageNo, pageSize int) (*PendingListResponse, error) {
	req := map[string]any{
		"method":  "POST",
		"url":     "/mms/marigold/price/v2/search_sales_boost",
		"headers": a.defaultHeaders(),
		"body": &PendingListRequest{
			PageSize: pageSize,
			PageNo:   pageNo,
			Scene:    "PRICING_HEALTH_SALES_BOOST",
		},
	}

	var result PendingListResponse
	if err := a.client.SendTEMURequest(req, &result); err != nil {
		if isTimeoutError(err) {
			return nil, fmt.Errorf("API调用超时: %w", err)
		}
		return nil, fmt.Errorf("获取待核价列表失败: %w", err)
	}
	if !result.Success {
		return nil, fmt.Errorf("获取待核价列表失败: errorCode=%d", result.ErrorCode)
	}
	return &result, nil
}

// AcceptPrice 接受平台报价
func (a *API) AcceptPrice(goodsID string, sku *SalesBoostSku) error {
	h := client.GetDefaultHeaders()
	h["content-type"] = "application/json;charset=UTF-8"
	h["x-document-referer"] = "https://seller.temu.com/products.html"

	req := map[string]any{
		"method":  "POST",
		"url":     "/mms/marigold/price/goods/change",
		"headers": h,
		"body": &AcceptRequest{
			Scene:   2,
			GoodsID: goodsID,
			SkuList: []AcceptSkuInfo{{
				SkuID:                  sku.SkuID,
				Currency:               sku.TargetSupplierPrice.Currency,
				TargetSupplierPriceStr: sku.TargetSupplierPrice.Amount,
			}},
		},
	}

	var result AcceptResponse
	if err := a.client.SendTEMURequest(req, &result); err != nil {
		return fmt.Errorf("接受平台报价失败: %w", err)
	}
	if !result.Success {
		return fmt.Errorf("接受平台报价失败: errorCode=%d", result.ErrorCode)
	}
	return nil
}

// RejectPrice 拒绝平台报价
func (a *API) RejectPrice(goodsID string, skuIDs []string) error {
	h := client.GetDefaultHeaders()
	h["content-type"] = "application/json;charset=UTF-8"
	h["x-document-referer"] = "https://seller.temu.com/products.html"

	req := map[string]any{
		"method":  "POST",
		"url":     "/mms/marigold/sku/offline",
		"headers": h,
		"body":    &RejectRequest{GoodsID: goodsID, SkuIDs: skuIDs, OperationSource: 1005},
	}

	var result RejectResponse
	if err := a.client.SendTEMURequest(req, &result); err != nil {
		return fmt.Errorf("拒绝平台报价失败: %w", err)
	}
	if !result.Success {
		return fmt.Errorf("拒绝平台报价失败: errorCode=%d", result.ErrorCode)
	}
	return nil
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
