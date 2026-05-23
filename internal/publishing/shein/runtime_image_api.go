package shein

import (
	"context"

	sheinimage "task-processor/internal/shein/api/image"
)

type runtimeImageAPIBuilder struct {
	factory *runtimeAPIFactory
}

func NewRuntimeImageAPIBuilder(factory RuntimeAPIClientFactory) ImageAPIBuilder {
	return &runtimeImageAPIBuilder{
		factory: newRuntimeAPIFactory(factory),
	}
}

func (b *runtimeImageAPIBuilder) BuildImageAPI(ctx context.Context, storeID int64) (sheinimage.ImageAPI, string) {
	if b == nil || b.factory == nil {
		return nil, "shein runtime client factory 不可用，SHEIN 图片上传未启用"
	}
	baseClient, fallback := b.factory.BuildBaseClient(ctx, storeID)
	if baseClient == nil {
		return nil, fallback
	}
	return sheinimage.NewClient(baseClient), ""
}
