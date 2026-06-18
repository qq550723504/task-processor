package shein

import sheinproduct "task-processor/internal/shein/api/product"

func PreValidateSubmitProduct(product *sheinproduct.Product) error {
	return PreValidateSubmitProductWithOptions(product, false)
}

func PreValidateSubmitProductWithOptions(product *sheinproduct.Product, allowPrimaryOnlyMultiSKU bool) error {
	validator := submitProductValidator{}
	return validator.preValidate(submitProductValidationInput{
		ProductData:              product,
		AllowPrimaryOnlyMultiSKU: allowPrimaryOnlyMultiSKU,
	})
}
