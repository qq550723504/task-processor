package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	"task-processor/internal/commercial/billing"
	"task-processor/internal/httproute"
)

const (
	walletPath        = "/api/v1/workbench/commercial/wallet"
	walletEntriesPath = walletPath + "/entries"
	quotePath         = "/api/v1/workbench/commercial/quotes"
	orderPath         = "/api/v1/workbench/commercial/orders"
	orderSummaryPath  = orderPath + "/summary"
	orderDetailPath   = orderPath + "/:order_id"
	topUpIntentPath   = walletPath + "/top-up-intents"
	maxBodyBytes      = 16 * 1024
)

type Handler struct{ service *billing.Service }

func NewHandler(service *billing.Service) *Handler { return &Handler{service: service} }

func (h *Handler) Wallet(c *gin.Context) {
	if !validReadRequest(c) || c.Request.URL.RawQuery != "" || h == nil || h.service == nil {
		writeError(c, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}
	organizationID, ok := effectiveOrganization(c)
	if !ok {
		writeError(c, http.StatusConflict, "ORGANIZATION_SELECTION_REQUIRED")
		return
	}
	value, err := h.service.ReadWallet(c.Request.Context(), organizationID)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, walletResponse{OrganizationID: value.OrganizationID, Currency: value.Currency, AvailableMinor: strconv.FormatInt(value.AvailableMinor, 10), ReservedMinor: strconv.FormatInt(value.ReservedMinor, 10), DebtMinor: strconv.FormatInt(value.DebtMinor, 10), LifetimeTopUpMinor: strconv.FormatInt(value.LifetimeTopUpMinor, 10), LifetimeSpendMinor: strconv.FormatInt(value.LifetimeSpendMinor, 10), Version: strconv.FormatInt(value.Version, 10), ObservedAt: value.UpdatedAt.UTC().Format(time.RFC3339Nano)})
}

func (h *Handler) WalletEntries(c *gin.Context) {
	if !validReadRequest(c) || h == nil || h.service == nil {
		writeError(c, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}
	organizationID, ok := effectiveOrganization(c)
	if !ok {
		writeError(c, http.StatusConflict, "ORGANIZATION_SELECTION_REQUIRED")
		return
	}
	limit := 50
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 100 {
			writeError(c, http.StatusBadRequest, "INVALID_REQUEST")
			return
		}
		limit = parsed
	}
	page, err := h.service.ListWalletEntries(c.Request.Context(), organizationID, c.Query("cursor"), limit)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	items := make([]walletEntryResponse, 0, len(page.Items))
	for _, entry := range page.Items {
		items = append(items, walletEntryResponse{EntryID: entry.EntryID, Currency: entry.Currency, Kind: string(entry.Kind), AvailableDelta: strconv.FormatInt(entry.AvailableDelta, 10), ReservedDelta: strconv.FormatInt(entry.ReservedDelta, 10), DebtDelta: strconv.FormatInt(entry.DebtDelta, 10), AvailableAfter: strconv.FormatInt(entry.AvailableAfter, 10), ReservedAfter: strconv.FormatInt(entry.ReservedAfter, 10), DebtAfter: strconv.FormatInt(entry.DebtAfter, 10), CommercialOrderID: nullable(entry.CommercialOrderID), PaymentID: nullable(entry.PaymentID), SourceIdentity: entry.SourceIdentity, OccurredAt: entry.OccurredAt.UTC().Format(time.RFC3339Nano)})
	}
	writeJSON(c, http.StatusOK, map[string]any{"organization_id": organizationID, "items": items, "next_cursor": page.NextCursor})
}

func (h *Handler) CreateQuote(c *gin.Context) {
	organizationID, ok := writeOrganization(c)
	if !ok || h == nil || h.service == nil {
		return
	}
	var request struct {
		OfferID  string `json:"offer_id"`
		Quantity string `json:"quantity"`
	}
	if !decodeStrict(c, &request) {
		return
	}
	quantity, err := strconv.ParseInt(request.Quantity, 10, 64)
	if err != nil || quantity <= 0 {
		writeError(c, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}
	quote, err := h.service.CreateQuote(c.Request.Context(), billing.QuoteRequest{OrganizationID: organizationID, OfferID: strings.TrimSpace(request.OfferID), Quantity: quantity})
	if err != nil {
		writeServiceError(c, err)
		return
	}
	writeJSON(c, http.StatusCreated, quoteResponse{QuoteID: quote.QuoteID, OrganizationID: quote.OrganizationID, OfferID: quote.OfferID, ProductKind: string(quote.ProductKind), ResourceType: string(quote.ResourceType), ResourceQuantity: strconv.FormatInt(quote.ResourceQuantity, 10), Currency: quote.Currency, TotalMinor: strconv.FormatInt(quote.TotalMinor, 10), PricingVersion: quote.PricingVersion, ExpiresAt: quote.ExpiresAt.UTC().Format(time.RFC3339Nano), Fingerprint: quote.Fingerprint, CreatedAt: quote.CreatedAt.UTC().Format(time.RFC3339Nano)})
}

func (h *Handler) CreateOrder(c *gin.Context) {
	organizationID, ok := writeOrganization(c)
	if !ok || h == nil || h.service == nil {
		return
	}
	idempotencyKey := strings.TrimSpace(c.GetHeader("Idempotency-Key"))
	if idempotencyKey == "" || len(idempotencyKey) > 192 {
		writeError(c, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}
	var request struct {
		QuoteID string `json:"quote_id"`
	}
	if !decodeStrict(c, &request) {
		return
	}
	order, err := h.service.CreateResourceOrder(c.Request.Context(), billing.CreateResourceOrderRequest{OrganizationID: organizationID, QuoteID: strings.TrimSpace(request.QuoteID), IdempotencyKey: idempotencyKey})
	if err != nil && order.OrderID == "" {
		writeServiceError(c, err)
		return
	}
	status := http.StatusCreated
	if err != nil {
		status = http.StatusConflict
	}
	writeJSON(c, status, orderResponseFromDomain(order))
}

func (h *Handler) Orders(c *gin.Context) {
	if !validReadRequest(c) || h == nil || h.service == nil {
		writeError(c, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}
	organizationID, ok := effectiveOrganization(c)
	if !ok {
		writeError(c, http.StatusConflict, "ORGANIZATION_SELECTION_REQUIRED")
		return
	}
	query := c.Request.URL.Query()
	for key := range query {
		switch key {
		case "q", "kind", "product_kind", "status", "from", "until", "cursor", "limit":
		default:
			writeError(c, http.StatusBadRequest, "INVALID_REQUEST")
			return
		}
	}
	filter := billing.OrderFilter{Limit: 50, Query: strings.TrimSpace(c.Query("q")), Cursor: c.Query("cursor")}
	if raw := c.Query("limit"); raw != "" {
		limit, err := strconv.Atoi(raw)
		if err != nil || limit < 1 || limit > 100 {
			writeError(c, http.StatusBadRequest, "INVALID_REQUEST")
			return
		}
		filter.Limit = limit
	}
	if raw := c.Query("kind"); raw != "" {
		kind := billing.OrderKind(raw)
		if kind != billing.OrderWalletTopUp && kind != billing.OrderResourcePurchase {
			writeError(c, http.StatusBadRequest, "INVALID_REQUEST")
			return
		}
		filter.Kind = &kind
	}
	if raw := c.Query("product_kind"); raw != "" {
		product := billing.ProductKind(raw)
		if _, ok := billing.ResourceTypeForProduct(product); !ok {
			writeError(c, http.StatusBadRequest, "INVALID_REQUEST")
			return
		}
		filter.ProductKind = &product
	}
	if raw := c.Query("status"); raw != "" {
		status := billing.OrderStatus(raw)
		if !validOrderStatus(status) {
			writeError(c, http.StatusBadRequest, "INVALID_REQUEST")
			return
		}
		filter.Status = &status
	}
	if raw := c.Query("from"); raw != "" {
		value, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			writeError(c, http.StatusBadRequest, "INVALID_REQUEST")
			return
		}
		filter.From = &value
	}
	if raw := c.Query("until"); raw != "" {
		value, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			writeError(c, http.StatusBadRequest, "INVALID_REQUEST")
			return
		}
		filter.Until = &value
	}
	page, err := h.service.ListOrders(c.Request.Context(), organizationID, filter)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	items := make([]orderResponse, 0, len(page.Items))
	for _, order := range page.Items {
		items = append(items, orderResponseFromDomain(order))
	}
	writeJSON(c, http.StatusOK, map[string]any{"organization_id": organizationID, "items": items, "next_cursor": page.NextCursor})
}

func (h *Handler) OrderSummary(c *gin.Context) {
	if !validReadRequest(c) || h == nil || h.service == nil {
		writeError(c, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}
	organizationID, ok := effectiveOrganization(c)
	if !ok {
		writeError(c, http.StatusConflict, "ORGANIZATION_SELECTION_REQUIRED")
		return
	}
	until := time.Now().UTC()
	from := until.Add(-30 * 24 * time.Hour)
	if raw := c.Query("from"); raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			writeError(c, http.StatusBadRequest, "INVALID_REQUEST")
			return
		}
		from = parsed.UTC()
	}
	if raw := c.Query("until"); raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			writeError(c, http.StatusBadRequest, "INVALID_REQUEST")
			return
		}
		until = parsed.UTC()
	}
	if !from.Before(until) || until.Sub(from) > 366*24*time.Hour {
		writeError(c, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}
	summary, err := h.service.ReadOrderSummary(c.Request.Context(), organizationID, from, until)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, map[string]any{"organization_id": organizationID, "currency": summary.Currency, "from": summary.From.Format(time.RFC3339Nano), "until": summary.Until.Format(time.RFC3339Nano), "spend_minor": strconv.FormatInt(summary.SpendMinor, 10), "store_renewal_spend_minor": strconv.FormatInt(summary.StoreRenewalSpendMinor, 10), "ai_point_spend_minor": strconv.FormatInt(summary.AIPointSpendMinor, 10), "data_row_spend_minor": strconv.FormatInt(summary.DataRowSpendMinor, 10), "other_spend_minor": strconv.FormatInt(summary.OtherSpendMinor, 10), "observed_at": summary.ObservedAt.Format(time.RFC3339Nano)})
}

func (h *Handler) Order(c *gin.Context) {
	if !validReadRequest(c) || h == nil || h.service == nil {
		writeError(c, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}
	organizationID, ok := effectiveOrganization(c)
	if !ok {
		writeError(c, http.StatusConflict, "ORGANIZATION_SELECTION_REQUIRED")
		return
	}
	order, err := h.service.ReadOrder(c.Request.Context(), organizationID, c.Param("order_id"))
	if err != nil {
		writeServiceError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, orderResponseFromDomain(order))
}

func (h *Handler) TopUpIntent(c *gin.Context) {
	if _, ok := writeOrganization(c); !ok {
		return
	}
	writeError(c, http.StatusServiceUnavailable, "FEATURE_UNAVAILABLE")
}

func Routes(handler *Handler) ([]httproute.Descriptor, error) {
	if handler == nil {
		return nil, errors.New("commercial billing handler is nil")
	}
	const read = authz.PermissionWorkbenchCommercialRead
	const purchase = authz.PermissionWorkbenchCommercialPurchase
	const topup = authz.PermissionWorkbenchCommercialWalletTopUp
	routes := make([]httproute.Descriptor, 0, 7)
	for _, route := range []struct {
		method, path, permission string
		handler                  gin.HandlerFunc
	}{
		{http.MethodGet, walletPath, read, handler.Wallet},
		{http.MethodGet, walletEntriesPath, read, handler.WalletEntries},
		{http.MethodPost, quotePath, purchase, handler.CreateQuote},
		{http.MethodPost, orderPath, purchase, handler.CreateOrder},
		{http.MethodGet, orderPath, read, handler.Orders},
		{http.MethodGet, orderSummaryPath, read, handler.OrderSummary},
		{http.MethodGet, orderDetailPath, read, handler.Order},
		{http.MethodPost, topUpIntentPath, topup, handler.TopUpIntent},
	} {
		routes = append(routes, httproute.Descriptor{Method: route.method, Path: route.path, Module: "commercial-billing", Permission: route.permission, AuthPolicy: httproute.AuthPolicyCurrentIdentity, OrganizationAccessPolicy: httproute.OrganizationAccessPolicyLiveWrite, RequestTimeout: 15 * time.Second, RejectUnreadRequestBody: true, Handler: route.handler})
	}
	return routes, nil
}

type walletResponse struct {
	OrganizationID     string `json:"organization_id"`
	Currency           string `json:"currency"`
	AvailableMinor     string `json:"available_minor"`
	ReservedMinor      string `json:"reserved_minor"`
	DebtMinor          string `json:"debt_minor"`
	LifetimeTopUpMinor string `json:"lifetime_topup_minor"`
	LifetimeSpendMinor string `json:"lifetime_spend_minor"`
	Version            string `json:"version"`
	ObservedAt         string `json:"observed_at"`
}
type walletEntryResponse struct {
	EntryID           string  `json:"entry_id"`
	Currency          string  `json:"currency"`
	Kind              string  `json:"entry_type"`
	AvailableDelta    string  `json:"available_delta_minor"`
	ReservedDelta     string  `json:"reserved_delta_minor"`
	DebtDelta         string  `json:"debt_delta_minor"`
	AvailableAfter    string  `json:"available_after_minor"`
	ReservedAfter     string  `json:"reserved_after_minor"`
	DebtAfter         string  `json:"debt_after_minor"`
	CommercialOrderID *string `json:"order_id,omitempty"`
	PaymentID         *string `json:"payment_id,omitempty"`
	SourceIdentity    string  `json:"source_id"`
	OccurredAt        string  `json:"occurred_at"`
}
type quoteResponse struct {
	QuoteID          string `json:"quote_id"`
	OrganizationID   string `json:"organization_id"`
	OfferID          string `json:"offer_id"`
	ProductKind      string `json:"product_kind"`
	ResourceType     string `json:"resource_type"`
	ResourceQuantity string `json:"resource_quantity"`
	Currency         string `json:"currency"`
	TotalMinor       string `json:"total_minor"`
	PricingVersion   string `json:"pricing_version"`
	ExpiresAt        string `json:"expires_at"`
	Fingerprint      string `json:"fingerprint"`
	CreatedAt        string `json:"created_at"`
}
type orderResponse struct {
	OrderID             string              `json:"order_id"`
	OrganizationID      string              `json:"organization_id"`
	Kind                string              `json:"kind"`
	QuoteID             *string             `json:"quote_id,omitempty"`
	Currency            string              `json:"currency"`
	AmountMinor         string              `json:"total_minor"`
	Status              string              `json:"status"`
	WalletReservationID *string             `json:"wallet_reservation_id,omitempty"`
	Items               []orderItemResponse `json:"items"`
	CreatedAt           string              `json:"created_at"`
	UpdatedAt           string              `json:"updated_at"`
}
type orderItemResponse struct {
	OrderItemID      string `json:"order_item_id"`
	ProductKind      string `json:"product_kind"`
	ResourceType     string `json:"resource_type"`
	ResourceQuantity string `json:"resource_quantity"`
	AmountMinor      string `json:"amount_minor"`
}

func orderResponseFromDomain(order billing.Order) orderResponse {
	items := make([]orderItemResponse, 0, len(order.Items))
	for _, item := range order.Items {
		items = append(items, orderItemResponse{OrderItemID: item.OrderItemID, ProductKind: string(item.ProductKind), ResourceType: string(item.ResourceType), ResourceQuantity: strconv.FormatInt(item.ResourceQuantity, 10), AmountMinor: strconv.FormatInt(item.AmountMinor, 10)})
	}
	return orderResponse{OrderID: order.OrderID, OrganizationID: order.OrganizationID, Kind: string(order.Kind), QuoteID: nullable(order.QuoteID), Currency: order.Currency, AmountMinor: strconv.FormatInt(order.AmountMinor, 10), Status: string(order.Status), WalletReservationID: nullable(order.WalletReservationID), Items: items, CreatedAt: order.CreatedAt.UTC().Format(time.RFC3339Nano), UpdatedAt: order.UpdatedAt.UTC().Format(time.RFC3339Nano)}
}

func effectiveOrganization(c *gin.Context) (string, bool) {
	identity, ok := authidentity.AuthenticatedIdentityFromContext(c.Request.Context())
	if !ok || strings.TrimSpace(identity.EffectiveOrganizationID) == "" || identity.TenantID != identity.EffectiveOrganizationID {
		return "", false
	}
	return strings.TrimSpace(identity.EffectiveOrganizationID), true
}
func writeOrganization(c *gin.Context) (string, bool) {
	if !validWriteRequest(c) {
		return "", false
	}
	organizationID, ok := effectiveOrganization(c)
	if !ok {
		writeError(c, http.StatusConflict, "ORGANIZATION_SELECTION_REQUIRED")
		return "", false
	}
	return organizationID, true
}
func validReadRequest(c *gin.Context) bool {
	return c.Request.Method == http.MethodGet && c.Request.URL.RawQuery == "" || c.Request.Method == http.MethodGet
}
func validWriteRequest(c *gin.Context) bool {
	return c.Request.Method == http.MethodPost && c.Request.ContentLength >= 0
}
func decodeStrict(c *gin.Context, target any) bool {
	if c.Request.ContentLength > maxBodyBytes {
		writeError(c, http.StatusBadRequest, "INVALID_REQUEST")
		return false
	}
	decoder := json.NewDecoder(io.LimitReader(c.Request.Body, maxBodyBytes+1))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeError(c, http.StatusBadRequest, "INVALID_REQUEST")
		return false
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		writeError(c, http.StatusBadRequest, "INVALID_REQUEST")
		return false
	}
	return true
}
func writeServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, billing.ErrInvalid):
		writeError(c, http.StatusBadRequest, "INVALID_REQUEST")
	case errors.Is(err, billing.ErrQuoteExpired):
		writeError(c, http.StatusConflict, "QUOTE_EXPIRED")
	case errors.Is(err, billing.ErrInsufficientFunds):
		writeError(c, http.StatusConflict, "INSUFFICIENT_FUNDS")
	case errors.Is(err, billing.ErrConflict):
		writeError(c, http.StatusConflict, "IDEMPOTENCY_CONFLICT")
	case errors.Is(err, billing.ErrFeatureUnavailable), errors.Is(err, billing.ErrOfferUnavailable):
		writeError(c, http.StatusServiceUnavailable, "FEATURE_UNAVAILABLE")
	case errors.Is(err, billing.ErrReconciliationRequired):
		writeError(c, http.StatusConflict, "RECONCILIATION_REQUIRED")
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		writeError(c, http.StatusGatewayTimeout, "DEADLINE_EXCEEDED")
	default:
		writeError(c, http.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE")
	}
}
func writeJSON(c *gin.Context, status int, value any) {
	body, err := json.Marshal(value)
	if err != nil || len(body) > 64*1024 {
		writeError(c, http.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE")
		return
	}
	c.Data(status, "application/json; charset=utf-8", body)
}
func writeError(c *gin.Context, status int, code string) {
	c.AbortWithStatusJSON(status, gin.H{"code": code, "message": "Commercial billing request could not be completed", "requestId": uuid.NewString(), "fieldErrors": []any{}})
}
func nullable(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return &value
}

func validOrderStatus(status billing.OrderStatus) bool {
	switch status {
	case billing.OrderPending, billing.OrderFundsReserved, billing.OrderFulfilling, billing.OrderFulfilled, billing.OrderCancelled, billing.OrderReconciliationRequired:
		return true
	default:
		return false
	}
}
