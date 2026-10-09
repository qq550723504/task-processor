package httpapi

import (
	"context"
	"errors"
	"github.com/gin-gonic/gin"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	d "task-processor/internal/agentcustomization"
	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	"task-processor/internal/httproute"
	"time"
)

const Base = "/api/v1/agent-customization/requests"
const AdminBase = "/api/v1/admin/agent-customization/requests"
const PrivateBase = "/api/v1/agent-customization/agents"

type ServicePort interface {
	Execute(context.Context, d.Command) (d.Receipt, error)
	List(context.Context, d.Scope, string) (d.Page, error)
	Read(context.Context, d.Scope, string, int64) (d.Detail, error)
	Download(context.Context, d.Scope, string, string) (d.Attachment, []byte, error)
}
type Handler struct{ Service ServicePort }

func NewHandler(s ServicePort) (*Handler, error) {
	if s == nil {
		return nil, d.ErrUnavailable
	}
	return &Handler{Service: s}, nil
}
func Routes(h *Handler) []httproute.Descriptor {
	var out []httproute.Descriptor
	for _, admin := range []bool{false, true} {
		base := Base
		policy := httproute.AuthPolicyCurrentIdentity
		org := httproute.OrganizationAccessPolicyLiveWrite
		if admin {
			base = AdminBase
			policy = httproute.AuthPolicyCurrentIdentityWithVerifiedRoles
			org = httproute.OrganizationAccessPolicyNone
		}
		for _, s := range []struct{ method, path, operation string }{{"GET", "", "list"}, {"GET", "/:id", "read"}, {"GET", "/:id/files/:file", "file"}, {"POST", "", "submit"}, {"POST", "/:id/progress", "progress"}} {
			if admin && s.operation == "submit" || !admin && s.operation == "progress" {
				continue
			}
			permission := authz.PermissionWorkbenchAgentRead
			if s.operation == "submit" {
				permission = authz.PermissionWorkbenchAgentUse
			}
			if admin {
				permission = authz.PermissionListingKitPlatformAdm
			}
			out = append(out, httproute.Descriptor{Method: s.method, Path: base + s.path, Module: "agent-customization", Permission: permission, AuthPolicy: policy, OrganizationAccessPolicy: org, RequestTimeout: 30 * time.Second, RejectUnreadRequestBody: s.method == "GET", Handler: httproute.WithRequestBodyReadTimeout(30*time.Second, func(c *gin.Context) {
				if h == nil || h.Service == nil {
					failure(c, d.ErrUnavailable)
					return
				}
				h.serve(c, admin, s.operation)
			})})
		}
	}
	for _, r := range []struct{ method, path, operation string }{{"GET", "", "deliveries"}, {"GET", "/:id", "delivery"}, {"GET", "/:id/reports", "reports"}, {"POST", "/:id/reports", "run"}} {
		permission := authz.PermissionWorkbenchAgentRead
		if r.operation == "run" {
			permission = authz.PermissionWorkbenchAgentUse
		}
		out = append(out, httproute.Descriptor{Method: r.method, Path: PrivateBase + r.path, Module: "agent-customization", Permission: permission, AuthPolicy: httproute.AuthPolicyCurrentIdentity, OrganizationAccessPolicy: httproute.OrganizationAccessPolicyLiveWrite, RequestTimeout: 30 * time.Second, RejectUnreadRequestBody: r.method == "GET", Handler: httproute.WithRequestBodyReadTimeout(30*time.Second, func(c *gin.Context) {
			if h == nil || h.Service == nil {
				failure(c, d.ErrUnavailable)
				return
			}
			h.serve(c, false, r.operation)
		})})
	}
	return out
}
func ValidateDescriptor(v httproute.Descriptor) error {
	for _, r := range Routes(nil) {
		if r.Method == v.Method && r.Path == v.Path && r.Module == v.Module && r.Permission == v.Permission && r.AuthPolicy == v.AuthPolicy && r.OrganizationAccessPolicy == v.OrganizationAccessPolicy && r.RequestTimeout == v.RequestTimeout && r.RejectUnreadRequestBody == v.RejectUnreadRequestBody && v.OrganizationTargetResolver == nil && v.Handler != nil {
			return nil
		}
	}
	return d.ErrForbidden
}
func failure(c *gin.Context, e error) {
	status, code := 503, d.ErrUnavailable.Error()
	for _, known := range []error{d.ErrInvalid, d.ErrForbidden, d.ErrNotFound, d.ErrConflict, d.ErrRevision} {
		if errors.Is(e, known) {
			code = known.Error()
			switch known {
			case d.ErrInvalid:
				status = 400
			case d.ErrForbidden:
				status = 403
			case d.ErrNotFound:
				status = 404
			case d.ErrConflict:
				status = 409
			case d.ErrRevision:
				status = 412
			}
			break
		}
	}
	c.AbortWithStatusJSON(status, gin.H{"code": code})
}
func (h *Handler) serve(c *gin.Context, platform bool, operation string) {
	c.Header("Cache-Control", "private, no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	if c.Request.Method == http.MethodGet && (c.Request.ContentLength != 0 || len(c.Request.TransferEncoding) > 0) {
		httproute.RejectUnreadRequestBody(c)
		failure(c, d.ErrInvalid)
		return
	}
	identity, ok := authidentity.AuthenticatedIdentityFromContext(c.Request.Context())
	if !ok {
		failure(c, d.ErrForbidden)
		return
	}
	scope := d.Scope{ActorID: identity.UserID, Platform: platform}
	if !platform {
		scope.OrganizationID = identity.EffectiveOrganizationID
	}
	if !d.ValidScope(scope) {
		failure(c, d.ErrForbidden)
		return
	}
	if operation == "deliveries" || operation == "delivery" || operation == "reports" || operation == "run" {
		h.servePrivate(c, scope, operation)
		return
	}
	query, e := url.ParseQuery(c.Request.URL.RawQuery)
	if e != nil || len(c.Request.URL.RawQuery) > 256 || c.Request.URL.ForceQuery {
		failure(c, d.ErrInvalid)
		return
	}
	var cursor string
	var after int64
	for k, values := range query {
		if len(values) != 1 {
			failure(c, d.ErrInvalid)
			return
		}
		if operation == "list" && k == "cursor" && d.UUID(values[0]) {
			cursor = values[0]
		} else if operation == "read" && k == "after" {
			after, e = strconv.ParseInt(values[0], 10, 64)
			if e != nil || after < 1 || strconv.FormatInt(after, 10) != values[0] {
				failure(c, d.ErrInvalid)
				return
			}
		} else {
			failure(c, d.ErrInvalid)
			return
		}
	}
	if c.Request.Method == "GET" {
		switch operation {
		case "list":
			v, e := h.Service.List(c.Request.Context(), scope, cursor)
			if e != nil {
				failure(c, e)
				return
			}
			c.JSON(200, v)
		case "read":
			v, e := h.Service.Read(c.Request.Context(), scope, c.Param("id"), after)
			if e != nil {
				failure(c, e)
				return
			}
			c.JSON(200, v)
		case "file":
			meta, data, e := h.Service.Download(c.Request.Context(), scope, c.Param("id"), c.Param("file"))
			if e != nil {
				failure(c, e)
				return
			}
			c.Header("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": meta.Name}))
			c.Data(200, meta.ContentType, data)
		}
		return
	}
	if len(query) > 0 || c.GetHeader("Content-Encoding") != "" || len(c.Request.Header.Values("Content-Type")) != 1 || c.GetHeader("If-None-Match") != "" {
		failure(c, d.ErrInvalid)
		return
	}
	media, params, e := mime.ParseMediaType(c.GetHeader("Content-Type"))
	if e != nil || media != "application/json" || len(params) > 1 || params["charset"] != "" && !strings.EqualFold(params["charset"], "utf-8") {
		failure(c, d.ErrInvalid)
		return
	}
	keys := c.Request.Header.Values("Idempotency-Key")
	if len(keys) != 1 || !d.UUID(keys[0]) {
		failure(c, d.ErrInvalid)
		return
	}
	cmd := d.Command{Scope: scope, Key: keys[0], Operation: operation}
	limit := int64(9 << 20)
	if operation == "progress" {
		cmd.ID = c.Param("id")
		matches := c.Request.Header.Values("If-Match")
		if len(matches) != 1 || len(matches[0]) < 3 || !strings.HasPrefix(matches[0], `"`) || !strings.HasSuffix(matches[0], `"`) {
			failure(c, d.ErrInvalid)
			return
		}
		value := matches[0][1 : len(matches[0])-1]
		cmd.Expected, e = strconv.ParseInt(value, 10, 64)
		if e != nil || cmd.Expected < 1 || strconv.FormatInt(cmd.Expected, 10) != value {
			failure(c, d.ErrInvalid)
			return
		}
		limit = 64 << 10
	} else if len(c.Request.Header.Values("If-Match")) > 0 {
		failure(c, d.ErrInvalid)
		return
	}
	raw, e := io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, limit))
	if e != nil {
		failure(c, d.ErrInvalid)
		return
	}
	var target any = &cmd.Input
	if operation == "progress" {
		target = &cmd.Update
	}
	if !strings.HasPrefix(strings.TrimSpace(string(raw)), "{") || httproute.DecodeJSON(raw, target, int(limit), true) != nil {
		failure(c, d.ErrInvalid)
		return
	}
	receipt, e := h.Service.Execute(c.Request.Context(), cmd)
	if e != nil {
		failure(c, e)
		return
	}
	c.JSON(200, receipt)
}
