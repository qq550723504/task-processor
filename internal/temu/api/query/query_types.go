package query

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
	GoodsBasic            *CommitDetailGoodsBasic    `json:"goods_basic,omitempty"`
	GoodsSaleInfo         *CommitDetailGoodsSaleInfo `json:"goods_sale_info,omitempty"`
	GoodsServicePromise   map[string]any             `json:"goods_service_promise,omitempty"`
	GoodsExtensionInfo    map[string]any             `json:"goods_extension_info,omitempty"`
	Extra                 *CommitDetailExtra         `json:"extra,omitempty"`
	CanSave               bool                       `json:"can_save"`
	SupportMaxRetailPrice bool                       `json:"support_max_retail_price"`
	PlatformExpressBill   bool                       `json:"platform_express_bill"`
}

// CommitDetailGoodsBasic 商品基础信息
type CommitDetailGoodsBasic struct {
	GoodsID              string                          `json:"goods_id"`
	ListingCommitID      string                          `json:"listing_commit_id"`
	ListingCommitVersion string                          `json:"listing_commit_version"`
	GoodsName            string                          `json:"goods_name"`
	GoodsCommitID        string                          `json:"goods_commit_id"`
	CatID                int                             `json:"cat_id"`
	CatIDs               []int                           `json:"cat_ids"`
	CategoryTree         *CommitDetailCategoryTree       `json:"category_tree,omitempty"`
	CategoryDisclaimer   *CommitDetailCategoryDisclaimer `json:"category_disclaimer,omitempty"`
	GoodsType            int                             `json:"goods_type"`
	IsClothes            bool                            `json:"is_clothes"`
	IsBooks              bool                            `json:"is_books"`
	Customized           bool                            `json:"customized"`
	SecondHand           bool                            `json:"second_hand"`
	MadeToOrder          bool                            `json:"made_to_order"`
	OutGoodsSn           string                          `json:"out_goods_sn"`
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

// CommitDetailExtra 额外信息
type CommitDetailExtra struct {
	Tab             int `json:"tab"`
	MinSkuImageSize int `json:"min_sku_image_size"`
	MaxSkuImageSize int `json:"max_sku_image_size"`
}

// SkuQueryRequest SKU查询请求
type SkuQueryRequest struct {
	CommitID             string `json:"commit_id"`
	GoodsID              string `json:"goods_id"`
	SourceTypeOfSkuQuery int    `json:"source_type_of_sku_query"`
	Source               int    `json:"source"`
}

// SkuQueryResponse SKU查询响应
type SkuQueryResponse struct {
	Success   bool `json:"success"`
	ErrorCode int  `json:"error_code"`
	Result    struct {
		Total   int              `json:"total"`
		SkuList []SkuQueryResult `json:"sku_list"`
	} `json:"result"`
}

// SkuQueryResult SKU查询结果项
type SkuQueryResult struct {
	GoodsName           string         `json:"goods_name"`
	SpecName            string         `json:"spec_name"`
	SpecList            []SkuSpecInfo  `json:"spec_list"`
	ThumbURL            string         `json:"thumb_url"`
	GoodsID             string         `json:"goods_id"`
	GoodsCommitID       string         `json:"goods_commit_id"`
	ListingCommitID     string         `json:"listing_commit_id"`
	MallID              string         `json:"mall_id"`
	SkuID               string         `json:"sku_id"`
	SkuSN               string         `json:"sku_sn"`
	Stock               int            `json:"stock"`
	Price               float64        `json:"price"`
	SupplierPrice       float64        `json:"supplier_price"`
	Currency            string         `json:"currency"`
	SkcID               string         `json:"skc_id"`
	OutGoodsSN          string         `json:"out_goods_sn"`
	ShippingMode        int            `json:"shipping_mode"`
	OrdinaryStock       int            `json:"ordinary_stock"`
	CheckPriceAuditInfo map[string]any `json:"check_price_audit_info"`
}

// SkuSpecInfo SKU规格信息
type SkuSpecInfo struct {
	SpecID         string `json:"spec_id"`
	SpecName       string `json:"spec_name"`
	ParentSpecName string `json:"parent_spec_name"`
}

// SkuQueryOptions SKU查询选项
type SkuQueryOptions struct {
	CommitID             string
	GoodsID              string
	SourceTypeOfSkuQuery int
	Source               int
}

// NewSkuQueryOptions 创建SKU查询选项
func NewSkuQueryOptions(commitID, goodsID string) SkuQueryOptions {
	return SkuQueryOptions{
		CommitID:             commitID,
		GoodsID:              goodsID,
		SourceTypeOfSkuQuery: 1,
		Source:               0,
	}
}
