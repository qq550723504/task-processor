// Package product 产品模型数据结构
package product

import (
	"task-processor/internal/shein/api/attribute"
	"time"
)

// Product 产品完整信息
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
	ProductNameAttributeList []any              `json:"product_name_attribute_list"`

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
	BackSizeAttributeList   []any                               `json:"back_size_attribute_list"`
	CustomAttributeRelation []attribute.CustomAttributeRelation `json:"custom_attribute_relation"`
	Extra                   Extra                               `json:"extra"`
}

// Extra 扩展信息
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

// SizeAttribute 尺寸属性
type SizeAttribute struct {
	AttributeID                int    `json:"attribute_id"`
	AttributeExtraValue        string `json:"attribute_extra_value"`
	RelateSaleAttributeID      int    `json:"relate_sale_attribute_id"`
	RelateSaleAttributeValueID int    `json:"relate_sale_attribute_value_id"`
}

// ProductListing 产品上架记录
type ProductListing struct {
	SupplierCode string    `json:"supplier_code"`
	SpuName      string    `json:"spu_name"`
	Status       string    `json:"status"`
	Error        string    `json:"error"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
