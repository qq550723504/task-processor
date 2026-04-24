package shein

import (
	"task-processor/internal/infra/clients/management"
	sheinproduct "task-processor/internal/shein/api/product"
)

type ProductAPIBuilder interface {
	BuildProductAPI(storeID int64) (sheinproduct.ProductAPI, string)
}

type managedProductAPIBuilder struct {
	factory *managedAPIFactory
}

func NewManagedProductAPIBuilder(client *management.ClientManager) ProductAPIBuilder {
	return &managedProductAPIBuilder{
		factory: newManagedAPIFactory(client),
	}
}

func (b *managedProductAPIBuilder) BuildProductAPI(storeID int64) (sheinproduct.ProductAPI, string) {
	if b == nil || b.factory == nil {
		return nil, "management client 不可用，SHEIN 提交未启用"
	}
	baseClient, fallback := b.factory.BuildBaseClient(storeID)
	if baseClient == nil {
		return nil, fallback
	}
	return sheinproduct.NewClient(baseClient), ""
}
