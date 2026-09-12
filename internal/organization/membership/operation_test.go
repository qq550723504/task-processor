package membership

import "testing"

func TestOperationTransitionsNeverRedispatchUnknown(t *testing.T) {
	op := Operation{Kind: CommandRole, Phase: PhaseReady, Revision: 1}
	dispatched, err := transition(op, EventDispatch, "attempt-1")
	if err != nil || dispatched.Phase != PhaseDispatched || dispatched.DispatchID != "attempt-1" {
		t.Fatalf("dispatch=%+v %v", dispatched, err)
	}
	for _, event := range []OperationEvent{EventDispatch, EventReject, EventIdentityVerified, EventAdvanceGrant} {
		if _, err := transition(dispatched, event, "attempt-2"); err != ErrConflict {
			t.Fatalf("unknown accepted %s: %v", event, err)
		}
	}
	if _, err := transition(dispatched, EventAcknowledge, "attempt-2"); err != ErrConflict {
		t.Fatal("foreign dispatch acknowledged")
	}
	completed, err := transition(dispatched, EventAcknowledge, "attempt-1")
	if err != nil || completed.Phase != PhaseCompleted || completed.Revision != 3 {
		t.Fatalf("complete=%+v %v", completed, err)
	}
	if _, err := transition(completed, EventDispatch, "attempt-3"); err != ErrConflict {
		t.Fatal("completed redispatched")
	}
}

func TestInviteIdentityProofOnlyAdvancesOriginalReadyGrant(t *testing.T) {
	op := Operation{Kind: CommandInvite, Phase: PhaseReady, Step: StepUser, Revision: 1, TargetUserID: "fixed-user"}
	op, err := transition(op, EventDispatch, "create-user")
	if err != nil {
		t.Fatal(err)
	}
	op, err = transition(op, EventIdentityVerified, "")
	if err != nil || op.Phase != PhaseIdentityVerified {
		t.Fatal(err)
	}
	if op.Phase == PhaseCompleted {
		t.Fatal("identity proof completed invite")
	}
	op, err = transition(op, EventAdvanceGrant, "")
	if err != nil || op.Phase != PhaseReady || op.Step != StepGrant || op.TargetUserID != "fixed-user" {
		t.Fatalf("next=%+v %v", op, err)
	}
	op, err = transition(op, EventDispatch, "create-grant")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = transition(op, EventIdentityVerified, ""); err != ErrConflict {
		t.Fatal("grant resolved by identity proof")
	}
	if _, err = transition(op, EventReject, ""); err != ErrConflict {
		t.Fatal("unknown grant released")
	}
	op, err = transition(op, EventAcknowledge, "create-grant")
	if err != nil || op.Phase != PhaseCompleted {
		t.Fatalf("final=%+v %v", op, err)
	}
}

func TestPartialInviteCannotReleaseReservationByReject(t *testing.T) {
	for _, phase := range []OperationPhase{PhaseAcknowledged, PhaseIdentityVerified} {
		op := Operation{Kind: CommandInvite, Step: StepUser, Phase: phase, Revision: 3}
		if _, err := transition(op, EventReject, ""); err != ErrConflict {
			t.Fatal("partial released")
		}
		ready, err := transition(op, EventAdvanceGrant, "")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := transition(ready, EventReject, ""); err != ErrConflict {
			t.Fatal("ready grant abandoned user and released")
		}
	}
}

func TestInviteRetainsFirstStepAcknowledgment(t *testing.T) {
	op := Operation{Kind: CommandInvite, Step: StepUser, Phase: PhaseDispatched, DispatchID: "user-send", Revision: 2}
	op, err := Transition(op, OperationChange{Event: EventAcknowledge, DispatchID: "user-send", Acknowledgment: &Acknowledgment{ID: "fixed-user", At: "2026-09-12T00:00:00Z"}})
	if err != nil {
		t.Fatal(err)
	}
	op, err = Transition(op, OperationChange{Event: EventAdvanceGrant})
	if err != nil || op.UserAcknowledgment == nil || op.UserAcknowledgment.ID != "fixed-user" || op.Acknowledgment != nil {
		t.Fatalf("lost first-step ACK: %+v %v", op, err)
	}
}
