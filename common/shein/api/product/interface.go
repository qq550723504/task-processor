package product

import (
	"task-processor/common/shein/api/attribute"
	"time"
)

// ProductAPI 产品相关API接口
type ProductAPI interface {
	GetProduct(productID string) (*Product, error)

	UpdateProduct(product *Product) error

	DeleteProduct(productID string) error

	GetPartInfo(categoryID int) (*PartInfoResponse, error)

	// SaveDraftProduct 保存产品草稿
	SaveDraftProduct(product *Product) (*SheinResponse, string, error)

	// PublishProduct 发布产品
	PublishProduct(product *Product) (*SheinResponse, string, error)

	// ConfirmPublish 确认发布
	ConfirmPublish(product *Product) (bool, string, error)

	Record(request *ProductRecordRequest) (*RecordResponse, error)

	// ListProducts 获取产品列表
	ListProducts(pageNum, pageSize int, request *ProductListRequest) (*ProductListResponse, error)
}

type RecordResponse struct {
	Code string `json:"code"`
	Msg  string `json:"msg"`
	Info struct {
		Data []struct {
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
		} `json:"data"`
		Meta struct {
			Count     int `json:"count"`
			CustomObj struct {
				ScrollID string `json:"scroll_id"`
			} `json:"customObj"`
		} `json:"meta"`
	} `json:"info"`
	BBL int `json:"bbl"`
}

type ProductRecordRequest struct {
	Language                  string    `json:"language"`
	OnlyCurrentMonthRecommend bool      `json:"only_current_month_recommend"`
	OnlySpmbCopyProduct       bool      `json:"only_spmb_copy_product"`
	QueryTimeOut              bool      `json:"query_time_out"`
	QueryState                *int      `json:"query_state"`
	SearchDiyCustom           bool      `json:"search_diy_custom"`
	SupplierCodeList          *[]string `json:"supplier_code_list"`
	SupplierCodeSearchType    int       `json:"supplier_code_search_type"`
}

type SheinResponse struct {
	Code string       `json:"code"`
	Msg  string       `json:"msg"`
	Info ResponseInfo `json:"info"`
	BBL  interface{}  `json:"bbl"`
}

// ResponseInfo 响应信息
type ResponseInfo struct {
	Success        bool          `json:"success"`
	SPUName        string        `json:"spu_name"`
	SKCList        []ResponseSKC `json:"skc_list"`
	Version        string        `json:"version"`
	PreValidResult interface{}   `json:"pre_valid_result"`
	MCCValidResult interface{}   `json:"mcc_valid_result"`
	Extra          struct{}      `json:"extra"`
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

// ProductType 产品类型信息
type ProductType struct {
	ProductTypeID     int    `json:"product_type_id"`
	ProductTypeName   string `json:"product_type_name"`
	ProductTypeCNName string `json:"product_type_cn_name"`
}

// PartInfo 产品部件信息
type PartInfo struct {
	PartID          int           `json:"part_id"`
	PartName        string        `json:"part_name"`
	ProductTypeList []ProductType `json:"product_type_list"`
}

// PartInfoResponse 产品部件信息响应
type PartInfoResponse struct {
	Data []PartInfo `json:"data"`
	Meta struct {
		Count     int         `json:"count"`
		CustomObj interface{} `json:"customObj"`
	} `json:"meta"`
}

type Product struct {
	// 基本信息
	BrandCode          *string  `json:"brand_code"`
	BrandSeriesList    []string `json:"brand_series_list"`
	CategoryID         int      `json:"category_id"`
	CategoryIDList     []int    `json:"category_id_list"`
	CategoryMisplace   bool     `json:"category_misplace"`
	CommodityType      int      `json:"commodity_type"`
	ConfirmSizeImg     bool     `json:"confirm_size_img"`
	InputSizeChartFlag int      `json:"input_size_chart_flag"`
	IsSuggestCategory  bool     `json:"is_suggest_category"`
	PointKey           string   `json:"point_key"`
	ProductTypeID      *int     `json:"product_type_id"`
	SourceSystem       string   `json:"source_system"`
	SPPRelateSPUName   string   `json:"spp_relate_spu_name"`
	SPUName            string   `json:"spu_name"`
	SuitFlag           int      `json:"suit_flag"`
	SupplierCode       string   `json:"supplier_code"`
	TopCategoryID      int      `json:"top_category_id"`
	Invalid            *string  `json:"invalid"`

	// 证书相关
	ProductCertificateList    []int    `json:"product_certificate_list"`
	CertificateList           []int    `json:"certificate_list"`
	DelOtherCertificateSNList []string `json:"del_other_certificate_sn_list"`

	// 图片信息
	ImageInfo *ImageInfo `json:"image_info,omitempty"`

	// 多语言信息
	MultiLanguageDescList             []LanguageContent `json:"multi_language_desc_list"`
	MultiLanguageNameList             []LanguageContent `json:"multi_language_name_list"`
	MultiLanguageMakeupIngredientList []any             `json:"multi_language_makeup_ingredient_list"`

	// 产品属性
	ProductAttributeList     []ProductAttribute `json:"product_attribute_list"`
	ProductNameAttributeList []interface{}      `json:"product_name_attribute_list"`

	// 视频信息
	ProductVideoList []ProductVideo `json:"product_video_list"`
	PartInfoList     []any          `json:"part_info_list"`
	PLMPatternIDList []any          `json:"plm_pattern_id_list"`
	// 站点信息
	SiteList         []SiteInfo `json:"site_list"`
	PopPriceSiteInfo *string    `json:"pop_price_site_info"`

	// SKC列表
	SKCList           []SKC           `json:"skc_list"`
	SizeAttributeList []SizeAttribute `json:"size_attribute_list"`
	SampleSkuBackSize *string         `json:"sample_sku_back_size,omitempty"`

	// 其他属性
	BackSizeAttributeList   []interface{}                       `json:"back_size_attribute_list"`
	CustomAttributeRelation []attribute.CustomAttributeRelation `json:"custom_attribute_relation"`
	Extra                   Extra                               `json:"extra"`
}

type Extra struct {
	BiddingID                   string            `json:"bidding_id"`
	BestSaleSPU                 string            `json:"best_sale_spu"`
	ActivityType                string            `json:"activity_type"`
	BiddingKey                  string            `json:"bidding_key"`
	SubjectSKC                  string            `json:"subject_skc"`
	BestSaleAssociateKey        string            `json:"best_sale_associate_key"`
	SwitchToSPUPic              bool              `json:"switch_to_spu_pic"`
	FromPageID                  *string           `json:"from_page_id"`
	OptimizeSKCName             *string           `json:"optimize_skc_name"`
	SPUTag                      []string          `json:"spu_tag"`
	TransformCVSizeImage        bool              `json:"transformCvSizeImage"`
	UseCVTransformImage         bool              `json:"useCvTransformImage"`
	ConfirmVolumeSKU            []string          `json:"confirm_volume_sku"`
	ConfirmWeightSKU            []string          `json:"confirm_weight_sku"`
	ControlPriceData            map[string]string `json:"control_price_data"`
	FaddishPriceInfo            struct{}          `json:"faddish_price_info"`
	BestSaleAddColorSize        string            `json:"best_sale_add_color_size"`
	CopySKCConfirmUpdated       bool              `json:"copy_skc_confirm_updated"`
	ExtraClearBiddingOrBestSale bool              `json:"extra_clear_bidding_or_best_sale"`
	IsSpmbCopyProduct           bool              `json:"is_spmb_copy_product"`
	ArmorToken                  string            `json:"armor_token"`
	BlackBox                    string            `json:"black_box"`
}

type SizeAttribute struct {
	AttributeID                int    `json:"attribute_id"`
	AttributeExtraValue        string `json:"attribute_extra_value"`
	RelateSaleAttributeID      int    `json:"relate_sale_attribute_id"`
	RelateSaleAttributeValueID int    `json:"relate_sale_attribute_value_id"`
}

// LanguageContent 多语言内容
type LanguageContent struct {
	Language string `json:"language"`
	Name     string `json:"name"`
}

// ProductAttribute 产品属性
type ProductAttribute struct {
	AttributeID         int    `json:"attribute_id"`
	AttributeValueID    *int   `json:"attribute_value_id"`
	CVSuggestType       string `json:"cv_suggest_type"`
	AttributeExtraValue string `json:"attribute_extra_value"`
}

// ProductVideo 产品视频
type ProductVideo struct {
	Sites         []string `json:"sites"`
	URL           *string  `json:"url"`
	OriginFileURL string   `json:"origin_file_url"`
}

// SiteInfo 站点信息
type SiteInfo struct {
	MainSite    string   `json:"main_site"`
	SubSiteList []string `json:"sub_site_list"`
}

// SKC 库存单位
type SKC struct {
	FromPreProduct          *string               `json:"from_pre_product"`
	SaleAttribute           SaleAttribute         `json:"sale_attribute"`
	SKUS                    []SKU                 `json:"sku_list"`
	ImageInfo               ImageInfo             `json:"image_info,omitempty"`
	IsMaternalStyle         *bool                 `json:"is_maternal_style"`
	MultiLanguageName       LanguageContent       `json:"multi_language_name"`
	MultiLanguageNameList   []LanguageContent     `json:"multi_language_name_list"`
	SiteDetailImageInfoList []SiteDetailImageInfo `json:"site_detail_image_info_list"`
	SppRelateSkcName        *string               `json:"spp_relate_skc_name"`
	HopeOnSaleDate          string                `json:"hope_on_sale_date"`
	ShelfWay                int                   `json:"shelf_way"`
	ShelfRequire            int                   `json:"shelf_require"`
	ProofOfStockList        []interface{}         `json:"proof_of_stock_list"`
	Sort                    int                   `json:"sort"`
	PartCodeRelList         *interface{}          `json:"part_code_rel_list"`
	PriceFillingTips        *string               `json:"price_filling_tips"`
	SKCMaterial             *interface{}          `json:"skc_material"`
	SiteSpecImageInfoList   []interface{}         `json:"site_spec_image_info_list"`
	SKCScopeAttributeList   []interface{}         `json:"skc_scope_attribute_list"`
	SampleInfo              *interface{}          `json:"sample_info"`
	IsFirstPublish          bool                  `json:"is_first_publish"`
	QualitySN               *string               `json:"quality_sn"`
	ReQuoteFileList         *[]interface{}        `json:"re_quote_file_list"`
	ReQuoteReason           *string               `json:"re_quote_reason"`
	SupplierCode            *string               `json:"supplier_code"`
	Extra                   SkcExtra              `json:"extra"`
}

type SkcExtra struct {
	IsSourceEmbryo             bool `json:"isSourceEmbryo"`
	IsEmbryoInclude            bool `json:"isEmbryoInclude"`
	PhysicalPriceReviewSuccess bool `json:"physical_price_review_success"`
}

// SaleAttribute 销售属性
type SaleAttribute struct {
	AttributeID        int  `json:"attribute_id"`
	AttributeValueID   int  `json:"attribute_value_id"`
	PreFillSpec        bool `json:"pre_fill_spec"`
	IsSPPSaleAttribute bool `json:"is_spp_sale_attribute"`
}

// SKU 具体规格
type SKU struct {
	SaleAttributeList        []SaleAttribute `json:"sale_attribute_list"`
	SppRelateSkuCode         *string         `json:"spp_relate_sku_code"`
	CostInfo                 *CostInfo       `json:"cost_info"`
	Height                   string          `json:"height"`
	Length                   string          `json:"length"`
	Width                    string          `json:"width"`
	LengthUnit               string          `json:"length_unit"`
	StockInfoList            []StockInfo     `json:"stock_info_list"`
	Weight                   float64         `json:"weight"`
	WeightUnit               string          `json:"weight_unit"`
	StockCount               *int            `json:"stock_count"`
	SupplierSKU              string          `json:"supplier_sku"`
	StopPurchase             int             `json:"stop_purchase"`
	MallState                int             `json:"mall_state"`
	PriceInfoList            []PriceInfo     `json:"price_info_list"`
	ImageInfo                *ImageInfo      `json:"image_info"`
	CompetingProductLink     *any            `json:"competing_product_link"`
	CompetingCostPriceImages []any           `json:"competing_cost_price_images"`
	SuggestedRetailPrice     *any            `json:"suggested_retail_price"`
	QuantityInfo             *QuantityInfo   `json:"quantity_info"`
	Extra                    SkuExtra        `json:"extra"`
}

type QuantityInfo struct {
	Quantity     *int `json:"quantity"`
	QuantityType *int `json:"quantity_type"`
	QuantityUnit *int `json:"quantity_unit"`
}

type SkuExtra struct {
	FieldDisabledInfo FieldDisabledInfo `json:"fieldDisabledInfo"`
}

type FieldDisabledInfo struct {
	SkuQuantity     *int `json:"skuQuantity"`
	SkuQuantityType *int `json:"skuQuantityType"`
	SkuQuantityUnit *int `json:"skuQuantityUnit"`
}

// CostInfo 成本信息
type CostInfo struct {
	CostPrice string `json:"cost_price"`
	Currency  string `json:"currency"`
}

// PriceInfo 价格信息
type PriceInfo struct {
	SubSite      string   `json:"sub_site"`
	BasePrice    float64  `json:"base_price"`
	SpecialPrice *float64 `json:"special_price"`
	Currency     string   `json:"currency"`
}

// StockInfo 库存信息
type StockInfo struct {
	InventoryNum          int    `json:"inventory_num"`
	MerchantWarehouseCode string `json:"merchant_warehouse_code"`
}

// ImageInfo 图片信息
type ImageInfo struct {
	ImageGroupCode        *string        `json:"image_group_code"`
	ImageInfoList         []ImageDetail  `json:"image_info_list,omitempty"`
	OriginalImageInfoList *[]interface{} `json:"original_image_info_list,omitempty"`
}

// ImageDetail 图片详情
type ImageDetail struct {
	ImageType             int      `json:"image_type"`
	ImageSort             int      `json:"image_sort"`
	ImageURL              string   `json:"image_url"`
	ImageItemID           *string  `json:"image_item_id"`
	SizeImgFlag           bool     `json:"size_img_flag"`
	TransformCVSizeImage  bool     `json:"transformCvSizeImage"`
	AISStatus             int      `json:"ai_status"`
	PSTypes               []string `json:"ps_types"`
	MarketingMainImage    bool     `json:"marketing_main_image"`
	CommodityCategoryFlag *string  `json:"commodity_category_flag"`
}

// SiteDetailImageInfo 站点详情图片信息
type SiteDetailImageInfo struct {
	SiteAbbrList   []string      `json:"site_abbr_list"`
	ImageGroupCode *string       `json:"image_group_code"`
	ImageInfoList  []DetailImage `json:"image_info_list"`
}

// DetailImage 详情图片
type DetailImage struct {
	ImageSort   int     `json:"image_sort"`
	ImageURL    string  `json:"image_url"`
	ImageItemID *string `json:"image_item_id"`
}

// ProductListing 产品上架记录
type ProductListing struct {
	SupplierCode string    `json:"supplier_code"`
	SpuName      string    `json:"spu_name"` // Shein平台上的产品ID
	Status       string    `json:"status"`   // pending, success, failed
	Error        string    `json:"error"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// ProductInfoWrapper 产品信息包装器
type ProductInfoWrapper struct {
	Product *Product `json:"product"`
}

// ProductResponse 产品信息响应
type ProductResponse struct {
	Code string             `json:"code"`
	Msg  string             `json:"msg"`
	Info ProductInfoWrapper `json:"info"`
	BBL  interface{}        `json:"bbl"`
}

// ConfirmData 确认数据
type ConfirmData struct {
	NeedConfirm      bool        `json:"need_confirm"`
	SimPriceInfoList interface{} `json:"sim_price_info_list"`
}

// ConfirmInfo 确认信息
type ConfirmInfo struct {
	Data ConfirmData `json:"data"`
}

// ConfirmPublishResponse 确认发布响应
type ConfirmPublishResponse struct {
	Code string      `json:"code"`
	Msg  string      `json:"msg"`
	Info ConfirmInfo `json:"info"`
	BBL  interface{} `json:"bbl"`
}

// ProductListRequest 产品列表请求
type ProductListRequest struct {
	Language             string `json:"language"`
	OnlyRecommendResell  bool   `json:"only_recommend_resell"`
	OnlySpmbCopyProduct  bool   `json:"only_spmb_copy_product"`
	SearchAbandonProduct bool   `json:"search_abandon_product"`
	SearchIllegal        bool   `json:"search_illegal"`
	SearchLessInventory  bool   `json:"search_less_inventory"`
	ShelfType            string `json:"shelf_type"` // ON_SHELF, OFF_SHELF
	SortType             int    `json:"sort_type"`
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
	BBL interface{} `json:"bbl"`
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
	TagInfoList      []interface{} `json:"tag_info_list"`
}

// SkcInfoItem SKC 信息项
type SkcInfoItem struct {
	SkcName               string        `json:"skc_name"`
	SkcCode               string        `json:"skc_code"`
	SaleName              string        `json:"sale_name"`
	MainImageThumbnailURL string        `json:"main_image_thumbnail_url"`
	SupplierCode          string        `json:"supplier_code"`
	BusinessModel         int           `json:"business_model"`
	IsSaleAttribute       int           `json:"is_sale_attribute"`
	SupplierID            int64         `json:"supplier_id"`
	SkuInfo               []SkuInfo     `json:"sku_info"`
	MallSellStatus        int           `json:"mall_sell_status"`
	Abandoned             bool          `json:"abandoned"`
	TagInfoList           []interface{} `json:"tag_info_list"`
	ShelfFailReason       *string       `json:"shelf_fail_reason"`
	HasOriginalImage      bool          `json:"has_original_image"`
}

// SkuInfoItem SKU 信息项
type SkuInfo struct {
	SkuCode string `json:"sku_code"`
}
