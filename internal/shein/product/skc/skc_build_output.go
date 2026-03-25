package skc

import (
	api_attribute "task-processor/internal/shein/api/attribute"
	"task-processor/internal/shein/api/product"
)

type SKCBuildOutput struct {
	SKCList                  []product.SKC
	CustomAttributeRelations []api_attribute.CustomAttributeRelation
}

func newSKCBuildOutput(skcList []product.SKC, customAttributeRelations []api_attribute.CustomAttributeRelation) *SKCBuildOutput {
	return &SKCBuildOutput{
		SKCList:                  skcList,
		CustomAttributeRelations: customAttributeRelations,
	}
}

func (out *SKCBuildOutput) IsEmpty() bool {
	return out == nil || len(out.SKCList) == 0
}
