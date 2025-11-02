package handlers

import (
	"encoding/json"
	"fmt"
	"strings"
	"task-processor/common/amazon"
	"task-processor/common/config"
	"task-processor/common/management/api"
	"task-processor/common/pipeline"

	"github.com/sirupsen/logrus"
)

// RawJsonDataHandlerV2 原始JSON数据处理器V2（参考SHEIN架构）
type RawJsonDataHandlerV2 struct {
	logger            *logrus.Entry
	rawJsonDataClient interface {
		GetRawJsonData(req *api.RawJsonDataReqDTO) (*api.RawJsonDataRespDTO, error)
	}
	amazonProcessor *amazon.AmazonProcessor
	amazonConfig    *config.AmazonConfig
}

// NewRawJsonDataHandlerV2 创建新的原始JSON数据处理器V2
func NewRawJsonDataHandlerV2(rawJsonDataClient interface {
	GetRawJsonData(req *api.RawJsonDataReqDTO) (*api.RawJsonDataRespDTO, error)
}, amazonConfig *config.AmazonConfig) *RawJsonDataHandlerV2 {
	handler := &RawJsonDataHandlerV2{
		logger:            logrus.WithField("handler", "RawJsonDataHandlerV2"),
		rawJsonDataClient: rawJsonDataClient,
		amazonConfig:      amazonConfig,
	}

	// 如果启用Amazon爬虫，初始化处理器
	if amazonConfig != nil && amazonConfig.Enabled {
		handler.amazonProcessor = amazon.NewAmazonProcessor(amazonConfig)
		logrus.Info("[TEMU] Amazon爬虫已启用")
	}

	return handler
}

// Name 返回处理器名称
func (h *RawJsonDataHandlerV2) Name() string {
	return "原始JSON数据处理器V2"
}

// Handle 处理任务（参考SHEIN的处理流程）
func (h *RawJsonDataHandlerV2) Handle(ctx *pipeline.TaskContext) error {
	h.logger.Info("开始获取原始JSON数据")

	// 检查任务上下文中的必要数据
	if ctx.Task == nil {
		return fmt.Errorf("任务信息为空")
	}

	if ctx.Task.ProductID == "" {
		return fmt.Errorf("产品ID为空")
	}

	var amazonProduct *amazon.Product
	var err error

	// 第一步：先检查服务器是否有历史数据
	h.logger.Infof("检查服务器是否有历史数据: ProductID=%s, Region=%s", ctx.Task.ProductID, ctx.Task.Region)
	req := &api.RawJsonDataReqDTO{
		TenantID:   ctx.Task.TenantID,
		Platform:   ctx.Task.Platform,
		ProductID:  ctx.Task.ProductID,
		Region:     ctx.Task.Region,
		StoreID:    ctx.Task.StoreID,
		CategoryID: ctx.Task.CategoryID,
		Creator:    ctx.Task.Creator,
	}

	rawJsonData, err := h.rawJsonDataClient.GetRawJsonData(req)
	if err == nil && rawJsonData != nil && rawJsonData.RawJSONData != "" {
		// 服务器有历史数据，直接使用
		h.logger.Infof("服务器有历史数据，直接使用: ProductID=%s", ctx.Task.ProductID)
		amazonProduct, parseErr := h.parseAmazonProduct(rawJsonData.RawJSONData)
		if parseErr == nil {
			ctx.AmazonProduct = amazonProduct
			ctx.RawJsonData = rawJsonData
			return nil
		}
		h.logger.Warnf("解析服务器历史数据失败: %v", parseErr)
	} else if err != nil {
		h.logger.Infof("服务器无历史数据或获取失败: %v", err)
	}

	// 第二步：服务器没有数据，判断是否使用Amazon爬虫抓取
	if h.shouldUseAmazonCrawler(ctx) && h.amazonProcessor != nil {
		h.logger.Infof("服务器无数据，使用Amazon爬虫抓取: ProductID=%s, Region=%s", ctx.Task.ProductID, ctx.Task.Region)
		amazonProduct, err = h.fetchFromAmazonCrawler(ctx)
		if err != nil {
			return fmt.Errorf("Amazon爬虫抓取失败: %w", err)
		}
		ctx.NeedsAmazonData = true
	} else {
		// 非Amazon平台或未启用爬虫，返回错误
		return fmt.Errorf("服务器无产品数据且无法抓取: ProductID=%s, Platform=%s", ctx.Task.ProductID, ctx.Task.Platform)
	}

	// 将Amazon产品数据存储到上下文中
	ctx.AmazonProduct = amazonProduct

	h.logger.Infof("成功获取原始JSON数据: ProductID=%s, Platform=%s",
		ctx.Task.ProductID, ctx.Task.Platform)
	return nil
}

// shouldUseAmazonCrawler 判断是否应该使用Amazon爬虫
func (h *RawJsonDataHandlerV2) shouldUseAmazonCrawler(ctx *pipeline.TaskContext) bool {
	platform := ctx.Task.Platform
	return strings.EqualFold(platform, "amazon")
}

// fetchFromAmazonCrawler 使用Amazon爬虫抓取数据
func (h *RawJsonDataHandlerV2) fetchFromAmazonCrawler(ctx *pipeline.TaskContext) (*amazon.Product, error) {
	if h.amazonProcessor == nil {
		return nil, fmt.Errorf("Amazon爬虫未初始化")
	}

	asin := ctx.Task.ProductID

	// 根据地区获取正确的Amazon域名
	domain := h.getAmazonDomainByRegion(ctx.Task.Region)
	url := fmt.Sprintf("https://www.%s/dp/%s", domain, asin)

	// 根据地区获取邮编
	zipcode := h.getZipcodeForRegion(ctx.Task.Region)

	h.logger.Infof("开始使用爬虫抓取Amazon产品: Region=%s, Domain=%s, URL=%s, Zipcode=%s",
		ctx.Task.Region, domain, url, zipcode)

	// 调用Amazon爬虫
	amazonProduct, err := h.amazonProcessor.Process(url, zipcode)
	if err != nil {
		return nil, fmt.Errorf("抓取Amazon产品失败: %w", err)
	}

	h.logger.Infof("Amazon数据获取完成: 标题=%s, 价格=%.2f %s",
		amazonProduct.Title, amazonProduct.FinalPrice, amazonProduct.Currency)

	return amazonProduct, nil
}

// parseAmazonProduct 解析Amazon产品JSON数据
func (h *RawJsonDataHandlerV2) parseAmazonProduct(jsonData string) (*amazon.Product, error) {
	if jsonData == "" {
		return nil, fmt.Errorf("JSON数据为空")
	}

	var product amazon.Product
	if err := json.Unmarshal([]byte(jsonData), &product); err != nil {
		return nil, fmt.Errorf("解析JSON数据失败: %w", err)
	}

	return &product, nil
}

// getAmazonDomainByRegion 根据地区获取Amazon域名
func (h *RawJsonDataHandlerV2) getAmazonDomainByRegion(region string) string {
	switch strings.ToLower(region) {
	case "us", "usa", "united states":
		return "amazon.com"
	case "uk", "gb", "united kingdom":
		return "amazon.co.uk"
	case "de", "germany":
		return "amazon.de"
	case "fr", "france":
		return "amazon.fr"
	case "it", "italy":
		return "amazon.it"
	case "es", "spain":
		return "amazon.es"
	case "ca", "canada":
		return "amazon.ca"
	case "jp", "japan":
		return "amazon.co.jp"
	default:
		return "amazon.com" // 默认美国站
	}
}

// getZipcodeForRegion 根据地区获取邮编
func (h *RawJsonDataHandlerV2) getZipcodeForRegion(region string) string {
	switch strings.ToLower(region) {
	case "us", "usa", "united states":
		return "10001" // 纽约
	case "uk", "gb", "united kingdom":
		return "SW1A 1AA" // 伦敦
	case "de", "germany":
		return "10115" // 柏林
	case "fr", "france":
		return "75001" // 巴黎
	case "ca", "canada":
		return "M5H 2N2" // 多伦多
	default:
		return "10001" // 默认美国邮编
	}
}

// Shutdown 关闭处理器，释放资源
func (h *RawJsonDataHandlerV2) Shutdown() {
	if h.amazonProcessor != nil {
		h.logger.Info("关闭Amazon爬虫处理器...")
		h.amazonProcessor.Shutdown()
	}
}
