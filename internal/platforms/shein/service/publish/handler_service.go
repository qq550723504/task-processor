// Package modules 提供SHEIN平台产品发布核心处理器
package publish

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	product "task-processor/internal/platforms/shein/api/product"
	"task-processor/internal/platforms/shein/model"

	"github.com/sirupsen/logrus"
)

// PublishProductHandler 发布产品处理器
type PublishProductHandler struct {
	validator    *PublishProductValidator
	errorHandler *PublishProductErrorHandler
	saver        *PublishProductSaver
	checker      *PublishProductChecker
}

// NewPublishProductHandler 创建新的发布产品处理器
func NewPublishProductHandler() *PublishProductHandler {
	return &PublishProductHandler{
		validator:    NewPublishProductValidator(),
		errorHandler: NewPublishProductErrorHandler(),
		saver:        NewPublishProductSaver(),
		checker:      NewPublishProductChecker(),
	}
}

// Name 返回处理器名称
func (h *PublishProductHandler) Name() string {
	return "发布产品"
}

// Handle 执行发布产品处理
func (h *PublishProductHandler) Handle(ctx *model.TaskContext) error {
	// 检查是否已获取产品数据
	if ctx.ProductData == nil {
		// 这是一个程序逻辑错误，不应该发生，不可重试
		return model.NewNonRetryableError("产品数据未获取，请先执行获取产品数据步骤", nil)
	}

	// 检查是否已获取店铺客户端
	if ctx.ShopClient == nil {
		// 这是一个程序逻辑错误，不应该发生，不可重试
		return model.NewNonRetryableError("店铺客户端未获取，请先执行获取店铺API客户端步骤", nil)
	}

	// 产品存在性检查已移至管道早期阶段执行

	// 方案3：发布前预验证
	logrus.Info("🔍 开始发布前预验证...")

	if err := h.validator.PreValidateProductData(ctx); err != nil {
		logrus.Errorf("❌ 发布前预验证失败: %v", err)

		// 验证失败时保存产品数据到JSON文件用于调试
		if ctx.Task != nil && ctx.ProductData != nil {
			taskID := fmt.Sprintf("%d", ctx.Task.ID)
			if jsonData, jsonErr := h.marshalWithoutHTMLEscape(ctx.ProductData); jsonErr == nil {
				filename := fmt.Sprintf("%s_%s_validation_failed.json", ctx.Task.ProductID, taskID)
				if saveErr := h.saveJSONToFileWithName(filename, jsonData); saveErr != nil {
					logrus.Errorf("保存验证失败JSON文件失败: %v", saveErr)
				} else {
					logrus.Infof("📄 验证失败时产品数据已保存: %s", filename)
				}
			} else {
				logrus.Errorf("序列化验证失败产品数据失败: %v", jsonErr)
			}
		}

		// 预验证失败通常是数据问题，可重试（可能通过重新处理解决）
		return model.NewRetryableError("发布前预验证失败", err)
	}

	logrus.Info("✅ 发布前预验证通过")

	// 保存产品数据到JSON文件用于调试
	if ctx.Task != nil && ctx.ProductData != nil {
		taskID := fmt.Sprintf("%d", ctx.Task.ID)
		if jsonData, jsonErr := h.marshalWithoutHTMLEscape(ctx.ProductData); jsonErr == nil {
			if saveErr := h.saveJSONToFile(taskID, jsonData, ctx.Task.ProductID); saveErr != nil {
				logrus.Errorf("保存JSON文件失败: %v", saveErr)
			}
		} else {
			logrus.Errorf("序列化产品数据失败: %v", jsonErr)
		}
	}

	// 发布产品
	response, err := h.publishProduct(ctx)
	if err != nil {
		// 发布失败可能是网络问题或临时性错误，可重试
		return model.NewRetryableError("发布产品失败", err)
	}

	return h.errorHandler.HandlePublishResponse(ctx, response)
}

// publishProduct 统一的产品发布方法
func (h *PublishProductHandler) publishProduct(ctx *model.TaskContext) (*product.SheinResponse, error) {
	response, _, err := ctx.ShopClient.PublishProduct(ctx.ProductData)

	// 保存产品发布结果
	ctx.SheinResponse = response

	return response, err
}

// SaveDraftProduct 保存产品到草稿箱
func (h *PublishProductHandler) SaveDraftProduct(ctx *model.TaskContext) (*product.SheinResponse, error) {
	response, _, err := ctx.ShopClient.SaveDraftProduct(ctx.ProductData)
	if err != nil {
		return nil, err
	}

	// 保存到草稿箱成功后，更新任务状态为草稿箱
	h.saver.UpdateTaskStatusToDraft(ctx)

	return response, nil
}

// marshalWithoutHTMLEscape 序列化JSON但不转义HTML字符
func (h *PublishProductHandler) marshalWithoutHTMLEscape(v any) ([]byte, error) {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false) // 关闭HTML转义，避免&被转义为\u0026

	if err := encoder.Encode(v); err != nil {
		return nil, err
	}

	// 移除最后的换行符
	result := buf.Bytes()
	if len(result) > 0 && result[len(result)-1] == '\n' {
		result = result[:len(result)-1]
	}

	return result, nil
}

// saveJSONToFile 保存JSON数据到文件
func (h *PublishProductHandler) saveJSONToFile(taskID string, jsonData []byte, prefix string) error {
	// 创建文件名
	filename := fmt.Sprintf("%s_%s.json", prefix, taskID)
	return h.saveJSONToFileWithName(filename, jsonData)
}

// saveJSONToFileWithName 使用指定文件名保存JSON数据到文件
func (h *PublishProductHandler) saveJSONToFileWithName(filename string, jsonData []byte) error {
	// 确保目录存在
	if err := os.MkdirAll("logs", 0755); err != nil {
		return fmt.Errorf("创建日志目录失败: %w", err)
	}

	// 写入文件
	filePath := filepath.Join("logs", filename)
	if err := os.WriteFile(filePath, jsonData, 0644); err != nil {
		return fmt.Errorf("写入文件失败: %w", err)
	}

	logrus.Infof("JSON数据已保存到文件: %s", filePath)
	return nil
}
