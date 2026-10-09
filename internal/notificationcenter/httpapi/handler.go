package notificationhttp

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	sigjson "sigs.k8s.io/json"
	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	"task-processor/internal/httproute"
	n "task-processor/internal/notificationcenter"
)

const Module = "notification-center"

type Handler struct {
	Service        *n.Service
	Realm          string
	PrepareContext func(context.Context, string) (context.Context, error)
}

func Routes(h *Handler) []httproute.Descriptor {
	var routes []httproute.Descriptor
	for _, base := range []string{"/api/v1/notifications/official", "/api/v1/notifications/personal", "/api/v1/workbench/notifications"} {
		org := httproute.OrganizationAccessPolicyNone
		if base == "/api/v1/workbench/notifications" {
			org = httproute.OrganizationAccessPolicyLiveWrite
		}
		for _, entry := range []struct{ method, path, action string }{{"GET", "", "list"}, {"GET", "/:notification_ref", "detail"}, {"POST", "/read", "read"}, {"POST", "/snapshot", "snapshot"}, {"POST", "/read-all", "read-all"}, {"GET", "/commands/:command_id", "command"}} {
			base, action := base, entry.action
			var handler gin.HandlerFunc = func(c *gin.Context) { h.handle(c, base, action) }
			if entry.method == "POST" {
				handler = httproute.WithRequestBodyReadTimeout(5*time.Second, handler)
			}
			routes = append(routes, httproute.Descriptor{Method: entry.method, Path: base + entry.path, Module: Module, AuthPolicy: httproute.AuthPolicyCurrentIdentity, OrganizationAccessPolicy: org, RequestTimeout: 10 * time.Second, RejectUnreadRequestBody: entry.method == "GET", Handler: handler})
		}
	}
	for _, entry := range []struct{ path, action string }{{"/api/v1/platform/notifications/official", "publish"}, {"/api/v1/platform/notifications/official/:id/withdraw", "withdraw"}} {
		action := entry.action
		routes = append(routes, httproute.Descriptor{Method: "POST", Path: entry.path, Module: Module, Permission: authz.PermissionListingKitPlatformAdm, AuthPolicy: httproute.AuthPolicyCurrentIdentityWithVerifiedRoles, OrganizationAccessPolicy: httproute.OrganizationAccessPolicyNone, RequestTimeout: 10 * time.Second, Handler: httproute.WithRequestBodyReadTimeout(5*time.Second, func(c *gin.Context) { h.handle(c, "/api/v1/notifications/official", action) })})
	}
	return routes
}
func ValidateDescriptor(r httproute.Descriptor) error {
	for _, expected := range Routes(nil) {
		if expected.Method == r.Method && expected.Path == r.Path {
			if r.Module != Module || r.Handler == nil || r.AuthPolicy != expected.AuthPolicy || r.OrganizationAccessPolicy != expected.OrganizationAccessPolicy || r.Permission != expected.Permission || r.RequestTimeout != expected.RequestTimeout || r.RejectUnreadRequestBody != expected.RejectUnreadRequestBody || r.OrganizationTargetResolver != nil {
				return n.ErrForbidden
			}
			return nil
		}
	}
	return n.ErrForbidden
}
func (h *Handler) scope(c *gin.Context, base string) (n.Scope, error) {
	identity, ok := authidentity.AuthenticatedIdentityFromContext(c.Request.Context())
	if !ok || !authidentity.IsBoundedIdentifier(identity.UserID) || identity.TokenExpiresAt.IsZero() || !time.Now().Before(identity.TokenExpiresAt) {
		return n.Scope{}, n.ErrForbidden
	}
	s := n.Scope{Realm: h.Realm, Subject: identity.UserID, Category: n.Business}
	if base == "/api/v1/notifications/official" {
		s.Category = n.Official
	}
	if base == "/api/v1/workbench/notifications" {
		s.OrganizationID = identity.TenantID
		if s.OrganizationID == "" {
			return s, n.ErrForbidden
		}
	}
	if !n.ValidScope(s) {
		return s, n.ErrForbidden
	}
	return s, nil
}
func readJSON(c *gin.Context, v any, max int64) error {
	media, params, e := mime.ParseMediaType(c.GetHeader("Content-Type"))
	if e != nil || media != "application/json" || (params["charset"] != "" && !strings.EqualFold(params["charset"], "utf-8")) || c.GetHeader("Content-Encoding") != "" || c.Request.Body == nil {
		return n.ErrInvalid
	}
	raw, e := io.ReadAll(io.LimitReader(c.Request.Body, max+1))
	if e != nil || len(raw) == 0 || int64(len(raw)) > max || !utf8.Valid(raw) {
		return n.ErrInvalid
	}
	violations, e := sigjson.UnmarshalStrict(raw, v, sigjson.DisallowDuplicateFields, sigjson.DisallowUnknownFields)
	if e != nil || len(violations) != 0 {
		return n.ErrInvalid
	}
	return nil
}
func key(c *gin.Context) (string, error) {
	v := c.Request.Header.Values("Idempotency-Key")
	if len(v) != 1 || !n.ValidKey(v[0]) {
		return "", n.ErrInvalid
	}
	return v[0], nil
}
func write(c *gin.Context, value any, e error) {
	c.Header("Cache-Control", "no-store")
	if e != nil {
		status, code := 503, "NOTIFICATION_UNAVAILABLE"
		switch {
		case errors.Is(e, n.ErrInvalid):
			status, code = 400, "INVALID_REQUEST"
		case errors.Is(e, n.ErrForbidden):
			status, code = 403, "FORBIDDEN"
		case errors.Is(e, n.ErrNotFound):
			status, code = 404, "NOT_FOUND"
		case errors.Is(e, n.ErrConflict):
			status, code = 409, "IDEMPOTENCY_CONFLICT"
		case errors.Is(e, n.ErrStale):
			status, code = 409, "STALE_SNAPSHOT"
		case errors.Is(e, n.ErrCapacity):
			status, code = 422, "COLLECTION_LIMIT"
		}
		c.JSON(status, gin.H{"code": code})
		return
	}
	raw, e := json.Marshal(value)
	if e != nil || len(raw) > 128<<10 {
		write(c, nil, n.ErrCapacity)
		return
	}
	c.Data(200, "application/json; charset=utf-8", raw)
}
func (h *Handler) handle(c *gin.Context, base, action string) {
	if h == nil || h.Service == nil {
		write(c, nil, n.ErrUnavailable)
		return
	}
	scope, e := h.scope(c, base)
	if e != nil {
		write(c, nil, e)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	if h.PrepareContext != nil {
		ctx, e = h.PrepareContext(ctx, c.GetHeader("Authorization"))
		if e != nil {
			write(c, nil, n.ErrForbidden)
			return
		}
	}
	if action == "list" {
		q := c.Request.URL.Query()
		for name, values := range q {
			if (name != "filter" && name != "after" && name != "limit") || len(values) != 1 {
				write(c, nil, n.ErrInvalid)
				return
			}
		}
		limit := 20
		if c.Query("limit") != "" {
			limit, e = strconv.Atoi(c.Query("limit"))
			if e != nil || strconv.Itoa(limit) != c.Query("limit") {
				write(c, nil, n.ErrInvalid)
				return
			}
		}
		v, e := h.Service.List(ctx, scope, n.ListRequest{Filter: c.Query("filter"), After: c.Query("after"), Limit: limit})
		write(c, v, e)
		return
	}
	if c.Request.URL.RawQuery != "" || c.Request.URL.ForceQuery {
		write(c, nil, n.ErrInvalid)
		return
	}
	if action == "detail" {
		v, e := h.Service.Detail(ctx, scope, c.Param("notification_ref"))
		write(c, v, e)
		return
	}
	if action == "command" {
		v, e := h.Service.Repository.Command(ctx, scope, c.Param("command_id"))
		write(c, v, e)
		return
	}
	commandKey, e := key(c)
	if e != nil {
		write(c, nil, e)
		return
	}
	switch action {
	case "read":
		var b struct {
			Ref string `json:"ref"`
		}
		if e = readJSON(c, &b, 16<<10); e != nil {
			write(c, nil, e)
			return
		}
		v, e := h.Service.Read(ctx, scope, commandKey, b.Ref)
		write(c, v, e)
	case "snapshot":
		var b struct{}
		if e = readJSON(c, &b, 16<<10); e != nil {
			write(c, nil, e)
			return
		}
		v, e := h.Service.PrepareReadAll(ctx, scope, commandKey)
		write(c, v, e)
	case "read-all":
		var b struct {
			ID          string `json:"id"`
			Fingerprint string `json:"fingerprint"`
		}
		if e = readJSON(c, &b, 16<<10); e != nil {
			write(c, nil, e)
			return
		}
		v, e := h.Service.ReadAll(ctx, scope, commandKey, b.ID, b.Fingerprint)
		write(c, v, e)
	case "publish":
		var b n.AnnouncementInput
		if e = readJSON(c, &b, 32<<10); e != nil {
			write(c, nil, e)
			return
		}
		v, e := h.Service.Publish(ctx, scope, commandKey, b)
		write(c, v, e)
	case "withdraw":
		var b struct {
			Revision int64 `json:"revision"`
		}
		if e = readJSON(c, &b, 16<<10); e != nil {
			write(c, nil, e)
			return
		}
		v, e := h.Service.Withdraw(ctx, scope, commandKey, c.Param("id"), b.Revision)
		write(c, v, e)
	default:
		write(c, nil, n.ErrInvalid)
	}
}
