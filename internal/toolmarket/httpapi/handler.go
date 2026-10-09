package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/gin-gonic/gin"
	"io"
	"mime"
	"net/http"
	strict "sigs.k8s.io/json"
	"strconv"
	"strings"
	"task-processor/internal/authz"
	"task-processor/internal/httproute"
	tm "task-processor/internal/toolmarket"
	"time"
)

const Base = "/api/v1/workbench/tool-market"
const AdminBase = "/api/v1/admin/tool-market"
const ModuleName = "tool-market"

type Handler struct {
	Repository tm.Repository
	Bind       func(context.Context, string) (context.Context, error)
	// Authorize resolves current trusted identity and fresh permission. Platform
	// requests use verified platform roles, without requiring customer membership.
	Authorize func(context.Context, string, bool) (tm.Scope, error)
	Readiness tm.Readiness
	plugin    []byte
}

// ConfigurePackage runs before serving startup; arbitrary unverified bytes are
// never an injection port. Invalid records leave the download unavailable.
func (h *Handler) ConfigurePackage(c tm.PackageConfig) error {
	h.plugin = nil
	raw, e := tm.LoadPackage(c)
	if e != nil {
		return e
	}
	h.plugin = raw
	return nil
}

// BuildRoutes checks the feature dependencies. Process configuration and the
// kernel Module wrapper belong to the existing application assembly layer.
func BuildRoutes(h *Handler) ([]httproute.Descriptor, error) {
	if h == nil || h.Repository == nil || h.Authorize == nil || h.Bind == nil {
		return nil, tm.ErrUnavailable
	}
	return Routes(h), nil
}

type spec struct {
	method, path, permission, operation string
	platform                            bool
}

func specs() []spec {
	return []spec{
		{"GET", Base + "/market", authz.PermissionWorkbenchToolsRead, "market", false},
		{"GET", Base + "/mine", authz.PermissionWorkbenchToolsRead, "mine", false},
		{"GET", Base + "/plugin", authz.PermissionWorkbenchToolsRead, "plugin", false},
		{"PUT", Base + "/activations/:id", authz.PermissionWorkbenchToolsManage, "activation", false},
		{"GET", Base + "/requests", authz.PermissionWorkbenchToolsRead, "requests", false},
		{"POST", Base + "/requests", authz.PermissionWorkbenchToolsCustomize, "create", false},
		{"GET", Base + "/requests/:id", authz.PermissionWorkbenchToolsRead, "detail", false},
		{"GET", AdminBase + "/requests", authz.PermissionListingKitPlatformAdm, "requests", true},
		{"GET", AdminBase + "/requests/:id", authz.PermissionListingKitPlatformAdm, "detail", true},
		{"POST", AdminBase + "/requests/:id/progress", authz.PermissionListingKitPlatformAdm, "progress", true},
	}
}
func Routes(h *Handler) []httproute.Descriptor {
	out := []httproute.Descriptor{}
	for _, s := range specs() {
		policy, org := httproute.AuthPolicyCurrentIdentity, httproute.OrganizationAccessPolicyCachedRead
		if s.method != "GET" {
			org = httproute.OrganizationAccessPolicyLiveWrite
		}
		if s.platform {
			policy, org = httproute.AuthPolicyCurrentIdentityWithVerifiedRoles, httproute.OrganizationAccessPolicyNone
		}
		out = append(out, httproute.Descriptor{Method: s.method, Path: s.path, Module: ModuleName, Permission: s.permission, AuthPolicy: policy, OrganizationAccessPolicy: org, RequestTimeout: 10 * time.Second, RejectUnreadRequestBody: s.method == "GET", Handler: httproute.WithRequestBodyReadTimeout(10*time.Second, func(c *gin.Context) {
			if h == nil || h.Repository == nil || h.Authorize == nil {
				failure(c, tm.ErrUnavailable)
				return
			}
			h.serve(c, s)
		})})
	}
	return out
}
func ValidateDescriptor(d httproute.Descriptor) error {
	for _, r := range Routes(nil) {
		if d.Method == r.Method && d.Path == r.Path && d.Module == r.Module && d.Permission == r.Permission && d.AuthPolicy == r.AuthPolicy && d.OrganizationAccessPolicy == r.OrganizationAccessPolicy && d.RequestTimeout == r.RequestTimeout && d.RejectUnreadRequestBody == r.RejectUnreadRequestBody && d.OrganizationTargetResolver == nil && d.Handler != nil {
			return nil
		}
	}
	return errors.New("tool market loses authorization boundary")
}
func failure(c *gin.Context, e error) {
	status, code := 503, tm.ErrUnavailable.Error()
	for _, v := range []error{tm.ErrInvalid, tm.ErrForbidden, tm.ErrNotFound, tm.ErrConflict, tm.ErrRevision, tm.ErrPrecondition} {
		if errors.Is(e, v) {
			code = v.Error()
			status = map[error]int{tm.ErrInvalid: 400, tm.ErrForbidden: 403, tm.ErrNotFound: 404, tm.ErrConflict: 409, tm.ErrRevision: 412, tm.ErrPrecondition: 428}[v]
			break
		}
	}
	c.Header("Cache-Control", "private, no-store")
	httproute.RejectUnreadRequestBody(c)
	c.AbortWithStatusJSON(status, gin.H{"code": code})
}
func reply(c *gin.Context, v any) {
	raw, e := json.Marshal(v)
	if e != nil || len(raw) > 256<<10 {
		failure(c, tm.ErrUnavailable)
		return
	}
	c.Header("Cache-Control", "private, no-store")
	c.Data(200, "application/json; charset=utf-8", raw)
}
func body(c *gin.Context, v any) error {
	media, _, e := mime.ParseMediaType(c.GetHeader("Content-Type"))
	if e != nil || media != "application/json" || c.GetHeader("Content-Encoding") != "" {
		return tm.ErrInvalid
	}
	raw, e := io.ReadAll(io.LimitReader(c.Request.Body, (16<<10)+1))
	if e != nil || len(raw) > 16<<10 || !strings.HasPrefix(strings.TrimSpace(string(raw)), "{") {
		return tm.ErrInvalid
	}
	violations, e := strict.UnmarshalStrict(raw, v)
	if e != nil || len(violations) > 0 {
		return tm.ErrInvalid
	}
	return nil
}
func precondition(c *gin.Context, cmd *tm.Command) error {
	keys := c.Request.Header.Values("Idempotency-Key")
	if len(keys) != 1 || !tm.UUID(keys[0]) {
		return tm.ErrInvalid
	}
	cmd.Key = keys[0]
	matches, absent := c.Request.Header.Values("If-Match"), c.Request.Header.Values("If-None-Match")
	if len(absent) > 0 {
		if len(absent) != 1 || absent[0] != "*" || len(matches) > 0 || cmd.Operation == "progress" {
			return tm.ErrInvalid
		}
		cmd.Absent = true
	} else if len(matches) > 0 {
		if len(matches) != 1 || cmd.Operation == "create" || len(matches[0]) < 3 || matches[0][0] != '"' || matches[0][len(matches[0])-1] != '"' {
			return tm.ErrInvalid
		}
		s := matches[0][1 : len(matches[0])-1]
		n, e := strconv.ParseInt(s, 10, 64)
		if e != nil || n <= 0 || strconv.FormatInt(n, 10) != s {
			return tm.ErrInvalid
		}
		cmd.Expected = n
	} else {
		return tm.ErrPrecondition
	}
	return nil
}
func (h *Handler) serve(c *gin.Context, s spec) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	if !s.platform && h.Bind != nil {
		values := c.Request.Header.Values("Authorization")
		if len(values) != 1 {
			failure(c, tm.ErrForbidden)
			return
		}
		bound, e := h.Bind(ctx, values[0])
		if e != nil {
			failure(c, e)
			return
		}
		ctx = bound
	}
	scope, e := h.Authorize(ctx, s.permission, s.platform)
	if e != nil {
		failure(c, e)
		return
	}
	if !scope.Valid(s.platform) {
		failure(c, tm.ErrForbidden)
		return
	}
	q := c.Request.URL.Query()
	limit := 20
	cursor := ""
	for key, vs := range q {
		if s.operation != "requests" || len(vs) != 1 {
			failure(c, tm.ErrInvalid)
			return
		}
		switch key {
		case "cursor":
			cursor = vs[0]
			if !tm.UUID(cursor) {
				failure(c, tm.ErrInvalid)
				return
			}
		case "pageSize":
			n, e := strconv.Atoi(vs[0])
			if e != nil || n < 1 || n > 50 || strconv.Itoa(n) != vs[0] {
				failure(c, tm.ErrInvalid)
				return
			}
			limit = n
		default:
			failure(c, tm.ErrInvalid)
			return
		}
	}
	if s.method == http.MethodGet {
		if c.Request.ContentLength != 0 || len(c.Request.TransferEncoding) > 0 {
			failure(c, tm.ErrInvalid)
			return
		}
		switch s.operation {
		case "market", "mine", "plugin":
			activations, e := h.Repository.Activations(ctx, scope)
			if e != nil {
				failure(c, e)
				return
			}
			ready := h.Readiness
			ready.Download = ready.LocalCapture && len(h.plugin) > 0
			if s.operation == "plugin" {
				enabled := false
				for _, a := range activations {
					if a.ToolID == tm.AcquisitionID && a.Enabled {
						enabled = true
					}
				}
				if !enabled || !ready.Download {
					failure(c, tm.ErrUnavailable)
					return
				}
				c.Header("Cache-Control", "private, no-store")
				c.Header("Content-Disposition", `attachment; filename="shuomi-1688-capture.zip"`)
				c.Header("X-Content-Type-Options", "nosniff")
				c.Data(200, "application/zip", h.plugin)
				return
			}
			manage, em := h.Authorize(ctx, authz.PermissionWorkbenchToolsManage, false)
			customize, ec := h.Authorize(ctx, authz.PermissionWorkbenchToolsCustomize, false)
			reply(c, gin.H{"tools": tm.Catalog(ready, activations, s.operation == "mine"), "canManage": em == nil && manage == scope, "canCustomize": ec == nil && customize == scope})
		case "requests":
			v, e := h.Repository.Requests(ctx, scope, s.platform, cursor, limit)
			if e != nil {
				failure(c, e)
				return
			}
			reply(c, v)
		case "detail":
			v, e := h.Repository.Detail(ctx, scope, s.platform, c.Param("id"))
			if e != nil {
				failure(c, e)
				return
			}
			reply(c, v)
		}
		return
	}
	cmd := tm.Command{Scope: scope, Platform: s.platform, Operation: s.operation, ID: c.Param("id")}
	if e := precondition(c, &cmd); e != nil {
		failure(c, e)
		return
	}
	switch s.operation {
	case "activation":
		var input struct {
			Enabled *bool `json:"enabled"`
		}
		e = body(c, &input)
		if e == nil {
			if input.Enabled == nil {
				e = tm.ErrInvalid
			} else {
				cmd.Enabled = *input.Enabled
			}
		}
	case "create":
		e = body(c, &cmd.Demand)
	case "progress":
		e = body(c, &cmd.Progress)
	}
	if e != nil {
		failure(c, e)
		return
	}
	if !cmd.Valid() {
		failure(c, tm.ErrInvalid)
		return
	}
	guard := func(ctx context.Context) error {
		current, e := h.Authorize(ctx, s.permission, s.platform)
		if e != nil {
			return e
		}
		if current != scope {
			return tm.ErrForbidden
		}
		return nil
	}
	beforeApply := func(context.Context) error {
		if cmd.Operation == "activation" && cmd.Enabled && !h.Readiness.LocalCapture && !h.Readiness.OnlineCapture {
			return tm.ErrUnavailable
		}
		return nil
	}
	v, e := h.Repository.Execute(ctx, cmd, guard, beforeApply)
	if e != nil {
		failure(c, e)
		return
	}
	reply(c, v)
}
