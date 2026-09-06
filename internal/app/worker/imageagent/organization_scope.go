package imageagentworker

import (
	"context"
	"gorm.io/gorm"
	"task-processor/internal/authidentity"
	zitadel "task-processor/internal/authruntime/zitadel"
	"task-processor/internal/authz"
	"task-processor/internal/imageagent"
	openai "task-processor/internal/integration/openai"
	"task-processor/internal/workbenchcontext"
)

// OrganizationExecutionAuthorizer is explicitly assembled; no default worker
// installs it or obtains service credentials on behalf of the process.
type OrganizationExecutionAuthorizer struct {
	Client             *zitadel.AuthorizationClient
	ServiceToken       func(context.Context) (string, error)
	ProjectID          string
	Authorizer         *authz.ListingKitAuthorizer
	OrganizationStatus workbenchcontext.OrganizationBusinessStatusChecker
}

func (a OrganizationExecutionAuthorizer) AuthorizeExecution(ctx context.Context, identity imageagent.ExecutionIdentity) error {
	if a.Client == nil || a.ServiceToken == nil || a.Authorizer == nil || a.ProjectID == "" {
		return imageagent.ErrIdentityRequired
	}
	if err := imageagent.ValidateOrganizationExecution(identity, identity.RunID); err != nil {
		return err
	}
	token, err := a.ServiceToken(ctx)
	if err != nil {
		return imageagent.ErrIdentityRequired
	}
	grants, err := a.Client.ListServiceProjectAuthorizations(ctx, token, identity.UserID, a.ProjectID, identity.TenantID)
	if err != nil {
		return imageagent.ErrIdentityRequired
	}
	if a.OrganizationStatus != nil {
		suspended, err := a.OrganizationStatus.IsOrganizationSuspended(ctx, identity.TenantID)
		if err != nil || suspended {
			return imageagent.ErrIdentityRequired
		}
	}
	for _, grant := range grants {
		if grant.OrganizationID == identity.TenantID && grant.ProjectID == a.ProjectID && a.Authorizer.Authorize(identity.UserID, grant.Roles, authz.PermissionImageAgentWrite) {
			return nil
		}
	}
	return imageagent.ErrIdentityRequired
}

// BuildOrganizationImageCapabilities reuses the current production ports and
// adapters with a mandatory organization credential resolver. It intentionally
// does not attach InvocationRecorder; #334 owns that subsequent change.
func BuildOrganizationImageCapabilities(manager *openai.Manager, db *gorm.DB) (ImageCapabilities, error) {
	if manager == nil || db == nil {
		return ImageCapabilities{}, imageagent.ErrIdentityRequired
	}
	manager.SetConfigResolver(organizationCredentialAdmission{resolver: openai.NewOrganizationCredentialResolver(db)})
	return buildProductionImageCapabilities(imageCapabilityRuntime{OpenAIManager: manager})
}

var _ imageagent.ExecutionAuthorizer = OrganizationExecutionAuthorizer{}

// App owns the authenticated-to-provider identity boundary. The Integration
// resolver consumes only its existing shared identity contract and never
// imports application authentication policy.
type organizationCredentialAdmission struct{ resolver openai.ClientConfigResolver }

func (r organizationCredentialAdmission) ResolveClientConfig(ctx context.Context, name string, fallback *openai.ClientConfig) (*openai.ResolvedClientConfig, error) {
	identity := openai.IdentityFromContext(ctx)
	verified, ok := authidentity.AuthenticatedIdentityFromContext(ctx)
	if !ok || verified.EffectiveOrganizationID == "" || verified.EffectiveOrganizationID != identity.TenantID || verified.TenantID != identity.TenantID || verified.UserID != identity.UserID || r.resolver == nil {
		return nil, openai.ErrClientConfigurationUnavailable
	}
	return r.resolver.ResolveClientConfig(ctx, name, fallback)
}
