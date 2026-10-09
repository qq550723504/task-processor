package storeobservationsapp

import (
	"context"
	"errors"
	"task-processor/internal/authidentity"
	"task-processor/internal/authruntime/zitadel"
	"task-processor/internal/authz"
	o "task-processor/internal/marketplace/shein/observations"
	"task-processor/internal/storecenter"
	"task-processor/internal/workbenchcontext"
	"time"
)

type ExactIAM interface {
	ReadExactServiceProjectAuthorization(context.Context, string, string, string, string) (zitadel.ExactServiceProjectAuthorization, error)
}
type Authorization struct {
	Client             ExactIAM
	ServiceToken       func(context.Context) (string, error)
	ProjectID          string
	Permissions        *authz.ListingKitAuthorizer
	OrganizationStatus workbenchcontext.OrganizationBusinessStatusChecker
}

func (a Authorization) current(ctx context.Context, scope o.Scope) ([]string, error) {
	if ctx == nil || !scope.Valid() {
		return nil, o.ErrForbidden
	}
	if _, ok := ctx.Deadline(); !ok {
		return nil, o.ErrForbidden
	}
	if ctx.Err() != nil || a.Client == nil || a.ServiceToken == nil || a.ProjectID == "" || a.Permissions == nil {
		return nil, o.ErrUnavailable
	}
	if identity, ok := authidentity.AuthenticatedIdentityFromContext(ctx); ok && (identity.TenantID != scope.OrganizationID || identity.EffectiveOrganizationID != scope.OrganizationID || identity.UserID != scope.ActorID || identity.EffectiveMemberID != scope.MemberID || !time.Now().Before(identity.TokenExpiresAt)) {
		return nil, o.ErrForbidden
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	token, e := a.ServiceToken(ctx)
	if e != nil {
		return nil, o.ErrUnavailable
	}
	grant, e := a.Client.ReadExactServiceProjectAuthorization(ctx, token, scope.ActorID, a.ProjectID, scope.OrganizationID)
	if e != nil {
		return nil, o.ErrUnavailable
	}
	if !grant.Found || grant.State != "STATE_ACTIVE" || grant.AuthorizationID != scope.MemberID {
		return nil, o.ErrForbidden
	}
	if a.OrganizationStatus != nil {
		suspended, e := a.OrganizationStatus.IsOrganizationSuspended(ctx, scope.OrganizationID)
		if e != nil {
			return nil, o.ErrUnavailable
		}
		if suspended {
			return nil, o.ErrForbidden
		}
	}
	if ctx.Err() != nil {
		return nil, o.ErrUnavailable
	}
	return grant.Roles, nil
}
func (a Authorization) Authorize(ctx context.Context, s o.Scope, kind o.Kind, sync bool) error {
	if !kind.Valid() {
		return o.ErrForbidden
	}
	roles, e := a.current(ctx, s)
	if e != nil {
		return e
	}
	return a.permissions(ctx, s, kind, sync, roles)
}
func (a Authorization) permissions(ctx context.Context, s o.Scope, kind o.Kind, sync bool, roles []string) error {
	permissions := []string{authz.PermissionWorkbenchStoreRead, o.ReadPermission(kind)}
	if sync {
		permissions = append(permissions, o.SyncPermission(kind))
	}
	for _, p := range permissions {
		allowed, e := authz.AuthorizeOrganization(ctx, a.Permissions, s.ActorID, s.OrganizationID, roles, p)
		if e != nil {
			return o.ErrUnavailable
		}
		if !allowed {
			return o.ErrForbidden
		}
	}
	return nil
}
func (a Authorization) AuthorizeObservation(ctx context.Context, s storecenter.ObservationSubject) (storecenter.ObservationAuthorization, error) {
	kind := o.Products
	switch s.Purpose {
	case storecenter.ObservationPurposeProducts:
	case storecenter.ObservationPurposeOrders:
		kind = o.Orders
	default:
		return storecenter.ObservationAuthorization{}, storecenter.ErrNotFound
	}
	scope := o.Scope{OrganizationID: s.OrganizationID, ActorID: s.ActorID, MemberID: s.MemberID}
	roles, e := a.current(ctx, scope)
	if e == nil {
		e = a.permissions(ctx, scope, kind, s.Sync, roles)
	}
	if errors.Is(e, o.ErrForbidden) {
		return storecenter.ObservationAuthorization{}, storecenter.ErrNotFound
	}
	if e != nil {
		return storecenter.ObservationAuthorization{}, storecenter.ErrDependencyUnavailable
	}
	return storecenter.ObservationAuthorization{Allowed: true, Access: storecenter.StoreMemberAccess{OrganizationID: s.OrganizationID, ActorID: s.ActorID, MemberID: s.MemberID, Administrator: a.Permissions.IsTenantAdmin("", roles)}}, nil
}
