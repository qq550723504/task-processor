package shein

import (
	sheinmarketpub "task-processor/internal/marketplace/shein/publishing"
	sheinproduct "task-processor/internal/shein/api/product"
)

func NormalizeSubmitCollections(product *sheinproduct.Product) {
	sheinmarketpub.NormalizeSubmitCollections(product)
}

func NormalizeSubmitExtra(product *sheinproduct.Product) {
	sheinmarketpub.NormalizeSubmitExtra(product)
}

func FinalizeSubmitTransportFields(product *sheinproduct.Product) {
	sheinmarketpub.FinalizeSubmitTransportFields(product)
}
