package commercetoolauth

import (
	"context"
	"errors"
	"reflect"
	"time"

	"task-processor/internal/authidentity"
	"task-processor/internal/commercetool"
)

type organizationRequestKey struct{}

// OrganizationRequest contains only the request-local inputs needed to select
// and verify the current effective organization. It must be installed anew for
// each Commerce Tool invocation.
type OrganizationRequest struct {
	Identity                authidentity.AuthenticatedIdentity
	BearerToken             string
	RequestedOrganizationID string
}

// CachedReadOrganizationResolver is the narrow request-identity port. Its
// implementation must apply the Workbench cached-read grant policy; the method
// name makes that policy part of the adapter contract without importing HTTP
// routing or Workbench implementation packages into this integration seam.
type CachedReadOrganizationResolver interface {
	ResolveCachedReadOrganization(context.Context, OrganizationRequest) (authidentity.AuthenticatedIdentity, error)
}

type CachedReadOrganizationResolverFunc func(context.Context, OrganizationRequest) (authidentity.AuthenticatedIdentity, error)

func (f CachedReadOrganizationResolverFunc) ResolveCachedReadOrganization(ctx context.Context, request OrganizationRequest) (authidentity.AuthenticatedIdentity, error) {
	return f(ctx, request)
}

func WithOrganizationRequest(ctx context.Context, request OrganizationRequest) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, organizationRequestKey{}, request)
}

// WorkbenchPrincipalResolver performs cached-read grant resolution during
// every BoundToolSet.Invoke preflight. It never retains a resolved principal.
type WorkbenchPrincipalResolver struct {
	resolver CachedReadOrganizationResolver
	now      func() time.Time
}

func NewWorkbenchPrincipalResolver(resolver CachedReadOrganizationResolver, now func() time.Time) (*WorkbenchPrincipalResolver, error) {
	if nilCachedReadOrganizationResolver(resolver) {
		return nil, errors.New("workbench organization resolver is nil")
	}
	if now == nil {
		now = time.Now
	}
	return &WorkbenchPrincipalResolver{resolver: resolver, now: now}, nil
}

func nilCachedReadOrganizationResolver(resolver CachedReadOrganizationResolver) bool {
	if resolver == nil {
		return true
	}
	value := reflect.ValueOf(resolver)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

func (r *WorkbenchPrincipalResolver) ResolvePrincipal(ctx context.Context) (commercetool.Principal, error) {
	if r == nil || r.resolver == nil {
		return commercetool.Principal{}, errTrustedIdentityUnavailable
	}
	request, ok := ctx.Value(organizationRequestKey{}).(OrganizationRequest)
	if !ok {
		return commercetool.Principal{}, errTrustedIdentityUnavailable
	}
	resolved, err := r.resolver.ResolveCachedReadOrganization(ctx, request)
	if err != nil {
		return commercetool.Principal{}, err
	}
	resolvedCtx := authidentity.WithAuthenticatedIdentity(ctx, resolved)
	return (ContextPrincipalResolver{Now: r.now}).ResolvePrincipal(resolvedCtx)
}

var _ commercetool.PrincipalResolver = (*WorkbenchPrincipalResolver)(nil)
