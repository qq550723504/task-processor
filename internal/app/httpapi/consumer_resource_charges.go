package httpapi

import (
	"context"
	"gorm.io/gorm"
	"task-processor/internal/app/productsourcing"
	resourceadapter "task-processor/internal/integration/orgresource"
	acquisitionstore "task-processor/internal/integration/persistence/product/acquisition"
	"task-processor/internal/ledger/orgresource"
)

func buildProductResourceCharges(ctx context.Context, productDB, resourceDB *gorm.DB) (*orgresource.ConsumerChargeService, error) {
	if productDB == nil || resourceDB == nil || productDB == resourceDB {
		return nil, orgresource.ErrInvalidInput
	}
	if err := resourceadapter.VerifyRuntimePermissions(ctx, resourceDB); err != nil {
		return nil, err
	}
	operations, err := acquisitionstore.NewRepository(ctx, productDB)
	if err != nil {
		return nil, err
	}
	owner, err := productsourcing.NewAcquisitionChargeOwner(operations)
	if err != nil {
		return nil, err
	}
	repository, err := resourceadapter.NewGormConsumerChargeRepository(resourceDB, resourceadapter.TransactionConfig{})
	if err != nil {
		return nil, err
	}
	return orgresource.NewConsumerChargeService(repository, map[orgresource.ResourceConsumer]orgresource.ConsumerChargeOwner{orgresource.ConsumerProductAcquisition: owner})
}
