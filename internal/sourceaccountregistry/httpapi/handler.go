package httpapi

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"task-processor/internal/authidentity"
	"task-processor/internal/sourceaccountregistry"
)

const (
	requestBodyMaxBytes  = 8 * 1024
	requestQueryMaxBytes = 1024
	cursorMaxBytes       = 512
)

var strongVersionETag = regexp.MustCompile(`^"([1-9][0-9]*)"$`)

type Service interface {
	Register(context.Context, string, sourceaccountregistry.RegisterInput) (sourceaccountregistry.MutationResult, error)
	Get(context.Context, string) (sourceaccountregistry.Account, error)
	List(context.Context, sourceaccountregistry.PageRequest) (sourceaccountregistry.Page, error)
	Enable(context.Context, string, string, int64) (sourceaccountregistry.MutationResult, error)
	Disable(context.Context, string, string, int64) (sourceaccountregistry.MutationResult, error)
}

type Handler struct{ service Service }

func NewHandler(service Service) (*Handler, error) {
	if nilInterface(service) {
		return nil, sourceaccountregistry.ErrUnavailable
	}
	return &Handler{service: service}, nil
}

type SourceAccountResponse struct {
	ID               string    `json:"id"`
	Platform         string    `json:"platform"`
	DisplayName      string    `json:"displayName"`
	ManagementStatus string    `json:"managementStatus"`
	ConnectionStatus string    `json:"connectionStatus"`
	Version          string    `json:"version"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

type MutationResponse struct {
	SchemaVersion int                   `json:"schemaVersion"`
	Account       SourceAccountResponse `json:"account"`
	Replayed      bool                  `json:"replayed"`
}

type DetailResponse struct {
	SchemaVersion int                   `json:"schemaVersion"`
	Account       SourceAccountResponse `json:"account"`
}

type PageResponse struct {
	SchemaVersion int                     `json:"schemaVersion"`
	Items         []SourceAccountResponse `json:"items"`
	NextCursor    *string                 `json:"nextCursor"`
}

func (h *Handler) Create(c *gin.Context) {
	organizationID, ok := currentOrganization(c)
	if !ok {
		return
	}
	if err := requireNoQuery(c.Request.URL.RawQuery); err != nil {
		writeError(c, sourceaccountregistry.ErrInvalid)
		return
	}
	key, err := requiredCanonicalUUIDHeader(c.Request, "Idempotency-Key")
	if err != nil {
		writeError(c, sourceaccountregistry.ErrInvalid)
		return
	}
	input, err := parseRegisterInput(c.Request)
	if err != nil {
		writeError(c, err)
		return
	}
	result, err := h.service.Register(c.Request.Context(), key, input)
	if err != nil {
		writeError(c, err)
		return
	}
	status := http.StatusCreated
	if result.Replayed {
		status = http.StatusOK
	}
	writeMutation(c, status, result, organizationID, "")
}

func (h *Handler) Get(c *gin.Context) {
	organizationID, ok := currentOrganization(c)
	if !ok {
		return
	}
	if err := requireNoQuery(c.Request.URL.RawQuery); err != nil {
		writeError(c, sourceaccountregistry.ErrInvalid)
		return
	}
	if err := requireNoBody(c.Request.Body); err != nil {
		writeError(c, sourceaccountregistry.ErrInvalid)
		return
	}
	id := c.Param("source_account_id")
	if !validUUIDv7(id) {
		writeError(c, sourceaccountregistry.ErrInvalid)
		return
	}
	account, err := h.service.Get(c.Request.Context(), id)
	if err != nil {
		writeError(c, err)
		return
	}
	response, err := projectAccount(account, organizationID, id)
	if err != nil {
		writeError(c, err)
		return
	}
	protectedHeaders(c)
	c.Header("ETag", quoteVersion(account.Version))
	c.JSON(http.StatusOK, DetailResponse{SchemaVersion: 1, Account: response})
}

func (h *Handler) List(c *gin.Context) {
	organizationID, ok := currentOrganization(c)
	if !ok {
		return
	}
	if err := requireNoBody(c.Request.Body); err != nil {
		writeError(c, sourceaccountregistry.ErrInvalid)
		return
	}
	request, err := parsePageRequest(c.Request.URL.RawQuery, organizationID)
	if err != nil {
		writeError(c, err)
		return
	}
	page, err := h.service.List(c.Request.Context(), request)
	if err != nil {
		writeError(c, err)
		return
	}
	items := make([]SourceAccountResponse, 0, len(page.Items))
	for _, account := range page.Items {
		item, projectErr := projectAccount(account, organizationID, "")
		if projectErr != nil {
			writeError(c, projectErr)
			return
		}
		items = append(items, item)
	}
	var nextCursor *string
	if page.Next != nil {
		if len(page.Items) == 0 {
			writeError(c, sourceaccountregistry.ErrUnavailable)
			return
		}
		last := page.Items[len(page.Items)-1]
		if page.Next.ID != last.ID || !page.Next.CreatedAt.Equal(last.CreatedAt) {
			writeError(c, sourceaccountregistry.ErrUnavailable)
			return
		}
		encoded, encodeErr := encodeCursor(organizationID, *page.Next)
		if encodeErr != nil {
			writeError(c, sourceaccountregistry.ErrUnavailable)
			return
		}
		nextCursor = &encoded
	}
	protectedHeaders(c)
	c.JSON(http.StatusOK, PageResponse{SchemaVersion: 1, Items: items, NextCursor: nextCursor})
}

func (h *Handler) Enable(c *gin.Context)  { h.lifecycle(c, h.service.Enable) }
func (h *Handler) Disable(c *gin.Context) { h.lifecycle(c, h.service.Disable) }

func (h *Handler) lifecycle(c *gin.Context, mutate func(context.Context, string, string, int64) (sourceaccountregistry.MutationResult, error)) {
	organizationID, ok := currentOrganization(c)
	if !ok {
		return
	}
	if err := requireNoQuery(c.Request.URL.RawQuery); err != nil {
		writeError(c, sourceaccountregistry.ErrInvalid)
		return
	}
	id := c.Param("source_account_id")
	if !validUUIDv7(id) {
		writeError(c, sourceaccountregistry.ErrInvalid)
		return
	}
	key, err := requiredCanonicalUUIDHeader(c.Request, "Idempotency-Key")
	if err != nil {
		writeError(c, sourceaccountregistry.ErrInvalid)
		return
	}
	version, err := requiredIfMatch(c.Request)
	if err != nil {
		writeError(c, sourceaccountregistry.ErrInvalid)
		return
	}
	if err := requireNoBody(c.Request.Body); err != nil {
		writeError(c, sourceaccountregistry.ErrInvalid)
		return
	}
	result, err := mutate(c.Request.Context(), key, id, version)
	if err != nil {
		writeError(c, err)
		return
	}
	writeMutation(c, http.StatusOK, result, organizationID, id)
}

func writeMutation(c *gin.Context, status int, result sourceaccountregistry.MutationResult, organizationID, id string) {
	response, err := projectAccount(result.Account, organizationID, id)
	if err != nil {
		writeError(c, err)
		return
	}
	protectedHeaders(c)
	c.Header("ETag", quoteVersion(result.Account.Version))
	c.JSON(status, MutationResponse{SchemaVersion: 1, Account: response, Replayed: result.Replayed})
}

func projectAccount(account sourceaccountregistry.Account, organizationID, id string) (SourceAccountResponse, error) {
	if err := account.Validate(); err != nil || account.OrganizationID != organizationID || id != "" && account.ID != id {
		return SourceAccountResponse{}, sourceaccountregistry.ErrUnavailable
	}
	return SourceAccountResponse{
		ID: account.ID, Platform: string(account.Platform), DisplayName: account.DisplayName,
		ManagementStatus: string(account.ManagementStatus), ConnectionStatus: string(account.ConnectionStatus),
		Version: strconv.FormatInt(account.Version, 10), CreatedAt: account.CreatedAt.UTC(), UpdatedAt: account.UpdatedAt.UTC(),
	}, nil
}

func parseRegisterInput(request *http.Request) (sourceaccountregistry.RegisterInput, error) {
	if request == nil || len(request.Header.Values("Content-Type")) != 1 {
		return sourceaccountregistry.RegisterInput{}, sourceaccountregistry.ErrInvalid
	}
	mediaType, _, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return sourceaccountregistry.RegisterInput{}, sourceaccountregistry.ErrInvalid
	}
	data, err := readBoundedBody(request.Body)
	if err != nil {
		return sourceaccountregistry.RegisterInput{}, err
	}
	if !utf8.Valid(data) {
		return sourceaccountregistry.RegisterInput{}, sourceaccountregistry.ErrInvalid
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return sourceaccountregistry.RegisterInput{}, sourceaccountregistry.ErrInvalid
	}
	values := map[string]string{}
	for decoder.More() {
		fieldToken, tokenErr := decoder.Token()
		field, fieldOK := fieldToken.(string)
		if tokenErr != nil || !fieldOK || field != "displayName" && field != "platform" {
			return sourceaccountregistry.RegisterInput{}, sourceaccountregistry.ErrInvalid
		}
		if _, duplicate := values[field]; duplicate {
			return sourceaccountregistry.RegisterInput{}, sourceaccountregistry.ErrInvalid
		}
		var value string
		if decodeErr := decoder.Decode(&value); decodeErr != nil {
			return sourceaccountregistry.RegisterInput{}, sourceaccountregistry.ErrInvalid
		}
		values[field] = value
	}
	token, err = decoder.Token()
	if err != nil || token != json.Delim('}') || len(values) != 2 {
		return sourceaccountregistry.RegisterInput{}, sourceaccountregistry.ErrInvalid
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return sourceaccountregistry.RegisterInput{}, sourceaccountregistry.ErrInvalid
	}
	return sourceaccountregistry.RegisterInput{DisplayName: values["displayName"], Platform: values["platform"]}, nil
}

func readBoundedBody(body io.Reader) ([]byte, error) {
	if body == nil {
		return nil, sourceaccountregistry.ErrInvalid
	}
	data, err := io.ReadAll(io.LimitReader(body, requestBodyMaxBytes+1))
	if err != nil {
		return nil, sourceaccountregistry.ErrInvalid
	}
	if len(data) > requestBodyMaxBytes {
		return nil, sourceaccountregistry.ErrTooLarge
	}
	return data, nil
}

func parsePageRequest(rawQuery, organizationID string) (sourceaccountregistry.PageRequest, error) {
	if len([]byte(rawQuery)) > requestQueryMaxBytes {
		return sourceaccountregistry.PageRequest{}, sourceaccountregistry.ErrTooLarge
	}
	query, err := url.ParseQuery(rawQuery)
	if err != nil {
		return sourceaccountregistry.PageRequest{}, sourceaccountregistry.ErrInvalid
	}
	for key, values := range query {
		if key != "limit" && key != "cursor" || len(values) != 1 {
			return sourceaccountregistry.PageRequest{}, sourceaccountregistry.ErrInvalid
		}
	}
	request := sourceaccountregistry.PageRequest{Limit: sourceaccountregistry.DefaultPageLimit}
	if values, ok := query["limit"]; ok {
		if values[0] == "" || strings.HasPrefix(values[0], "0") || strings.ContainsAny(values[0], "+-") {
			return sourceaccountregistry.PageRequest{}, sourceaccountregistry.ErrInvalid
		}
		limit, parseErr := strconv.ParseInt(values[0], 10, 32)
		if parseErr != nil || limit < 1 || limit > sourceaccountregistry.MaxPageLimit {
			return sourceaccountregistry.PageRequest{}, sourceaccountregistry.ErrInvalid
		}
		request.Limit = int(limit)
	}
	if values, ok := query["cursor"]; ok {
		if len([]byte(values[0])) > cursorMaxBytes {
			return sourceaccountregistry.PageRequest{}, sourceaccountregistry.ErrTooLarge
		}
		position, decodeErr := decodeCursor(values[0], organizationID)
		if decodeErr != nil {
			return sourceaccountregistry.PageRequest{}, sourceaccountregistry.ErrInvalid
		}
		request.After = &position
	}
	return request, nil
}

type cursorPayload struct {
	Version          int    `json:"v"`
	OrganizationHash string `json:"o"`
	CreatedAt        string `json:"t"`
	ID               string `json:"id"`
}

func encodeCursor(organizationID string, position sourceaccountregistry.PagePosition) (string, error) {
	if organizationID == "" || position.CreatedAt.IsZero() || !validUUIDv7(position.ID) {
		return "", sourceaccountregistry.ErrInvalid
	}
	payload := cursorPayload{Version: 1, OrganizationHash: organizationHash(organizationID), CreatedAt: position.CreatedAt.UTC().Format(time.RFC3339Nano), ID: position.ID}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}

func decodeCursor(value, organizationID string) (sourceaccountregistry.PagePosition, error) {
	if value == "" || base64.RawURLEncoding.EncodeToString(mustDecodeBase64(value)) != value {
		return sourceaccountregistry.PagePosition{}, sourceaccountregistry.ErrInvalid
	}
	data, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return sourceaccountregistry.PagePosition{}, sourceaccountregistry.ErrInvalid
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var payload cursorPayload
	if err := decoder.Decode(&payload); err != nil {
		return sourceaccountregistry.PagePosition{}, sourceaccountregistry.ErrInvalid
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return sourceaccountregistry.PagePosition{}, sourceaccountregistry.ErrInvalid
	}
	canonical, err := json.Marshal(payload)
	if err != nil || !bytes.Equal(canonical, data) || payload.Version != 1 || payload.OrganizationHash != organizationHash(organizationID) || !validUUIDv7(payload.ID) {
		return sourceaccountregistry.PagePosition{}, sourceaccountregistry.ErrInvalid
	}
	createdAt, err := time.Parse(time.RFC3339Nano, payload.CreatedAt)
	if err != nil || createdAt.IsZero() || createdAt.Location() != time.UTC || createdAt.Format(time.RFC3339Nano) != payload.CreatedAt {
		return sourceaccountregistry.PagePosition{}, sourceaccountregistry.ErrInvalid
	}
	return sourceaccountregistry.PagePosition{CreatedAt: createdAt, ID: payload.ID}, nil
}

func mustDecodeBase64(value string) []byte {
	data, _ := base64.RawURLEncoding.DecodeString(value)
	return data
}

func organizationHash(organizationID string) string {
	sum := sha256.Sum256([]byte(organizationID))
	return hex.EncodeToString(sum[:])
}

func currentOrganization(c *gin.Context) (string, bool) {
	identity, ok := authidentity.AuthenticatedIdentityFromContext(c.Request.Context())
	if !ok {
		writeProtocolError(c, http.StatusUnauthorized, "AUTHENTICATION_REQUIRED", "Authentication is required")
		return "", false
	}
	if identity.EffectiveOrganizationID == "" {
		writeProtocolError(c, http.StatusConflict, "ORGANIZATION_SELECTION_REQUIRED", "Select an organization to continue")
		return "", false
	}
	return identity.EffectiveOrganizationID, true
}

func requiredCanonicalUUIDHeader(request *http.Request, name string) (string, error) {
	if request == nil || len(request.Header.Values(name)) != 1 {
		return "", sourceaccountregistry.ErrInvalid
	}
	value := request.Header.Values(name)[0]
	parsed, err := uuid.Parse(value)
	if err != nil || parsed == uuid.Nil || parsed.String() != value || parsed.Variant() != uuid.RFC4122 {
		return "", sourceaccountregistry.ErrInvalid
	}
	return value, nil
}

func requiredIfMatch(request *http.Request) (int64, error) {
	if request == nil || len(request.Header.Values("If-Match")) != 1 {
		return 0, sourceaccountregistry.ErrInvalid
	}
	match := strongVersionETag.FindStringSubmatch(request.Header.Values("If-Match")[0])
	if match == nil {
		return 0, sourceaccountregistry.ErrInvalid
	}
	version, err := strconv.ParseInt(match[1], 10, 64)
	if err != nil || version <= 0 {
		return 0, sourceaccountregistry.ErrInvalid
	}
	return version, nil
}

func requireNoBody(body io.Reader) error {
	if body == nil {
		return nil
	}
	data, err := io.ReadAll(io.LimitReader(body, 1))
	if err != nil || len(data) != 0 {
		return sourceaccountregistry.ErrInvalid
	}
	return nil
}

func requireNoQuery(rawQuery string) error {
	query, err := url.ParseQuery(rawQuery)
	if err != nil || len(query) != 0 {
		return sourceaccountregistry.ErrInvalid
	}
	return nil
}

func validUUIDv7(value string) bool {
	parsed, err := uuid.Parse(value)
	return err == nil && parsed != uuid.Nil && parsed.String() == value && parsed.Variant() == uuid.RFC4122 && parsed.Version() == 7
}

func quoteVersion(version int64) string { return fmt.Sprintf("\"%d\"", version) }

func protectedHeaders(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Header("X-Content-Type-Options", "nosniff")
}

func writeError(c *gin.Context, err error) {
	if c.Request.Context().Err() != nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		writeProtocolError(c, http.StatusGatewayTimeout, "DEADLINE_EXCEEDED", "Request deadline exceeded")
		return
	}
	switch {
	case errors.Is(err, sourceaccountregistry.ErrInvalid):
		writeProtocolError(c, http.StatusBadRequest, "INVALID_REQUEST", "Request is invalid")
	case errors.Is(err, sourceaccountregistry.ErrForbidden):
		writeProtocolError(c, http.StatusForbidden, "PERMISSION_DENIED", "Permission is denied")
	case errors.Is(err, sourceaccountregistry.ErrNotFound):
		writeProtocolError(c, http.StatusNotFound, "SOURCE_ACCOUNT_NOT_FOUND", "Source account was not found")
	case errors.Is(err, sourceaccountregistry.ErrIdempotencyConflict):
		writeProtocolError(c, http.StatusConflict, "IDEMPOTENCY_CONFLICT", "Idempotency key conflicts with an earlier request")
	case errors.Is(err, sourceaccountregistry.ErrVersionConflict):
		writeProtocolError(c, http.StatusConflict, "VERSION_CONFLICT", "Source account version conflicts")
	case errors.Is(err, sourceaccountregistry.ErrInvalidTransition):
		writeProtocolError(c, http.StatusConflict, "INVALID_TRANSITION", "Source account transition is invalid")
	case errors.Is(err, sourceaccountregistry.ErrResourceLimitReached):
		writeProtocolError(c, http.StatusConflict, "RESOURCE_LIMIT_REACHED", "Source account resource limit was reached")
	case errors.Is(err, sourceaccountregistry.ErrTooLarge):
		writeProtocolError(c, http.StatusRequestEntityTooLarge, "INPUT_TOO_LARGE", "Request is too large")
	case errors.Is(err, sourceaccountregistry.ErrOutcomeUnknown):
		writeProtocolError(c, http.StatusServiceUnavailable, "OUTCOME_UNKNOWN", "Transaction outcome is unknown; retry the same request")
	default:
		writeProtocolError(c, http.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE", "Source account service is temporarily unavailable")
	}
}

func writeProtocolError(c *gin.Context, status int, code, message string) {
	protectedHeaders(c)
	c.AbortWithStatusJSON(status, gin.H{"code": code, "message": message, "requestId": strings.TrimSpace(c.GetHeader("X-Request-ID")), "fieldErrors": []any{}})
}

func nilInterface(value any) bool {
	if value == nil {
		return true
	}
	rv := reflect.ValueOf(value)
	switch rv.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return rv.IsNil()
	default:
		return false
	}
}
