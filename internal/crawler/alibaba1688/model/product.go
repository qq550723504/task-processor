// Package model 定义1688平台特定的数据模型
package model

import (
	"time"
)

// Product1688 1688产品信息
type Product1688 struct {
	// 基础信息
	ID        string   `json:"id"`        // 商品ID
	Title     string   `json:"title"`     // 商品标题
	URL       string   `json:"url"`       // 商品链接
	Images    []string `json:"images"`    // 商品图片列表
	MainImage string   `json:"mainImage"` // 主图
	Videos    []Video  `json:"videos"`    // 商品视频列表

	// 价格信息（1688特有的阶梯价格）
	PriceRanges []PriceRange `json:"priceRanges"` // 价格阶梯
	MinPrice    float64      `json:"minPrice"`    // 最低价格
	MaxPrice    float64      `json:"maxPrice"`    // 最高价格
	Currency    string       `json:"currency"`    // 货币单位，通常是CNY

	// 起订量信息
	MinOrderQuantity int    `json:"minOrderQuantity"` // 最小起订量
	Unit             string `json:"unit"`             // 单位（件、个、套等）

	// 供应商信息
	Supplier SupplierInfo `json:"supplier"` // 供应商信息

	// 商品规格和详情
	Specifications   []Specification  `json:"specifications"`   // 基础规格参数
	ProductDetails   []ProductDetail  `json:"productDetails"`   // 商品详情
	PackInfo         *PackInfo        `json:"packInfo"`         // 产品包装信息
	VariationsValues []VariationValue `json:"variationsValues"` // 变体值
	Variants         []Variant        `json:"variants"`         // 商品变体（颜色、尺寸等）

	// 库存和销售信息
	SalesVolume int     `json:"salesVolume"` // 销量
	ReviewCount int     `json:"reviewCount"` // 评价数量
	Rating      float64 `json:"rating"`      // 评分

	// 物流信息
	ShippingInfo ShippingInfo `json:"shippingInfo"` // 物流信息

	// 其他信息
	Category     string   `json:"category"`     // 商品分类
	Brand        string   `json:"brand"`        // 品牌
	Keywords     []string `json:"keywords"`     // 关键词
	IsCustomized bool     `json:"isCustomized"` // 是否支持定制

	// 元数据
	CrawledAt time.Time `json:"crawledAt"` // 爬取时间
	UpdatedAt time.Time `json:"updatedAt"` // 更新时间
}

// Video 商品视频信息
type Video struct {
	VideoID  int64  `json:"videoId"`  // 视频ID
	Title    string `json:"title"`    // 视频标题
	VideoURL string `json:"videoUrl"` // 视频链接
	CoverURL string `json:"coverUrl"` // 视频封面
	State    int    `json:"state"`    // 视频状态
}

// ProductDetail 商品详情
type ProductDetail struct {
	Section string   `json:"section"` // 详情分类（如产品特点、使用说明等）
	Content string   `json:"content"` // 详情内容
	Images  []string `json:"images"`  // 详情图片
}

// PriceRange 价格阶梯
type PriceRange struct {
	MinQuantity int     `json:"minQuantity"` // 最小数量
	MaxQuantity int     `json:"maxQuantity"` // 最大数量（-1表示无上限）
	Price       float64 `json:"price"`       // 单价
}

// SupplierInfo 供应商信息
type SupplierInfo struct {
	ID              string  `json:"id"`              // 供应商ID
	Name            string  `json:"name"`            // 供应商名称
	CompanyName     string  `json:"companyName"`     // 公司名称
	Location        string  `json:"location"`        // 所在地区
	ShopURL         string  `json:"shopUrl"`         // 店铺地址
	CardType        string  `json:"cardType"`        // 商家类型（factory/trade等）
	YearsInBusiness int     `json:"yearsInBusiness"` // 经营年限
	Rating          float64 `json:"rating"`          // 供应商评分
	ResponseRate    float64 `json:"responseRate"`    // 响应率
	IsGoldSupplier  bool    `json:"isGoldSupplier"`  // 是否金牌供应商
	IsVerified      bool    `json:"isVerified"`      // 是否实名认证
}

// Specification 商品规格参数
type Specification struct {
	Name  string `json:"name"`  // 规格名称
	Value string `json:"value"` // 规格值
}

// VariationValue 变体值（维度名称及其可选值）
type VariationValue struct {
	VariantName string   `json:"variant_name"` // 维度名称（如 Color, Size）
	Values      []string `json:"values"`       // 该维度的所有可选值
}

// Variant 商品变体
type Variant struct {
	Attributes map[string]any `json:"attributes,omitempty"`
	Name       string         `json:"name"`  // 变体名称（如颜色、尺寸）
	Image      string         `json:"image"` // 选项图片
	Stock      int            `json:"stock"` // 该选项库存
	Price      float64        `json:"price"` // 该选项价格
}

// ShippingInfo 物流信息
type ShippingInfo struct {
	ShippingFrom    string           `json:"shippingFrom"`    // 发货地
	ShippingMethods []ShippingMethod `json:"shippingMethods"` // 配送方式
	ProcessingTime  string           `json:"processingTime"`  // 处理时间
	IsFreeShipping  bool             `json:"isFreeShipping"`  // 是否包邮
}

// ShippingMethod 配送方式
type ShippingMethod struct {
	Name         string  `json:"name"`         // 配送方式名称
	Cost         float64 `json:"cost"`         // 配送费用
	DeliveryTime string  `json:"deliveryTime"` // 配送时间
}

// ProductPackInfo 产品包装信息
type PackInfo struct {
	PackageType     string             `json:"packageType"`     // 包装类型
	PackageSize     *PackageDimensions `json:"packageSize"`     // 包装尺寸
	Weight          float64            `json:"weight"`          // 重量（克）
	PackageImages   []string           `json:"packageImages"`   // 包装图片
	PackageContents []string           `json:"packageContents"` // 包装内容清单
	Instructions    string             `json:"instructions"`    // 包装说明
}

// PackageDimensions 包装尺寸
type PackageDimensions struct {
	Length float64 `json:"length"` // 长度（厘米）
	Width  float64 `json:"width"`  // 宽度（厘米）
	Height float64 `json:"height"` // 高度（厘米）
	Unit   string  `json:"unit"`   // 单位
}

// Product1688Request 1688产品请求
type Product1688Request struct {
	URL       string            `json:"url"`       // 产品URL
	Options   map[string]string `json:"options"`   // 额外选项
	RequestID string            `json:"requestId"` // 请求ID
}

// Product1688Result 1688产品处理结果
type Product1688Result struct {
	Request   Product1688Request `json:"request"`   // 原始请求
	Product   *Product1688       `json:"product"`   // 产品信息
	Error     error              `json:"error"`     // 错误信息
	Duration  time.Duration      `json:"duration"`  // 处理耗时
	Timestamp time.Time          `json:"timestamp"` // 处理时间
}
