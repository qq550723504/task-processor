// Package types 提供 TEMU API 各子包共用的数据类型
package types

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
