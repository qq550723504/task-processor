package sale

import sheinattr "task-processor/internal/shein/product/attribute"

type SaleAttributeOutput struct {
	Result             sheinattr.ResultSaleAttribute
	VariantCount       int
	SaleAttributeCount int
}

func NewSaleAttributeOutput(result sheinattr.ResultSaleAttribute) *SaleAttributeOutput {
	return &SaleAttributeOutput{
		Result:             result,
		VariantCount:       len(result.Variants),
		SaleAttributeCount: len(result.SaleAttributes),
	}
}
