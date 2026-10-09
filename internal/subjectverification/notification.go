package subjectverification

import (
	"context"
	"task-processor/internal/authidentity"
	"time"
)

// NoticeFact deliberately has no identity document, phone, provider id or URL.
type NoticeFact struct {
	ID, State             string
	ExpiresAt, VerifiedAt time.Time
}

func (s *PersonalService) AuthorizedNoticeFact(ctx context.Context, user string) (NoticeFact, error) {
	i, ok := authidentity.AuthenticatedIdentityFromContext(ctx)
	if !ok || i.UserID != user || !time.Now().Before(i.TokenExpiresAt) {
		return NoticeFact{}, ErrInvalid
	}
	if s == nil || s.Store == nil {
		return NoticeFact{}, ErrUnavailable
	}
	snap, e := s.Store.ReadPersonal(ctx, user, s.Limits)
	if e != nil {
		return NoticeFact{}, e
	}
	a := snap.Application
	if a.ID == "" {
		return NoticeFact{}, nil
	}
	if a.UserID != user || a.Scope != s.Scope {
		return NoticeFact{}, ErrConflict
	}
	state := a.State
	if (state == Pending || state == Unknown) && !a.ExpiresAt.After(snap.Quota.ServerTime) {
		state = "EXPIRED"
	}
	return NoticeFact{ID: a.ID, State: state, ExpiresAt: a.ExpiresAt, VerifiedAt: a.VerifiedAt}, nil
}
func (s *Service) AuthorizedNoticeFact(ctx context.Context, actor Actor) (NoticeFact, error) {
	i, ok := authidentity.AuthenticatedIdentityFromContext(ctx)
	if !ok || i.UserID != actor.UserID || i.TenantID != actor.OrganizationID || !time.Now().Before(i.TokenExpiresAt) {
		return NoticeFact{}, ErrInvalid
	}
	if s == nil || s.Store == nil {
		return NoticeFact{}, ErrUnavailable
	}
	a, e := s.Store.Read(ctx, actor.OrganizationID)
	if e == ErrNotFound {
		return NoticeFact{}, nil
	}
	if e != nil {
		return NoticeFact{}, e
	}
	if a.OrganizationID != actor.OrganizationID || a.Scope != s.Scope {
		return NoticeFact{}, ErrConflict
	}
	state := a.State
	if state == Pending && !a.ExpiresAt.After(s.now()) {
		state = "EXPIRED"
	}
	return NoticeFact{ID: a.ID, State: state, ExpiresAt: a.ExpiresAt, VerifiedAt: a.ProviderVerifiedAt}, nil
}
