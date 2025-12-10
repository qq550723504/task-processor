package handlers

import (
	"encoding/json"
	"fmt"
	"task-processor/platforms/amazon"

	"github.com/sirupsen/logrus"
)

// DataParserHandler 数据解析处理器
// 将1688的JSON数据解析为结构化数据
type DataParserHandler struct{}

// NewDataParserHandler 创建数据解析处理器
func NewDataParserHandler() *DataParserHandler {
	return &DataParserHandler{}
}

// Name 返回处理器名称
func (h *DataParserHandler) Name() string {
	return "解析1688产品数据"
}

// Handle 处理逻辑
func (h *DataParserHandler) Handle(ctx *amazon.TaskContext) error {
	logrus.Info("[DataParser] 开始解析1688产品数据")

	// 获取原始JSON数据
	rawJSON, exists := ctx.GetData("raw_json_data")
	if !exists {
		return fmt.Errorf("原始JSON数据不存在")
	}

	rawJSONStr, ok := rawJSON.(string)
	if !ok {
		return fmt.Errorf("原始JSON数据类型错误")
	}

	// 解析JSON为map
	var productData map[string]interface{}
	if err := json.Unmarshal([]byte(rawJSONStr), &productData); err != nil {
		return fmt.Errorf("解析JSON失败: %w", err)
	}

	logrus.Infof("[DataParser] 成功解析1688产品数据，字段数: %d", len(productData))

	// 保存解析后的数据
	ctx.SetData("raw_product_data", productData)

	// 记录关键字段
	h.logKeyFields(productData)

	return nil
}

// logKeyFields 记录关键字段
func (h *DataParserHandler) logKeyFields(data map[string]interface{}) {
	keyFields := []string{
		"title", "subject", "name",
		"price", "salePrice",
		"imageUrl", "images",
		"description", "desc",
		"brand",
		"category", "categoryName",
	}

	logrus.Info("[DataParser] 1688产品关键字段:")
	for _, field := range keyFields {
		if value, exists := data[field]; exists {
			logrus.Infof("  - %s: %v", field, value)
		}
	}
}
