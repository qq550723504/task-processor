package handlers

import (
	"fmt"
	"regexp"
	"strings"
	"task-processor/internal/pipeline"
	temucontext "task-processor/internal/platforms/temu/context"
	temuTypes "task-processor/internal/platforms/temu/types"
	"task-processor/internal/types"
	"task-processor/internal/utils"

	"github.com/sirupsen/logrus"
)

// CommitCreateHandler 提交创建处理器
type CommitCreateHandler struct {
	logger *logrus.Entry
}

// CommitCreateRequest 提交创建请求结构体
type CommitCreateRequest struct {
	GoodsName  string `json:"goods_name"`
	CatID      int    `json:"cat_id"`
	StoreID    int64  `json:"store_id"`
	Lang       string `json:"lang,omitempty"`
	GoodsType  int    `json:"goods_type,omitempty"`
	Source     int    `json:"source,omitempty"`
	OutGoodsSN string `json:"out_goods_sn,omitempty"`
	Customized bool   `json:"customized,omitempty"`
	SecondHand bool   `json:"second_hand,omitempty"`
}

// CommitCreateResponse 提交创建响应结构体
type CommitCreateResponse struct {
	Success   bool                `json:"success"`
	ErrorCode int                 `json:"error_code"`
	Result    *CommitCreateResult `json:"result,omitempty"`
	Message   string              `json:"error_msg,omitempty"`
}

// CommitCreateResult 提交创建结果数据
type CommitCreateResult struct {
	GoodsID              string `json:"goods_id"`
	ListingCommitID      string `json:"listing_commit_id"`
	ListingCommitVersion string `json:"listing_commit_version"`
	GoodsCommitID        string `json:"goods_commit_id"`
}

// NewCommitCreateHandler 创建新的提交创建处理器
func NewCommitCreateHandler() *CommitCreateHandler {
	return &CommitCreateHandler{
		logger: logrus.WithField("handler", "CommitCreateHandler"),
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
		h.logger.Errorf("产品信息验证失败: %v", err)
		return fmt.Errorf("产品信息验证失败: %w", err)
	}

	// 创建商品提交
	err := h.createCommit(temuCtx, temuProduct)
	if err != nil {
		h.logger.Errorf("创建商品提交失败: %v", err)
		return fmt.Errorf("创建商品提交失败: %w", err)
	}

	h.logger.Info("商品提交创建完成")
	return nil
}

// createCommit 创建商品提交
func (h *CommitCreateHandler) createCommit(temuCtx *temucontext.TemuTaskContext, temuProduct *temuTypes.Product) error {
	// 获取API客户端
	if temuCtx.APIClient == nil {
		h.logger.Error("API客户端未初始化")
		return fmt.Errorf("API客户端未初始化")
	}

	// 清理商品名称，确保符合TEMU要求
	cleanedGoodsName := utils.CleanProductTitle(temuProduct.GoodsBasic.GoodsName)

	// 【关键修复】确保左括号前有空格（TEMU API强制要求）
	cleanedGoodsName = h.ensureParenthesesSpacing(cleanedGoodsName)

	if cleanedGoodsName != temuProduct.GoodsBasic.GoodsName {
		temuProduct.GoodsBasic.GoodsName = cleanedGoodsName
	}

	// 构造请求体 - 使用简化的结构匹配工作版本
	requestBody := map[string]interface{}{
		"cat_ids":      temuProduct.GoodsBasic.CatIDs,
		"cat_id":       temuProduct.GoodsBasic.CatID,
		"goods_name":   cleanedGoodsName,
		"operate_type": 1,
		//"select_category_source": 1,
	}

	// 构造API请求
	apiReq := map[string]interface{}{
		"method": "POST",
		"url":    "/mms/marigold/edit/commit/create_new",
		"headers": map[string]string{
			"accept":             "application/json, text/plain, */*",
			"accept-language":    "zh-CN,zh;q=0.9",
			"content-type":       "application/json;charset=UTF-8",
			"priority":           "u=1, i",
			"sec-ch-ua":          "\"Chromium\";v=\"140\", \"Not=A?Brand\";v=\"24\", \"Google Chrome\";v=\"140\"",
			"sec-ch-ua-mobile":   "?0",
			"sec-ch-ua-platform": "\"Windows\"",
			"sec-fetch-dest":     "empty",
			"sec-fetch-mode":     "cors",
			"sec-fetch-site":     "same-origin",
			"x-document-referer": "https://seller.temu.com/product-add.html?is_back=1",
		},
		"body": requestBody,
	}

	// 发送API请求（Cookie检查和重试逻辑已在API客户端中处理）
	response := &CommitCreateResponse{}

	// 类型断言获取TEMU API客户端
	type TEMUAPIClient interface {
		SendTEMURequest(request map[string]any, response any) error
	}

	if temuClient, ok := interface{}(temuCtx.APIClient).(TEMUAPIClient); ok {
		err := temuClient.SendTEMURequest(apiReq, response)
		if err != nil {
			h.logger.Errorf("创建商品提交API调用失败: %v", err)
			return fmt.Errorf("创建商品提交API调用失败: %w", err)
		}
	} else {
		return fmt.Errorf("API客户端不支持TEMU请求")
	}

	if !response.Success {
		errorMsg := fmt.Sprintf("创建商品提交失败，API返回失败状态 (错误码: %d)", response.ErrorCode)
		if response.Message != "" {
			errorMsg = fmt.Sprintf("%s: %s", errorMsg, response.Message)
		}

		// 检查是否为不可重试的错误
		if h.isNonRetryableError(response.ErrorCode) {
			h.logger.Errorf("检测到不可重试错误: %s", errorMsg)
			return fmt.Errorf("TERMINATED: %s", errorMsg)
		}

		return fmt.Errorf("%s", errorMsg)
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
func (h *CommitCreateHandler) validateProductInfo(task *types.Task, temuProduct *temuTypes.Product) error {
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
		h.logger.Infof("识别到不可重试错误: %s (error_code=%d)", reason, errorCode)
		return true
	}

	return false
}
