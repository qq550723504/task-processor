package amazonlisting

import (
	"context"

	"github.com/gin-gonic/gin"

	amazonapi "task-processor/internal/amazon/api"
	productasset "task-processor/internal/product/asset"
	"task-processor/internal/product/catalog"
)

type TaskSubmitter interface {
	Submit(taskID string) error
}

type ProductSnapshotQuery struct {
	TenantID   string
	ProductKey string
	Version    uint64
}

type ProductSnapshotReader interface {
	GetProductSnapshot(ctx context.Context, query ProductSnapshotQuery) (catalog.ProductSnapshot, error)
}

// VersionedProductSnapshotReader lets task creation pin the immutable catalog
// version that asynchronous processing must later read.
type VersionedProductSnapshotReader interface {
	GetPublishedProductSnapshot(ctx context.Context, query ProductSnapshotQuery) (catalog.PublishedSnapshot, error)
}

type ApprovedAssetInventoryReader interface {
	GetApprovedInventory(ctx context.Context, scope productasset.InventoryScope) (productasset.ApprovedAssetInventory, error)
}

type Repository interface {
	CreateTask(ctx context.Context, task *Task) error
	GetTask(ctx context.Context, taskID string) (*Task, error)
	ListTasks(ctx context.Context, statuses []TaskStatus, limit int) ([]*Task, error)
	MarkProcessing(ctx context.Context, taskID string) error
	MarkCompleted(ctx context.Context, taskID string, result *AmazonListingDraft) error
	MarkNeedsReview(ctx context.Context, taskID string, result *AmazonListingDraft, reason string) error
	MarkRejected(ctx context.Context, taskID string, reason string) error
	MarkFailed(ctx context.Context, taskID string, errorMsg string) error
	PrepareRetry(ctx context.Context, taskID string) error
	IncrementRetryCount(ctx context.Context, taskID string) error
	UpdateTaskStatus(ctx context.Context, taskID string, status TaskStatus) error
	UpdateTaskError(ctx context.Context, taskID string, errorMsg string) error
	SaveTaskResult(ctx context.Context, taskID string, result *AmazonListingDraft) error
	ResetForRetry(ctx context.Context, taskID string) error
}

type Assembler interface {
	Build(input DraftInput) (*AmazonListingDraft, error)
}

type DraftInput struct {
	TaskID         string
	Request        *GenerateRequest
	Snapshot       catalog.ProductSnapshot
	ApprovedAssets productasset.ApprovedAssetInventory
}

type ValidationReport struct {
	Ready          bool
	NeedsReview    bool
	BlockingIssues []string
	Warnings       []string
	ReviewReasons  []string
}

type Validator interface {
	Validate(req *GenerateRequest, draft *AmazonListingDraft) *ValidationReport
}

type ExportBuilder interface {
	Build(req *GenerateRequest, draft *AmazonListingDraft) *AmazonListingExport
}

type ListingSubmitter interface {
	Preview(ctx context.Context, export *AmazonListingsAPIExport) (*amazonapi.ListingResponse, error)
	Create(ctx context.Context, export *AmazonListingsAPIExport) (*amazonapi.ListingResponse, error)
	Update(ctx context.Context, export *AmazonListingsAPIExport) (*amazonapi.ListingResponse, error)
}

type Service interface {
	CreateGenerateTask(ctx context.Context, req *GenerateRequest) (*Task, error)
	GetTaskResult(ctx context.Context, taskID string) (*TaskResult, error)
	GetTaskWorkbench(ctx context.Context, taskID string) (*TaskWorkbench, error)
	ListTaskQueue(ctx context.Context, query TaskQueueQuery) (*TaskQueueResult, error)
	ReviewTask(ctx context.Context, taskID string, req *ReviewTaskRequest) (*TaskResult, error)
	SubmitTask(ctx context.Context, taskID string, req *SubmitTaskRequest) (*TaskResult, error)
	ProcessListing(ctx context.Context, task *Task) (*AmazonListingDraft, error)
	SetTaskSubmitter(submitter TaskSubmitter)
}

type HandlerService interface {
	CreateGenerateTask(ctx context.Context, req *GenerateRequest) (*Task, error)
	GetTaskResult(ctx context.Context, taskID string) (*TaskResult, error)
	GetTaskWorkbench(ctx context.Context, taskID string) (*TaskWorkbench, error)
	ListTaskQueue(ctx context.Context, query TaskQueueQuery) (*TaskQueueResult, error)
	ReviewTask(ctx context.Context, taskID string, req *ReviewTaskRequest) (*TaskResult, error)
	SubmitTask(ctx context.Context, taskID string, req *SubmitTaskRequest) (*TaskResult, error)
}

type Handler interface {
	GenerateListing(c *gin.Context)
	ListTaskQueue(c *gin.Context)
	GetTaskResult(c *gin.Context)
	GetTaskWorkbench(c *gin.Context)
	ReviewTask(c *gin.Context)
	SubmitTask(c *gin.Context)
}
