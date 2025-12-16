package modules

import (
	"strings"
	"task-processor/common/amazon"
	"task-processor/common/amazon/model"
	"task-processor/common/management/api"
	"task-processor/common/product"
	"task-processor/internal/config"

	"github.com/sirupsen/logrus"
)

// RawJsonDataHandler 获取原始Json数据处理器
type RawJsonDataHandler struct {
	fetcher *product.ProductFetcher
}

// sheinRawJsonDataClient SHEIN 原始 JSON 数据客户端（简单包装）
type sheinRawJsonDataClient struct {
	client api.RawJsonDataAPI
}

func (c *sheinRawJsonDataClient) GetRawJsonData(req *api.RawJsonDataReqDTO) (*api.RawJsonDataRespDTO, error) {
	return c.client.GetRawJsonData(req)
}

func (c *sheinRawJsonDataClient) CreateRawJsonData(req *api.RawJsonDataCreateReqDTO) (int64, error) {
	return c.client.CreateRawJsonData(req)
}

// NewRawJsonDataHandler 创建新的获取原始Json数据处理器
func NewRawJsonDataHandler(
	rawJsonDataClient api.RawJsonDataAPI,
	amazonConfig *config.AmazonConfig,
	amazonProcessor interface{},
) *RawJsonDataHandler {
	// 提取 Amazon 处理器
	var ap *amazon.AmazonProcessor
	if amazonProcessor != nil {
		if processor, ok := amazonProcessor.(*amazon.AmazonProcessor); ok {
			ap = processor
			logrus.Info("[SHEIN] 使用共享的 Amazon 爬虫实例")
		}
	} else if amazonConfig != nil && amazonConfig.Enabled {
		// 如果没有提供共享实例，则创建新的（向后兼容）
		ap = amazon.NewAmazonProcessor(amazonConfig)
		logrus.Info("[SHEIN] Amazon 爬虫已启用")
	}

	// 包装客户端
	client := &sheinRawJsonDataClient{client: rawJsonDataClient}

	return &RawJsonDataHandler{
		fetcher: product.NewProductFetcher(client, amazonConfig, ap),
	}
}

// Name 返回处理器名称
func (h *RawJsonDataHandler) Name() string {
	return "获取原始Json数据"
}

// isProductNotFoundError 检查错误是否为产品不存在错误
func isProductNotFoundError(err error) bool {
	if err == nil {
		return false
	}

	// 检查是否为 ProductNotFoundError 类型
	if _, ok := err.(*model.ProductNotFoundError); ok {
		return true
	}

	// 检查错误信息（使用包含匹配，而不是精确匹配）
	errorStr := strings.ToLower(err.Error())
	productNotFoundPatterns := []string{
		"产品页面不存在",
		"产品页面缺少必要元素",
		"page not found",
		"产品不存在",
		"asin无效",
		"产品已下架",
	}

	for _, pattern := range productNotFoundPatterns {
		if strings.Contains(errorStr, strings.ToLower(pattern)) {
			return true
		}
	}

	return false
}

func (h *RawJsonDataHandler) Handle(ctx *TaskContext) error {
	logrus.Infof("开始获取原始JSON数据: ProductID=%s, Region=%s", ctx.Task.ProductID, ctx.Task.Region)

	// 使用公共ProductFetcher获取产品数据
	req := &product.FetchRequest{
		TenantID:   ctx.Task.TenantID,
		Platform:   ctx.Task.Platform,
		Region:     ctx.Task.Region,
		ProductID:  ctx.Task.ProductID,
		StoreID:    ctx.Task.StoreID,
		CategoryID: ctx.Task.CategoryID,
		Creator:    ctx.Task.Creator,
	}

	amazonProduct, err := h.fetcher.FetchProduct(req)
	if err != nil {
		// 检查是否为产品不存在错误
		if isProductNotFoundError(err) {
			logrus.Warnf("产品不存在，不需要重试: ProductID=%s, Error=%v", ctx.Task.ProductID, err)
			return NewNonRetryableError("Amazon产品不存在", err)
		}
		// 其他错误（如超时、网络错误）可以重试
		return NewRetryableError("获取产品数据失败", err)
	}

	// 将原始JSON数据存储到上下文中
	ctx.AmazonProduct = amazonProduct

	return nil
}

// Shutdown 关闭处理器，释放资源（现在由共享的Amazon处理器管理）
func (h *RawJsonDataHandler) Shutdown() {
	// Amazon处理器由外部管理，不需要在这里关闭
	logrus.Debug("RawJsonDataHandler 关闭（Amazon处理器由外部管理）")
}
