// Package service 提供业务逻辑层
package service

import (
	"context"
	"fmt"
	"log"

	"task-processor/common/amazon"
	amazonModel "task-processor/common/amazon/model"
	"task-processor/internal/model"
	"task-processor/internal/repo"
	"task-processor/internal/utils"
)

// CrawlerService 爬虫服务
type CrawlerService struct {
	configService *ConfigService
	fileRepo      *repo.FileRepository
	urlBuilder    *utils.URLBuilder
}

// NewCrawlerService 创建爬虫服务实例
func NewCrawlerService(
	configService *ConfigService,
	fileRepo *repo.FileRepository,
	urlBuilder *utils.URLBuilder,
) *CrawlerService {
	return &CrawlerService{
		configService: configService,
		fileRepo:      fileRepo,
		urlBuilder:    urlBuilder,
	}
}

// CrawlerRequest 爬虫请求参数
type CrawlerRequest struct {
	URL        string
	Zipcode    string
	Region     string
	Output     string
	ConfigFile string
}

// ProcessProduct 处理产品爬取
func (s *CrawlerService) ProcessProduct(ctx context.Context, req *CrawlerRequest) error {
	// 处理URL和邮编
	url, zipcode := s.processURLAndZipcode(req)

	// 加载配置
	cfg := s.configService.LoadConfig(req.ConfigFile)

	// 创建处理器
	processor := amazon.NewAmazonProcessor(&cfg.Amazon)
	defer processor.Shutdown()

	// 处理页面
	log.Printf("开始处理Amazon产品: %s", url)
	product, err := processor.Process(url, zipcode)
	if err != nil {
		return fmt.Errorf("处理页面失败: %w", err)
	}

	// 转换为内部模型
	internalProduct := s.convertToInternalModel(product)

	// 保存结果
	if err := s.fileRepo.SaveProduct(internalProduct, req.Output); err != nil {
		return fmt.Errorf("保存文件失败: %w", err)
	}

	log.Printf("成功保存结果到: %s", req.Output)
	log.Printf("产品标题: %s", internalProduct.Title)
	log.Printf("产品价格: %.2f %s", internalProduct.FinalPrice, internalProduct.Currency)

	return nil
}

// processURLAndZipcode 处理URL和邮编
func (s *CrawlerService) processURLAndZipcode(req *CrawlerRequest) (string, string) {
	url := req.URL
	zipcode := req.Zipcode

	// 如果没有提供URL，构建默认URL
	if url == "" {
		url = s.urlBuilder.BuildDefaultURL(req.Region)
	}

	// 如果没有提供邮编，使用默认邮编
	if zipcode == "" {
		zipcode = utils.GetDefaultZipcode(req.Region)
	}

	return url, zipcode
}

// convertToInternalModel 转换为内部模型
func (s *CrawlerService) convertToInternalModel(product *amazonModel.Product) *model.Product {
	// 转换复杂类型字段
	var productDescription []model.Description
	for _, desc := range product.ProductDescription {
		productDescription = append(productDescription, model.Description{
			Text: desc.Text,
			Type: desc.Type,
			URL:  desc.URL,
		})
	}

	var variations []model.Variation
	for _, variation := range product.Variations {
		variations = append(variations, model.Variation{
			Name:       variation.Name,
			Asin:       variation.Asin,
			Price:      variation.Price,
			Currency:   variation.Currency,
			Image:      variation.Image,
			Attributes: variation.Attributes,
		})
	}

	var variationsValues []model.VariationValue
	for _, vv := range product.VariationsValues {
		variationsValues = append(variationsValues, model.VariationValue{
			VariantName: vv.VariantName,
			Values:      vv.Values,
		})
	}

	var productDetails []model.ProductDetail
	for _, detail := range product.ProductDetails {
		productDetails = append(productDetails, model.ProductDetail{
			Type:  detail.Type,
			Value: detail.Value,
		})
	}

	var subcategoryRank []model.Subcategory
	for _, sub := range product.SubcategoryRank {
		subcategoryRank = append(subcategoryRank, model.Subcategory{
			SubcategoryName: sub.SubcategoryName,
			SubcategoryRank: sub.SubcategoryRank,
		})
	}

	// 转换价格明细
	pricesBreakdown := model.PriceBreakdown{
		TypicalPrice: product.PricesBreakdown.TypicalPrice,
		ListPrice:    product.PricesBreakdown.ListPrice,
		DealType:     product.PricesBreakdown.DealType,
	}

	// 转换购买框价格
	var buyboxPrices *model.BuyboxPrices
	if product.BuyboxPrices != nil {
		buyboxPrices = &model.BuyboxPrices{
			FinalPrice: product.BuyboxPrices.FinalPrice,
			UnitPrice:  product.BuyboxPrices.UnitPrice,
		}
	}

	// 转换客户评价
	var customersSay *model.CustomersSay
	if product.CustomersSay != nil {
		customersSay = &model.CustomersSay{
			Text: product.CustomersSay.Text,
			Keywords: model.CustomersKeywords{
				Positive: product.CustomersSay.Keywords.Positive,
				Negative: product.CustomersSay.Keywords.Negative,
				Mixed:    product.CustomersSay.Keywords.Mixed,
			},
		}
	}

	// 转换非活跃购买框
	var inactiveBuyBox *model.InactiveBuyBox
	if product.InactiveBuyBox != nil {
		inactiveBuyBox = &model.InactiveBuyBox{
			Price: product.InactiveBuyBox.Price,
		}
	}

	// 转换时间戳
	timestamp := model.NullableTime{
		Time: product.Timestamp.Time,
	}

	return &model.Product{
		// 基本信息
		Title:              product.Title,
		Brand:              product.Brand,
		Description:        product.Description,
		ProductDescription: productDescription,

		// 价格信息
		InitialPrice:    product.InitialPrice,
		FinalPrice:      product.FinalPrice,
		FinalPriceHigh:  product.FinalPriceHigh,
		Currency:        product.Currency,
		PricesBreakdown: pricesBreakdown,
		BuyboxPrices:    buyboxPrices,

		// 库存和可用性
		Availability:         product.Availability,
		IsAvailable:          product.IsAvailable,
		MaxQuantityAvailable: product.MaxQuantityAvailable,
		BoughtPastMonth:      product.BoughtPastMonth,

		// 评价信息
		ReviewsCount: product.ReviewsCount,
		Rating:       product.Rating,
		TopReview:    product.TopReview,
		CustomersSay: customersSay,

		// 分类和排名
		Categories:      product.Categories,
		BsRank:          product.BsRank,
		BsCategory:      product.BsCategory,
		RootBsRank:      product.RootBsRank,
		RootBsCategory:  product.RootBsCategory,
		SubcategoryRank: subcategoryRank,

		// ASIN和标识
		ParentAsin: product.ParentAsin,
		Asin:       product.Asin,

		// 卖家信息
		SellerName:      product.SellerName,
		SellerID:        product.SellerID,
		SellerURL:       product.SellerURL,
		BuyboxSeller:    product.BuyboxSeller,
		NumberOfSellers: product.NumberOfSellers,

		// URL和图片
		URL:         product.URL,
		ImageURL:    product.ImageURL,
		Images:      product.Images,
		ImagesCount: product.ImagesCount,

		// 视频
		Videos:             product.Videos,
		VideoCount:         product.VideoCount,
		Video:              product.Video,
		DownloadableVideos: product.DownloadableVideos,

		// 产品特性
		Features:         product.Features,
		Variations:       variations,
		VariationsValues: variationsValues,
		ProductDetails:   productDetails,

		// 产品详细信息
		ProductDimensions:  product.ProductDimensions,
		ItemWeight:         product.ItemWeight,
		ModelNumber:        product.ModelNumber,
		Department:         product.Department,
		DateFirstAvailable: product.DateFirstAvailable,
		Manufacturer:       product.Manufacturer,
		CountryOfOrigin:    product.CountryOfOrigin,

		// 配送信息
		Delivery:  product.Delivery,
		ShipsFrom: product.ShipsFrom,

		// 标记和徽章
		Badge:                 product.Badge,
		AmazonChoice:          product.AmazonChoice,
		PlusContent:           product.PlusContent,
		ClimatePledgeFriendly: product.ClimatePledgeFriendly,
		Sponsored:             product.Sponsored,

		// 其他信息
		Domain:            product.Domain,
		Zipcode:           product.Zipcode,
		Timestamp:         timestamp,
		AnsweredQuestions: product.AnsweredQuestions,
		StoreURL:          product.StoreURL,
		ReturnPolicy:      product.ReturnPolicy,
		InactiveBuyBox:    inactiveBuyBox,

		// 额外内容
		FromTheBrand:           product.FromTheBrand,
		SustainabilityFeatures: product.SustainabilityFeatures,
		OtherSellersPrices:     product.OtherSellersPrices,
		EditorialReviews:       product.EditorialReviews,
		AboutTheAuthor:         product.AboutTheAuthor,
	}
}
