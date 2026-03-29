package rules

import (
	"fmt"
	"task-processor/internal/core/logger"
	"task-processor/internal/pipeline"
	"task-processor/internal/temu/api"
	temuquery "task-processor/internal/temu/api/query"
	temucontext "task-processor/internal/temu/context"
)

// TextCheckHandler 文本检查处理器
type TextCheckHandler struct{}

// NewTextCheckHandler 创建新的文本检查处理器
func NewTextCheckHandler() *TextCheckHandler {
	return &TextCheckHandler{}
}

// Name 返回处理器名称
func (h *TextCheckHandler) Name() string {
	return "文本检查"
}

// Handle 处理任务（兼容pipeline.Handler接口）
func (h *TextCheckHandler) Handle(ctx pipeline.TaskContext) error {
	// 类型断言为强类型上下文
	temuCtx, ok := ctx.(*temucontext.TemuTaskContext)
	if !ok {
		return fmt.Errorf("上下文类型错误，期望TemuTaskContext")
	}
	return h.HandleTemu(temuCtx)
}

// HandleTemu 处理任务（强类型上下文）
func (h *TextCheckHandler) HandleTemu(temuCtx *temucontext.TemuTaskContext) error {
	// 实现文本检查逻辑
	log := logger.GetGlobalLogger("temu.handlers.text_check")
	log.Info("执行文本检查")

	// 获取Amazon产品数据
	amazonProduct := temuCtx.GetAmazonProduct()
	if amazonProduct == nil {
		log.Warn("Amazon产品数据为空，跳过文本检查")
		return nil
	}

	// 示例文本检查请求
	content := amazonProduct.Title

	// 发送文本检查请求
	err := h.checkText(temuCtx, content)
	if err != nil {
		log.WithError(err).Error("文本检查失败")
		return err
	}

	// 文本检查通过后，将标题赋值给TEMU产品
	err = h.assignTitleToTemuProduct(temuCtx, content)
	if err != nil {
		log.WithError(err).Error("赋值标题到TEMU产品失败")
		return err
	}

	log.Info("文本检查完成")
	return nil
}

// checkText 发送文本检查请求到TEMU API
func (h *TextCheckHandler) checkText(temuCtx *temucontext.TemuTaskContext, content string) error {
	log := logger.GetGlobalLogger("temu.handlers.text_check")

	// 检查API客户端
	if temuCtx.APIClient == nil {
		log.Error("API客户端未初始化")
		return fmt.Errorf("API客户端未初始化")
	}

	// 创建QueryAPI
	queryAPI := api.NewQueryAPI(temuCtx.APIClient, logger.GetGlobalLogger("TextCheckHandler"))

	// 构造请求
	request := &temuquery.TextCheckRequest{
		Content: content,
		Type:    1,
	}

	// 调用API检查文本
	response, err := queryAPI.CheckText(request)
	if err != nil {
		log.WithFields(map[string]any{
			"content": content,
		}).WithError(err).Error("文本检查API调用失败")
		return fmt.Errorf("文本检查失败: %w", err)
	}

	if response == nil {
		log.Error("文本检查响应为空")
		return fmt.Errorf("文本检查响应为空")
	}

	if !response.Result.Success {
		log.WithField("content", content).Warn("文本检查未通过")
		return fmt.Errorf("文本检查未通过")
	}

	log.WithField("content", content).Info("文本检查通过")
	return nil
}

// assignTitleToTemuProduct 将检查通过的标题赋值给TEMU产品
func (h *TextCheckHandler) assignTitleToTemuProduct(temuCtx *temucontext.TemuTaskContext, checkedTitle string) error {
	log := logger.GetGlobalLogger("temu.handlers.text_check")

	// 检查TEMU产品信息
	if temuCtx.TemuProduct == nil {
		return fmt.Errorf("TEMU产品信息为空")
	}

	// 设置商品名称到GoodsBasic中
	temuCtx.TemuProduct.GoodsBasic.GoodsName = checkedTitle

	log.WithField("title", checkedTitle).Info("已将检查通过的标题设置到TEMU产品")
	return nil
}
