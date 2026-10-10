package storeobservationshttp

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/gin-gonic/gin"
	"io"
	"mime"
	"net/http"
	"net/url"
	sigjson "sigs.k8s.io/json"
	"strconv"
	"strings"
	app "task-processor/internal/app/storeobservations"
	"task-processor/internal/httproute"
	o "task-processor/internal/marketplace/shein/observations"
	"time"
	"unicode/utf8"
)

const BasePath = "/api/v1/workbench/store-observations"
const maxResponse = 2 << 20

func Routes(a *app.Application, bind func(context.Context, string) (context.Context, error)) []httproute.Descriptor {
	out := []httproute.Descriptor{}
	for _, kind := range []o.Kind{o.Products, o.Orders} {
		specs := []struct {
			method, path, action string
			sync                 bool
		}{{"GET", "/capabilities", "capabilities", false}, {"GET", "", "list", false}, {"POST", "/syncs", "begin", true}, {"GET", "/commands/:key", "command", false}, {"GET", "/syncs/:sync_id", "status", false}, {"POST", "/syncs/:sync_id/ensure", "ensure", true}, {"GET", "/stores/:store_id/syncs/:sync_id/records/:record_id", "detail", false}}
		if kind == o.Orders {
			specs = append(specs, struct {
				method, path, action string
				sync                 bool
			}{"GET", "/stores/:store_id/syncs/:sync_id/records/:record_id/packages/:package_id/track", "track", false})
		}
		for _, spec := range specs {
			permission := o.ReadPermission(kind)
			if spec.sync {
				permission = o.SyncPermission(kind)
			}
			out = append(out, httproute.Descriptor{Method: spec.method, Path: BasePath + "/" + string(kind) + spec.path, Module: "store-" + string(kind), Permission: permission, AuthPolicy: httproute.AuthPolicyCurrentIdentity, OrganizationAccessPolicy: httproute.OrganizationAccessPolicyLiveWrite, RequestTimeout: 20 * time.Second, RejectUnreadRequestBody: spec.action != "begin", Handler: httproute.WithRequestBodyReadTimeout(3*time.Second, func(c *gin.Context) {
				c.Header("Cache-Control", "private, no-store")
				c.Header("X-Content-Type-Options", "nosniff")
				if spec.action != "begin" {
					httproute.RejectUnreadRequestBody(c)
					if c.Request.ContentLength != 0 || len(c.Request.TransferEncoding) != 0 || c.Request.Body != nil && c.Request.Body != http.NoBody {
						failure(c, o.ErrInvalid)
						return
					}
					if len(c.Request.Header.Values("Idempotency-Key")) != 0 {
						failure(c, o.ErrInvalid)
						return
					}
				}
				if spec.action != "list" && c.Request.URL.RawQuery != "" {
					failure(c, o.ErrInvalid)
					return
				}
				var input o.BeginInput
				key := ""
				var query o.Query
				var e error
				if spec.action == "begin" {
					key, e = beginBody(c.Request, &input)
					if e != nil || input.Kind != kind {
						failure(c, o.ErrInvalid)
						return
					}
				}
				if spec.action == "list" {
					query, e = listQuery(c.Request, kind)
					if e != nil {
						failure(c, e)
						return
					}
				}
				if a == nil || a.Service == nil || bind == nil {
					failure(c, o.ErrUnavailable)
					return
				}
				ctx, e := bind(c.Request.Context(), c.GetHeader("Authorization"))
				if e != nil {
					failure(c, o.ErrForbidden)
					return
				}
				scope, e := app.Scope(ctx)
				if e != nil {
					failure(c, e)
					return
				}
				if spec.action != "capabilities" && !a.Available() {
					failure(c, o.ErrUnavailable)
					return
				}
				var data any
				switch spec.action {
				case "capabilities":
					data, e = a.Capabilities(ctx, scope, kind)
				case "list":
					data, e = a.Service.List(ctx, scope, query)
				case "begin":
					data, e = a.Service.Begin(ctx, scope, key, input)
				case "command":
					var v o.Command
					v, e = a.Service.Command(ctx, scope, c.Param("key"))
					if e == nil && v.Input.Kind != kind {
						e = o.ErrNotFound
					}
					data = v
				case "status", "ensure":
					var v o.Sync
					v, e = a.Service.Status(ctx, scope, c.Param("sync_id"))
					if e == nil && v.Kind != kind {
						e = o.ErrNotFound
					}
					if e == nil && spec.action == "ensure" {
						v, e = a.Service.Ensure(ctx, scope, v.ID)
					}
					data = v
				case "detail":
					data, e = a.Service.Detail(ctx, scope, c.Param("store_id"), c.Param("sync_id"), c.Param("record_id"), kind)
				case "track":
					data, e = a.Service.Logistics(ctx, scope, c.Param("store_id"), c.Param("sync_id"), c.Param("record_id"), c.Param("package_id"))
				}
				if e != nil {
					failure(c, e)
					return
				}
				raw, e := json.Marshal(struct {
					OrganizationID string `json:"organizationId"`
					UserID         string `json:"userId"`
					Data           any    `json:"data"`
				}{scope.OrganizationID, scope.ActorID, data})
				if e != nil || len(raw) > maxResponse {
					failure(c, o.ErrUnavailable)
					return
				}
				c.Data(http.StatusOK, "application/json; charset=utf-8", raw)
			})})
		}
	}
	return out
}
func listQuery(r *http.Request, kind o.Kind) (o.Query, error) {
	q := o.Query{Kind: kind, Limit: 20}
	if len(r.URL.RawQuery) > 8192 {
		return q, o.ErrInvalid
	}
	values, e := url.ParseQuery(r.URL.RawQuery)
	if e != nil {
		return q, o.ErrInvalid
	}
	for key, v := range values {
		if len(v) != 1 {
			return q, o.ErrInvalid
		}
		switch key {
		case "storeId":
			if !o.ValidID(v[0]) {
				return q, o.ErrInvalid
			}
			q.Stores = []string{v[0]}
		case "syncId":
			if !o.ValidID(v[0]) {
				return q, o.ErrInvalid
			}
			q.SyncID = v[0]
		case "keyword":
			q.Keyword = v[0]
		case "status":
			q.Status = v[0]
		case "after":
			q.After = v[0]
		case "limit":
			if len(v[0]) > 2 || v[0] == "" || v[0][0] == '0' {
				return q, o.ErrInvalid
			}
			q.Limit, e = strconv.Atoi(v[0])
			if e != nil {
				return q, o.ErrInvalid
			}
		default:
			return q, o.ErrInvalid
		}
	}
	if q.Limit < 1 || q.Limit > 50 || !o.Text(q.Keyword, 200) || len(q.After) > 4096 || !o.Text(q.Status, 32) {
		return q, o.ErrInvalid
	}
	return q, nil
}
func beginBody(r *http.Request, in *o.BeginInput) (string, error) {
	keys := r.Header.Values("Idempotency-Key")
	if len(keys) != 1 || !o.ValidID(keys[0]) || r.Body == nil || r.ContentLength > 65536 {
		return "", o.ErrInvalid
	}
	media, params, e := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if e != nil || media != "application/json" || params["charset"] != "" && !strings.EqualFold(params["charset"], "utf-8") || r.Header.Get("Content-Encoding") != "" && r.Header.Get("Content-Encoding") != "identity" {
		return "", o.ErrInvalid
	}
	raw, e := io.ReadAll(io.LimitReader(r.Body, 65537))
	if e != nil || len(raw) > 65536 || !utf8.Valid(raw) {
		return "", o.ErrInvalid
	}
	strict, e := sigjson.UnmarshalStrict(raw, in, sigjson.DisallowDuplicateFields, sigjson.DisallowUnknownFields)
	if e != nil || len(strict) > 0 {
		return "", o.ErrInvalid
	}
	return keys[0], nil
}
func failure(c *gin.Context, e error) {
	status, code := 503, "DEPENDENCY_UNAVAILABLE"
	switch {
	case errors.Is(e, o.ErrInvalid):
		status, code = 400, "INVALID_REQUEST"
	case errors.Is(e, o.ErrForbidden):
		status, code = 403, "PERMISSION_DENIED"
	case errors.Is(e, o.ErrNotFound):
		status, code = 404, "NOT_FOUND"
	case errors.Is(e, o.ErrConflict):
		status, code = 409, "REVISION_CONFLICT"
	case errors.Is(e, o.ErrUnsupported):
		status, code = 409, "UNSUPPORTED_APPLICATION"
	case errors.Is(e, context.DeadlineExceeded), errors.Is(e, context.Canceled):
		status, code = 504, "DEADLINE_EXCEEDED"
	}
	c.JSON(status, gin.H{"code": code})
}
