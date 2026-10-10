package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/gin-gonic/gin"
	"io"
	"mime"
	strict "sigs.k8s.io/json"
	"strconv"
	"strings"
	pc "task-processor/internal/aiworkbench/projectcenter"
	"task-processor/internal/httproute"
	"time"
	"unicode/utf8"
)

const Base = "/api/v1/workbench/projects"
const ModuleName = "project-center"

type Handler struct {
	Service *pc.Service
	Bind    func(context.Context, string) (context.Context, pc.Scope, error)
}
type route struct{ method, path, operation string }

func specs() []route {
	return []route{
		{"GET", Base, "list"}, {"POST", Base, "create"}, {"GET", Base + "/templates", "templates"}, {"POST", Base + "/templates", "template_save"},
		{"POST", Base + "/templates/:id/archive", "template_archive"}, {"GET", Base + "/:id", "get"}, {"PATCH", Base + "/:id", "edit"},
		{"POST", Base + "/:id/archive", "archive"}, {"POST", Base + "/:id/restore", "restore"}, {"POST", Base + "/:id/visit", "visit"},
		{"POST", Base + "/:id/references", "add"}, {"POST", Base + "/:id/references/:slot/remove", "remove"},
	}
}
func Routes(h *Handler) []httproute.Descriptor {
	out := []httproute.Descriptor{}
	for _, r := range specs() {
		permission := pc.PermissionRead
		if r.method != "GET" && r.operation != "visit" {
			permission = pc.PermissionManage
		}
		out = append(out, httproute.Descriptor{Method: r.method, Path: r.path, Module: ModuleName, Permission: permission, AuthPolicy: httproute.AuthPolicyCurrentIdentity, OrganizationAccessPolicy: httproute.OrganizationAccessPolicyLiveWrite, RequestTimeout: 10 * time.Second, RejectUnreadRequestBody: r.method == "GET", Handler: httproute.WithRequestBodyReadTimeout(10*time.Second, func(c *gin.Context) {
			if h == nil || h.Service == nil || h.Bind == nil {
				failure(c, pc.ErrUnavailable)
				return
			}
			h.serve(c, r)
		})})
	}
	return out
}
func ValidateDescriptor(d httproute.Descriptor) error {
	for _, expected := range Routes(nil) {
		if d.Method == expected.Method && d.Path == expected.Path && d.Module == expected.Module && d.Permission == expected.Permission && d.AuthPolicy == expected.AuthPolicy && d.OrganizationAccessPolicy == expected.OrganizationAccessPolicy && d.OrganizationTargetResolver == nil && d.RequestTimeout == expected.RequestTimeout && d.RejectUnreadRequestBody == expected.RejectUnreadRequestBody && d.Handler != nil {
			return nil
		}
	}
	return pc.ErrForbidden
}
func failure(c *gin.Context, e error) {
	status, code := 503, pc.ErrUnavailable.Error()
	for _, v := range []error{pc.ErrInvalid, pc.ErrForbidden, pc.ErrNotFound, pc.ErrConflict, pc.ErrRevision, pc.ErrArchived} {
		if errors.Is(e, v) {
			code = v.Error()
			status = map[error]int{pc.ErrInvalid: 400, pc.ErrForbidden: 403, pc.ErrNotFound: 404, pc.ErrConflict: 409, pc.ErrRevision: 409, pc.ErrArchived: 409}[v]
			break
		}
	}
	c.Header("Cache-Control", "private, no-store")
	httproute.RejectUnreadRequestBody(c)
	c.AbortWithStatusJSON(status, gin.H{"code": code})
}
func reply(c *gin.Context, v any) {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	e := encoder.Encode(v)
	raw := buffer.Bytes()
	if e != nil || len(raw) > 256<<10 {
		failure(c, pc.ErrUnavailable)
		return
	}
	c.Header("Cache-Control", "private, no-store")
	c.Data(200, "application/json; charset=utf-8", raw)
}
func body(c *gin.Context, v any) error {
	media, p, e := mime.ParseMediaType(c.GetHeader("Content-Type"))
	if e != nil || media != "application/json" || p["charset"] != "" && !strings.EqualFold(p["charset"], "utf-8") || c.GetHeader("Content-Encoding") != "" || c.Request.Body == nil {
		return pc.ErrInvalid
	}
	raw, e := io.ReadAll(io.LimitReader(c.Request.Body, (16<<10)+1))
	if e != nil || len(raw) == 0 || len(raw) > 16<<10 || !utf8.Valid(raw) || !strings.HasPrefix(strings.TrimSpace(string(raw)), "{") {
		return pc.ErrInvalid
	}
	violations, e := strict.UnmarshalStrict(raw, v, strict.DisallowDuplicateFields, strict.DisallowUnknownFields)
	if e != nil || len(violations) > 0 {
		return pc.ErrInvalid
	}
	return nil
}
func revision(c *gin.Context) (uint64, error) {
	v := c.GetHeader("If-Match")
	n, e := strconv.ParseUint(v, 10, 53)
	if e != nil || n == 0 || strconv.FormatUint(n, 10) != v || len(c.Request.Header.Values("If-Match")) != 1 {
		return 0, pc.ErrInvalid
	}
	return n, nil
}
func (h *Handler) serve(c *gin.Context, r route) {
	ctx, scope, e := h.Bind(c.Request.Context(), c.GetHeader("Authorization"))
	if e != nil {
		failure(c, e)
		return
	}
	query := c.Request.URL.Query()
	if r.method == "GET" {
		allowed := map[string]bool{"after": true}
		if r.operation == "list" {
			for _, k := range []string{"mode", "search", "kind", "workScope", "storeId"} {
				allowed[k] = true
			}
		}
		for k, v := range query {
			if !allowed[k] || len(v) != 1 {
				failure(c, pc.ErrInvalid)
				return
			}
		}
		if r.operation == "get" && len(query) != 0 {
			failure(c, pc.ErrInvalid)
			return
		}
		switch r.operation {
		case "list":
			mode := c.Query("mode")
			if mode == "" {
				mode = "active"
			}
			v, e := h.Service.List(ctx, scope, pc.Query{Mode: mode, Search: c.Query("search"), Kind: c.Query("kind"), WorkScope: c.Query("workScope"), StoreID: c.Query("storeId"), After: c.Query("after")})
			if e != nil {
				failure(c, e)
			} else {
				reply(c, v)
			}
		case "get":
			v, e := h.Service.Get(ctx, scope, c.Param("id"))
			if e != nil {
				failure(c, e)
			} else {
				reply(c, v)
			}
		case "templates":
			v, e := h.Service.Templates(ctx, scope, c.Query("after"))
			if e != nil {
				failure(c, e)
			} else {
				reply(c, v)
			}
		}
		return
	}
	if len(query) > 0 || len(c.Request.Header.Values("Idempotency-Key")) != 1 {
		failure(c, pc.ErrInvalid)
		return
	}
	cmd := pc.Command{Operation: r.operation, ID: c.Param("id")}
	if r.operation != "create" && r.operation != "visit" {
		cmd.Expected, e = revision(c)
		if e != nil {
			failure(c, e)
			return
		}
	} else if c.GetHeader("If-Match") != "" {
		failure(c, pc.ErrInvalid)
		return
	}
	switch r.operation {
	case "create", "edit":
		var b struct {
			pc.Fields
			StoreID *string `json:"storeId,omitempty"`
		}
		e = body(c, &b)
		cmd.Fields = &b.Fields
		cmd.StoreID = b.StoreID
	case "add":
		var b struct {
			Kind     string `json:"kind"`
			TargetID string `json:"targetId"`
		}
		e = body(c, &b)
		cmd.Reference = &pc.Reference{Kind: b.Kind, TargetID: b.TargetID}
	case "template_save":
		var b struct {
			ProjectID string `json:"projectId"`
			Name      string `json:"name"`
		}
		e = body(c, &b)
		cmd.ID = b.ProjectID
		cmd.Name = b.Name
	default:
		var b struct{}
		e = body(c, &b)
		if r.operation == "remove" {
			cmd.SlotID = c.Param("slot")
		}
	}
	if e != nil {
		failure(c, e)
		return
	}
	v, e := h.Service.Execute(ctx, scope, c.GetHeader("Idempotency-Key"), cmd)
	if e != nil {
		failure(c, e)
	} else {
		reply(c, v)
	}
}
