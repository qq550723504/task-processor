package membership

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"regexp"
	"time"

	"github.com/google/uuid"
	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
)

type CommandInput struct {
	Kind            CommandKind `json:"kind"`
	AuthorizationID string      `json:"authorizationId"`
	ExpectedVersion string      `json:"expectedVersion"`
	Role            string      `json:"role"`
	Invitation      *Invitation `json:"invitation,omitempty"`
}

type ProviderWriter interface {
	Write(context.Context, Operation) (Acknowledgment, error)
}

// RefreshIdentity must invoke the existing current live-grant authority. It
// cannot infer a grant from a stored receipt, platform flag or client roles.
type RefreshIdentity func(context.Context) (context.Context, error)
type Commands struct {
	service *Service
	store   ReceiptStore
	writer  ProviderWriter
	refresh RefreshIdentity
}

func NewCommands(service *Service, store ReceiptStore, writer ProviderWriter, refresh RefreshIdentity) *Commands {
	return &Commands{service, store, writer, refresh}
}

var versionPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

func (c *Commands) current(ctx context.Context) (context.Context, authidentity.AuthenticatedIdentity, error) {
	if c == nil || c.service == nil || c.store == nil || c.writer == nil || c.refresh == nil {
		return ctx, authidentity.AuthenticatedIdentity{}, ErrUnavailable
	}
	refreshed, err := c.refresh(ctx)
	if err != nil {
		return ctx, authidentity.AuthenticatedIdentity{}, err
	}
	if refreshed == nil {
		return ctx, authidentity.AuthenticatedIdentity{}, ErrUnavailable
	}
	identity, err := c.service.authorize(refreshed, authz.PermissionWorkbenchOrganizationMemberManage)
	return refreshed, identity, err
}

func (c *Commands) Execute(ctx context.Context, key string, input CommandInput) (Operation, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	ctx, identity, err := c.current(ctx)
	if err != nil {
		return Operation{}, err
	}
	id, err := uuid.Parse(key)
	if err != nil || id == uuid.Nil || id.String() != key {
		return Operation{}, ErrInvalidRequest
	}
	if input.Kind != CommandRole && input.Kind != CommandRemove && input.Kind != CommandInvite {
		return Operation{}, ErrInvalidRequest
	}
	if ((input.Kind == CommandRole || input.Kind == CommandInvite) && !c.service.assignable(input.Role)) || (input.Kind == CommandRemove && input.Role != "") {
		return Operation{}, ErrInvalidRequest
	}
	if input.Kind == CommandInvite {
		if input.AuthorizationID != "" || input.ExpectedVersion != "" {
			return Operation{}, ErrInvalidRequest
		}
		invitation, err := normalizeInvitation(input.Invitation)
		if err != nil {
			return Operation{}, err
		}
		input.Invitation = &invitation
	} else if !authidentity.IsBoundedIdentifier(input.AuthorizationID) || !versionPattern.MatchString(input.ExpectedVersion) || input.Invitation != nil {
		return Operation{}, ErrInvalidRequest
	}
	scope := OperationScope{ProjectID: c.service.projectID, OrganizationID: identity.EffectiveOrganizationID, ActorID: identity.UserID}
	payload, _ := json.Marshal(input)
	digest := sha256.Sum256(payload)
	fingerprint := hex.EncodeToString(digest[:])
	existing, err := c.store.Read(ctx, scope, key)
	if err == nil {
		if existing.Fingerprint != fingerprint {
			return Operation{}, ErrConflict
		}
		return c.resume(ctx, existing)
	}
	if !errors.Is(err, ErrNotFound) {
		return Operation{}, ErrUnavailable
	}
	if input.Kind == CommandInvite {
		op, err := c.store.Begin(ctx, Operation{Scope: scope, Key: key, Fingerprint: fingerprint, Kind: CommandInvite, TargetUserID: uuid.NewString(), Role: input.Role, Invitation: input.Invitation, Step: StepUser, Phase: PhaseReady, Revision: 1})
		if err != nil {
			return Operation{}, err
		}
		return c.resumeInvite(ctx, op)
	}
	member, err := c.readTarget(ctx, input.AuthorizationID)
	if err != nil {
		return Operation{}, err
	}
	// Reserve the verified target before evaluating mutable preconditions. A
	// conflict then has a durable terminal receipt, serialized with same-key work.
	step := StepRole
	if input.Kind == CommandRemove {
		step = StepRemove
	}
	op, err := c.store.Begin(ctx, Operation{Scope: scope, Key: key, Fingerprint: fingerprint, Kind: input.Kind, TargetUserID: member.UserID, AuthorizationID: input.AuthorizationID, ExpectedVersion: input.ExpectedVersion, Role: input.Role, Step: step, Phase: PhaseReady, Revision: 1})
	if err != nil {
		return Operation{}, err
	}
	return c.resume(ctx, op)
}

func (c *Commands) readTarget(ctx context.Context, id string) (Member, error) {
	result, err := c.service.Read(ctx, id)
	if err != nil {
		return Member{}, err
	}
	if len(result.Items) != 1 {
		return Member{}, ErrInvalidResponse
	}
	return result.Items[0], nil
}

func (c *Commands) resume(ctx context.Context, op Operation) (Operation, error) {
	if op.Kind == CommandInvite {
		return c.resumeInvite(ctx, op)
	}
	// Never resend an already dispatched step, including after process rebuild or
	// a committed dispatch whose acknowledgment was lost before the HTTP send.
	if op.Phase != PhaseReady {
		return op, nil
	}
	ctx, identity, err := c.current(ctx)
	if err != nil {
		return Operation{}, err
	}
	if identity.UserID != op.Scope.ActorID || identity.EffectiveOrganizationID != op.Scope.OrganizationID || c.service.projectID != op.Scope.ProjectID {
		return Operation{}, ErrPermission
	}
	member, err := c.readTarget(ctx, op.AuthorizationID)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return op, err
	}
	if errors.Is(err, ErrNotFound) || member.UserID != op.TargetUserID || observedVersion(member) != op.ExpectedVersion || !c.service.editable(member) || (op.Kind == CommandRole && !c.service.assignable(op.Role)) {
		rejected, saveErr := c.store.Apply(ctx, op.Scope, op.Key, op.Revision, OperationChange{Event: EventReject})
		if saveErr != nil {
			return op, saveErr
		}
		return rejected, ErrConflict
	}
	if ctx.Err() != nil {
		return op, ErrUnavailable
	}
	// Expiry is rechecked after provider read and immediately before dispatch.
	if _, err := c.service.authorize(ctx, authz.PermissionWorkbenchOrganizationMemberManage); err != nil {
		return op, err
	}
	dispatchID := uuid.NewString()
	dispatched, err := c.store.Apply(ctx, op.Scope, op.Key, op.Revision, OperationChange{Event: EventDispatch, DispatchID: dispatchID})
	if err != nil {
		return op, err
	}
	// The receipt transaction may have waited past caller expiry or cancellation.
	// A committed dispatch stays reserved even when sending is no longer allowed.
	if ctx.Err() != nil {
		return dispatched, ErrUnavailable
	}
	if _, err := c.service.authorize(ctx, authz.PermissionWorkbenchOrganizationMemberManage); err != nil {
		return dispatched, err
	}
	ack, err := c.writer.Write(ctx, dispatched)
	if err != nil {
		return dispatched, nil
	}
	if ack.ID != op.AuthorizationID {
		return dispatched, nil
	}
	if _, err := time.Parse(time.RFC3339Nano, ack.At); err != nil {
		return dispatched, nil
	}
	completed, err := c.store.Apply(ctx, op.Scope, op.Key, dispatched.Revision, OperationChange{Event: EventAcknowledge, DispatchID: dispatchID, Acknowledgment: &ack})
	if err != nil {
		return dispatched, nil
	}
	return completed, nil
}
