package listingkit

import (
	"context"

	sheinsync "task-processor/internal/listingkit/sheinsync"
	sheinpub "task-processor/internal/publishing/shein"
	"task-processor/internal/shein/activity"
	sheinproduct "task-processor/internal/shein/api/product"
)

type SheinCostPriceSource = sheinsync.SheinCostPriceSource

const (
	SheinCostPriceSourceNone   = sheinsync.SheinCostPriceSourceNone
	SheinCostPriceSourceAuto   = sheinsync.SheinCostPriceSourceAuto
	SheinCostPriceSourceManual = sheinsync.SheinCostPriceSourceManual
)

type SheinSyncTriggerMode = sheinsync.SheinSyncTriggerMode

const (
	SheinSyncTriggerModeManual   = sheinsync.SheinSyncTriggerModeManual
	SheinSyncTriggerModeSchedule = sheinsync.SheinSyncTriggerModeSchedule
)

type SheinSyncJobStatus = sheinsync.SheinSyncJobStatus

const (
	SheinSyncJobStatusPending            = sheinsync.SheinSyncJobStatusPending
	SheinSyncJobStatusRunning            = sheinsync.SheinSyncJobStatusRunning
	SheinSyncJobStatusSucceeded          = sheinsync.SheinSyncJobStatusSucceeded
	SheinSyncJobStatusPartiallySucceeded = sheinsync.SheinSyncJobStatusPartiallySucceeded
	SheinSyncJobStatusFailed             = sheinsync.SheinSyncJobStatusFailed
)

type SheinCandidateEligibilityStatus = sheinsync.SheinCandidateEligibilityStatus

const (
	SheinCandidateEligibilityStatusEligible   = sheinsync.SheinCandidateEligibilityStatusEligible
	SheinCandidateEligibilityStatusIneligible = sheinsync.SheinCandidateEligibilityStatusIneligible
)

type SheinCandidateReviewStatus = sheinsync.SheinCandidateReviewStatus

const (
	SheinCandidateReviewStatusPendingReview = sheinsync.SheinCandidateReviewStatusPendingReview
	SheinCandidateReviewStatusApproved      = sheinsync.SheinCandidateReviewStatusApproved
	SheinCandidateReviewStatusRejected      = sheinsync.SheinCandidateReviewStatusRejected
	SheinCandidateReviewStatusAutoQueued    = sheinsync.SheinCandidateReviewStatusAutoQueued
	SheinCandidateReviewStatusEnrolled      = sheinsync.SheinCandidateReviewStatusEnrolled
	SheinCandidateReviewStatusFailed        = sheinsync.SheinCandidateReviewStatusFailed
)

type SheinEnrollmentRunTriggerMode = sheinsync.SheinEnrollmentRunTriggerMode

const (
	SheinEnrollmentRunTriggerModeManualConfirmed = sheinsync.SheinEnrollmentRunTriggerModeManualConfirmed
	SheinEnrollmentRunTriggerModeAutoSchedule    = sheinsync.SheinEnrollmentRunTriggerModeAutoSchedule
)

type SheinEnrollmentRunStatus = sheinsync.SheinEnrollmentRunStatus

const (
	SheinEnrollmentRunStatusPending            = sheinsync.SheinEnrollmentRunStatusPending
	SheinEnrollmentRunStatusRunning            = sheinsync.SheinEnrollmentRunStatusRunning
	SheinEnrollmentRunStatusSucceeded          = sheinsync.SheinEnrollmentRunStatusSucceeded
	SheinEnrollmentRunStatusPartiallySucceeded = sheinsync.SheinEnrollmentRunStatusPartiallySucceeded
	SheinEnrollmentRunStatusFailed             = sheinsync.SheinEnrollmentRunStatusFailed
	SheinEnrollmentRunStatusCancelled          = sheinsync.SheinEnrollmentRunStatusCancelled
)

type SheinEnrollmentItemStatus = sheinsync.SheinEnrollmentItemStatus

const (
	SheinEnrollmentItemStatusPending   = sheinsync.SheinEnrollmentItemStatusPending
	SheinEnrollmentItemStatusRunning   = sheinsync.SheinEnrollmentItemStatusRunning
	SheinEnrollmentItemStatusSucceeded = sheinsync.SheinEnrollmentItemStatusSucceeded
	SheinEnrollmentItemStatusFailed    = sheinsync.SheinEnrollmentItemStatusFailed
	SheinEnrollmentItemStatusCancelled = sheinsync.SheinEnrollmentItemStatusCancelled
)

type SheinSyncedProductRecord = sheinsync.SheinSyncedProductRecord
type SheinSKUCostPrice = sheinsync.SheinSKUCostPrice
type SheinSDSCostGroupRecord = sheinsync.SheinSDSCostGroupRecord
type SheinSourceSDSCostGroupRecord = sheinsync.SheinSourceSDSCostGroupRecord
type SheinSourceSDSSKUCostGroupRecord = sheinsync.SheinSourceSDSSKUCostGroupRecord
type SheinSDSCostGroupIdentity = sheinsync.SheinSDSCostGroupIdentity
type SheinSyncJobRecord = sheinsync.SheinSyncJobRecord
type SheinActivityCandidateRecord = sheinsync.SheinActivityCandidateRecord
type SheinActivityEnrollmentRunRecord = sheinsync.SheinActivityEnrollmentRunRecord
type SheinActivityEnrollmentItemRecord = sheinsync.SheinActivityEnrollmentItemRecord
type SheinSyncedProductQuery = sheinsync.SheinSyncedProductQuery
type SheinSDSCostGroupQuery = sheinsync.SheinSDSCostGroupQuery
type SheinSourceSDSCostGroupQuery = sheinsync.SheinSourceSDSCostGroupQuery
type SheinSyncJobQuery = sheinsync.SheinSyncJobQuery
type SheinActivityCandidateQuery = sheinsync.SheinActivityCandidateQuery
type SheinEnrollmentRunQuery = sheinsync.SheinEnrollmentRunQuery
type SheinEnrollmentItemQuery = sheinsync.SheinEnrollmentItemQuery
type SheinEnrollmentStoreSummary = sheinsync.SheinEnrollmentStoreSummary
type SheinSyncedProductRepository = sheinsync.SheinSyncedProductRepository
type SheinSyncJobRepository = sheinsync.SheinSyncJobRepository
type SheinActivityCandidateRepository = sheinsync.SheinActivityCandidateRepository
type SheinActivityEnrollmentRunRepository = sheinsync.SheinActivityEnrollmentRunRepository
type SheinActivityEnrollmentItemRepository = sheinsync.SheinActivityEnrollmentItemRepository
type SheinSyncRepository = sheinsync.SheinSyncRepository
type SheinActivityEnrollmentCandidate = sheinsync.SheinActivityEnrollmentCandidate
type SheinActivityEnrollmentResult = sheinsync.SheinActivityEnrollmentResult
type SheinActivityAdapter = sheinsync.SheinActivityAdapter
type SheinPromotionStrategy = sheinsync.SheinPromotionStrategy
type SheinPromotionStrategyProvider = sheinsync.SheinPromotionStrategyProvider
type SheinSyncService = sheinsync.SheinSyncService
type SheinCandidateService = sheinsync.SheinCandidateService
type SheinCandidateRefreshResult = sheinsync.SheinCandidateRefreshResult
type SheinCandidateResetRequest = sheinsync.SheinCandidateResetRequest
type SheinCandidateResetResult = sheinsync.SheinCandidateResetResult
type SheinEnrollmentService = sheinsync.SheinEnrollmentService
type SheinCostResolver = sheinsync.SheinCostResolver
type SheinInventoryMappingSource = sheinsync.SheinInventoryMappingSource
type SheinSyncScheduler = sheinsync.SheinSyncScheduler
type SheinEnrollmentScheduler = sheinsync.SheinEnrollmentScheduler

func ApplyEffectiveCostPrice(record *SheinSyncedProductRecord) {
	sheinsync.ApplyEffectiveCostPrice(record)
}

func ResolveSheinSDSCostGroupIdentity(record SheinSyncedProductRecord) sheinsync.SheinSDSCostGroupIdentity {
	return sheinsync.ResolveSheinSDSCostGroupIdentity(record)
}

func ResolveSheinSDSSKUCostGroupIdentities(record SheinSyncedProductRecord) []sheinsync.SheinSDSCostGroupIdentity {
	return sheinsync.ResolveSheinSDSSKUCostGroupIdentities(record)
}

func ResolveSheinSDSVariantCostGroupIdentity(record SheinSyncedProductRecord) sheinsync.SheinSDSCostGroupIdentity {
	return sheinsync.ResolveSheinSDSVariantCostGroupIdentity(record)
}

func ResolveSheinSDSVariantCostGroupIdentities(record SheinSyncedProductRecord) []sheinsync.SheinSDSCostGroupIdentity {
	return sheinsync.ResolveSheinSDSVariantCostGroupIdentities(record)
}

func SheinSyncedProductSKUCodes(record SheinSyncedProductRecord) []string {
	return sheinsync.SheinSyncedProductSKUCodes(record)
}

func NewSheinCostResolver(productAPI sheinproduct.ProductAPI) SheinCostResolver {
	return sheinsync.NewSheinCostResolver(productAPI)
}

func NewSheinSyncService(repo SheinSyncRepository, productAPI sheinproduct.ProductAPI, costResolver SheinCostResolver) SheinSyncService {
	return sheinsync.NewSheinSyncService(repo, productAPI, costResolver)
}

func NewSheinSyncServiceWithInventoryMappingSource(repo SheinSyncRepository, productAPI sheinproduct.ProductAPI, costResolver SheinCostResolver, mappingSource SheinInventoryMappingSource) SheinSyncService {
	return sheinsync.NewSheinSyncServiceWithInventoryMappingSource(repo, productAPI, costResolver, mappingSource)
}

type sheinSyncProductAPIBuilder interface {
	BuildProductAPI(ctx context.Context, storeID int64) (sheinproduct.ProductAPI, string)
}

func NewSheinSyncServiceWithBuilder(repo SheinSyncRepository, productAPIBuilder sheinSyncProductAPIBuilder, costResolver SheinCostResolver) SheinSyncService {
	return sheinsync.NewSheinSyncServiceWithBuilder(repo, sheinSyncProductAPIBuilderBridge{builder: productAPIBuilder}, costResolver)
}

func NewSheinSyncServiceWithBuilderAndInventoryMappingSource(repo SheinSyncRepository, productAPIBuilder sheinSyncProductAPIBuilder, costResolver SheinCostResolver, mappingSource SheinInventoryMappingSource) SheinSyncService {
	return sheinsync.NewSheinSyncServiceWithBuilderAndInventoryMappingSource(repo, sheinSyncProductAPIBuilderBridge{builder: productAPIBuilder}, costResolver, mappingSource)
}

func NewAsyncSheinSyncService(repo SheinSyncRepository, productAPI sheinproduct.ProductAPI, costResolver SheinCostResolver) SheinSyncService {
	return sheinsync.NewAsyncSheinSyncService(repo, productAPI, costResolver)
}

func NewAsyncSheinSyncServiceWithBuilder(repo SheinSyncRepository, productAPIBuilder sheinSyncProductAPIBuilder, costResolver SheinCostResolver) SheinSyncService {
	return sheinsync.NewAsyncSheinSyncServiceWithBuilder(repo, sheinSyncProductAPIBuilderBridge{builder: productAPIBuilder}, costResolver)
}

func NewAsyncSheinSyncServiceWithBuilderAndInventoryMappingSource(repo SheinSyncRepository, productAPIBuilder sheinSyncProductAPIBuilder, costResolver SheinCostResolver, mappingSource SheinInventoryMappingSource) SheinSyncService {
	return sheinsync.NewAsyncSheinSyncServiceWithBuilderAndInventoryMappingSource(repo, sheinSyncProductAPIBuilderBridge{builder: productAPIBuilder}, costResolver, mappingSource)
}

// NewStoreValidatedSheinSyncService prevents a stale, disabled, or foreign store
// from creating sync jobs or building a remote product API client.
func NewStoreValidatedSheinSyncService(delegate SheinSyncService, validator StoreAccessValidator) SheinSyncService {
	return storeValidatedSheinSyncService{delegate: delegate, validator: validator}
}

// NewStoreValidatedSheinProductAPIBuilder rechecks access when a background
// sync job is about to build its remote API client.
func NewStoreValidatedSheinProductAPIBuilder(delegate sheinpub.ProductAPIBuilder, validator StoreAccessValidator) sheinpub.ProductAPIBuilder {
	return storeValidatedSheinProductAPIBuilder{delegate: delegate, validator: validator}
}

type storeValidatedSheinProductAPIBuilder struct {
	delegate  sheinpub.ProductAPIBuilder
	validator StoreAccessValidator
}

func (b storeValidatedSheinProductAPIBuilder) BuildProductAPI(ctx context.Context, storeID int64) (sheinproduct.ProductAPI, string) {
	tenantID, ok := tenantIDInt64FromContext(ctx)
	if !ok || tenantID <= 0 || b.validator == nil {
		return nil, "store is unavailable"
	}
	if _, err := b.validator.ValidateStoreAccess(ctx, tenantID, storeID, "SHEIN"); err != nil {
		return nil, err.Error()
	}
	if b.delegate == nil {
		return nil, "SHEIN product API builder is unavailable"
	}
	return b.delegate.BuildProductAPI(ctx, storeID)
}

type storeValidatedSheinSyncService struct {
	delegate  SheinSyncService
	validator StoreAccessValidator
}

func (s storeValidatedSheinSyncService) SyncSheinOnShelfProducts(ctx context.Context, tenantID, storeID int64, triggerMode SheinSyncTriggerMode) (*SheinSyncJobRecord, error) {
	if err := s.validateStore(ctx, tenantID, storeID); err != nil {
		return nil, err
	}
	if s.delegate == nil {
		return nil, NewStoreAccessError(StoreAccessUnavailable, "store is unavailable")
	}
	return s.delegate.SyncSheinOnShelfProducts(ctx, tenantID, storeID, triggerMode)
}

func (s storeValidatedSheinSyncService) SyncSheinSourceSDSProduct(ctx context.Context, tenantID, storeID int64, sourceCode string) (int, error) {
	if err := s.validateStore(ctx, tenantID, storeID); err != nil {
		return 0, err
	}
	if s.delegate == nil {
		return 0, NewStoreAccessError(StoreAccessUnavailable, "store is unavailable")
	}
	return s.delegate.SyncSheinSourceSDSProduct(ctx, tenantID, storeID, sourceCode)
}

func (s storeValidatedSheinSyncService) ListSyncedProducts(ctx context.Context, query *SheinSyncedProductQuery) ([]SheinSyncedProductRecord, int64, error) {
	if s.delegate == nil {
		return nil, 0, NewStoreAccessError(StoreAccessUnavailable, "store is unavailable")
	}
	return s.delegate.ListSyncedProducts(ctx, query)
}

func (s storeValidatedSheinSyncService) UpdateManualCostPrice(ctx context.Context, productID int64, manualCostPrice *float64) error {
	if s.delegate == nil {
		return NewStoreAccessError(StoreAccessUnavailable, "store is unavailable")
	}
	return s.delegate.UpdateManualCostPrice(ctx, productID, manualCostPrice)
}

func (s storeValidatedSheinSyncService) ResolveProductAPI(ctx context.Context, storeID int64) (sheinproduct.ProductAPI, error) {
	tenantID, ok := tenantIDInt64FromContext(ctx)
	if !ok || tenantID <= 0 {
		return nil, NewStoreAccessError(StoreAccessUnavailable, "store is unavailable")
	}
	if err := s.validateStore(ctx, tenantID, storeID); err != nil {
		return nil, err
	}
	if s.delegate == nil {
		return nil, NewStoreAccessError(StoreAccessUnavailable, "store is unavailable")
	}
	return s.delegate.ResolveProductAPI(ctx, storeID)
}

func (s storeValidatedSheinSyncService) SupportsImmediateRefresh() bool {
	aware, ok := s.delegate.(interface {
		SupportsImmediateRefresh() bool
	})
	return ok && aware.SupportsImmediateRefresh()
}

func (s storeValidatedSheinSyncService) validateStore(ctx context.Context, tenantID, storeID int64) error {
	if s.validator == nil {
		return NewStoreAccessError(StoreAccessUnavailable, "store is unavailable")
	}
	_, err := s.validator.ValidateStoreAccess(ctx, tenantID, storeID, "SHEIN")
	return err
}

type sheinSyncProductAPIBuilderBridge struct {
	builder sheinSyncProductAPIBuilder
}

func (b sheinSyncProductAPIBuilderBridge) BuildProductAPI(ctx context.Context, storeID int64) (sheinproduct.ProductAPI, string) {
	if b.builder == nil {
		return nil, ""
	}
	return b.builder.BuildProductAPI(ctx, storeID)
}

func NewSheinCandidateService(repo SheinSyncRepository) SheinCandidateService {
	return sheinsync.NewSheinCandidateService(repo)
}

func NewSheinActivityAdapter(strategyProvider SheinPromotionStrategyProvider, promotionBridge activity.PromotionRegistrationBridge) SheinActivityAdapter {
	return sheinsync.NewSheinActivityAdapter(strategyProvider, promotionBridge)
}

type sheinPromotionBridgeFactory interface {
	BuildPromotionBridge(ctx context.Context, storeID int64) (activity.PromotionRegistrationBridge, error)
}

func NewSheinActivityAdapterWithFactory(strategyProvider SheinPromotionStrategyProvider, promotionBridgeFactory sheinPromotionBridgeFactory) SheinActivityAdapter {
	return sheinsync.NewSheinActivityAdapterWithFactory(strategyProvider, promotionBridgeFactory)
}

func NewSheinEnrollmentService(repo SheinSyncRepository, adapter SheinActivityAdapter) SheinEnrollmentService {
	return sheinsync.NewSheinEnrollmentService(repo, adapter)
}

func NewSheinSyncScheduler(syncService SheinSyncService) *SheinSyncScheduler {
	return sheinsync.NewSheinSyncScheduler(syncService)
}

func NewSheinEnrollmentScheduler(syncService SheinSyncService, candidateService SheinCandidateService, enrollmentService SheinEnrollmentService) *SheinEnrollmentScheduler {
	return sheinsync.NewSheinEnrollmentScheduler(syncService, candidateService, enrollmentService)
}
