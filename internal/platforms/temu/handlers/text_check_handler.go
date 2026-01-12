package handlers

import (
	"fmt"
	"task-processor/internal/pipeline"
	"task-processor/internal/platforms/temu/api"
	"task-processor/internal/platforms/temu/api/models"
	temucontext "task-processor/internal/platforms/temu/context"

	"github.com/sirupsen/logrus"
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
	logrus.Println("执行文本检查")

	// 获取Amazon产品数据
	amazonProduct := temuCtx.GetAmazonProduct()
	if amazonProduct == nil {
		logrus.Warn("Amazon产品数据为空，跳过文本检查")
		return nil
	}

	// 示例文本检查请求
	content := amazonProduct.Title

	// 发送文本检查请求
	err := h.checkText(temuCtx, content)
	if err != nil {
		logrus.Errorf("文本检查失败: %v", err)
		return err
	}

	// 文本检查通过后，将标题赋值给TEMU产品
	err = h.assignTitleToTemuProduct(temuCtx, content)
	if err != nil {
		logrus.Errorf("赋值标题到TEMU产品失败: %v", err)
		return err
	}

	logrus.Println("文本检查完成")
	return nil
}

// checkText 发送文本检查请求到TEMU API
func (h *TextCheckHandler) checkText(temuCtx *temucontext.TemuTaskContext, content string) error {
	// 检查API客户端
	if temuCtx.APIClient == nil {
		logrus.Error("API客户端未初始化")
		return fmt.Errorf("API客户端未初始化")
	}

	// 创建QueryAPI
	queryAPI := api.NewQueryAPI(temuCtx.APIClient, logrus.WithField("handler", "TextCheckHandler"))

	// 构造请求
	request := &models.TextCheckRequest{
		Text: content,
	}

	// 调用API检查文本
	response, err := queryAPI.CheckText(request)
	if err != nil {
		logrus.Errorf("文本检查失败: %v", err)
		return fmt.Errorf("文本检查失败: %v", err)
	}

	logrus.Infof("文本检查成功: 有效=%v, 消息=%s", response.Result.IsValid, response.Result.Message)

	if !response.Result.IsValid {
		return fmt.Errorf("文本检查未通过: %s", response.Result.Message)
	}

	return nil
}

// assignTitleToTemuProduct 将检查通过的标题赋值给TEMU产品
func (h *TextCheckHandler) assignTitleToTemuProduct(temuCtx *temucontext.TemuTaskContext, checkedTitle string) error {
	// 检查TEMU产品信息
	if temuCtx.TemuProduct == nil {
		return fmt.Errorf("TEMU产品信息为空")
	}

	// 设置商品名称到GoodsBasic中
	temuCtx.TemuProduct.GoodsBasic.GoodsName = checkedTitle

	logrus.Infof("已将检查通过的标题设置到TEMU产品: %s", checkedTitle)
	return nil
}
