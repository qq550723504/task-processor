package shein

import (
	sheinimage "task-processor/internal/shein/api/image"
	sheinproduct "task-processor/internal/shein/api/product"
	sheintranslate "task-processor/internal/shein/api/translate"
)

type ProductAPIBuilder interface {
	BuildProductAPI(storeID int64) (sheinproduct.ProductAPI, string)
}

type ImageAPIBuilder interface {
	BuildImageAPI(storeID int64) (sheinimage.ImageAPI, string)
}

type TranslateAPIBuilder interface {
	BuildTranslateAPI(storeID int64) (sheintranslate.TranslateAPI, string)
}
