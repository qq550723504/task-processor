package shein

import (
	"strings"

	"github.com/google/uuid"
	sheinproduct "task-processor/internal/shein/api/product"
)

// PrepareProductForNewSubmit normalizes a product for a new SHEIN submit using default marketplace settings.
func PrepareProductForNewSubmit(product *sheinproduct.Product) {
	PrepareProductForSubmit(product, SubmitPayloadSettings{
		Site:          "US",
		WarehouseCode: defaultSubmitWarehouseCode,
	})
}

// PrepareProductForSubmit normalizes a SHEIN product payload before save-draft or publish submit.
func PrepareProductForSubmit(product *sheinproduct.Product, settings SubmitPayloadSettings) {
	if product == nil {
		return
	}
	if strings.TrimSpace(product.PointKey) == "" {
		product.PointKey = uuid.NewString()
	}
	normalizeProductForSubmit(product, settings)
}

// PrepareProductForValidation applies the same offline defaults as new submit
// without creating a submission identity or reading randomness. It mutates only
// the supplied product; validators must pass an owned copy.
func PrepareProductForValidation(product *sheinproduct.Product) {
	normalizeProductForSubmit(product, SubmitPayloadSettings{
		Site:          "US",
		WarehouseCode: defaultSubmitWarehouseCode,
	})
}

func normalizeProductForSubmit(product *sheinproduct.Product, settings SubmitPayloadSettings) {
	if product == nil {
		return
	}
	// SHEIN generates spu_name for new products. Sending a display title here
	// makes the product API reject the draft/publish request.
	product.SPUName = ""
	product.SourceSystem = "listingkit"
	product.SupplierCode = DeriveSubmitProductSupplierCode(product)
	NormalizeSubmitCollections(product)
	EnsureSubmitSites(product, settings)
	EnsureSubmitSKUs(product, settings)
	NormalizeSubmitImages(product)
	NormalizeSubmitExtra(product)
	FinalizeSubmitTransportFields(product)
}
