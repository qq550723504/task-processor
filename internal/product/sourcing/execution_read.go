package sourcing

import (
	"context"
	"task-processor/internal/authidentity"
	"time"
)

// PublicationExecutionScope is resolved from a durable, owner-qualified source
// and current original membership by the explicitly injected app authority.
type publicationExecutionProofKey struct{}
type publicationExecutionProof struct {
	subject           PublicationExecutionScope
	expires, deadline time.Time
}

type PublicationExecutionScope struct {
	OrganizationID, ActorID, MemberID, PublicationID, ProductKey string
	CatalogVersion                                               uint64
}

func (s PublicationExecutionScope) valid() bool {
	for _, v := range []string{s.OrganizationID, s.ActorID, s.MemberID, s.PublicationID, s.ProductKey} {
		if !authidentity.IsBoundedIdentifier(v) {
			return false
		}
	}
	return s.CatalogVersion > 0 && s.CatalogVersion <= 1<<63-1
}

type ExecutionPublicationGateway struct {
	store   PublicationReadStore
	resolve func(context.Context) (PublicationExecutionScope, error)
}

func NewExecutionPublicationGateway(store PublicationReadStore, resolve func(context.Context) (PublicationExecutionScope, error)) (*ExecutionPublicationGateway, error) {
	if store == nil || resolve == nil {
		return nil, ErrSourcePublicationUnavailable
	}
	return &ExecutionPublicationGateway{store, resolve}, nil
}
func (g *ExecutionPublicationGateway) current(ctx context.Context) (PublicationExecutionScope, error) {
	if g == nil || ctx == nil || ctx.Err() != nil {
		return PublicationExecutionScope{}, ErrPublicationForbidden
	}
	if _, authenticated := authidentity.AuthenticatedIdentityFromContext(ctx); authenticated {
		return PublicationExecutionScope{}, ErrPublicationForbidden
	}
	deadline, bounded := ctx.Deadline()
	if !bounded || !time.Now().Before(deadline) {
		return PublicationExecutionScope{}, ErrPublicationForbidden
	}
	subject, err := g.resolve(ctx)
	if err != nil || !subject.valid() || ctx.Err() != nil {
		return PublicationExecutionScope{}, ErrPublicationForbidden
	}
	return subject, nil
}
func (g *ExecutionPublicationGateway) AuthorizePublicationExecution(ctx context.Context, org, actor string) error {
	subject, err := g.current(ctx)
	if err != nil || subject.OrganizationID != org || subject.ActorID != actor {
		return ErrPublicationForbidden
	}
	return nil
}

type executionPublicationAuthority struct{ subject PublicationExecutionScope }

func (a executionPublicationAuthority) Authorize(context.Context) (PublicationScope, error) {
	return PublicationScope{OrganizationID: a.subject.OrganizationID, ActorID: a.subject.ActorID}, nil
}
func (g *ExecutionPublicationGateway) Read(ctx context.Context, id string) (PersistedPublication, error) {
	subject, err := g.current(ctx)
	if err != nil {
		return PersistedPublication{}, err
	}
	if id != subject.PublicationID {
		return PersistedPublication{}, ErrPublicationForbidden
	}
	value, err := readPersistedPublication(ctx, executionPublicationAuthority{subject}, g.store, id)
	if err != nil {
		return value, err
	}
	if value.Receipt.ActorID != subject.ActorID || value.Receipt.ProductKey != subject.ProductKey || value.Receipt.CatalogVersion != subject.CatalogVersion {
		return PersistedPublication{}, ErrSourcePublicationConflict
	}
	return value, nil
}
func (g *ExecutionPublicationGateway) AuthorizeRead(ctx context.Context) (context.Context, error) {
	subject, err := g.current(ctx)
	if err != nil {
		return ctx, err
	}
	return withPublicationExecutionProof(ctx, subject, time.Now())
}
func withPublicationExecutionProof(ctx context.Context, subject PublicationExecutionScope, now time.Time) (context.Context, error) {
	deadline, bounded := ctx.Deadline()
	_, authenticated := authidentity.AuthenticatedIdentityFromContext(ctx)
	if !bounded || authenticated || !subject.valid() || !now.Before(deadline) {
		return ctx, ErrPublicationForbidden
	}
	expires := now.Add(publicationReadProofTTL)
	if deadline.Before(expires) {
		expires = deadline
	}
	return context.WithValue(ctx, publicationExecutionProofKey{}, publicationExecutionProof{subject: subject, expires: expires, deadline: deadline}), nil
}
func verifyPublicationExecutionProof(ctx context.Context, now time.Time) (PublicationExecutionScope, bool, error) {
	proof, present := ctx.Value(publicationExecutionProofKey{}).(publicationExecutionProof)
	if !present {
		return PublicationExecutionScope{}, false, nil
	}
	deadline, bounded := ctx.Deadline()
	_, authenticated := authidentity.AuthenticatedIdentityFromContext(ctx)
	if authenticated || !bounded || !proof.subject.valid() || deadline.After(proof.deadline) || !now.Before(deadline) || !now.Before(proof.expires) {
		return PublicationExecutionScope{}, true, ErrPublicationForbidden
	}
	return proof.subject, true, nil
}
