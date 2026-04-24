package template

// ListParams 是已从 SDS 前端 bundle 和真实请求中确认的分页参数。
type ListParams struct {
	Page               int
	Size               int
	Keyword            string
	CategoryID         string
	Sort               string
	SortField          string
	SortType           string
	BestStatus         *int
	HotSellStatus      *int
	OnSaleStatus       *int
	NewStatus          *int
	PublicStatus       *int
	MemberLevel        string
	ProductSupplyChain string
	PreciseSearch      string
	ShipmentArea       string
	OverseasArea       string
	SideActiveID       string
	IsOverseas         string
	Timestamp          int64
}

// ListResponse 是 `/products/page` 的分页结构。
type ListResponse struct {
	TotalCount int              `json:"totalCount"`
	Page       int              `json:"page"`
	Size       int              `json:"size"`
	Items      []ProductSummary `json:"items"`
}

// ProductSummary 表示 SDS 的父商品或子 SKU。
// 列表接口里父商品和子 SKU 结构高度重合，因此这里保留共用字段，
// 通过 ParentID / ParentSKU / Subproducts / DesignPrototype 区分层级。
type ProductSummary struct {
	ID                          int64               `json:"id"`
	ParentID                    int64               `json:"parent_id"`
	Name                        string              `json:"name"`
	SKU                         string              `json:"sku"`
	ParentSKU                   string              `json:"parentSku"`
	EnglishName                 string              `json:"english_name"`
	DeclarationName             string              `json:"declaration_name"`
	DeclarationEnglishName      string              `json:"declaration_english_name"`
	PrototypeType               string              `json:"prototypeType"`
	PrototypeID                 int64               `json:"prototypeId"`
	PrototypeSKU                string              `json:"prototype_sku"`
	CurrentPrice                float64             `json:"currentPrice"`
	OriginalPrice               float64             `json:"originalPrice"`
	OnePieceCurrentPrice        float64             `json:"onePieceCurrentPrice"`
	MinPrice                    float64             `json:"min_price"`
	UnitPrice                   float64             `json:"unit_price"`
	BlankDesignURL              string              `json:"blankDesignUrl"`
	ThumbImgURL                 string              `json:"thumbImgUrl"`
	ImgURL                      string              `json:"img_url"`
	PSDImgURL                   string              `json:"psd_img_url"`
	ShowImg                     string              `json:"show_img"`
	MaterialColor               string              `json:"materialColor"`
	Remark                      string              `json:"remark"`
	Price                       string              `json:"price"`
	Size                        string              `json:"size"`
	SizeRemark                  string              `json:"sizeRemark"`
	SizeID                      int64               `json:"sizeId"`
	SizeDTO                     SizeDTO             `json:"sizeDto"`
	SizeStrList                 []string            `json:"sizeStrList"`
	Color                       ColorInfo           `json:"color"`
	ColorID                     int64               `json:"colorId"`
	ColorName                   string              `json:"color_name"`
	ColorBlock                  string              `json:"color_block"`
	ColorStr                    []string            `json:"color_str"`
	Weight                      float64             `json:"weight"`
	WeightMin                   float64             `json:"weightMin"`
	WeightMax                   float64             `json:"weightMax"`
	MinWeight                   float64             `json:"minWeight"`
	BoxLength                   float64             `json:"box_length"`
	BoxWidth                    float64             `json:"box_width"`
	BoxHeight                   float64             `json:"box_height"`
	ProductionCycle             int                 `json:"productionCycle"`
	ProductionCycleMin          int                 `json:"production_cycle_min"`
	ProductionCycleMax          int                 `json:"production_cycle_max"`
	SmallOrderProductionCycle   int                 `json:"smallOrderProductionCycle"`
	OnePieceSupplyChainStatus   string              `json:"onePieceSupplyChainStatus"`
	SmallOrderSupplyChainStatus string              `json:"smallOrderSupplyChainStatus"`
	SmallOrderPrice             string              `json:"smallOrderPrice"`
	AmazonCustom                int                 `json:"amazonCustom"`
	DesignStatus                int                 `json:"design_status"`
	PublicStatus                int                 `json:"public_status"`
	Status                      int                 `json:"status"`
	MemberLevel                 float64             `json:"memberLevel"`
	AccumulateLevel             float64             `json:"accumulateLevel"`
	HotSellStatus               int                 `json:"hotSellStatus"`
	RecommendStatus             int                 `json:"recommend_status"`
	NewStatus                   int                 `json:"new_status"`
	BestStatus                  int                 `json:"best_status"`
	BargainStatus               int                 `json:"bargain_status"`
	OnSaleStatus                int                 `json:"on_sale_status"`
	OnSaleValidStatus           int                 `json:"on_sale_valid_status"`
	OnSalePrice                 float64             `json:"on_sale_price"`
	AddStatus                   int                 `json:"add_status"`
	AttributeSort               string              `json:"attribute_sort"`
	SizeSort                    int                 `json:"size_sort"`
	ColorSort                   int                 `json:"color_sort"`
	PreferentialLabel           int                 `json:"preferentialLabel"`
	TenantID                    int64               `json:"tenantId"`
	UserID                      int64               `json:"userId"`
	CategoryID                  int64               `json:"category_id"`
	TextureID                   int64               `json:"texture_id"`
	IsDistribution              int                 `json:"isDistribution"`
	IsElectricity               int                 `json:"isElectricity"`
	OfflineType                 int                 `json:"offlineType"`
	UpdateDay                   int                 `json:"update_day"`
	CreatedTime                 int64               `json:"createdTime"`
	UpdateTime                  int64               `json:"updateTime"`
	Categories                  []Category          `json:"categories"`
	Texture                     Texture             `json:"texture"`
	ProductDetails              ProductDetails      `json:"product_details"`
	PlatformPrice               []PriceTier         `json:"platform_price"`
	ProductDeliveryCycles       []ProductCycleGroup `json:"productDeliveryCycles"`
	IssuingBayArea              IssuingBayArea      `json:"issuingBayArea"`
	DesignPrototype             *DesignPrototype    `json:"designPrototype"`
	Subproducts                 *Subproducts        `json:"subproducts"`
	Supply                      SupplySummary       `json:"supply"`
	ProductAvailableCartonViews []CartonPreview     `json:"productAvailableCartonPreviews"`
}

// ProductDetail 表示 `/products/{id}` 的详情结构。
type ProductDetail struct {
	ProductSummary
	IsOfficial                     string `json:"isOfficial"`
	SystemTime                     int64  `json:"systemTime"`
	MerchantType                   string `json:"merchantType"`
	ProductAvailableCartonPreviews []any  `json:"productAvailableCartonPreviews"`
}

// ProductDetails 保存 SDS 详情页里的扩展字段。
type ProductDetails struct {
	ID                     int64  `json:"id"`
	ProductID              int64  `json:"product_id"`
	Reminder               string `json:"reminder"`
	ProductionProcess      string `json:"production_process"`
	MaterialDescription    string `json:"material_description"`
	ProductPerformance     string `json:"product_performance"`
	ApplicableScenarios    string `json:"applicable_scenarios"`
	WashingInstructions    string `json:"washing_instructions"`
	SpecialDescription     string `json:"special_description"`
	DesignExplanation      string `json:"design_explanation"`
	DesignArea             string `json:"design_area"`
	PictureRequest         string `json:"picture_request"`
	ProductSize            string `json:"product_size"`
	PackagingSpecification string `json:"packaging_specification"`
}

// Category 是分类节点。
type Category struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	ParentID int64  `json:"parent_id"`
}

// Texture 是材质信息。
type Texture struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// SizeDTO 是尺码对象。
type SizeDTO struct {
	ID       int64  `json:"id"`
	SizeName string `json:"sizeName"`
}

// ColorInfo 是颜色对象。
type ColorInfo struct {
	ColorSort   int    `json:"colorSort"`
	ChineseName string `json:"chineseName"`
	Color       string `json:"color"`
	Opacity     int    `json:"opacity"`
	ColorName   string `json:"color_name"`
	ColorID     int64  `json:"colorId"`
}

// PriceTier 是阶梯价格。
type PriceTier struct {
	Num   int     `json:"num"`
	Price float64 `json:"price"`
}

// ProductCycleGroup 表示不同供应链的生产周期。
type ProductCycleGroup struct {
	Type           string      `json:"type"`
	DeliveryCycles []CycleItem `json:"deliveryCycles"`
}

// CycleItem 表示单个生产周期配置。
type CycleItem struct {
	Num   int `json:"num"`
	Cycle int `json:"cycle"`
}

// CartonPreview 保留产品包装预览对象。
type CartonPreview struct {
	ImageURL string `json:"imageUrl"`
}

// Subproducts 保存规格维度和具体 SKU。
type Subproducts struct {
	Attributers []VariantAttribute `json:"attributers"`
	Items       []ProductSummary   `json:"items"`
}

// VariantAttribute 是尺码维度和可选颜色。
type VariantAttribute struct {
	Size   string      `json:"size"`
	SizeID int64       `json:"sizeId"`
	Colors []ColorInfo `json:"colors"`
}

// DesignPrototype 是 SDS 设计模板原型。
type DesignPrototype struct {
	PrototypeID           string                 `json:"prototypeId"`
	PrototypeGroupID      int64                  `json:"prototypeGroupId"`
	ProductID             int64                  `json:"productId"`
	ProductParentID       int64                  `json:"productParentId"`
	Name                  string                 `json:"name"`
	BuildType             string                 `json:"buildType"`
	DesignScope           bool                   `json:"designScope"`
	DetailImgURLs         []DetailImage          `json:"detailImgUrls"`
	PrototypeResultGroups []PrototypeResultGroup `json:"prototypeResultGroups"`
	PrototypeLayerList    []PrototypeLayer       `json:"prototypeLayerList"`
}

// DetailImage 是详情图素材。
type DetailImage struct {
	ID          string `json:"id"`
	ImageURL    string `json:"imageUrl"`
	PrototypeID string `json:"prototypeId"`
	Sort        int    `json:"sort"`
}

// PrototypeResultGroup 是模板预览结果图。
type PrototypeResultGroup struct {
	ID             string `json:"id"`
	ResultImage    string `json:"resultImage"`
	Sort           int    `json:"sort"`
	PrototypeID    string `json:"prototypeId"`
	FaceSheetState bool   `json:"faceSheetState"`
}

// PrototypeLayer 是可设计图层定义。
type PrototypeLayer struct {
	ID             string  `json:"id"`
	PrototypeID    string  `json:"prototypeId"`
	Name           string  `json:"name"`
	Type           int     `json:"type"`
	Height         float64 `json:"height"`
	Width          float64 `json:"width"`
	PrintHeight    float64 `json:"printHeight"`
	PrintWidth     float64 `json:"printWidth"`
	IsMasterMap    int     `json:"isMasterMap"`
	MaskURL        string  `json:"maskUrl"`
	MaskShowURL    string  `json:"maskShowUrl"`
	ThumbnailURL   string  `json:"thumbnailUrl"`
	ImageURL       string  `json:"imageUrl"`
	PageType       string  `json:"pageType"`
	ChineseTitle   string  `json:"chineseTitle"`
	EnglishTitle   string  `json:"englishTitle"`
	FontGroup      string  `json:"fontGroup"`
	PreFilterGroup string  `json:"preFilterGroup"`
	IsMustDesign   int     `json:"isMustDesign"`
	WordNumLimit   int     `json:"wordNumLimit"`
	FileNumLimit   int     `json:"fileNumLimit"`
	UpdateDate     string  `json:"updateDate"`
	CreateDate     string  `json:"createDate"`
}

// IssuingBayArea 是发货仓区域。
type IssuingBayArea struct {
	ID          int64  `json:"id"`
	Type        string `json:"type"`
	Name        string `json:"name"`
	Address     string `json:"address"`
	IconURL     string `json:"iconUrl"`
	CountryCode string `json:"countryCode"`
}

// SupplySummary 是供应链聚合信息。
type SupplySummary struct {
	Items              []SupplyItem `json:"items"`
	TotalCount         int          `json:"total_count"`
	ProductionCycleMin int          `json:"production_cycle_min"`
	ProductionCycleMax int          `json:"production_cycle_max"`
	PriceMin           float64      `json:"price_min"`
	PriceMax           float64      `json:"price_max"`
}

// SupplyItem 是单个供应链记录。
type SupplyItem struct {
	ID                 int64       `json:"id"`
	ProductID          int64       `json:"productId"`
	ProductParentID    int64       `json:"productParentId"`
	SKU                string      `json:"sku"`
	Code               string      `json:"code"`
	SupplyChainType    string      `json:"supplyChainType"`
	SupplyPrice        float64     `json:"supply_price"`
	DistributionPct    int         `json:"distribution_percent"`
	ProductionCycleMin int         `json:"production_cycle_min"`
	ProductionCycleMax int         `json:"production_cycle_max"`
	LadderPrice        []PriceTier `json:"ladderPrice"`
}

// OptionGroups 表示 `/products/pageOptionGroup` 的最小结构。
type OptionGroups struct {
	ShipmentCountryList []ShipmentCountry `json:"shipmentCountryList"`
}

// OptionGroupParams 表示 `/products/pageOptionGroup` 的查询参数。
type OptionGroupParams struct {
	Size          int
	Page          int
	PreciseSearch int
	ShipmentArea  string
	OverseasArea  string
	Timestamp     int64
}

// ShipmentCountry 表示可发货国家统计。
type ShipmentCountry struct {
	CountryCode string `json:"countryCode"`
	Country     string `json:"country"`
	Num         int    `json:"num"`
	Icon        string `json:"icon"`
}

// CycleInfo 表示生产周期信息。
type CycleInfo struct {
	CurrentCycle    int `json:"currentCycle"`
	AverageCycle    int `json:"averageCycle"`
	ProductionCycle int `json:"productionCycle"`
}
