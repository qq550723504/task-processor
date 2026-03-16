package shein

import (
	"task-processor/internal/domain/model"
	"task-processor/internal/shein/api/product"
	"time"
)

// SheinProductResponse SHEIN 产品列表响应
type SheinProductResponse struct {
	SpuName           string    `json:"spu_name"`
	SpuCode           string    `json:"spu_code"`
	CategoryID        int64     `json:"category_id"`
	BrandCode         string    `json:"brand_code"`
	BrandName         string    `json:"brand_name"`
	ProductNameCh     string    `json:"product_name_ch"`
	ProductNameEn     string    `json:"product_name_en"`
	ProductNameMulti  string    `json:"product_name_multi"`
	SkcInfoList       []SkcInfo `json:"skc_info_list"`
	ShelfStatus       string    `json:"shelf_status"` // ON_SHELF, OFF_SHELF
	CreateTime        string    `json:"create_time"`
	PublishTime       string    `json:"publish_time"`
	FirstShelfTime    string    `json:"first_shelf_time"`
	ExpectShelfTime   *string   `json:"expect_shelf_time"`
	TagInfoList       []any     `json:"tag_info_list"`
	DiagFlag          int       `json:"diag_flag"`
	CustomizeCategory bool      `json:"customize_category"`
}

// SkcInfo SKC 信息（颜色变体）
type SkcInfo struct {
	SkcName               string    `json:"skc_name"`
	SkcCode               string    `json:"skc_code"`
	SaleName              string    `json:"sale_name"` // 销售属性名称，如"灰色"、"米色"
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

// ParseTime 解析 SHEIN 时间格式
func ParseTime(timeStr string) (*time.Time, error) {
	if timeStr == "" {
		return nil, nil
	}
	// SHEIN 时间格式: "2025-11-25 11:07:09"
	// 使用 ParseInLocation 指定时区为 UTC
	t, err := time.ParseInLocation("2006-01-02 15:04:05", timeStr, time.UTC)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// SKUCreationParams SKU创建参数
type SKUCreationParams struct {
	ASIN              string
	ProductInfo       *model.Product
	WarehouseCode     string
	SaleAttributeList []product.SaleAttribute
	Variant           Variant
}

// SKCCreationParams SKC创建参数
type SKCCreationParams struct {
	AttributeID      int
	AttributeValueID int
	SKUS             []product.SKU
	ImageInfo        product.ImageInfo
	SupplierCode     string
	Sort             int
}

type SKUBuildRequest struct {
	SaleAttributeData ResultSaleAttribute
	Strategy          AttributeStrategy
	PrimaryAttrValue  string
	WarehouseCode     string
}
