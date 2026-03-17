package product

import (
	"fmt"
	"regexp"
	"strings"
	"task-processor/internal/core/logger"
	types "task-processor/internal/model"
	"task-processor/internal/pipeline"
	"task-processor/internal/pkg/strx"
	temuapi "task-processor/internal/temu/api"
	temucontext "task-processor/internal/temu/context"

	"github.com/sirupsen/logrus"
)

// CommitCreateHandler 提交创建处理器
type CommitCreateHandler struct {
	logger *logrus.Entry
}

// NewCommitCreateHandler 创建新的提交创建处理器
func NewCommitCreateHandler() *CommitCreateHandler {
	return &CommitCreateHandler{
		logger: logger.GetGlobalLogger("temu.handlers.commit_create").WithField("handler", "CommitCreateHandler"),
	}
}

// Name 返回处理器名称
func (h *CommitCreateHandler) Name() string {
	return "提交创建处理器"
}

// ensureParenthesesSpacing 确保括号前后有正确的空格（TEMU API要求）
func (h *CommitCreateHandler) ensureParenthesesSpacing(name string) string {
	// 1. 确保左括号前有空格（TEMU强制要求）
	name = regexp.MustCompile(`(\S)\(`).ReplaceAllString(name, "$1 (")

	// 2. 确保右括号后有空格（如果后面是字母或数字，但不是标点符号）
	name = regexp.MustCompile(`\)([a-zA-Z0-9])`).ReplaceAllString(name, ") $1")

	// 3. 清理多余的空格
	name = regexp.MustCompile(`\s+`).ReplaceAllString(name, " ")
	name = strings.TrimSpace(name)

	return name
}

// Handle 处理任务（兼容pipeline.Handler接口）
func (h *CommitCreateHandler) Handle(ctx pipeline.TaskContext) error {
	// 类型断言为强类型上下文
	temuCtx, ok := ctx.(*temucontext.TemuTaskContext)
	if !ok {
		return fmt.Errorf("上下文类型错误，期望TemuTaskContext")
	}
	return h.HandleTemu(temuCtx)
}

// HandleTemu 处理任务（强类型上下文）
func (h *CommitCreateHandler) HandleTemu(temuCtx *temucontext.TemuTaskContext) error {
	h.logger.Info("开始创建商品提交")

	// 获取任务信息
	task := temuCtx.GetTask()
	if task == nil {
		return fmt.Errorf("任务信息为空")
	}

	// 从上下文获取TEMU产品信息
	if temuCtx.TemuProduct == nil {
		return fmt.Errorf("TEMU产品信息为空")
	}

	temuProduct := temuCtx.TemuProduct

	// 验证必要的产品信息
	if err := h.validateProductInfo(task, temuProduct); err != nil {
		h.logger.WithError(err).Error("产品信息验证失败")
		return fmt.Errorf("产品信息验证失败: %w", err)
	}

	// 创建商品提交
	err := h.createCommit(temuCtx, temuProduct)
	if err != nil {
		h.logger.WithError(err).Error("创建商品提交失败")
		return fmt.Errorf("创建商品提交失败: %w", err)
	}

	h.logger.Info("商品提交创建完成")
	return nil
}

// createCommit 创建商品提交
func (h *CommitCreateHandler) createCommit(temuCtx *temucontext.TemuTaskContext, temuProduct *temuapi.Product) error {
	// 获取API客户端
	if temuCtx.APIClient == nil {
		h.logger.Error("API客户端未初始化")
		return fmt.Errorf("API客户端未初始化")
	}

	// 清理商品名称，确保符合TEMU要求
	cleanedGoodsName := strx.CleanProductTitle(temuProduct.GoodsBasic.GoodsName)

	// 【关键修复】确保左括号前有空格（TEMU API强制要求）
	cleanedGoodsName = h.ensureParenthesesSpacing(cleanedGoodsName)

	if cleanedGoodsName != temuProduct.GoodsBasic.GoodsName {
		temuProduct.GoodsBasic.GoodsName = cleanedGoodsName
	}

	// 构造请求体 - 使用简化的结构匹配工作版本
	request := &temuapi.CreateCommitRequest{
		CatIDs:      temuProduct.GoodsBasic.CatIDs,
		CatID:       temuProduct.GoodsBasic.CatID,
		GoodsName:   cleanedGoodsName,
		OperateType: 1,
	}

	// 创建SubmitAPI实例
	submitAPI := temuapi.NewSubmitAPI(temuCtx.APIClient, h.logger)

	// 发送API请求
	response, err := submitAPI.CreateCommit(request)
	if err != nil {
		h.logger.WithError(err).Error("创建商品提交API调用失败")

		// 检查是否为不可重试的错误（从错误消息中解析）
		if strings.Contains(err.Error(), "errorCode=") {
			// 尝试从错误消息中提取错误码
			if strings.Contains(err.Error(), "errorCode=10000003") ||
				strings.Contains(err.Error(), "errorCode=10000046") ||
				strings.Contains(err.Error(), "errorCode=10000104") ||
				strings.Contains(err.Error(), "errorCode=10000105") {
				h.logger.WithError(err).Error("检测到不可重试错误")
				return fmt.Errorf("TERMINATED: %w", err)
			}
		}

		return fmt.Errorf("创建商品提交API调用失败: %w", err)
	}

	// 检查结果数据
	if response.Result == nil {
		return fmt.Errorf("创建商品提交成功但结果数据为空")
	}

	// 将创建的提交信息保存到产品数据中
	temuProduct.GoodsBasic.ListingCommitID = response.Result.ListingCommitID
	temuProduct.GoodsBasic.GoodsCommitID = response.Result.GoodsCommitID
	temuProduct.GoodsBasic.GoodsID = response.Result.GoodsID
	temuProduct.GoodsBasic.ListingCommitVersion = response.Result.ListingCommitVersion

	// 更新产品数据中的商品名称为清理后的版本
	temuProduct.GoodsBasic.GoodsName = cleanedGoodsName

	return nil
}

// validateProductInfo 验证产品信息
func (h *CommitCreateHandler) validateProductInfo(task *types.Task, temuProduct *temuapi.Product) error {
	// 检查店铺ID（这是创建提交必需的）
	if task.StoreID == 0 {
		return fmt.Errorf("店铺ID不能为空")
	}

	// 检查是否有基本的产品数据结构
	if temuProduct == nil {
		return fmt.Errorf("TEMU产品信息未初始化")
	}

	h.logger.Info("产品信息验证通过，准备创建提交")
	return nil
}

// isNonRetryableError 判断错误是否不可重试
func (h *CommitCreateHandler) isNonRetryableError(errorCode int) bool {
	// 定义不可重试的错误码
	nonRetryableErrorCodes := map[int]string{
		10000003: "产品名称为空或无效",
		10000046: "类目不可用",
		10000104: "商品已存在",
		10000105: "商品ID重复",
	}

	// 检查错误码
	if reason, exists := nonRetryableErrorCodes[errorCode]; exists {
		h.logger.WithFields(map[string]any{
			"error_code": errorCode,
			"reason":     reason,
		}).Info("识别到不可重试错误")
		return true
	}

	return false
}

