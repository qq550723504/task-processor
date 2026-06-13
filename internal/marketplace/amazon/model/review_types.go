package model

import "time"

type TaskStatus string

const (
	TaskStatusPending     TaskStatus = "pending"
	TaskStatusProcessing  TaskStatus = "processing"
	TaskStatusCompleted   TaskStatus = "completed"
	TaskStatusNeedsReview TaskStatus = "needs_review"
	TaskStatusRejected    TaskStatus = "rejected"
	TaskStatusFailed      TaskStatus = "failed"
)

type TaskWorkbench struct {
	TaskID        string               `json:"task_id"`
	Status        TaskStatus           `json:"status"`
	Ready         bool                 `json:"ready"`
	NeedsReview   bool                 `json:"needs_review"`
	ChildTasks    []ChildTaskState     `json:"child_tasks,omitempty"`
	ReviewItems   []AmazonReviewItem   `json:"review_items,omitempty"`
	ReviewSummary *ReviewItemSummary   `json:"review_summary,omitempty"`
	TotalItems    int                  `json:"total_items"`
	TopAction     string               `json:"top_action,omitempty"`
	ActionBuckets []WorkbenchActionBox `json:"action_buckets,omitempty"`
}

type TaskQueueQuery struct {
	Status      []TaskStatus `json:"status,omitempty"`
	Action      string       `json:"action,omitempty"`
	Field       string       `json:"field,omitempty"`
	Severity    string       `json:"severity,omitempty"`
	Source      string       `json:"source,omitempty"`
	ChildStatus string       `json:"child_status,omitempty"`
	NeedsHuman  *bool        `json:"needs_human,omitempty"`
	Limit       int          `json:"limit,omitempty"`
}

type TaskQueueResult struct {
	Items []TaskWorkbench `json:"items,omitempty"`
	Count int             `json:"count"`
	Query TaskQueueQuery  `json:"query"`
}

type ReviewItemSummary struct {
	TotalCount      int            `json:"total_count"`
	BlockingCount   int            `json:"blocking_count"`
	NeedsHumanCount int            `json:"needs_human_count"`
	ByAction        map[string]int `json:"by_action,omitempty"`
	ByField         map[string]int `json:"by_field,omitempty"`
	BySeverity      map[string]int `json:"by_severity,omitempty"`
}

type ChildTaskState struct {
	Kind   string `json:"kind"`
	TaskID string `json:"task_id,omitempty"`
	Status string `json:"status,omitempty"`
	Error  string `json:"error,omitempty"`
}

type WorkbenchActionBox struct {
	Action        string        `json:"action"`
	Label         string        `json:"label"`
	Count         int           `json:"count"`
	BlockingCount int           `json:"blocking_count"`
	Priority      int           `json:"priority"`
	Rank          int           `json:"rank"`
	Items         []AmazonIssue `json:"items,omitempty"`
}

type AmazonReviewItem struct {
	Field          string                 `json:"field,omitempty"`
	Action         string                 `json:"action,omitempty"`
	Severity       string                 `json:"severity,omitempty"`
	Reason         string                 `json:"reason"`
	Source         string                 `json:"source,omitempty"`
	IsBlocking     bool                   `json:"is_blocking,omitempty"`
	NeedsHuman     bool                   `json:"needs_human,omitempty"`
	CurrentValue   string                 `json:"current_value,omitempty"`
	RecommendedFix string                 `json:"recommended_fix,omitempty"`
	Confidence     float64                `json:"confidence,omitempty"`
	IsInferred     bool                   `json:"is_inferred,omitempty"`
	Evidence       []AmazonReviewEvidence `json:"evidence,omitempty"`
}

type AmazonReviewEvidence struct {
	Type   string `json:"type,omitempty"`
	Detail string `json:"detail,omitempty"`
}

type AmazonIssue struct {
	Code           string `json:"code,omitempty"`
	Message        string `json:"message,omitempty"`
	Severity       string `json:"severity,omitempty"`
	Type           string `json:"type,omitempty"`
	Target         string `json:"target,omitempty"`
	IsBlocking     bool   `json:"is_blocking,omitempty"`
	Retryable      bool   `json:"retryable,omitempty"`
	OperatorAdvice string `json:"operator_advice,omitempty"`
	OperatorAction string `json:"operator_action,omitempty"`
}

type AmazonFixRecord struct {
	At      time.Time `json:"at"`
	Issue   string    `json:"issue"`
	Action  string    `json:"action"`
	Success bool      `json:"success"`
}

type AmazonFixEvaluation struct {
	Attempted             bool `json:"attempted"`
	BeforeIssueCount      int  `json:"before_issue_count"`
	AfterIssueCount       int  `json:"after_issue_count"`
	BeforeBlockingCount   int  `json:"before_blocking_count"`
	AfterBlockingCount    int  `json:"after_blocking_count"`
	BlockingReduced       bool `json:"blocking_reduced"`
	FullyResolvedBlocking bool `json:"fully_resolved_blocking"`
}

type AmazonIssueSummary struct {
	TotalCount      int            `json:"total_count"`
	BlockingCount   int            `json:"blocking_count"`
	RetryableCount  int            `json:"retryable_count"`
	ManualCount     int            `json:"manual_count"`
	RetryableIssues []AmazonIssue  `json:"retryable_issues,omitempty"`
	ManualIssues    []AmazonIssue  `json:"manual_issues,omitempty"`
	ManualAdvices   []string       `json:"manual_advices,omitempty"`
	ManualActions   []string       `json:"manual_actions,omitempty"`
	ActionCounts    map[string]int `json:"action_counts,omitempty"`
}

const (
	OperatorActionFillBrand       = "fill_brand"
	OperatorActionEditBrand       = "edit_brand"
	OperatorActionFillBullets     = "fill_bullets"
	OperatorActionEditBullets     = "edit_bullets"
	OperatorActionEditTitle       = "edit_title"
	OperatorActionFillMainImage   = "fill_main_image"
	OperatorActionFillImages      = "fill_images"
	OperatorActionFillPrice       = "fill_price"
	OperatorActionEditPrice       = "edit_price"
	OperatorActionFillSKU         = "fill_sku"
	OperatorActionEditSKU         = "edit_sku"
	OperatorActionCheckCompliance = "check_compliance"
	OperatorActionCheckHazmat     = "check_hazmat"
	OperatorActionEditCategory    = "edit_category"
	OperatorActionFillAttributes  = "fill_attributes"
	OperatorActionManualReview    = "manual_review"
)
