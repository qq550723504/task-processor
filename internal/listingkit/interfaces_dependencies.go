package listingkit

import (
	"context"
	"time"

	"task-processor/internal/amazonlisting"
	"task-processor/internal/catalog/canonical"
	"task-processor/internal/listingkit/core"
	"task-processor/internal/productenrich"
	"task-processor/internal/productimage"
)

type TaskSubmitter interface{ Submit(taskID string) error }

type ProductService interface {
	CreateGenerateTask(ctx context.Context, req *productenrich.GenerateRequest) (*productenrich.Task, error)
	GetTaskResult(ctx context.Context, taskID string) (*productenrich.TaskResult, error)
	ProcessProduct(ctx context.Context, task *productenrich.Task) (*productenrich.ProductJSON, error)
}

type ImageService interface {
	CreateProcessTask(ctx context.Context, req *productimage.ImageProcessRequest) (*productimage.Task, error)
	GetTaskResult(ctx context.Context, taskID string) (*productimage.TaskResult, error)
	ProcessImages(ctx context.Context, task *productimage.Task) (*productimage.ImageProcessResult, error)
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

// UsageSettlementRepository is an optional task-repository extension used to
// clear a settlement-only retryable block without re-running generation.
type UsageSettlementRepository interface {
	ResolveUsageSettlement(ctx context.Context, taskID string) error
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

type CanonicalProductCacheRepository interface {
	GetCanonicalProductCache(ctx context.Context, fingerprint string) (*canonical.Product, error)
	SaveCanonicalProductCache(ctx context.Context, fingerprint string, product *canonical.Product, sourceTaskID string) error
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
	Assemble(task *Task, canonical *canonical.Product, image *productimage.ImageProcessResult) *ListingKitResult
}

type TargetAwareAssembler interface {
	AssembleForTargets(task *Task, canonical *canonical.Product, images map[string]*productimage.ImageProcessResult) *ListingKitResult
}

type AmazonDraftBuilder interface {
	Build(req *GenerateRequest, canonical *canonical.Product, image *productimage.ImageProcessResult) *amazonlisting.AmazonListingDraft
}

type TaskSubmitterConfigurer interface {
	SetTaskSubmitter(submitter TaskSubmitter)
}

type WorkflowClientConfigurer interface {
	ConfigureSheinPublishWorkflowClient(client SheinPublishWorkflowClient, enabled bool)
	ConfigureStandardProductWorkflowClient(client StandardProductWorkflowClient, enabled bool)
	ConfigurePlatformAdaptWorkflowClient(client PlatformAdaptWorkflowClient, enabled bool)
}
