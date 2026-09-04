package listingkit

import (
	"context"
	"errors"
	"time"

	"task-processor/internal/amazonlisting"
	"task-processor/internal/listingkit/core"
	productasset "task-processor/internal/product/asset"
	"task-processor/internal/product/catalog"
)

type TaskSubmitter interface{ Submit(taskID string) error }

var ErrProductSnapshotNotReady = errors.New("product snapshot is not ready")

type ProductSnapshotQuery struct {
	TenantID   string
	ProductKey string
	Version    uint64
}

type ProductSnapshotReader interface {
	GetProductSnapshot(ctx context.Context, query ProductSnapshotQuery) (catalog.ProductSnapshot, error)
}

type PublishedProductSnapshotReader interface {
	GetPublishedProductSnapshot(ctx context.Context, query ProductSnapshotQuery) (catalog.PublishedSnapshot, error)
}

type ApprovedAssetInventoryReader interface {
	GetApprovedInventory(ctx context.Context, scope productasset.InventoryScope) (productasset.ApprovedAssetInventory, error)
}

type AIClientCredentialStore interface {
	SaveCredential(ctx context.Context, credential AIClientCredential) error
	GetCredential(ctx context.Context, tenantID, userID, clientName string) (*AIClientCredential, error)
}

type Repository interface {
	CreateTask(ctx context.Context, task *Task) error
	GetTask(ctx context.Context, taskID string) (*Task, error)
	ListTasks(ctx context.Context, query *TaskListQuery) ([]Task, int64, error)
	MarkProcessing(ctx context.Context, taskID string) error
	MarkCompleted(ctx context.Context, taskID string, result *ListingKitResult) error
	MarkNeedsReview(ctx context.Context, taskID string, result *ListingKitResult, reason string) error
	MarkFailed(ctx context.Context, taskID string, errorMsg string) error
	MarkBlockedRetryable(ctx context.Context, taskID string, block *RetryableBlock, errorMsg string) error
	ListRecoverableTasks(ctx context.Context, query *RecoverableTaskQuery) ([]Task, error)
	RecoverBlockedTaskNow(ctx context.Context, taskID string, recoveredAt time.Time) error
	// BulkRecoverBlockedTasks is a persistence-only repository helper that clears
	// blocked state for due tasks. It does not submit recovered tasks back to the
	// queue and must not be treated as the authoritative recovery flow.
	// TaskRecoveryService owns the full recover-and-submit semantics.
	BulkRecoverBlockedTasks(ctx context.Context, query *RecoverBlockedTasksQuery) (int64, error)
	PrepareRetry(ctx context.Context, taskID string) error
	IncrementRetryCount(ctx context.Context, taskID string) error
	SaveTaskResult(ctx context.Context, taskID string, result *ListingKitResult) error
}

// ProcessingFailureRepository provides the compare-and-set boundary used by
// durable workflows when recording a terminal failure. It must never replace
// a completed or needs-review result after an activity response is lost.
type ProcessingFailureRepository interface {
	MarkFailedIfProcessing(ctx context.Context, taskID string, errorMessage string) (bool, error)
}

// UsageSettlementRepository is an optional task-repository extension used to
// clear a settlement-only retryable block without re-running generation.
type UsageSettlementRepository interface {
	ResolveUsageSettlement(ctx context.Context, taskID string) error
}

// ConditionalRetryableBlockRepository atomically replaces a retryable block
// only while the task still carries the expected recovery state. Recovery
// workers use it to avoid overwriting a terminal resolution completed by a
// concurrent worker after an upstream settlement call returns an error.
type ConditionalRetryableBlockRepository interface {
	MarkBlockedRetryableIfCurrent(ctx context.Context, taskID string, expected, next *RetryableBlock, errorMsg string) (bool, error)
}

// GenerationUsageReservationRepository persists the task-side reservation
// intent independently from the PAY-041 ledger. It is intentionally an
// optional extension so unrelated task repositories do not need billing
// behavior, while metered generation can require it at runtime.
type GenerationUsageReservationRepository interface {
	BeginGenerationUsageReservation(ctx context.Context, taskID string, leaseUntil time.Time) error
	MarkGenerationUsageReserved(ctx context.Context, taskID string, leaseUntil time.Time) error
	RenewGenerationUsageReservation(ctx context.Context, taskID string, leaseUntil time.Time) error
	ClearGenerationUsageReservation(ctx context.Context, taskID string) error
	ListExpiredGenerationUsageReservations(ctx context.Context, dueBefore time.Time, limit int) ([]Task, error)
	// ResolveExpiredGenerationUsageReservation atomically claims an intent only
	// while its durable lease is still expired at dueBefore.
	ResolveExpiredGenerationUsageReservation(ctx context.Context, taskID string, expectedStatus core.TaskStatus, dueBefore time.Time, block *RetryableBlock, errorMsg string, clearReservation bool) error
}

// GenerationUsageAdmissionRepository atomically finishes a task-side usage
// admission intent after the ledger has definitively rejected it. Unlike a
// release saga, quota rejection creates no ledger event to replay.
type GenerationUsageAdmissionRepository interface {
	FinalizeGenerationUsageAdmission(ctx context.Context, taskID string, status core.TaskStatus, block *RetryableBlock, errorMsg string) error
}

// GenerationUsageReleaseRecoveryRepository persists the release saga around
// the external PAY-041 release. Preparing the recovery state before the
// external call keeps an idempotent replay intent durable; resolving it clears
// that intent, the task-side reservation, and the terminal block atomically.
type GenerationUsageReleaseRecoveryRepository interface {
	PrepareGenerationUsageRelease(ctx context.Context, taskID string, block *RetryableBlock, errorMsg string, result *ListingKitResult) error
	ResolveGenerationUsageRelease(ctx context.Context, taskID, terminalError string) error
}

type GenerationUsageEventState string

const (
	GenerationUsageEventReserved  GenerationUsageEventState = "reserved"
	GenerationUsageEventCommitted GenerationUsageEventState = "committed"
	GenerationUsageEventReleased  GenerationUsageEventState = "released"
	GenerationUsageEventReversed  GenerationUsageEventState = "reversed"
)

// GenerationUsageLedgerLookup reads only the deterministic event identity
// used by generation recovery. found=false means that no event was created.
type GenerationUsageLedgerLookup interface {
	LookupGeneration(ctx context.Context, tenantID, taskID string) (state GenerationUsageEventState, found bool, err error)
}

type TaskListSummarySource interface {
	ListTaskSummaryTasks(ctx context.Context, query *TaskListQuery) ([]Task, error)
}

// TaskSDSRepairRepository atomically replaces persisted SDS options before a
// retry and keeps the repair decision in the task execution history.
type TaskSDSRepairRepository interface {
	ReplaceTaskSDSOptionsForRetry(ctx context.Context, taskID string, options *SDSSyncOptions, audit PodExecutionAuditEvent) (*Task, error)
}

type SheinSourceSDSMetadataSource interface {
	ListSheinSourceSDSMetadata(ctx context.Context, query *SheinSourceSDSMetadataQuery) ([]SheinSourceSDSMetadataRecord, error)
}

type SDSBaselineCacheRepository interface {
	// tenantID is optional. When empty, implementations resolve the tenant from ctx.
	// If both tenantID and ctx resolve to a tenant, they must match or the call fails.
	GetSDSBaselineCache(ctx context.Context, tenantID string, baselineKey string) (*SDSBaselineCacheEntry, error)
	// entry.TenantID follows the same contract as GetSDSBaselineCache's tenantID argument.
	SaveSDSBaselineCache(ctx context.Context, entry *SDSBaselineCacheEntry) error
}

type SDSRetirementRepository interface {
	CreateSDSRetirementRun(ctx context.Context, run *SDSRetirementRunRecord, items []SDSRetirementItemRecord) error
	GetSDSRetirementRun(ctx context.Context, runID string) (*SDSRetirementRunRecord, []SDSRetirementItemRecord, error)
	UpdateSDSRetirementItems(ctx context.Context, runID string, updates []SDSRetirementItemSelectionUpdate) error
	SaveSDSRetirementExecution(ctx context.Context, run *SDSRetirementRunRecord, items []SDSRetirementItemRecord) error
	MarkSyncedProductOffShelf(ctx context.Context, tenantID, storeID, syncedProductID int64, now time.Time) error
}

type Assembler interface {
	Assemble(task *Task, product *catalog.ProductSnapshot, approved *productasset.ApprovedAssetInventory) (*ListingKitResult, error)
}

type TargetAwareAssembler interface {
	AssembleForTargets(task *Task, product *catalog.ProductSnapshot, approved *productasset.ApprovedAssetInventory) (*ListingKitResult, error)
}

type AmazonDraftBuilder interface {
	Build(req *GenerateRequest, snapshot *catalog.ProductSnapshot, approved *productasset.ApprovedAssetInventory) (*amazonlisting.AmazonListingDraft, error)
}

type TaskSubmitterConfigurer interface {
	SetTaskSubmitter(submitter TaskSubmitter)
}

type WorkflowClientConfigurer interface {
	ConfigureSheinPublishWorkflowClient(client SheinPublishWorkflowClient, enabled bool)
	ConfigureStandardProductWorkflowClient(client StandardProductWorkflowClient, enabled bool)
	ConfigurePlatformAdaptWorkflowClient(client PlatformAdaptWorkflowClient, enabled bool)
}
