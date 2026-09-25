package membership

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type PendingPageRequest struct {
	Limit int
	After string
}

func (p PendingPageRequest) Valid() bool {
	if p.Limit < 1 || p.Limit > 100 {
		return false
	}
	if p.After == "" {
		return true
	}
	id, err := uuid.Parse(p.After)
	return err == nil && id != uuid.Nil && id.String() == p.After
}

type PendingPage struct {
	Scope OperationScope
	Items []Operation
	Next  string
}

// PendingOperationReader reads the existing receipt owner, never the provider.
type PendingOperationReader interface {
	ListPending(context.Context, OperationScope, PendingPageRequest) (PendingPage, error)
}

func (c *Commands) ListPending(ctx context.Context, page PendingPageRequest) (PendingPage, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if !page.Valid() {
		return PendingPage{}, ErrInvalidRequest
	}
	ctx, identity, err := c.current(ctx)
	if err != nil {
		return PendingPage{}, err
	}
	reader, ok := c.store.(PendingOperationReader)
	if !ok {
		return PendingPage{}, ErrUnavailable
	}
	scope := OperationScope{ProjectID: c.service.projectID, OrganizationID: identity.EffectiveOrganizationID, ActorID: identity.UserID}
	result, err := reader.ListPending(ctx, scope, page)
	if err != nil {
		return PendingPage{}, err
	}
	if ctx.Err() != nil {
		return PendingPage{}, ErrUnavailable
	}
	_, current, err := c.current(ctx)
	if err != nil {
		return PendingPage{}, err
	}
	if current.UserID != scope.ActorID || current.EffectiveOrganizationID != scope.OrganizationID {
		return PendingPage{}, ErrPermission
	}
	if result.Scope != scope || len(result.Items) > page.Limit {
		return PendingPage{}, ErrInvalidResponse
	}
	previous := page.After
	for _, op := range result.Items {
		if op.Scope != scope || !(PendingPageRequest{Limit: 1, After: op.Key}).Valid() || op.Key == "" || op.Key <= previous || !PendingPhase(op.Phase) {
			return PendingPage{}, ErrInvalidResponse
		}
		previous = op.Key
	}
	if result.Next != "" && (len(result.Items) != page.Limit || result.Next != previous) {
		return PendingPage{}, ErrInvalidResponse
	}
	return result, nil
}

func PendingPhase(phase OperationPhase) bool {
	return phase == PhaseReady || phase == PhaseDispatched || phase == PhaseAcknowledged || phase == PhaseIdentityVerified
}
