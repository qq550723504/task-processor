package modules

import (
	"encoding/json"
	"fmt"
	"strings"
	"task-processor/internal/common/amazon"
	"task-processor/internal/common/amazon/model"
	"task-processor/internal/common/management/api"
	"task-processor/internal/common/product"
	"task-processor/internal/config"

	"github.com/sirupsen/logrus"
)

// VariantJsonDataHandler 获取所有变体原始Json数据处理器
type VariantJsonDataHandler struct {
	// 原始JSON数据客户端
	rawJsonDataClient api.RawJsonDataAPI
	// Amazon爬虫处理器
	amazonProcessor *amazon.AmazonProcessor
	// Amazon配置
	amazonConfig *config.AmazonConfig
}

// NewVariantJsonDataHandler 创建新的获取变体原始Json数据处理器
func NewVariantJsonDataHandler(
	rawJsonDataClient api.RawJsonDataAPI,
	amazonConfig *config.AmazonConfig,
	amazonProcessor interface{},
) *VariantJsonDataHandler {
	handler := &VariantJsonDataHandler{
		rawJsonDataClient: rawJsonDataClient,
		amazonConfig:      amazonConfig,
	}

	// 使用共享的Amazon处理器（如果提供）
	if amazonProcessor != nil {
		if ap, ok := amazonProcessor.(*amazon.AmazonProcessor); ok {
			handler.amazonProcessor = ap
			logrus.Info("[SHEIN] 变体数据使用共享的Amazon爬虫实例")
		}
	} else if amazonConfig != nil && amazonConfig.Enabled {
		// 如果没有提供共享实例，则创建新的（向后兼容）
		handler.amazonProcessor = amazon.NewAmazonProcessor(amazonConfig)
		logrus.Info("变体数据Amazon爬虫已启用")
	}

	return handler
}

// Name 返回处理器名称
func (h *VariantJsonDataHandler) Name() string {
	return "获取所有变体的Json数据"
}

// Handle 执行获取所有变体的Json数据处理
func (h *VariantJsonDataHandler) Handle(ctx *TaskContext) error {

	// 从上下文中获取所有变体ASIN列表（包括主产品，因为主产品也可能是可售卖的SKU）
	mainProductAsin := ctx.Task.ProductID
	variantAsins := getAsinListFromMap(ctx.AsinSkuMap, mainProductAsin)

	// 如果没有变体（单品情况），初始化空列表并继续
	if len(variantAsins) == 0 {
		logrus.Infof("✅ 产品 %s 没有变体（单品），跳过变体数据获取", mainProductAsin)
		emptyVariants := make([]model.Product, 0)
		ctx.Variants = &emptyVariants
		return nil
	}

	logrus.Infof("找到 %d 个变体ASIN（包含所有可售卖的SKU）\n", len(variantAsins))

	if len(variantAsins) > 100 {
		logrus.Infof("警告：变体ASIN数量过多（%d），可能会导致处理时间过长或请求失败\n", len(variantAsins))
		return NewNonRetryableError("变体ASIN数量过多，停止处理", nil)
	}

	// 为每个变体ASIN获取JSON数据
	variants := make([]model.Product, 0, len(variantAsins))

	// 第一步：先检查服务器是否有所有变体的历史数据
	logrus.Infof("检查服务器是否有 %d 个变体的历史数据", len(variantAsins))
	missingAsins := make([]string, 0)

	for _, asin := range variantAsins {
		task := &api.RawJsonDataReqDTO{
			TenantID:   ctx.Task.TenantID,
			Platform:   ctx.Task.Platform,
			ProductID:  asin,
			Region:     ctx.Task.Region,
			StoreID:    ctx.Task.StoreID,
			CategoryID: ctx.Task.CategoryID,
			Creator:    ctx.Task.Creator,
		}

		rawJsonData, err := h.rawJsonDataClient.GetRawJsonData(task)
		if err == nil && rawJsonData != nil && rawJsonData.RawJSONData != "" {
			// 服务器有历史数据，直接使用
			dataParser := product.NewDataParser(logrus.NewEntry(logrus.StandardLogger()))
			variant, parseErr := dataParser.ParseAmazonProduct(rawJsonData.RawJSONData)
			if parseErr == nil {
				variants = append(variants, *variant)
				logrus.Infof("变体 %s 使用服务器历史数据", asin)
				continue
			}
			logrus.Warnf("变体 %s 服务器数据解析失败: %v", asin, parseErr)
		}

		// 服务器没有数据或解析失败，记录需要抓取的ASIN
		missingAsins = append(missingAsins, asin)
	}

	logrus.Infof("服务器有 %d/%d 个变体的历史数据，需要抓取 %d 个",
		len(variants), len(variantAsins), len(missingAsins))

	// 第二步：对于服务器没有数据的变体，判断是否使用爬虫抓取
	if len(missingAsins) > 0 {
		if h.amazonConfig != nil && h.amazonConfig.Enabled && h.shouldUseAmazonCrawler(ctx) {
			logrus.Infof("使用Amazon爬虫批量抓取 %d 个缺失的变体", len(missingAsins))
			crawledVariants := h.fetchVariantsBatchFromAmazonCrawler(ctx, missingAsins)
			variants = append(variants, crawledVariants...)
		} else {
			logrus.Warnf("服务器缺少 %d 个变体数据且无法抓取（非Amazon平台或未启用爬虫）", len(missingAsins))
			// 非Amazon平台或未启用爬虫，只使用已有的数据
		}
	}

	logrus.Infof("最终获取到 %d/%d 个变体数据", len(variants), len(variantAsins))

	ctx.Variants = &variants

	return nil
}

// shouldUseAmazonCrawler 判断是否应该使用Amazon爬虫
func (h *VariantJsonDataHandler) shouldUseAmazonCrawler(ctx *TaskContext) bool {
	platform := ctx.Task.Platform
	return platform == "amazon" || platform == "Amazon" || platform == "AMAZON"
}

// fetchVariantsBatchFromAmazonCrawler 批量抓取变体数据
func (h *VariantJsonDataHandler) fetchVariantsBatchFromAmazonCrawler(ctx *TaskContext, asins []string) []model.Product {
	if h.amazonProcessor == nil {
		logrus.Error("Amazon爬虫未初始化")
		return []model.Product{}
	}

	// 使用公共函数获取地区信息
	domain := GetAmazonDomainByRegion(ctx.Task.Region)
	zipcode := GetZipcodeForRegion(ctx.Task.Region, h.amazonConfig.Zipcodes)

	// 准备批量请求
	requests := make([]model.ProductRequest, 0, len(asins))
	for _, asin := range asins {
		url := fmt.Sprintf("https://www.%s/dp/%s?th=1&psc=1", domain, asin)
		requests = append(requests, model.ProductRequest{
			URL:     url,
			Zipcode: zipcode,
		})
	}

	logrus.Infof("开始批量抓取 %d 个变体: Region=%s, Domain=%s, Zipcode=%s",
		len(requests), ctx.Task.Region, domain, zipcode)

	// 批量处理
	results := h.amazonProcessor.ProcessBatch(requests)

	// 收集结果并保存
	variants := make([]model.Product, 0, len(results))
	successCount := 0
	for i, result := range results {
		if result.Error != nil {
			logrus.Warnf("变体 %s 抓取失败: %v", asins[i], result.Error)
			continue
		}

		if result.Product != nil {
			variants = append(variants, *result.Product)
			successCount++

			// 异步保存抓取到的变体数据
			go h.saveVariantData(asins[i], ctx.Task.Region, result.Product, ctx.Task.TenantID, ctx.Task.StoreID)
		}
	}

	logrus.Infof("批量抓取完成: 成功 %d/%d", successCount, len(asins))

	return variants
}

// saveVariantData 保存变体数据到数据库（异步）
func (h *VariantJsonDataHandler) saveVariantData(asin, region string, product *model.Product, tenantID, storeID int64) {
	rawJSON, err := json.Marshal(product)
	if err != nil {
		logrus.WithError(err).WithField("asin", asin).Error("序列化变体数据失败")
		return
	}

	createReq := &api.RawJsonDataCreateReqDTO{
		TenantID:    tenantID,
		Platform:    "Amazon",
		Region:      region,
		ProductID:   asin,
		RawJsonData: string(rawJSON),
		StoreID:     storeID,
		Creator:     "variant_handler",
	}

	_, err = h.rawJsonDataClient.CreateRawJsonData(createReq)
	if err != nil {
		logrus.WithError(err).WithField("asin", asin).Error("保存变体数据到数据库失败")
	} else {
		logrus.WithField("asin", asin).Debug("已保存变体数据到数据库")
	}
}

// Shutdown 关闭处理器，释放资源（现在由共享的Amazon处理器管理）
func (h *VariantJsonDataHandler) Shutdown() {
	// Amazon处理器由外部管理，不需要在这里关闭
	logrus.Debug("[SHEIN] VariantJsonDataHandler 关闭（Amazon处理器由外部管理）")
}

// getAsinListFromMap 从AsinSkuMap中提取所有ASIN（包括主产品ASIN）
// 注意：主产品ASIN也可能是一个可售卖的SKU，不应该被排除
func getAsinListFromMap(asinSkuMap map[string]string, mainProductAsin string) []string {
	if len(asinSkuMap) == 0 {
		return []string{}
	}

	// 创建ASIN列表，包含所有ASIN（包括主产品）
	asinList := make([]string, 0, len(asinSkuMap))
	mainProductCount := 0

	for asin := range asinSkuMap {
		asinList = append(asinList, asin)

		// 统计主产品（仅用于日志）
		normalizedAsin := strings.TrimSpace(strings.ToUpper(asin))
		normalizedMainAsin := strings.TrimSpace(strings.ToUpper(mainProductAsin))
		if normalizedAsin == normalizedMainAsin {
			mainProductCount++
		}
	}

	logrus.Infof("🔍 [SHEIN变体] 从AsinSkuMap获取完成: 总变体数=%d (包含主产品=%d)",
		len(asinList), mainProductCount)

	return asinList
}

// getVariantByAsinFromVariants 通过ASIN从Variants中获取变体
func GetVariantByAsinFromVariants(variants *[]model.Product, asin string) *model.Product {
	if variants == nil {
		return nil
	}
	for _, variant := range *variants {
		if variant.Asin == asin {
			return &variant
		}
	}
	return nil
}
