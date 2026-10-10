package httpapi

import (
	"context"
	"gorm.io/gorm"
	storeapp "task-processor/internal/app/storecenter"
	"task-processor/internal/authz"
	resourceadapter "task-processor/internal/integration/orgresource"
	"task-processor/internal/ledger/orgresource"
	"task-processor/internal/storecenter"
)

func buildProductResourceCharges(ctx context.Context, productDB, resourceDB *gorm.DB) (*orgresource.ConsumerChargeService, error) {
	return buildCurrentResourceCharges(ctx, productDB, nil, resourceDB, nil, storecenter.RuntimeCapabilities{})
}
func buildCurrentResourceCharges(ctx context.Context, productDB, storeDB, resourceDB *gorm.DB, authorizer *authz.ListingKitAuthorizer, capabilities storecenter.RuntimeCapabilities) (*orgresource.ConsumerChargeService, error) {
	return buildCurrentResourceChargesWithOwners(ctx, productDB, storeDB, resourceDB, authorizer, capabilities, nil)
}

func buildCurrentResourceChargesWithOwners(ctx context.Context, productDB, storeDB, resourceDB *gorm.DB, authorizer *authz.ListingKitAuthorizer, capabilities storecenter.RuntimeCapabilities, additional map[orgresource.ResourceConsumer]orgresource.ConsumerChargeOwner) (*orgresource.ConsumerChargeService, error) {
	if resourceDB == nil || productDB == resourceDB || storeDB == resourceDB || (productDB == nil && storeDB == nil) || (storeDB != nil && storeDB == productDB) {
		return nil, orgresource.ErrInvalidInput
	}
	if err := resourceadapter.VerifyRuntimePermissions(ctx, resourceDB); err != nil {
		return nil, err
	}
	owners := map[orgresource.ResourceConsumer]orgresource.ConsumerChargeOwner{}
	if productDB != nil {
		owner, err := buildProductChargeOwner(ctx, productDB)
		if err != nil {
			return nil, err
		}
		owners[orgresource.ConsumerProductAcquisition] = owner
	}
	if storeDB != nil {
		if authorizer == nil {
			return nil, orgresource.ErrInvalidInput
		}
		if err := storecenter.VerifyCurrentSchema(ctx, storeDB); err != nil {
			return nil, err
		}
		if err := storecenter.VerifyRuntimePermissionsForCapabilities(ctx, storeDB, capabilities); err != nil {
			return nil, err
		}
		stores, err := storecenter.NewMemberScopedStoreRepository(storeDB, currentStoreMemberAuthorizer{authorizer: authorizer})
		if err != nil {
			return nil, err
		}
		owner, err := storeapp.NewServiceChargeOwner(stores)
		if err != nil {
			return nil, err
		}
		owners[orgresource.ConsumerStoreService] = owner
	}
	for consumer, owner := range additional {
		if consumer != orgresource.ConsumerAmazonData || owner == nil || owners[consumer] != nil {
			return nil, orgresource.ErrInvalidInput
		}
		owners[consumer] = owner
	}
	repository, err := resourceadapter.NewGormConsumerChargeRepository(resourceDB, resourceadapter.TransactionConfig{})
	if err != nil {
		return nil, err
	}
	return orgresource.NewConsumerChargeService(repository, owners)
}
