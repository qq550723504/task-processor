package handlers

import (
	"fmt"
	"task-processor/internal/pipeline"
	"task-processor/internal/platforms/temu/api"
	"task-processor/internal/platforms/temu/api/models"
	temucontext "task-processor/internal/platforms/temu/context"

	"github.com/sirupsen/logrus"
)

// CategoryDisclaimHandler 分类免责声明处理器
type CategoryDisclaimHandler struct {
	logger *logrus.Entry
}

// NewCategoryDisclaimHandler 创建新的分类免责声明处理器
func NewCategoryDisclaimHandler() *CategoryDisclaimHandler {
	return &CategoryDisclaimHandler{
		logger: logrus.WithField("handler", "CategoryDisclaimHandler"),
	}
}

// Name 返回处理器名称
func (h *CategoryDisclaimHandler) Name() string {
	return "分类免责声明处理器"
}

// Handle 处理任务（兼容pipeline.Handler接口）
func (h *CategoryDisclaimHandler) Handle(ctx pipeline.TaskContext) error {
	// 类型断言为强类型上下文
	temuCtx, ok := ctx.(*temucontext.TemuTaskContext)
	if !ok {
		return fmt.Errorf("上下文类型错误，期望TemuTaskContext")
	}
	return h.HandleTemu(temuCtx)
}

// HandleTemu 处理任务（强类型上下文）
func (h *CategoryDisclaimHandler) HandleTemu(temuCtx *temucontext.TemuTaskContext) error {
	h.logger.Info("开始处理分类免责声明")

	// 获取任务信息
	task := temuCtx.GetTask()
	if task == nil {
		return fmt.Errorf("任务信息为空")
	}

	// 从强类型上下文获取TEMU产品信息
	if temuCtx.TemuProduct == nil {
		return fmt.Errorf("TEMU产品信息为空")
	}

	temuProduct := temuCtx.TemuProduct

	// 检查分类ID是否存在
	if temuProduct.GoodsBasic.CatID == 0 {
		h.logger.Warn("分类ID为空，跳过免责声明处理")
		return nil
	}

	// 获取分类免责声明
	err := h.getCategoryDisclaimer(temuCtx, temuProduct)
	if err != nil {
		h.logger.Warnf("获取分类免责声明警告: %v", err)
		// 免责声明获取失败不阻止流程继续
	}

	h.logger.Info("分类免责声明处理完成")
	return nil
}

// getCategoryDisclaimer 获取分类免责声明
func (h *CategoryDisclaimHandler) getCategoryDisclaimer(temuCtx *temucontext.TemuTaskContext, temuProduct *models.Product) error {
	catID := temuProduct.GoodsBasic.CatID
	if catID == 0 {
		return fmt.Errorf("分类ID为空")
	}

	h.logger.Infof("获取分类免责声明: CatID=%d", catID)

	// 获取API客户端
	if temuCtx.APIClient == nil {
		h.logger.Warn("API客户端未初始化，使用默认免责声明")
		return nil
	}

	// 创建CategoryAPI
	categoryAPI := api.NewCategoryAPI(temuCtx.APIClient, h.logger)

	// 调用API获取分类免责声明
	response, err := categoryAPI.GetCategoryDisclaimer(int(catID))
	if err != nil {
		h.logger.Warnf("API获取免责声明失败，使用默认免责声明: %v", err)
		return err
	}

	// 设置免责声明到产品
	disclaimer := models.Disclaimer{
		PromptList: response.Result.DisclaimerDTO.PromptList,
	}
	temuProduct.GoodsBasic.CategoryDisclaimer = disclaimer

	h.logger.Infof("成功设置分类免责声明: %d 条提示", len(disclaimer.PromptList))
	return nil
}
