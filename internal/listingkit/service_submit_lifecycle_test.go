package listingkit

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"task-processor/internal/listingkit/core"
	sheinpub "task-processor/internal/publishing/shein"
	sheinother "task-processor/internal/shein/api/other"
	sheinproduct "task-processor/internal/shein/api/product"
)

func TestSubmitTaskReturnsBlockedWhenReadinessIsNotReady(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	task.Result.Shein.SaleAttributeResolution.Status = "partial"
	task.Result.Shein.SaleAttributeResolution.SKCAttributes = nil
	task.Result.Shein.RequestDraft.SKCList[0].SaleAttribute = nil
	task.Result.Shein.RequestDraft.SKCList[0].SKUList[0].SaleAttributes = nil
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	svc, err := NewService(newTestServiceConfig(
		repo,
		withTestSheinProductAPIBuilder(stubSheinProductAPIBuilder{}),
	))
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	_, err = svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{Platform: "shein", Action: "publish", IdempotencyKey: "publish-fail-123"})
	if err == nil || !errors.Is(err, ErrSubmitBlocked) {
		t.Fatalf("submit err = %v, want ErrSubmitBlocked", err)
	}
}

func TestSubmitTaskPersistsSheinSubmissionWhenProductAPIUnavailable(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	svc, err := NewService(newTestServiceConfig(
		repo,
		withTestSheinProductAPIBuilder(stubSheinProductAPIBuilder{
			msg: "store token missing",
		}),
	))
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	_, err = svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{Platform: "shein", Action: "publish", IdempotencyKey: "publish-fail-123"})
	if err == nil || !strings.Contains(err.Error(), "store token missing") {
		t.Fatalf("submit err = %v, want store token missing", err)
	}
	saved, getErr := repo.GetTask(context.Background(), task.ID)
	if getErr != nil {
		t.Fatalf("get task: %v", getErr)
	}
	if saved.Result == nil || saved.Result.Shein == nil || saved.Result.Shein.Submission == nil {
		t.Fatalf("submission was not persisted: %+v", saved.Result)
	}
	if saved.Result.Shein.Submission.LastAction != "publish" ||
		saved.Result.Shein.Submission.LastStatus != "failed" ||
		!strings.Contains(saved.Result.Shein.Submission.LastError, "store token missing") {
		t.Fatalf("submission failure = %+v", saved.Result.Shein.Submission)
	}
	if saved.Result.Shein.Submission.CurrentAction != "" || saved.Result.Shein.Submission.CurrentPhase != "" || saved.Result.Shein.Submission.CurrentRequestID != "" {
		t.Fatalf("submit current state was not cleared: %+v", saved.Result.Shein.Submission)
	}
	if saved.Result.Shein.Submission.Publish == nil || saved.Result.Shein.Submission.Publish.RequestID != "publish-fail-123" {
		t.Fatalf("publish record = %+v, want request id publish-fail-123", saved.Result.Shein.Submission.Publish)
	}
	if saved.Result.Shein.Submission.Publish.Phase != sheinpub.SubmissionPhaseValidate {
		t.Fatalf("publish phase = %q, want %q", saved.Result.Shein.Submission.Publish.Phase, sheinpub.SubmissionPhaseValidate)
	}
	if len(saved.Result.Shein.SubmissionEvents) == 0 || saved.Result.Shein.SubmissionEvents[len(saved.Result.Shein.SubmissionEvents)-1].RequestID != "publish-fail-123" {
		t.Fatalf("submission events = %+v, want request id publish-fail-123", saved.Result.Shein.SubmissionEvents)
	}
}

func TestSubmitTaskPersistsSheinSubmissionOnPublishSuccess(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	task.Result.Shein.PreviewProduct.SPUName = "Display Title Should Not Be Submitted"
	var submitted *sheinproduct.Product
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	svc, err := NewService(newTestServiceConfig(
		repo,
		withTestSheinProductAPIBuilder(stubSheinProductAPIBuilder{
			api: stubSheinProductAPI{
				publishHook: func(product *sheinproduct.Product) {
					submitted = product
				},
				publishResponse: &sheinproduct.SheinResponse{
					Code: "0",
					Msg:  "success",
					Info: sheinproduct.ResponseInfo{
						Success: true,
						SPUName: "SPU-123",
						Version: "v1",
					},
				},
			},
		}),
		withDefaultTestSheinImageAPI(),
	))
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	preview, err := svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{Platform: "shein", Action: "publish", IdempotencyKey: "publish-123"})
	if err != nil {
		t.Fatalf("submit task: %v", err)
	}
	if submitted == nil {
		t.Fatal("expected publish payload to be captured")
	}
	if submitted.SPUName != "" {
		t.Fatalf("submitted spu_name = %q, want empty for new SHEIN product", submitted.SPUName)
	}
	if len(submitted.MultiLanguageNameList) == 0 {
		t.Fatal("submitted product title is missing from multi_language_name_list")
	}
	if preview == nil || preview.Shein == nil || preview.Shein.Submission == nil {
		t.Fatalf("preview submission = %+v", preview)
	}
	if preview.Shein.Submission.LastAction != "publish" || preview.Shein.Submission.LastStatus != "success" {
		t.Fatalf("submission = %+v", preview.Shein.Submission)
	}
	if preview.Shein.Submission.Publish == nil || preview.Shein.Submission.Publish.Result == nil || !preview.Shein.Submission.Publish.Result.Success {
		t.Fatalf("submission publish = %+v", preview.Shein.Submission.Publish)
	}
	if preview.Shein.Submission.CurrentAction != "" || preview.Shein.Submission.CurrentPhase != "" || preview.Shein.Submission.CurrentRequestID != "" {
		t.Fatalf("submit current state was not cleared: %+v", preview.Shein.Submission)
	}
	if preview.Shein.Submission.Publish.RequestID != "publish-123" {
		t.Fatalf("publish request id = %q, want publish-123", preview.Shein.Submission.Publish.RequestID)
	}
	if preview.Shein.Submission.Publish.StartedAt.IsZero() || preview.Shein.Submission.Publish.FinishedAt == nil {
		t.Fatalf("publish timing was not recorded: %+v", preview.Shein.Submission.Publish)
	}
	if preview.Shein.Submission.Publish.SubmitSnapshot == nil || len(preview.Shein.Submission.Publish.SubmitSnapshot.MultiLanguageNameList) == 0 {
		t.Fatalf("submit snapshot = %+v", preview.Shein.Submission.Publish.SubmitSnapshot)
	}
	if len(preview.Shein.SubmissionEvents) == 0 || preview.Shein.SubmissionEvents[len(preview.Shein.SubmissionEvents)-1].RequestID != "publish-123" {
		t.Fatalf("submission events = %+v, want request id publish-123", preview.Shein.SubmissionEvents)
	}
}

func TestSubmitTaskClearsNeedsReviewAfterPublishSuccess(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	task.Status = TaskStatusNeedsReview
	task.Error = "旧的待审核原因"
	task.Result.Status = string(TaskStatusNeedsReview)
	task.Result.ReviewReasons = []string{"需要人工确认类目"}
	task.Result.Summary = &GenerationSummary{
		NeedsReview:   true,
		Warnings:      []string{"需要人工确认类目"},
		ReviewCount:   1,
		BlockingCount: 0,
		IssueCount:    1,
	}
	task.Result.WorkflowIssues = []WorkflowIssue{{
		Code:     "shein_review_required",
		Severity: WorkflowIssueSeverityReview,
		Stage:    "shein_review",
		Message:  "需要人工确认类目",
	}}
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	svc, err := NewService(newTestServiceConfig(
		repo,
		withTestSheinProductAPIBuilder(stubSheinProductAPIBuilder{
			api: stubSheinProductAPI{
				publishResponse: &sheinproduct.SheinResponse{
					Code: "0",
					Msg:  "success",
					Info: sheinproduct.ResponseInfo{Success: true, SPUName: "SPU-123"},
				},
			},
		}),
		withDefaultTestSheinImageAPI(),
	))
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	preview, err := svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{
		Platform:       "shein",
		Action:         "publish",
		IdempotencyKey: "publish-clear-review-123",
	})
	if err != nil {
		t.Fatalf("submit task: %v", err)
	}
	if preview.Status != TaskStatusCompleted {
		t.Fatalf("preview status = %q, want %q", preview.Status, TaskStatusCompleted)
	}
	if preview.NeedsReview {
		t.Fatalf("preview needs_review = true, want false")
	}
	if preview.Overview != nil && len(preview.Overview.ReviewReasons) != 0 {
		t.Fatalf("preview review reasons = %#v, want none", preview.Overview.ReviewReasons)
	}

	saved, err := repo.GetTask(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if saved.Status != TaskStatusCompleted {
		t.Fatalf("saved status = %q, want %q", saved.Status, TaskStatusCompleted)
	}
	if saved.Error != "" {
		t.Fatalf("saved error = %q, want empty", saved.Error)
	}
	if saved.Result == nil || saved.Result.Status != string(TaskStatusCompleted) {
		t.Fatalf("saved result status = %+v, want completed", saved.Result)
	}
	if len(saved.Result.ReviewReasons) != 0 {
		t.Fatalf("saved review reasons = %#v, want none", saved.Result.ReviewReasons)
	}
	if saved.Result.Summary == nil || saved.Result.Summary.NeedsReview {
		t.Fatalf("saved summary = %+v, want needs_review false", saved.Result.Summary)
	}
	if len(saved.Result.WorkflowIssues) != 0 {
		t.Fatalf("saved workflow issues = %+v, want cleared shein review issues", saved.Result.WorkflowIssues)
	}
}

func TestNormalizeSheinSubmitPackageRepairsResolvedSaleAttributes(t *testing.T) {
	t.Parallel()

	task := makeReadySheinTask()
	task.Result.Shein.RequestDraft.SKCList[0].SaleAttribute = nil
	task.Result.Shein.RequestDraft.SKCList[0].SKUList[0].SaleAttributes = nil
	task.Result.Shein.PreviewProduct.SKCList[0].SaleAttribute.AttributeID = 0
	task.Result.Shein.PreviewProduct.SKCList[0].SaleAttribute.AttributeValueID = 0
	task.Result.Shein.PreviewProduct.SKCList[0].SKUS[0].SaleAttributeList = nil

	svc, err := NewService(newTestServiceConfig(&stubSubmitRepo{}))
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	host := svc.(*service)

	host.taskSubmissionExecutionOrDefault().normalizeSheinSubmitPackage(task, task.Result.Shein, &SubmitTaskRequest{Platform: "shein", Action: "publish"}, "publish")

	if task.Result.Shein.RequestDraft.SKCList[0].SaleAttribute == nil || task.Result.Shein.RequestDraft.SKCList[0].SaleAttribute.AttributeID <= 0 {
		t.Fatalf("request draft skc sale attribute = %+v, want repaired resolved value", task.Result.Shein.RequestDraft.SKCList[0].SaleAttribute)
	}
	if len(task.Result.Shein.RequestDraft.SKCList[0].SKUList[0].SaleAttributes) == 0 || task.Result.Shein.RequestDraft.SKCList[0].SKUList[0].SaleAttributes[0].AttributeID <= 0 {
		t.Fatalf("request draft sku sale attributes = %+v, want repaired resolved value", task.Result.Shein.RequestDraft.SKCList[0].SKUList[0].SaleAttributes)
	}
	if task.Result.Shein.PreviewProduct == nil || task.Result.Shein.PreviewProduct.SKCList[0].SaleAttribute.AttributeID <= 0 {
		t.Fatalf("preview skc sale attribute = %+v, want repaired preview product", task.Result.Shein.PreviewProduct)
	}
	if len(task.Result.Shein.PreviewProduct.SKCList[0].SKUS[0].SaleAttributeList) == 0 || task.Result.Shein.PreviewProduct.SKCList[0].SKUS[0].SaleAttributeList[0].AttributeID <= 0 {
		t.Fatalf("preview sku sale attribute list = %+v, want repaired preview product", task.Result.Shein.PreviewProduct.SKCList[0].SKUS[0].SaleAttributeList)
	}
}

func TestSubmitTaskBlocksResolvedSaleAttributesWithoutValueIDs(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	task.Result.Shein.SaleAttributeResolution.Status = "resolved"
	task.Result.Shein.SaleAttributeResolution.PrimaryAttributeID = 1001466
	task.Result.Shein.SaleAttributeResolution.SecondaryAttributeID = 0
	task.Result.Shein.SaleAttributeResolution.SKCAttributes = []SheinResolvedSaleAttribute{{
		Scope:       "skc",
		Name:        "Plug(Voltage)",
		Value:       "white",
		AttributeID: 1001466,
	}}
	task.Result.Shein.SaleAttributeResolution.SKUAttributes = nil
	task.Result.Shein.RequestDraft.SKCList[0].SaleAttribute = nil
	task.Result.Shein.RequestDraft.SKCList[0].SKUList[0].SaleAttributes = nil
	task.Result.Shein.PreviewProduct.SKCList[0].SaleAttribute.AttributeID = 0
	task.Result.Shein.PreviewProduct.SKCList[0].SaleAttribute.AttributeValueID = 0
	task.Result.Shein.PreviewProduct.SKCList[0].SKUS[0].SaleAttributeList = nil
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	svc, err := NewService(newTestServiceConfig(
		repo,
		withTestSheinProductAPIBuilder(stubSheinProductAPIBuilder{}),
	))
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	_, err = svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{
		Platform:       "shein",
		Action:         "publish",
		IdempotencyKey: "publish-missing-sale-value-id",
	})
	if err == nil || !errors.Is(err, ErrSubmitBlocked) {
		t.Fatalf("submit err = %v, want ErrSubmitBlocked", err)
	}
	if !strings.Contains(err.Error(), "当前仍有关键字段未完成") {
		t.Fatalf("submit err = %v, want readiness blocked summary", err)
	}
}

func TestSubmitTaskRejectsCodeZeroPublishWhenInfoSuccessIsFalse(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	svc, err := NewService(newTestServiceConfig(
		repo,
		withTestSheinProductAPIBuilder(stubSheinProductAPIBuilder{
			api: stubSheinProductAPI{
				publishResponse: &sheinproduct.SheinResponse{
					Code: "0",
					Msg:  "OK",
					Info: sheinproduct.ResponseInfo{
						Success: false,
						SPUName: "SPU-123",
					},
				},
				recordResponse: makeSheinRecordResponse(sheinproduct.RecordItem{
					RecordID:     "record-accepted",
					SupplierCode: "SUP-submit-task-1",
					State:        2,
					AuditState:   3,
				}),
			},
		}),
		withDefaultTestSheinImageAPI(),
	))
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	_, err = svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{Platform: "shein", Action: "publish", IdempotencyKey: "publish-ok-123"})
	if err == nil {
		t.Fatal("submit task err = nil, want failure")
	}
	saved, getErr := repo.GetTask(context.Background(), task.ID)
	if getErr != nil {
		t.Fatalf("get task: %v", getErr)
	}
	if saved.Result.Shein.Submission == nil || saved.Result.Shein.Submission.LastStatus != sheinpub.SubmissionStatusFailed {
		t.Fatalf("submission = %+v, want failed", saved.Result.Shein.Submission)
	}
	if saved.Result.Shein.Submission.Publish == nil || saved.Result.Shein.Submission.Publish.Result == nil || saved.Result.Shein.Submission.Publish.Result.Success {
		t.Fatalf("publish result = %+v, want unsuccessful result", saved.Result.Shein.Submission.Publish)
	}
}

func TestSubmitTaskRemembersSheinResolutionCacheAfterPublishSuccess(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}
	cacheStore := &submitResolutionCacheStore{}
	svc, err := NewService(newTestServiceConfig(
		repo,
		withTestConfig(func(cfg *ServiceConfig) {
			cfg.Shein.SheinResolutionCacheStore = cacheStore
			cfg.Shein.SheinCategoryResolver = sheinpub.NewCachedCategoryResolver(
				sheinpub.NewCategoryResolver(nil),
				cacheStore,
			)
			cfg.Shein.SheinAttributeResolver = sheinpub.NewCachedAttributeResolver(
				sheinpub.NewAttributeResolver(nil, nil),
				cacheStore,
			)
			cfg.Shein.SheinSaleAttributeResolver = sheinpub.NewCachedSaleAttributeResolver(
				sheinpub.NewSaleAttributeResolver(nil, nil),
				cacheStore,
			)
		}),
		withTestSheinProductAPIBuilder(stubSheinProductAPIBuilder{
			api: stubSheinProductAPI{
				publishResponse: &sheinproduct.SheinResponse{
					Code: "0",
					Msg:  "success",
					Info: sheinproduct.ResponseInfo{Success: true, SPUName: "SPU-123"},
				},
			},
		}),
		withDefaultTestSheinImageAPI(),
	))
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	preview, err := svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{Platform: "shein", Action: "publish", IdempotencyKey: "publish-cache-123"})
	if err != nil {
		t.Fatalf("submit task: %v", err)
	}

	if preview.Shein.ResolutionCache == nil ||
		preview.Shein.ResolutionCache.Category == nil ||
		preview.Shein.ResolutionCache.Attributes == nil ||
		preview.Shein.ResolutionCache.SaleAttributes == nil {
		t.Fatalf("resolution cache summary = %+v, want category/attribute/sale_attribute after publish", preview.Shein.ResolutionCache)
	}
	if preview.Shein.ResolutionCache.Category.HitSource != sheinpub.ResolutionCacheHitSourcePublishRemembered ||
		preview.Shein.ResolutionCache.Attributes.HitSource != sheinpub.ResolutionCacheHitSourcePublishRemembered ||
		preview.Shein.ResolutionCache.SaleAttributes.HitSource != sheinpub.ResolutionCacheHitSourcePublishRemembered {
		t.Fatalf("resolution cache hit sources = %+v, want publish_remembered", preview.Shein.ResolutionCache)
	}
	if preview.Shein.ResolutionCache.Pricing == nil || preview.Shein.ResolutionCache.Pricing.UpdatedAt == nil {
		t.Fatalf("pricing resolution cache = %+v, want updated_at after publish", preview.Shein.ResolutionCache.Pricing)
	}
	entries := cacheStore.snapshot()
	if len(entries) != 4 {
		t.Fatalf("cache entry count = %d, want 4 including pricing: %+v", len(entries), entries)
	}
	foundPricing := false
	for _, entry := range entries {
		if entry.Source != "manual_cache" || !entry.Manual {
			t.Fatalf("cache entry = %+v, want manual_cache confirmed by publish", entry)
		}
		if entry.CacheKind == sheinpub.ResolutionCacheKindPricing {
			foundPricing = true
		}
	}
	if !foundPricing {
		t.Fatalf("cache entries = %+v, want pricing cache entry", entries)
	}
}

func TestSubmitTaskDoesNotAutoConfirmRemoteRecordAfterPublishSuccess(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}
	svc, err := NewService(newTestServiceConfig(
		repo,
		withTestSheinProductAPIBuilder(stubSheinProductAPIBuilder{
			api: stubSheinProductAPI{
				publishResponse: &sheinproduct.SheinResponse{
					Code: "0",
					Msg:  "success",
					Info: sheinproduct.ResponseInfo{Success: true, SPUName: "SPU-123"},
				},
				recordResponse: makeSheinRecordResponse(sheinproduct.RecordItem{
					RecordID:     "record-123",
					SupplierCode: "SUP-submit-task-1",
					State:        2,
					AuditState:   3,
				}),
			},
		}),
		withDefaultTestSheinImageAPI(),
	))
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	preview, err := svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{Platform: "shein", Action: "publish", IdempotencyKey: "remote-123"})
	if err != nil {
		t.Fatalf("submit task: %v", err)
	}

	if got := preview.Shein.Submission.RemoteStatus; got != "" {
		t.Fatalf("remote status = %q, want empty without auto confirmation", got)
	}
	if preview.Shein.Submission.Publish.RemoteRecordID != "" {
		t.Fatalf("remote record id = %q, want empty without auto confirmation", preview.Shein.Submission.Publish.RemoteRecordID)
	}
	if repo.hasSavedSubmissionPhase(sheinpub.SubmissionPhaseConfirmRemote) {
		t.Fatalf("confirm_remote phase should not be persisted; saved phases = %+v", repo.savedSubmissionPhases)
	}
}

func TestCollectSheinRemoteLookupCodesIncludesNormalizedSupplierSKUs(t *testing.T) {
	t.Parallel()

	pkg := &SheinPackage{
		PreviewProduct: &sheinproduct.Product{
			SupplierCode: "MG8089003001",
			SKCList: []sheinproduct.SKC{
				{SupplierCode: strPtr("MG8089003001"), SKUS: []sheinproduct.SKU{{SupplierSKU: "MG8089003001-V245612-T47D318DF"}}},
				{SupplierCode: strPtr("MG8089003002"), SKUS: []sheinproduct.SKU{{SupplierSKU: "MG8089003002-V245613-T47D318DF"}}},
				{SupplierCode: strPtr("MG8089003003"), SKUS: []sheinproduct.SKU{{SupplierSKU: "MG8089003003-V245614-T47D318DF"}}},
			},
		},
	}

	codes := sheinpub.CollectRemoteLookupCodes(pkg, "MG8089003001")

	expected := []string{
		"MG8089003001",
		"MG8089003001-V245612-T47D318DF",
		"MG8089003002",
		"MG8089003002-V245613-T47D318DF",
		"MG8089003003",
		"MG8089003003-V245614-T47D318DF",
	}
	for _, want := range expected {
		if !containsString(codes, want) {
			t.Fatalf("lookup codes = %+v, want to contain %q", codes, want)
		}
	}
}

func TestSubmitTaskTreatsAcceptedPublishAsSuccessfulWithoutRemoteConfirmation(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}
	svc, err := NewService(newTestServiceConfig(
		repo,
		withTestSheinProductAPIBuilder(stubSheinProductAPIBuilder{
			api: stubSheinProductAPI{
				publishResponse: &sheinproduct.SheinResponse{
					Code: "0",
					Msg:  "success",
					Info: sheinproduct.ResponseInfo{Success: true, SPUName: "SPU-123"},
				},
				recordResponse: makeSheinRecordResponse(),
			},
		}),
		withDefaultTestSheinImageAPI(),
	))
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	preview, err := svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{Platform: "shein", Action: "publish", IdempotencyKey: "pending-123"})
	if err != nil {
		t.Fatalf("submit task: %v", err)
	}

	if got := preview.Shein.Submission.RemoteStatus; got != "" {
		t.Fatalf("remote status = %q, want empty", got)
	}
	if preview.Shein.Submission.LastStatus != sheinpub.SubmissionStatusSuccess {
		t.Fatalf("last status = %q, want success", preview.Shein.Submission.LastStatus)
	}
	if got := preview.Shein.Submission.Publish.RemoteMessage; got != "" {
		t.Fatalf("remote message = %q, want empty without remote confirmation", got)
	}
}

func TestSubmitTaskDoesNotQueryRemoteInventoryWhenPublishSucceeds(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}
	var queriedSPU string
	svc, err := NewService(newTestServiceConfig(
		repo,
		withTestSheinProductAPIBuilder(stubSheinProductAPIBuilder{
			api: stubSheinProductAPI{
				publishResponse: &sheinproduct.SheinResponse{
					Code: "0",
					Msg:  "success",
					Info: sheinproduct.ResponseInfo{Success: true, SPUName: "SPU-123"},
				},
				recordResponse: makeSheinRecordResponse(),
				inventoryHook: func(spuName string) {
					queriedSPU = spuName
				},
				inventoryResp: &sheinproduct.InventoryQueryResponse{
					Code: "0",
					Msg:  "success",
					Info: sheinproduct.InventoryInfo{SpuName: "SPU-123"},
				},
			},
		}),
		withDefaultTestSheinImageAPI(),
	))
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	preview, err := svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{Platform: "shein", Action: "publish", IdempotencyKey: "inventory-confirm-123"})
	if err != nil {
		t.Fatalf("submit task: %v", err)
	}

	if queriedSPU != "" {
		t.Fatalf("inventory lookup spu_name = %q, want no inventory lookup on success", queriedSPU)
	}
	if got := preview.Shein.Submission.RemoteStatus; got != "" {
		t.Fatalf("remote status = %q, want empty", got)
	}
	if got := preview.Shein.Submission.Publish.RemoteMessage; got != "" {
		t.Fatalf("remote message = %q, want empty without remote confirmation", got)
	}
}

func TestSubmitTaskKeepsPublishSuccessWithoutInspectingRemoteRecord(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}
	svc, err := NewService(newTestServiceConfig(
		repo,
		withTestSheinProductAPIBuilder(stubSheinProductAPIBuilder{
			api: stubSheinProductAPI{
				publishResponse: &sheinproduct.SheinResponse{
					Code: "0",
					Msg:  "success",
					Info: sheinproduct.ResponseInfo{Success: true, SPUName: "SPU-123"},
				},
				recordResponse: makeSheinRecordResponse(sheinproduct.RecordItem{
					RecordID:     "record-draft",
					SupplierCode: "SUP-submit-task-1",
					State:        1,
					AuditState:   2,
				}),
			},
		}),
		withDefaultTestSheinImageAPI(),
	))
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	preview, err := svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{Platform: "shein", Action: "publish", IdempotencyKey: "draft-remote-123"})
	if err != nil {
		t.Fatalf("submit err = %v, want nil", err)
	}

	saved, err := repo.GetTask(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	submission := saved.Result.Shein.Submission
	if submission == nil {
		t.Fatal("expected submission report")
	}
	if submission.RemoteStatus != "" {
		t.Fatalf("remote status = %q, want empty", submission.RemoteStatus)
	}
	if submission.LastStatus != sheinpub.SubmissionStatusSuccess {
		t.Fatalf("last status = %q, want success", submission.LastStatus)
	}
	if submission.Publish == nil || submission.Publish.RemoteMessage != "" {
		t.Fatalf("publish record = %+v, want no remote confirmation message", submission.Publish)
	}
	if preview.Shein.Submission.RemoteStatus != "" {
		t.Fatalf("preview remote status = %q, want empty", preview.Shein.Submission.RemoteStatus)
	}
}

func TestSubmitTaskSkipsRemoteRecordLookupAfterAcceptedPublish(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}
	var recordCalls int32
	svc, err := NewService(newTestServiceConfig(
		repo,
		withTestSheinProductAPIBuilder(stubSheinProductAPIBuilder{
			api: stubSheinProductAPI{
				publishResponse: &sheinproduct.SheinResponse{
					Code: "0",
					Msg:  "success",
					Info: sheinproduct.ResponseInfo{Success: true, SPUName: "SPU-123"},
				},
				recordHook: func(request *sheinproduct.ProductRecordRequest) {
					atomic.AddInt32(&recordCalls, 1)
				},
				recordResponse: nil,
			},
		}),
		withDefaultTestSheinImageAPI(),
	))
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	preview, err := svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{Platform: "shein", Action: "publish", IdempotencyKey: "confirm-only-123"})
	if err != nil {
		t.Fatalf("submit task: %v", err)
	}

	if got := preview.Shein.Submission.RemoteStatus; got != "" {
		t.Fatalf("remote status = %q, want empty", got)
	}
	if got := atomic.LoadInt32(&recordCalls); got != 0 {
		t.Fatalf("record calls = %d, want 0 record lookups", got)
	}
	if preview.Shein.Submission.Publish.RemoteRecordID != "" {
		t.Fatalf("remote record id = %q, want empty when record not visible", preview.Shein.Submission.Publish.RemoteRecordID)
	}
	if got := preview.Shein.Submission.Publish.RemoteMessage; got != "" {
		t.Fatalf("remote message = %q, want empty without remote confirmation", got)
	}
}

func TestRefreshSubmissionStatusUpdatesRemoteRecordWithoutSubmitting(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	now := time.Now().Add(-time.Hour)
	task.Result.Shein.Submission = &sheinpub.SubmissionReport{
		LastAction:  "publish",
		LastStatus:  sheinpub.SubmissionStatusSuccess,
		SubmittedAt: &now,
		Publish: &sheinpub.SubmissionRecord{
			Action:       "publish",
			Status:       sheinpub.SubmissionStatusSuccess,
			SubmittedAt:  now,
			RequestID:    "refresh-123",
			SupplierCode: "SKC-1",
			StartedAt:    now,
			FinishedAt:   &now,
		},
	}
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}
	var publishCalls int32
	var recordCalls int32
	svc, err := NewService(newTestServiceConfig(
		repo,
		withTestSheinProductAPIBuilder(stubSheinProductAPIBuilder{
			api: stubSheinProductAPI{
				publishHook: func(product *sheinproduct.Product) {
					atomic.AddInt32(&publishCalls, 1)
				},
				recordHook: func(request *sheinproduct.ProductRecordRequest) {
					atomic.AddInt32(&recordCalls, 1)
				},
				recordResponse: makeSheinRecordResponse(sheinproduct.RecordItem{
					RecordID:     "record-refreshed",
					SupplierCode: "SKC-1",
					State:        4,
					AuditState:   5,
				}),
			},
		}),
		withDefaultTestSheinImageAPI(),
	))
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	preview, err := svc.RefreshSubmissionStatus(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("refresh submission status: %v", err)
	}

	if got := atomic.LoadInt32(&publishCalls); got != 0 {
		t.Fatalf("publish calls = %d, want 0", got)
	}
	if got := atomic.LoadInt32(&recordCalls); got != 1 {
		t.Fatalf("record calls = %d, want 1", got)
	}
	if preview.Shein.Submission.RemoteStatus != sheinpub.SubmissionRemoteStatusConfirmed {
		t.Fatalf("remote status = %q, want confirmed", preview.Shein.Submission.RemoteStatus)
	}
	if preview.Shein.Submission.Publish.RemoteRecordID != "record-refreshed" {
		t.Fatalf("remote record id = %q, want record-refreshed", preview.Shein.Submission.Publish.RemoteRecordID)
	}
	if len(preview.Shein.SubmissionEvents) == 0 || preview.Shein.SubmissionEvents[0].Phase != sheinpub.SubmissionPhaseConfirmRemote {
		t.Fatalf("submission events = %+v, want confirm_remote event", preview.Shein.SubmissionEvents)
	}
}

func TestRefreshSubmissionStatusFallsBackToRecordLookupWhenOnWayClientUnavailable(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	now := time.Now().Add(-time.Hour)
	task.Result.Shein.Submission = &sheinpub.SubmissionReport{
		LastAction:  "publish",
		LastStatus:  sheinpub.SubmissionStatusSuccess,
		SubmittedAt: &now,
		LastResult: &sheinpub.SubmissionResponse{
			Code:    "0",
			Message: "OK",
			Success: true,
			SPUName: "SPU-PUBLISH",
		},
		Publish: &sheinpub.SubmissionRecord{
			Action:       "publish",
			Status:       sheinpub.SubmissionStatusSuccess,
			SubmittedAt:  now,
			RequestID:    "refresh-publish-spu-skip",
			SupplierCode: "SKC-1",
			StartedAt:    now,
			FinishedAt:   &now,
			Result: &sheinpub.SubmissionResponse{
				Code:    "0",
				Message: "OK",
				Success: true,
				SPUName: "SPU-PUBLISH",
			},
		},
	}
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}
	var publishCalls int32
	var recordCalls int32
	svc, err := NewService(newTestServiceConfig(
		repo,
		withTestSheinProductAPIBuilder(stubSheinProductAPIBuilder{
			api: stubSheinProductAPI{
				recordHook: func(*sheinproduct.ProductRecordRequest) {
					atomic.AddInt32(&recordCalls, 1)
				},
				recordResponse: makeSheinRecordResponse(sheinproduct.RecordItem{
					RecordID:     "record-should-not-be-used",
					SupplierCode: "SKC-1",
					SpuName:      "SPU-PUBLISH",
					State:        1,
					AuditState:   2,
				}),
				publishHook: func(product *sheinproduct.Product) {
					atomic.AddInt32(&publishCalls, 1)
				},
			},
		}),
		withDefaultTestSheinImageAPI(),
	))
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	preview, err := svc.RefreshSubmissionStatus(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("refresh submission status: %v", err)
	}
	if got := atomic.LoadInt32(&publishCalls); got != 0 {
		t.Fatalf("publish calls = %d, want 0", got)
	}
	if got := atomic.LoadInt32(&recordCalls); got != 1 {
		t.Fatalf("record calls = %d, want 1", got)
	}
	if preview.Shein.Submission.RemoteStatus != sheinpub.SubmissionRemoteStatusConfirmed {
		t.Fatalf("remote status = %q, want confirmed", preview.Shein.Submission.RemoteStatus)
	}
	if preview.Shein.Submission.Publish.RemoteRecordID != "record-should-not-be-used" {
		t.Fatalf("remote record id = %q, want record-should-not-be-used", preview.Shein.Submission.Publish.RemoteRecordID)
	}
	if got := preview.Shein.Submission.Publish.RemoteMessage; !strings.Contains(got, "publish API reported success") {
		t.Fatalf("remote message = %q, want record-based confirmation message", got)
	}
}

func TestResolveSheinSubmitRemoteStatusConfirmsPublishViaBatchCheckOnWay(t *testing.T) {
	t.Parallel()

	var recordCalls int32
	confirmation, err := resolveSheinSubmitRemoteStatus(&sheinRemoteStatusRequest{
		otherAPI: stubSheinOtherAPI{
			batchCheckOnWayResp: &sheinother.BatchCheckOnWayResponse{
				Code: "0",
				Msg:  "OK",
				Info: []struct {
					SpuName    string `json:"spu_name"`
					SkcName    string `json:"skc_name"`
					DocumentSn string `json:"document_sn"`
				}{
					{
						SpuName:    "SPU-PUBLISH",
						SkcName:    "SKC-PUBLISH",
						DocumentSn: "SPMPA4202605293776059",
					},
				},
			},
		},
		action:           "publish",
		requestID:        "request-on-way-123",
		lookupCodes:      []string{"SUP-1"},
		spuName:          "SPU-PUBLISH",
		defaultConfirmed: false,
		fallbackMessage:  "refreshing SHEIN remote record",
		startedAt:        time.Now(),
		taskID:           "task-on-way-123",
	})
	if err != nil {
		t.Fatalf("resolveSheinSubmitRemoteStatus() error = %v", err)
	}
	if confirmation == nil {
		t.Fatal("expected remote confirmation")
	}
	if confirmation.RemoteStatus != sheinpub.SubmissionRemoteStatusConfirmed {
		t.Fatalf("remote status = %q, want confirmed", confirmation.RemoteStatus)
	}
	if !strings.Contains(confirmation.Message, "SPMPA4202605293776059") {
		t.Fatalf("remote message = %q, want document_sn detail", confirmation.Message)
	}
	if confirmation.Event == nil || confirmation.Event.Phase != sheinpub.SubmissionPhaseConfirmRemote {
		t.Fatalf("event = %+v, want confirm_remote event", confirmation.Event)
	}
	if got := atomic.LoadInt32(&recordCalls); got != 0 {
		t.Fatalf("record calls = %d, want 0 when on-way document confirms publish", got)
	}
}

func TestSubmitTaskRecoversRemoteSubmitAfterFinalSaveFailure(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{failSaveWhenCurrentPhaseCleared: true}
	task := makeReadySheinTask()
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}
	var publishCalls int32
	var recordCalls int32
	svc, err := NewService(newTestServiceConfig(
		repo,
		withTestSheinProductAPIBuilder(stubSheinProductAPIBuilder{
			api: stubSheinProductAPI{
				publishHook: func(product *sheinproduct.Product) {
					atomic.AddInt32(&publishCalls, 1)
				},
				publishResponse: &sheinproduct.SheinResponse{
					Code: "0",
					Msg:  "success",
					Info: sheinproduct.ResponseInfo{Success: true, SPUName: "SPU-123"},
				},
				recordHook: func(*sheinproduct.ProductRecordRequest) {
					atomic.AddInt32(&recordCalls, 1)
				},
				recordResponse: makeSheinRecordResponse(sheinproduct.RecordItem{
					RecordID:     "record-recovered",
					SupplierCode: "SUP-submit-task-1",
				}),
			},
		}),
		withDefaultTestSheinImageAPI(),
	))
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	_, firstErr := svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{Platform: "shein", Action: "publish", IdempotencyKey: "recover-123"})
	if firstErr == nil || !strings.Contains(firstErr.Error(), "save task result failed") {
		t.Fatalf("first submit err = %v, want save failure", firstErr)
	}
	savedAfterFailure, err := repo.GetTask(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("get task after failed save: %v", err)
	}
	if savedAfterFailure.Result == nil || savedAfterFailure.Result.Shein == nil || savedAfterFailure.Result.Shein.Submission == nil {
		t.Fatalf("saved result after failed save = %+v", savedAfterFailure.Result)
	}
	publishRecord := savedAfterFailure.Result.Shein.Submission.Publish
	if publishRecord == nil {
		t.Fatalf("publish record after failed save = %+v", savedAfterFailure.Result.Shein.Submission)
	}
	if publishRecord.SupplierCode == "" {
		t.Fatalf("publish record supplier_code = %q, want persisted recovery code", publishRecord.SupplierCode)
	}
	if publishRecord.SubmitSnapshot == nil {
		t.Fatalf("publish record submit snapshot = %+v, want persisted snapshot", publishRecord)
	}
	preview, err := svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{Platform: "shein", Action: "publish", IdempotencyKey: "recover-123"})
	if err != nil {
		t.Fatalf("recovery submit: %v", err)
	}

	if got := atomic.LoadInt32(&publishCalls); got != 1 {
		t.Fatalf("publish calls = %d, want 1", got)
	}
	if got := atomic.LoadInt32(&recordCalls); got != 0 {
		t.Fatalf("record calls = %d, want 0 when local success can be recovered without remote lookup", got)
	}
	if preview.Shein.Submission.RemoteStatus != "" {
		t.Fatalf("remote status = %q, want empty for local recovery", preview.Shein.Submission.RemoteStatus)
	}
	if preview.Shein.Submission.CurrentPhase != "" {
		t.Fatalf("current phase = %q, want cleared", preview.Shein.Submission.CurrentPhase)
	}
}

func TestSubmitTaskRecoversRemoteSubmitAfterPersistResultSaveFailure(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{failSaveOnCurrentPhase: sheinpub.SubmissionPhasePersistResult}
	task := makeReadySheinTask()
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}
	var publishCalls int32
	var recordCalls int32
	svc, err := NewService(newTestServiceConfig(
		repo,
		withTestSheinProductAPIBuilder(stubSheinProductAPIBuilder{
			api: stubSheinProductAPI{
				publishHook: func(product *sheinproduct.Product) {
					atomic.AddInt32(&publishCalls, 1)
				},
				publishResponse: &sheinproduct.SheinResponse{
					Code: "0",
					Msg:  "success",
					Info: sheinproduct.ResponseInfo{Success: true, SPUName: "SPU-123"},
				},
				recordHook: func(*sheinproduct.ProductRecordRequest) {
					atomic.AddInt32(&recordCalls, 1)
				},
				recordResponse: makeSheinRecordResponse(sheinproduct.RecordItem{
					RecordID:     "record-persist-result",
					SupplierCode: "SUP-submit-task-1",
				}),
			},
		}),
		withDefaultTestSheinImageAPI(),
	))
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	_, firstErr := svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{Platform: "shein", Action: "publish", IdempotencyKey: "recover-persist-123"})
	if firstErr == nil || !strings.Contains(firstErr.Error(), "save task result failed") {
		t.Fatalf("first submit err = %v, want save failure", firstErr)
	}
	savedAfterFailure, err := repo.GetTask(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("get task after failed save: %v", err)
	}
	if savedAfterFailure.Result == nil || savedAfterFailure.Result.Shein == nil || savedAfterFailure.Result.Shein.Submission == nil {
		t.Fatalf("saved result after failed save = %+v", savedAfterFailure.Result)
	}
	if savedAfterFailure.Result.Shein.Submission.Publish == nil || savedAfterFailure.Result.Shein.Submission.Publish.Result == nil {
		t.Fatalf("publish result after failed save = %+v, want persisted remote response", savedAfterFailure.Result.Shein.Submission.Publish)
	}

	preview, err := svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{Platform: "shein", Action: "publish", IdempotencyKey: "recover-persist-123"})
	if err != nil {
		t.Fatalf("recovery submit: %v", err)
	}
	if got := atomic.LoadInt32(&publishCalls); got != 1 {
		t.Fatalf("publish calls = %d, want 1", got)
	}
	if got := atomic.LoadInt32(&recordCalls); got != 0 {
		t.Fatalf("record calls = %d, want 0 when persisted result is already available", got)
	}
	if preview.Shein.Submission.RemoteStatus != "" {
		t.Fatalf("remote status = %q, want empty for local recovery", preview.Shein.Submission.RemoteStatus)
	}
	if preview.Shein.Submission.CurrentPhase != "" {
		t.Fatalf("current phase = %q, want cleared", preview.Shein.Submission.CurrentPhase)
	}
}

func TestSubmitTaskReplaysCompletedIdempotencyKeyWithoutPublishingAgain(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}
	publishCalls := 0
	svc, err := NewService(newTestServiceConfig(
		repo,
		withTestSheinProductAPIBuilder(stubSheinProductAPIBuilder{
			api: stubSheinProductAPI{
				publishHook: func(product *sheinproduct.Product) {
					publishCalls++
				},
				publishResponse: &sheinproduct.SheinResponse{
					Code: "0",
					Msg:  "success",
					Info: sheinproduct.ResponseInfo{Success: true, SPUName: "SPU-123"},
				},
			},
		}),
		withDefaultTestSheinImageAPI(),
	))
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	for i := 0; i < 2; i++ {
		if _, err := svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{Platform: "shein", Action: "publish", IdempotencyKey: "replay-123"}); err != nil {
			t.Fatalf("submit task %d: %v", i+1, err)
		}
	}

	if publishCalls != 1 {
		t.Fatalf("publish calls = %d, want 1", publishCalls)
	}
	saved, err := repo.GetTask(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if saved.Result.Shein.Submission.AttemptCount != 1 {
		t.Fatalf("attempt count = %d, want 1", saved.Result.Shein.Submission.AttemptCount)
	}
}

func TestSubmitTaskReturnsCurrentPreviewForSameInFlightIdempotencyKey(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	startedAt := time.Now().Add(-time.Minute)
	beginSheinSubmitAttempt(task.Result.Shein, "publish", "in-flight-123", sheinpub.SubmissionPhaseSubmitRemote, startedAt)
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}
	publishCalls := 0
	svc, err := NewService(newTestServiceConfig(
		repo,
		withTestSheinProductAPIBuilder(stubSheinProductAPIBuilder{
			api: stubSheinProductAPI{
				publishHook: func(product *sheinproduct.Product) {
					publishCalls++
				},
				publishResponse: &sheinproduct.SheinResponse{
					Code: "0",
					Msg:  "success",
					Info: sheinproduct.ResponseInfo{Success: true, SPUName: "SPU-123"},
				},
			},
		}),
		withDefaultTestSheinImageAPI(),
	))
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	preview, err := svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{Platform: "shein", Action: "publish", IdempotencyKey: "in-flight-123"})
	if err != nil {
		t.Fatalf("submit task: %v", err)
	}

	if publishCalls != 0 {
		t.Fatalf("publish calls = %d, want 0", publishCalls)
	}
	if preview.Shein == nil || preview.Shein.Submission == nil || preview.Shein.Submission.CurrentPhase != sheinpub.SubmissionPhaseSubmitRemote {
		t.Fatalf("preview submission = %+v, want current submit_remote phase", preview.Shein)
	}
}

func TestSubmitTaskBlocksDifferentIdempotencyKeyWhileSubmitInFlight(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	startedAt := time.Now().Add(-time.Minute)
	beginSheinSubmitAttempt(task.Result.Shein, "publish", "in-flight-123", sheinpub.SubmissionPhaseSubmitRemote, startedAt)
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}
	publishCalls := 0
	svc, err := NewService(newTestServiceConfig(
		repo,
		withTestSheinProductAPIBuilder(stubSheinProductAPIBuilder{
			api: stubSheinProductAPI{
				publishHook: func(product *sheinproduct.Product) {
					publishCalls++
				},
				publishResponse: &sheinproduct.SheinResponse{
					Code: "0",
					Msg:  "success",
					Info: sheinproduct.ResponseInfo{Success: true, SPUName: "SPU-123"},
				},
			},
		}),
		withDefaultTestSheinImageAPI(),
	))
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	_, err = svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{Platform: "shein", Action: "publish", IdempotencyKey: "different-123"})

	if !errors.Is(err, core.ErrSubmitInProgress) {
		t.Fatalf("submit err = %v, want core.ErrSubmitInProgress", err)
	}
	if publishCalls != 0 {
		t.Fatalf("publish calls = %d, want 0", publishCalls)
	}
}

func TestSubmitTaskRoutesSheinPublishToTemporalWhenEnabled(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}
	workflowClient := &stubSheinPublishWorkflowClient{}
	publishCalls := 0
	svc, err := NewService(newTestServiceConfig(
		repo,
		withTestSheinPublishWorkflow(workflowClient, true),
		withTestSheinProductAPIBuilder(stubSheinProductAPIBuilder{api: stubSheinProductAPI{publishHook: func(product *sheinproduct.Product) { publishCalls++ }}}),
		withDefaultTestSheinImageAPI(),
	))
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	preview, err := svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{Platform: "shein", Action: "publish", IdempotencyKey: "temporal-publish-123"})
	if err != nil {
		t.Fatalf("submit task: %v", err)
	}

	if preview == nil || preview.TaskID != task.ID {
		t.Fatalf("preview = %+v, want task preview", preview)
	}
	if workflowClient.startCalls != 1 {
		t.Fatalf("workflow start calls = %d, want 1", workflowClient.startCalls)
	}
	if workflowClient.lastStart.TaskID != task.ID || workflowClient.lastStart.Action != "publish" || workflowClient.lastStart.RequestID != "temporal-publish-123" {
		t.Fatalf("workflow start input = %+v, want shein publish request", workflowClient.lastStart)
	}
	if publishCalls != 0 {
		t.Fatalf("publish calls = %d, want inline publish skipped", publishCalls)
	}
}

func TestSubmitTaskRoutesConfirmedFinalToTemporalWhenEnabled(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}
	workflowClient := &stubSheinPublishWorkflowClient{}
	svc, err := NewService(newTestServiceConfig(
		repo,
		withTestSheinPublishWorkflow(workflowClient, true),
	))
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	if _, err := svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{
		Platform:       "shein",
		Action:         "publish",
		IdempotencyKey: "temporal-final-123",
		ConfirmedFinal: true,
	}); err != nil {
		t.Fatalf("submit task: %v", err)
	}

	if !workflowClient.lastStart.ConfirmedFinal {
		t.Fatalf("workflow start input = %+v, want ConfirmedFinal=true", workflowClient.lastStart)
	}
}

func TestSubmitTaskTemporalReplayReturnsCurrentPreviewWithoutRestartingWorkflow(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	now := time.Now().Add(-time.Minute)
	record := sheinpub.CompleteSubmitAttemptAt(task.Result.Shein, "publish", "temporal-replay-123", &sheinpub.SubmissionResponse{
		Code:    "0",
		Message: "success",
		Success: true,
		SPUName: "SPU-123",
	}, nil, now)
	sheinpub.AppendSubmissionEvent(task.Result.Shein, sheinpub.BuildSubmissionAttemptEvent(task.ID, "publish", record, record.Result, nil, record.StartedAt))
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}
	workflowClient := &stubSheinPublishWorkflowClient{}
	svc, err := NewService(newTestServiceConfig(
		repo,
		withTestSheinPublishWorkflow(workflowClient, true),
	))
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	preview, err := svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{
		Platform:       "shein",
		Action:         "publish",
		IdempotencyKey: "temporal-replay-123",
	})
	if err != nil {
		t.Fatalf("submit task: %v", err)
	}

	if workflowClient.startCalls != 0 {
		t.Fatalf("workflow start calls = %d, want replay handled locally", workflowClient.startCalls)
	}
	if preview == nil || preview.Shein == nil || preview.Shein.Submission == nil || preview.Shein.Submission.Publish == nil {
		t.Fatalf("preview = %+v, want existing publish record", preview)
	}
	if preview.Shein.Submission.Publish.RequestID != "temporal-replay-123" {
		t.Fatalf("publish request id = %q, want temporal-replay-123", preview.Shein.Submission.Publish.RequestID)
	}
}

func TestSubmitTaskTemporalReplayReturnsPreviewDuringPendingWorkflowStart(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}
	workflowClient := &stubSheinPublishWorkflowClient{}
	svc, err := NewService(newTestServiceConfig(
		repo,
		withTestSheinPublishWorkflow(workflowClient, true),
	))
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	for i := 0; i < 2; i++ {
		if _, err := svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{
			Platform:       "shein",
			Action:         "publish",
			IdempotencyKey: "temporal-pending-123",
		}); err != nil {
			t.Fatalf("submit task %d: %v", i+1, err)
		}
	}

	if workflowClient.startCalls != 1 {
		t.Fatalf("workflow start calls = %d, want 1 while start is pending", workflowClient.startCalls)
	}
}

func TestSubmitTaskKeepsSheinSaveDraftInlineWhenTemporalEnabled(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}
	workflowClient := &stubSheinPublishWorkflowClient{}
	saveCalls := 0
	svc, err := NewService(newTestServiceConfig(
		repo,
		withTestSheinPublishWorkflow(workflowClient, true),
		withTestSheinProductAPIBuilder(stubSheinProductAPIBuilder{
			api: stubSheinProductAPI{
				saveHook: func(product *sheinproduct.Product) { saveCalls++ },
				saveResponse: &sheinproduct.SheinResponse{
					Code: "0",
					Msg:  "success",
					Info: sheinproduct.ResponseInfo{Success: true, SPUName: "SPU-DRAFT"},
				},
			},
		}),
		withDefaultTestSheinImageAPI(),
	))
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	if _, err := svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{Platform: "shein", Action: "save_draft", IdempotencyKey: "draft-123"}); err != nil {
		t.Fatalf("submit task: %v", err)
	}

	if workflowClient.startCalls != 0 {
		t.Fatalf("workflow start calls = %d, want save_draft inline", workflowClient.startCalls)
	}
	if saveCalls != 1 {
		t.Fatalf("save draft calls = %d, want 1", saveCalls)
	}
}

func TestSubmitTaskMapsTemporalRepeatedPublishToSubmitInProgress(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}
	workflowClient := &stubSheinPublishWorkflowClient{startErr: core.ErrSubmitInProgress}
	svc, err := NewService(newTestServiceConfig(
		repo,
		withTestSheinPublishWorkflow(workflowClient, true),
	))
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	_, err = svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{Platform: "shein", Action: "publish", IdempotencyKey: "repeat-123"})

	if !errors.Is(err, core.ErrSubmitInProgress) {
		t.Fatalf("submit err = %v, want core.ErrSubmitInProgress", err)
	}
	if workflowClient.startCalls != 1 {
		t.Fatalf("workflow start calls = %d, want 1", workflowClient.startCalls)
	}
}

func TestSubmitTaskBlocksDifferentTemporalRequestWhileWorkflowStartPending(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}
	workflowClient := &stubSheinPublishWorkflowClient{}
	svc, err := NewService(newTestServiceConfig(
		repo,
		withTestSheinPublishWorkflow(workflowClient, true),
	))
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	if _, err := svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{
		Platform:       "shein",
		Action:         "publish",
		IdempotencyKey: "temporal-pending-a",
	}); err != nil {
		t.Fatalf("submit task first: %v", err)
	}

	_, err = svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{
		Platform:       "shein",
		Action:         "publish",
		IdempotencyKey: "temporal-pending-b",
	})

	if !errors.Is(err, core.ErrSubmitInProgress) {
		t.Fatalf("submit err = %v, want core.ErrSubmitInProgress", err)
	}
	if workflowClient.startCalls != 1 {
		t.Fatalf("workflow start calls = %d, want 1 while first start is pending", workflowClient.startCalls)
	}
}

func TestSubmitTaskMarksTemporalStartFailureAsFailedInsteadOfLeavingRunningAttempt(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}
	workflowClient := &stubSheinPublishWorkflowClient{startErr: errors.New("temporal unavailable")}
	svc, err := NewService(newTestServiceConfig(
		repo,
		withTestSheinPublishWorkflow(workflowClient, true),
	))
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	_, err = svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{
		Platform:       "shein",
		Action:         "publish",
		IdempotencyKey: "temporal-start-fail-123",
	})
	if err == nil || !strings.Contains(err.Error(), "temporal unavailable") {
		t.Fatalf("submit err = %v, want temporal unavailable", err)
	}

	saved, err := repo.GetTask(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if saved.Result == nil || saved.Result.Shein == nil || saved.Result.Shein.Submission == nil || saved.Result.Shein.Submission.Publish == nil {
		t.Fatalf("saved result = %+v, want failed publish record", saved.Result)
	}
	record := saved.Result.Shein.Submission.Publish
	if record.Status != sheinpub.SubmissionStatusFailed {
		t.Fatalf("publish status = %q, want failed", record.Status)
	}
	if record.FinishedAt == nil {
		t.Fatalf("publish record = %+v, want finished_at recorded on start failure", record)
	}
	if record.StartedAt.IsZero() || !record.SubmittedAt.Equal(record.StartedAt) {
		t.Fatalf("publish timing = %+v, want submitted_at preserved as start time", record)
	}
	if saved.Result.Shein.Submission.CurrentAction != "" || saved.Result.Shein.Submission.CurrentPhase != "" || saved.Result.Shein.Submission.CurrentRequestID != "" {
		t.Fatalf("submission current state = %+v, want cleared after start failure", saved.Result.Shein.Submission)
	}
}

func TestSubmitTaskClearsTemporalLeaseWhenPersistingStartFailureFails(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{failSaveWhenCurrentPhaseCleared: true}
	task := makeReadySheinTask()
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}
	workflowClient := &stubSheinPublishWorkflowClient{startErr: errors.New("temporal unavailable")}
	svc, err := NewService(newTestServiceConfig(
		repo,
		withTestSheinPublishWorkflow(workflowClient, true),
	))
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	_, err = svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{
		Platform:       "shein",
		Action:         "publish",
		IdempotencyKey: "temporal-start-fail-save-error",
	})
	if err == nil || !strings.Contains(err.Error(), "save task result failed") {
		t.Fatalf("submit err = %v, want save task result failed", err)
	}

	saved, err := repo.GetTask(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if saved.Result == nil || saved.Result.Shein == nil || saved.Result.Shein.Submission == nil {
		t.Fatalf("saved result = %+v, want submission state", saved.Result)
	}
	if saved.Result.Shein.Submission.CurrentAction != "" || saved.Result.Shein.Submission.CurrentPhase != "" || saved.Result.Shein.Submission.CurrentRequestID != "" {
		t.Fatalf("submission current state = %+v, want cleared even when persisting failure fails", saved.Result.Shein.Submission)
	}
	if saved.Result.Shein.Submission.Publish == nil || saved.Result.Shein.Submission.Publish.Status != sheinpub.SubmissionStatusFailed {
		t.Fatalf("publish record = %+v, want failed fallback record", saved.Result.Shein.Submission.Publish)
	}
	if saved.Result.Shein.Submission.LastStatus != sheinpub.SubmissionStatusFailed || !strings.Contains(saved.Result.Shein.Submission.LastError, "temporal unavailable") {
		t.Fatalf("submission summary = %+v, want failed summary after fallback cleanup", saved.Result.Shein.Submission)
	}
}

func TestSubmitTaskGeneratesTemporalRequestIDWhenMissing(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}
	workflowClient := &stubSheinPublishWorkflowClient{}
	svc, err := NewService(newTestServiceConfig(
		repo,
		withTestSheinPublishWorkflow(workflowClient, true),
	))
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	if _, err := svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{
		Platform: "shein",
		Action:   "publish",
	}); err != nil {
		t.Fatalf("submit task: %v", err)
	}

	if strings.TrimSpace(workflowClient.lastStart.RequestID) == "" {
		t.Fatalf("workflow start input = %+v, want generated non-empty request id", workflowClient.lastStart)
	}
	if !strings.HasPrefix(workflowClient.lastStart.RequestID, "temporal:"+task.ID+":publish:") {
		t.Fatalf("workflow start request id = %q, want temporal-derived audit id", workflowClient.lastStart.RequestID)
	}
}

func TestSubmitTaskAllowsNewAttemptWhenInFlightAttemptIsStale(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	startedAt := time.Now().Add(-sheinSubmitInFlightTTL - time.Minute)
	beginSheinSubmitAttempt(task.Result.Shein, "publish", "stale-123", sheinpub.SubmissionPhaseSubmitRemote, startedAt)
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}
	publishCalls := 0
	svc, err := NewService(newTestServiceConfig(
		repo,
		withTestSheinProductAPIBuilder(stubSheinProductAPIBuilder{
			api: stubSheinProductAPI{
				publishHook: func(product *sheinproduct.Product) {
					publishCalls++
				},
				publishResponse: &sheinproduct.SheinResponse{
					Code: "0",
					Msg:  "success",
					Info: sheinproduct.ResponseInfo{Success: true, SPUName: "SPU-123"},
				},
			},
		}),
		withDefaultTestSheinImageAPI(),
	))
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	if _, err := svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{Platform: "shein", Action: "publish", IdempotencyKey: "new-123"}); err != nil {
		t.Fatalf("submit task: %v", err)
	}

	if publishCalls != 1 {
		t.Fatalf("publish calls = %d, want 1", publishCalls)
	}
	saved, err := repo.GetTask(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if saved.Result.Shein.Submission.Publish == nil || saved.Result.Shein.Submission.Publish.RequestID != "new-123" {
		t.Fatalf("publish record = %+v, want new request id", saved.Result.Shein.Submission.Publish)
	}
}

func TestSubmitTaskPersistsSubmitRemotePhaseBeforePublishCall(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}
	svc, err := NewService(newTestServiceConfig(
		repo,
		withTestSheinProductAPIBuilder(stubSheinProductAPIBuilder{
			api: stubSheinProductAPI{
				publishHook: func(product *sheinproduct.Product) {
					if !repo.hasSavedSubmissionPhase(sheinpub.SubmissionPhaseSubmitRemote) {
						t.Fatalf("submit_remote phase was not persisted before publish call; saved phases = %+v", repo.savedSubmissionPhases)
					}
				},
				publishResponse: &sheinproduct.SheinResponse{
					Code: "0",
					Msg:  "success",
					Info: sheinproduct.ResponseInfo{Success: true, SPUName: "SPU-123"},
				},
			},
		}),
		withDefaultTestSheinImageAPI(),
	))
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	if _, err := svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{Platform: "shein", Action: "publish", IdempotencyKey: "phase-123"}); err != nil {
		t.Fatalf("submit task: %v", err)
	}
}

func TestSubmitTaskPersistsDirectSubmitPhasesInOrder(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}
	svc, err := NewService(newTestServiceConfig(
		repo,
		withTestSheinProductAPIBuilder(stubSheinProductAPIBuilder{
			api: stubSheinProductAPI{
				publishResponse: &sheinproduct.SheinResponse{
					Code: "0",
					Msg:  "success",
					Info: sheinproduct.ResponseInfo{Success: true, SPUName: "SPU-123"},
				},
			},
		}),
		withDefaultTestSheinImageAPI(),
	))
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	if _, err := svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{
		Platform:       "shein",
		Action:         "publish",
		IdempotencyKey: "phase-order-123",
	}); err != nil {
		t.Fatalf("submit task: %v", err)
	}

	assertSubmissionPhasesContainOrderedSubsequence(
		t,
		repo.savedSubmissionPhases,
		[]string{
			sheinpub.SubmissionPhasePrepareProduct,
			sheinpub.SubmissionPhasePreValidate,
			sheinpub.SubmissionPhaseSubmitRemote,
			sheinpub.SubmissionPhasePersistResult,
		},
	)
}

func TestSubmitTaskSerializesConcurrentSameIdempotencyKey(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}
	var publishCalls int32
	enteredPublish := make(chan struct{}, 2)
	releasePublish := make(chan struct{})
	svc, err := NewService(newTestServiceConfig(
		repo,
		withTestSheinProductAPIBuilder(stubSheinProductAPIBuilder{
			api: stubSheinProductAPI{
				publishHook: func(product *sheinproduct.Product) {
					atomic.AddInt32(&publishCalls, 1)
					enteredPublish <- struct{}{}
					<-releasePublish
				},
				publishResponse: &sheinproduct.SheinResponse{
					Code: "0",
					Msg:  "success",
					Info: sheinproduct.ResponseInfo{Success: true, SPUName: "SPU-123"},
				},
			},
		}),
		withDefaultTestSheinImageAPI(),
	))
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	errs := make(chan error, 2)
	go func() {
		_, err := svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{Platform: "shein", Action: "publish", IdempotencyKey: "concurrent-123"})
		errs <- err
	}()
	select {
	case <-enteredPublish:
	case <-time.After(time.Second):
		t.Fatal("first submit did not reach publish")
	}
	go func() {
		_, err := svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{Platform: "shein", Action: "publish", IdempotencyKey: "concurrent-123"})
		errs <- err
	}()
	time.Sleep(30 * time.Millisecond)
	close(releasePublish)
	for i := 0; i < 2; i++ {
		if err := <-errs; err != nil {
			t.Fatalf("submit %d error: %v", i+1, err)
		}
	}

	if got := atomic.LoadInt32(&publishCalls); got != 1 {
		t.Fatalf("publish calls = %d, want 1", got)
	}
}

func TestSubmitTaskBlocksConcurrentDifferentRequestAcrossServiceInstances(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}
	var publishCalls int32
	enteredPublish := make(chan struct{}, 1)
	releasePublish := make(chan struct{})
	productAPI := stubSheinProductAPI{
		publishHook: func(product *sheinproduct.Product) {
			atomic.AddInt32(&publishCalls, 1)
			enteredPublish <- struct{}{}
			<-releasePublish
		},
		publishResponse: &sheinproduct.SheinResponse{
			Code: "0",
			Msg:  "success",
			Info: sheinproduct.ResponseInfo{Success: true, SPUName: "SPU-123"},
		},
		recordResponse: makeSheinRecordResponse(),
	}
	newSvc := func() Service {
		svc, err := NewService(newTestServiceConfig(
			repo,
			withTestSheinProductAPIBuilder(stubSheinProductAPIBuilder{api: productAPI}),
			withDefaultTestSheinImageAPI(),
		))
		if err != nil {
			t.Fatalf("new service: %v", err)
		}
		return svc
	}
	svc1 := newSvc()
	svc2 := newSvc()
	errs := make(chan error, 2)
	go func() {
		_, err := svc1.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{Platform: "shein", Action: "publish", IdempotencyKey: "request-a"})
		errs <- err
	}()
	select {
	case <-enteredPublish:
	case <-time.After(time.Second):
		t.Fatal("first submit did not reach publish")
	}
	go func() {
		_, err := svc2.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{Platform: "shein", Action: "publish", IdempotencyKey: "request-b"})
		errs <- err
	}()
	var conflict error
	select {
	case conflict = <-errs:
	case <-time.After(time.Second):
		t.Fatal("second submit did not return")
	}
	if !errors.Is(conflict, core.ErrSubmitInProgress) {
		t.Fatalf("second submit err = %v, want core.ErrSubmitInProgress", conflict)
	}
	close(releasePublish)
	if err := <-errs; err != nil {
		t.Fatalf("first submit err: %v", err)
	}
	if got := atomic.LoadInt32(&publishCalls); got != 1 {
		t.Fatalf("publish calls = %d, want 1", got)
	}
}

func TestSubmitTaskMarksPublishPreValidationNotesAsFailed(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	svc, err := NewService(newTestServiceConfig(
		repo,
		withTestSheinProductAPIBuilder(stubSheinProductAPIBuilder{
			api: stubSheinProductAPI{
				publishResponse: &sheinproduct.SheinResponse{
					Code: "0",
					Msg:  "OK",
					Info: sheinproduct.ResponseInfo{
						Success: false,
						PreValidResult: []sheinproduct.PreValidResult{{
							Messages: []string{
								"数量: 类型下模板属性为必填项",
								"方形图必须有一个",
							},
						}},
					},
				},
			},
		}),
		withDefaultTestSheinImageAPI(),
	))
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	_, err = svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{Platform: "shein", Action: "publish"})
	if err == nil || !strings.Contains(err.Error(), "数量: 类型下模板属性为必填项") {
		t.Fatalf("submit err = %v, want pre-validation note", err)
	}
	saved, getErr := repo.GetTask(context.Background(), task.ID)
	if getErr != nil {
		t.Fatalf("get task: %v", getErr)
	}
	submission := saved.Result.Shein.Submission
	if submission == nil || submission.LastStatus != "failed" || !strings.Contains(submission.LastError, "方形图必须有一个") {
		t.Fatalf("submission = %+v", submission)
	}
	if submission.Publish == nil || submission.Publish.Result == nil || len(submission.Publish.Result.ValidationNotes) != 2 {
		t.Fatalf("publish result = %+v", submission.Publish)
	}
}

func TestSubmitTaskMarksSKCMultiTitleValidationFailuresAsFailed(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	svc, err := NewService(newTestServiceConfig(
		repo,
		withTestSheinProductAPIBuilder(stubSheinProductAPIBuilder{
			api: stubSheinProductAPI{
				publishResponse: &sheinproduct.SheinResponse{
					Code: "0",
					Msg:  "OK",
					Info: sheinproduct.ResponseInfo{
						Success: false,
						PreValidResult: []sheinproduct.PreValidResult{{
							Form: "skc_multi_title",
							SkcErrorMessageMap: map[string]sheinproduct.SkcErrorMessage{
								"0": {
									Messages: []string{"共1个其他语种存在敏感词，请前往修改，敏感词：ADA"},
								},
							},
						}},
					},
				},
			},
		}),
		withDefaultTestSheinImageAPI(),
	))
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	_, err = svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{Platform: "shein", Action: "publish"})
	if err == nil || !strings.Contains(err.Error(), "敏感词：ADA") {
		t.Fatalf("submit err = %v, want skc validation note", err)
	}
	saved, getErr := repo.GetTask(context.Background(), task.ID)
	if getErr != nil {
		t.Fatalf("get task: %v", getErr)
	}
	submission := saved.Result.Shein.Submission
	if submission == nil || submission.LastStatus != "failed" || !strings.Contains(submission.LastError, "敏感词：ADA") {
		t.Fatalf("submission = %+v", submission)
	}
	if submission.Publish == nil || submission.Publish.Result == nil || len(submission.Publish.Result.ValidationNotes) != 1 {
		t.Fatalf("publish result = %+v", submission.Publish)
	}
}

func TestSubmitTaskRetriesSensitiveWordValidationNotesBeforeFailing(t *testing.T) {
	restore := overrideSensitiveWordsConfigForTest(t)
	defer restore()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	task.Result.Shein.PreviewProduct.MultiLanguageNameList = []sheinproduct.LanguageContent{{Language: "en", Name: "Whimsy Door Curtain"}}
	task.Result.Shein.PreviewProduct.MultiLanguageDescList = []sheinproduct.LanguageContent{{Language: "en", Name: "Whimsy decor for a relaxed room."}}
	task.Result.Shein.PreviewProduct.SKCList[0].MultiLanguageName = sheinproduct.LanguageContent{Language: "en", Name: "whimsy white"}
	task.Result.Shein.PreviewProduct.SKCList[0].MultiLanguageNameList = []sheinproduct.LanguageContent{{Language: "en", Name: "whimsy white"}}
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	var publishCalls int
	var submitted []*sheinproduct.Product
	svc, err := NewService(newTestServiceConfig(
		repo,
		withTestSheinProductAPIBuilder(stubSheinProductAPIBuilder{
			api: stubSheinProductAPI{
				publishFunc: func(product *sheinproduct.Product) (*sheinproduct.SheinResponse, error) {
					publishCalls++
					submitted = append(submitted, product)
					if publishCalls == 1 {
						return &sheinproduct.SheinResponse{
							Code: "0",
							Msg:  "OK",
							Info: sheinproduct.ResponseInfo{
								Success: false,
								PreValidResult: []sheinproduct.PreValidResult{{
									Messages: []string{"敏感词：whimsy"},
								}},
							},
						}, nil
					}
					return &sheinproduct.SheinResponse{
						Code: "0",
						Msg:  "success",
						Info: sheinproduct.ResponseInfo{Success: true, SPUName: "SPU-456"},
					}, nil
				},
			},
		}),
		withDefaultTestSheinImageAPI(),
	))
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	preview, err := svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{Platform: "shein", Action: "publish"})
	if err != nil {
		t.Fatalf("submit task: %v", err)
	}
	if publishCalls != 2 {
		t.Fatalf("publish calls = %d, want 2", publishCalls)
	}
	if len(submitted) != 2 {
		t.Fatalf("submitted payload count = %d, want 2", len(submitted))
	}
	if strings.Contains(strings.ToLower(findSheinLanguageContent(submitted[1].MultiLanguageNameList, "en")), "whimsy") {
		t.Fatalf("retried product name still contains whimsy: %+v", submitted[1].MultiLanguageNameList)
	}
	if !repo.hasSavedSubmissionPhase(sheinpub.SubmissionPhaseSubmitRemote) {
		t.Fatalf("submit_remote phase was not persisted; saved phases = %+v", repo.savedSubmissionPhases)
	}
	if preview == nil || preview.Shein == nil || preview.Shein.Submission == nil || preview.Shein.Submission.LastStatus != "success" {
		t.Fatalf("preview submission = %+v", preview)
	}
	if preview.Shein.Submission.Publish == nil || preview.Shein.Submission.Publish.SubmitSnapshot == nil {
		t.Fatalf("publish snapshot = %+v", preview.Shein.Submission.Publish)
	}
	if snapshotName := localizedSubmitSnapshotText(preview.Shein.Submission.Publish.SubmitSnapshot.MultiLanguageNameList, "en"); strings.Contains(strings.ToLower(snapshotName), "whimsy") {
		t.Fatalf("submit snapshot still contains whimsy: %+v", preview.Shein.Submission.Publish.SubmitSnapshot)
	}
	if len(preview.Shein.SubmissionEvents) == 0 {
		t.Fatalf("submission events = %+v, want retry and completion events", preview.Shein.SubmissionEvents)
	}
	foundRetryEvent := false
	for _, event := range preview.Shein.SubmissionEvents {
		if event.Phase == sheinpub.SubmissionPhaseSubmitRemote && strings.Contains(event.Detail, "检测到敏感词") {
			foundRetryEvent = true
			break
		}
	}
	if !foundRetryEvent {
		t.Fatalf("submission events = %+v, want sensitive-word retry event", preview.Shein.SubmissionEvents)
	}
}

func TestRefreshSubmissionStatusFallsBackToRecordLookupForPublishSPU(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	now := time.Now().Add(-time.Hour)
	task.Result.Shein.Submission = &sheinpub.SubmissionReport{
		LastAction:  "publish",
		LastStatus:  sheinpub.SubmissionStatusSuccess,
		SubmittedAt: &now,
		LastResult: &sheinpub.SubmissionResponse{
			Code:    "0",
			Message: "OK",
			Success: true,
			SPUName: "SPU-PUBLISH",
		},
		Publish: &sheinpub.SubmissionRecord{
			Action:       "publish",
			Status:       sheinpub.SubmissionStatusSuccess,
			SubmittedAt:  now,
			RequestID:    "refresh-publish-spu",
			SupplierCode: "SKC-1",
			StartedAt:    now,
			FinishedAt:   &now,
			Result: &sheinpub.SubmissionResponse{
				Code:    "0",
				Message: "OK",
				Success: true,
				SPUName: "SPU-PUBLISH",
			},
		},
	}
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}
	var recordCalls int32
	svc, err := NewService(newTestServiceConfig(
		repo,
		withTestSheinProductAPIBuilder(stubSheinProductAPIBuilder{
			api: stubSheinProductAPI{
				recordHook: func(*sheinproduct.ProductRecordRequest) {
					atomic.AddInt32(&recordCalls, 1)
				},
				recordResponse: makeSheinRecordResponse(
					sheinproduct.RecordItem{
						RecordID:     "record-draft-old",
						SupplierCode: "SKC-1",
						SpuName:      "SPU-DRAFT",
						State:        0,
						AuditState:   1,
						CreateTime:   "2026-05-12 13:19:24",
					},
					sheinproduct.RecordItem{
						RecordID:     "record-publish-new",
						SupplierCode: "SKC-1",
						SpuName:      "SPU-PUBLISH",
						State:        14,
						AuditState:   4,
						CreateTime:   "2026-05-12 13:45:13",
					},
				),
			},
		}),
		withDefaultTestSheinImageAPI(),
	))
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	preview, err := svc.RefreshSubmissionStatus(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("refresh submission status: %v", err)
	}
	if preview.Shein.Submission.RemoteStatus != sheinpub.SubmissionRemoteStatusConfirmed {
		t.Fatalf("remote status = %q, want confirmed", preview.Shein.Submission.RemoteStatus)
	}
	if got := atomic.LoadInt32(&recordCalls); got != 1 {
		t.Fatalf("record calls = %d, want 1", got)
	}
	if preview.Shein.Submission.Publish.RemoteRecordID != "record-publish-new" {
		t.Fatalf("remote record id = %q, want record-publish-new", preview.Shein.Submission.Publish.RemoteRecordID)
	}
	if got := preview.Shein.Submission.Publish.RemoteMessage; !strings.Contains(got, "publish API reported success") {
		t.Fatalf("remote message = %q, want record-based confirmation message", got)
	}
	if repo.mutateCalls == 0 {
		t.Fatal("expected RefreshSubmissionStatus to persist through mutate task result")
	}
	if repo.saveCalls != 0 {
		t.Fatalf("save calls = %d, want 0 when transaction mutation is available", repo.saveCalls)
	}
}

func strPtr(value string) *string {
	return &value
}

func containsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

func assertSubmissionPhasesContainOrderedSubsequence(t *testing.T, savedPhases []string, want []string) {
	t.Helper()

	cursor := 0
	for _, phase := range savedPhases {
		if cursor < len(want) && phase == want[cursor] {
			cursor++
		}
	}
	if cursor != len(want) {
		t.Fatalf("saved submission phases = %+v, want ordered subsequence %+v", savedPhases, want)
	}
}
