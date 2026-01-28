package pricing

type PricingAPI interface {
	BatchHandleCostDiscuss(reqBody *BatchHandleCostDiscussRequest) (*BatchHandleCostDiscussResponse, error)

	BargainPage(req *PageRequest, status int) (*BargainPageResponse, error)
}

type PageRequest struct {
	PageNum   int    `json:"page_num"`
	PageSize  int    `json:"page_size"`
	StartTime string `json:"start_time,omitempty"` // 开始时间，格式：2025-10-30 00:00:00
	EndTime   string `json:"end_time,omitempty"`   // 结束时间，格式：2026-01-30 23:59:59
}

type BatchHandleCostDiscussRequest struct {
	ConfirmInfos        *[]ConfirmInfo       `json:"confirm_infos"`
	CreateCostDiscusses *[]CreateCostDiscuss `json:"create_cost_discusses"`
}

type ConfirmInfo struct {
	DiscussAuditType int    `json:"discuss_audit_type"`
	DiscussSn        string `json:"discuss_sn"`
	DocumentSn       string `json:"document_sn"`
}

type CreateCostDiscuss struct {
	DiscussSn       string                   `json:"discuss_sn"`
	DiscussStep     int                      `json:"discuss_step"`
	DocumentSn      string                   `json:"document_sn"`
	SkcName         string                   `json:"skc_name"`
	Reason          string                   `json:"reason"`
	FileUploadList  []string                 `json:"file_upload_list,omitempty"`
	SkuCostInfoList []SkuCostInfoForReappeal `json:"sku_cost_info_list,omitempty"`
}

// SkuCostInfoForReappeal 重新议价时的SKU成本信息
type SkuCostInfoForReappeal struct {
	Cost         float64 `json:"cost"`
	Currency     string  `json:"currency"`
	LastCost     float64 `json:"last_cost"`
	LastCurrency string  `json:"last_currency"`
	SkuCode      string  `json:"sku_code"`
}

type BatchHandleCostDiscussResponse struct {
	Code string `json:"code"`
	Msg  string `json:"msg"`
	Info struct {
		SuccessCount int `json:"success_count"` // 成功数量
		FailCount    int `json:"fail_count"`    // 失败数量
	} `json:"info"`
	Data interface{} `json:"data"`
}

type BargainPageResponse struct {
	Code string `json:"code"`
	Msg  string `json:"msg"`
	Info struct {
		Data []BargainPageData `json:"data"`
		Meta BargainPageMeta   `json:"meta"`
	} `json:"info"`
	Bbl *string `json:"bbl"`
}

type BargainPageData struct {
	BargainSn             string                `json:"bargain_sn"`
	BargainStatus         int                   `json:"bargain_status"`
	SpuName               string                `json:"spu_name"`
	SkcName               string                `json:"skc_name"`
	DocumentSn            string                `json:"document_sn"`
	ReceiptSn             string                `json:"receipt_sn"`
	SupplierCode          string                `json:"supplier_code"`
	ProductTitle          string                `json:"product_title"`
	SaleAttributeValue    string                `json:"sale_attribute_value"`
	MainPicUrl            string                `json:"main_pic_url"`
	AllPicUrls            []string              `json:"all_pic_urls"`
	SerialNumber          int                   `json:"serial_number"`
	SkuCostPrices         []SkuCostPrice        `json:"sku_cost_prices"`
	SkuCostPriceHistories []SkuCostPriceHistory `json:"sku_cost_price_histories"`
	BargainType           int                   `json:"bargain_type"`
	AppealReason          string                `json:"appeal_reason"`
	BomType               int                   `json:"bom_type"`
	CompareProductLink    string                `json:"compare_product_link"`
	AppealCount           int                   `json:"appeal_count"`
	BomVersion            string                `json:"bom_version"`
	FileUploadList        string                `json:"file_upload_list"`
	Reason                string                `json:"reason"`
	IsSizeSamePrice       int                   `json:"is_size_same_price"`
	OutLowFlag            int                   `json:"out_low_flag"`
	ShowPrivilegeFlag     int                   `json:"show_privilege_flag"`
}

type SkuCostPrice struct {
	SkuCode                string             `json:"sku_code"`
	SaleAttributeValues    []string           `json:"sale_attribute_values"`
	CostPriceHistories     []CostPriceHistory `json:"cost_price_histories"`
	SuggestCostPrice       float64            `json:"suggest_cost_price"`
	SuggestCostCurrency    string             `json:"suggest_cost_currency"`
	CompareProductLink     string             `json:"compare_product_link"`
	LatestCostPrice        float64            `json:"latest_cost_price"`
	OutLowSuggestCostPrice float64            `json:"out_low_suggest_cost_price"`
	HasLowSuggestPrice     bool               `json:"has_low_suggest_price"`
	NoneLowSuggestPrice    bool               `json:"none_low_suggest_price"`
	CostLowSuggestPrice    bool               `json:"cost_low_suggest_price"`
}

type CostPriceHistory struct {
	SerialNumber int     `json:"serial_number"`
	CostPrice    float64 `json:"cost_price"`
	Currency     string  `json:"currency"`
}

type SkuCostPriceHistory struct {
	SerialNumber int     `json:"serial_number"`
	CostPrice    float64 `json:"cost_price"`
	Currency     string  `json:"currency"`
}

type BargainPageMeta struct {
	Count     int                    `json:"count"`
	CustomObj []BargainPageCustomObj `json:"customObj"`
}

type BargainPageCustomObj struct {
	BargainStatus     int    `json:"bargain_status"`
	BargainStatusDesc string `json:"bargain_status_desc"`
	TotalQuantity     int    `json:"total_quantity"`
}
