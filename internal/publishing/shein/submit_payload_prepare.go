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
	// SHEIN generates spu_name for new products. Sending a display title here
	// makes the product API reject the draft/publish request.
	product.SPUName = ""
	if strings.TrimSpace(product.PointKey) == "" {
		product.PointKey = uuid.NewString()
	}
	product.SourceSystem = "listingkit"
	product.SupplierCode = DeriveSubmitProductSupplierCode(product)
	NormalizeSubmitCollections(product)
	EnsureSubmitSites(product, settings)
	EnsureSubmitSKUs(product, settings)
	NormalizeSubmitImages(product)
	NormalizeSubmitExtra(product)
	FinalizeSubmitTransportFields(product)
}
