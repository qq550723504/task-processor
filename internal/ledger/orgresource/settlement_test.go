package orgresource

import (
	"context"
	"errors"
	"testing"
)

func TestSettleReservationRejectsUntrustedCallerBeforeReplay(t *testing.T) {
	authorizer := &recordingSettlementAuthorizer{err: ErrForbidden}
	executor := &recordingSettlementExecutor{}
	service, err := NewSettlementService(executor, authorizer)
	if err != nil {
		t.Fatal(err)
	}

	_, err = service.Settle(context.Background(), validSettlementInput())
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("Settle() error = %v, want ErrForbidden", err)
	}
	if authorizer.calls != 1 || executor.replayCalls != 0 || executor.executeCalls != 0 {
		t.Fatalf("untrusted settlement reached persistence: auth=%d replay=%d execute=%d", authorizer.calls, executor.replayCalls, executor.executeCalls)
	}
}

func TestSettleReservationHasNoCallerControlledDecisionAndReplaysDurably(t *testing.T) {
	authorizer := &recordingSettlementAuthorizer{}
	executor := &recordingSettlementExecutor{replayFound: true, replayResult: SettlementResult{
		Snapshot: SettlementSnapshot{ReservationID: "reservation-a", Decision: SettlementCommit},
	}}
	service, err := NewSettlementService(executor, authorizer)
	if err != nil {
		t.Fatal(err)
	}

	result, err := service.Settle(context.Background(), validSettlementInput())
	if err != nil {
		t.Fatal(err)
	}
	if !result.Replayed || result.Snapshot.Decision != SettlementCommit {
		t.Fatalf("result = %#v", result)
	}
	if executor.replayCalls != 1 || executor.executeCalls != 0 || executor.replay.RequestFingerprint == "" {
		t.Fatalf("executor = %#v", executor)
	}
}

func TestSettleReservationPassesOnlyCanonicalIdentityToExecutor(t *testing.T) {
	executor := &recordingSettlementExecutor{}
	service, err := NewSettlementService(executor, &recordingSettlementAuthorizer{})
	if err != nil {
		t.Fatal(err)
	}
	input := validSettlementInput()
	input.OrganizationID = " org-a "
	input.OperationID = " settle-a "
	input.ReservationID = " reservation-a "

	if _, err := service.Settle(context.Background(), input); err != nil {
		t.Fatal(err)
	}
	got := executor.execution
	if got.OrganizationID != "org-a" || got.OperationID != "settle-a" || got.ReservationID != "reservation-a" || got.OperationType != OperationSettleReservation {
		t.Fatalf("execution = %#v", got)
	}
	if got.RequestFingerprint == "" || got.ActorID != "owner-reconciler" {
		t.Fatalf("execution = %#v", got)
	}
}

func validSettlementInput() SettlementInput {
	return SettlementInput{
		OrganizationID: "org-a", OperationID: "settle-a", ReservationID: "reservation-a",
		Principal: Principal{ID: "owner-reconciler", Kind: PrincipalTrustedProvisioning},
	}
}

type recordingSettlementAuthorizer struct {
	err   error
	calls int
}

func (a *recordingSettlementAuthorizer) AuthorizeSettlement(context.Context, Principal) error {
	a.calls++
	return a.err
}

type recordingSettlementExecutor struct {
	replay        SettlementReplay
	replayResult  SettlementResult
	replayFound   bool
	replayErr     error
	replayCalls   int
	execution     SettlementExecution
	executeResult SettlementResult
	executeErr    error
	executeCalls  int
}

func (e *recordingSettlementExecutor) ReplaySettlement(_ context.Context, replay SettlementReplay) (SettlementResult, bool, error) {
	e.replayCalls++
	e.replay = replay
	return e.replayResult, e.replayFound, e.replayErr
}

func (e *recordingSettlementExecutor) ExecuteSettlement(_ context.Context, execution SettlementExecution) (SettlementResult, error) {
	e.executeCalls++
	e.execution = execution
	return e.executeResult, e.executeErr
}
