package listingkit

import (
	"strings"

	"github.com/google/uuid"
	sheinpub "task-processor/internal/publishing/shein"
	sheinproduct "task-processor/internal/shein/api/product"
)

func prepareSheinProductForNewSubmit(product *sheinproduct.Product) {
	prepareSheinProductForSubmit(product, SheinSettings{
		Site:          "US",
		WarehouseCode: "DEFAULT",
	})
}

func prepareSheinProductForSubmit(product *sheinproduct.Product, settings SheinSettings) {
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
	product.SupplierCode = deriveSheinSubmitProductSupplierCode(product)
	normalizeSheinSubmitCollections(product)
	ensureSheinSubmitSites(product, settings)
	ensureSheinSubmitSKUs(product, settings)
	normalizeSheinSubmitImages(product)
	normalizeSheinSubmitExtra(product)
	finalizeSheinSubmitTransportFields(product)
}

func normalizeSheinSubmitCollections(product *sheinproduct.Product) {
	sheinpub.NormalizeSubmitCollections(product)
}

func normalizeSheinSubmitExtra(product *sheinproduct.Product) {
	sheinpub.NormalizeSubmitExtra(product)
}

func finalizeSheinSubmitTransportFields(product *sheinproduct.Product) {
	sheinpub.FinalizeSubmitTransportFields(product)
}
