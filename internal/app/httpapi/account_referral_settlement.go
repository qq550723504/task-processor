package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"task-processor/internal/ledger/money"
)

// These endpoints are a controlled canonical-settlement ingress for the
// isolated account-center deployment. They do not contact a payment provider;
// an approved internal settlement observer supplies facts to the money owner.
type paymentSettlementBody struct {
	PaymentID                 string `json:"paymentId"`
	PayerUserID               string `json:"payerUserId"`
	Currency                  string `json:"currency"`
	GrossAmountMinor          string `json:"grossAmountMinor"`
	DiscountAmountMinor       string `json:"discountAmountMinor"`
	CommissionableAmountMinor string `json:"commissionableAmountMinor"`
	SettledAt                 string `json:"settledAt"`
	ProviderReference         string `json:"providerReference"`
	Version                   string `json:"version"`
}

type settlementReversalBody struct {
	ID                string `json:"id"`
	PaymentID         string `json:"paymentId"`
	AmountMinor       string `json:"amountMinor"`
	OccurredAt        string `json:"occurredAt"`
	ProviderReference string `json:"providerReference"`
}

func (m referralHTTPModule) settlementObserver(c *gin.Context) (money.SettlementObserver, bool) {
	if _, ok := m.trustedCommand(c); !ok {
		return nil, false
	}
	if m.settlements == nil || m.economics == nil {
		writeReferralEconomicsError(c, 503, "DEPENDENCY_UNAVAILABLE")
		return nil, false
	}
	observer, ok := m.economics.(money.SettlementObserver)
	if !ok {
		writeReferralEconomicsError(c, 503, "DEPENDENCY_UNAVAILABLE")
		return nil, false
	}
	return observer, true
}

func (m referralHTTPModule) recordPaymentSettlement(c *gin.Context) {
	observer, ok := m.settlementObserver(c)
	if !ok {
		return
	}
	var body paymentSettlementBody
	if !decodeEconomicsJSONInto(c, &body) {
		return
	}
	gross, err := parseNonnegativeEconomics(body.GrossAmountMinor)
	if err != nil || gross <= 0 {
		writeReferralEconomicsError(c, 400, "INVALID_REQUEST")
		return
	}
	discount, err := parseNonnegativeEconomics(body.DiscountAmountMinor)
	if err != nil {
		writeReferralEconomicsError(c, 400, "INVALID_REQUEST")
		return
	}
	commissionable, err := parseNonnegativeEconomics(body.CommissionableAmountMinor)
	if err != nil {
		writeReferralEconomicsError(c, 400, "INVALID_REQUEST")
		return
	}
	version, err := parseNonnegativeEconomics(body.Version)
	if err != nil || version < 1 {
		writeReferralEconomicsError(c, 400, "INVALID_REQUEST")
		return
	}
	settledAt, err := time.Parse(time.RFC3339Nano, body.SettledAt)
	if err != nil {
		writeReferralEconomicsError(c, 400, "INVALID_REQUEST")
		return
	}
	payment := money.PaymentSettlement{PaymentID: strings.TrimSpace(body.PaymentID), PayerUserID: strings.TrimSpace(body.PayerUserID), Currency: strings.TrimSpace(body.Currency), GrossAmountMinor: gross, DiscountAmountMinor: discount, CommissionableAmountMinor: commissionable, Status: money.PaymentSettled, SettledAt: settledAt, ProviderReference: strings.TrimSpace(body.ProviderReference), Version: version}
	if err := m.settlements.RecordPaymentSettlementAndNotify(c.Request.Context(), payment, observer); err != nil {
		writeMoneySettlementError(c, err)
		return
	}
	writeReferralEconomicsJSON(c, http.StatusAccepted, gin.H{"status": "accepted", "paymentId": payment.PaymentID})
}

func (m referralHTTPModule) recordRefundSettlement(c *gin.Context) {
	observer, ok := m.settlementObserver(c)
	if !ok {
		return
	}
	var body settlementReversalBody
	if !decodeEconomicsJSONInto(c, &body) {
		return
	}
	reversal, err := decodeReversal(body)
	if err != nil {
		writeReferralEconomicsError(c, 400, "INVALID_REQUEST")
		return
	}
	refund := money.RefundSettlement{RefundID: reversal.id, PaymentID: reversal.paymentID, AmountMinor: reversal.amount, OccurredAt: reversal.at, ProviderReference: reversal.reference}
	if err := m.settlements.RecordRefundSettlementAndNotify(c.Request.Context(), refund, observer); err != nil {
		writeMoneySettlementError(c, err)
		return
	}
	writeReferralEconomicsJSON(c, http.StatusAccepted, gin.H{"status": "accepted", "refundId": refund.RefundID})
}

func (m referralHTTPModule) recordChargebackSettlement(c *gin.Context) {
	observer, ok := m.settlementObserver(c)
	if !ok {
		return
	}
	var body settlementReversalBody
	if !decodeEconomicsJSONInto(c, &body) {
		return
	}
	reversal, err := decodeReversal(body)
	if err != nil {
		writeReferralEconomicsError(c, 400, "INVALID_REQUEST")
		return
	}
	chargeback := money.ChargebackSettlement{ChargebackID: reversal.id, PaymentID: reversal.paymentID, AmountMinor: reversal.amount, OccurredAt: reversal.at, ProviderReference: reversal.reference}
	if err := m.settlements.RecordChargebackSettlementAndNotify(c.Request.Context(), chargeback, observer); err != nil {
		writeMoneySettlementError(c, err)
		return
	}
	writeReferralEconomicsJSON(c, http.StatusAccepted, gin.H{"status": "accepted", "chargebackId": chargeback.ChargebackID})
}

func (m referralHTTPModule) matureReferralEarnings(c *gin.Context) {
	if _, ok := m.trustedCommand(c); !ok {
		return
	}
	if m.economics == nil {
		writeReferralEconomicsError(c, 503, "DEPENDENCY_UNAVAILABLE")
		return
	}
	if c.Request.ContentLength != 0 || c.Request.URL.RawQuery != "" || c.Request.URL.ForceQuery {
		writeReferralEconomicsError(c, 400, "INVALID_REQUEST")
		return
	}
	if err := m.economics.Mature(c.Request.Context(), time.Now().UTC()); err != nil {
		writeReferralEconomicsError(c, 503, "DEPENDENCY_UNAVAILABLE")
		return
	}
	writeReferralEconomicsJSON(c, http.StatusAccepted, gin.H{"status": "accepted"})
}

type decodedReversal struct {
	id, paymentID, reference string
	amount                   int64
	at                       time.Time
}

func decodeReversal(body settlementReversalBody) (decodedReversal, error) {
	amount, err := parseNonnegativeEconomics(body.AmountMinor)
	if err != nil || amount <= 0 {
		return decodedReversal{}, errors.New("invalid amount")
	}
	at, err := time.Parse(time.RFC3339Nano, body.OccurredAt)
	if err != nil {
		return decodedReversal{}, err
	}
	value := decodedReversal{id: strings.TrimSpace(body.ID), paymentID: strings.TrimSpace(body.PaymentID), reference: strings.TrimSpace(body.ProviderReference), amount: amount, at: at}
	if value.id == "" || value.paymentID == "" || value.reference == "" {
		return decodedReversal{}, errors.New("missing settlement identity")
	}
	return value, nil
}

func decodeEconomicsJSONInto(c *gin.Context, out any) bool {
	value, err := decodeEconomicsBody(c.Request, out)
	if err != nil || !value {
		writeReferralEconomicsError(c, 400, "INVALID_REQUEST")
		return false
	}
	return true
}

func decodeEconomicsBody(r *http.Request, out any) (bool, error) {
	if r.ContentLength <= 0 || r.ContentLength > 8*1024 || len(r.TransferEncoding) > 0 {
		return false, errors.New("invalid settlement body")
	}
	decoder := json.NewDecoder(io.LimitReader(r.Body, 8*1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		return false, err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return false, errors.New("trailing settlement body")
	}
	return true, nil
}

func writeMoneySettlementError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, money.ErrConflict):
		writeReferralEconomicsError(c, 409, "CONFLICT")
	case errors.Is(err, money.ErrInvalid):
		writeReferralEconomicsError(c, 400, "INVALID_REQUEST")
	case errors.Is(err, money.ErrNotFound):
		writeReferralEconomicsError(c, 404, "NOT_FOUND")
	default:
		writeReferralEconomicsError(c, 503, "DEPENDENCY_UNAVAILABLE")
	}
}
