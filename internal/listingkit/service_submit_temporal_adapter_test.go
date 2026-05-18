package listingkit

import (
	"context"
	"errors"
	"testing"
	"time"

	sheinpub "task-processor/internal/publishing/shein"
)

func TestSheinPublishActivityHostPreparePayloadPersistsPhase(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	svc, err := NewService(&ServiceConfig{
		Repository:     repo,
		ProductService: stubSubmitProductService{},
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	host, err := NewSheinPublishActivityHost(svc)
	if err != nil {
		t.Fatalf("new shein publish activity host: %v", err)
	}

	in := SheinPublishAttemptInput{
		TaskID:      task.ID,
		Action:      "publish",
		RequestID:   "temporal-host-prepare-123",
		RequestedAt: time.Now(),
	}
	if err := host.BeginSheinPublishAttempt(context.Background(), in); err != nil {
		t.Fatalf("begin shein publish attempt: %v", err)
	}

	payload, err := host.PrepareSheinPublishPayload(context.Background(), in)
	if err != nil {
		t.Fatalf("prepare shein publish payload: %v", err)
	}
	if payload == nil || payload.Product == nil {
		t.Fatalf("prepared payload = %+v, want product payload", payload)
	}
	if payload.RequestID != in.RequestID {
		t.Fatalf("prepared payload request id = %q, want %q", payload.RequestID, in.RequestID)
	}
	if payload.Snapshot == nil || len(payload.Snapshot.MultiLanguageNameList) == 0 {
		t.Fatalf("prepared snapshot = %+v, want populated submit snapshot", payload.Snapshot)
	}

	saved, err := repo.GetTask(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if saved.Result == nil || saved.Result.Shein == nil || saved.Result.Shein.Submission == nil {
		t.Fatalf("saved result = %+v, want shein submission state", saved.Result)
	}
	if got := saved.Result.Shein.Submission.CurrentPhase; got != sheinpub.SubmissionPhasePrepareProduct {
		t.Fatalf("current phase = %q, want %q", got, sheinpub.SubmissionPhasePrepareProduct)
	}
	if got := saved.Result.Shein.Submission.CurrentRequestID; got != in.RequestID {
		t.Fatalf("current request id = %q, want %q", got, in.RequestID)
	}
}

func TestSheinPublishActivityHostValidateReadinessReturnsBlockedError(t *testing.T) {
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
		Repository:     repo,
		ProductService: stubSubmitProductService{},
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	host, err := NewSheinPublishActivityHost(svc)
	if err != nil {
		t.Fatalf("new shein publish activity host: %v", err)
	}

	in := SheinPublishAttemptInput{
		TaskID:      task.ID,
		Action:      "publish",
		RequestID:   "temporal-host-blocked-123",
		RequestedAt: time.Now(),
	}
	if err := host.BeginSheinPublishAttempt(context.Background(), in); err != nil {
		t.Fatalf("begin shein publish attempt: %v", err)
	}

	err = host.ValidateSheinPublishReadiness(context.Background(), in)
	if err == nil || !errors.Is(err, ErrSubmitBlocked) {
		t.Fatalf("validate shein publish readiness err = %v, want ErrSubmitBlocked", err)
	}

	saved, err := repo.GetTask(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if saved.Result.Shein.FinalDraft != nil && saved.Result.Shein.FinalDraft.Confirmed {
		t.Fatalf("final draft = %+v, want unchanged without ConfirmedFinal", saved.Result.Shein.FinalDraft)
	}
}

func TestSheinPublishActivityHostValidateReadinessPersistsConfirmedFinalOnBlockedError(t *testing.T) {
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
		Repository:     repo,
		ProductService: stubSubmitProductService{},
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	host, err := NewSheinPublishActivityHost(svc)
	if err != nil {
		t.Fatalf("new shein publish activity host: %v", err)
	}

	in := SheinPublishAttemptInput{
		TaskID:         task.ID,
		Action:         "publish",
		RequestID:      "temporal-host-blocked-final-123",
		ConfirmedFinal: true,
		RequestedAt:    time.Now(),
	}
	if err := host.BeginSheinPublishAttempt(context.Background(), in); err != nil {
		t.Fatalf("begin shein publish attempt: %v", err)
	}

	err = host.ValidateSheinPublishReadiness(context.Background(), in)
	if err == nil || !errors.Is(err, ErrSubmitBlocked) {
		t.Fatalf("validate shein publish readiness err = %v, want ErrSubmitBlocked", err)
	}

	saved, err := repo.GetTask(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if saved.Result.Shein.FinalDraft == nil || !saved.Result.Shein.FinalDraft.Confirmed {
		t.Fatalf("final draft = %+v, want confirmed final draft persisted", saved.Result.Shein.FinalDraft)
	}
}

func TestSheinPublishActivityHostPersistFailureUsesWorkflowPhaseAndRequestID(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	svc, err := NewService(&ServiceConfig{
		Repository:     repo,
		ProductService: stubSubmitProductService{},
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	host, err := NewSheinPublishActivityHost(svc)
	if err != nil {
		t.Fatalf("new shein publish activity host: %v", err)
	}

	in := SheinPersistSubmitFailureInput{
		TaskID:       task.ID,
		Action:       "publish",
		RequestID:    "temporal-host-failure-123",
		Phase:        sheinpub.SubmissionPhaseValidate,
		ErrorMessage: "validate failed before prepare",
	}
	if err := host.PersistSheinPublishFailure(context.Background(), in); err != nil {
		t.Fatalf("persist shein publish failure: %v", err)
	}

	saved, err := repo.GetTask(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if saved.Result == nil || saved.Result.Shein == nil || saved.Result.Shein.Submission == nil {
		t.Fatalf("saved result = %+v, want shein submission state", saved.Result)
	}
	record := saved.Result.Shein.Submission.Publish
	if record == nil {
		t.Fatalf("publish record = nil, want persisted failure record")
	}
	if record.RequestID != in.RequestID {
		t.Fatalf("publish request id = %q, want %q", record.RequestID, in.RequestID)
	}
	if record.Phase != in.Phase {
		t.Fatalf("publish phase = %q, want %q", record.Phase, in.Phase)
	}
	if record.Status != sheinpub.SubmissionStatusFailed {
		t.Fatalf("publish status = %q, want %q", record.Status, sheinpub.SubmissionStatusFailed)
	}
}
