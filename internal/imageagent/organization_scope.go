package imageagent

import (
	"context"
	"strings"
	"task-processor/internal/authidentity"
)

const OrganizationScopeProtocol = "image-agent.organization.v1"

func (s *Service) identityForRun(identity ExecutionIdentity, run Run) (ExecutionIdentity, error) {
	identity.BusinessTaskID = run.BusinessTaskID
	if s.organizationScope {
		if run.ScopeProtocol != OrganizationScopeProtocol || run.TenantID != identity.TenantID || run.UserID != identity.UserID {
			return ExecutionIdentity{}, ErrIdentityRequired
		}
		identity.RunID = run.ID
		if err := ValidateOrganizationExecution(identity, run.ID); err != nil {
			return ExecutionIdentity{}, err
		}
	}
	return identity, nil
}

// ExecutionAuthorizer checks current permission for the immutable run owner.
// Implementations must query current grants, never replay captured roles.
type ExecutionAuthorizer interface {
	AuthorizeExecution(context.Context, ExecutionIdentity) error
}

func WithOrganizationScope() ServiceOption {
	return func(service *Service) error { service.organizationScope = true; return nil }
}

func (s *Service) executionIdentity(ctx context.Context) (ExecutionIdentity, error) {
	identity, err := verifiedExecutionIdentity(ctx)
	if err != nil || !s.organizationScope {
		return identity, err
	}
	verified, ok := authidentity.AuthenticatedIdentityFromContext(ctx)
	if !ok || verified.EffectiveOrganizationID == "" || verified.TenantID != verified.EffectiveOrganizationID {
		return ExecutionIdentity{}, ErrIdentityRequired
	}
	identity.ScopeProtocol = OrganizationScopeProtocol
	return identity, nil
}

func ValidateOrganizationExecution(identity ExecutionIdentity, runID string) error {
	if identity.ScopeProtocol != OrganizationScopeProtocol || identity.RunID != runID ||
		runID == "" || identity.TenantID == "" || identity.UserID == "" || identity.BusinessTaskID == "" {
		return ErrIdentityRequired
	}
	for _, value := range []string{identity.TenantID, identity.UserID, runID, identity.BusinessTaskID} {
		if value != strings.TrimSpace(value) {
			return ErrIdentityRequired
		}
	}
	return nil
}
