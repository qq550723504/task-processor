package shein

import (
	sheinmarketpub "task-processor/internal/marketplace/shein/publishing"
	sheinproduct "task-processor/internal/shein/api/product"
)

func PreValidateSubmitProduct(product *sheinproduct.Product) error {
	return sheinmarketpub.PreValidateSubmitProduct(product)
}

func PreValidateSubmitProductWithOptions(product *sheinproduct.Product, allowPrimaryOnlyMultiSKU bool) error {
	return sheinmarketpub.PreValidateSubmitProductWithOptions(product, allowPrimaryOnlyMultiSKU)
}
