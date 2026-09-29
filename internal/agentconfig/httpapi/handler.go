package httpapi

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"github.com/gin-gonic/gin"
	"io"
	"mime"
	"net/url"
	strict "sigs.k8s.io/json"
	"strconv"
	"strings"
	"task-processor/internal/agent"
	"task-processor/internal/agentconfig"
	"task-processor/internal/authz"
	"task-processor/internal/core/config"
	"task-processor/internal/httproute"
	kernelmodule "task-processor/internal/kernel/module"
	"time"
)

const Base = "/api/v1/workbench/agents"
const Module = "agent-configuration"

type Handler struct {
	Bind         func(context.Context, string) (context.Context, error)
	Repository   agentconfig.Repository
	Catalog      agentconfig.CatalogReader
	Authorize    func(context.Context, ...string) (agent.Scope, error)
	Knowledge    func(context.Context, agent.Scope, string) error
	Capabilities func(context.Context, agentconfig.CatalogEntry) []agentconfig.Capability
	Recent       func(context.Context, agent.Scope, string, string, int) (any, string, error)
}
type routeModule struct{ h *Handler }

func NewModule(h *Handler) kernelmodule.Module    { return routeModule{h} }
func (routeModule) Name() string                  { return Module }
func (m routeModule) Enabled(*config.Config) bool { return m.h != nil }
func (m routeModule) Register(r *kernelmodule.Registry) error {
	r.AddRoutes(Routes(m.h)...)
	return nil
}

type spec struct{ method, path, operation string }

var specs = []spec{{"GET", "/market", "market"}, {"GET", "/mine", "mine"}, {"GET", "/:agent_id", "detail"}, {"POST", "/:agent_id/enable", "enable"}, {"POST", "/:agent_id/disable", "disable"}, {"PUT", "/:agent_id/default-template", "default"}, {"GET", "/:agent_id/templates", "templates"}, {"POST", "/:agent_id/templates", "create-template"}, {"GET", "/:agent_id/templates/:template_id", "template"}, {"GET", "/:agent_id/templates/:template_id/revisions/:version", "template-version"}, {"PUT", "/:agent_id/templates/:template_id", "update-template"}, {"POST", "/:agent_id/templates/:template_id/archive", "archive-template"}, {"GET", "/:agent_id/recent-runs", "recent"}}

func Routes(h *Handler) []httproute.Descriptor {
	out := []httproute.Descriptor{}
	for _, s := range specs {
		policy := httproute.OrganizationAccessPolicyCachedRead
		permission := ""
		if s.method != "GET" {
			policy = httproute.OrganizationAccessPolicyLiveWrite
			permission = authz.PermissionWorkbenchAgentConfigure
		}
		if s.operation == "recent" {
			policy = httproute.OrganizationAccessPolicyLiveWrite
			permission = authz.PermissionWorkbenchAgentUse
		}
		out = append(out, httproute.Descriptor{Method: s.method, Path: Base + s.path, Module: Module, AuthPolicy: httproute.AuthPolicyCurrentIdentity, OrganizationAccessPolicy: policy, Permission: permission, RequestTimeout: 10 * time.Second, RejectUnreadRequestBody: s.method == "GET", Handler: func(c *gin.Context) {
			if h == nil {
				Failure(c, agentconfig.ErrUnavailable)
				return
			}
			h.serve(c, s)
		}})
	}
	return out
}
func ValidateDescriptor(d httproute.Descriptor) error {
	for _, r := range Routes(nil) {
		if d.Method == r.Method && d.Path == r.Path {
			if d.Module == r.Module && d.AuthPolicy == r.AuthPolicy && d.OrganizationAccessPolicy == r.OrganizationAccessPolicy && d.Permission == r.Permission && d.RequestTimeout == r.RequestTimeout && d.RejectUnreadRequestBody == r.RejectUnreadRequestBody && d.OrganizationTargetResolver == nil && d.Handler != nil {
				return nil
			}
			break
		}
	}
	return errors.New("agent configuration loses scoped permission boundary")
}
func Failure(c *gin.Context, e error) {
	status := 503
	code := agentconfig.ErrUnavailable.Error()
	for _, known := range []error{agentconfig.ErrInvalid, agentconfig.ErrForbidden, agentconfig.ErrNotFound, agentconfig.ErrConflict, agentconfig.ErrChanged, agentconfig.ErrNotEnabled, agentconfig.ErrArchived, agentconfig.ErrDefault, agentconfig.ErrDefinition, agentconfig.ErrRevision, agentconfig.ErrPrecondition} {
		if errors.Is(e, known) {
			code = known.Error()
			switch known {
			case agentconfig.ErrInvalid:
				status = 400
			case agentconfig.ErrForbidden:
				status = 403
			case agentconfig.ErrNotFound:
				status = 404
			case agentconfig.ErrRevision:
				status = 412
			case agentconfig.ErrPrecondition:
				status = 428
			default:
				status = 409
			}
			break
		}
	}
	c.Header("Cache-Control", "no-store")
	c.AbortWithStatusJSON(status, gin.H{"code": code})
}
func reply(c *gin.Context, v any, revision string) {
	raw, e := json.Marshal(v)
	if e != nil || len(raw) > 128<<10 {
		Failure(c, agentconfig.ErrUnavailable)
		return
	}
	c.Header("Cache-Control", "no-store")
	if revision != "" {
		c.Header("ETag", `"`+revision+`"`)
	}
	c.Data(200, "application/json; charset=utf-8", raw)
}
func body(c *gin.Context, v any) error {
	media, _, mediaErr := mime.ParseMediaType(c.GetHeader("Content-Type"))
	if c.GetHeader("Content-Encoding") != "" || mediaErr != nil || media != "application/json" {
		return agentconfig.ErrInvalid
	}
	raw, e := io.ReadAll(io.LimitReader(c.Request.Body, 8193))
	if e != nil || len(raw) > 8192 {
		return agentconfig.ErrInvalid
	}
	if !strings.HasPrefix(strings.TrimSpace(string(raw)), "{") {
		return agentconfig.ErrInvalid
	}
	violations, e := strict.UnmarshalStrict(raw, v)
	if e != nil || len(violations) > 0 {
		return agentconfig.ErrInvalid
	}
	return nil
}
func number(s string) (uint64, error) {
	n, e := strconv.ParseUint(s, 10, 63)
	if e != nil || n == 0 || strconv.FormatUint(n, 10) != s {
		return 0, agentconfig.ErrInvalid
	}
	return n, nil
}
func command(c *gin.Context, scope agent.Scope, s spec) (agentconfig.Command, error) {
	cmd := agentconfig.Command{Scope: scope, AgentID: c.Param("agent_id"), TemplateID: c.Param("template_id"), Operation: s.operation}
	keys := c.Request.Header.Values("Idempotency-Key")
	if len(keys) != 1 || !agentconfig.UUID(keys[0]) {
		return cmd, agentconfig.ErrInvalid
	}
	cmd.Key = keys[0]
	matches, absent := c.Request.Header.Values("If-Match"), c.Request.Header.Values("If-None-Match")
	if len(absent) > 0 {
		if s.operation != "enable" || len(absent) != 1 || absent[0] != "*" || len(matches) > 0 {
			return cmd, agentconfig.ErrInvalid
		}
		cmd.Absent = true
	} else if len(matches) > 0 {
		if s.operation == "create-template" || len(matches) != 1 || len(matches[0]) < 3 || matches[0][0] != '"' || matches[0][len(matches[0])-1] != '"' {
			return cmd, agentconfig.ErrInvalid
		}
		n, e := number(matches[0][1 : len(matches[0])-1])
		if e != nil {
			return cmd, e
		}
		cmd.Expected = n
	} else if s.operation != "create-template" {
		return cmd, agentconfig.ErrPrecondition
	}
	switch s.operation {
	case "create-template", "update-template":
		if e := body(c, &cmd.Input); e != nil {
			return cmd, e
		}
		if !cmd.Input.Valid() {
			return cmd, agentconfig.ErrInvalid
		}
	case "default":
		var input struct {
			TemplateID json.RawMessage `json:"templateId"`
			Revision   json.RawMessage `json:"revision"`
		}
		if e := body(c, &input); e != nil {
			return cmd, e
		}
		if len(input.TemplateID) == 0 || len(input.Revision) == 0 || (string(input.TemplateID) == "null") != (string(input.Revision) == "null") {
			return cmd, agentconfig.ErrInvalid
		}
		if string(input.TemplateID) != "null" {
			var templateID, revision string
			if json.Unmarshal(input.TemplateID, &templateID) != nil || json.Unmarshal(input.Revision, &revision) != nil {
				return cmd, agentconfig.ErrInvalid
			}
			if !agentconfig.UUID(templateID) {
				return cmd, agentconfig.ErrInvalid
			}
			if _, e := number(revision); e != nil {
				return cmd, e
			}
			cmd.Default = &agentconfig.TemplateRef{TemplateID: templateID, Revision: revision}
		}
	default:
		var empty struct{}
		if e := body(c, &empty); e != nil {
			return cmd, e
		}
	}
	return cmd, nil
}
func page(c *gin.Context, allowedFilter string, max int) (string, string, int, error) {
	if len(c.Request.URL.RawQuery) > 512 || c.Request.URL.ForceQuery {
		return "", "", 0, agentconfig.ErrInvalid
	}
	q, parseErr := url.ParseQuery(c.Request.URL.RawQuery)
	if parseErr != nil {
		return "", "", 0, agentconfig.ErrInvalid
	}
	cursor, filter, size := "", "", 20
	for k, v := range q {
		if len(v) != 1 {
			return "", "", 0, agentconfig.ErrInvalid
		}
		switch k {
		case "cursor":
			b, e := base64.RawURLEncoding.DecodeString(v[0])
			if e != nil || len(b) > 128 || base64.RawURLEncoding.EncodeToString(b) != v[0] {
				return "", "", 0, agentconfig.ErrInvalid
			}
			cursor = string(b)
		case "pageSize":
			n, e := strconv.Atoi(v[0])
			if e != nil || n < 1 || n > max || strconv.Itoa(n) != v[0] {
				return "", "", 0, agentconfig.ErrInvalid
			}
			size = n
		default:
			if k != allowedFilter || allowedFilter == "" {
				return "", "", 0, agentconfig.ErrInvalid
			}
			filter = v[0]
		}
	}
	return cursor, filter, size, nil
}
func cursor(s string) string {
	if s == "" {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString([]byte(s))
}
func (h *Handler) protect(ctx context.Context, scope agent.Scope, t agentconfig.Template) gin.H {
	b, _ := json.Marshal(t)
	out := gin.H{}
	_ = json.Unmarshal(b, &out)
	if t.DefaultKnowledgeBaseID != "" && (h.Knowledge == nil || h.Knowledge(ctx, scope, t.DefaultKnowledgeBaseID) != nil) {
		delete(out, "defaultKnowledgeBaseId")
		out["knowledgeAvailability"] = "UNAVAILABLE"
	} else if t.DefaultKnowledgeBaseID != "" {
		out["knowledgeAvailability"] = "AVAILABLE"
	}
	return out
}
func (h *Handler) project(ctx context.Context, scope agent.Scope, e agentconfig.CatalogEntry) (gin.H, error) {
	a, err := h.Repository.ReadAgent(ctx, scope, e.Definition.ID)
	if errors.Is(err, agentconfig.ErrNotFound) {
		a = agentconfig.OrganizationAgent{AgentID: e.Definition.ID, Activation: "NOT_ENABLED"}
		err = nil
	}
	if err != nil {
		return nil, err
	}
	caps := []agentconfig.Capability{}
	if h.Capabilities != nil {
		caps = h.Capabilities(ctx, e)
	}
	_, configure := h.Authorize(ctx, authz.PermissionWorkbenchAgentConfigure)
	_, use := h.Authorize(ctx, authz.PermissionWorkbenchAgentUse)
	ready := len(caps) > 0
	for _, cap := range caps {
		if cap.Support == "REQUIRED" && cap.Readiness != "AVAILABLE" {
			ready = false
		}
	}
	_, domain := h.Authorize(ctx, authz.PermissionListingKitAdminWrite)
	return gin.H{"agent": a, "name": e.Name, "description": e.Description, "definitionVersion": e.Definition.Version, "parameterSchema": e.ParameterSchema, "capabilities": caps, "canConfigure": configure == nil, "canUse": use == nil && domain == nil && ready && a.Activation == "ENABLED", "canReadRuns": use == nil && domain == nil}, nil
}
func (h *Handler) serve(c *gin.Context, s spec) {
	if h.Bind != nil {
		ctx, e := h.Bind(c.Request.Context(), c.GetHeader("Authorization"))
		if e != nil {
			Failure(c, agentconfig.ErrForbidden)
			return
		}
		c.Request = c.Request.WithContext(ctx)
	}
	permissions := []string{authz.PermissionWorkbenchAgentRead, authz.PermissionWorkbenchAgentConfigure}
	if s.method != "GET" {
		permissions = []string{authz.PermissionWorkbenchAgentConfigure}
	}
	if s.operation == "recent" {
		permissions = []string{authz.PermissionWorkbenchAgentUse}
	}
	if h.Repository == nil || h.Catalog == nil || h.Authorize == nil {
		Failure(c, agentconfig.ErrUnavailable)
		return
	}
	scope, e := h.Authorize(c.Request.Context(), permissions...)
	if e != nil {
		Failure(c, e)
		return
	}
	entries, e := h.Catalog.ReadCatalog(c.Request.Context())
	if e != nil {
		Failure(c, e)
		return
	}
	id := c.Param("agent_id")
	var entry agentconfig.CatalogEntry
	for _, v := range entries {
		if v.Definition.ID == id {
			entry = v
			break
		}
	}
	if id != "" && entry.Definition.ID == "" {
		Failure(c, agentconfig.ErrNotFound)
		return
	}
	if t := c.Param("template_id"); t != "" && !agentconfig.UUID(t) {
		Failure(c, agentconfig.ErrInvalid)
		return
	}
	ctx := c.Request.Context()
	if s.method != "GET" {
		if c.Request.URL.RawQuery != "" {
			Failure(c, agentconfig.ErrInvalid)
			return
		}
		cmd, e := command(c, scope, s)
		if e != nil {
			Failure(c, e)
			return
		}
		eligibility := func(ctx context.Context) error {
			if cmd.Input.DefaultKnowledgeBaseID != "" {
				if h.Knowledge == nil {
					return agentconfig.ErrUnavailable
				}
				if e := h.Knowledge(ctx, scope, cmd.Input.DefaultKnowledgeBaseID); e != nil {
					return e
				}
			}
			return nil
		}
		r, e := h.Repository.Execute(ctx, cmd, eligibility)
		if e != nil {
			Failure(c, e)
			return
		}
		reply(c, r, "")
		return
	}
	if c.Request.ContentLength > 0 {
		Failure(c, agentconfig.ErrInvalid)
		return
	}
	switch s.operation {
	case "market", "mine":
		after, filter, size, e := page(c, "activation", 100)
		if e != nil {
			Failure(c, e)
			return
		}
		out := []gin.H{}
		next := ""
		if s.operation == "market" {
			if filter != "" {
				Failure(c, agentconfig.ErrInvalid)
				return
			}
			for _, v := range entries {
				if v.Definition.ID <= after {
					continue
				}
				p, e := h.project(ctx, scope, v)
				if e != nil {
					Failure(c, e)
					return
				}
				out = append(out, p)
			}
			if len(out) > size {
				out = out[:size]
				next = out[size-1]["agent"].(agentconfig.OrganizationAgent).AgentID
			}
		} else {
			rows, n, e := h.Repository.ListAgents(ctx, scope, after, filter, size)
			if e != nil {
				Failure(c, e)
				return
			}
			next = n
			for _, row := range rows {
				for _, v := range entries {
					if v.Definition.ID == row.AgentID {
						p, e := h.project(ctx, scope, v)
						if e != nil {
							Failure(c, e)
							return
						}
						out = append(out, p)
					}
				}
			}
		}
		reply(c, gin.H{"items": out, "nextCursor": cursor(next)}, "")
	case "detail":
		if c.Request.URL.RawQuery != "" {
			Failure(c, agentconfig.ErrInvalid)
			return
		}
		v, e := h.project(ctx, scope, entry)
		if e != nil {
			Failure(c, e)
			return
		}
		reply(c, v, v["agent"].(agentconfig.OrganizationAgent).Revision)
	case "templates":
		after, filter, size, e := page(c, "lifecycle", 100)
		if e != nil {
			Failure(c, e)
			return
		}
		rows, n, e := h.Repository.Templates(ctx, scope, id, after, filter, size)
		if e != nil {
			Failure(c, e)
			return
		}
		out := []gin.H{}
		for _, r := range rows {
			out = append(out, h.protect(ctx, scope, r))
		}
		reply(c, gin.H{"items": out, "nextCursor": cursor(n)}, "")
	case "template", "template-version":
		if c.Request.URL.RawQuery != "" {
			Failure(c, agentconfig.ErrInvalid)
			return
		}
		v := uint64(0)
		if s.operation == "template-version" {
			v, e = number(c.Param("version"))
			if e != nil {
				Failure(c, e)
				return
			}
		}
		t, e := h.Repository.ReadTemplate(ctx, scope, id, c.Param("template_id"), v)
		if e != nil {
			Failure(c, e)
			return
		}
		reply(c, h.protect(ctx, scope, t), t.Revision)
	case "recent":
		after, _, size, e := page(c, "", 20)
		if e != nil {
			Failure(c, e)
			return
		}
		if h.Recent == nil {
			Failure(c, agentconfig.ErrUnavailable)
			return
		}
		items, n, e := h.Recent(ctx, scope, id, after, size)
		if e != nil {
			Failure(c, e)
			return
		}
		reply(c, gin.H{"items": items, "nextCursor": cursor(n)}, "")
	}
}
