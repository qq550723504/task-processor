package commercetoolauth

import (
	"context"
	"reflect"
	"strings"
	"time"

	"task-processor/internal/authidentity"
	"task-processor/internal/commercetool"
)

// FreshOrganizationResolver must obtain current grants without a cache hit or
// error fallback. Assembly delegates to Workbench Resolve(LiveWrite); that
// policy controls grant freshness, while Casbin still checks read permission.
type FreshOrganizationResolver interface {
	ResolveFreshOrganization(context.Context, OrganizationRequest) (authidentity.AuthenticatedIdentity, error)
}

type FreshOrganizationResolverFunc func(context.Context, OrganizationRequest) (authidentity.AuthenticatedIdentity, error)

func (f FreshOrganizationResolverFunc) ResolveFreshOrganization(ctx context.Context, request OrganizationRequest) (authidentity.AuthenticatedIdentity, error) {
	return f(ctx, request)
}

type FreshWorkbenchPrincipalResolver struct {
	resolver FreshOrganizationResolver
	now      func() time.Time
}

func NewFreshWorkbenchPrincipalResolver(resolver FreshOrganizationResolver, now func() time.Time) (*FreshWorkbenchPrincipalResolver, error) {
	if resolver == nil {
		return nil, errTrustedIdentityUnavailable
	}
	value := reflect.ValueOf(resolver)
	switch value.Kind() {
	case reflect.Pointer, reflect.Func, reflect.Interface, reflect.Map, reflect.Slice, reflect.Chan:
		if value.IsNil() {
			return nil, errTrustedIdentityUnavailable
		}
	}
	if now == nil {
		now = time.Now
	}
	return &FreshWorkbenchPrincipalResolver{resolver: resolver, now: now}, nil
}

func (r *FreshWorkbenchPrincipalResolver) ResolvePrincipal(ctx context.Context) (commercetool.Principal, error) {
	return r.ResolveFreshPrincipal(ctx)
}

// ResolveFreshPrincipal also supplies an explicit freshness port to tools
// without requiring a domain-to-integration import.
func (r *FreshWorkbenchPrincipalResolver) ResolveFreshPrincipal(ctx context.Context) (commercetool.Principal, error) {
	if r == nil || r.resolver == nil || r.now == nil || ctx == nil {
		return commercetool.Principal{}, errTrustedIdentityUnavailable
	}
	if err := ctx.Err(); err != nil {
		return commercetool.Principal{}, err
	}
	request, ok := ctx.Value(organizationRequestKey{}).(OrganizationRequest)
	selected := request.RequestedOrganizationID
	if !ok || selected == "" || selected != strings.TrimSpace(selected) || strings.TrimSpace(request.Identity.UserID) == "" || request.Identity.UserID != strings.TrimSpace(request.Identity.UserID) || strings.TrimSpace(request.BearerToken) == "" || !request.Identity.TokenExpiresAt.After(r.now()) {
		return commercetool.Principal{}, errTrustedIdentityUnavailable
	}
	resolved, err := r.resolver.ResolveFreshOrganization(ctx, request)
	if err != nil {
		return commercetool.Principal{}, err
	}
	if err := ctx.Err(); err != nil {
		return commercetool.Principal{}, err
	}
	if resolved.UserID != request.Identity.UserID || !resolved.TokenExpiresAt.Equal(request.Identity.TokenExpiresAt) ||
		resolved.TenantID != selected || resolved.EffectiveOrganizationID != selected {
		return commercetool.Principal{}, errTrustedIdentityUnavailable
	}
	return (ContextPrincipalResolver{Now: r.now}).ResolvePrincipal(authidentity.WithAuthenticatedIdentity(ctx, resolved))
}
