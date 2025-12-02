package product

import (
	"encoding/json"
	"fmt"
	"strings"
	"task-processor/common/amazon"
	"task-processor/common/config"
	"task-processor/common/management/api"

	"github.com/sirupsen/logrus"
)

// ProductFetcher 产品数据获取器（支持从API或Amazon爬虫获取）
type ProductFetcher struct {
	rawJsonDataClient RawJsonDataClient
	amazonProcessor   *amazon.AmazonProcessor
	amazonConfig      *config.AmazonConfig
	logger            *logrus.Entry
}

// RawJsonDataClient 原始JSON数据客户端接口
type RawJsonDataClient interface {
	GetRawJsonData(req *api.RawJsonDataReqDTO) (*api.RawJsonDataRespDTO, error)
	CreateRawJsonData(req *api.RawJsonDataCreateReqDTO) (int64, error)
}

// FetchRequest 获取请求
type FetchRequest struct {
	TenantID   int64
	Platform   string
	Region     string
	ProductID  string
	StoreID    int64
	CategoryID int64
	Creator    string
}

// NewProductFetcher 创建产品数据获取器
func NewProductFetcher(
	rawJsonDataClient RawJsonDataClient,
	amazonConfig *config.AmazonConfig,
	amazonProcessor *amazon.AmazonProcessor,
) *ProductFetcher {
	return &ProductFetcher{
		rawJsonDataClient: rawJsonDataClient,
		amazonProcessor:   amazonProcessor,
		amazonConfig:      amazonConfig,
		logger:            logrus.WithField("component", "ProductFetcher"),
	}
}

// FetchProduct 获取产品数据（优先从API，如果没有则从Amazon爬虫）
func (f *ProductFetcher) FetchProduct(req *FetchRequest) (*amazon.Product, error) {
	f.logger.Infof("🔍 开始获取产品数据: ProductID=%s, Platform=%s, Region=%s",
		req.ProductID, req.Platform, req.Region)

	// 第一步：检查服务器是否有历史数据
	apiReq := &api.RawJsonDataReqDTO{
		TenantID:   req.TenantID,
		Platform:   req.Platform,
		ProductID:  req.ProductID,
		Region:     req.Region,
		StoreID:    req.StoreID,
		CategoryID: req.CategoryID,
		Creator:    req.Creator,
	}

	rawJsonData, err := f.rawJsonDataClient.GetRawJsonData(apiReq)
	if err == nil && rawJsonData != nil && rawJsonData.RawJSONData != "" {
		// 服务器有历史数据，直接使用
		f.logger.Infof("✅ 服务器有历史数据: ProductID=%s, 数据长度=%d", req.ProductID, len(rawJsonData.RawJSONData))

		product, parseErr := ParseAmazonProduct(rawJsonData.RawJSONData)
		if parseErr == nil {
			// 检查是否为旧版数据（variations 没有 attributes 字段）
			needsRefetch := f.needsRefetchForOldFormat(product)
			if needsRefetch {
				f.logger.Warnf("⚠️ 检测到旧版数据格式（variations 缺少 attributes），需要重新抓取: ProductID=%s", req.ProductID)
			} else {
				f.logger.Infof("✅ 成功解析服务器数据: ProductID=%s, Title=%s", req.ProductID, product.Title)
				return product, nil
			}
		} else {
			f.logger.Warnf("⚠️ 解析服务器数据失败: %v", parseErr)
		}
	} else if err != nil {
		f.logger.Debugf("服务器无历史数据或获取失败: %v", err)
	}

	// 第二步：服务器没有数据，判断是否使用Amazon爬虫抓取
	if f.shouldUseAmazonCrawler(req.Platform) && f.amazonProcessor != nil {
		f.logger.Infof("🌐 服务器无数据，使用Amazon爬虫抓取: ProductID=%s", req.ProductID)

		product, err := f.fetchFromAmazonCrawler(req)
		if err != nil {
			return nil, fmt.Errorf("Amazon爬虫抓取失败: %w", err)
		}

		// 保存到服务器
		if saveErr := f.saveToServer(req, product); saveErr != nil {
			f.logger.Warnf("⚠️ 保存到服务器失败: %v", saveErr)
		}

		return product, nil
	}

	return nil, fmt.Errorf("服务器无产品数据且无法抓取: ProductID=%s, Platform=%s", req.ProductID, req.Platform)
}

// CacheProduct 缓存产品数据到服务器（用于已获取的产品数据）
// 这个方法用于将已经获取到的产品数据（无论来源）保存到服务器缓存
func (f *ProductFetcher) CacheProduct(req *FetchRequest, product *amazon.Product) error {
	if product == nil {
		f.logger.Warn("产品数据为空，跳过缓存")
		return nil
	}

	f.logger.Infof("💾 开始缓存产品数据到服务器: ProductID=%s", req.ProductID)

	// 检查服务器是否已有该产品数据
	apiReq := &api.RawJsonDataReqDTO{
		TenantID:   req.TenantID,
		Platform:   req.Platform,
		ProductID:  req.ProductID,
		Region:     req.Region,
		StoreID:    req.StoreID,
		CategoryID: req.CategoryID,
		Creator:    req.Creator,
	}

	rawJsonData, err := f.rawJsonDataClient.GetRawJsonData(apiReq)
	if err == nil && rawJsonData != nil && rawJsonData.RawJSONData != "" {
		f.logger.Infof("⏭️ 服务器已有产品数据缓存，跳过: ProductID=%s", req.ProductID)
		return nil
	}

	// 服务器没有数据，保存
	return f.saveToServer(req, product)
}

// shouldUseAmazonCrawler 判断是否应该使用Amazon爬虫
func (f *ProductFetcher) shouldUseAmazonCrawler(platform string) bool {
	if f.amazonConfig == nil || !f.amazonConfig.Enabled {
		return false
	}
	return strings.EqualFold(platform, "amazon")
}

// fetchFromAmazonCrawler 使用Amazon爬虫抓取数据
func (f *ProductFetcher) fetchFromAmazonCrawler(req *FetchRequest) (*amazon.Product, error) {
	if f.amazonProcessor == nil {
		return nil, fmt.Errorf("Amazon爬虫未初始化")
	}

	// 根据地区获取Amazon域名和邮编
	domain := GetAmazonDomainByRegion(req.Region)
	zipcode := GetZipcodeForRegion(req.Region, f.amazonConfig.Zipcodes)

	// 构建URL
	url := fmt.Sprintf("https://www.%s/dp/%s?th=1&psc=1&language=en_US", domain, req.ProductID)

	f.logger.Infof("🚀 开始爬取: URL=%s, Zipcode=%s", url, zipcode)

	// 调用Amazon爬虫
	product, err := f.amazonProcessor.Process(url, zipcode)
	if err != nil {
		return nil, fmt.Errorf("抓取失败: %w", err)
	}

	f.logger.Infof("✅ 爬取成功: ProductID=%s, Title=%s", req.ProductID, product.Title)
	return product, nil
}

// CacheVariants 批量缓存变体数据到服务器
// 这个方法用于将已经获取到的变体数据批量保存到服务器缓存
func (f *ProductFetcher) CacheVariants(req *FetchRequest, variants []*amazon.Product) error {
	if len(variants) == 0 {
		f.logger.Debug("没有变体数据，跳过缓存")
		return nil
	}

	f.logger.Infof("💾 开始批量缓存变体数据到服务器: 数量=%d", len(variants))

	successCount := 0
	failCount := 0
	skipCount := 0

	for _, variant := range variants {
		if variant == nil {
			f.logger.Warn("变体数据为空，跳过")
			skipCount++
			continue
		}

		// 检查服务器是否已有该变体数据
		apiReq := &api.RawJsonDataReqDTO{
			TenantID:   req.TenantID,
			Platform:   req.Platform,
			ProductID:  variant.Asin,
			Region:     req.Region,
			StoreID:    req.StoreID,
			CategoryID: req.CategoryID,
			Creator:    req.Creator,
		}

		rawJsonData, err := f.rawJsonDataClient.GetRawJsonData(apiReq)
		if err == nil && rawJsonData != nil && rawJsonData.RawJSONData != "" {
			f.logger.Debugf("⏭️ 服务器已有变体数据缓存，跳过: ASIN=%s", variant.Asin)
			skipCount++
			continue
		}

		// 构建变体请求
		variantReq := &FetchRequest{
			TenantID:   req.TenantID,
			Platform:   req.Platform,
			Region:     req.Region,
			ProductID:  variant.Asin,
			StoreID:    req.StoreID,
			CategoryID: req.CategoryID,
			Creator:    req.Creator,
		}

		// 保存变体数据
		if saveErr := f.saveToServer(variantReq, variant); saveErr != nil {
			f.logger.Errorf("保存变体数据失败 (ASIN: %s): %v", variant.Asin, saveErr)
			failCount++
			continue
		}

		successCount++
	}

	f.logger.Infof("✅ 变体数据缓存完成: 成功=%d, 失败=%d, 跳过=%d, 总数=%d",
		successCount, failCount, skipCount, len(variants))

	// 如果所有变体都失败，返回错误
	if failCount > 0 && successCount == 0 {
		return fmt.Errorf("所有变体数据缓存失败: 失败数=%d", failCount)
	}

	return nil
}

// saveToServer 保存产品数据到服务器
func (f *ProductFetcher) saveToServer(req *FetchRequest, product *amazon.Product) error {
	if product == nil {
		return fmt.Errorf("产品数据为空")
	}

	// 序列化为JSON
	jsonData, err := json.Marshal(product)
	if err != nil {
		return fmt.Errorf("序列化失败: %w", err)
	}

	// 构建创建请求
	createReq := &api.RawJsonDataCreateReqDTO{
		TenantID:    req.TenantID,
		Platform:    req.Platform,
		Region:      req.Region,
		ProductID:   req.ProductID,
		RawJsonData: string(jsonData),
		Creator:     req.Creator,
		StoreID:     req.StoreID,
		CategoryID:  req.CategoryID,
	}

	// 调用API保存
	id, err := f.rawJsonDataClient.CreateRawJsonData(createReq)
	if err != nil {
		return fmt.Errorf("保存失败: %w", err)
	}

	f.logger.Infof("✅ 保存成功: ProductID=%s, ID=%d", req.ProductID, id)
	return nil
}

// ParseAmazonProduct 解析Amazon产品JSON数据
func ParseAmazonProduct(jsonData string) (*amazon.Product, error) {
	if jsonData == "" {
		return nil, fmt.Errorf("JSON数据为空")
	}

	// 首先尝试解析为单个对象
	var product amazon.Product
	if err := json.Unmarshal([]byte(jsonData), &product); err == nil {
		// 重新计算 IsAvailable 字段（修复历史数据中的错误）
		product.IsAvailable = recalculateIsAvailable(&product)
		return &product, nil
	}

	// 如果解析单个对象失败，尝试解析为数组并取第一个元素
	var products []amazon.Product
	if err := json.Unmarshal([]byte(jsonData), &products); err == nil {
		if len(products) > 0 {
			// 重新计算 IsAvailable 字段（修复历史数据中的错误）
			products[0].IsAvailable = recalculateIsAvailable(&products[0])
			return &products[0], nil
		}
		return nil, fmt.Errorf("JSON数组为空")
	}

	return nil, fmt.Errorf("解析JSON数据失败")
}

// recalculateIsAvailable 重新计算产品是否可用
func recalculateIsAvailable(product *amazon.Product) bool {
	lowerText := strings.ToLower(strings.TrimSpace(product.Availability))

	// 不可用的关键词（优先检查）
	unavailableKeywords := []string{
		"currently unavailable", "unavailable", "out of stock",
		"temporarily out of stock", "not available", "discontinued", "sold out",
		"no disponible", "agotado", "sin stock", "temporalmente agotado",
		"actualmente no disponible", "在庫切れ", "一時的に在庫切れ",
		"取り扱い終了", "現在お取り扱いでき���せん",
	}

	for _, keyword := range unavailableKeywords {
		if strings.Contains(lowerText, keyword) {
			logrus.WithFields(logrus.Fields{
				"asin":         product.Asin,
				"availability": product.Availability,
				"keyword":      keyword,
			}).Debug("❌ 匹配到不可用关键词，判定为不可用")
			return false
		}
	}

	// 可用的关键词
	availableKeywords := []string{
		"in stock", "available", "ships", "delivery", "arrives",
		"left in stock", "more on the way", "usually ships", "in stock soon",
		"disponible", "en stock", "envío", "entrega", "llega",
		"在庫あり", "配送", "お届け", "発送",
	}

	for _, keyword := range availableKeywords {
		if strings.Contains(lowerText, keyword) {
			logrus.WithFields(logrus.Fields{
				"asin":         product.Asin,
				"availability": product.Availability,
				"keyword":      keyword,
			}).Debug("✅ 匹配到可用关键词，判定为可用")
			return true
		}
	}

	// 无法明确判断时，保持原有值
	logrus.WithFields(logrus.Fields{
		"asin":           product.Asin,
		"availability":   product.Availability,
		"original_value": product.IsAvailable,
	}).Debug("⚠️ 无法明确判断可用性，保持原有值")
	return product.IsAvailable
}

// GetAmazonDomainByRegion 根据地区获取Amazon域名
func GetAmazonDomainByRegion(region string) string {
	domainMap := map[string]string{
		"us":                   "amazon.com",
		"usa":                  "amazon.com",
		"united states":        "amazon.com",
		"uk":                   "amazon.co.uk",
		"gb":                   "amazon.co.uk",
		"united kingdom":       "amazon.co.uk",
		"de":                   "amazon.de",
		"germany":              "amazon.de",
		"fr":                   "amazon.fr",
		"france":               "amazon.fr",
		"it":                   "amazon.it",
		"italy":                "amazon.it",
		"es":                   "amazon.es",
		"spain":                "amazon.es",
		"ca":                   "amazon.ca",
		"canada":               "amazon.ca",
		"jp":                   "amazon.co.jp",
		"japan":                "amazon.co.jp",
		"au":                   "amazon.com.au",
		"australia":            "amazon.com.au",
		"mx":                   "amazon.com.mx",
		"mexico":               "amazon.com.mx",
		"ae":                   "amazon.ae",
		"uae":                  "amazon.ae",
		"united arab emirates": "amazon.ae",
		"sa":                   "amazon.sa",
		"saudi":                "amazon.sa",
		"saudi arabia":         "amazon.sa",
	}

	if domain, exists := domainMap[strings.ToLower(region)]; exists {
		return domain
	}

	return "amazon.com" // 默认美国站
}

// needsRefetchForOldFormat 检查产品是否为旧版格式需要重新抓取
func (f *ProductFetcher) needsRefetchForOldFormat(product *amazon.Product) bool {
	if product == nil {
		return false
	}

	// 检查是否有 variations
	if len(product.Variations) == 0 {
		return false
	}

	// 检查 variations 是否缺少 attributes 字段
	for _, variation := range product.Variations {
		if len(variation.Attributes) == 0 {
			// 发现旧版格式（没有 attributes）
			return true
		}
	}

	return false
}

// GetZipcodeForRegion 根据地区获取邮编
func GetZipcodeForRegion(region string, configZipcodes map[string]string) string {
	// 优先使用配置中的邮编
	if configZipcodes != nil {
		if zipcode, exists := configZipcodes[region]; exists {
			return zipcode
		}
	}

	// 使用默认邮编映射
	zipcodeMap := map[string]string{
		"us":                   "10001", // 纽约
		"usa":                  "10001",
		"united states":        "10001",
		"uk":                   "SW1A 1AA", // 伦敦
		"gb":                   "SW1A 1AA",
		"united kingdom":       "SW1A 1AA",
		"de":                   "10115", // 柏林
		"germany":              "10115",
		"fr":                   "75001", // 巴黎
		"france":               "75001",
		"it":                   "00118", // 罗马
		"italy":                "00118",
		"es":                   "28001", // 马德里
		"spain":                "28001",
		"ca":                   "M5H 2N2", // 多伦多
		"canada":               "M5H 2N2",
		"jp":                   "153-0064", // 东京
		"japan":                "153-0064",
		"au":                   "2000", // 悉尼
		"australia":            "2000",
		"mx":                   "11000", // 墨西哥城
		"mexico":               "11000",
		"ae":                   "", // 阿联酋（不需要邮编）
		"uae":                  "",
		"united arab emirates": "",
		"sa":                   "", // 沙特（不需要邮编）
		"saudi":                "",
		"saudi arabia":         "",
	}

	if zipcode, exists := zipcodeMap[strings.ToLower(region)]; exists {
		return zipcode
	}

	return "10001" // 默认美国邮编
}
