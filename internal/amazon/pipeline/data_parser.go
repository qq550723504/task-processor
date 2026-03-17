// package pipeline 提供Amazon数据解析处理器
package pipeline

import (
	"context"
	"fmt"
	"task-processor/internal/amazon/model"
	"task-processor/internal/pkg/jsonx"
)

// DataParserHandler 数据解析处理器
// 将1688的JSON数据解析为结构化数据
type DataParserHandler struct {
	*BaseHandler
}

// NewDataParserHandler 创建数据解析处理器
func NewDataParserHandler(services *model.Services) *DataParserHandler {
	return &DataParserHandler{
		BaseHandler: NewBaseHandler("数据解析器"),
	}
}

// Handle 处理逻辑
func (h *DataParserHandler) Handle(ctx context.Context, taskContext *model.TaskContext) error {
	h.logger.Info("开始解析1688产品数据")

	// 获取原始JSON数据
	rawJSON, exists := taskContext.Data["raw_json_data"]
	if !exists {
		return fmt.Errorf("原始JSON数据不存在")
	}

	rawJSONStr, ok := rawJSON.(string)
	if !ok {
		return fmt.Errorf("原始JSON数据类型错误")
	}

	// 解析JSON为map
	var productData map[string]any
	if err := jsonx.UnmarshalString(rawJSONStr, &productData, "解析JSON失败"); err != nil {
		return err
	}

	h.logger.Infof("成功解析1688产品数据，字段数: %d", len(productData))

	// 保存解析后的数据
	taskContext.SetResult("raw_product_data", productData)

	// 记录关键字段
	h.logKeyFields(productData)

	return nil
}

// logKeyFields 记录关键字段
func (h *DataParserHandler) logKeyFields(data map[string]any) {
	keyFields := []string{
		"title", "subject", "name",
		"price", "salePrice",
		"imageUrl", "images",
		"description", "desc",
		"brand",
		"category", "categoryName",
	}

	h.logger.Info("1688产品关键字段:")
	for _, field := range keyFields {
		if value, exists := data[field]; exists {
			h.logger.Infof("  - %s: %v", field, value)
		}
	}
}
