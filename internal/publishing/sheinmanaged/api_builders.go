package sheinmanaged

import (
	"context"

	sheinpub "task-processor/internal/publishing/shein"
	sheinimage "task-processor/internal/shein/api/image"
	sheinproduct "task-processor/internal/shein/api/product"
	sheintranslate "task-processor/internal/shein/api/translate"
)

type productAPIBuilder struct {
	factory *apiFactory
}

func NewProductAPIBuilder(_ ...any) sheinpub.ProductAPIBuilder {
	return &productAPIBuilder{factory: newAPIFactory()}
}

func (b *productAPIBuilder) BuildProductAPI(_ context.Context, storeID int64) (sheinproduct.ProductAPI, string) {
	if b == nil || b.factory == nil {
		return nil, "SHEIN managed runtime 已下线，SHEIN 提交未启用"
	}
	baseClient, fallback := b.factory.BuildBaseClient(storeID)
	if baseClient == nil {
		return nil, fallback
	}
	return sheinproduct.NewClient(baseClient), ""
}

type imageAPIBuilder struct {
	factory *apiFactory
}

func NewImageAPIBuilder(_ ...any) sheinpub.ImageAPIBuilder {
	return &imageAPIBuilder{factory: newAPIFactory()}
}

func (b *imageAPIBuilder) BuildImageAPI(_ context.Context, storeID int64) (sheinimage.ImageAPI, string) {
	if b == nil || b.factory == nil {
		return nil, "SHEIN managed runtime 已下线，SHEIN 图片上传未启用"
	}
	baseClient, fallback := b.factory.BuildBaseClient(storeID)
	if baseClient == nil {
		return nil, fallback
	}
	return sheinimage.NewClient(baseClient), ""
}

type translateAPIBuilder struct {
	factory *apiFactory
}

func NewTranslateAPIBuilder(_ ...any) sheinpub.TranslateAPIBuilder {
	return &translateAPIBuilder{factory: newAPIFactory()}
}

func (b *translateAPIBuilder) BuildTranslateAPI(_ context.Context, storeID int64) (sheintranslate.TranslateAPI, string) {
	if b == nil || b.factory == nil {
		return nil, "SHEIN managed runtime 已下线，SHEIN 翻译未启用"
	}
	baseClient, fallback := b.factory.BuildBaseClient(storeID)
	if baseClient == nil {
		return nil, fallback
	}
	return sheintranslate.NewClient(baseClient), ""
}
