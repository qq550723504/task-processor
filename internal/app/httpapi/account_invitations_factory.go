package httpapi

import (
	"context"
	"net/http"
	"strings"
	"task-processor/internal/authidentity"
	zitadel "task-processor/internal/authruntime/zitadel"
	"task-processor/internal/authz"
	"task-processor/internal/httproute"
	invitationmail "task-processor/internal/integration/mail"
	store "task-processor/internal/integration/persistence/organization/membership"
	provider "task-processor/internal/integration/zitadel/membership"
	domain "task-processor/internal/organization/membership"
	memberhttp "task-processor/internal/organization/membership/httpapi"
	flow "task-processor/internal/organization/membership/inviteflow"
	"task-processor/internal/workbenchcontext"
)

func buildInvitationFactory(deps MembershipDependencies, project string, auth routeAuthDependencies, service *domain.Service, receipts *store.Repository, writer *provider.Writer, authorizer *authz.ListingKitAuthorizer) (memberhttp.InvitationFactory, error) {
	var sender *invitationmail.Sender
	var err error
	if deps.InvitationMail != nil {
		sender, err = invitationmail.New(*deps.InvitationMail)
		if err != nil {
			return nil, err
		}
	}
	directory := zitadel.NewAuthorizationClient(deps.ProviderOrigin, nil)
	self := zitadel.NewSelfServiceClient(deps.ProviderOrigin, nil)
	return func(request *http.Request) (*flow.Service, error) {
		initial, ok := authidentity.AuthenticatedIdentityFromContext(request.Context())
		parts := strings.Fields(request.Header.Get("Authorization"))
		if !ok || len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			return nil, domain.ErrAuthentication
		}
		token := parts[1]
		fresh := func(ctx context.Context) (authidentity.AuthenticatedIdentity, error) {
			verified, err := auth.workbenchVerifier.Verify(ctx, token)
			if err != nil || verified.UserID != initial.UserID {
				return verified, domain.ErrAuthentication
			}
			return verified, nil
		}
		scoped := func(ctx context.Context) (context.Context, error) {
			verified, err := fresh(ctx)
			if err != nil {
				return nil, err
			}
			resolved, err := auth.organizationResolver.Resolve(ctx, httproute.OrganizationAccessPolicyLiveWrite, workbenchcontext.ResolveInput{Identity: verified, BearerToken: token, RequestedOrganizationID: initial.EffectiveOrganizationID})
			if err != nil || resolved.EffectiveOrganizationID != initial.EffectiveOrganizationID {
				return nil, domain.ErrPermission
			}
			return authidentity.WithAuthenticatedIdentity(ctx, resolved), nil
		}
		dependencies := flow.Dependencies{Store: receipts.Invitations(), RoleAllowed: service.AssignableInvitationRole, Authorize: authorizer.Authorize,
			ScopedRoleAllowed: service.ValidInvitationRole,
			ScopedPermissions: func(ctx context.Context, org string, roles []string) ([]string, error) {
				return authorizer.ScopedPermissions(ctx, "", org, roles)
			},
			Manager: func(ctx context.Context, role string) (authidentity.AuthenticatedIdentity, error) {
				ctx, err := scoped(ctx)
				if err != nil {
					return authidentity.AuthenticatedIdentity{}, err
				}
				return service.InvitationManager(ctx, role)
			}, Reader: func(ctx context.Context) (authidentity.AuthenticatedIdentity, error) {
				ctx, err := scoped(ctx)
				if err != nil {
					return authidentity.AuthenticatedIdentity{}, err
				}
				return service.InvitationReader(ctx)
			},
			ReadSelf: func(ctx context.Context) (authidentity.SelfProfile, error) {
				identity, err := fresh(ctx)
				if err != nil {
					return authidentity.SelfProfile{}, err
				}
				facts, err := self.ReadUserFacts(ctx, token, identity.UserID)
				return facts.Profile, err
			},
			ReadGrant: func(ctx context.Context, org, user string) (flow.Grant, error) {
				grant, err := directory.ReadExactServiceProjectAuthorization(ctx, deps.ReadToken, user, project, org)
				state := "inactive"
				if grant.State == "STATE_ACTIVE" {
					state = "active"
				}
				return flow.Grant{Found: grant.Found, ID: grant.AuthorizationID, State: state, Roles: grant.Roles}, err
			},
			WriteGrant: func(ctx context.Context, inv flow.Invitation) error {
				if !service.ValidInvitationRole(ctx, inv.OrganizationID, inv.Role) {
					return flow.ErrPermission
				}
				_, err := writer.Write(ctx, domain.Operation{Scope: domain.OperationScope{ProjectID: inv.ProjectID, OrganizationID: inv.OrganizationID, ActorID: inv.CreatorID}, Key: inv.ID, Kind: domain.CommandInvite, TargetUserID: inv.RecipientID, Role: inv.Role, Step: domain.StepGrant, Phase: domain.PhaseDispatched, DispatchID: inv.DispatchID})
				return err
			}}
		if sender != nil {
			dependencies.Notify = sender.Send
		}
		return flow.New(project, dependencies), nil
	}, nil
}
