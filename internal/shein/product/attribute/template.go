package attribute

import (
	"fmt"

	"task-processor/internal/core/logger"
	"task-processor/internal/pkg/jsonx"
	sheinctx "task-processor/internal/shein/context"
)

type AttributeTemplateHandler struct{}

func NewAttributeTemplateHandler() *AttributeTemplateHandler {
	return &AttributeTemplateHandler{}
}

func (h *AttributeTemplateHandler) Name() string {
	return "attribute_template"
}

func (h *AttributeTemplateHandler) Handle(ctx *sheinctx.TaskContext) error {
	categoryID := ctx.ProductData.CategoryID
	if categoryID == 0 {
		return fmt.Errorf("category id is not set")
	}

	logger.GetGlobalLogger("shein/product").Debugf("load attribute templates: category_id=%d", categoryID)
	attributeTemplates, err := ctx.AttributeAPI.GetAttributeTemplates(categoryID)
	if err != nil {
		return fmt.Errorf("get attribute templates failed: %w", err)
	}

	ctx.SetAttributeTemplates(attributeTemplates)
	logger.GetGlobalLogger("shein/product").Infof("loaded attribute templates: total=%d", len(attributeTemplates.Data))
	return nil
}

func (h *AttributeTemplateHandler) marshalWithoutHTMLEscape(v any) ([]byte, error) {
	return jsonx.MarshalWithoutHTMLEscape(v)
}

func (h *AttributeTemplateHandler) saveJSONToFileWithName(filename string, jsonData []byte) error {
	if err := jsonx.SaveToFile(filename, jsonData); err != nil {
		return err
	}
	logger.GetGlobalLogger("shein/product").Infof("saved JSON to logs/%s", filename)
	return nil
}
