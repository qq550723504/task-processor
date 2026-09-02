// Package workbenchcontext resolves a verified workbench organization without
// widening a user's organization-scoped authorization grants.
package workbenchcontext

import (
	"errors"
	"strings"

	"task-processor/internal/authidentity"
)

var (
	ErrOrganizationSelectionRequired      = errors.New("organization selection required")
	ErrOrganizationAccessDenied           = errors.New("organization access denied")
	ErrOrganizationAccessRevoked          = errors.New("organization access revoked")
	ErrAuthorizationDependencyUnavailable = errors.New("authorization dependency unavailable")
)

// SelectOrganization applies the default organization policy to verified,
// organization-scoped grants. A requested organization must be authorized;
// it never falls back to another organization.
func SelectOrganization(
	requestedOrganizationID string,
	homeOrganizationID string,
	grants []authidentity.OrganizationGrant,
) (authidentity.OrganizationGrant, error) {
	requestedOrganizationID = strings.TrimSpace(requestedOrganizationID)
	homeOrganizationID = strings.TrimSpace(homeOrganizationID)

	if len(grants) == 0 {
		return authidentity.OrganizationGrant{}, ErrOrganizationAccessRevoked
	}
	if requestedOrganizationID != "" {
		for _, grant := range grants {
			if grant.OrganizationID == requestedOrganizationID {
				return cloneGrant(grant), nil
			}
		}
		return authidentity.OrganizationGrant{}, ErrOrganizationAccessDenied
	}
	if homeOrganizationID != "" {
		for _, grant := range grants {
			if grant.OrganizationID == homeOrganizationID {
				return cloneGrant(grant), nil
			}
		}
	}
	if len(grants) == 1 {
		return cloneGrant(grants[0]), nil
	}
	return authidentity.OrganizationGrant{}, ErrOrganizationSelectionRequired
}

func cloneGrant(grant authidentity.OrganizationGrant) authidentity.OrganizationGrant {
	grant.Roles = append([]string(nil), grant.Roles...)
	return grant
}
