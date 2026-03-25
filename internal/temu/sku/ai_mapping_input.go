package sku

import (
	"fmt"

	"task-processor/internal/model"
	temutemplate "task-processor/internal/temu/api/template"
	temucontext "task-processor/internal/temu/context"
	"task-processor/internal/temu/template"
)

type AIMappingInput struct {
	AmazonProduct      *model.Product
	Variants           []*model.Product
	TemuSpecProperties []temutemplate.TemplateRespGoodsSpecProperty
}

func BuildAIMappingInput(temuCtx *temucontext.TemuTaskContext, variants []*model.Product, specHandler *SkuSpecHandler) (*AIMappingInput, error) {
	if temuCtx == nil {
		return nil, fmt.Errorf("temu context is nil")
	}
	if specHandler == nil {
		return nil, fmt.Errorf("spec handler is nil")
	}
	if temuCtx.AmazonProduct == nil {
		return nil, fmt.Errorf("amazon product is nil")
	}

	return &AIMappingInput{
		AmazonProduct:      temuCtx.AmazonProduct,
		Variants:           variants,
		TemuSpecProperties: buildTemuSpecProperties(temuCtx, specHandler),
	}, nil
}

func buildTemuSpecProperties(temuCtx *temucontext.TemuTaskContext, specHandler *SkuSpecHandler) []temutemplate.TemplateRespGoodsSpecProperty {
	if templateInfo, exists := template.GetTemplateInfoFromContext(temuCtx); exists {
		if len(templateInfo.GoodsSpecProperties) > 0 {
			return templateInfo.GoodsSpecProperties
		}
	}

	if userInputSpecs, exists := template.GetUserInputParentSpecListFromContext(temuCtx); exists {
		return specHandler.convertUserInputSpecsToGoodsSpecProperties(userInputSpecs)
	}

	return []temutemplate.TemplateRespGoodsSpecProperty{}
}
