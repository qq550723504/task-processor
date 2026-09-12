package membership

import (
	"context"
	"errors"
	"net/mail"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
)

type HumanIdentity struct{ ID, OrganizationID, Username, Email, FirstName, LastName string }
type HumanReader interface {
	ReadHuman(context.Context, string, string) (HumanIdentity, error)
}

func normalizeInvitation(input *Invitation) (Invitation, error) {
	if input == nil {
		return Invitation{}, ErrInvalidRequest
	}
	value := *input
	value.Email = strings.ToLower(strings.TrimSpace(value.Email))
	value.FirstName = strings.TrimSpace(value.FirstName)
	value.LastName = strings.TrimSpace(value.LastName)
	address, err := mail.ParseAddress(value.Email)
	if err != nil || address.Address != value.Email || len(value.Email) > 200 || strings.HasSuffix(value.Email, "@phone.invalid") {
		return Invitation{}, ErrInvalidRequest
	}
	for _, text := range []string{value.FirstName, value.LastName} {
		if text == "" || len(text) > 200 || !utf8.ValidString(text) {
			return Invitation{}, ErrInvalidRequest
		}
		for _, r := range text {
			if unicode.IsControl(r) {
				return Invitation{}, ErrInvalidRequest
			}
		}
	}
	return value, nil
}

func matchingHuman(h HumanIdentity, op Operation) bool {
	return op.Invitation != nil && h.ID == op.TargetUserID && h.OrganizationID == op.Scope.OrganizationID && h.Username == op.Invitation.Email && strings.ToLower(h.Email) == op.Invitation.Email && h.FirstName == op.Invitation.FirstName && h.LastName == op.Invitation.LastName
}

func (c *Commands) ReadOperation(ctx context.Context, key string) (Operation, error) {
	ctx, identity, err := c.current(ctx)
	if err != nil {
		return Operation{}, err
	}
	id, err := uuid.Parse(key)
	if err != nil || id == uuid.Nil || id.String() != key {
		return Operation{}, ErrInvalidRequest
	}
	return c.store.Read(ctx, OperationScope{ProjectID: c.service.projectID, OrganizationID: identity.EffectiveOrganizationID, ActorID: identity.UserID}, key)
}

func (c *Commands) Verify(ctx context.Context, key string) (Operation, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	op, err := c.ReadOperation(ctx, key)
	if err != nil {
		return Operation{}, err
	}
	if op.Kind == CommandInvite && op.Step == StepUser && op.Phase == PhaseDispatched {
		reader, ok := c.writer.(HumanReader)
		if !ok {
			return op, ErrUnavailable
		}
		human, err := reader.ReadHuman(ctx, op.Scope.OrganizationID, op.TargetUserID)
		if err != nil || !matchingHuman(human, op) {
			return op, nil
		}
		op, err = c.store.Apply(ctx, op.Scope, op.Key, op.Revision, OperationChange{Event: EventIdentityVerified})
		if err != nil {
			return Operation{}, err
		}
	}
	return c.resume(ctx, op)
}

func (c *Commands) resumeInvite(ctx context.Context, op Operation) (Operation, error) {
	if op.Phase == PhaseDispatched || op.Phase == PhaseCompleted || op.Phase == PhaseRejected {
		return op, nil
	}
	reader, ok := c.writer.(HumanReader)
	if !ok {
		return op, ErrUnavailable
	}
	// At most two steps; each iteration reloads live authorization.
	for range 2 {
		refreshed, identity, err := c.current(ctx)
		if err != nil {
			return op, err
		}
		ctx = refreshed
		if identity.UserID != op.Scope.ActorID || identity.EffectiveOrganizationID != op.Scope.OrganizationID || c.service.projectID != op.Scope.ProjectID {
			return op, ErrPermission
		}
		if !c.service.assignable(op.Role) {
			return op, ErrPermission
		}
		human, readErr := reader.ReadHuman(ctx, op.Scope.OrganizationID, op.TargetUserID)
		if op.Step == StepUser && op.Phase == PhaseReady {
			if readErr == nil {
				rejected, err := c.store.Apply(ctx, op.Scope, op.Key, op.Revision, OperationChange{Event: EventReject})
				if err != nil {
					return op, err
				}
				return rejected, ErrConflict
			}
			if !errors.Is(readErr, ErrNotFound) {
				return op, ErrUnavailable
			}
		} else {
			if readErr != nil || !matchingHuman(human, op) {
				return op, ErrUnavailable
			}
			if op.Step == StepUser {
				op, err = c.store.Apply(ctx, op.Scope, op.Key, op.Revision, OperationChange{Event: EventAdvanceGrant})
				if err != nil {
					return Operation{}, err
				}
			}
		}
		if ctx.Err() != nil {
			return op, ErrUnavailable
		}
		if _, err := c.service.authorize(ctx, authz.PermissionWorkbenchOrganizationMemberManage); err != nil {
			return op, err
		}
		dispatchID := uuid.NewString()
		dispatched, err := c.store.Apply(ctx, op.Scope, op.Key, op.Revision, OperationChange{Event: EventDispatch, DispatchID: dispatchID})
		if err != nil {
			return op, err
		}
		ack, err := c.writer.Write(ctx, dispatched)
		if err != nil || !authidentity.IsBoundedIdentifier(ack.ID) || (op.Step == StepUser && ack.ID != op.TargetUserID) {
			return dispatched, nil
		}
		if _, err := time.Parse(time.RFC3339Nano, ack.At); err != nil {
			return dispatched, nil
		}
		op, err = c.store.Apply(ctx, op.Scope, op.Key, dispatched.Revision, OperationChange{Event: EventAcknowledge, DispatchID: dispatchID, Acknowledgment: &ack})
		if err != nil {
			return dispatched, nil
		}
		if op.Phase == PhaseCompleted {
			return op, nil
		}
	}
	return op, nil
}
