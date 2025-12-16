// Package amazon 提供Amazon产品数据结构定义
package amazon

import "task-processor/internal/amazon/model"

// ProductData 产品数据
type ProductData struct {
	SKU          string
	Title        string
	Brand        string
	Price        float64
	Currency     string
	Condition    string
	Quantity     int
	Description  string
	BulletPoints []string
	ProductType  string // 产品类型
	// 外部产品标识符配置
	IdentifierConfig *model.ProductIdentifierConfig
	// 价格相关
	PriceExcludingTax float64 // 不含税价格
	TaxRate           float64 // 税率（百分比）
	// 图片相关
	MainImageURL     string   // 主图URL
	AdditionalImages []string // 附加图片URLs
	// SEO优化相关
	SearchTerms    []string // 搜索关键词
	TargetAudience string   // 目标受众
	// 产品详细信息
	Manufacturer string            // 制造商
	ModelNumber  string            // 型号
	Dimensions   map[string]string // 尺寸信息
	Weight       string            // 重量
	Materials    []string          // 材质
	// 变体信息（用于创建父子产品关系）
	VariationData *VariationData // 变体数据
	// 合规性信息
	SafetyWarnings  []string // 安全警告
	AgeRestriction  string   // 年龄限制
	CountryOfOrigin string   // 原产国
	// 动态属性
	Attributes map[string]any
}

// isAutomotiveCategory 判断是否为汽配类目（保留用于产品标识符处理）
func isAutomotiveCategory(productType string) bool {
	automotiveTypes := map[string]bool{
		"AUTO_ACCESSORY": true,
		"AUTO_PART":      true,
		"AUTOMOTIVE":     true,
	}
	return automotiveTypes[productType]
}
