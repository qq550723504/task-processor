package shein

import (
	"context"

	sheinproduct "task-processor/internal/shein/api/product"
)

type runtimeProductAPIBuilder struct {
	factory *runtimeAPIFactory
}

func NewRuntimeProductAPIBuilder(factory RuntimeAPIClientFactory) ProductAPIBuilder {
	return &runtimeProductAPIBuilder{
		factory: newRuntimeAPIFactory(factory),
	}
}

func (b *runtimeProductAPIBuilder) BuildProductAPI(storeID int64) (sheinproduct.ProductAPI, string) {
	if b == nil || b.factory == nil {
		return nil, "shein runtime client factory 不可用，SHEIN 提交未启用"
	}
	baseClient, fallback := b.factory.BuildBaseClient(context.Background(), storeID)
	if baseClient == nil {
		return nil, fallback
	}
	return sheinproduct.NewClient(baseClient), ""
}
