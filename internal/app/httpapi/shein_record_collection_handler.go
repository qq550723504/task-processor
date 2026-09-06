package httpapi

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"time"

	"task-processor/internal/authz"
	"task-processor/internal/httproute"
	"task-processor/internal/listing/record"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	sigjson "sigs.k8s.io/json"
)

const maxSheinRecordCollectionQueryBytes = 1024
const maxSheinRecordCollectionResponseBytes = 128 << 10

var collectionLimitPattern = regexp.MustCompile(`^[1-9][0-9]{0,2}$`)

type sheinRecordCursorDTO struct {
	Version   int    `json:"v"`
	CreatedAt string `json:"created_at"`
	RecordID  string `json:"record_id"`
}

type sheinRecordCollectionItemDTO struct {
	RecordID        string `json:"record_id"`
	ProductKey      string `json:"product_key"`
	SnapshotVersion string `json:"snapshot_version"`
	Country         string `json:"country"`
	Language        string `json:"language"`
	CreatedAt       string `json:"created_at"`
}

type sheinRecordCollectionDTO struct {
	Items      []sheinRecordCollectionItemDTO `json:"items"`
	NextCursor *string                        `json:"next_cursor"`
}

func sheinRecordCollectionRoutes(service *record.CollectionService) []httproute.Descriptor {
	if service == nil {
		return nil
	}
	return []httproute.Descriptor{{Method: http.MethodGet, Path: sheinRecordPath, Module: "listing-record", Permission: authz.PermissionListingKitAdminRead, AuthPolicy: httproute.AuthPolicyVerifiedIdentity, OrganizationAccessPolicy: httproute.OrganizationAccessPolicyCachedRead, Handler: func(c *gin.Context) { listSheinRecords(c, service) }}}
}

func listSheinRecords(c *gin.Context, service *record.CollectionService) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), record.Timeout)
	defer cancel()
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, 1))
	var transport net.Error
	if ctx.Err() != nil || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) || (errors.As(err, &transport) && transport.Timeout()) {
		writeSheinRecordCollectionError(c, context.DeadlineExceeded)
		return
	}
	rawQuery := c.Request.URL.RawQuery
	query, queryErr := url.ParseQuery(rawQuery)
	if err != nil || len(body) != 0 || len(rawQuery) > maxSheinRecordCollectionQueryBytes || queryErr != nil {
		writeSheinRecordCollectionError(c, record.ErrInvalid)
		return
	}
	request, err := parseSheinRecordPageQuery(query)
	if err != nil {
		writeSheinRecordCollectionError(c, err)
		return
	}
	page, err := service.List(ctx, request)
	if err != nil {
		writeSheinRecordCollectionError(c, err)
		return
	}
	wire, err := marshalSheinRecordPage(page)
	if ctx.Err() != nil {
		writeSheinRecordCollectionError(c, ctx.Err())
		return
	}
	if err != nil || len(wire) > maxSheinRecordCollectionResponseBytes {
		writeSheinRecordCollectionError(c, record.ErrUnavailable)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Data(http.StatusOK, "application/json; charset=utf-8", wire)
}

func parseSheinRecordPageQuery(query url.Values) (record.PageRequest, error) {
	for key, values := range query {
		if (key != "limit" && key != "cursor") || len(values) != 1 {
			return record.PageRequest{}, record.ErrInvalid
		}
	}
	limit := 20
	if raw, ok := query["limit"]; ok {
		if !collectionLimitPattern.MatchString(raw[0]) {
			return record.PageRequest{}, record.ErrInvalid
		}
		parsed, err := strconv.Atoi(raw[0])
		if err != nil || parsed > record.MaxPageSize {
			return record.PageRequest{}, record.ErrInvalid
		}
		limit = parsed
	}
	request := record.PageRequest{Limit: limit}
	if raw, ok := query["cursor"]; ok {
		cursor, err := decodeSheinRecordCursor(raw[0])
		if err != nil {
			return record.PageRequest{}, err
		}
		request.Cursor = &cursor
	}
	return request, request.Validate()
}

func encodeSheinRecordCursor(cursor record.PageCursor) (string, error) {
	if err := (record.PageRequest{Limit: 1, Cursor: &cursor}).Validate(); err != nil {
		return "", err
	}
	wire, err := json.Marshal(sheinRecordCursorDTO{Version: 1, CreatedAt: cursor.CreatedAt.UTC().Format(time.RFC3339Nano), RecordID: cursor.ID})
	if err != nil {
		return "", record.ErrInvalid
	}
	return base64.RawURLEncoding.EncodeToString(wire), nil
}

func decodeSheinRecordCursor(value string) (record.PageCursor, error) {
	if value == "" || len(value) > 512 {
		return record.PageCursor{}, record.ErrInvalid
	}
	wire, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return record.PageCursor{}, record.ErrInvalid
	}
	var dto sheinRecordCursorDTO
	violations, err := sigjson.UnmarshalStrict(wire, &dto)
	if err != nil || len(violations) != 0 || dto.Version != 1 {
		return record.PageCursor{}, record.ErrInvalid
	}
	created, err := time.Parse(time.RFC3339Nano, dto.CreatedAt)
	parsedID, idErr := uuid.Parse(dto.RecordID)
	if err != nil || idErr != nil || parsedID.String() != dto.RecordID || created.Location() != time.UTC || created.Format(time.RFC3339Nano) != dto.CreatedAt {
		return record.PageCursor{}, record.ErrInvalid
	}
	cursor := record.PageCursor{ID: dto.RecordID, CreatedAt: created}
	canonical, err := encodeSheinRecordCursor(cursor)
	if err != nil || canonical != value {
		return record.PageCursor{}, record.ErrInvalid
	}
	return cursor, nil
}

func marshalSheinRecordPage(page record.Page) ([]byte, error) {
	items := make([]sheinRecordCollectionItemDTO, 0, len(page.Items))
	for _, item := range page.Items {
		items = append(items, sheinRecordCollectionItemDTO{RecordID: item.ID, ProductKey: item.Input.ProductKey, SnapshotVersion: strconv.FormatUint(item.Input.SnapshotVersion, 10), Country: item.Input.Country, Language: item.Input.Language, CreatedAt: item.CreatedAt.UTC().Format(time.RFC3339Nano)})
	}
	var next *string
	if page.NextCursor != nil {
		encoded, err := encodeSheinRecordCursor(*page.NextCursor)
		if err != nil {
			return nil, err
		}
		next = &encoded
	}
	return json.Marshal(sheinRecordCollectionDTO{Items: items, NextCursor: next})
}

func writeSheinRecordCollectionError(c *gin.Context, err error) {
	status, code := http.StatusServiceUnavailable, "unavailable"
	switch {
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		status, code = http.StatusGatewayTimeout, "deadline_exceeded"
	case errors.Is(err, record.ErrForbidden):
		status, code = http.StatusForbidden, "permission_denied"
	case errors.Is(err, record.ErrInvalid):
		status, code = http.StatusBadRequest, "invalid_request"
	}
	c.Header("Cache-Control", "no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	c.JSON(status, gin.H{"error": code})
}
