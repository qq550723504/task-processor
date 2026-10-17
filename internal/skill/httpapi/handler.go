package skillhttp

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
	"task-processor/internal/agentconfig"
	"task-processor/internal/authz"
	"task-processor/internal/core/config"
	"task-processor/internal/httproute"
	kernelmodule "task-processor/internal/kernel/module"
	d "task-processor/internal/skill"
	"time"
)

const Base = "/api/v1/workbench/skills"

type Handler struct {
	Service *d.Service
	Policy  agentconfig.SkillAdmissionRepository
	Bind    func(context.Context, string, bool) (context.Context, error)
}
type routeSpec struct{ method, path, scope, operation string }

func specs() []routeSpec {
	out := []routeSpec{}
	for _, scope := range []string{"personal", "organization", "official"} {
		for _, item := range []routeSpec{{"GET", "", "", "list"}, {"GET", "/info", "", "info"}, {"POST", "", "", "create"}, {"POST", "/validate", "", "validate"}, {"POST", "/copy", "", "copy"}, {"GET", "/:skill_id/versions", "", "versions"}, {"POST", "/:skill_id/versions", "", "save-version"}, {"GET", "/:skill_id/versions/:version", "", "read"}, {"GET", "/:skill_id/versions/:version/export", "", "export"}, {"POST", "/:skill_id/versions/:version/:action", "", "lifecycle"}} {
			item.path = Base + "/" + scope + item.path
			item.scope = scope
			out = append(out, item)
		}
	}
	out = append(out, routeSpec{"POST", Base + "/official/install", "official", "install-official"})
	return append(out, routeSpec{"GET", Base + "/personal/shares", "personal", "shares"}, routeSpec{"POST", Base + "/personal/shares/:share_id/withdraw", "personal", "withdraw"}, routeSpec{"GET", Base + "/organization/shares", "organization", "shares"}, routeSpec{"POST", Base + "/organization/shares", "organization", "share"}, routeSpec{"GET", Base + "/organization/shares/:share_id", "organization", "read-shared"}, routeSpec{"POST", Base + "/organization/shares/:share_id/:action", "organization", "decide-share"}, routeSpec{"GET", Base + "/organization/candidates", "organization", "candidates"}, routeSpec{"GET", Base + "/organization/admission", "organization", "read-admission"}, routeSpec{"PUT", Base + "/organization/admission", "organization", "set-admission"})
}
func Routes(h *Handler) []httproute.Descriptor {
	out := []httproute.Descriptor{}
	for _, spec := range specs() {
		authPolicy, orgPolicy, permission := httproute.AuthPolicyCurrentIdentity, httproute.OrganizationAccessPolicyNone, ""
		if spec.scope == "official" {
			authPolicy = httproute.AuthPolicyCurrentIdentityWithVerifiedRoles
			if spec.method != "GET" {
				permission = authz.PermissionListingKitPlatformAdm
			}
		}
		if spec.scope == "organization" {
			orgPolicy = httproute.OrganizationAccessPolicyLiveWrite
			if spec.method != "GET" {
				permission = authz.PermissionWorkbenchAgentUse
			}
			if spec.operation == "lifecycle" || spec.operation == "set-admission" || spec.operation == "decide-share" || spec.operation == "read-shared" || spec.operation == "shares" {
				permission = authz.PermissionWorkbenchAgentConfigure
			}
			if spec.operation == "candidates" {
				permission = authz.PermissionWorkbenchAgentUse
			}
		}
		out = append(out, httproute.Descriptor{Method: spec.method, Path: spec.path, Module: "skills", AuthPolicy: authPolicy, OrganizationAccessPolicy: orgPolicy, Permission: permission, RequestTimeout: 10 * time.Second, RejectUnreadRequestBody: spec.method == "GET", Handler: func(c *gin.Context) {
			if h == nil {
				Failure(c, d.ErrUnavailable)
				return
			}
			h.serve(c, spec)
		}})
	}
	return out
}
func ValidateDescriptor(actual httproute.Descriptor) error {
	for _, expected := range Routes(nil) {
		if actual.Method == expected.Method && actual.Path == expected.Path && actual.Module == expected.Module && actual.AuthPolicy == expected.AuthPolicy && actual.OrganizationAccessPolicy == expected.OrganizationAccessPolicy && actual.Permission == expected.Permission && actual.RequestTimeout == expected.RequestTimeout && actual.RejectUnreadRequestBody == expected.RejectUnreadRequestBody && actual.OrganizationTargetResolver == nil && actual.Handler != nil {
			return nil
		}
	}
	return errors.New("skill route loses its admitted identity, scope, action or deadline")
}

type Module struct{ Handler *Handler }

func (Module) Name() string                { return "skills" }
func (Module) Enabled(*config.Config) bool { return true }
func (m Module) Register(r *kernelmodule.Registry) error {
	r.AddRoutes(Routes(m.Handler)...)
	return nil
}
func Failure(c *gin.Context, e error) {
	status, code := 503, d.ErrUnavailable.Error()
	for _, item := range []struct {
		err    error
		status int
	}{{d.ErrInvalid, 400}, {d.ErrUnsupported, 400}, {d.ErrForbidden, 403}, {d.ErrNotFound, 404}, {d.ErrConflict, 409}, {d.ErrRevision, 412}, {d.ErrNotEnabled, 409}, {agentconfig.ErrInvalid, 400}, {agentconfig.ErrForbidden, 403}, {agentconfig.ErrNotFound, 404}, {agentconfig.ErrConflict, 409}, {agentconfig.ErrRevision, 412}, {agentconfig.ErrNotEnabled, 409}} {
		if errors.Is(e, item.err) {
			status, code = item.status, item.err.Error()
			break
		}
	}
	c.Header("Cache-Control", "private, no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	c.AbortWithStatusJSON(status, gin.H{"code": code})
}
func reply(c *gin.Context, value any, revision uint64) {
	raw, e := json.Marshal(value)
	if e != nil || len(raw) > 256<<10 {
		Failure(c, d.ErrUnavailable)
		return
	}
	c.Header("Cache-Control", "private, no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	if revision > 0 {
		c.Header("ETag", `"`+strconv.FormatUint(revision, 10)+`"`)
	}
	c.Data(200, "application/json; charset=utf-8", raw)
}
func body(c *gin.Context, out any) error {
	media, _, e := mime.ParseMediaType(c.GetHeader("Content-Type"))
	if e != nil || media != "application/json" || c.GetHeader("Content-Encoding") != "" {
		return d.ErrInvalid
	}
	raw, e := io.ReadAll(io.LimitReader(c.Request.Body, d.MaxRequestBytes+1))
	if e != nil || len(raw) > d.MaxRequestBytes || !strings.HasPrefix(strings.TrimSpace(string(raw)), "{") {
		return d.ErrInvalid
	}
	problems, e := strict.UnmarshalStrict(raw, out)
	if e != nil || len(problems) > 0 {
		return d.ErrInvalid
	}
	return nil
}
func number(raw string, zero bool) (uint64, error) {
	n, e := strconv.ParseUint(raw, 10, 63)
	if e != nil || !zero && n == 0 || strconv.FormatUint(n, 10) != raw {
		return 0, d.ErrInvalid
	}
	return n, nil
}
func key(c *gin.Context) (string, error) {
	keys := c.Request.Header.Values("Idempotency-Key")
	if len(keys) != 1 || !agentconfig.UUID(keys[0]) {
		return "", d.ErrInvalid
	}
	return keys[0], nil
}
func expected(c *gin.Context, zero bool) (uint64, error) {
	values := c.Request.Header.Values("If-Match")
	if len(values) != 1 || len(values[0]) < 3 || values[0][0] != '"' || values[0][len(values[0])-1] != '"' {
		return 0, d.ErrInvalid
	}
	return number(values[0][1:len(values[0])-1], zero)
}
func project(v d.Version, p d.Principal) gin.H {
	return gin.H{"version": v, "canEdit": d.Authorized(p, v, d.Edit), "canPublish": d.Authorized(p, v, d.Publish), "canUse": d.Authorized(p, v, d.Use), "canCopy": d.Authorized(p, v, d.Copy), "canExport": d.Authorized(p, v, d.Export)}
}
func (h *Handler) serve(c *gin.Context, s routeSpec) {
	if s.operation == "info" && h.Service == nil {
		reply(c, gin.H{"available": false, "canCreate": false, "canInstallOfficial": false, "reason": d.ErrUnavailable.Error()}, 0)
		return
	}
	if h.Service == nil {
		Failure(c, d.ErrUnavailable)
		return
	}
	ctx := c.Request.Context()
	if h.Bind != nil {
		bound, e := h.Bind(ctx, c.GetHeader("Authorization"), s.scope == "organization")
		if e != nil {
			Failure(c, e)
			return
		}
		ctx = bound
		c.Request = c.Request.WithContext(ctx)
	}
	if c.Request.URL.ForceQuery {
		Failure(c, d.ErrInvalid)
		return
	}
	if s.operation != "list" && s.operation != "candidates" && c.Request.URL.RawQuery != "" {
		Failure(c, d.ErrInvalid)
		return
	}
	if s.method == "GET" && (c.Request.Body != nil && c.Request.Body != http.NoBody || c.Request.ContentLength > 0 || len(c.Request.TransferEncoding) > 0) {
		Failure(c, d.ErrInvalid)
		return
	}
	switch s.operation {
	case "info":
		p, e := h.Service.Principal(ctx)
		if e != nil {
			Failure(c, e)
			return
		}
		canCreate := s.scope == "personal" || s.scope == "organization" && (p.UseOrganization || p.ConfigureOrganization) || s.scope == "official" && p.PlatformMaintainer
		reply(c, gin.H{"available": true, "canCreate": canCreate, "canInstallOfficial": s.scope == "official" && p.PlatformMaintainer, "reason": ""}, 0)
	case "install-official":
		var input struct{}
		if err := body(c, &input); err != nil {
			Failure(c, err)
			return
		}
		if _, err := key(c); err != nil {
			Failure(c, err)
			return
		}
		result, err := h.Service.InstallOfficial(ctx)
		if err != nil {
			Failure(c, err)
			return
		}
		reply(c, result, 0)
	case "list":
		size := 20
		q := c.Request.URL.Query()
		for name, values := range q {
			if (name != "cursor" && name != "limit") || len(values) != 1 {
				Failure(c, d.ErrInvalid)
				return
			}
		}
		if value := q.Get("limit"); value != "" {
			n, e := number(value, false)
			if e != nil || n > 50 {
				Failure(c, d.ErrInvalid)
				return
			}
			size = int(n)
		}
		items, next, p, e := h.Service.List(ctx, s.scope, q.Get("cursor"), size)
		if e != nil {
			Failure(c, e)
			return
		}
		out := []gin.H{}
		for _, v := range items {
			out = append(out, project(v, p))
		}
		reply(c, gin.H{"items": out, "nextCursor": next}, 0)
	case "read", "export", "versions":
		n := uint64(0)
		var e error
		if s.operation != "versions" {
			n, e = number(c.Param("version"), false)
			if e != nil {
				Failure(c, e)
				return
			}
		}
		if s.operation == "export" {
			pack, e := h.Service.Export(ctx, s.scope, c.Param("skill_id"), n)
			if e != nil {
				Failure(c, e)
				return
			}
			reply(c, pack, 0)
			return
		}
		if s.operation == "versions" {
			items, e := h.Service.Versions(ctx, s.scope, c.Param("skill_id"))
			if e != nil {
				Failure(c, e)
				return
			}
			reply(c, gin.H{"items": items}, 0)
			return
		}
		v, p, e := h.Service.Read(ctx, s.scope, c.Param("skill_id"), n)
		if e != nil {
			Failure(c, e)
			return
		}
		reply(c, project(v, p), v.Metadata.Revision)
	case "create", "save-version", "validate", "copy", "lifecycle":
		h.command(c, s)
	case "shares", "share", "read-shared", "decide-share", "withdraw":
		h.share(c, s)
	case "candidates":
		query := c.Request.URL.Query()
		for name, values := range query {
			if name != "target" || len(values) != 1 {
				Failure(c, d.ErrInvalid)
				return
			}
		}
		target := d.TitleTarget
		if values, ok := query["target"]; ok {
			target = values[0]
		}
		items, e := h.Service.CandidatesForTarget(ctx, target)
		if e != nil {
			Failure(c, e)
			return
		}
		reply(c, gin.H{"items": items}, 0)
	case "read-admission", "set-admission":
		h.admission(c, s)
	default:
		Failure(c, d.ErrInvalid)
	}
}
