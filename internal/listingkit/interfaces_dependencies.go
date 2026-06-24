package listingkit

import (
	"context"
	"time"

	"task-processor/internal/amazonlisting"
	"task-processor/internal/catalog/canonical"
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

type TaskListSummarySource interface {
	ListTaskSummaryTasks(ctx context.Context, query *TaskListQuery) ([]Task, error)
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
