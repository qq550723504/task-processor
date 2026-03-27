package amazonlisting

import (
	"context"
	"testing"
	"time"
)

func TestGetTaskWorkbenchGroupsManualActions(t *testing.T) {
	repo := &stubRepository{}
	svc, err := NewService(&ServiceConfig{
		Repository:     repo,
		ProductService: &stubProductService{},
		Assembler:      NewAssembler(),
		ExportBuilder:  NewExportBuilder(),
		Validator:      NewValidator(),
		AutoFixer:      NewAutoFixer(),
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	task := &Task{
		ID:        "task-workbench",
		Status:    TaskStatusNeedsReview,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Result: &AmazonListingDraft{
			TaskID: "task-workbench",
			Compliance: &AmazonComplianceReport{
				Ready: false,
			},
			Submission: &AmazonSubmissionReport{
				IssueSummary: &AmazonIssueSummary{
					ManualIssues: []AmazonIssue{
						{
							Message:        "Restricted product compliance approval required",
							IsBlocking:     true,
							OperatorAdvice: "该商品可能涉及限制类目或合规审批，需要人工确认资质、证书或审核要求。",
							OperatorAction: OperatorActionCheckCompliance,
						},
						{
							Message:        "Current category is invalid",
							IsBlocking:     false,
							OperatorAdvice: "当前类目或产品类型可能不准确，需要人工重新选择 Amazon 类目和产品类型。",
							OperatorAction: OperatorActionEditCategory,
						},
						{
							Message:        "Another compliance document needed",
							IsBlocking:     true,
							OperatorAdvice: "该商品可能涉及限制类目或合规审批，需要人工确认资质、证书或审核要求。",
							OperatorAction: OperatorActionCheckCompliance,
						},
					},
				},
			},
		},
	}
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	workbench, err := svc.GetTaskWorkbench(context.Background(), "task-workbench")
	if err != nil {
		t.Fatalf("get workbench: %v", err)
	}
	if workbench.Status != TaskStatusNeedsReview {
		t.Fatalf("unexpected status: %s", workbench.Status)
	}
	if workbench.TotalItems != 3 {
		t.Fatalf("expected 3 items, got %d", workbench.TotalItems)
	}
	if workbench.TopAction != OperatorActionCheckCompliance {
		t.Fatalf("expected top action to be compliance, got %s", workbench.TopAction)
	}
	if len(workbench.ActionBuckets) != 2 {
		t.Fatalf("expected 2 action buckets, got %d", len(workbench.ActionBuckets))
	}
	if workbench.ActionBuckets[0].Action != OperatorActionCheckCompliance {
		t.Fatalf("expected compliance bucket first, got %s", workbench.ActionBuckets[0].Action)
	}
	if workbench.ActionBuckets[0].Count != 2 || workbench.ActionBuckets[0].BlockingCount != 2 {
		t.Fatalf("expected compliance bucket counts to match")
	}
	if workbench.ActionBuckets[0].Rank != 1 {
		t.Fatalf("expected first bucket rank 1")
	}
	if workbench.ActionBuckets[0].Priority >= workbench.ActionBuckets[1].Priority {
		t.Fatalf("expected compliance bucket to have higher priority")
	}
}
