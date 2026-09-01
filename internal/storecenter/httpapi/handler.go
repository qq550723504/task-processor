package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"task-processor/internal/authidentity"
	"task-processor/internal/storecenter"
)

const requestBodyMaxBytes = 16 * 1024

var (
	canonicalDecimal = regexp.MustCompile(`^(0|[1-9][0-9]*)$`)
	positiveETag     = regexp.MustCompile(`^"([1-9][0-9]*)"$`)
)

type StoreService interface {
	List(context.Context, storecenter.ListStoresRequest) (storecenter.ListStoresResult, error)
	Create(context.Context, storecenter.CreateStoreRequest) (storecenter.CreateStoreResult, error)
	ResumeCreate(context.Context, storecenter.ResumeCreateStoreRequest) (storecenter.CreateStoreResult, error)
	Get(context.Context, storecenter.GetStoreRequest) (storecenter.StoreProjection, error)
	Update(context.Context, storecenter.UpdateStoreRequest) (storecenter.StoreMutationResult, error)
	Disable(context.Context, storecenter.StoreLifecycleRequest) (storecenter.StoreMutationResult, error)
	Enable(context.Context, storecenter.StoreLifecycleRequest) (storecenter.StoreMutationResult, error)
	Delete(context.Context, storecenter.DeleteStoreRequest) (storecenter.DeleteStoreResult, error)
}

type Handler struct{ service StoreService }

func NewHandler(service StoreService) (*Handler, error) {
	if isNilInterface(service) {
		return nil, errors.New("store service is required")
	}
	return &Handler{service: service}, nil
}

type StoreResponse struct {
	ID               string                       `json:"id"`
	Name             string                       `json:"name"`
	Platform         string                       `json:"platform"`
	Region           string                       `json:"region"`
	ExternalStoreID  string                       `json:"externalStoreId"`
	LifecycleStatus  storecenter.LifecycleStatus  `json:"lifecycleStatus"`
	ConnectionStatus storecenter.ConnectionStatus `json:"connectionStatus"`
	Version          int64                        `json:"version"`
	CreatedAt        time.Time                    `json:"createdAt"`
	UpdatedAt        time.Time                    `json:"updatedAt"`
}

type QuotaResponse struct {
	Used     int64  `json:"used"`
	Reserved int64  `json:"reserved"`
	Limit    *int64 `json:"limit"`
	Allowed  bool   `json:"allowed"`
	Reason   string `json:"reason"`
}

type PaginationResponse struct {
	Page     int   `json:"page"`
	PageSize int   `json:"pageSize"`
	Total    int64 `json:"total"`
}

type ListStoresResponse struct {
	Items      []StoreResponse    `json:"items"`
	Quota      QuotaResponse      `json:"quota"`
	Pagination PaginationResponse `json:"pagination"`
}

type DeleteStoreResponse struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
	Version int64  `json:"version"`
}

func (h *Handler) List(c *gin.Context) {
	identity, ok := authoritativeIdentity(c)
	if !ok {
		return
	}
	request, field, err := parseListRequest(c.Request.URL.RawQuery)
	if err != nil {
		writeInvalid(c, field, "invalid")
		return
	}
	request.OrganizationID = identity.EffectiveOrganizationID
	result, err := h.service.List(c.Request.Context(), request)
	if err != nil {
		writeStoreError(c, err)
		return
	}
	response, err := listResponse(result, request)
	if err != nil {
		writeStoreError(c, err)
		return
	}
	c.JSON(http.StatusOK, response)
}

func (h *Handler) Create(c *gin.Context) {
	identity, ok := authoritativeIdentity(c)
	if !ok {
		return
	}
	if err := requireNoQuery(c.Request.URL.RawQuery); err != nil {
		writeInvalid(c, "query", "not_allowed")
		return
	}
	key, err := requiredCanonicalUUIDHeader(c.Request, "Idempotency-Key")
	if err != nil {
		writeInvalid(c, "Idempotency-Key", "invalid")
		return
	}
	values, field, err := parseStringObject(c.Request.Body, map[string]bool{"name": true, "platform": true, "region": true, "externalStoreId": false})
	if err != nil {
		writeInvalid(c, field, "invalid")
		return
	}
	name, err := normalizePublicValue(values["name"], storecenter.MaxStoreNameCodePoints, true)
	if err != nil {
		writeInvalid(c, "name", "invalid")
		return
	}
	if values["platform"] != string(storecenter.PlatformShein) {
		writeInvalid(c, "platform", "invalid")
		return
	}
	region, err := normalizePublicValue(values["region"], storecenter.MaxStoreRegionCodePoints, true)
	if err != nil {
		writeInvalid(c, "region", "invalid")
		return
	}
	externalID, err := normalizePublicValue(values["externalStoreId"], storecenter.MaxExternalStoreIDCodePoints, false)
	if err != nil {
		writeInvalid(c, "externalStoreId", "invalid")
		return
	}
	result, err := h.service.Create(c.Request.Context(), storecenter.CreateStoreRequest{OrganizationID: identity.EffectiveOrganizationID, ActorSubject: identity.UserID, IdempotencyKey: key, Name: name, Platform: values["platform"], Region: region, ExternalStoreID: externalID})
	if err != nil {
		writeStoreError(c, err)
		return
	}
	h.writeCreateResponse(c, result, identity.EffectiveOrganizationID, http.StatusCreated)
}

func (h *Handler) ResumeCreate(c *gin.Context) {
	identity, storeID, ok := h.itemIdentity(c)
	if !ok {
		return
	}
	version, err := requiredIfMatch(c.Request)
	if err != nil {
		writeInvalid(c, "If-Match", "invalid")
		return
	}
	if err := requireNoBody(c.Request.Body); err != nil {
		writeInvalid(c, "body", "not_allowed")
		return
	}
	result, err := h.service.ResumeCreate(c.Request.Context(), storecenter.ResumeCreateStoreRequest{
		OrganizationID:  identity.EffectiveOrganizationID,
		ActorSubject:    identity.UserID,
		StoreID:         storeID,
		ExpectedVersion: version,
	})
	if err != nil {
		writeStoreError(c, err)
		return
	}
	h.writeCreateResponse(c, result, identity.EffectiveOrganizationID, http.StatusOK)
}

func (h *Handler) writeCreateResponse(c *gin.Context, result storecenter.CreateStoreResult, organizationID string, status int) {
	if result.Store == nil || result.Store.OrganizationID() != organizationID {
		writeStoreError(c, storecenter.ErrDependencyUnavailable)
		return
	}
	storeID, err := canonicalUUID(result.Store.ID())
	if err != nil {
		writeStoreError(c, storecenter.ErrDependencyUnavailable)
		return
	}
	projection, err := h.service.Get(c.Request.Context(), storecenter.GetStoreRequest{OrganizationID: organizationID, StoreID: storeID})
	if err != nil {
		writeStoreError(c, storecenter.ErrDependencyUnavailable)
		return
	}
	response, err := projectionResponse(projection, organizationID, storeID)
	if err != nil {
		writeStoreError(c, err)
		return
	}
	c.JSON(status, response)
}

func (h *Handler) Get(c *gin.Context) {
	identity, storeID, ok := h.itemIdentity(c)
	if !ok {
		return
	}
	result, err := h.service.Get(c.Request.Context(), storecenter.GetStoreRequest{OrganizationID: identity.EffectiveOrganizationID, StoreID: storeID})
	if err != nil {
		writeStoreError(c, err)
		return
	}
	response, err := projectionResponse(result, identity.EffectiveOrganizationID, storeID)
	if err != nil {
		writeStoreError(c, err)
		return
	}
	c.JSON(http.StatusOK, response)
}

func (h *Handler) Update(c *gin.Context) {
	identity, storeID, ok := h.mutationIdentity(c, true)
	if !ok {
		return
	}
	version, err := requiredIfMatch(c.Request)
	if err != nil {
		writeInvalid(c, "If-Match", "invalid")
		return
	}
	values, field, err := parseStringObject(c.Request.Body, map[string]bool{"name": true, "region": true})
	if err != nil {
		writeInvalid(c, field, "invalid")
		return
	}
	name, err := normalizePublicValue(values["name"], storecenter.MaxStoreNameCodePoints, true)
	if err != nil {
		writeInvalid(c, "name", "invalid")
		return
	}
	region, err := normalizePublicValue(values["region"], storecenter.MaxStoreRegionCodePoints, true)
	if err != nil {
		writeInvalid(c, "region", "invalid")
		return
	}
	result, err := h.service.Update(c.Request.Context(), storecenter.UpdateStoreRequest{OrganizationID: identity.EffectiveOrganizationID, ActorSubject: identity.UserID, StoreID: storeID, ExpectedVersion: version, Name: name, Region: region})
	if err != nil {
		writeStoreError(c, err)
		return
	}
	h.writeMutation(c, result, identity.EffectiveOrganizationID, storeID)
}

func (h *Handler) Disable(c *gin.Context) { h.lifecycle(c, h.service.Disable) }
func (h *Handler) Enable(c *gin.Context)  { h.lifecycle(c, h.service.Enable) }

func (h *Handler) lifecycle(c *gin.Context, mutate func(context.Context, storecenter.StoreLifecycleRequest) (storecenter.StoreMutationResult, error)) {
	identity, storeID, ok := h.mutationIdentity(c, true)
	if !ok {
		return
	}
	version, err := requiredIfMatch(c.Request)
	if err != nil {
		writeInvalid(c, "If-Match", "invalid")
		return
	}
	if err := requireNoBody(c.Request.Body); err != nil {
		writeInvalid(c, "body", "not_allowed")
		return
	}
	result, err := mutate(c.Request.Context(), storecenter.StoreLifecycleRequest{OrganizationID: identity.EffectiveOrganizationID, ActorSubject: identity.UserID, StoreID: storeID, ExpectedVersion: version})
	if err != nil {
		writeStoreError(c, err)
		return
	}
	h.writeMutation(c, result, identity.EffectiveOrganizationID, storeID)
}

func (h *Handler) Delete(c *gin.Context) {
	identity, storeID, ok := h.mutationIdentity(c, false)
	if !ok {
		return
	}
	version, err := requiredIfMatch(c.Request)
	if err != nil {
		writeInvalid(c, "If-Match", "invalid")
		return
	}
	key, err := requiredCanonicalUUIDHeader(c.Request, "Idempotency-Key")
	if err != nil {
		writeInvalid(c, "Idempotency-Key", "invalid")
		return
	}
	if err := requireNoBody(c.Request.Body); err != nil {
		writeInvalid(c, "body", "not_allowed")
		return
	}
	result, err := h.service.Delete(c.Request.Context(), storecenter.DeleteStoreRequest{OrganizationID: identity.EffectiveOrganizationID, ActorSubject: identity.UserID, StoreID: storeID, ExpectedVersion: version, OperationKey: key})
	if err != nil {
		writeStoreError(c, err)
		return
	}
	if result.StoreID != storeID || result.Version <= 0 {
		writeStoreError(c, storecenter.ErrDependencyUnavailable)
		return
	}
	c.JSON(http.StatusOK, DeleteStoreResponse{ID: result.StoreID, Deleted: true, Version: result.Version})
}

func (h *Handler) writeMutation(c *gin.Context, result storecenter.StoreMutationResult, organizationID, storeID string) {
	response, err := projectionResponse(result.Store, organizationID, storeID)
	if err != nil {
		writeStoreError(c, err)
		return
	}
	c.JSON(http.StatusOK, response)
}

func (h *Handler) itemIdentity(c *gin.Context) (authidentity.AuthenticatedIdentity, string, bool) {
	identity, ok := authoritativeIdentity(c)
	if !ok {
		return authidentity.AuthenticatedIdentity{}, "", false
	}
	if err := requireNoQuery(c.Request.URL.RawQuery); err != nil {
		writeInvalid(c, "query", "not_allowed")
		return authidentity.AuthenticatedIdentity{}, "", false
	}
	storeID, err := canonicalUUID(c.Param("store_id"))
	if err != nil {
		writeInvalid(c, "store_id", "invalid")
		return authidentity.AuthenticatedIdentity{}, "", false
	}
	return identity, storeID, true
}

func (h *Handler) mutationIdentity(c *gin.Context, forbidIdempotency bool) (authidentity.AuthenticatedIdentity, string, bool) {
	identity, storeID, ok := h.itemIdentity(c)
	if !ok {
		return authidentity.AuthenticatedIdentity{}, "", false
	}
	if forbidIdempotency && len(c.Request.Header.Values("Idempotency-Key")) != 0 {
		writeInvalid(c, "Idempotency-Key", "not_allowed")
		return authidentity.AuthenticatedIdentity{}, "", false
	}
	return identity, storeID, true
}

func authoritativeIdentity(c *gin.Context) (authidentity.AuthenticatedIdentity, bool) {
	identity, ok := authidentity.AuthenticatedIdentityFromContext(c.Request.Context())
	if !ok {
		writeError(c, http.StatusUnauthorized, "AUTHENTICATION_REQUIRED", "Authentication is required", nil)
		return authidentity.AuthenticatedIdentity{}, false
	}
	if strings.TrimSpace(identity.EffectiveOrganizationID) == "" {
		writeError(c, http.StatusConflict, "ORGANIZATION_SELECTION_REQUIRED", "An Organization must be selected", nil)
		return authidentity.AuthenticatedIdentity{}, false
	}
	return identity, true
}

func parseListRequest(rawQuery string) (storecenter.ListStoresRequest, string, error) {
	query, err := url.ParseQuery(rawQuery)
	if err != nil {
		return storecenter.ListStoresRequest{}, "query", err
	}
	allowed := map[string]bool{"page": true, "pageSize": true, "platform": true, "status": true}
	for key, values := range query {
		if !allowed[key] || len(values) != 1 {
			return storecenter.ListStoresRequest{}, key, errors.New("invalid query")
		}
	}
	request := storecenter.ListStoresRequest{Page: 1, PageSize: 20}
	if value, ok := query["page"]; ok {
		parsed, err := parseCanonicalInt(value[0], 1, int64(^uint(0)>>1))
		if err != nil {
			return storecenter.ListStoresRequest{}, "page", err
		}
		request.Page = int(parsed)
	}
	if value, ok := query["pageSize"]; ok {
		parsed, err := parseCanonicalInt(value[0], 1, 100)
		if err != nil {
			return storecenter.ListStoresRequest{}, "pageSize", err
		}
		request.PageSize = int(parsed)
	}
	maxInt := int64(^uint(0) >> 1)
	if int64(request.Page-1) > maxInt/int64(request.PageSize) {
		return storecenter.ListStoresRequest{}, "page", errors.New("page offset overflows")
	}
	if value, ok := query["platform"]; ok {
		if value[0] != string(storecenter.PlatformShein) {
			return storecenter.ListStoresRequest{}, "platform", errors.New("invalid platform")
		}
		request.Platform = value[0]
	}
	if value, ok := query["status"]; ok {
		status := storecenter.LifecycleStatus(value[0])
		switch status {
		case storecenter.StoreStatusProvisioning, storecenter.StoreStatusActive, storecenter.StoreStatusDisabled, storecenter.StoreStatusDeleting:
			request.Status = status
		default:
			return storecenter.ListStoresRequest{}, "status", errors.New("invalid status")
		}
	}
	return request, "", nil
}

func parseStringObject(body io.Reader, fields map[string]bool) (map[string]string, string, error) {
	data, err := readBoundedBody(body)
	if err != nil || !utf8.Valid(data) {
		return nil, "body", errors.New("invalid body")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return nil, "body", errors.New("body must be object")
	}
	values := make(map[string]string, len(fields))
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return nil, "body", err
		}
		name, ok := token.(string)
		if !ok {
			return nil, "body", errors.New("invalid field")
		}
		required, allowed := fields[name]
		_ = required
		if !allowed {
			return nil, name, errors.New("unknown field")
		}
		if _, duplicate := values[name]; duplicate {
			return nil, name, errors.New("duplicate field")
		}
		var value string
		if err := decoder.Decode(&value); err != nil {
			return nil, name, errors.New("field must be string")
		}
		values[name] = value
	}
	token, err = decoder.Token()
	if err != nil || token != json.Delim('}') {
		return nil, "body", errors.New("invalid body")
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, "body", errors.New("trailing JSON")
	}
	requiredFields := make([]string, 0, len(fields))
	for name, required := range fields {
		if required {
			requiredFields = append(requiredFields, name)
		}
	}
	sort.Strings(requiredFields)
	for _, name := range requiredFields {
		if _, ok := values[name]; !ok {
			return nil, name, errors.New("missing field")
		}
	}
	return values, "", nil
}

func readBoundedBody(body io.Reader) ([]byte, error) {
	if body == nil {
		return nil, errors.New("body required")
	}
	data, err := io.ReadAll(io.LimitReader(body, requestBodyMaxBytes+1))
	if err != nil || len(data) > requestBodyMaxBytes {
		return nil, errors.New("body too large or unreadable")
	}
	return data, nil
}

func requireNoBody(body io.Reader) error {
	if body == nil {
		return nil
	}
	data, err := io.ReadAll(io.LimitReader(body, 1))
	if err != nil || len(data) != 0 {
		return errors.New("body not allowed")
	}
	return nil
}

func requireNoQuery(rawQuery string) error {
	query, err := url.ParseQuery(rawQuery)
	if err != nil || len(query) != 0 {
		return errors.New("query not allowed")
	}
	return nil
}

func requiredCanonicalUUIDHeader(request *http.Request, name string) (string, error) {
	values := request.Header.Values(name)
	if len(values) != 1 {
		return "", errors.New("header must occur once")
	}
	return canonicalUUID(values[0])
}

func requiredIfMatch(request *http.Request) (int64, error) {
	values := request.Header.Values("If-Match")
	if len(values) != 1 {
		return 0, errors.New("If-Match must occur once")
	}
	match := positiveETag.FindStringSubmatch(values[0])
	if match == nil {
		return 0, errors.New("invalid If-Match")
	}
	version, err := strconv.ParseInt(match[1], 10, 64)
	if err != nil || version <= 0 {
		return 0, errors.New("invalid If-Match")
	}
	return version, nil
}

func canonicalUUID(value string) (string, error) {
	parsed, err := uuid.Parse(value)
	if err != nil || parsed == uuid.Nil || parsed.String() != value || parsed.Variant() != uuid.RFC4122 || parsed.Version() < 1 || parsed.Version() > 5 {
		return "", errors.New("invalid canonical UUID")
	}
	return value, nil
}

func parseCanonicalInt(value string, min, max int64) (int64, error) {
	if !canonicalDecimal.MatchString(value) {
		return 0, errors.New("invalid integer")
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed < min || parsed > max {
		return 0, errors.New("integer out of range")
	}
	return parsed, nil
}

func normalizePublicValue(value string, max int, required bool) (string, error) {
	if !utf8.ValidString(value) {
		return "", errors.New("invalid UTF-8")
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return "", errors.New("control character")
		}
	}
	value = strings.TrimSpace(value)
	if required && value == "" {
		return "", errors.New("required")
	}
	if utf8.RuneCountInString(value) > max {
		return "", errors.New("too long")
	}
	return value, nil
}

func listResponse(result storecenter.ListStoresResult, request storecenter.ListStoresRequest) (ListStoresResponse, error) {
	if result.Total < int64(len(result.Items)) || len(result.Items) > result.PageSize || result.Page != request.Page || result.PageSize != request.PageSize || !validQuotaProjection(result.Quota) {
		return ListStoresResponse{}, storecenter.ErrDependencyUnavailable
	}
	items := make([]StoreResponse, 0, len(result.Items))
	for _, item := range result.Items {
		response, err := projectionResponse(item, request.OrganizationID, "")
		if err != nil {
			return ListStoresResponse{}, err
		}
		items = append(items, response)
	}
	var limit *int64
	if result.Quota.Limit != nil {
		value := *result.Quota.Limit
		limit = &value
	}
	return ListStoresResponse{Items: items, Quota: QuotaResponse{Used: result.Quota.Used, Reserved: result.Quota.Reserved, Limit: limit, Allowed: result.Quota.Allowed, Reason: result.Quota.Reason}, Pagination: PaginationResponse{Page: result.Page, PageSize: result.PageSize, Total: result.Total}}, nil
}

func validQuotaProjection(quota storecenter.StoreQuotaProjection) bool {
	if quota.Used < 0 || quota.Reserved < 0 {
		return false
	}
	if quota.Limit == nil {
		return !quota.Allowed && quota.Reason == "subscription_required"
	}
	if *quota.Limit <= 0 {
		return false
	}
	allowed := quota.Used < *quota.Limit && quota.Reserved < *quota.Limit-quota.Used
	if quota.Allowed != allowed {
		return false
	}
	if allowed {
		return quota.Reason == ""
	}
	return quota.Reason == "store_limit_reached"
}

func projectionResponse(projection storecenter.StoreProjection, organizationID, storeID string) (StoreResponse, error) {
	return storeResponse(&projection.Store, projection.ConnectionStatus, organizationID, storeID)
}

func storeResponse(store *storecenter.Store, connection storecenter.ConnectionStatus, organizationID, storeID string) (StoreResponse, error) {
	if store == nil || store.Version() <= 0 || store.CreatedAt().IsZero() || store.UpdatedAt().IsZero() {
		return StoreResponse{}, storecenter.ErrDependencyUnavailable
	}
	if store.OrganizationID() != organizationID || (storeID != "" && store.ID() != storeID) {
		return StoreResponse{}, storecenter.ErrDependencyUnavailable
	}
	if _, err := canonicalUUID(store.ID()); err != nil {
		return StoreResponse{}, storecenter.ErrDependencyUnavailable
	}
	switch connection {
	case storecenter.ConnectionStatusDisconnected, storecenter.ConnectionStatusConnected, storecenter.ConnectionStatusExpired, storecenter.ConnectionStatusUnavailable:
	default:
		return StoreResponse{}, storecenter.ErrDependencyUnavailable
	}
	return StoreResponse{ID: store.ID(), Name: store.Name(), Platform: string(store.Platform()), Region: store.Region(), ExternalStoreID: store.ExternalStoreID(), LifecycleStatus: store.LifecycleStatus(), ConnectionStatus: connection, Version: store.Version(), CreatedAt: store.CreatedAt().UTC(), UpdatedAt: store.UpdatedAt().UTC()}, nil
}

func isNilInterface(value any) bool {
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
