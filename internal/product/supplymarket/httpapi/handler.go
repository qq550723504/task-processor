package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"task-processor/internal/authz"
	"task-processor/internal/httproute"
	"task-processor/internal/product/collection"
	"task-processor/internal/product/supplymarket"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	sigjson "sigs.k8s.io/json"
)

const BasePath = "/api/v1/workbench/supply-market"
const AdminPath = "/api/v1/admin/supply-market"

type Service interface {
	ExecuteMember(context.Context, string, supplymarket.Mutation) (supplymarket.Receipt, error)
	ExecutePlatform(context.Context, string, supplymarket.Mutation) (supplymarket.Receipt, error)
	ListMarket(context.Context, supplymarket.Query) (supplymarket.Page[supplymarket.Release], error)
	ReadMarket(context.Context, string) (supplymarket.Release, error)
	ListApplications(context.Context, supplymarket.Query, bool) (supplymarket.Page[supplymarket.Record], error)
	ReadApplication(context.Context, string, bool) (supplymarket.Record, error)
	Events(context.Context, string, supplymarket.Query, bool) (supplymarket.Page[supplymarket.Event], error)
	ByKey(context.Context, string, bool) (supplymarket.Receipt, error)
	Choice(context.Context, string) (supplymarket.ProductChoice, error)
	RecordReleases(context.Context, string, supplymarket.Query, bool) (supplymarket.Page[supplymarket.Release], error)
}
type Files interface {
	Upload(context.Context, string, string, []byte) (supplymarket.PrivateFile, error)
	Download(context.Context, string, string, bool) (supplymarket.PrivateFile, []byte, error)
}
type routeSpec struct {
	method, path, action string
	platform             bool
}

func Routes(service Service, files Files, bind func(context.Context, string) (context.Context, error)) []httproute.Descriptor {
	specs := []routeSpec{{http.MethodGet, BasePath + "/releases", "market", false}, {http.MethodGet, BasePath + "/releases/:id", "product", false}, {http.MethodGet, BasePath + "/choices/:id", "choice", false}, {http.MethodPost, BasePath + "/select", "select", false}, {http.MethodPost, BasePath + "/uploads", "upload", false}}
	for _, base := range []string{BasePath, AdminPath} {
		platform := base == AdminPath
		specs = append(specs, []routeSpec{{http.MethodGet, base + "/records", "records", platform}, {http.MethodGet, base + "/records/:id", "record", platform}, {http.MethodGet, base + "/records/:id/releases", "record-releases", platform}, {http.MethodGet, base + "/records/:id/events", "events", platform}, {http.MethodGet, base + "/records/:id/files/:file_id", "download", platform}, {http.MethodGet, base + "/by-key/:key", "operation", platform}, {http.MethodPost, base + "/commands", "command", platform}}...)
	}
	routes := make([]httproute.Descriptor, 0, len(specs))
	for _, spec := range specs {
		policy, organization, permission := httproute.AuthPolicyCurrentIdentity, httproute.OrganizationAccessPolicyCachedRead, supplymarket.PermissionRead
		timeout := supplymarket.Timeout
		if spec.method == http.MethodPost {
			organization = httproute.OrganizationAccessPolicyLiveWrite
			permission = supplymarket.PermissionApply
		}
		if spec.action == "select" {
			permission = supplymarket.PermissionSelect
		}
		if spec.action == "choice" {
			permission = supplymarket.PermissionApply
		}
		if spec.action == "upload" || spec.action == "download" {
			timeout = 30 * time.Second
		}
		if spec.platform {
			policy = httproute.AuthPolicyCurrentIdentityWithVerifiedRoles
			organization = httproute.OrganizationAccessPolicyNone
			permission = authz.PermissionListingKitPlatformAdm
		}
		routes = append(routes, httproute.Descriptor{Method: spec.method, Path: spec.path, Module: "supply-market", Permission: permission, AuthPolicy: policy, OrganizationAccessPolicy: organization, RequestTimeout: timeout, Handler: httproute.WithRequestBodyReadTimeout(5*time.Second, func(c *gin.Context) {
			c.Header("Cache-Control", "no-store")
			c.Header("X-Content-Type-Options", "nosniff")
			if service == nil || bind == nil {
				writeError(c, supplymarket.ErrUnavailable)
				return
			}
			if spec.method == http.MethodGet && (c.Request.ContentLength != 0 || len(c.Request.TransferEncoding) > 0 || c.Request.Body != nil && c.Request.Body != http.NoBody) {
				httproute.RejectUnreadRequestBody(c)
				writeError(c, supplymarket.ErrInvalid)
				return
			}
			ctx, err := bind(c.Request.Context(), c.GetHeader("Authorization"))
			if err != nil {
				writeError(c, supplymarket.ErrForbidden)
				return
			}
			var value any
			switch spec.action {
			case "market", "records", "events", "record-releases":
				q, e := readQuery(c.Request.URL.RawQuery)
				if e != nil {
					writeError(c, e)
					return
				}
				switch spec.action {
				case "record-releases":
					value, err = service.RecordReleases(ctx, c.Param("id"), q, spec.platform)
				case "market":
					value, err = service.ListMarket(ctx, q)
				case "records":
					value, err = service.ListApplications(ctx, q, spec.platform)
				case "events":
					value, err = service.Events(ctx, c.Param("id"), q, spec.platform)
				}
			case "product", "record", "operation", "choice":
				if c.Request.URL.RawQuery != "" {
					writeError(c, supplymarket.ErrInvalid)
					return
				}
				switch spec.action {
				case "choice":
					value, err = service.Choice(ctx, c.Param("id"))
				case "product":
					value, err = service.ReadMarket(ctx, c.Param("id"))
				case "record":
					value, err = service.ReadApplication(ctx, c.Param("id"), spec.platform)
				case "operation":
					value, err = service.ByKey(ctx, c.Param("key"), spec.platform)
				}
			case "command", "select":
				key, input, e := readMutation(c.Request)
				if e != nil || spec.action == "select" && input.Action != "select_release" || spec.action == "command" && !spec.platform && input.Action == "select_release" {
					writeError(c, supplymarket.ErrInvalid)
					return
				}
				if spec.platform {
					value, err = service.ExecutePlatform(ctx, key, input)
				} else {
					value, err = service.ExecuteMember(ctx, key, input)
				}
			case "upload":
				key := c.Request.Header.Values("Idempotency-Key")
				contentType := c.GetHeader("Content-Type")
				if files == nil {
					writeError(c, supplymarket.ErrUnavailable)
					return
				}
				if c.Request.URL.RawQuery != "" || len(key) != 1 || !collection.ValidID(key[0]) || len(c.Request.Header.Values("If-Match")) > 0 || c.GetHeader("Content-Encoding") != "" || c.Request.Body == nil || contentType != "image/png" && contentType != "image/jpeg" && contentType != "application/pdf" {
					writeError(c, supplymarket.ErrInvalid)
					return
				}
				raw, e := io.ReadAll(io.LimitReader(c.Request.Body, supplymarket.MaxQualificationBytes+1))
				if e != nil || len(raw) > supplymarket.MaxQualificationBytes {
					writeError(c, supplymarket.ErrInvalid)
					return
				}
				value, err = files.Upload(ctx, key[0], contentType, raw)
			case "download":
				if files == nil {
					writeError(c, supplymarket.ErrUnavailable)
					return
				}
				if c.Request.URL.RawQuery != "" {
					writeError(c, supplymarket.ErrInvalid)
					return
				}
				file, data, e := files.Download(ctx, c.Param("id"), c.Param("file_id"), spec.platform)
				if e != nil {
					writeError(c, e)
					return
				}
				extension := map[string]string{"image/png": ".png", "image/jpeg": ".jpg", "application/pdf": ".pdf"}[file.ContentType]
				if extension == "" || !collection.ValidID(file.ID) || int64(len(data)) != file.Size || len(data) > supplymarket.MaxQualificationBytes {
					writeError(c, supplymarket.ErrUnavailable)
					return
				}
				c.Header("Content-Disposition", `attachment; filename="qualification-`+file.ID+extension+`"`)
				c.Data(http.StatusOK, file.ContentType, data)
				return
			}
			if err != nil {
				writeError(c, err)
				return
			}
			raw, e := json.Marshal(value)
			if e != nil || len(raw) > 2<<20 {
				writeError(c, supplymarket.ErrUnavailable)
				return
			}
			c.Data(http.StatusOK, "application/json; charset=utf-8", raw)
		})})
	}
	return routes
}
func readQuery(raw string) (supplymarket.Query, error) {
	values, err := url.ParseQuery(raw)
	if err != nil {
		return supplymarket.Query{}, supplymarket.ErrInvalid
	}
	q := supplymarket.Query{Limit: 20}
	for key, value := range values {
		if len(value) != 1 {
			return q, supplymarket.ErrInvalid
		}
		switch key {
		case "after":
			q.After = value[0]
		case "keyword":
			q.Keyword = value[0]
		case "kind":
			q.Kind = value[0]
		case "limit":
			q.Limit, err = strconv.Atoi(value[0])
			if err != nil {
				return q, supplymarket.ErrInvalid
			}
		case "ended":
			if value[0] != "true" && value[0] != "false" {
				return q, supplymarket.ErrInvalid
			}
			q.Ended = value[0] == "true"
		default:
			return q, supplymarket.ErrInvalid
		}
	}
	if q.Validate() != nil {
		return q, supplymarket.ErrInvalid
	}
	return q, nil
}
func readMutation(request *http.Request) (string, supplymarket.Mutation, error) {
	var i supplymarket.Mutation
	keys := request.Header.Values("Idempotency-Key")
	media, params, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if len(keys) != 1 || !collection.ValidID(keys[0]) || request.URL.RawQuery != "" || request.Body == nil || err != nil || media != "application/json" || params["charset"] != "" && !strings.EqualFold(params["charset"], "utf-8") || request.Header.Get("Content-Encoding") != "" {
		return "", i, supplymarket.ErrInvalid
	}
	raw, err := io.ReadAll(io.LimitReader(request.Body, (64<<10)+1))
	if err != nil || len(raw) > 64<<10 || !utf8.Valid(raw) {
		return "", i, supplymarket.ErrInvalid
	}
	strict, err := sigjson.UnmarshalStrict(raw, &i, sigjson.DisallowDuplicateFields, sigjson.DisallowUnknownFields)
	if err != nil || len(strict) > 0 {
		return "", i, supplymarket.ErrInvalid
	}
	match := request.Header.Values("If-Match")
	if i.ID == "" {
		if len(match) > 0 || i.ExpectedRevision != 0 {
			return "", i, supplymarket.ErrInvalid
		}
	} else {
		if len(match) != 1 || i.ExpectedRevision < 1 || match[0] != `"`+strconv.FormatInt(i.ExpectedRevision, 10)+`"` {
			return "", i, supplymarket.ErrInvalid
		}
	}
	return keys[0], i, nil
}
func writeError(c *gin.Context, err error) {
	status, code := http.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE"
	switch {
	case errors.Is(err, supplymarket.ErrInvalid), errors.Is(err, collection.ErrInvalid):
		status, code = 400, "INVALID_REQUEST"
	case errors.Is(err, supplymarket.ErrForbidden), errors.Is(err, collection.ErrForbidden):
		status, code = 403, "PERMISSION_DENIED"
	case errors.Is(err, supplymarket.ErrNotFound), errors.Is(err, collection.ErrNotFound):
		status, code = 404, "NOT_FOUND"
	case errors.Is(err, supplymarket.ErrConflict), errors.Is(err, collection.ErrConflict):
		status, code = 409, "REVISION_CONFLICT"
	case errors.Is(err, supplymarket.ErrUnknown), errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		status, code = 409, "OUTCOME_UNKNOWN"
	}
	c.JSON(status, gin.H{"code": code})
}
