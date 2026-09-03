package authidentity

import (
	"context"
	"strings"
	"time"

	"task-processor/internal/shared/aiidentity"
)

type authenticatedIdentityContextKey struct{}

// OrganizationGrant preserves roles within the Organization and project that grant them.
type OrganizationGrant struct {
	OrganizationID   string
	OrganizationName string
	ProjectID        string
	Roles            []string
}

// AuthenticatedIdentity identifies the user and Organization scope verified for a request.
type AuthenticatedIdentity struct {
	// TenantID and the pre-resolution Roles are retained for existing routes. New
	// workbench handlers use EffectiveOrganizationID and its scoped Roles only.
	TenantID                string
	UserID                  string
	Roles                   []string
	HomeOrganizationID      string
	EffectiveOrganizationID string
	OrganizationGrants      []OrganizationGrant
	TokenExpiresAt          time.Time
}

// WithAuthenticatedIdentity attaches a normalized verified identity to ctx.
func WithAuthenticatedIdentity(ctx context.Context, identity AuthenticatedIdentity) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	identity = normalizeIdentity(identity)
	ctx = context.WithValue(ctx, authenticatedIdentityContextKey{}, identity)
	ai := aiidentity.FromContext(ctx)
	ai.TenantID = identity.TenantID
	ai.UserID = identity.UserID
	return aiidentity.WithIdentity(ctx, ai)
}

// AuthenticatedIdentityFromContext returns the verified identity stored in ctx.
func AuthenticatedIdentityFromContext(ctx context.Context) (AuthenticatedIdentity, bool) {
	if ctx == nil {
		return AuthenticatedIdentity{}, false
	}
	identity, ok := ctx.Value(authenticatedIdentityContextKey{}).(AuthenticatedIdentity)
	if !ok || strings.TrimSpace(identity.UserID) == "" {
		return AuthenticatedIdentity{}, false
	}
	return normalizeIdentity(identity), true
}

func normalizeIdentity(identity AuthenticatedIdentity) AuthenticatedIdentity {
	identity.TenantID = strings.TrimSpace(identity.TenantID)
	identity.UserID = strings.TrimSpace(identity.UserID)
	identity.Roles = append([]string(nil), identity.Roles...)
	identity.HomeOrganizationID = strings.TrimSpace(identity.HomeOrganizationID)
	identity.EffectiveOrganizationID = strings.TrimSpace(identity.EffectiveOrganizationID)
	identity.OrganizationGrants = cloneOrganizationGrants(identity.OrganizationGrants)
	if !identity.TokenExpiresAt.IsZero() {
		identity.TokenExpiresAt = identity.TokenExpiresAt.Round(0).UTC()
	}
	return identity
}

func cloneOrganizationGrants(grants []OrganizationGrant) []OrganizationGrant {
	if grants == nil {
		return nil
	}
	cloned := make([]OrganizationGrant, len(grants))
	for index, grant := range grants {
		grant.OrganizationID = strings.TrimSpace(grant.OrganizationID)
		grant.OrganizationName = strings.TrimSpace(grant.OrganizationName)
		grant.ProjectID = strings.TrimSpace(grant.ProjectID)
		grant.Roles = append([]string(nil), grant.Roles...)
		cloned[index] = grant
	}
	return cloned
}
