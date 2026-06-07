package shein

import (
	sheinproduct "task-processor/internal/shein/api/product"
	sheinpublish "task-processor/internal/shein/publish"
)

func PreValidateSubmitProduct(product *sheinproduct.Product) error {
	return PreValidateSubmitProductWithOptions(product, false)
}

func PreValidateSubmitProductWithOptions(product *sheinproduct.Product, allowPrimaryOnlyMultiSKU bool) error {
	validator := sheinpublish.NewPublishProductValidator()
	return validator.PreValidateProductData(nil, &sheinpublish.ValidationInput{
		ProductData:              product,
		AllowPrimaryOnlyMultiSKU: allowPrimaryOnlyMultiSKU,
	})
}
