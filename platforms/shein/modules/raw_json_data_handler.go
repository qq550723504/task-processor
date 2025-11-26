package modules

import (
	"strings"
	"task-processor/common/amazon"
	"task-processor/common/config"
	"task-processor/common/management/api"
	"task-processor/common/product"

	"github.com/sirupsen/logrus"
)

// RawJsonDataHandler 获取原始Json数据处理器（使用公共ProductFetcher）
type RawJsonDataHandler struct {
	// 添加管理系统的原始JSON数据客户端（用于变体确认）
	rawJsonDataClient interface {
		GetRawJsonData(req *api.RawJsonDataReqDTO) (*api.RawJsonDataRespDTO, error)
		ConfirmProductVariants(req *api.ProductVariantConfirmationReqDTO) (bool, error)
	}
	// 产品数据获取器
	fetcher *product.ProductFetcher
}

// rawJsonDataClientAdapter 适配器，将SHEIN的客户端接口适配到ProductFetcher需要的接口
type rawJsonDataClientAdapter struct {
	client interface {
		GetRawJsonData(req *api.RawJsonDataReqDTO) (*api.RawJsonDataRespDTO, error)
		ConfirmProductVariants(req *api.ProductVariantConfirmationReqDTO) (bool, error)
	}
}

func (a *rawJsonDataClientAdapter) GetRawJsonData(req *api.RawJsonDataReqDTO) (*api.RawJsonDataRespDTO, error) {
	return a.client.GetRawJsonData(req)
}

func (a *rawJsonDataClientAdapter) CreateRawJsonData(req *api.RawJsonDataCreateReqDTO) (int64, error) {
	// SHEIN不需要保存数据到服务器，返回0表示成功
	logrus.Debug("[SHEIN] CreateRawJsonData 被调用，但SHEIN不需要保存数据")
	return 0, nil
}

// NewRawJsonDataHandler 创建新的获取原始Json数据处理器
// 注意：在pipeline.go中需要传入管理系统的客户端和Amazon配置
func NewRawJsonDataHandler(rawJsonDataClient interface {
	GetRawJsonData(req *api.RawJsonDataReqDTO) (*api.RawJsonDataRespDTO, error)
	ConfirmProductVariants(req *api.ProductVariantConfirmationReqDTO) (bool, error)
}, amazonConfig *config.AmazonConfig, amazonProcessor interface{}) *RawJsonDataHandler {

	// 提取Amazon处理器
	var ap *amazon.AmazonProcessor
	if amazonProcessor != nil {
		if processor, ok := amazonProcessor.(*amazon.AmazonProcessor); ok {
			ap = processor
			logrus.Info("[SHEIN] 使用共享的Amazon爬虫实例")
		}
	} else if amazonConfig != nil && amazonConfig.Enabled {
		// 如果没有提供共享实例，则创建新的（向后兼容）
		ap = amazon.NewAmazonProcessor(amazonConfig)
		logrus.Info("[SHEIN] Amazon爬虫已启用")
	}

	// 使用适配器包装客户端
	adapter := &rawJsonDataClientAdapter{client: rawJsonDataClient}

	return &RawJsonDataHandler{
		rawJsonDataClient: rawJsonDataClient,
		fetcher:           product.NewProductFetcher(adapter, amazonConfig, ap),
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
	if _, ok := err.(*amazon.ProductNotFoundError); ok {
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
