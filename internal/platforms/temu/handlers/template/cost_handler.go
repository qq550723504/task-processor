package template

import (
	"fmt"
	"task-processor/internal/core/logger"
	"task-processor/internal/pipeline"
	"task-processor/internal/platforms/temu/api"
	temuproduct "task-processor/internal/platforms/temu/api/product"
	temuquery "task-processor/internal/platforms/temu/api/query"
	temucontext "task-processor/internal/platforms/temu/context"

	"github.com/sirupsen/logrus"
)

// CostTemplateHandler 成本模板处理器
type CostTemplateHandler struct {
	logger *logrus.Entry
}

func NewCostTemplateHandler() *CostTemplateHandler {
	return &CostTemplateHandler{
		logger: logger.GetGlobalLogger("temu.handlers.cost_template").WithField("handler", "CostTemplateHandler"),
	}
}

func (h *CostTemplateHandler) Name() string {
	return "成本模板处理器"
}

// Handle 兼容性方法，实现pipeline.Handler接口
func (h *CostTemplateHandler) Handle(ctx pipeline.TaskContext) error {
	// 类型断言转换为强类型上下文
	temuCtx, ok := ctx.(*temucontext.TemuTaskContext)
	if !ok {
		return pipeline.NewHandlerError(h.Name(), "上下文类型错误：期望 *TemuTaskContext")
	}

	return h.HandleTemu(temuCtx)
}

func (h *CostTemplateHandler) HandleTemu(temuCtx *temucontext.TemuTaskContext) error {
	h.logger.Info("开始查询成本模板")

	// 检查TEMU产品信息
	if temuCtx.TemuProduct == nil {
		return fmt.Errorf("TEMU产品信息为空")
	}

	temuProduct := temuCtx.TemuProduct

	// 检查API客户端
	if temuCtx.APIClient == nil {
		h.logger.Warn("API客户端未初始化，使用默认成本模板")
		return nil
	}

	// 构造成本模板查询请求
	request := h.buildCostTemplateRequest(temuProduct)

	// 发送成本模板查询请求
	err := h.queryCostTemplate(temuCtx, temuProduct, request)
	if err != nil {
		h.logger.WithError(err).Warn("查询成本模板失败，使用默认模板")
		// 设置默认成本模板
		return fmt.Errorf("无法解析成本模板ID，终止处理")
	}

	h.logger.Info("成本模板处理完成")
	return nil
}

// buildCostTemplateRequest 构造成本模板查询请求
func (h *CostTemplateHandler) buildCostTemplateRequest(temuProduct *temuproduct.Product) *temuquery.CostTemplateRequest {
	request := &temuquery.CostTemplateRequest{
		ClickType:            "8",  // 根据实际API调用设置
		ListingCommitVersion: "1",  // 默认版本
		QueryAll:             true, // 查询所有模板
	}

	// 从产品数据中获取实际的参数值
	if temuProduct != nil {
		if temuProduct.GoodsBasic.ListingCommitID != "" {
			request.ListingCommitID = temuProduct.GoodsBasic.ListingCommitID
		}
		if temuProduct.GoodsBasic.GoodsCommitID != "" {
			request.GoodsCommitID = temuProduct.GoodsBasic.GoodsCommitID
		}
		if temuProduct.GoodsBasic.GoodsID != "" {
			request.GoodsID = temuProduct.GoodsBasic.GoodsID
		}
		if temuProduct.GoodsBasic.CatID > 0 {
			request.CatID = temuProduct.GoodsBasic.CatID
		}
	}

	return request
}

// queryCostTemplate 发送成本模板查询请求到TEMU API
func (h *CostTemplateHandler) queryCostTemplate(temuCtx *temucontext.TemuTaskContext, temuProduct *temuproduct.Product, request *temuquery.CostTemplateRequest) error {
	// 创建QueryAPI实例
	queryAPI := api.NewQueryAPI(temuCtx.APIClient, h.logger)

	// 发送请求
	response, err := queryAPI.QueryCostTemplate(request)
	if err != nil {
		return fmt.Errorf("成本模板查询失败: %v", err)
	}

	templateCount := 0
	if response.Result != nil {
		templateCount = len(response.Result.CostTemplateList)
	}

	h.logger.WithFields(logrus.Fields{
		"listingCommitID": request.ListingCommitID,
		"goodsCommitID":   request.GoodsCommitID,
		"catID":           request.CatID,
		"success":         response.Success,
		"errorCode":       response.ErrorCode,
		"templateCount":   templateCount,
	}).Info("成本模板查询响应")

	// 从响应中解析成本模板ID
	if temuProduct != nil {
		costTemplateID := h.extractCostTemplateID(response)
		if costTemplateID != "" {
			temuProduct.GoodsServicePromise.CostTemplateID = costTemplateID
			h.logger.WithField("cost_template_id", costTemplateID).Info("设置成本模板ID")
		} else {
			// 如果无法解析成本模板ID，终止处理
			return fmt.Errorf("无法解析成本模板ID，终止处理")
		}
	}

	return nil
}

// extractCostTemplateID 从响应中提取成本模板ID
func (h *CostTemplateHandler) extractCostTemplateID(response *temuquery.CostTemplateResponse) string {
	if response == nil || response.Result == nil {
		h.logger.Warn("响应数据为空")
		return ""
	}

	if len(response.Result.CostTemplateList) == 0 {
		h.logger.Warn("成本模板列表为空")
		return ""
	}

	// 优先选择默认模板
	for _, template := range response.Result.CostTemplateList {
		if template.DefaultTemplate && !template.Disabled {
			h.logger.WithFields(map[string]interface{}{
				"template_name": template.TemplateName,
				"template_id":   template.CostTemplateID,
			}).Info("选择默认成本模板")
			return template.CostTemplateID
		}
	}

	// 如果没有默认模板，选择第一个可用的模板
	for _, template := range response.Result.CostTemplateList {
		if !template.Disabled {
			h.logger.WithFields(map[string]interface{}{
				"template_name": template.TemplateName,
				"template_id":   template.CostTemplateID,
			}).Info("选择第一个可用成本模板")
			return template.CostTemplateID
		}
	}

	// 如果所有模板都被禁用，选择第一个模板
	if len(response.Result.CostTemplateList) > 0 {
		template := response.Result.CostTemplateList[0]
		h.logger.WithFields(map[string]interface{}{
			"template_name": template.TemplateName,
			"template_id":   template.CostTemplateID,
		}).Warn("所有模板都被禁用，强制选择第一个")
		return template.CostTemplateID
	}

	h.logger.Warn("响应中未找到任何成本模板")
	return ""
}
