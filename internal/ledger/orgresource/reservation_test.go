package orgresource

import (
	"context"
	"errors"
	"testing"
)

func TestReserveRejectsUntrustedOwnerBeforeReplayOrWrite(t *testing.T) {
	authorizer := &recordingReservationAuthorizer{err: ErrForbidden}
	executor := &recordingReservationExecutor{}
	service, err := NewReservationService(executor, authorizer)
	if err != nil {
		t.Fatal(err)
	}

	_, err = service.Reserve(context.Background(), validReserveInput())
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("Reserve() error = %v, want ErrForbidden", err)
	}
	if authorizer.calls != 1 || executor.replayCalls != 0 || executor.executeCalls != 0 {
		t.Fatalf("untrusted reserve reached persistence: auth=%d replay=%d execute=%d", authorizer.calls, executor.replayCalls, executor.executeCalls)
	}
}

func TestReserveBuildsOwnerScopedCommandAndReplaysBeforePolicyValidation(t *testing.T) {
	authorizer := &recordingReservationAuthorizer{authorization: ReservationAuthorization{MaxQuantity: 1}}
	executor := &recordingReservationExecutor{replayFound: true, replayResult: ReservationResult{
		Snapshot: ReservationSnapshot{ReservationID: "reservation-a", Quantity: "2"},
	}}
	service, err := NewReservationService(executor, authorizer)
	if err != nil {
		t.Fatal(err)
	}
	input := validReserveInput()
	input.Quantity = 2

	result, err := service.Reserve(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Replayed || result.Snapshot.ReservationID != "reservation-a" {
		t.Fatalf("result = %#v", result)
	}
	if executor.replayCalls != 1 || executor.executeCalls != 0 {
		t.Fatalf("calls: replay=%d execute=%d", executor.replayCalls, executor.executeCalls)
	}
	if executor.replay.RequestFingerprint == "" {
		t.Fatal("request fingerprint is empty")
	}
}

func TestReserveRejectsNewQuantityAboveRegisteredOwnerLimit(t *testing.T) {
	authorizer := &recordingReservationAuthorizer{authorization: ReservationAuthorization{MaxQuantity: 1}}
	executor := &recordingReservationExecutor{}
	service, err := NewReservationService(executor, authorizer)
	if err != nil {
		t.Fatal(err)
	}
	input := validReserveInput()
	input.Quantity = 2

	_, err = service.Reserve(context.Background(), input)
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("Reserve() error = %v, want ErrInvalidInput", err)
	}
	if executor.replayCalls != 1 || executor.executeCalls != 0 {
		t.Fatalf("calls: replay=%d execute=%d", executor.replayCalls, executor.executeCalls)
	}
}

func TestReservePassesCanonicalOwnerBindingToExecutor(t *testing.T) {
	authorizer := &recordingReservationAuthorizer{authorization: ReservationAuthorization{MaxQuantity: 10}}
	executor := &recordingReservationExecutor{executeResult: ReservationResult{Snapshot: ReservationSnapshot{ReservationID: "reservation-a"}}}
	service, err := NewReservationService(executor, authorizer)
	if err != nil {
		t.Fatal(err)
	}
	input := validReserveInput()
	input.OrganizationID = " org-a "
	input.BusinessScope = " listing-kit:task-a "

	if _, err := service.Reserve(context.Background(), input); err != nil {
		t.Fatal(err)
	}
	got := executor.execution
	if got.OrganizationID != "org-a" || got.OwnerType != "listingkit_generation" || got.OwnerAttemptID != "attempt-a" || got.BusinessScope != "listing-kit:task-a" {
		t.Fatalf("owner binding = %#v", got)
	}
	if got.ResourceType != ResourceAIPoint || got.Quantity != 1 || got.ReservationPurpose != "generation" {
		t.Fatalf("resource binding = %#v", got)
	}
	if got.OperationType != OperationReserve || got.RequestFingerprint == "" {
		t.Fatalf("operation = %#v", got)
	}
}

func validReserveInput() ReserveInput {
	return ReserveInput{
		OrganizationID:     "org-a",
		OperationID:        "operation-a",
		OwnerType:          "listingkit_generation",
		OwnerAttemptID:     "attempt-a",
		BusinessScope:      "listing-kit:task-a",
		ResourceType:       ResourceAIPoint,
		Quantity:           1,
		ReservationPurpose: "generation",
		Principal:          Principal{ID: "listingkit-worker", Kind: PrincipalTrustedProvisioning},
	}
}

type recordingReservationAuthorizer struct {
	authorization ReservationAuthorization
	err           error
	calls         int
}

func (a *recordingReservationAuthorizer) AuthorizeReservation(context.Context, Principal, string, ResourceType) (ReservationAuthorization, error) {
	a.calls++
	return a.authorization, a.err
}

type recordingReservationExecutor struct {
	replay        ReservationReplay
	replayResult  ReservationResult
	replayFound   bool
	replayErr     error
	replayCalls   int
	execution     ReservationExecution
	executeResult ReservationResult
	executeErr    error
	executeCalls  int
}

func (e *recordingReservationExecutor) ReplayReservation(_ context.Context, replay ReservationReplay) (ReservationResult, bool, error) {
	e.replayCalls++
	e.replay = replay
	return e.replayResult, e.replayFound, e.replayErr
}

func (e *recordingReservationExecutor) ExecuteReservation(_ context.Context, execution ReservationExecution) (ReservationResult, error) {
	e.executeCalls++
	e.execution = execution
	return e.executeResult, e.executeErr
}
