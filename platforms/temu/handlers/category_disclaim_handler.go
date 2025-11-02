package handlers

import (
	"fmt"
	"task-processor/common/pipeline"
	"task-processor/platforms/temu/types"

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

// Handle 处理任务
func (h *CategoryDisclaimHandler) Handle(ctx *pipeline.TaskContext) error {
	h.logger.Info("开始处理分类免责声明")

	// 检查任务上下文中的必要数据
	if ctx.Task == nil {
		return fmt.Errorf("任务信息为空")
	}

	if ctx.TemuProduct == nil {
		return fmt.Errorf("TEMU产品信息为空")
	}

	// 获取分类免责声明
	err := h.getCategoryDisclaimer(ctx)
	if err != nil {
		h.logger.Warnf("获取分类免责声明警告: %v", err)
		// 免责声明获取失败不阻止流程继续
	}

	h.logger.Info("分类免责声明处理完成")
	return nil
}

// getCategoryDisclaimer 获取分类免责声明
func (h *CategoryDisclaimHandler) getCategoryDisclaimer(ctx *pipeline.TaskContext) error {
	catID := ctx.TemuProduct.GoodsBasic.CatID
	if catID == 0 {
		return fmt.Errorf("分类ID为空")
	}

	h.logger.Infof("获取分类免责声明: CatID=%d", catID)

	// 这里应该调用TEMU API获取分类免责声明
	// 为了简化，我们模拟免责声明数据
	disclaimer := h.getDefaultDisclaimer(catID)

	// 设置免责声明到产品
	ctx.TemuProduct.GoodsBasic.CategoryDisclaimer = disclaimer

	h.logger.Infof("成功设置分类免责声明: %d 条提示", len(disclaimer.PromptList))
	return nil
}

// getDefaultDisclaimer 获取默认免责声明
func (h *CategoryDisclaimHandler) getDefaultDisclaimer(catID int) types.Disclaimer {
	// 根据不同分类返回不同的免责声明
	switch {
	case catID >= 30000 && catID < 40000: // 服装类
		return types.Disclaimer{
			PromptList: []string{
				"请确保产品符合当地服装安全标准",
				"请提供准确的尺码信息",
				"请注意面料成分标识",
			},
		}
	case catID >= 40000 && catID < 50000: // 电子产品类
		return types.Disclaimer{
			PromptList: []string{
				"请确保产品符合电子产品安全认证",
				"请提供准确的技术规格",
				"请注意电池安全要求",
			},
		}
	default: // 通用免责声明
		return types.Disclaimer{
			PromptList: []string{
				"请确保产品符合当地法律法规",
				"请提供准确的产品信息",
				"请注意产品质量要求",
			},
		}
	}
}
