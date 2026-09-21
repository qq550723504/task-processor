package httpapi

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"task-processor/internal/authidentity"
	"task-processor/internal/ledger/money"
	economics "task-processor/internal/referraleconomics"
)

type withdrawalBody struct {
	AmountMinor     string `json:"amountMinor"`
	PayoutMethodID  string `json:"payoutMethodId"`
	ExpectedVersion string `json:"expectedVersion"`
}
type cancelBody struct {
	ExpectedVersion string `json:"expectedVersion"`
}
type payoutMethodBody struct {
	Type        string `json:"type"`
	DisplayName string `json:"displayName"`
	Destination string `json:"destination"`
}
type reviewBody struct {
	Action                   string `json:"action"`
	ExpectedVersion          string `json:"expectedVersion"`
	ExternalPaymentReference string `json:"externalPaymentReference"`
}

func (m referralHTTPModule) readEarnings(c *gin.Context) {
	if !m.referralRequest(c) || m.economics == nil {
		writeReferralEconomicsError(c, http.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE")
		return
	}
	identity, ok := authidentity.AuthenticatedIdentityFromContext(c.Request.Context())
	if !ok {
		return
	}
	earnings, err := m.economics.ReadEarnings(c.Request.Context(), identity.UserID, economics.CurrencyCNY)
	if err != nil {
		writeReferralEconomicsError(c, http.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE")
		return
	}
	writeReferralEconomicsJSON(c, http.StatusOK, gin.H{"schemaVersion": "referral-earnings-v1", "referrer": earnings.Referrer, "currency": earnings.Currency, "pendingMinor": strconv.FormatInt(earnings.PendingMinor, 10), "availableMinor": strconv.FormatInt(earnings.AvailableMinor, 10), "reservedMinor": strconv.FormatInt(earnings.ReservedMinor, 10), "adjustmentMinor": strconv.FormatInt(earnings.AdjustmentMinor, 10), "version": strconv.FormatInt(earnings.Version, 10), "updatedAt": nullableTime(earnings.UpdatedAt), "source": "referral_earnings_projection"})
}

func (m referralHTTPModule) readPayoutMethods(c *gin.Context) {
	if !m.referralRequest(c) || m.payoutMethods == nil {
		writeReferralEconomicsError(c, http.StatusServiceUnavailable, "PAYOUT_METHOD_UNAVAILABLE")
		return
	}
	identity, ok := authidentity.AuthenticatedIdentityFromContext(c.Request.Context())
	if !ok {
		return
	}
	methods, err := m.payoutMethods.ListActivePayoutMethods(c.Request.Context(), identity.UserID)
	if err != nil {
		writeReferralEconomicsError(c, http.StatusServiceUnavailable, "PAYOUT_METHOD_UNAVAILABLE")
		return
	}
	items := make([]gin.H, 0, len(methods))
	for _, method := range methods {
		if method.Status != money.PayoutMethodActive {
			continue
		}
		items = append(items, gin.H{"methodId": method.MethodID, "type": method.Type, "displayName": method.DisplayName, "maskedDestination": method.MaskedDestination, "version": strconv.FormatInt(method.Version, 10)})
	}
	writeReferralEconomicsJSON(c, http.StatusOK, gin.H{"schemaVersion": "payout-methods-v1", "methods": items})
}

func (m referralHTTPModule) readWithdrawals(c *gin.Context) {
	if !m.referralRequest(c) || m.withdrawals == nil {
		writeReferralEconomicsError(c, http.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE")
		return
	}
	identity, ok := authidentity.AuthenticatedIdentityFromContext(c.Request.Context())
	if !ok {
		return
	}
	items, err := m.withdrawals.ListWithdrawals(c.Request.Context(), identity.UserID)
	if err != nil {
		writeReferralEconomicsError(c, http.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE")
		return
	}
	writeWithdrawalsJSON(c, items)
}

func (m referralHTTPModule) readWithdrawalQueue(c *gin.Context) {
	if !m.referralRequest(c) || m.withdrawals == nil {
		writeReferralEconomicsError(c, http.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE")
		return
	}
	if _, ok := authidentity.AuthenticatedIdentityFromContext(c.Request.Context()); !ok {
		return
	}
	items, err := m.withdrawals.ListPendingWithdrawals(c.Request.Context())
	if err != nil {
		writeReferralEconomicsError(c, http.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE")
		return
	}
	writeWithdrawalsJSON(c, items)
}

// createPayoutMethod is the personal money-owner write path. The destination
// is encrypted before it reaches persistence; ordinary reads only expose the
// masked projection. It intentionally has no organization context.
func (m referralHTTPModule) createPayoutMethod(c *gin.Context) {
	if !m.economicsRequest(c) || m.payoutMethodWriter == nil || len(m.payoutEncryptionKey) == 0 {
		writeReferralEconomicsError(c, http.StatusServiceUnavailable, "PAYOUT_METHOD_UNAVAILABLE")
		return
	}
	identity, ok := authidentity.AuthenticatedIdentityFromContext(c.Request.Context())
	if !ok {
		return
	}
	var body payoutMethodBody
	decoder := json.NewDecoder(io.LimitReader(c.Request.Body, 8<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil || strings.TrimSpace(body.DisplayName) == "" || strings.TrimSpace(body.Destination) == "" {
		writeReferralEconomicsError(c, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}
	typeValue := money.PayoutMethodType(strings.TrimSpace(body.Type))
	if typeValue != money.PayoutAlipay && typeValue != money.PayoutBankTransfer || len([]rune(body.DisplayName)) > 128 || len([]rune(body.Destination)) > 512 {
		writeReferralEconomicsError(c, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}
	destination := strings.TrimSpace(body.Destination)
	key := strings.TrimSpace(c.GetHeader("Idempotency-Key"))
	if key == "" || len(key) > 128 {
		writeReferralEconomicsError(c, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}
	fingerprint := payoutMethodFingerprint(identity.UserID, typeValue, strings.TrimSpace(body.DisplayName), destination)
	ciphertext, err := encryptPayoutDestination(m.payoutEncryptionKey, identity.UserID, typeValue, destination)
	if err != nil {
		writeReferralEconomicsError(c, http.StatusServiceUnavailable, "PAYOUT_METHOD_UNAVAILABLE")
		return
	}
	now := time.Now().UTC()
	method := money.PayoutMethod{MethodID: uuid.NewString(), SubjectUserID: identity.UserID, Type: typeValue, DisplayName: strings.TrimSpace(body.DisplayName), MaskedDestination: maskPayoutDestination(destination), SecureReference: ciphertext, Status: money.PayoutMethodActive, CreatedAt: now, UpdatedAt: now, Version: 1}
	created, err := m.payoutMethodWriter.CreatePayoutMethodIdempotent(c.Request.Context(), method, key, fingerprint)
	if err != nil {
		writeReferralEconomicsError(c, http.StatusServiceUnavailable, "PAYOUT_METHOD_UNAVAILABLE")
		return
	}
	writeReferralEconomicsJSON(c, http.StatusCreated, gin.H{"schemaVersion": "payout-method-v1", "methodId": created.MethodID, "type": created.Type, "displayName": created.DisplayName, "maskedDestination": created.MaskedDestination, "version": strconv.FormatInt(created.Version, 10)})
}

func payoutMethodFingerprint(subject string, methodType money.PayoutMethodType, displayName, destination string) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{subject, string(methodType), displayName, destination}, "\x00")))
	return fmt.Sprintf("%x", sum[:])
}

func encryptPayoutDestination(key []byte, subject string, methodType money.PayoutMethodType, destination string) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	associated := []byte(subject + "|" + string(methodType))
	return aead.Seal(nonce, nonce, []byte(destination), associated), nil
}

func maskPayoutDestination(value string) string {
	value = strings.TrimSpace(value)
	runes := []rune(value)
	if len(runes) <= 4 {
		return strings.Repeat("*", len(runes))
	}
	return fmt.Sprintf("%s***%s", string(runes[:1]), string(runes[len(runes)-3:]))
}

func (m referralHTTPModule) requestWithdrawal(c *gin.Context) {
	if !m.economicsRequest(c) || m.economics == nil {
		writeReferralEconomicsError(c, http.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE")
		return
	}
	identity, ok := authidentity.AuthenticatedIdentityFromContext(c.Request.Context())
	if !ok {
		return
	}
	if !m.verifiedPayoutIdentity(c) {
		return
	}
	body, err := decodeWithdrawalBody(c.Request)
	if err != nil {
		writeReferralEconomicsError(c, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}
	key := strings.TrimSpace(c.GetHeader("Idempotency-Key"))
	if key == "" {
		writeReferralEconomicsError(c, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}
	amount, version, err := parseEconomicsInts(body.AmountMinor, body.ExpectedVersion)
	if err != nil {
		writeReferralEconomicsError(c, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}
	if m.payoutMethods == nil {
		writeReferralEconomicsError(c, http.StatusServiceUnavailable, "PAYOUT_METHOD_UNAVAILABLE")
		return
	}
	methods, err := m.payoutMethods.ListActivePayoutMethods(c.Request.Context(), identity.UserID)
	if err != nil {
		writeReferralEconomicsError(c, http.StatusServiceUnavailable, "PAYOUT_METHOD_UNAVAILABLE")
		return
	}
	var selected *money.PayoutMethodSummary
	for index := range methods {
		if methods[index].MethodID == body.PayoutMethodID {
			selected = &methods[index]
			break
		}
	}
	if selected == nil {
		writeReferralEconomicsError(c, http.StatusConflict, "PAYOUT_ELIGIBILITY_UNMET")
		return
	}
	method := economics.WithdrawalMethod(selected.Type)
	out, err := m.economics.RequestWithdrawal(c.Request.Context(), economics.RequestWithdrawal{Referrer: identity.UserID, Currency: economics.CurrencyCNY, AmountMinor: amount, Method: method, PayoutMethodID: body.PayoutMethodID, IdempotencyKey: key, ExpectedVersion: version})
	if err != nil {
		writeEconomicsDomainError(c, err)
		return
	}
	writeWithdrawalJSON(c, out)
}

func (m referralHTTPModule) cancelWithdrawal(c *gin.Context) {
	if !m.economicsRequest(c) || m.economics == nil {
		writeReferralEconomicsError(c, 503, "DEPENDENCY_UNAVAILABLE")
		return
	}
	identity, ok := authidentity.AuthenticatedIdentityFromContext(c.Request.Context())
	if !ok {
		return
	}
	if !m.verifiedPayoutIdentity(c) {
		return
	}
	body, err := decodeCancelBody(c.Request)
	if err != nil {
		writeReferralEconomicsError(c, 400, "INVALID_REQUEST")
		return
	}
	version, err := parseNonnegativeEconomics(body.ExpectedVersion)
	if err != nil {
		writeReferralEconomicsError(c, 400, "INVALID_REQUEST")
		return
	}
	key := strings.TrimSpace(c.GetHeader("Idempotency-Key"))
	if key == "" {
		writeReferralEconomicsError(c, 400, "INVALID_REQUEST")
		return
	}
	out, err := m.economics.CancelWithdrawal(c.Request.Context(), c.Param("withdrawal_id"), identity.UserID, version, key)
	if err != nil {
		writeEconomicsDomainError(c, err)
		return
	}
	writeWithdrawalJSON(c, out)
}

func (m referralHTTPModule) reviewWithdrawal(c *gin.Context) {
	if !m.economicsRequest(c) || m.economics == nil {
		writeReferralEconomicsError(c, 503, "DEPENDENCY_UNAVAILABLE")
		return
	}
	body, err := decodeReviewBody(c.Request)
	if err != nil {
		writeReferralEconomicsError(c, 400, "INVALID_REQUEST")
		return
	}
	version, e := parseNonnegativeEconomics(body.ExpectedVersion)
	if e != nil {
		writeReferralEconomicsError(c, 400, "INVALID_REQUEST")
		return
	}
	key := strings.TrimSpace(c.GetHeader("Idempotency-Key"))
	if key == "" {
		writeReferralEconomicsError(c, 400, "INVALID_REQUEST")
		return
	}
	identity, ok := authidentity.AuthenticatedIdentityFromContext(c.Request.Context())
	if !ok {
		return
	}
	out, err := m.economics.ReviewWithdrawal(c.Request.Context(), economics.ReviewWithdrawal{WithdrawalID: c.Param("withdrawal_id"), ExpectedVersion: version, IdempotencyKey: key, Actor: identity.UserID, Action: economics.WithdrawalStatus(body.Action), ExternalPaymentReference: body.ExternalPaymentReference})
	if err != nil {
		writeEconomicsDomainError(c, err)
		return
	}
	writeWithdrawalJSON(c, out)
}

func (m referralHTTPModule) verifiedPayoutIdentity(c *gin.Context) bool {
	if m.profileReader == nil {
		writeReferralEconomicsError(c, 503, "DEPENDENCY_UNAVAILABLE")
		return false
	}
	identity, ok := authidentity.AuthenticatedIdentityFromContext(c.Request.Context())
	if !ok {
		writeReferralEconomicsError(c, 401, "AUTHENTICATION_REQUIRED")
		return false
	}
	parts := strings.Fields(c.GetHeader("Authorization"))
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		writeReferralEconomicsError(c, 401, "AUTHENTICATION_REQUIRED")
		return false
	}
	profile, err := m.profileReader.ReadSelf(c.Request.Context(), parts[1], identity.UserID)
	if err != nil || profile.EmailVerified == nil || !(*profile.EmailVerified) || profile.PhoneNumberVerified == nil || !(*profile.PhoneNumberVerified) {
		writeReferralEconomicsError(c, 409, "PAYOUT_ELIGIBILITY_UNMET")
		return false
	}
	return true
}
func (m referralHTTPModule) referralRequest(c *gin.Context) bool {
	return c.Request.URL.RawQuery == "" && !c.Request.URL.ForceQuery && c.Request.ContentLength == 0 && len(c.Request.TransferEncoding) == 0
}
func (m referralHTTPModule) economicsRequest(c *gin.Context) bool {
	return c.Request.URL.RawQuery == "" && !c.Request.URL.ForceQuery && c.GetHeader("X-Referral-Service-Credential") == ""
}
func decodeWithdrawalBody(r *http.Request) (withdrawalBody, error) {
	return decodeEconomicsJSON[withdrawalBody](r)
}
func decodeCancelBody(r *http.Request) (cancelBody, error) {
	return decodeEconomicsJSON[cancelBody](r)
}
func decodeReviewBody(r *http.Request) (reviewBody, error) { return decodeEconomicsJSON[reviewBody](r) }
func decodeEconomicsJSON[T any](r *http.Request) (T, error) {
	var value T
	if r.ContentLength <= 0 || r.ContentLength > 8*1024 || len(r.TransferEncoding) > 0 {
		return value, errors.New("invalid economics body")
	}
	decoder := json.NewDecoder(io.LimitReader(r.Body, 8*1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return value, err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return value, errors.New("trailing body")
	}
	return value, nil
}
func parseEconomicsInts(amount, version string) (int64, int64, error) {
	a, e := parseNonnegativeEconomics(amount)
	if e != nil {
		return 0, 0, e
	}
	v, e := parseNonnegativeEconomics(version)
	return a, v, e
}
func parseNonnegativeEconomics(value string) (int64, error) {
	if value == "" || strings.TrimSpace(value) != value || strings.HasPrefix(value, "-") {
		return 0, errors.New("invalid integer")
	}
	parsed, e := strconv.ParseInt(value, 10, 64)
	if e != nil || parsed < 0 {
		return 0, errors.New("invalid integer")
	}
	return parsed, nil
}
func nullableTime(value time.Time) *string {
	if value.IsZero() {
		return nil
	}
	v := value.UTC().Format(time.RFC3339Nano)
	return &v
}
func writeWithdrawalJSON(c *gin.Context, value economics.Withdrawal) {
	writeReferralEconomicsJSON(c, 200, gin.H{"schemaVersion": "referral-withdrawal-v1", "id": value.ID, "currency": value.Currency, "method": value.Method, "payoutMethodId": value.PayoutMethodID, "amountMinor": strconv.FormatInt(value.AmountMinor, 10), "status": value.Status, "payoutReference": value.PayoutReference, "version": strconv.FormatInt(value.Version, 10), "createdAt": value.CreatedAt.UTC(), "updatedAt": value.UpdatedAt.UTC()})
}
func writeWithdrawalsJSON(c *gin.Context, values []economics.Withdrawal) {
	items := make([]gin.H, 0, len(values))
	for _, value := range values {
		items = append(items, gin.H{"id": value.ID, "currency": value.Currency, "method": value.Method, "payoutMethodId": value.PayoutMethodID, "amountMinor": strconv.FormatInt(value.AmountMinor, 10), "status": value.Status, "payoutReference": value.PayoutReference, "version": strconv.FormatInt(value.Version, 10), "createdAt": value.CreatedAt.UTC(), "updatedAt": value.UpdatedAt.UTC()})
	}
	writeReferralEconomicsJSON(c, http.StatusOK, gin.H{"schemaVersion": "referral-withdrawals-v1", "withdrawals": items})
}
func writeReferralEconomicsJSON(c *gin.Context, status int, value any) {
	c.Header("Cache-Control", "private, no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	c.JSON(status, value)
}
func writeReferralEconomicsError(c *gin.Context, status int, code string) {
	writeReferralEconomicsJSON(c, status, gin.H{"code": code, "message": "Referral economics request could not be completed", "requestId": "", "fieldErrors": []any{}})
}
func writeEconomicsDomainError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, economics.ErrConflict), errors.Is(err, economics.ErrIdempotencyConflict):
		writeReferralEconomicsError(c, 409, "CONFLICT")
	case errors.Is(err, economics.ErrInsufficient), errors.Is(err, economics.ErrNotEligible):
		writeReferralEconomicsError(c, 409, "PAYOUT_ELIGIBILITY_UNMET")
	case errors.Is(err, economics.ErrInvalidTransition):
		writeReferralEconomicsError(c, 409, "INVALID_TRANSITION")
	case errors.Is(err, economics.ErrInvalid):
		writeReferralEconomicsError(c, 400, "INVALID_REQUEST")
	default:
		writeReferralEconomicsError(c, 503, "DEPENDENCY_UNAVAILABLE")
	}
}
