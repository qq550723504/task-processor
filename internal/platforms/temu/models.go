package temu

import (
	"fmt"
	"time"
)

// TemuProductResponse TEMU 产品列表响应
type TemuProductResponse struct {
	ListingCommitID       string           `json:"listing_commit_id"`
	GoodsCommitID         string           `json:"goods_commit_id"`
	GoodsID               string           `json:"goods_id"`
	GoodsName             string           `json:"goods_name"`
	SpecName              string           `json:"spec_name"`
	ThumbURL              string           `json:"thumb_url"`
	SkuPreviewURL         string           `json:"sku_preview_url"`
	MallID                string           `json:"mall_id"`
	OutGoodsSn            string           `json:"out_goods_sn"`
	Status4Vo             int              `json:"status4_vo"`     // 3=已上架
	SubStatus4Vo          int              `json:"sub_status4_vo"` // 子状态
	ClosedTypeList        []interface{}    `json:"closed_type_list"`
	Currency              string           `json:"currency"`
	MarketPrice           string           `json:"market_price"`
	MarketPriceVo         PriceVO          `json:"market_price_vo"`
	ListPrice             PriceVO          `json:"list_price"`
	ListPriceVo           PriceVO          `json:"list_price_vo"`
	OutSkuSnList          []string         `json:"out_sku_sn_list"`
	SkuIDList             []string         `json:"sku_id_list"`
	Price                 float64          `json:"price"`
	PriceVo               PriceVO          `json:"price_vo"`
	Quantity              int              `json:"quantity"`
	VariationsCount       int              `json:"variations_count"`
	CrtTime               string           `json:"crt_time"`
	StatusUpdateTime      string           `json:"status_update_time"`
	SupplierPrice         float64          `json:"supplier_price"`
	GoodsAllowSiteList    []interface{}    `json:"goods_allow_site_list"`
	CatType               int              `json:"cat_type"`
	CatID                 int64            `json:"cat_id"`
	CatNameList           []string         `json:"cat_name_list"`
	LockInfo              LockInfo         `json:"lock_info"`
	MultiSiteGoods        bool             `json:"multi_site_goods"`
	CheckPriceAuditList   []interface{}    `json:"check_price_audit_list"`
	AdjustPriceAuditList  []interface{}    `json:"adjust_price_audit_list"`
	ShowSubStatus4Vo      int              `json:"show_sub_status4_vo"`
	PersonalizationStatus int              `json:"personalization_status"`
	PunishTags            int              `json:"punish_tags"`
	StockDisplayTag       int              `json:"stock_display_tag"`
	LowTrafficTag         int              `json:"low_traffic_tag"`
	RestrictedTrafficTag  int              `json:"restricted_traffic_tag"`
	PreSaleStockInfo      PreSaleStockInfo `json:"pre_sale_stock_info"`
	OrdinaryStock         int              `json:"ordinary_stock"`
	ShippingMode          int              `json:"shipping_mode"`
	EasyGainsTag          int              `json:"easy_gains_tag"`
	IsBooks               bool             `json:"is_books"`
}

// PriceVO 价格对象
type PriceVO struct {
	Amount   string `json:"amount"`
	Currency string `json:"currency"`
}

// LockInfo 锁定信息
type LockInfo struct {
	IsForbidEdit                 bool                  `json:"is_forbid_edit"`
	AllowOperateDelete           bool                  `json:"allow_operate_delete"`
	ReplicateButtonConfig        ReplicateButtonConfig `json:"replicate_button_config"`
	AllowEditPersonalizationInfo bool                  `json:"allow_edit_personalization_info"`
}

// ReplicateButtonConfig 复制按钮配置
type ReplicateButtonConfig struct {
	AllowOperate bool `json:"allow_operate"`
	AllowShow    bool `json:"allow_show"`
}

// PreSaleStockInfo 预售库存信息
type PreSaleStockInfo struct {
	PreSaleStockShowStatus         int `json:"pre_sale_stock_show_status"`
	MaximumPreSaleStock            int `json:"maximum_pre_sale_stock"`
	Version                        int `json:"version"`
	PreSaleEnableDays              int `json:"pre_sale_enable_days"`
	PreSaleStockReopenIntervalDate int `json:"pre_sale_stock_reopen_interval_date"`
}

// ParseTime 解析 TEMU 时间格式（毫秒时间戳）
func ParseTime(timeStr string) (*time.Time, error) {
	if timeStr == "" {
		return nil, nil
	}
	// TEMU 时间格式: "1742177867237" (毫秒时间戳)
	var timestamp int64
	if _, err := fmt.Sscanf(timeStr, "%d", &timestamp); err != nil {
		return nil, err
	}
	t := time.Unix(timestamp/1000, (timestamp%1000)*1000000)
	return &t, nil
}
