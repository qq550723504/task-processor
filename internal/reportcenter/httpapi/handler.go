package reporthttp

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/gin-gonic/gin"
	"io"
	"mime"
	"net/url"
	strict "sigs.k8s.io/json"
	"strconv"
	"strings"
	"task-processor/internal/authidentity"
	"task-processor/internal/httproute"
	rc "task-processor/internal/reportcenter"
	"time"
)

const Base = "/api/v1/workbench/reports"
const ModuleName = "report-center"

type Handler struct {
	Service *rc.Service
	Bind    func(context.Context, string) (context.Context, error)
}

func BuildRoutes(h *Handler) ([]httproute.Descriptor, error) {
	if h == nil || h.Service == nil || h.Service.Repository == nil || h.Service.Authorize == nil || h.Bind == nil {
		return nil, rc.ErrUnavailable
	}
	return Routes(h), nil
}
func Routes(h *Handler) []httproute.Descriptor {
	result := []httproute.Descriptor{}
	for _, s := range []struct{ method, path, op string }{{"GET", Base, "list"}, {"POST", Base, "save"}, {"GET", Base + "/summary", "summary"}, {"GET", Base + "/sources/:kind/:id", "source"}, {"GET", Base + "/:id", "read"}, {"POST", Base + "/:id/favorite", "favorite"}} {
		permission := rc.ReadPermission
		access := httproute.OrganizationAccessPolicyCachedRead
		if s.method != "GET" {
			permission = rc.ManagePermission
			access = httproute.OrganizationAccessPolicyLiveWrite
		}
		result = append(result, httproute.Descriptor{Method: s.method, Path: s.path, Module: ModuleName, Permission: permission, AuthPolicy: httproute.AuthPolicyCurrentIdentity, OrganizationAccessPolicy: access, RequestTimeout: 10 * time.Second, RejectUnreadRequestBody: s.method == "GET", Handler: httproute.WithRequestBodyReadTimeout(10*time.Second, func(c *gin.Context) {
			if s.method == "GET" && (c.Request.ContentLength != 0 || len(c.Request.TransferEncoding) != 0) {
				failure(c, rc.ErrInvalid)
				return
			}
			if h == nil || h.Service == nil || h.Bind == nil {
				failure(c, rc.ErrUnavailable)
				return
			}
			h.serve(c, s.op)
		})})
	}
	return result
}
func ValidateDescriptor(d httproute.Descriptor) error {
	for _, r := range Routes(nil) {
		if d.Method == r.Method && d.Path == r.Path && d.Module == r.Module && d.Permission == r.Permission && d.AuthPolicy == r.AuthPolicy && d.OrganizationAccessPolicy == r.OrganizationAccessPolicy && d.RequestTimeout == r.RequestTimeout && d.RejectUnreadRequestBody == r.RejectUnreadRequestBody && d.OrganizationTargetResolver == nil && d.Handler != nil {
			return nil
		}
	}
	return rc.ErrForbidden
}
func failure(c *gin.Context, e error) {
	status, code := 503, rc.ErrUnavailable.Error()
	for _, v := range []error{rc.ErrInvalid, rc.ErrForbidden, rc.ErrNotFound, rc.ErrConflict} {
		if errors.Is(e, v) {
			code = v.Error()
			status = map[error]int{rc.ErrInvalid: 400, rc.ErrForbidden: 403, rc.ErrNotFound: 404, rc.ErrConflict: 409}[v]
			break
		}
	}
	c.Header("Cache-Control", "private, no-store")
	httproute.RejectUnreadRequestBody(c)
	c.AbortWithStatusJSON(status, gin.H{"code": code})
}
func body(c *gin.Context, v any) error {
	media, _, e := mime.ParseMediaType(c.GetHeader("Content-Type"))
	if e != nil || media != "application/json" || c.GetHeader("Content-Encoding") != "" {
		return rc.ErrInvalid
	}
	raw, e := io.ReadAll(io.LimitReader(c.Request.Body, 2049))
	if e != nil || len(raw) > 2048 || !strings.HasPrefix(strings.TrimSpace(string(raw)), "{") {
		return rc.ErrInvalid
	}
	violations, e := strict.UnmarshalStrict(raw, v)
	if e != nil || len(violations) > 0 {
		return rc.ErrInvalid
	}
	return nil
}
func (h *Handler) serve(c *gin.Context, op string) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	ctx, e := h.Bind(ctx, c.GetHeader("Authorization"))
	if e != nil {
		failure(c, e)
		return
	}
	id, ok := authidentity.AuthenticatedIdentityFromContext(ctx)
	if !ok {
		failure(c, rc.ErrForbidden)
		return
	}
	scope := rc.Scope{OrganizationID: id.EffectiveOrganizationID, ActorID: id.UserID}
	query, e := url.ParseQuery(c.Request.URL.RawQuery)
	if e != nil || len(c.Request.URL.RawQuery) > 1024 || c.Request.URL.ForceQuery {
		e = rc.ErrInvalid
	}
	if op != "list" && len(query) > 0 {
		e = rc.ErrInvalid
	}
	for k, v := range query {
		if len(v) != 1 || (k != "view" && k != "kind" && k != "search" && k != "cursor" && k != "limit") {
			e = rc.ErrInvalid
		}
	}
	if e != nil {
		failure(c, e)
		return
	}
	var out any
	switch op {
	case "list":
		limit := 50
		if query.Has("limit") {
			limit, e = strconv.Atoi(query.Get("limit"))
			if e != nil {
				failure(c, rc.ErrInvalid)
				return
			}
		}
		view := query.Get("view")
		if view == "" {
			view = "all"
		}
		out, e = h.Service.List(ctx, scope, rc.Filter{View: view, Kind: query.Get("kind"), Search: query.Get("search"), Cursor: query.Get("cursor"), Limit: limit})
	case "summary":
		out, e = h.Service.Summary(ctx, scope)
	case "source":
		out, e = h.Service.Source(ctx, scope, c.Param("kind"), c.Param("id"))
	case "read":
		out, e = h.Service.Read(ctx, scope, c.Param("id"))
	case "save", "favorite":
		keys := c.Request.Header.Values("Idempotency-Key")
		if len(keys) != 1 || !rc.UUID(keys[0]) {
			failure(c, rc.ErrInvalid)
			return
		}
		if op == "save" {
			var payload struct {
				Source rc.SourceRef `json:"source"`
			}
			if e = body(c, &payload); e == nil {
				out, e = h.Service.Save(ctx, scope, keys[0], payload.Source)
			}
		} else {
			var payload struct {
				Favorite *bool `json:"favorite"`
			}
			if e = body(c, &payload); e == nil {
				if payload.Favorite == nil {
					e = rc.ErrInvalid
				} else {
					out, e = h.Service.Favorite(ctx, scope, keys[0], c.Param("id"), *payload.Favorite)
				}
			}
		}
	}
	if e != nil {
		failure(c, e)
		return
	}
	raw, e := json.Marshal(out)
	if e != nil || len(raw) > 160<<10 {
		failure(c, rc.ErrUnavailable)
		return
	}
	c.Header("Cache-Control", "private, no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Data(200, "application/json; charset=utf-8", raw)
}
