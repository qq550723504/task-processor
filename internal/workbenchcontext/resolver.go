package workbenchcontext

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"task-processor/internal/authidentity"
	"task-processor/internal/httproute"
)

var (
	ErrAuthenticationRequired = errors.New("authentication required")
	ErrOrganizationSuspended  = errors.New("organization suspended")
)

// GrantLoader is the verified ZITADEL grant source needed by request-time
// organization resolution.
type GrantLoader interface {
	Load(context.Context, GrantSource, GrantRequest) (GrantResult, error)
	Invalidate(subject string, projectID string)
}

// OrganizationBusinessStatusChecker is a deny-only local overlay. A nil
// checker means no additional local suspension policy is configured.
type OrganizationBusinessStatusChecker interface {
	IsOrganizationSuspended(context.Context, string) (bool, error)
}

// ResolveInput contains request-local authenticated inputs. BearerToken must
// be supplied directly from the current HTTP request and is never retained.
type ResolveInput struct {
	Identity                authidentity.AuthenticatedIdentity
	BearerToken             string
	RequestedOrganizationID string
}

// ResolverOption customizes deterministic resolver behavior.
type ResolverOption func(*Resolver)

// WithResolverClock injects the clock used for token-expiry decisions.
func WithResolverClock(now func() time.Time) ResolverOption {
	return func(resolver *Resolver) {
		if now != nil {
			resolver.now = now
		}
	}
}

// Resolver derives one request's effective organization from verified grants.
type Resolver struct {
	grants          GrantLoader
	projectID       string
	contractVersion string
	status          OrganizationBusinessStatusChecker
	now             func() time.Time
}

func NewResolver(
	grants GrantLoader,
	projectID string,
	contractVersion string,
	status OrganizationBusinessStatusChecker,
	options ...ResolverOption,
) *Resolver {
	resolver := &Resolver{
		grants:          grants,
		projectID:       strings.TrimSpace(projectID),
		contractVersion: strings.TrimSpace(contractVersion),
		status:          status,
		now:             time.Now,
	}
	for _, option := range options {
		if option != nil {
			option(resolver)
		}
	}
	return resolver
}

func (resolver *Resolver) Resolve(
	ctx context.Context,
	policy httproute.OrganizationAccessPolicy,
	input ResolveInput,
) (authidentity.AuthenticatedIdentity, error) {
	if resolver == nil || resolver.grants == nil {
		return authidentity.AuthenticatedIdentity{}, fmt.Errorf("%w: grant resolver is not configured", ErrAuthorizationDependencyUnavailable)
	}
	input.BearerToken = strings.TrimSpace(input.BearerToken)
	input.Identity.UserID = strings.TrimSpace(input.Identity.UserID)
	if input.BearerToken == "" || input.Identity.UserID == "" || input.Identity.TokenExpiresAt.IsZero() ||
		!resolver.now().Before(input.Identity.TokenExpiresAt) {
		return authidentity.AuthenticatedIdentity{}, ErrAuthenticationRequired
	}
	source := GrantReadCached
	switch policy {
	case httproute.OrganizationAccessPolicyCachedRead, httproute.OrganizationAccessPolicyContextRead:
	case httproute.OrganizationAccessPolicyLiveWrite:
		source = GrantLive
	case httproute.OrganizationAccessPolicyLiveSwitch:
		source = GrantLive
		subject := strings.TrimSpace(input.Identity.UserID)
		resolver.grants.Invalidate(subject, resolver.projectID)
		defer resolver.grants.Invalidate(subject, resolver.projectID)
	default:
		return authidentity.AuthenticatedIdentity{}, fmt.Errorf("%w: unsupported organization access policy", ErrAuthorizationDependencyUnavailable)
	}
	result, err := resolver.grants.Load(ctx, source, GrantRequest{
		BearerToken:     input.BearerToken,
		Subject:         input.Identity.UserID,
		ProjectID:       resolver.projectID,
		ContractVersion: resolver.contractVersion,
		TokenExpiresAt:  input.Identity.TokenExpiresAt,
	})
	if err != nil {
		return authidentity.AuthenticatedIdentity{}, fmt.Errorf("%w: grant lookup failed", ErrAuthorizationDependencyUnavailable)
	}
	grants, err := normalizeGrants(result.Grants, resolver.projectID)
	if err != nil {
		return authidentity.AuthenticatedIdentity{}, err
	}
	selected, err := SelectOrganization(input.RequestedOrganizationID, input.Identity.HomeOrganizationID, grants)
	if err != nil {
		if policy == httproute.OrganizationAccessPolicyContextRead && strings.TrimSpace(input.RequestedOrganizationID) == "" &&
			(err == ErrOrganizationSelectionRequired || err == ErrOrganizationAccessRevoked) {
			identity := input.Identity
			identity.TenantID = ""
			identity.EffectiveOrganizationID = ""
			identity.Roles = nil
			identity.OrganizationGrants = grants
			identity, _ = authidentity.AuthenticatedIdentityFromContext(authidentity.WithAuthenticatedIdentity(ctx, identity))
			return identity, nil
		}
		return authidentity.AuthenticatedIdentity{}, err
	}
	if resolver.status != nil {
		suspended, statusErr := resolver.status.IsOrganizationSuspended(ctx, selected.OrganizationID)
		if statusErr != nil {
			return authidentity.AuthenticatedIdentity{}, fmt.Errorf("%w: organization status check failed", ErrAuthorizationDependencyUnavailable)
		}
		if suspended {
			return authidentity.AuthenticatedIdentity{}, ErrOrganizationSuspended
		}
	}
	identity := input.Identity
	identity.TenantID = selected.OrganizationID
	identity.EffectiveOrganizationID = selected.OrganizationID
	identity.Roles = append([]string(nil), selected.Roles...)
	identity.OrganizationGrants = grants
	identity, _ = authidentity.AuthenticatedIdentityFromContext(authidentity.WithAuthenticatedIdentity(ctx, identity))
	return identity, nil
}
