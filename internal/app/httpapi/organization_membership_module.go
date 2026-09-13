package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	"task-processor/internal/core/config"
	"task-processor/internal/httproute"
	memberstore "task-processor/internal/integration/persistence/organization/membership"
	memberprovider "task-processor/internal/integration/zitadel/membership"
	kernelmodule "task-processor/internal/kernel/module"
	"task-processor/internal/organization/membership"
	memberhttp "task-processor/internal/organization/membership/httpapi"
	"task-processor/internal/workbenchcontext"
)

type MembershipDependencies struct {
	ReceiptDB                             *gorm.DB
	ProviderOrigin, ReadToken, WriteToken string
}

// NewCurrentApplicationWithMembership borrows pools owned by the caller. It
// incrementally extends current defaults and never installs schema or closes DBs.
func NewCurrentApplicationWithMembership(ctx context.Context, sourceDB, commercialDB *gorm.DB, cfg *config.Config, logger *logrus.Logger, membershipDeps MembershipDependencies) (*http.Server, error) {
	if ctx == nil {
		return nil, errors.New("membership startup context unavailable")
	}
	if membershipDeps.ReceiptDB == nil || membershipDeps.ReceiptDB == sourceDB || membershipDeps.ReceiptDB == commercialDB || cfg == nil {
		return nil, errors.New("membership requires an independent receipt pool")
	}
	factories := defaultCurrentApplicationFactories(ctx)
	factories.buildMembership = func(startup context.Context, authorizer *authz.ListingKitAuthorizer, auth routeAuthDependencies) (kernelmodule.Module, error) {
		return buildMembershipModule(startup, cfg, membershipDeps, authorizer, auth)
	}
	return buildCurrentApplication(ctx, sourceDB, commercialDB, cfg, logger, factories)
}

func buildMembershipModule(ctx context.Context, cfg *config.Config, deps MembershipDependencies, authorizer *authz.ListingKitAuthorizer, auth routeAuthDependencies) (kernelmodule.Module, error) {
	if cfg == nil || authorizer == nil || auth.authorizer != authorizer || auth.workbenchVerifier == nil || auth.organizationResolver == nil {
		return nil, errors.New("membership current authority unavailable")
	}
	origin, err := url.Parse(deps.ProviderOrigin)
	if err != nil {
		return nil, membership.ErrUnavailable
	}
	issuer, err := url.Parse(cfg.ListingKit.Zitadel.IssuerURL)
	if err != nil || origin.Scheme != "http" || origin.Scheme != issuer.Scheme || origin.Host != issuer.Host || origin.User != nil || origin.RawQuery != "" || origin.Fragment != "" || (origin.Path != "" && origin.Path != "/") || (origin.Hostname() != "127.0.0.1" && origin.Hostname() != "localhost" && origin.Hostname() != "::1") || origin.Port() == "" {
		return nil, membership.ErrUnavailable
	}
	if deps.ReadToken == deps.WriteToken || len(deps.ReadToken) > 4096 || len(deps.WriteToken) > 4096 {
		return nil, membership.ErrUnavailable
	}
	if err := memberstore.VerifyRuntimePermissions(ctx, deps.ReceiptDB); err != nil {
		return nil, err
	}
	store, err := memberstore.NewRepository(ctx, deps.ReceiptDB)
	if err != nil {
		return nil, err
	}
	directory, err := memberprovider.NewClient(deps.ProviderOrigin, deps.ReadToken, cfg.ListingKit.Zitadel.ProjectID, nil)
	if err != nil {
		return nil, err
	}
	writer, err := memberprovider.NewWriter(deps.ProviderOrigin, deps.ReadToken, deps.WriteToken, cfg.ListingKit.Zitadel.ProjectID, nil)
	if err != nil {
		return nil, err
	}
	service := membership.NewService(directory, authorizer, cfg.ListingKit.Zitadel.ProjectID, cfg.ListingKit.PlatformAdminRoles...)
	handler := memberhttp.NewCommandHandler(service, func(request *http.Request) (memberhttp.CommandService, error) {
		initial, ok := authidentity.AuthenticatedIdentityFromContext(request.Context())
		if !ok {
			return nil, membership.ErrAuthentication
		}
		parts := strings.Fields(request.Header.Get("Authorization"))
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			return nil, membership.ErrAuthentication
		}
		token := parts[1]
		refresh := func(ctx context.Context) (context.Context, error) {
			verified, err := auth.workbenchVerifier.Verify(ctx, token)
			if err != nil || verified.UserID != initial.UserID {
				return nil, membership.ErrAuthentication
			}
			resolved, err := auth.organizationResolver.Resolve(ctx, httproute.OrganizationAccessPolicyLiveWrite, workbenchcontext.ResolveInput{Identity: verified, BearerToken: token, RequestedOrganizationID: initial.EffectiveOrganizationID})
			if err != nil {
				return nil, membership.ErrPermission
			}
			if resolved.EffectiveOrganizationID != initial.EffectiveOrganizationID {
				return nil, membership.ErrPermission
			}
			return authidentity.WithAuthenticatedIdentity(ctx, resolved), nil
		}
		return membership.NewCommands(service, store, writer, refresh), nil
	})
	if ctx.Err() != nil {
		return nil, membership.ErrUnavailable
	}
	return memberhttp.NewModule(handler), nil
}
