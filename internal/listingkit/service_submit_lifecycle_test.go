package listingkit

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	sheinpub "task-processor/internal/publishing/shein"
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

	svc, err := NewService(&ServiceConfig{
		Repository:             repo,
		ProductService:         stubSubmitProductService{},
		SheinProductAPIBuilder: stubSheinProductAPIBuilder{},
	})
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

	svc, err := NewService(&ServiceConfig{
		Repository:     repo,
		ProductService: stubSubmitProductService{},
		SheinProductAPIBuilder: stubSheinProductAPIBuilder{
			msg: "store token missing",
		},
	})
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

	svc, err := NewService(&ServiceConfig{
		Repository:     repo,
		ProductService: stubSubmitProductService{},
		SheinProductAPIBuilder: stubSheinProductAPIBuilder{
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
		},
		SheinImageAPIBuilder: stubSheinImageAPIBuilder{api: &stubSheinImageAPI{}},
	})
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

func TestSubmitTaskRemembersSheinResolutionCacheAfterPublishSuccess(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}
	cacheStore := &submitResolutionCacheStore{}
	svc, err := NewService(&ServiceConfig{
		Repository:     repo,
		ProductService: stubSubmitProductService{},
		SheinCategoryResolver: sheinpub.NewCachedCategoryResolver(
			sheinpub.NewCategoryResolver(nil),
			cacheStore,
		),
		SheinAttributeResolver: sheinpub.NewCachedAttributeResolver(
			sheinpub.NewAttributeResolver(nil, nil),
			cacheStore,
		),
		SheinSaleAttributeResolver: sheinpub.NewCachedSaleAttributeResolver(
			sheinpub.NewSaleAttributeResolver(nil, nil),
			cacheStore,
		),
		SheinProductAPIBuilder: stubSheinProductAPIBuilder{
			api: stubSheinProductAPI{
				publishResponse: &sheinproduct.SheinResponse{
					Code: "0",
					Msg:  "success",
					Info: sheinproduct.ResponseInfo{Success: true, SPUName: "SPU-123"},
				},
			},
		},
		SheinImageAPIBuilder: stubSheinImageAPIBuilder{api: &stubSheinImageAPI{}},
	})
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
	entries := cacheStore.snapshot()
	if len(entries) != 3 {
		t.Fatalf("cache entry count = %d, want 3: %+v", len(entries), entries)
	}
	for _, entry := range entries {
		if entry.Source != "manual_cache" || !entry.Manual {
			t.Fatalf("cache entry = %+v, want manual_cache confirmed by publish", entry)
		}
	}
}

func TestSubmitTaskConfirmsRemoteRecordAfterPublishSuccess(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}
	var recordSupplierCodes []string
	svc, err := NewService(&ServiceConfig{
		Repository:     repo,
		ProductService: stubSubmitProductService{},
		SheinProductAPIBuilder: stubSheinProductAPIBuilder{
			api: stubSheinProductAPI{
				publishResponse: &sheinproduct.SheinResponse{
					Code: "0",
					Msg:  "success",
					Info: sheinproduct.ResponseInfo{Success: true, SPUName: "SPU-123"},
				},
				recordHook: func(request *sheinproduct.ProductRecordRequest) {
					if request.SupplierCodeList != nil {
						recordSupplierCodes = append(recordSupplierCodes, (*request.SupplierCodeList)...)
					}
				},
				recordResponse: makeSheinRecordResponse(sheinproduct.RecordItem{
					RecordID:     "record-123",
					SupplierCode: "SUP-submit-task-1",
					State:        2,
					AuditState:   3,
				}),
			},
		},
		SheinImageAPIBuilder: stubSheinImageAPIBuilder{api: &stubSheinImageAPI{}},
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	preview, err := svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{Platform: "shein", Action: "publish", IdempotencyKey: "remote-123"})
	if err != nil {
		t.Fatalf("submit task: %v", err)
	}

	if got := preview.Shein.Submission.RemoteStatus; got != sheinpub.SubmissionRemoteStatusConfirmed {
		t.Fatalf("remote status = %q, want confirmed", got)
	}
	if preview.Shein.Submission.Publish.RemoteRecordID != "record-123" {
		t.Fatalf("remote record id = %q, want record-123", preview.Shein.Submission.Publish.RemoteRecordID)
	}
	if len(recordSupplierCodes) != 1 || recordSupplierCodes[0] == "" {
		t.Fatalf("record supplier codes = %+v, want one supplier code", recordSupplierCodes)
	}
	if !repo.hasSavedSubmissionPhase(sheinpub.SubmissionPhaseConfirmRemote) {
		t.Fatalf("confirm_remote phase was not persisted; saved phases = %+v", repo.savedSubmissionPhases)
	}
}

func TestSubmitTaskMarksRemoteConfirmationPendingWhenRecordMissing(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}
	svc, err := NewService(&ServiceConfig{
		Repository:     repo,
		ProductService: stubSubmitProductService{},
		SheinProductAPIBuilder: stubSheinProductAPIBuilder{
			api: stubSheinProductAPI{
				publishResponse: &sheinproduct.SheinResponse{
					Code: "0",
					Msg:  "success",
					Info: sheinproduct.ResponseInfo{Success: true, SPUName: "SPU-123"},
				},
				recordResponse: makeSheinRecordResponse(),
			},
		},
		SheinImageAPIBuilder: stubSheinImageAPIBuilder{api: &stubSheinImageAPI{}},
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	preview, err := svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{Platform: "shein", Action: "publish", IdempotencyKey: "pending-123"})
	if err != nil {
		t.Fatalf("submit task: %v", err)
	}

	if got := preview.Shein.Submission.RemoteStatus; got != sheinpub.SubmissionRemoteStatusPending {
		t.Fatalf("remote status = %q, want pending", got)
	}
	if preview.Shein.Submission.LastStatus != sheinpub.SubmissionStatusSuccess {
		t.Fatalf("last status = %q, want success", preview.Shein.Submission.LastStatus)
	}
}

func TestSubmitTaskFailsWhenRemoteRecordShowsDraftState(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}
	svc, err := NewService(&ServiceConfig{
		Repository:     repo,
		ProductService: stubSubmitProductService{},
		SheinProductAPIBuilder: stubSheinProductAPIBuilder{
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
		},
		SheinImageAPIBuilder: stubSheinImageAPIBuilder{api: &stubSheinImageAPI{}},
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	_, err = svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{Platform: "shein", Action: "publish", IdempotencyKey: "draft-remote-123"})
	if err == nil || !strings.Contains(err.Error(), "landed in draft state") {
		t.Fatalf("submit err = %v, want draft state failure", err)
	}

	saved, err := repo.GetTask(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	submission := saved.Result.Shein.Submission
	if submission == nil {
		t.Fatal("expected submission report")
	}
	if submission.RemoteStatus != sheinpub.SubmissionRemoteStatusFailed {
		t.Fatalf("remote status = %q, want failed", submission.RemoteStatus)
	}
	if submission.LastStatus != sheinpub.SubmissionStatusFailed {
		t.Fatalf("last status = %q, want failed", submission.LastStatus)
	}
	if submission.Publish == nil || !strings.Contains(submission.Publish.RemoteMessage, "draft state") {
		t.Fatalf("publish record = %+v, want draft-state remote message", submission.Publish)
	}
}

func TestSubmitTaskMarksRemoteConfirmationPendingWhenRecordNotVisible(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}
	var recordCalls int32
	svc, err := NewService(&ServiceConfig{
		Repository:     repo,
		ProductService: stubSubmitProductService{},
		SheinProductAPIBuilder: stubSheinProductAPIBuilder{
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
		},
		SheinImageAPIBuilder: stubSheinImageAPIBuilder{api: &stubSheinImageAPI{}},
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	preview, err := svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{Platform: "shein", Action: "publish", IdempotencyKey: "confirm-only-123"})
	if err != nil {
		t.Fatalf("submit task: %v", err)
	}

	if got := preview.Shein.Submission.RemoteStatus; got != sheinpub.SubmissionRemoteStatusPending {
		t.Fatalf("remote status = %q, want pending", got)
	}
	if got := atomic.LoadInt32(&recordCalls); got != 1 {
		t.Fatalf("record calls = %d, want 1 record lookup", got)
	}
	if preview.Shein.Submission.Publish.RemoteRecordID != "" {
		t.Fatalf("remote record id = %q, want empty when record not visible", preview.Shein.Submission.Publish.RemoteRecordID)
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
	svc, err := NewService(&ServiceConfig{
		Repository:     repo,
		ProductService: stubSubmitProductService{},
		SheinProductAPIBuilder: stubSheinProductAPIBuilder{
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
		},
		SheinImageAPIBuilder: stubSheinImageAPIBuilder{api: &stubSheinImageAPI{}},
	})
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

func TestSubmitTaskRecoversRemoteSubmitAfterFinalSaveFailure(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{failSaveWhenCurrentPhaseCleared: true}
	task := makeReadySheinTask()
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}
	var publishCalls int32
	svc, err := NewService(&ServiceConfig{
		Repository:     repo,
		ProductService: stubSubmitProductService{},
		SheinProductAPIBuilder: stubSheinProductAPIBuilder{
			api: stubSheinProductAPI{
				publishHook: func(product *sheinproduct.Product) {
					atomic.AddInt32(&publishCalls, 1)
				},
				publishResponse: &sheinproduct.SheinResponse{
					Code: "0",
					Msg:  "success",
					Info: sheinproduct.ResponseInfo{Success: true, SPUName: "SPU-123"},
				},
				recordResponse: makeSheinRecordResponse(sheinproduct.RecordItem{
					RecordID:     "record-recovered",
					SupplierCode: "SUP-submit-task-1",
				}),
			},
		},
		SheinImageAPIBuilder: stubSheinImageAPIBuilder{api: &stubSheinImageAPI{}},
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	_, firstErr := svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{Platform: "shein", Action: "publish", IdempotencyKey: "recover-123"})
	if firstErr == nil || !strings.Contains(firstErr.Error(), "save task result failed") {
		t.Fatalf("first submit err = %v, want save failure", firstErr)
	}
	preview, err := svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{Platform: "shein", Action: "publish", IdempotencyKey: "recover-123"})
	if err != nil {
		t.Fatalf("recovery submit: %v", err)
	}

	if got := atomic.LoadInt32(&publishCalls); got != 1 {
		t.Fatalf("publish calls = %d, want 1", got)
	}
	if preview.Shein.Submission.RemoteStatus != sheinpub.SubmissionRemoteStatusPending {
		t.Fatalf("remote status = %q, want pending", preview.Shein.Submission.RemoteStatus)
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
	svc, err := NewService(&ServiceConfig{
		Repository:     repo,
		ProductService: stubSubmitProductService{},
		SheinProductAPIBuilder: stubSheinProductAPIBuilder{
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
		},
		SheinImageAPIBuilder: stubSheinImageAPIBuilder{api: &stubSheinImageAPI{}},
	})
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
	svc, err := NewService(&ServiceConfig{
		Repository:     repo,
		ProductService: stubSubmitProductService{},
		SheinProductAPIBuilder: stubSheinProductAPIBuilder{
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
		},
		SheinImageAPIBuilder: stubSheinImageAPIBuilder{api: &stubSheinImageAPI{}},
	})
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
	svc, err := NewService(&ServiceConfig{
		Repository:     repo,
		ProductService: stubSubmitProductService{},
		SheinProductAPIBuilder: stubSheinProductAPIBuilder{
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
		},
		SheinImageAPIBuilder: stubSheinImageAPIBuilder{api: &stubSheinImageAPI{}},
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	_, err = svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{Platform: "shein", Action: "publish", IdempotencyKey: "different-123"})

	if !errors.Is(err, ErrSubmitInProgress) {
		t.Fatalf("submit err = %v, want ErrSubmitInProgress", err)
	}
	if publishCalls != 0 {
		t.Fatalf("publish calls = %d, want 0", publishCalls)
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
	svc, err := NewService(&ServiceConfig{
		Repository:     repo,
		ProductService: stubSubmitProductService{},
		SheinProductAPIBuilder: stubSheinProductAPIBuilder{
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
		},
		SheinImageAPIBuilder: stubSheinImageAPIBuilder{api: &stubSheinImageAPI{}},
	})
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
	svc, err := NewService(&ServiceConfig{
		Repository:     repo,
		ProductService: stubSubmitProductService{},
		SheinProductAPIBuilder: stubSheinProductAPIBuilder{
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
		},
		SheinImageAPIBuilder: stubSheinImageAPIBuilder{api: &stubSheinImageAPI{}},
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	if _, err := svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{Platform: "shein", Action: "publish", IdempotencyKey: "phase-123"}); err != nil {
		t.Fatalf("submit task: %v", err)
	}
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
	svc, err := NewService(&ServiceConfig{
		Repository:     repo,
		ProductService: stubSubmitProductService{},
		SheinProductAPIBuilder: stubSheinProductAPIBuilder{
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
		},
		SheinImageAPIBuilder: stubSheinImageAPIBuilder{api: &stubSheinImageAPI{}},
	})
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
		svc, err := NewService(&ServiceConfig{
			Repository:             repo,
			ProductService:         stubSubmitProductService{},
			SheinProductAPIBuilder: stubSheinProductAPIBuilder{api: productAPI},
			SheinImageAPIBuilder:   stubSheinImageAPIBuilder{api: &stubSheinImageAPI{}},
		})
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
	if !errors.Is(conflict, ErrSubmitInProgress) {
		t.Fatalf("second submit err = %v, want ErrSubmitInProgress", conflict)
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

	svc, err := NewService(&ServiceConfig{
		Repository:     repo,
		ProductService: stubSubmitProductService{},
		SheinProductAPIBuilder: stubSheinProductAPIBuilder{
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
		},
		SheinImageAPIBuilder: stubSheinImageAPIBuilder{api: &stubSheinImageAPI{}},
	})
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
	svc, err := NewService(&ServiceConfig{
		Repository:     repo,
		ProductService: stubSubmitProductService{},
		SheinProductAPIBuilder: stubSheinProductAPIBuilder{
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
		},
		SheinImageAPIBuilder: stubSheinImageAPIBuilder{api: &stubSheinImageAPI{}},
	})
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
