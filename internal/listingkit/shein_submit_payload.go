package listingkit

import (
	sheinpub "task-processor/internal/publishing/shein"
	sheinproduct "task-processor/internal/shein/api/product"
)

func prepareSheinProductForNewSubmit(product *sheinproduct.Product) {
	sheinpub.PrepareProductForNewSubmit(product)
}

func prepareSheinProductForSubmit(product *sheinproduct.Product, settings SheinSettings) {
	sheinpub.PrepareProductForSubmit(product, sheinSubmitPayloadSettings(settings))
}

func sheinSubmitPayloadSettings(settings SheinSettings) sheinpub.SubmitPayloadSettings {
	return sheinpub.SubmitPayloadSettings{
		Site:          settings.Site,
		WarehouseCode: settings.WarehouseCode,
	}
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
