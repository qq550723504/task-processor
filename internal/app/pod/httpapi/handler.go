package httpapi

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
	podapp "task-processor/internal/app/pod"
	"task-processor/internal/httproute"
	"task-processor/internal/product/asset"
	"task-processor/internal/product/collection"
	"task-processor/internal/product/pod"
	"task-processor/internal/product/supplymarket"
	"time"
	"unicode/utf8"
)

const BasePath = "/api/v1/workbench/pod"

type spec struct{ method, path, action, permission string }

func Routes(s *podapp.Service, bind func(context.Context, string) (context.Context, error)) []httproute.Descriptor {
	specs := []spec{{"GET", "/templates", "templates", supplymarket.PermissionRead}, {"GET", "/templates/:id", "template", supplymarket.PermissionRead}, {"POST", "/templates/select", "select-template", supplymarket.PermissionSelect}, {"GET", "/manifests/:id", "manifest", supplymarket.PermissionDesign}, {"GET", "/artwork/:id", "artwork", supplymarket.PermissionDesign}, {"POST", "/approvals", "approve", supplymarket.PermissionDesign}, {"POST", "/designs", "design", supplymarket.PermissionDesign}, {"GET", "/designs/:id", "operation", supplymarket.PermissionRead}, {"GET", "/designs/:id/verify", "verify", supplymarket.PermissionDesign}, {"POST", "/designs/:id/select", "select-finished", supplymarket.PermissionSelect}, {"GET", "/by-key/:key", "by-key", supplymarket.PermissionRead}, {"GET", "/imports/by-key/:key", "import-by-key", supplymarket.PermissionRead}}
	specs = append(specs, spec{"GET", "/artwork/:id/approvals/:action_id", "approval", supplymarket.PermissionDesign})
	result := []httproute.Descriptor{}
	for _, sp := range specs {
		access := httproute.OrganizationAccessPolicyCachedRead
		if sp.method == "POST" {
			access = httproute.OrganizationAccessPolicyLiveWrite
		}
		result = append(result, httproute.Descriptor{Method: sp.method, Path: BasePath + sp.path, Module: "supply-market", Permission: sp.permission, AuthPolicy: httproute.AuthPolicyCurrentIdentity, OrganizationAccessPolicy: access, RequestTimeout: 10 * time.Second, Handler: httproute.WithRequestBodyReadTimeout(5*time.Second, func(c *gin.Context) {
			c.Header("Cache-Control", "no-store")
			c.Header("X-Content-Type-Options", "nosniff")
			if s == nil || bind == nil {
				writeError(c, pod.ErrUnavailable)
				return
			}
			if sp.method == "GET" && (c.Request.ContentLength != 0 || len(c.Request.TransferEncoding) > 0 || c.Request.Body != nil && c.Request.Body != http.NoBody) {
				httproute.RejectUnreadRequestBody(c)
				writeError(c, pod.ErrInvalid)
				return
			}
			ctx, e := bind(c.Request.Context(), c.GetHeader("Authorization"))
			if e != nil {
				writeError(c, pod.ErrForbidden)
				return
			}
			var value any
			if sp.action != "templates" && sp.action != "manifest" && c.Request.URL.RawQuery != "" {
				writeError(c, pod.ErrInvalid)
				return
			}
			switch sp.action {
			case "templates":
				page, size, keyword, err := listQuery(c.Request.URL.RawQuery)
				if err != nil {
					writeError(c, err)
					return
				}
				value, e = s.ListTemplates(ctx, page, size, keyword)
			case "template":
				var t pod.Template
				t, e = s.Template(ctx, c.Param("id"))
				value = struct {
					Template pod.Template `json:"template"`
					Hash     string       `json:"hash"`
				}{t, collection.Digest(t)}
			case "manifest":
				q, err := url.ParseQuery(c.Request.URL.RawQuery)
				if err != nil || len(q) != 1 || len(q["variant"]) != 1 {
					writeError(c, pod.ErrInvalid)
					return
				}
				value, e = s.Manifest(ctx, c.Param("id"), q.Get("variant"))
			case "artwork":
				value, e = s.Artwork(ctx, c.Param("id"))
			case "approval":
				value, e = s.Approval(ctx, c.Param("id"), c.Param("action_id"))
			case "operation", "verify":
				value, e = s.Operation(ctx, c.Param("id"), sp.action == "verify")
			case "by-key":
				value, e = s.ByKey(ctx, c.Param("key"))
			case "import-by-key":
				value, e = s.ImportByKey(ctx, c.Param("key"))
			case "design":
				var in podapp.DesignRequest
				key, err := readBody(c.Request, &in)
				if err != nil {
					writeError(c, err)
					return
				}
				value, e = s.Create(ctx, key, in)
			case "approve":
				var in asset.SourceApprovalCommand
				key, err := readBody(c.Request, &in)
				if err != nil || key != in.ActionID {
					writeError(c, pod.ErrInvalid)
					return
				}
				value, e = s.Approve(ctx, in)
			case "select-template":
				var in struct {
					ID   string `json:"id"`
					Hash string `json:"hash"`
				}
				key, err := readBody(c.Request, &in)
				if err != nil {
					writeError(c, err)
					return
				}
				value, e = s.ImportTemplate(ctx, key, in.ID, in.Hash)
			case "select-finished":
				var in struct{}
				key, err := readBody(c.Request, &in)
				if err != nil {
					writeError(c, err)
					return
				}
				value, e = s.ImportFinished(ctx, key, c.Param("id"))
			}
			if e != nil {
				writeError(c, e)
				return
			}
			raw, e := json.Marshal(value)
			if e != nil || len(raw) > 2<<20 {
				writeError(c, pod.ErrUnavailable)
				return
			}
			c.Data(200, "application/json; charset=utf-8", raw)
		})})
	}
	return result
}
func readBody(r *http.Request, out any) (string, error) {
	keys := r.Header.Values("Idempotency-Key")
	media, params, e := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if len(keys) != 1 || !collection.ValidID(keys[0]) || r.Body == nil || r.URL.RawQuery != "" || len(r.Header.Values("If-Match")) != 0 || r.Header.Get("Content-Encoding") != "" || e != nil || media != "application/json" || params["charset"] != "" && !strings.EqualFold(params["charset"], "utf-8") {
		return "", pod.ErrInvalid
	}
	raw, e := io.ReadAll(io.LimitReader(r.Body, 65537))
	if e != nil || len(raw) > 65536 || !utf8.Valid(raw) {
		return "", pod.ErrInvalid
	}
	strict, e := sigjson.UnmarshalStrict(raw, out, sigjson.DisallowDuplicateFields, sigjson.DisallowUnknownFields)
	if e != nil || len(strict) > 0 {
		return "", pod.ErrInvalid
	}
	return keys[0], nil
}
func listQuery(raw string) (int, int, string, error) {
	q, e := url.ParseQuery(raw)
	if e != nil {
		return 0, 0, "", pod.ErrInvalid
	}
	for k, v := range q {
		if len(v) != 1 || k != "page" && k != "size" && k != "keyword" {
			return 0, 0, "", pod.ErrInvalid
		}
	}
	page, size := 1, 10
	if q.Has("page") {
		page, e = strconv.Atoi(q.Get("page"))
		if e != nil {
			return 0, 0, "", pod.ErrInvalid
		}
	}
	if q.Has("size") {
		size, e = strconv.Atoi(q.Get("size"))
		if e != nil {
			return 0, 0, "", pod.ErrInvalid
		}
	}
	keyword := q.Get("keyword")
	if page < 1 || page > 10000 || size < 1 || size > 50 || len(keyword) > 200 || !utf8.ValidString(keyword) || strings.ContainsAny(keyword, "\x00\r\n") {
		return 0, 0, "", pod.ErrInvalid
	}
	return page, size, keyword, nil
}
func writeError(c *gin.Context, e error) {
	status, code := 503, "DEPENDENCY_UNAVAILABLE"
	switch {
	case errors.Is(e, pod.ErrInvalid), errors.Is(e, collection.ErrInvalid), errors.Is(e, asset.ErrInvalidApproval):
		status, code = 400, "INVALID_REQUEST"
	case errors.Is(e, pod.ErrForbidden), errors.Is(e, collection.ErrForbidden), errors.Is(e, asset.ErrSourceApprovalForbidden):
		status, code = 403, "PERMISSION_DENIED"
	case errors.Is(e, pod.ErrNotFound), errors.Is(e, collection.ErrNotFound):
		status, code = 404, "NOT_FOUND"
	case errors.Is(e, pod.ErrConflict), errors.Is(e, collection.ErrConflict), errors.Is(e, asset.ErrApprovalConflict):
		status, code = 409, "REVISION_CONFLICT"
	case errors.Is(e, pod.ErrUnknown), errors.Is(e, context.DeadlineExceeded), errors.Is(e, context.Canceled):
		status, code = 409, "OUTCOME_UNKNOWN"
	}
	c.JSON(status, gin.H{"code": code})
}
