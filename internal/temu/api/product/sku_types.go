package product

// ImageInfo 图片信息
type ImageInfo struct {
	Type   *int   `json:"type"`
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

// SpecInfo 规格信息
type SpecInfo struct {
	SpecID         string `json:"spec_id"`
	SpecName       string `json:"spec_name"`
	ParentSpecID   string `json:"parent_spec_id"`
	ParentSpecName string `json:"parent_spec_name,omitempty"`
	ParentID       string `json:"parent_id,omitempty"`
}

// MultiplePackage 多包装信息
type MultiplePackage struct {
	SkuClassification  int    `json:"sku_classification"`
	NumberOfPieces     int    `json:"number_of_pieces"`
	IndividuallyPacked int    `json:"individually_packed"`
	NumberOfPiecesNew  string `json:"number_of_pieces_new"`
	PieceUnitCode      int    `json:"piece_unit_code"`
	PieceNewUnitCode   int    `json:"piece_new_unit_code"`
}

// ProductExpressInfo 产品物流信息
type ProductExpressInfo struct {
	WeightInfo WeightInfo `json:"weight_info"`
	VolumeInfo VolumeInfo `json:"volume_info"`
}

// VolumeInfo 体积信息
type VolumeInfo struct {
	Height string `json:"height"`
	Length string `json:"length"`
	Width  string `json:"width"`
}

// WeightInfo 重量信息
type WeightInfo struct {
	Weight string `json:"weight"`
}

// Skc SKC商品结构体
type Skc struct {
	SkuList          []Sku       `json:"sku_list"`
	CarouselGallery  []ImageInfo `json:"carousel_gallery,omitempty"`
	ColorImageUrl    string      `json:"color_image_url,omitempty"`
	CommitDeleteType int         `json:"commit_delete_type,omitempty"`
	CommitDeleted    int         `json:"commit_deleted,omitempty"`
	Priority         int         `json:"priority,omitempty"`
	SkcComplete      bool        `json:"skc_complete,omitempty"`
	Spec             []SpecInfo  `json:"spec,omitempty"`
}

// Sku SKU商品结构体
type Sku struct {
	Spec                     []SpecInfo         `json:"spec"`
	Currency                 string             `json:"currency"`
	UseEstimateSupplierPrice bool               `json:"use_estimate_supplier_price"`
	DimensionGallery         []ImageInfo        `json:"dimension_gallery"`
	CarouselGallery          []ImageInfo        `json:"carousel_gallery"`
	FoodIngredientGallery    []ImageInfo        `json:"food_ingredient_gallery"`
	Quantity                 string             `json:"quantity"`
	ProductExpressInfo       ProductExpressInfo `json:"product_express_info"`
	SupplierPriceStr         string             `json:"supplier_price_str"`
	OutSkuSN                 string             `json:"out_sku_sn"`
	MultiplePackage          MultiplePackage    `json:"multiple_package"`
	OriginNetContentNumber   string             `json:"origin_net_content_number,omitempty"`
	NetContentUnitCode       int                `json:"net_content_unit_code,omitempty"`
	MaxRetailPriceStr        string             `json:"max_retail_price_str"`
	SupplierPrice            int                `json:"supplier_price"`
	SkuPriceDocuments        map[string]any     `json:"sku_price_documents"`
	MarketPrice              int                `json:"market_price,omitempty"`
	MarketPriceStr           string             `json:"market_price_str,omitempty"`
	MaxRetailPrice           int                `json:"max_retail_price,omitempty"`
	Price                    int                `json:"price,omitempty"`
	PriceStr                 string             `json:"price_str,omitempty"`
	Priority                 int                `json:"priority,omitempty"`
	RetailPriceCurrency      string             `json:"retail_price_currency,omitempty"`
	SkuComplete              bool               `json:"sku_complete,omitempty"`
	SkuDeleted               bool               `json:"sku_deleted,omitempty"`
}

// BatchSkuInfo 批量SKU信息
type BatchSkuInfo struct {
	ProductExpressInfo ProductExpressInfo `json:"product_express_info"`
	SupplierPriceStr   string             `json:"supplier_price_str"`
	Quantity           string             `json:"quantity"`
	MultiplePackage    MultiplePackage    `json:"multiple_package"`
	OutSkuSN           string             `json:"out_sku_sn"`
}
