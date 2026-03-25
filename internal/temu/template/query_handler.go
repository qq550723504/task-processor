package template

import (
	"fmt"

	"task-processor/internal/core/logger"
	temutemplate "task-processor/internal/temu/api/template"
	temucontext "task-processor/internal/temu/context"

	"github.com/sirupsen/logrus"
)

type TemplateQueryHandler struct {
	logger *logrus.Entry
}

func NewTemplateQueryHandler() *TemplateQueryHandler {
	return &TemplateQueryHandler{
		logger: logger.GetGlobalLogger("temu.handlers.template_query").WithField("handler", "TemplateQueryHandler"),
	}
}

func (h *TemplateQueryHandler) Name() string {
	return "template_query_handler"
}

func (h *TemplateQueryHandler) HandleTemu(temuCtx *temucontext.TemuTaskContext) error {
	h.logger.Info("start template query handler")

	if temuCtx.APIClient == nil {
		h.logger.Error("API client is not initialized, skip template query")
		return nil
	}

	input := BuildTemplateQueryInput(temuCtx)
	output, err := h.queryTemplate(temuCtx, input)
	if err != nil {
		h.logger.WithError(err).Error("template query failed")
		return err
	}
	applyTemplateQueryOutput(temuCtx, output)

	h.logger.Info("template query completed")
	return nil
}

func (h *TemplateQueryHandler) queryTemplate(temuCtx *temucontext.TemuTaskContext, input TemplateQueryInput) (*TemplateQueryOutput, error) {
	if temuCtx.TemuProduct == nil {
		h.logger.Warn("TemuProduct is nil, template query request will be partial")
	}
	if temuCtx.APIClient == nil {
		return nil, fmt.Errorf("API client is not initialized")
	}
	if temuCtx.QueryAPI == nil {
		return nil, fmt.Errorf("query API is not initialized")
	}

	request := input.ToRequest()
	response, err := temuCtx.QueryAPI.QueryTemplateAdvanced(&request)
	if err != nil {
		return nil, fmt.Errorf("query template request failed: %w", err)
	}
	output := buildTemplateQueryOutput(&response.Result)

	h.logger.WithFields(logrus.Fields{
		"listingCommitID":     request.ListingCommitID,
		"goodsCommitID":       request.GoodsCommitID,
		"catID":               request.CatID,
		"templateID":          response.Result.TemplateInfo.TemplateID,
		"specPropertiesCount": len(response.Result.TemplateInfo.GoodsSpecProperties),
		"success":             response.Success,
	}).Info("template query succeeded")

	return output, nil
}

func GetTemplateInfoFromContext(temuCtx *temucontext.TemuTaskContext) (*temutemplate.TemplateInfo, bool) {
	return temuCtx.TemplateInfo, temuCtx.TemplateInfo != nil
}

func GetUserInputParentSpecListFromContext(temuCtx *temucontext.TemuTaskContext) ([]temutemplate.UserInputParentSpec, bool) {
	return temuCtx.UserInputParentSpecList, len(temuCtx.UserInputParentSpecList) > 0
}
