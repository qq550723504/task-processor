package product

import "encoding/json"

// Product TEMU商品结构体
type Product struct {
	CanSave                *bool          `json:"can_save"`
	Extra                  ExtraInfo      `json:"extra"`
	GoodsBasic             GoodsBasicInfo `json:"goods_basic"`
	GoodsSaleInfo          GoodsSaleInfo  `json:"goods_sale_info"`
	GoodsServicePromise    ServicePromise `json:"goods_service_promise"`
	GoodsExtensionInfo     ExtensionInfo  `json:"goods_extension_info"`
	PlatformExpressBill    *bool          `json:"platform_express_bill"`
	SupportMaxRetailPrice  *bool          `json:"support_max_retail_price"`
	ReplicateToRelateGoods *bool          `json:"replicate_to_relate_goods"`
	SkcList                []Skc          `json:"skc_list"`
	BatchSkuInfo           BatchSkuInfo   `json:"batch_sku_info"`
}

// GoodsBasicInfo 商品基本信息
type GoodsBasicInfo struct {
	GoodsID                 string       `json:"goods_id"`
	ListingCommitID         string       `json:"listing_commit_id"`
	ListingCommitVersion    string       `json:"listing_commit_version"`
	GoodsName               string       `json:"goods_name"`
	GoodsCreateTime         int64        `json:"goods_create_time"`
	GoodsCommitID           string       `json:"goods_commit_id"`
	Lang                    string       `json:"lang"`
	AllowSite               []int        `json:"allow_site"`
	CatID                   int          `json:"cat_id"`
	CatIDs                  []int        `json:"cat_ids"`
	CategoryTree            CategoryTree `json:"category_tree"`
	CategoryDisclaimer      Disclaimer   `json:"category_disclaimer"`
	GoodsType               int          `json:"goods_type"`
	HdThumbURL              string       `json:"hd_thumb_url"`
	GoodsGallery            GoodsGallery `json:"goods_gallery,omitempty"`
	IsOnSale                int          `json:"is_on_sale"`
	CatType                 int          `json:"cat_type"`
	IsClothes               bool         `json:"is_clothes"`
	IsBooks                 bool         `json:"is_books"`
	CanSkipRequiredProperty bool         `json:"can_skip_required_property"`
	IsShop                  bool         `json:"is_shop"`
	FromCopy                bool         `json:"from_copy"`
	HasSubmitted            bool         `json:"has_submitted"`
	Source                  int          `json:"source"`
	OutGoodsSN              string       `json:"out_goods_sn"`
	ListPriceRequired       bool         `json:"list_price_required"`
	ListPriceDocuments      bool         `json:"list_price_documents"`
	NeedAccessoryInfo       bool         `json:"need_accessory_info"`
	AccessoryInfoRequired   bool         `json:"accessory_info_required"`
	Customized              bool         `json:"customized"`
	SecondHand              bool         `json:"second_hand"`
	SupportCustomizedGoods  bool         `json:"support_customized_goods"`
	RecommendURLPrice       bool         `json:"recommend_url_price"`
	AgreeMaxRetailPrice     bool         `json:"agree_max_retail_price"`
	CanEditSecondHand       bool         `json:"can_edit_second_hand"`
	MadeToOrder             bool         `json:"made_to_order"`
}

// GoodsSaleInfo 商品销售信息
type GoodsSaleInfo struct {
	GoodsPattern int `json:"goods_pattern"`
}

// ServicePromise 服务承诺
type ServicePromise struct {
	ShipmentLimitSecond int    `json:"shipment_limit_second"`
	FulfillmentType     int    `json:"fulfillment_type"`
	CostTemplateID      string `json:"cost_template_id"`
}

// ExtraInfo 额外信息
type ExtraInfo struct {
	CreateEmptyGoods bool `json:"create_empty_goods"`
	VersionType      any  `json:"version_type"`
	Tab              int  `json:"tab"`
	MinSkuImageSize  int  `json:"min_sku_image_size"`
	MaxSkuImageSize  int  `json:"max_sku_image_size"`
	CurrentTab       int  `json:"current_tab"`
}

// CategoryTree 分类树
type CategoryTree struct {
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
	Cate5ID      int      `json:"cate5_id,omitempty"`
	Cate5Name    string   `json:"cate5_name,omitempty"`
	CateNameList []string `json:"cate_name_list"`
}

// Disclaimer 免责声明
type Disclaimer struct {
	PromptList []string `json:"prompt_list"`
}

// GoodsGallery 商品图库
type GoodsGallery struct {
	DetailImage   []ImageInfo `json:"detail_image"`
	CarouselVideo []any       `json:"carousel_video"`
	DetailVideo   []any       `json:"detail_video"`
}

func (g GoodsGallery) MarshalJSON() ([]byte, error) {
	if len(g.DetailImage) == 0 && len(g.CarouselVideo) == 0 && len(g.DetailVideo) == 0 {
		return []byte("{}"), nil
	}
	type Alias GoodsGallery
	return json.Marshal(Alias(g))
}

// GoodsTrademark 商品商标
type GoodsTrademark struct {
	NotSelectBrand bool `json:"not_select_brand"`
}

// GoodsOriginInfo 商品原产地信息
type GoodsOriginInfo struct {
	OriginRegionName1 string `json:"origin_region_name1"`
}

// ProductSaveResult 产品保存结果（旧版兼容）
type ProductSaveResult struct {
	GoodsID              *int `json:"goods_id"`
	ListingCommitID      int  `json:"listing_commit_id"`
	ListingCommitVersion int  `json:"listing_commit_version"`
	GoodsCommitID        int  `json:"goods_commit_id"`
}
