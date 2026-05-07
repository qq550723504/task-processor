package shein

import (
	sheinproduct "task-processor/internal/shein/api/product"
	sheinpublish "task-processor/internal/shein/publish"
)

func PreValidateSubmitProduct(product *sheinproduct.Product) error {
	validator := sheinpublish.NewPublishProductValidator()
	return validator.PreValidateProductData(nil, &sheinpublish.ValidationInput{
		ProductData: product,
	})
}
