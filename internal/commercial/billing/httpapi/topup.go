package httpapi

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	"task-processor/internal/commercial/billing"
	"task-processor/internal/httproute"

	"github.com/gin-gonic/gin"
)

const (
	topUpOptionsPath  = walletPath + "/top-up-options"
	topUpCheckoutPath = orderDetailPath + "/checkout"
	topUpCancelPath   = orderDetailPath + "/cancel-payment"
	TopUpRefundPath   = "/api/v1/admin/commercial/top-up-orders/:order_id/refunds"
	AlipayNotifyPath  = "/api/v1/payments/alipay/notify"
	WeChatNotifyPath  = "/api/v1/payments/wechat/notify"
)

type NotificationVerifier interface {
	VerifyNotification(*http.Request) (billing.ProviderObservation, error)
}

func (h *Handler) SetPaymentNotifications(alipay, wechat NotificationVerifier) {
	h.alipayNotify = alipay
	h.wechatNotify = wechat
}

func (h *Handler) TopUpOptions(c *gin.Context) {
	if h == nil || h.service == nil || c.Request.URL.RawQuery != "" {
		writeError(c, 400, "INVALID_REQUEST")
		return
	}
	org, ok := effectiveOrganization(c)
	if !ok {
		writeError(c, 409, "ORGANIZATION_SELECTION_REQUIRED")
		return
	}
	v := h.service.WalletTopUpOptions()
	amounts := make([]string, 0, len(v.Policy.QuickAmounts))
	for _, amount := range v.Policy.QuickAmounts {
		amounts = append(amounts, strconv.FormatInt(amount, 10))
	}
	channels := make([]map[string]any, 0, len(v.Channels))
	for _, channel := range v.Channels {
		channels = append(channels, map[string]any{"provider": channel.Provider, "product": channel.Product, "available": channel.Available, "reason": channel.Reason})
	}
	writeJSON(c, 200, map[string]any{"organization_id": org, "currency": "CNY", "min_minor": strconv.FormatInt(v.Policy.MinMinor, 10), "max_minor": strconv.FormatInt(v.Policy.MaxMinor, 10), "quick_amounts_minor": amounts, "channels": channels})
}
func (h *Handler) TopUpIntent(c *gin.Context) {
	org, actor, ok := writeOrganizationAndActor(c)
	if !ok || h == nil || h.service == nil {
		return
	}
	key, ok := singleIdempotencyKey(c)
	if !ok {
		writeError(c, 400, "INVALID_REQUEST")
		return
	}
	var request struct {
		Provider billing.PaymentProvider `json:"provider"`
		Amount   string                  `json:"amount_minor"`
	}
	if !decodeStrict(c, &request) {
		return
	}
	amount, err := positiveMinor(request.Amount)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	order, err := h.service.CreateWalletTopUpOrder(c.Request.Context(), billing.CreateWalletTopUpOrderRequest{OrganizationID: org, ActorID: actor, IdempotencyKey: key, Provider: request.Provider, Currency: "CNY", AmountMinor: amount})
	if err != nil {
		writeServiceError(c, err)
		return
	}
	h.writeOrderWithTopUp(c, 201, order)
}
func positiveMinor(s string) (int64, error) {
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n <= 0 || strconv.FormatInt(n, 10) != s {
		return 0, billing.ErrInvalid
	}
	return n, nil
}
func (h *Handler) TopUpCheckout(c *gin.Context) {
	org, actor, ok := writeOrganizationAndActor(c)
	if !ok || h == nil || h.service == nil {
		return
	}
	var request struct {
		Version string `json:"expected_version"`
	}
	if !decodeStrict(c, &request) {
		return
	}
	version, err := positiveMinor(request.Version)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	v, err := h.service.CheckoutTopUp(c.Request.Context(), org, actor, c.Param("order_id"), version)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Header("Referrer-Policy", "no-referrer")
	writeJSON(c, 200, map[string]any{"organization_id": org, "kind": v.Kind, "payload": v.Payload, "order_id": v.OrderID, "attempt_id": v.AttemptID, "provider": v.Provider, "expires_at": v.ExpiresAt.Format(time.RFC3339Nano)})
}
func (h *Handler) TopUpCancel(c *gin.Context) {
	org, actor, ok := writeOrganizationAndActor(c)
	if !ok || h == nil || h.service == nil {
		return
	}
	var request struct {
		Version string `json:"expected_version"`
	}
	if !decodeStrict(c, &request) {
		return
	}
	version, err := positiveMinor(request.Version)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	err = h.service.CancelTopUp(c.Request.Context(), org, actor, c.Param("order_id"), version)
	if err != nil && !errors.Is(err, billing.ErrReconciliationRequired) {
		writeServiceError(c, err)
		return
	}
	order, err := h.service.ReadOrder(c.Request.Context(), org, c.Param("order_id"))
	if err != nil {
		writeServiceError(c, err)
		return
	}
	h.writeOrderWithTopUp(c, 200, order)
}

type topUpSummaryResponse struct {
	Provider             billing.PaymentProvider `json:"provider"`
	Phase                billing.TopUpPhase      `json:"phase"`
	AttemptID            string                  `json:"attempt_id"`
	Version              string                  `json:"version"`
	ExpiresAt            string                  `json:"expires_at"`
	CloseRequested       bool                    `json:"close_requested"`
	LatePaymentCorrected bool                    `json:"late_payment_corrected"`
}

func (h *Handler) writeOrderWithTopUp(c *gin.Context, status int, order billing.Order) {
	response := orderResponseFromDomain(order)
	if order.Kind == billing.OrderWalletTopUp {
		a, err := h.service.ReadTopUpAttempt(c.Request.Context(), order.OrganizationID, order.OrderID)
		if err != nil {
			writeServiceError(c, err)
			return
		}
		response.TopUp = &topUpSummaryResponse{Provider: a.Merchant.Provider, Phase: a.Phase, AttemptID: a.AttemptID, Version: strconv.FormatInt(a.Version, 10), ExpiresAt: a.ExpiresAt.Format(time.RFC3339Nano), CloseRequested: a.CloseRequestedAt != nil, LatePaymentCorrected: a.LatePaymentCorrectedAt != nil}
	}
	c.Header("Cache-Control", "no-store")
	writeJSON(c, status, response)
}
func (h *Handler) TopUpRefund(c *gin.Context) {
	identity, ok := authidentity.AuthenticatedIdentityFromContext(c.Request.Context())
	if !ok || identity.EffectiveOrganizationID != "" || identity.TenantID != "" || h == nil || h.service == nil {
		writeError(c, 403, "FORBIDDEN")
		return
	}
	key, ok := singleIdempotencyKey(c)
	if !ok {
		writeError(c, 400, "INVALID_REQUEST")
		return
	}
	var request struct {
		Version string `json:"expected_version"`
		Amount  string `json:"amount_minor"`
		Reason  string `json:"reason"`
	}
	if !decodeStrict(c, &request) {
		return
	}
	amount, e1 := positiveMinor(request.Amount)
	version, e2 := positiveMinor(request.Version)
	if e1 != nil || e2 != nil {
		writeError(c, 400, "INVALID_REQUEST")
		return
	}
	r, err := h.service.ApproveTopUpRefund(c.Request.Context(), identity.UserID, c.Param("order_id"), key, request.Reason, amount, version)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	writeJSON(c, 200, map[string]any{"refund_id": r.RefundID, "order_id": r.OrderID, "amount_minor": strconv.FormatInt(r.AmountMinor, 10), "state": r.State})
}
func (h *Handler) paymentNotify(c *gin.Context, p NotificationVerifier, alipay bool) {
	if p == nil || h == nil || h.service == nil {
		c.Status(503)
		return
	}
	observation, err := p.VerifyNotification(c.Request)
	if err != nil {
		c.Status(400)
		return
	}
	if err = h.service.RecordVerifiedPaymentObservation(c.Request.Context(), observation); err != nil {
		c.Status(503)
		return
	}
	if alipay {
		c.Data(200, "text/plain; charset=utf-8", []byte("success"))
	} else {
		c.Status(http.StatusNoContent)
	}
}
func (h *Handler) topUpExternalRoutes() []httproute.Descriptor {
	return []httproute.Descriptor{
		{Method: http.MethodPost, Path: AlipayNotifyPath, Module: "commercial-billing", AuthPolicy: httproute.AuthPolicyPublic, OrganizationAccessPolicy: httproute.OrganizationAccessPolicyNone, RequestTimeout: 5 * time.Second, Handler: httproute.WithRequestBodyReadTimeout(3*time.Second, func(c *gin.Context) { h.paymentNotify(c, h.alipayNotify, true) })},
		{Method: http.MethodPost, Path: WeChatNotifyPath, Module: "commercial-billing", AuthPolicy: httproute.AuthPolicyPublic, OrganizationAccessPolicy: httproute.OrganizationAccessPolicyNone, RequestTimeout: 5 * time.Second, Handler: httproute.WithRequestBodyReadTimeout(3*time.Second, func(c *gin.Context) { h.paymentNotify(c, h.wechatNotify, false) })},
		{Method: http.MethodPost, Path: TopUpRefundPath, Module: "commercial-billing", Permission: authz.PermissionListingKitPlatformAdm, AuthPolicy: httproute.AuthPolicyCurrentIdentityWithVerifiedRoles, OrganizationAccessPolicy: httproute.OrganizationAccessPolicyNone, RequestTimeout: 30 * time.Second, Handler: httproute.WithRequestBodyReadTimeout(10*time.Second, h.TopUpRefund)},
	}
}
