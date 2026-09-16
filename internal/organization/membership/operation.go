package membership

import (
	"context"
	"errors"
)

var ErrConflict = errors.New("membership operation conflicts with current state")

type CommandKind string
type OperationPhase string
type OperationStep string
type OperationEvent string

const (
	CommandInvite         CommandKind    = "invite"
	CommandRole           CommandKind    = "role"
	CommandRemove         CommandKind    = "remove"
	PhaseReady            OperationPhase = "ready"
	PhaseDispatched       OperationPhase = "dispatched"
	PhaseAcknowledged     OperationPhase = "acknowledged"
	PhaseIdentityVerified OperationPhase = "identity_verified"
	PhaseCompleted        OperationPhase = "completed"
	PhaseRejected         OperationPhase = "rejected"
	StepUser              OperationStep  = "create_user"
	StepGrant             OperationStep  = "create_authorization"
	StepRole              OperationStep  = "update_authorization"
	StepRemove            OperationStep  = "delete_authorization"
	EventDispatch         OperationEvent = "dispatch"
	EventAcknowledge      OperationEvent = "acknowledge"
	EventIdentityVerified OperationEvent = "identity_verified"
	EventAdvanceGrant     OperationEvent = "advance_grant"
	EventReject           OperationEvent = "reject"
)

type OperationScope struct {
	ProjectID      string `json:"projectId"`
	OrganizationID string `json:"organizationId"`
	ActorID        string `json:"actorId"`
}

type Invitation struct {
	Email     string `json:"email"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
}

// Operation contains protocol facts only. Member state remains provider-owned.
type Operation struct {
	Scope              OperationScope  `json:"scope"`
	Key                string          `json:"key"`
	Fingerprint        string          `json:"fingerprint"`
	Kind               CommandKind     `json:"kind"`
	TargetUserID       string          `json:"targetUserId"`
	AuthorizationID    string          `json:"authorizationId"`
	ExpectedVersion    string          `json:"expectedVersion"`
	Role               string          `json:"role"`
	Invitation         *Invitation     `json:"invitation,omitempty"`
	Step               OperationStep   `json:"step"`
	Phase              OperationPhase  `json:"phase"`
	Revision           int64           `json:"revision"`
	DispatchID         string          `json:"dispatchId"`
	UserEvidence       string          `json:"userEvidence"`
	Acknowledgment     *Acknowledgment `json:"acknowledgment,omitempty"`
	UserAcknowledgment *Acknowledgment `json:"userAcknowledgment,omitempty"`
}

type Acknowledgment struct {
	ID string `json:"id"`
	At string `json:"at"`
}

type OperationChange struct {
	Event          OperationEvent
	DispatchID     string
	Acknowledgment *Acknowledgment
}

// Each method is a short independent transaction. Begin atomically claims
// operation identity, targetUserID and (for invitation) normalized email.
// Apply compares revision and validates the phase transition before committing.
// A returned error never authorizes an HTTP send, including commit ambiguity.
type ReceiptStore interface {
	Begin(context.Context, Operation) (Operation, error)
	Read(context.Context, OperationScope, string) (Operation, error)
	Apply(context.Context, OperationScope, string, int64, OperationChange) (Operation, error)
}

func Transition(op Operation, change OperationChange) (Operation, error) {
	next, err := transition(op, change.Event, change.DispatchID)
	if err != nil {
		return Operation{}, err
	}
	if change.Event == EventAcknowledge {
		if change.Acknowledgment == nil {
			return Operation{}, ErrInvalidRequest
		}
		ack := *change.Acknowledgment
		next.Acknowledgment = &ack
		if op.Kind == CommandInvite && op.Step == StepUser {
			next.UserAcknowledgment = &ack
		}
	}
	return next, nil
}

func transition(op Operation, event OperationEvent, dispatchID string) (Operation, error) {
	switch event {
	case EventDispatch:
		if op.Phase != PhaseReady || dispatchID == "" {
			return Operation{}, ErrConflict
		}
		op.Phase = PhaseDispatched
		op.DispatchID = dispatchID
	case EventAcknowledge:
		if op.Phase != PhaseDispatched || dispatchID == "" || op.DispatchID != dispatchID {
			return Operation{}, ErrConflict
		}
		if op.Kind == CommandInvite && op.Step == StepUser {
			op.Phase = PhaseAcknowledged
			op.UserEvidence = "acknowledged"
		} else {
			op.Phase = PhaseCompleted
		}
	case EventIdentityVerified:
		if op.Kind != CommandInvite || op.Step != StepUser || op.Phase != PhaseDispatched {
			return Operation{}, ErrConflict
		}
		op.Phase = PhaseIdentityVerified
		op.UserEvidence = "identity_verified"
	case EventAdvanceGrant:
		if op.Kind != CommandInvite || op.Step != StepUser || (op.Phase != PhaseAcknowledged && op.Phase != PhaseIdentityVerified) {
			return Operation{}, ErrConflict
		}
		op.Step = StepGrant
		op.Phase = PhaseReady
		op.DispatchID = ""
		op.Acknowledgment = nil
	case EventReject:
		if op.Phase != PhaseReady || (op.Kind == CommandInvite && op.Step != StepUser) {
			return Operation{}, ErrConflict
		}
		op.Phase = PhaseRejected
	default:
		return Operation{}, ErrConflict
	}
	op.Revision++
	return op, nil
}
