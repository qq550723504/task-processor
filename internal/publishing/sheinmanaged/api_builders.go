package sheinmanaged

import (
	"context"

	"task-processor/internal/infra/clients/management"
	sheinpub "task-processor/internal/publishing/shein"
	sheinimage "task-processor/internal/shein/api/image"
	sheinproduct "task-processor/internal/shein/api/product"
	sheintranslate "task-processor/internal/shein/api/translate"
)

type productAPIBuilder struct {
	factory *apiFactory
}

func NewProductAPIBuilder(client *management.ClientManager) sheinpub.ProductAPIBuilder {
	return &productAPIBuilder{factory: newAPIFactory(client)}
}

func (b *productAPIBuilder) BuildProductAPI(_ context.Context, storeID int64) (sheinproduct.ProductAPI, string) {
	if b == nil || b.factory == nil {
		return nil, "management client 不可用，SHEIN 提交未启用"
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

func NewImageAPIBuilder(client *management.ClientManager) sheinpub.ImageAPIBuilder {
	return &imageAPIBuilder{factory: newAPIFactory(client)}
}

func (b *imageAPIBuilder) BuildImageAPI(_ context.Context, storeID int64) (sheinimage.ImageAPI, string) {
	if b == nil || b.factory == nil {
		return nil, "management client 不可用，SHEIN 图片上传未启用"
	}
	baseClient, fallback := b.factory.BuildBaseClient(storeID)
	if baseClient == nil {
		return nil, fallback
	}
	return sheinimage.NewClientWithImageDownloader(baseClient, b.factory.client.GetImageDownloader()), ""
}

type translateAPIBuilder struct {
	factory *apiFactory
}

func NewTranslateAPIBuilder(client *management.ClientManager) sheinpub.TranslateAPIBuilder {
	return &translateAPIBuilder{factory: newAPIFactory(client)}
}

func (b *translateAPIBuilder) BuildTranslateAPI(_ context.Context, storeID int64) (sheintranslate.TranslateAPI, string) {
	if b == nil || b.factory == nil {
		return nil, "management client 不可用，SHEIN 翻译未启用"
	}
	baseClient, fallback := b.factory.BuildBaseClient(storeID)
	if baseClient == nil {
		return nil, fallback
	}
	return sheintranslate.NewClient(baseClient), ""
}
