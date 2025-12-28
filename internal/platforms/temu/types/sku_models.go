// Package types 提供TEMU平台的SKU相关数据结构定义
package types

// Skc SKC商品结构体
type Skc struct {
	SkuList []Sku `json:"sku_list"`
	// 以下字段在提交时可选，主要用于内部管理
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
	// 必需字段
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

	// 可选字段（用于内部管理）
	MarketPrice         int    `json:"market_price,omitempty"`     // 市场价（分），供货价*2
	MarketPriceStr      string `json:"market_price_str,omitempty"` // 市场价字符串（元），供货价*2
	MaxRetailPrice      int    `json:"max_retail_price,omitempty"`
	Price               int    `json:"price,omitempty"`
	PriceStr            string `json:"price_str,omitempty"`
	Priority            int    `json:"priority,omitempty"`
	RetailPriceCurrency string `json:"retail_price_currency,omitempty"`
	SkuComplete         bool   `json:"sku_complete,omitempty"`
	SkuDeleted          bool   `json:"sku_deleted,omitempty"`
}

// SpecInfo 规格信息
type SpecInfo struct {
	SpecID         string `json:"spec_id"`
	SpecName       string `json:"spec_name"`
	ParentSpecID   string `json:"parent_spec_id"`
	ParentSpecName string `json:"parent_spec_name,omitempty"`
	ParentID       string `json:"parent_id,omitempty"`
}

// SkuPriceDocument SKU价格文档
type SkuPriceDocument struct {
	Category1FileList []string `json:"category1_file_list"`
	Category2FileList []string `json:"category2_file_list"`
}

// BatchSkuInfo 批量SKU信息
type BatchSkuInfo struct {
	ProductExpressInfo ProductExpressInfo `json:"product_express_info"`
	SupplierPriceStr   string             `json:"supplier_price_str"`
	Quantity           string             `json:"quantity"`
	MultiplePackage    MultiplePackage    `json:"multiple_package"`
	OutSkuSN           string             `json:"out_sku_sn"`
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
