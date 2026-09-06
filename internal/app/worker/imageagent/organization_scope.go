package imageagentworker

import (
	"context"
	"gorm.io/gorm"
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
	manager.SetConfigResolver(openai.NewOrganizationCredentialResolver(db))
	return buildProductionImageCapabilities(imageCapabilityRuntime{OpenAIManager: manager})
}

var _ imageagent.ExecutionAuthorizer = OrganizationExecutionAuthorizer{}
