// Package types 提供TEMU平台的产品数据结构定义
package types

// Product TEMU商品提交请求结构体
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

// ProductSaveResult 产品保存结果
type ProductSaveResult struct {
	GoodsID              *int `json:"goods_id"`
	ListingCommitID      int  `json:"listing_commit_id"`
	ListingCommitVersion int  `json:"listing_commit_version"`
	GoodsCommitID        int  `json:"goods_commit_id"`
}

// SourceProduct 源产品数据
type SourceProduct struct {
	ID          string                 `json:"id"`
	Title       string                 `json:"title"`
	Description string                 `json:"description"`
	Images      []string               `json:"images"`
	Price       float64                `json:"price"`
	Currency    string                 `json:"currency"`
	Attributes  map[string]interface{} `json:"attributes"`
}

// Extra 额外信息（用于提交请求）
type Extra struct {
	Tab              int  `json:"tab"`
	MinSkuImageSize  int  `json:"min_sku_image_size"`
	MaxSkuImageSize  int  `json:"max_sku_image_size"`
	CreateEmptyGoods bool `json:"create_empty_goods"`
}

// 为了兼容性，添加类型别名
type GoodsBasic = GoodsBasicInfo
type GoodsExtensionInfo = ExtensionInfo
type GoodsServicePromise = ServicePromise
