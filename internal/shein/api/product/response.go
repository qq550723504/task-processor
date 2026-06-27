// Package product 产品响应数据结构
package product

// SheinResponse 通用响应
type SheinResponse struct {
	Code string       `json:"code"`
	Msg  string       `json:"msg"`
	Info ResponseInfo `json:"info"`
	BBL  any          `json:"bbl"`
}

// ResponseInfo 响应信息
type ResponseInfo struct {
	Success        bool             `json:"success"`
	SPUName        string           `json:"spu_name"`
	SKCList        []ResponseSKC    `json:"skc_list"`
	Version        string           `json:"version"`
	PreValidResult []PreValidResult `json:"pre_valid_result"`
	MCCValidResult any              `json:"mcc_valid_result"`
	Extra          struct{}         `json:"extra"`
}

// ResponseSKC 响应SKC信息
type ResponseSKC struct {
	SKCName string        `json:"skc_name"`
	SKUList []ResponseSKU `json:"sku_list"`
}

// ResponseSKU 响应SKU信息
type ResponseSKU struct {
	SKUCode     string `json:"sku_code"`
	SupplierSKU string `json:"supplier_sku"`
}

// RecordResponse 产品记录响应
type RecordResponse struct {
	Code string `json:"code"`
	Msg  string `json:"msg"`
	Info struct {
		Data []RecordItem `json:"data"`
		Meta struct {
			Count     int `json:"count"`
			CustomObj struct {
				ScrollID string `json:"scroll_id"`
			} `json:"customObj"`
		} `json:"meta"`
	} `json:"info"`
	BBL int `json:"bbl"`
}

// RecordItem 产品记录项
type RecordItem struct {
	RecordID              string   `json:"record_id"`
	DocType               string   `json:"doc_type"`
	Version               string   `json:"version"`
	SpuName               string   `json:"spu_name"`
	SkcName               string   `json:"skc_name"`
	ProductName           string   `json:"product_name"`
	ProductEnName         string   `json:"product_en_name"`
	BrandCode             string   `json:"brand_code"`
	BrandName             string   `json:"brand_name"`
	ChangeTagList         []string `json:"change_tag_list"`
	AuditState            int      `json:"audit_state"`
	State                 int      `json:"state"`
	Operator              string   `json:"operator"`
	EditType              int      `json:"edit_type"`
	CreateTime            string   `json:"create_time"`
	Auditor               string   `json:"auditor"`
	AuditDate             string   `json:"audit_date"`
	ProductNameMulti      string   `json:"product_name_multi"`
	SupplierCode          string   `json:"supplier_code"`
	MainImageThumbnailURL string   `json:"main_image_thumbnail_url"`
	SaleName              string   `json:"sale_name"`
	AppealRecord          bool     `json:"appeal_record"`
	HasJudgeResult        bool     `json:"has_judge_result"`
	IsEmbryo              bool     `json:"is_embryo"`
	TimeOut               bool     `json:"time_out"`
	DiscussType           int      `json:"discuss_type"`
	HasCategoryAuthority  bool     `json:"has_category_authority"`
	SubmitEntry           string   `json:"submit_entry"`
}

// PartInfoResponse 产品部件信息响应
type PartInfoResponse struct {
	Data []PartInfo `json:"data"`
	Meta struct {
		Count     int `json:"count"`
		CustomObj any `json:"customObj"`
	} `json:"meta"`
}

// PartInfo 产品部件信息
type PartInfo struct {
	PartID          int           `json:"part_id"`
	PartName        string        `json:"part_name"`
	ProductTypeList []ProductType `json:"product_type_list"`
}

// ProductType 产品类型信息
type ProductType struct {
	ProductTypeID     int    `json:"product_type_id"`
	ProductTypeName   string `json:"product_type_name"`
	ProductTypeCNName string `json:"product_type_cn_name"`
}

// ProductResponse 产品信息响应
type ProductResponse struct {
	Code string             `json:"code"`
	Msg  string             `json:"msg"`
	Info ProductInfoWrapper `json:"info"`
	BBL  any                `json:"bbl"`
}

// ProductInfoWrapper 产品信息包装器
type ProductInfoWrapper struct {
	Product *Product `json:"product"`
}

// ConfirmPublishResponse 确认发布响应
type ConfirmPublishResponse struct {
	Code string      `json:"code"`
	Msg  string      `json:"msg"`
	Info ConfirmInfo `json:"info"`
	BBL  any         `json:"bbl"`
}

// ConfirmInfo 确认信息
type ConfirmInfo struct {
	Data ConfirmData `json:"data"`
}

// ConfirmData 确认数据
type ConfirmData struct {
	NeedConfirm      bool `json:"need_confirm"`
	SimPriceInfoList any  `json:"sim_price_info_list"`
}

// ProductListResponse 产品列表响应
type ProductListResponse struct {
	Code string `json:"code"`
	Msg  string `json:"msg"`
	Info struct {
		Data []ProductListItem `json:"data"`
		Meta struct {
			Count int `json:"count"`
		} `json:"meta"`
	} `json:"info"`
	BBL any `json:"bbl"`
}

// BrandListResponse 品牌列表响应
type BrandListResponse struct {
	Code string `json:"code"`
	Msg  string `json:"msg"`
	Info struct {
		Data []BrandItem `json:"data"`
		Meta struct {
			Count     int `json:"count"`
			CustomObj any `json:"customObj"`
		} `json:"meta"`
	} `json:"info"`
	BBL any `json:"bbl"`
}

// BrandItem 品牌项
type BrandItem struct {
	BrandCode   string `json:"brand_code"`
	BrandName   string `json:"brand_name"`
	BrandNameEn string `json:"brand_name_en"`
}

// ProductListItem 产品列表项
type ProductListItem struct {
	SpuName          string        `json:"spu_name"`
	SpuCode          string        `json:"spu_code"`
	CategoryID       int64         `json:"category_id"`
	BrandCode        string        `json:"brand_code"`
	BrandName        string        `json:"brand_name"`
	ProductNameCh    string        `json:"product_name_ch"`
	ProductNameEn    string        `json:"product_name_en"`
	ProductNameMulti string        `json:"product_name_multi"`
	SkcInfoList      []SkcInfoItem `json:"skc_info_list"`
	ShelfStatus      string        `json:"shelf_status"`
	CreateTime       string        `json:"create_time"`
	PublishTime      string        `json:"publish_time"`
	FirstShelfTime   string        `json:"first_shelf_time"`
	ExpectShelfTime  *string       `json:"expect_shelf_time"`
	TagInfoList      []any         `json:"tag_info_list"`
}

// SkcInfoItem SKC 信息项
type SkcInfoItem struct {
	SkcName               string    `json:"skc_name"`
	SkcCode               string    `json:"skc_code"`
	SaleName              string    `json:"sale_name"`
	MainImageThumbnailURL string    `json:"main_image_thumbnail_url"`
	SupplierCode          string    `json:"supplier_code"`
	BusinessModel         int       `json:"business_model"`
	IsSaleAttribute       int       `json:"is_sale_attribute"`
	SupplierID            int64     `json:"supplier_id"`
	SkuInfo               []SkuInfo `json:"sku_info"`
	MallSellStatus        int       `json:"mall_sell_status"`
	Abandoned             bool      `json:"abandoned"`
	TagInfoList           []any     `json:"tag_info_list"`
	ShelfFailReason       *string   `json:"shelf_fail_reason"`
	HasOriginalImage      bool      `json:"has_original_image"`
}

// SkuInfo SKU 信息项
type SkuInfo struct {
	SkuCode     string `json:"sku_code"`
	SupplierSKU string `json:"supplier_sku"`
}

// PreValidResult 预验证结果
type PreValidResult struct {
	Form                    string                     `json:"form"`
	FormName                string                     `json:"form_name"`
	Messages                []string                   `json:"messages"`
	Module                  string                     `json:"module"`
	OtherLanguageMessageMap map[string][]string        `json:"other_language_message_map"`
	SkcErrorMessageMap      map[string]SkcErrorMessage `json:"skc_error_message_map"`
}

// SkcErrorMessage SKC错误信息
type SkcErrorMessage struct {
	Messages                []string            `json:"messages"`
	OtherLanguageMessageMap map[string][]string `json:"otherLanguageMessageMap"`
}
