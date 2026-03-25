package attribute

import (
	"fmt"

	"task-processor/internal/core/logger"
	"task-processor/internal/pkg/jsonx"
	apiattribute "task-processor/internal/shein/api/attribute"
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
	input, err := buildAttributeTemplateInput(ctx)
	if err != nil {
		return err
	}

	logger.GetGlobalLogger("shein/product").Debugf("load attribute templates: category_id=%d", input.CategoryID)
	attributeTemplates, err := h.loadAttributeTemplates(input)
	if err != nil {
		return fmt.Errorf("get attribute templates failed: %w", err)
	}

	ctx.SetAttributeTemplates(attributeTemplates)
	logger.GetGlobalLogger("shein/product").Infof("loaded attribute templates: total=%d", len(attributeTemplates.Data))
	return nil
}

func (h *AttributeTemplateHandler) loadAttributeTemplates(input *AttributeTemplateInput) (*apiattribute.AttributeTemplateInfo, error) {
	return input.AttributeAPI.GetAttributeTemplates(input.CategoryID)
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
