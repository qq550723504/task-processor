package extractor

import (
	"task-processor/internal/domain/model"

	"github.com/playwright-community/playwright-go"
	"github.com/sirupsen/logrus"
)

// Extractor 提取器接口
type Extractor interface {
	Extract(page playwright.Page, product *model.Product) error
}

// CompositeExtractor 组合提取器
type CompositeExtractor struct {
	extractors    []Extractor
	errorDetector *ErrorDetector
}

// NewCompositeExtractor 创建组合提取器
func NewCompositeExtractor(marketplace string) *CompositeExtractor {
	return &CompositeExtractor{
		extractors: []Extractor{
			&TitleExtractor{},
			&AvailabilityExtractor{},       // 先提取可用性，价格提取器需要依赖这个信息
			NewPriceExtractor(marketplace), // 使用构造函数正确初始化
			&BrandExtractor{},
			&RatingExtractor{}, // 包含评分和评论数量提取
			&ImageExtractor{},
			NewVideoExtractor(),         // 视频提取器
			&CategoriesExtractor{},      // 分类提取器
			NewParentAsinExtractor(),    // Parent ASIN提取器
			&SellerExtractor{},          // 卖家提取器
			&ShipsFromExtractor{},       // 发货地提取器
			&DeliveryExtractor{},        // 配送信息提取器
			NewDescriptionExtractor(),   // 使用构造函数正确初始化
			&ProductDetailsExtractor{},  // 产品详情提取器
			NewVariationsExtractor(),    // 变体提取器
			NewBestsellerExtractor(),    // 畅销排名提取器
			NewFeatureParserExtractor(), // 特性解析提取器
			&FeaturesExtractor{},        // 基础特性提取器
		},
		errorDetector: NewErrorDetector(),
	}
}

// Extract 提取所有信息
func (ce *CompositeExtractor) Extract(page playwright.Page, product *model.Product) error {
	for _, extractor := range ce.extractors {
		if err := extractor.Extract(page, product); err != nil {
			logrus.Infof("提取器执行失败 (%T): %v", extractor, err)

			// 使用新的错误检测器
			if ce.errorDetector.IsCriticalError(err) {
				logrus.Infof("检测到关键错误，停止后续提取器执行: %v", err)
				return err
			}
		}
	}
	return nil
}
