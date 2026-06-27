package listingkit

import (
	"context"

	sheinsync "task-processor/internal/listingkit/sheinsync"
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
type SheinEnrollmentService = sheinsync.SheinEnrollmentService
type SheinCostResolver = sheinsync.SheinCostResolver
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

type sheinSyncProductAPIBuilder interface {
	BuildProductAPI(ctx context.Context, storeID int64) (sheinproduct.ProductAPI, string)
}

func NewSheinSyncServiceWithBuilder(repo SheinSyncRepository, productAPIBuilder sheinSyncProductAPIBuilder, costResolver SheinCostResolver) SheinSyncService {
	return sheinsync.NewSheinSyncServiceWithBuilder(repo, sheinSyncProductAPIBuilderBridge{builder: productAPIBuilder}, costResolver)
}

func NewAsyncSheinSyncService(repo SheinSyncRepository, productAPI sheinproduct.ProductAPI, costResolver SheinCostResolver) SheinSyncService {
	return sheinsync.NewAsyncSheinSyncService(repo, productAPI, costResolver)
}

func NewAsyncSheinSyncServiceWithBuilder(repo SheinSyncRepository, productAPIBuilder sheinSyncProductAPIBuilder, costResolver SheinCostResolver) SheinSyncService {
	return sheinsync.NewAsyncSheinSyncServiceWithBuilder(repo, sheinSyncProductAPIBuilderBridge{builder: productAPIBuilder}, costResolver)
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
