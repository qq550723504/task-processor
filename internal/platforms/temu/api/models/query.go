package models

// TextCheckRequest 文本检查请求
type TextCheckRequest struct {
	Content string `json:"content"`
	Type    int    `json:"type"`
}

// TextCheckResponse 文本检查响应
type TextCheckResponse struct {
	Success   bool `json:"success"`
	ErrorCode int  `json:"error_code"`
	Result    struct {
		Success bool `json:"success"`
	} `json:"result"`
}

// TemplateQueryRequest 模板查询请求
type TemplateQueryRequest struct {
	CatID int `json:"cat_id"`
}

// TemplateQueryResponse 模板查询响应
type TemplateQueryResponse struct {
	Success   bool `json:"success"`
	ErrorCode int  `json:"error_code"`
	Result    struct {
		Templates []TemplateInfo `json:"templates"`
	} `json:"result"`
}

// TemplateInfo 模板信息
type TemplateInfo struct {
	TemplateID   int    `json:"template_id"`
	TemplateName string `json:"template_name"`
	CategoryID   int    `json:"category_id"`
}

// SpecQueryRequest 规格查询请求
type SpecQueryRequest struct {
	GoodsID       string   `json:"goods_id"`
	ChildSpecName string   `json:"child_spec_name"`
	ParentSpecID  string   `json:"parent_spec_id"`
	ExistSpecList []string `json:"exist_spec_list"`
}

// SpecQueryResponse 规格查询响应
type SpecQueryResponse struct {
	Success   bool             `json:"success"`
	ErrorCode int              `json:"error_code"`
	Result    *SpecQueryResult `json:"result"`
}

// SpecQueryResult 规格查询结果
type SpecQueryResult struct {
	SpecID string `json:"spec_id"`
}

// SkuSnCheckRequest SKU编码检查请求
type SkuSnCheckRequest struct {
	GoodsID   string         `json:"goods_id"`
	OutSnList []OutSkuSnItem `json:"out_sn_list"`
}

// OutSkuSnItem SKU编码项
type OutSkuSnItem struct {
	OutSkuSn string `json:"out_sku_sn"`
}

// SkuSnCheckResponse SKU编码检查响应
type SkuSnCheckResponse struct {
	Success   bool                   `json:"success"`
	ErrorCode int                    `json:"error_code"`
	Result    *OutGoodsSnCheckResult `json:"result,omitempty"`
	Message   string                 `json:"error_msg,omitempty"`
}

// OutGoodsSnCheckResult SKU编码检查结果
type OutGoodsSnCheckResult struct {
	FailList []OutSkuSnFailItem `json:"fail_list"`
}

// OutSkuSnFailItem 失败的SKU编码项
type OutSkuSnFailItem struct {
	OutSkuSn    string `json:"out_sku_sn"`
	UsedGoodsID string `json:"used_goods_id"`
	UsedSkuID   string `json:"used_sku_id"`
	FailReason  string `json:"fail_reason"`
}

// CostTemplateRequest 成本模板查询请求
type CostTemplateRequest struct {
	ListingCommitID      string `json:"listing_commit_id"`
	GoodsCommitID        string `json:"goods_commit_id"`
	GoodsID              string `json:"goods_id"`
	CatID                int    `json:"cat_id"`
	ListingCommitVersion string `json:"listing_commit_version"`
	ClickType            string `json:"click_type"`
	QueryAll             bool   `json:"query_all"`
}

// CostTemplateResponse 成本模板查询响应
type CostTemplateResponse struct {
	Success   bool                `json:"success"`
	ErrorCode int                 `json:"error_code"`
	Result    *CostTemplateResult `json:"result,omitempty"`
	Message   string              `json:"message,omitempty"`
}

// CostTemplateResult 成本模板结果数据
type CostTemplateResult struct {
	CostTemplateList []CostTemplateItem `json:"cost_template_list"`
	CostTemplateURL  string             `json:"cost_template_url"`
}

// CostTemplateItem 成本模板项
type CostTemplateItem struct {
	CostTemplateID  string `json:"cost_template_id"`
	TemplateName    string `json:"template_name"`
	Disabled        bool   `json:"disabled"`
	DefaultTemplate bool   `json:"default_template"`
}

// CommitDetailRequest 提交详情查询请求
type CommitDetailRequest struct {
	ListingCommitID      string `json:"listing_commit_id"`
	GoodsCommitID        string `json:"goods_commit_id"`
	GoodsID              string `json:"goods_id"`
	ListingCommitVersion string `json:"listing_commit_version"`
	ClickType            string `json:"click_type"`
}

// CommitDetailResponse 提交详情查询响应
type CommitDetailResponse struct {
	Success   bool                `json:"success"`
	ErrorCode int                 `json:"error_code,omitempty"`
	Message   string              `json:"message,omitempty"`
	Result    *CommitDetailResult `json:"result,omitempty"`
}

// CommitDetailResult 提交详情结果数据
type CommitDetailResult struct {
	GoodsBasic            *CommitDetailGoodsBasic     `json:"goods_basic,omitempty"`
	GoodsSaleInfo         *CommitDetailGoodsSaleInfo  `json:"goods_sale_info,omitempty"`
	GoodsServicePromise   *CommitDetailServicePromise `json:"goods_service_promise,omitempty"`
	GoodsExtensionInfo    *CommitDetailExtensionInfo  `json:"goods_extension_info,omitempty"`
	Extra                 *CommitDetailExtra          `json:"extra,omitempty"`
	CanSave               bool                        `json:"can_save"`
	SupportMaxRetailPrice bool                        `json:"support_max_retail_price"`
	PlatformExpressBill   bool                        `json:"platform_express_bill"`
}

// CommitDetailGoodsBasic 商品基础信息
type CommitDetailGoodsBasic struct {
	GoodsID                 string                          `json:"goods_id"`
	ListingCommitID         string                          `json:"listing_commit_id"`
	ListingCommitVersion    string                          `json:"listing_commit_version"`
	GoodsName               string                          `json:"goods_name"`
	GoodsCreateTime         int64                           `json:"goods_create_time"`
	GoodsCommitID           string                          `json:"goods_commit_id"`
	Lang                    string                          `json:"lang"`
	AllowSite               []int                           `json:"allow_site"`
	CatID                   int                             `json:"cat_id"`
	CatIDs                  []int                           `json:"cat_ids"`
	CategoryTree            *CommitDetailCategoryTree       `json:"category_tree,omitempty"`
	CategoryDisclaimer      *CommitDetailCategoryDisclaimer `json:"category_disclaimer,omitempty"`
	GoodsType               int                             `json:"goods_type"`
	HdThumbURL              string                          `json:"hd_thumb_url"`
	GoodsGallery            map[string]interface{}          `json:"goods_gallery,omitempty"`
	IsOnSale                int                             `json:"is_on_sale"`
	CatType                 int                             `json:"cat_type"`
	IsClothes               bool                            `json:"is_clothes"`
	IsBooks                 bool                            `json:"is_books"`
	CanSkipRequiredProperty bool                            `json:"can_skip_required_property"`
	IsShop                  bool                            `json:"is_shop"`
	FromCopy                bool                            `json:"from_copy"`
	HasSubmitted            bool                            `json:"has_submitted"`
	Source                  int                             `json:"source"`
	OutGoodsSn              string                          `json:"out_goods_sn"`
	ListPriceRequired       bool                            `json:"list_price_required"`
	ListPriceDocuments      bool                            `json:"list_price_documents"`
	NeedAccessoryInfo       bool                            `json:"need_accessory_info"`
	AccessoryInfoRequired   bool                            `json:"accessory_info_required"`
	Customized              bool                            `json:"customized"`
	SecondHand              bool                            `json:"second_hand"`
	SupportCustomizedGoods  bool                            `json:"support_customized_goods"`
	RecommendURLPrice       bool                            `json:"recommend_url_price"`
	AgreeMaxRetailPrice     bool                            `json:"agree_max_retail_price"`
	CanEditSecondHand       bool                            `json:"can_edit_second_hand"`
	MadeToOrder             bool                            `json:"made_to_order"`
}

// CommitDetailCategoryTree 分类树结构
type CommitDetailCategoryTree struct {
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
	Cate5ID      int      `json:"cate5_id"`
	Cate5Name    string   `json:"cate5_name"`
	Cate6ID      int      `json:"cate6_id"`
	Cate6Name    string   `json:"cate6_name"`
	CateNameList []string `json:"cate_name_list"`
}

// CommitDetailCategoryDisclaimer 分类免责声明
type CommitDetailCategoryDisclaimer struct {
	PromptList []string `json:"prompt_list"`
}

// CommitDetailGoodsSaleInfo 商品销售信息
type CommitDetailGoodsSaleInfo struct {
	GoodsPattern int `json:"goods_pattern"`
}

// CommitDetailServicePromise 服务承诺
type CommitDetailServicePromise struct {
	// 根据实际数据结构添加字段
}

// CommitDetailExtensionInfo 扩展信息
type CommitDetailExtensionInfo struct {
	// 根据实际数据结构添加字段
}

// CommitDetailExtra 额外信息
type CommitDetailExtra struct {
	Tab             int `json:"tab"`
	MinSkuImageSize int `json:"min_sku_image_size"`
	MaxSkuImageSize int `json:"max_sku_image_size"`
}

// PriceQueryRequest 价格查询请求结构体
type PriceQueryRequest struct {
	GoodsID                      string                    `json:"goods_id"`
	MmsSkuMaxRetailPriceQryItems []MaxRetailPriceQueryItem `json:"mms_sku_max_retail_price_qry_items"`
}

// MaxRetailPriceQueryItem 最大零售价查询项
type MaxRetailPriceQueryItem struct {
	BasePriceStr string `json:"base_price_str"`
	Currency     string `json:"currency"`
}

// PriceQueryResponse 价格查询响应结构体
type PriceQueryResponse struct {
	Success   bool              `json:"success"`
	ErrorCode int               `json:"error_code"`
	ErrorMsg  string            `json:"error_msg,omitempty"`
	Result    *PriceQueryResult `json:"result,omitempty"`
}

// PriceQueryResult 价格查询结果
type PriceQueryResult struct {
	MmsSkuMaxRetailPriceItems []MaxRetailPriceResultItem `json:"mms_sku_max_retail_price_items"`
}

// MaxRetailPriceResultItem 最大零售价结果项
type MaxRetailPriceResultItem struct {
	BasePriceStr        string `json:"base_price_str"`
	Currency            string `json:"currency"`
	MaxRetailPriceStr   string `json:"max_retail_price_str"`
	RetailPriceCurrency string `json:"retail_price_currency"`
}
