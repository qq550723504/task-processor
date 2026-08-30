package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"task-processor/internal/authidentity"
	zitadelruntime "task-processor/internal/authruntime/zitadel"
	"task-processor/internal/httproute"
	listingkithttpapi "task-processor/internal/listingkit/httpapi"
	"task-processor/internal/workbenchcontext"
)

type organizationIdentityResolver interface {
	Resolve(context.Context, httproute.OrganizationAccessPolicy, workbenchcontext.ResolveInput) (authidentity.AuthenticatedIdentity, error)
}

type routeAuthDependencies struct {
	// legacyZitadelAuth is the legacy combined authentication/allowlist middleware.
	legacyZitadelAuth    gin.HandlerFunc
	workbenchVerifier    zitadelruntime.Verifier
	organizationResolver organizationIdentityResolver
	roleMiddleware       func(httproute.Descriptor) gin.HandlerFunc
}

func newRouteAuthDependencies() routeAuthDependencies {
	return routeAuthDependencies{legacyZitadelAuth: newLegacyZitadelAuthMiddleware()}
}

func routeAuthHandlers(route httproute.Descriptor, zitadelAuth gin.HandlerFunc) []gin.HandlerFunc {
	if zitadelAuth == nil && !routeRequiresOrganizationResolution(route.OrganizationAccessPolicy) {
		return nil
	}
	return routeAuthHandlersWithDependencies(route, routeAuthDependencies{legacyZitadelAuth: zitadelAuth})
}

func routeAuthHandlersWithDependencies(route httproute.Descriptor, dependencies routeAuthDependencies) []gin.HandlerFunc {
	if route.Method == http.MethodOptions {
		return nil
	}
	requiresOrganization := routeRequiresOrganizationResolution(route.OrganizationAccessPolicy)
	if !requiresOrganization && !listingkithttpapi.RouteRequiresZitadelAuth(route) {
		return nil
	}
	handlers := make([]gin.HandlerFunc, 0, 3)
	if requiresOrganization {
		handlers = append(handlers, workbenchAuthenticationMiddleware(dependencies.workbenchVerifier))
	} else if dependencies.legacyZitadelAuth != nil {
		handlers = append(handlers, dependencies.legacyZitadelAuth)
	}
	if requiresOrganization {
		handlers = append(handlers, organizationResolutionMiddleware(route.OrganizationAccessPolicy, dependencies.organizationResolver))
	}
	var roleAuth gin.HandlerFunc
	if dependencies.roleMiddleware != nil {
		roleAuth = dependencies.roleMiddleware(route)
	} else if requiresOrganization {
		roleAuth = listingkithttpapi.NewRouteRoleMiddlewareWithResponder(route, workbenchRoleAuthorizationResponder)
	} else {
		roleAuth = listingkithttpapi.NewRouteRoleMiddleware(route)
	}
	if roleAuth != nil {
		handlers = append(handlers, roleAuth)
	}
	return handlers
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

func workbenchRoleAuthorizationResponder(c *gin.Context, failure listingkithttpapi.RoleAuthorizationFailure, _ string) {
	if failure == listingkithttpapi.RoleAuthorizationDependencyUnavailable {
		writeWorkbenchContextError(c, workbenchcontext.ErrAuthorizationDependencyUnavailable)
		return
	}
	writeWorkbenchProtocolError(c, http.StatusForbidden, "PERMISSION_DENIED", "Permission is denied")
}

func organizationResolutionMiddleware(policy httproute.OrganizationAccessPolicy, resolver organizationIdentityResolver) gin.HandlerFunc {
	return func(c *gin.Context) {
		identity, authenticated := authidentity.AuthenticatedIdentityFromContext(c.Request.Context())
		if !authenticated {
			writeWorkbenchContextError(c, workbenchcontext.ErrAuthenticationRequired)
			return
		}
		if resolver == nil {
			writeWorkbenchContextError(c, workbenchcontext.ErrAuthorizationDependencyUnavailable)
			return
		}
		resolved, err := resolver.Resolve(c.Request.Context(), policy, workbenchcontext.ResolveInput{
			Identity:                identity,
			BearerToken:             requestBearerToken(c.GetHeader("Authorization")),
			RequestedOrganizationID: c.GetHeader("X-Requested-Organization-ID"),
		})
		if err != nil {
			writeWorkbenchContextError(c, err)
			return
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

func newLegacyZitadelAuthMiddleware() gin.HandlerFunc {
	return listingkithttpapi.NewZitadelAuthMiddlewareFromEnv()
}
