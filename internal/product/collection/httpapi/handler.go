package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	sigjson "sigs.k8s.io/json"
	"task-processor/internal/httproute"
	"task-processor/internal/product/collection"
)

const BasePath = "/api/v1/workbench/collections"

type Service interface {
	Mutate(context.Context, string, collection.Mutation) (collection.Receipt, error)
	ListBatches(context.Context, collection.Query) (collection.Page[collection.Batch], error)
	ListItems(context.Context, string, collection.Query) (collection.Page[collection.Item], error)
	ReadItem(context.Context, string) (collection.ItemDetail, error)
	ReadOperation(context.Context, string) (collection.Receipt, error)
}

func Routes(service Service, bind func(context.Context, string) (context.Context, error)) []httproute.Descriptor {
	specs := []struct{ method, path, action string }{
		{http.MethodGet, BasePath + "/batches", "batches"},
		{http.MethodGet, BasePath + "/own-products", "own"},
		{http.MethodGet, BasePath + "/batches/:batch_id/items", "items"},
		{http.MethodGet, BasePath + "/items/:item_id", "detail"},
		{http.MethodGet, BasePath + "/by-key/:key", "operation"},
		{http.MethodPost, BasePath + "/commands", "mutate"},
	}
	result := make([]httproute.Descriptor, 0, len(specs))
	for _, spec := range specs {
		permission := collection.PermissionRead
		if spec.action == "mutate" {
			permission = collection.PermissionManage
		}
		result = append(result, httproute.Descriptor{Method: spec.method, Path: spec.path, Module: "product-collection", Permission: permission, AuthPolicy: httproute.AuthPolicyVerifiedIdentity, OrganizationAccessPolicy: httproute.OrganizationAccessPolicyLiveWrite, RequestTimeout: collection.Timeout, Handler: httproute.WithRequestBodyReadTimeout(3e9, func(c *gin.Context) {
			c.Header("Cache-Control", "no-store")
			if service == nil || bind == nil {
				writeError(c, collection.ErrUnavailable)
				return
			}
			ctx, err := bind(c.Request.Context(), c.GetHeader("Authorization"))
			if err != nil {
				writeError(c, collection.ErrForbidden)
				return
			}
			var value any
			switch spec.action {
			case "batches", "items", "own":
				query, queryErr := parseQuery(c.Request)
				if queryErr != nil {
					writeError(c, queryErr)
					return
				}
				if spec.action == "batches" {
					value, err = service.ListBatches(ctx, query)
				} else {
					value, err = service.ListItems(ctx, c.Param("batch_id"), query)
				}
			case "detail", "operation":
				if c.Request.URL.RawQuery != "" {
					writeError(c, collection.ErrInvalid)
					return
				}
				if spec.action == "detail" {
					value, err = service.ReadItem(ctx, c.Param("item_id"))
				} else {
					value, err = service.ReadOperation(ctx, c.Param("key"))
				}
			case "mutate":
				key, input, inputErr := readMutation(c.Request)
				if inputErr != nil {
					writeError(c, inputErr)
					return
				}
				value, err = service.Mutate(ctx, key, input)
			}
			if err != nil {
				writeError(c, err)
				return
			}
			raw, err := json.Marshal(value)
			if err != nil || len(raw) > collection.MaxPayloadBytes {
				writeError(c, collection.ErrUnavailable)
				return
			}
			c.Data(http.StatusOK, "application/json; charset=utf-8", raw)
		})})
	}
	return result
}
func parseQuery(request *http.Request) (collection.Query, error) {
	values, err := urlQuery(request.URL.RawQuery)
	if err != nil {
		return collection.Query{}, err
	}
	query := collection.Query{After: values["after"], Keyword: values["keyword"], Limit: 50}
	if values["limit"] != "" {
		query.Limit, err = strconv.Atoi(values["limit"])
	}
	if err != nil || query.Validate() != nil {
		return collection.Query{}, collection.ErrInvalid
	}
	return query, nil
}
func readMutation(request *http.Request) (string, collection.Mutation, error) {
	var input collection.Mutation
	keys := request.Header.Values("Idempotency-Key")
	media, params, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if request.URL.RawQuery != "" || len(keys) != 1 || !collection.ValidID(keys[0]) || err != nil || media != "application/json" || params["charset"] != "" && !strings.EqualFold(params["charset"], "utf-8") || request.Header.Get("Content-Encoding") != "" && request.Header.Get("Content-Encoding") != "identity" || request.Body == nil {
		return "", input, collection.ErrInvalid
	}
	raw, err := io.ReadAll(io.LimitReader(request.Body, collection.MaxPayloadBytes+1))
	if err != nil || len(raw) > collection.MaxPayloadBytes || !utf8.Valid(raw) {
		return "", input, collection.ErrInvalid
	}
	strict, err := sigjson.UnmarshalStrict(raw, &input, sigjson.DisallowDuplicateFields, sigjson.DisallowUnknownFields)
	if err != nil || len(strict) > 0 {
		return "", input, collection.ErrInvalid
	}
	return keys[0], input, nil
}
func writeError(c *gin.Context, err error) {
	status, code := http.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE"
	switch {
	case errors.Is(err, collection.ErrInvalid):
		status, code = http.StatusBadRequest, "INVALID_REQUEST"
	case errors.Is(err, collection.ErrForbidden):
		status, code = http.StatusForbidden, "PERMISSION_DENIED"
	case errors.Is(err, collection.ErrNotFound):
		status, code = http.StatusNotFound, "NOT_FOUND"
	case errors.Is(err, collection.ErrConflict):
		status, code = http.StatusConflict, "REVISION_CONFLICT"
	case errors.Is(err, collection.ErrUnknown), errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		status, code = http.StatusConflict, "OUTCOME_UNKNOWN"
	}
	c.JSON(status, gin.H{"code": code})
}
