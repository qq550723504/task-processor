package supplychainhttp

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
	supplyapp "task-processor/internal/app/supplychain"
	"task-processor/internal/httproute"
	"task-processor/internal/listing/preparation"
	record "task-processor/internal/listing/record/target"
	"task-processor/internal/product/asset"
	"task-processor/internal/product/collection"
	"unicode/utf8"
)

const SupplyBasePath = "/api/v1/workbench/supply-preparations"
const supplyMaxBytes = 2 << 20

func SupplyRoutes(app *supplyapp.Application, bind func(context.Context, string) (context.Context, error)) []httproute.Descriptor {
	specs := []struct{ method, path, action, permission string }{
		{"GET", "/optimization-options", "optimization-options", preparation.PermissionManage},
		{"GET", "", "list", preparation.PermissionRead}, {"POST", "/transfer", "transfer", preparation.PermissionManage}, {"GET", "/transfers/by-key/:key", "transfer-read", preparation.PermissionRead},
		{"GET", "/:preparation_id/sources", "sources", preparation.PermissionRead}, {"GET", "/sources/:source_id", "source", preparation.PermissionRead},
		{"GET", "/:preparation_id/stages", "stages", preparation.PermissionRead},
		{"GET", "/:preparation_id", "preparation", preparation.PermissionRead}, {"GET", "/:preparation_id/operations", "operations", preparation.PermissionRead},
		{"GET", "/sources/:source_id/publications/:store_id", "publication", preparation.PermissionRead},
		{"GET", "/sources/:source_id/targets/:store_id", "target", preparation.PermissionRead}, {"POST", "/targets", "save-target", preparation.PermissionManage}, {"POST", "/target-rules", "rules", preparation.PermissionRead}, {"GET", "/target-commands/:key", "target-command", preparation.PermissionRead}, {"GET", "/records/:record_id", "record", preparation.PermissionRead},
		{"POST", "/uploads/resolve", "resolve-upload", preparation.PermissionManage},
		{"GET", "/uploads/:record_id/:attempt_id", "upload-attempt", preparation.PermissionRead},
		{"POST", "/images/approve", "approve", preparation.PermissionManage}, {"POST", "/images/inventory", "inventory", preparation.PermissionManage},
		{"POST", "/operations", "create-operation", preparation.PermissionManage}, {"GET", "/operations/by-key/:key", "operation-key", preparation.PermissionRead}, {"GET", "/operations/:operation_id", "operation", preparation.PermissionRead}, {"GET", "/operations/:operation_id/items", "operation-items", preparation.PermissionRead}, {"POST", "/operations/:operation_id/ensure", "ensure", preparation.PermissionManage}, {"POST", "/operations/:operation_id/cancel", "cancel", preparation.PermissionManage},
	}
	result := []httproute.Descriptor{}
	for _, spec := range specs {
		result = append(result, httproute.Descriptor{Method: spec.method, Path: SupplyBasePath + spec.path, Module: "supply-chain", Permission: spec.permission, AuthPolicy: httproute.AuthPolicyVerifiedIdentity, OrganizationAccessPolicy: httproute.OrganizationAccessPolicyLiveWrite, RequestTimeout: 20e9, Handler: httproute.WithRequestBodyReadTimeout(3e9, func(c *gin.Context) {
			c.Header("Cache-Control", "no-store")
			if app == nil || bind == nil {
				supplyError(c, preparation.ErrUnavailable)
				return
			}
			ctx, err := bind(c.Request.Context(), c.GetHeader("Authorization"))
			if err != nil {
				supplyError(c, preparation.ErrForbidden)
				return
			}
			var output any
			if spec.method == http.MethodGet || spec.action == "ensure" || spec.action == "cancel" {
				if e := supplyEmptyBody(c.Request); e != nil {
					supplyError(c, e)
					return
				}
			}
			if spec.action != "optimization-options" && spec.action != "list" && spec.action != "sources" && spec.action != "stages" && spec.action != "operation-items" && spec.action != "operations" && c.Request.URL.RawQuery != "" {
				supplyError(c, preparation.ErrInvalid)
				return
			}
			switch spec.action {
			case "upload-attempt":
				if c.Request.URL.RawQuery != "" {
					supplyError(c, preparation.ErrInvalid)
					return
				}
				output, err = app.ReadUploadAttempt(ctx, c.Param("record_id"), c.Param("attempt_id"))
			case "optimization-options":
				q, e := supplyQuery(c.Request)
				if e != nil || q.Keyword != "" {
					supplyError(c, preparation.ErrInvalid)
					return
				}
				if app.OptimizationOptions == nil {
					output = supplyapp.OptimizationOptions{Titles: []supplyapp.TitleOptimizationChoice{}, Reason: "智能体优化未配置"}
					break
				}
				output, err = app.OptimizationOptions(ctx, q)
			case "preparation":
				output, err = app.Preparations.Read(ctx, c.Param("preparation_id"))
			case "stages":
				values, e := url.ParseQuery(c.Request.URL.RawQuery)
				if e != nil || len(values["storeId"]) != 1 || !collection.ValidID(values.Get("storeId")) || len(values["stage"]) != 1 || !validStage(values.Get("stage")) || len(values["sourceKind"]) > 1 || values.Has("sourceKind") && !collection.ValidSourceKind(values.Get("sourceKind")) {
					supplyError(c, preparation.ErrInvalid)
					return
				}
				storeID, stage, sourceKind := values.Get("storeId"), values.Get("stage"), values.Get("sourceKind")
				values.Del("storeId")
				values.Del("stage")
				values.Del("sourceKind")
				request := c.Request.Clone(ctx)
				request.URL.RawQuery = values.Encode()
				q, e := supplyQuery(request)
				if e != nil {
					supplyError(c, e)
					return
				}
				output, err = app.Stages(ctx, c.Param("preparation_id"), storeID, stage, q, sourceKind)
			case "operations":
				values, e := url.ParseQuery(c.Request.URL.RawQuery)
				if e != nil || len(values["storeId"]) != 1 || !collection.ValidID(values.Get("storeId")) {
					supplyError(c, preparation.ErrInvalid)
					return
				}
				storeID := values.Get("storeId")
				values.Del("storeId")
				request := c.Request.Clone(ctx)
				request.URL.RawQuery = values.Encode()
				q, e := supplyQuery(request)
				if e != nil {
					supplyError(c, e)
					return
				}
				output, err = app.Operations.List(ctx, c.Param("preparation_id"), storeID, q)
			case "list", "sources", "operation-items":
				q, e := supplyQuery(c.Request)
				if e != nil {
					supplyError(c, e)
					return
				}
				switch spec.action {
				case "list":
					output, err = app.Preparations.List(ctx, q)
				case "sources":
					output, err = app.Preparations.ListSources(ctx, c.Param("preparation_id"), q)
				case "operation-items":
					output, err = app.OperationItems(ctx, c.Param("operation_id"), q)
				}
			case "transfer":
				var in preparation.TransferInput
				key, e := supplyBody(c.Request, &in, true)
				if e != nil {
					supplyError(c, e)
					return
				}
				output, err = app.Preparations.Transfer(ctx, key, in)
			case "transfer-read":
				output, err = app.Preparations.ReadByKey(ctx, c.Param("key"))
			case "source":
				output, err = app.Source(ctx, c.Param("source_id"))
			case "target":
				output, err = app.ReadTarget(ctx, c.Param("source_id"), c.Param("store_id"))
			case "publication":
				output, err = app.Publication(ctx, c.Param("source_id"), c.Param("store_id"))
			case "record":
				output, err = app.ReadRecord(ctx, c.Param("record_id"))
			case "target-command":
				output, err = app.Targets.ReadCommand(ctx, c.Param("key"))
			case "save-target", "rules":
				var in record.TargetInput
				key, e := supplyBody(c.Request, &in, spec.action == "save-target")
				if e != nil {
					supplyError(c, e)
					return
				}
				if spec.action == "save-target" {
					output, err = app.Targets.Create(ctx, key, in)
				} else {
					output, err = app.QueryRules(ctx, in)
				}
			case "resolve-upload":
				var in supplyapp.ResolveUploadInput
				_, e := supplyBody(c.Request, &in, false)
				if e != nil {
					supplyError(c, e)
					return
				}
				output, err = app.ResolveUpload(ctx, in)
			case "approve":
				var in asset.SourceApprovalCommand
				key, e := supplyBody(c.Request, &in, true)
				if e != nil {
					supplyError(c, e)
					return
				}
				if in.ActionID != "" && in.ActionID != key {
					supplyError(c, preparation.ErrInvalid)
					return
				}
				in.ActionID = key
				output, err = app.Approvals.Approve(ctx, in)
			case "inventory":
				var in asset.SourceSelectionRequest
				_, e := supplyBody(c.Request, &in, false)
				if e != nil {
					supplyError(c, e)
					return
				}
				output, err = app.Inventory(ctx, in)
			case "create-operation":
				var in preparation.OperationInput
				key, e := supplyBody(c.Request, &in, true)
				if e != nil {
					supplyError(c, e)
					return
				}
				if in.Action == preparation.OperationOptimize {
					if app.AuthorizeOptimization == nil {
						supplyError(c, record.ErrNotReady)
						return
					}
					if e := app.AuthorizeOptimization(ctx, in); e != nil {
						supplyError(c, e)
						return
					}
				}
				receipt, e := app.Operations.Create(ctx, key, in)
				if e != nil {
					supplyError(c, e)
					return
				}
				operation, e := app.Execution.EnsureExecution(ctx, receipt.Operation.ID)
				if e == nil || errors.Is(e, preparation.ErrUnknown) {
					receipt.Operation = operation
					output = receipt
				} else {
					output = receipt
				}
			case "operation-key":
				output, err = app.OperationByKey(ctx, c.Param("key"))
			case "operation":
				output, err = app.Operations.Read(ctx, c.Param("operation_id"))
			case "ensure", "cancel":
				if spec.action == "ensure" {
					output, err = app.Execution.EnsureExecution(ctx, c.Param("operation_id"))
				} else {
					output, err = app.Operations.Cancel(ctx, c.Param("operation_id"))
				}
			}
			if err != nil {
				supplyError(c, err)
				return
			}
			raw, e := json.Marshal(output)
			if e != nil || len(raw) > supplyMaxBytes {
				supplyError(c, preparation.ErrUnavailable)
				return
			}
			c.Data(http.StatusOK, "application/json; charset=utf-8", raw)
		})})
	}
	return result
}
func supplyBody(r *http.Request, in any, requiresKey bool) (string, error) {
	keys := r.Header.Values("Idempotency-Key")
	key := ""
	if len(keys) == 1 {
		key = keys[0]
	}
	if requiresKey && (len(keys) != 1 || !collection.ValidID(key)) || !requiresKey && len(keys) > 0 || r.URL.RawQuery != "" || r.Body == nil {
		return "", preparation.ErrInvalid
	}
	media, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" || params["charset"] != "" && !strings.EqualFold(params["charset"], "utf-8") || r.Header.Get("Content-Encoding") != "" && r.Header.Get("Content-Encoding") != "identity" {
		return "", preparation.ErrInvalid
	}
	raw, err := io.ReadAll(io.LimitReader(r.Body, supplyMaxBytes+1))
	if err != nil || len(raw) > supplyMaxBytes || !utf8.Valid(raw) {
		return "", preparation.ErrInvalid
	}
	strict, err := sigjson.UnmarshalStrict(raw, in, sigjson.DisallowDuplicateFields, sigjson.DisallowUnknownFields)
	if err != nil || len(strict) > 0 {
		return "", preparation.ErrInvalid
	}
	return key, nil
}
func supplyQuery(r *http.Request) (collection.Query, error) {
	values, parseErr := url.ParseQuery(r.URL.RawQuery)
	if parseErr != nil {
		return collection.Query{}, preparation.ErrInvalid
	}
	q := collection.Query{Limit: 50, After: values.Get("after"), Keyword: values.Get("keyword")}
	for key, list := range values {
		if len(list) != 1 || key != "limit" && key != "after" && key != "keyword" {
			return q, preparation.ErrInvalid
		}
	}
	var err error
	if values.Get("limit") != "" {
		q.Limit, err = strconv.Atoi(values.Get("limit"))
	}
	if err != nil || q.Validate() != nil {
		return q, preparation.ErrInvalid
	}
	return q, nil
}
func supplyEmptyBody(r *http.Request) error {
	if len(r.Header.Values("Idempotency-Key")) > 0 || r.ContentLength > 0 {
		return preparation.ErrInvalid
	}
	if r.Body != nil {
		raw, err := io.ReadAll(io.LimitReader(r.Body, 1))
		if err != nil || len(raw) > 0 {
			return preparation.ErrInvalid
		}
	}
	return nil
}
func supplyError(c *gin.Context, err error) {
	status, code := http.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE"
	switch {
	case errors.Is(err, preparation.ErrInvalid), errors.Is(err, record.ErrInvalid), errors.Is(err, asset.ErrInvalidApproval):
		status, code = http.StatusBadRequest, "INVALID_REQUEST"
	case errors.Is(err, preparation.ErrForbidden), errors.Is(err, record.ErrForbidden), errors.Is(err, asset.ErrSourceApprovalForbidden):
		status, code = http.StatusForbidden, "PERMISSION_DENIED"
	case errors.Is(err, preparation.ErrNotFound), errors.Is(err, record.ErrNotFound):
		status, code = http.StatusNotFound, "NOT_FOUND"
	case errors.Is(err, preparation.ErrConflict), errors.Is(err, record.ErrConflict), errors.Is(err, asset.ErrApprovalConflict):
		status, code = http.StatusConflict, "REVISION_CONFLICT"
	case errors.Is(err, record.ErrNotReady):
		status, code = http.StatusConflict, "NOT_READY"
	case errors.Is(err, preparation.ErrUnknown), errors.Is(err, record.ErrTargetUnknown), errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		status, code = http.StatusConflict, "OUTCOME_UNKNOWN"
	}
	c.JSON(status, gin.H{"code": code})
}

func validStage(v string) bool {
	switch v {
	case "all", "waiting", "missing", "ready", "review", "uploaded":
		return true
	}
	return false
}
