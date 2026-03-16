package attribute

import (
	"fmt"
	"os"
	"path/filepath"
	"task-processor/internal/pkg/jsonutil"
	"task-processor/internal/platforms/shein"

	"github.com/sirupsen/logrus"
)

// AttributeTemplateHandler 属性模板处理器
type AttributeTemplateHandler struct {
}

// NewAttributeTemplateHandler 创建新的属性模板处理器
func NewAttributeTemplateHandler() *AttributeTemplateHandler {
	return &AttributeTemplateHandler{}
}

// Name 返回处理器名称
func (h *AttributeTemplateHandler) Name() string {
	return "获取属性模板"
}

// Handle 执行获取属性模板处理
func (h *AttributeTemplateHandler) Handle(ctx *shein.TaskContext) error {

	categoryID := ctx.ProductData.CategoryID
	if categoryID == 0 {
		return fmt.Errorf("分类ID未设置，请先执行AI分类选择步骤")
	}

	logrus.Debugf("开始获取属性模板，分类ID: %d", categoryID)

	// 调用API获取属性模板
	attributeTemplates, err := ctx.AttributeAPI.GetAttributeTemplates(categoryID)
	if err != nil {
		return fmt.Errorf("获取属性模板失败: %w", err)
	}

	logrus.Infof("成功获取属性模板，模板数量: %d\n", len(attributeTemplates.Data))

	// 将属性模板信息存储到上下文中
	ctx.AttributeTemplates = attributeTemplates

	// // 保存属性模板数据到JSON文件用于调试
	// if ctx.Task != nil && attributeTemplates != nil {
	// 	taskID := fmt.Sprintf("%d", ctx.Task.ID)
	// 	if jsonData, jsonErr := h.marshalWithoutHTMLEscape(attributeTemplates); jsonErr == nil {
	// 		filename := fmt.Sprintf("%s_%s_attribute_templates.json", ctx.Task.ProductID, taskID)
	// 		if saveErr := h.saveJSONToFileWithName(filename, jsonData); saveErr != nil {
	// 			logrus.Errorf("保存属性模板JSON文件失败: %v", saveErr)
	// 		} else {
	// 			logrus.Infof("📄 属性模板数据已保存: %s", filename)
	// 		}
	// 	} else {
	// 		logrus.Errorf("序列化属性模板数据失败: %v", jsonErr)
	// 	}
	// }

	return nil
}

// marshalWithoutHTMLEscape 序列化JSON但不转义HTML字符
func (h *AttributeTemplateHandler) marshalWithoutHTMLEscape(v any) ([]byte, error) {
	return jsonutil.MarshalWithoutHTMLEscape(v)
}

// saveJSONToFileWithName 使用指定文件名保存JSON数据到文件
func (h *AttributeTemplateHandler) saveJSONToFileWithName(filename string, jsonData []byte) error {
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
