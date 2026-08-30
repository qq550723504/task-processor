package httpapi

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"task-processor/internal/authidentity"
	"task-processor/internal/httproute"
	listingkithttpapi "task-processor/internal/listingkit/httpapi"
	"task-processor/internal/workbenchcontext"
)

type organizationIdentityResolver interface {
	Resolve(context.Context, httproute.OrganizationAccessPolicy, workbenchcontext.ResolveInput) (authidentity.AuthenticatedIdentity, error)
}

type routeAuthDependencies struct {
	zitadelAuth          gin.HandlerFunc
	organizationResolver organizationIdentityResolver
	roleMiddleware       func(httproute.Descriptor) gin.HandlerFunc
}

func routeAuthHandlers(route httproute.Descriptor, zitadelAuth gin.HandlerFunc) []gin.HandlerFunc {
	if zitadelAuth == nil && !routeRequiresOrganizationResolution(route.OrganizationAccessPolicy) {
		return nil
	}
	return routeAuthHandlersWithDependencies(route, routeAuthDependencies{zitadelAuth: zitadelAuth})
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
	if dependencies.zitadelAuth != nil {
		zitadelAuth := dependencies.zitadelAuth
		if requiresOrganization {
			zitadelAuth = workbenchAuthenticationMiddleware(zitadelAuth)
		}
		handlers = append(handlers, zitadelAuth)
	}
	if requiresOrganization {
		handlers = append(handlers, organizationResolutionMiddleware(route.OrganizationAccessPolicy, dependencies.organizationResolver))
	}
	var roleAuth gin.HandlerFunc
	if dependencies.roleMiddleware != nil {
		roleAuth = dependencies.roleMiddleware(route)
	} else {
		roleAuth = listingkithttpapi.NewRouteRoleMiddleware(route)
	}
	if roleAuth != nil {
		if requiresOrganization {
			roleAuth = workbenchRoleAuthorizationMiddleware(roleAuth)
		}
		handlers = append(handlers, roleAuth)
	}
	return handlers
}

func routeRequiresOrganizationResolution(policy httproute.OrganizationAccessPolicy) bool {
	return policy != "" && policy != httproute.OrganizationAccessPolicyNone
}

type bufferedGinResponseWriter struct {
	gin.ResponseWriter
	header http.Header
	body   bytes.Buffer
	status int
	wrote  bool
}

func newBufferedGinResponseWriter(writer gin.ResponseWriter) *bufferedGinResponseWriter {
	return &bufferedGinResponseWriter{
		ResponseWriter: writer,
		header:         writer.Header().Clone(),
		status:         http.StatusOK,
	}
}

func (writer *bufferedGinResponseWriter) Header() http.Header { return writer.header }

func (writer *bufferedGinResponseWriter) WriteHeader(status int) {
	if writer.wrote {
		return
	}
	writer.status = status
	writer.wrote = true
}

func (writer *bufferedGinResponseWriter) WriteHeaderNow() {
	if !writer.wrote {
		writer.WriteHeader(writer.status)
	}
}

func (writer *bufferedGinResponseWriter) Write(data []byte) (int, error) {
	writer.WriteHeaderNow()
	return writer.body.Write(data)
}

func (writer *bufferedGinResponseWriter) WriteString(data string) (int, error) {
	writer.WriteHeaderNow()
	return writer.body.WriteString(data)
}

func (writer *bufferedGinResponseWriter) Status() int { return writer.status }
func (writer *bufferedGinResponseWriter) Size() int   { return writer.body.Len() }
func (writer *bufferedGinResponseWriter) Written() bool {
	return writer.wrote
}
func (writer *bufferedGinResponseWriter) Flush() {}

func workbenchAuthenticationMiddleware(zitadelAuth gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		originalWriter := c.Writer
		bufferedWriter := newBufferedGinResponseWriter(originalWriter)
		c.Writer = bufferedWriter
		defer func() { c.Writer = originalWriter }()
		zitadelAuth(c)
		c.Writer = originalWriter

		if _, authenticated := authidentity.AuthenticatedIdentityFromContext(c.Request.Context()); !authenticated && c.IsAborted() {
			writeWorkbenchContextError(c, workbenchcontext.ErrAuthenticationRequired)
			return
		}
		commitBufferedResponse(originalWriter, bufferedWriter)
	}
}

func workbenchRoleAuthorizationMiddleware(roleAuth gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		originalWriter := c.Writer
		bufferedWriter := newBufferedGinResponseWriter(originalWriter)
		c.Writer = bufferedWriter
		defer func() { c.Writer = originalWriter }()
		roleAuth(c)
		c.Writer = originalWriter

		if c.IsAborted() && bufferedWriter.Status() == http.StatusForbidden {
			writeWorkbenchProtocolError(c, http.StatusForbidden, "PERMISSION_DENIED", "Permission is denied")
			return
		}
		commitBufferedResponse(originalWriter, bufferedWriter)
	}
}

func commitBufferedResponse(originalWriter gin.ResponseWriter, bufferedWriter *bufferedGinResponseWriter) {
	for key := range originalWriter.Header() {
		originalWriter.Header().Del(key)
	}
	for key, values := range bufferedWriter.Header() {
		originalWriter.Header()[key] = append([]string(nil), values...)
	}
	if bufferedWriter.Written() {
		originalWriter.WriteHeader(bufferedWriter.Status())
		_, _ = originalWriter.Write(bufferedWriter.body.Bytes())
	}
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

func newZitadelAuthMiddleware() gin.HandlerFunc {
	return listingkithttpapi.NewZitadelAuthMiddlewareFromEnv()
}
