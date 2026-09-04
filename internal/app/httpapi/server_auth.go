package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"task-processor/internal/authidentity"
	zitadelruntime "task-processor/internal/authruntime/zitadel"
	"task-processor/internal/authz"
	"task-processor/internal/core/config"
	"task-processor/internal/httproute"
	listingkithttpapi "task-processor/internal/listingkit/httpapi"
	"task-processor/internal/workbenchcontext"
)

type organizationIdentityResolver interface {
	Resolve(context.Context, httproute.OrganizationAccessPolicy, workbenchcontext.ResolveInput) (authidentity.AuthenticatedIdentity, error)
}

type routeAuthDependencies struct {
	identityMiddleware   gin.HandlerFunc
	authorizer           *authz.ListingKitAuthorizer
	workbenchVerifier    zitadelruntime.Verifier
	organizationResolver organizationIdentityResolver
	roleMiddleware       func(httproute.Descriptor) gin.HandlerFunc
	auditRecorder        workbenchcontext.AuditRecorder
	auditNow             func() time.Time
}

type routeAuthorization = routeAuthDependencies

const resolvedOrganizationTargetKey = "workbench.resolved-organization-target"

// buildRouteAuthorization creates the immutable authorization dependency owned
// by one HTTP server. It does not publish configuration through package globals.
func buildRouteAuthorization(cfg *config.Config) (routeAuthorization, error) {
	if cfg == nil {
		cfg = &config.Config{}
	}
	zitadel := cfg.ListingKit.Zitadel
	legacyUsernameAllowlistConfigured := zitadel.LegacyUsernameAllowlistConfigured || len(zitadel.AllowedUsernames) > 0
	authorizer, err := authz.NewListingKitAuthorizer(cfg.ListingKit.PlatformAdminUsers, cfg.ListingKit.PlatformAdminRoles)
	if err != nil {
		return routeAuthorization{}, fmt.Errorf("build route authorizer: %w", err)
	}
	return routeAuthorization{
		identityMiddleware: zitadelruntime.NewMiddleware(zitadelruntime.Config{
			IssuerURL:    strings.TrimRight(strings.TrimSpace(zitadel.IssuerURL), "/"),
			ClientID:     strings.TrimSpace(zitadel.ClientID),
			ClientSecret: strings.TrimSpace(zitadel.ClientSecret),
			ProjectID:    strings.TrimSpace(zitadel.ProjectID),
		}, zitadelruntime.AuthorizationConfig{
			Required:                          zitadel.AuthorizationRequired || legacyUsernameAllowlistConfigured,
			LegacyUsernameAllowlistConfigured: legacyUsernameAllowlistConfigured,
			AllowedTenantIDs:                  zitadelruntime.StringSliceToSet(zitadel.AllowedTenantIDs),
			AllowedUserIDs:                    zitadelruntime.StringSliceToSet(zitadel.AllowedUserIDs),
			AllowedRoles:                      zitadelruntime.StringSliceToSet(zitadel.AllowedRoles),
		}),
		authorizer: authorizer,
		auditNow:   time.Now,
	}, nil
}

func routeAuthHandlers(route httproute.Descriptor, authorization routeAuthorization) []gin.HandlerFunc {
	return routeAuthHandlersWithDependencies(route, authorization)
}

func (authorization routeAuthorization) withWorkbench(workbench routeAuthDependencies) routeAuthorization {
	authorization.workbenchVerifier = workbench.workbenchVerifier
	authorization.organizationResolver = workbench.organizationResolver
	authorization.auditRecorder = workbench.auditRecorder
	authorization.auditNow = workbench.auditNow
	if workbench.roleMiddleware != nil {
		authorization.roleMiddleware = workbench.roleMiddleware
	}
	return authorization
}

func newRouteAuthDependencies() routeAuthDependencies {
	return routeAuthDependencies{auditNow: time.Now}
}

func routeAuthHandlersWithDependencies(route httproute.Descriptor, dependencies routeAuthDependencies) []gin.HandlerFunc {
	if route.Method == http.MethodOptions {
		return nil
	}
	requiresOrganization := routeRequiresOrganizationResolution(route.OrganizationAccessPolicy)
	if !requiresOrganization && !listingkithttpapi.RouteRequiresZitadelAuth(route) {
		return nil
	}
	handlers := make([]gin.HandlerFunc, 0, 4)
	if requiresOrganization {
		handlers = append(handlers, workbenchAuthenticationMiddleware(dependencies.workbenchVerifier))
	} else if dependencies.identityMiddleware != nil {
		handlers = append(handlers, dependencies.identityMiddleware)
	}
	if requiresOrganization {
		if route.OrganizationTargetResolver != nil {
			handlers = append(handlers, organizationTargetResolutionMiddleware(route, dependencies))
		}
		handlers = append(handlers, organizationResolutionMiddleware(route, dependencies))
	}
	var roleAuth gin.HandlerFunc
	if dependencies.roleMiddleware != nil {
		roleAuth = dependencies.roleMiddleware(route)
	} else if requiresOrganization {
		roleAuth = listingkithttpapi.NewRouteRoleMiddlewareWithAuthorizerAndResponder(route, dependencies.authorizer, workbenchRoleAuthorizationResponder(route, dependencies))
	} else {
		roleAuth = listingkithttpapi.NewRouteRoleMiddlewareWithAuthorizer(route, dependencies.authorizer)
	}
	if roleAuth != nil {
		handlers = append(handlers, roleAuth)
	}
	return handlers
}

func organizationTargetResolutionMiddleware(route httproute.Descriptor, dependencies routeAuthDependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		target, err := route.OrganizationTargetResolver(c.Request)
		if err != nil {
			if identity, ok := authidentity.AuthenticatedIdentityFromContext(c.Request.Context()); ok {
				_ = recordWorkbenchAudit(c, route, dependencies, identity, "", workbenchcontext.AuditResultInvalidRequest)
			}
			writeWorkbenchProtocolError(c, http.StatusBadRequest, "INVALID_REQUEST", "Request is invalid")
			return
		}
		c.Set(resolvedOrganizationTargetKey, target)
		c.Next()
	}
}

func routeRequiresOrganizationResolution(policy httproute.OrganizationAccessPolicy) bool {
	return policy != "" && policy != httproute.OrganizationAccessPolicyNone
}

func workbenchAuthenticationMiddleware(verifier zitadelruntime.Verifier) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := requestBearerToken(c.GetHeader("Authorization"))
		if token == "" {
			writeWorkbenchContextError(c, workbenchcontext.ErrAuthenticationRequired)
			return
		}
		if verifier == nil {
			writeWorkbenchContextError(c, workbenchcontext.ErrAuthorizationDependencyUnavailable)
			return
		}
		identity, err := verifier.Verify(c.Request.Context(), token)
		if err != nil {
			if zitadelruntime.IsVerificationDependencyUnavailable(err) {
				writeWorkbenchContextError(c, workbenchcontext.ErrAuthorizationDependencyUnavailable)
			} else {
				writeWorkbenchContextError(c, workbenchcontext.ErrAuthenticationRequired)
			}
			return
		}
		for _, header := range []string{
			"X-User-ID", "X-User-Type", "X-User-Roles", "X-Zitadel-Roles", "X-User",
			"X-Tenant-ID", "tenant-id", "X-Tenant",
		} {
			c.Request.Header.Del(header)
		}
		c.Request = c.Request.WithContext(authidentity.WithAuthenticatedIdentity(c.Request.Context(), identity))
		c.Request.Header.Set("X-User-ID", identity.UserID)
		c.Request.Header.Set("X-User-Type", "zitadel")
		c.Next()
	}
}

func workbenchRoleAuthorizationResponder(route httproute.Descriptor, dependencies routeAuthDependencies) listingkithttpapi.RoleAuthorizationResponder {
	return func(c *gin.Context, failure listingkithttpapi.RoleAuthorizationFailure, _ string) {
		if failure == listingkithttpapi.RoleAuthorizationDependencyUnavailable {
			writeWorkbenchContextError(c, workbenchcontext.ErrAuthorizationDependencyUnavailable)
			return
		}
		if identity, ok := authidentity.AuthenticatedIdentityFromContext(c.Request.Context()); ok {
			_ = recordWorkbenchAudit(c, route, dependencies, identity, identity.EffectiveOrganizationID, workbenchcontext.AuditResultPermissionDenied)
		}
		writeWorkbenchProtocolError(c, http.StatusForbidden, "PERMISSION_DENIED", "Permission is denied")
	}
}

func organizationResolutionMiddleware(route httproute.Descriptor, dependencies routeAuthDependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		identity, authenticated := authidentity.AuthenticatedIdentityFromContext(c.Request.Context())
		if !authenticated {
			writeWorkbenchContextError(c, workbenchcontext.ErrAuthenticationRequired)
			return
		}
		if dependencies.organizationResolver == nil {
			writeWorkbenchContextError(c, workbenchcontext.ErrAuthorizationDependencyUnavailable)
			return
		}
		requestedOrganizationID := c.GetHeader("X-Requested-Organization-ID")
		if resolvedTarget, exists := c.Get(resolvedOrganizationTargetKey); exists {
			requestedOrganizationID, _ = resolvedTarget.(string)
		}
		resolved, err := dependencies.organizationResolver.Resolve(c.Request.Context(), route.OrganizationAccessPolicy, workbenchcontext.ResolveInput{
			Identity:                identity,
			BearerToken:             requestBearerToken(c.GetHeader("Authorization")),
			RequestedOrganizationID: requestedOrganizationID,
		})
		if err != nil {
			if result := workbenchAuditResultForError(err); result != "" || route.OrganizationAccessPolicy == httproute.OrganizationAccessPolicyLiveSwitch {
				if result == "" {
					result = workbenchcontext.AuditResultDependencyUnavailable
				}
				_ = recordWorkbenchAudit(c, route, dependencies, identity, requestedOrganizationID, result)
			}
			writeWorkbenchContextError(c, err)
			return
		}
		if route.OrganizationAccessPolicy == httproute.OrganizationAccessPolicyLiveSwitch {
			if err := recordWorkbenchAudit(c, route, dependencies, resolved, resolved.EffectiveOrganizationID, workbenchcontext.AuditResultSuccess); err != nil {
				writeWorkbenchContextError(c, workbenchcontext.ErrAuthorizationDependencyUnavailable)
				return
			}
		}
		c.Request = c.Request.WithContext(authidentity.WithAuthenticatedIdentity(c.Request.Context(), resolved))
		for _, header := range []string{"X-Tenant-ID", "tenant-id", "X-User-Roles"} {
			c.Request.Header.Del(header)
		}
		if resolved.EffectiveOrganizationID != "" {
			c.Request.Header.Set("X-Tenant-ID", resolved.EffectiveOrganizationID)
			c.Request.Header.Set("tenant-id", resolved.EffectiveOrganizationID)
		}
		if len(resolved.Roles) > 0 {
			c.Request.Header.Set("X-User-Roles", strings.Join(resolved.Roles, ","))
		}
		c.Next()
	}
}

func recordWorkbenchAudit(c *gin.Context, route httproute.Descriptor, dependencies routeAuthDependencies, identity authidentity.AuthenticatedIdentity, effectiveOrganizationID, result string) error {
	if dependencies.auditRecorder == nil {
		return errors.New("workbench audit recorder is not configured")
	}
	now := dependencies.auditNow
	if now == nil {
		now = time.Now
	}
	action := route.Permission
	if route.OrganizationAccessPolicy == httproute.OrganizationAccessPolicyLiveSwitch {
		action = workbenchcontext.AuditActionOrganizationSwitch
	} else if strings.TrimSpace(action) == "" {
		action = route.Method
	}
	return dependencies.auditRecorder.Record(c.Request.Context(), workbenchcontext.AuditEvent{
		Subject: identity.UserID, HomeOrganizationID: identity.HomeOrganizationID,
		EffectiveOrganizationID: effectiveOrganizationID,
		Resource:                route.Path, Action: action, Result: result,
		Timestamp: now().UTC(), RequestID: strings.TrimSpace(c.GetHeader("X-Request-ID")),
	})
}

func workbenchAuditResultForError(err error) string {
	switch {
	case errors.Is(err, workbenchcontext.ErrOrganizationAccessDenied):
		return workbenchcontext.AuditResultOrganizationAccessDenied
	case errors.Is(err, workbenchcontext.ErrOrganizationAccessRevoked):
		return workbenchcontext.AuditResultOrganizationAccessRevoked
	case errors.Is(err, workbenchcontext.ErrOrganizationSuspended):
		return workbenchcontext.AuditResultOrganizationSuspended
	case errors.Is(err, workbenchcontext.ErrOrganizationSelectionRequired):
		return workbenchcontext.AuditResultSelectionRequired
	default:
		return ""
	}
}

func requestBearerToken(authorization string) string {
	parts := strings.Fields(authorization)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func writeWorkbenchContextError(c *gin.Context, err error) {
	status := http.StatusServiceUnavailable
	code := "DEPENDENCY_UNAVAILABLE"
	message := "Workbench authorization is temporarily unavailable"
	switch {
	case errors.Is(err, workbenchcontext.ErrAuthenticationRequired):
		status = http.StatusUnauthorized
		code = "AUTHENTICATION_REQUIRED"
		message = "Authentication is required"
	case errors.Is(err, workbenchcontext.ErrOrganizationSelectionRequired):
		status = http.StatusConflict
		code = "ORGANIZATION_SELECTION_REQUIRED"
		message = "Select an organization to continue"
	case errors.Is(err, workbenchcontext.ErrOrganizationAccessDenied):
		status = http.StatusForbidden
		code = "ORGANIZATION_ACCESS_DENIED"
		message = "Organization access is denied"
	case errors.Is(err, workbenchcontext.ErrOrganizationAccessRevoked):
		status = http.StatusForbidden
		code = "ORGANIZATION_ACCESS_REVOKED"
		message = "Organization access is no longer available"
	case errors.Is(err, workbenchcontext.ErrOrganizationSuspended):
		status = http.StatusForbidden
		code = "ORGANIZATION_SUSPENDED"
		message = "Organization access is suspended"
	}
	writeWorkbenchProtocolError(c, status, code, message)
}

func writeWorkbenchProtocolError(c *gin.Context, status int, code string, message string) {
	c.AbortWithStatusJSON(status, gin.H{
		"code":        code,
		"message":     message,
		"requestId":   strings.TrimSpace(c.GetHeader("X-Request-ID")),
		"fieldErrors": []any{},
	})
}
