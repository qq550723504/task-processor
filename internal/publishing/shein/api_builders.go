package shein

import (
	"context"

	sheinimage "task-processor/internal/shein/api/image"
	sheinproduct "task-processor/internal/shein/api/product"
	sheintranslate "task-processor/internal/shein/api/translate"
)

type ProductAPIBuilder interface {
	BuildProductAPI(ctx context.Context, storeID int64) (sheinproduct.ProductAPI, string)
}

type ImageAPIBuilder interface {
	BuildImageAPI(ctx context.Context, storeID int64) (sheinimage.ImageAPI, string)
}

type TranslateAPIBuilder interface {
	BuildTranslateAPI(ctx context.Context, storeID int64) (sheintranslate.TranslateAPI, string)
}
