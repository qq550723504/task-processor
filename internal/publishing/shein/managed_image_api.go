package shein

import (
	"task-processor/internal/infra/clients/management"
	sheinimage "task-processor/internal/shein/api/image"
)

type ImageAPIBuilder interface {
	BuildImageAPI(storeID int64) (sheinimage.ImageAPI, string)
}

type managedImageAPIBuilder struct {
	factory *managedAPIFactory
}

func NewManagedImageAPIBuilder(client *management.ClientManager) ImageAPIBuilder {
	return &managedImageAPIBuilder{
		factory: newManagedAPIFactory(client),
	}
}

func (b *managedImageAPIBuilder) BuildImageAPI(storeID int64) (sheinimage.ImageAPI, string) {
	if b == nil || b.factory == nil {
		return nil, "management client 不可用，SHEIN 图片上传未启用"
	}
	baseClient, fallback := b.factory.BuildBaseClient(storeID)
	if baseClient == nil {
		return nil, fallback
	}
	return sheinimage.NewClientWithImageDownloader(baseClient, b.factory.client.GetImageDownloader()), ""
}
