package listingkit

import (
	sheinpub "task-processor/internal/publishing/shein"
	sheinproduct "task-processor/internal/shein/api/product"
)

func deriveSheinSubmitProductSupplierCode(product *sheinproduct.Product) string {
	return sheinpub.DeriveSubmitProductSupplierCode(product)
}

func validateSheinProductPublishPayload(product *sheinproduct.Product) error {
	return sheinpub.ValidateProductPublishPayload(product)
}
