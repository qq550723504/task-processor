package commercetoolauth

import (
	"context"
	"errors"
	"strings"

	"task-processor/internal/authidentity"
	"task-processor/internal/commercetool"
)

var errTrustedIdentityUnavailable = errors.New("trusted identity unavailable")

// ContextPrincipalResolver accepts only the identity installed by the
// authenticated server middleware. It deliberately ignores headers, query
// parameters, legacy identity contexts, and tool arguments.
type ContextPrincipalResolver struct{}

func (ContextPrincipalResolver) ResolvePrincipal(ctx context.Context) (commercetool.Principal, error) {
	identity, ok := authidentity.AuthenticatedIdentityFromContext(ctx)
	if !ok || identity.TenantID == "" || identity.UserID == "" || len(identity.Roles) == 0 {
		return commercetool.Principal{}, errTrustedIdentityUnavailable
	}
	for _, role := range identity.Roles {
		if role == "" || role != strings.TrimSpace(role) {
			return commercetool.Principal{}, errTrustedIdentityUnavailable
		}
	}
	return commercetool.Principal{
		TenantID: identity.TenantID,
		UserID:   identity.UserID,
		Roles:    append([]string(nil), identity.Roles...),
	}, nil
}
