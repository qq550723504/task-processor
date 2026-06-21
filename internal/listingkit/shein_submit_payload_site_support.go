package listingkit

import (
	sheinpub "task-processor/internal/publishing/shein"
	sheinproduct "task-processor/internal/shein/api/product"
)

func ensureSheinSubmitSites(product *sheinproduct.Product, settings SheinSettings) {
	sheinpub.EnsureSubmitSites(product, sheinSubmitPayloadSettings(settings))
}

func sheinSubmitPreferredWarehouseCode(settings SheinSettings) string {
	return sheinpub.SubmitPreferredWarehouseCode(sheinSubmitPayloadSettings(settings))
}

func ensureSheinSubmitSKUs(product *sheinproduct.Product, settings SheinSettings) {
	sheinpub.EnsureSubmitSKUs(product, sheinSubmitPayloadSettings(settings))
}

func normalizeSheinSubmitWeight(sku *sheinproduct.SKU) {
	sheinpub.NormalizeSubmitWeight(sku)
}

func sheinSubmitPayloadSettings(settings SheinSettings) sheinpub.SubmitPayloadSettings {
	return sheinpub.SubmitPayloadSettings{
		Site:          settings.Site,
		WarehouseCode: settings.WarehouseCode,
	}
}
