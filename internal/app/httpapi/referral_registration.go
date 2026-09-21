package httpapi

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net"
	"net/http"
	"net/netip"
	"time"

	"github.com/gin-gonic/gin"
	app "task-processor/internal/app/referralregistration"
	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	"task-processor/internal/core/config"
	"task-processor/internal/httproute"
	kernelmodule "task-processor/internal/kernel/module"
	"task-processor/internal/ledger/money"
	"task-processor/internal/referral"
	economics "task-processor/internal/referraleconomics"
)

const referralIntentsPath = "/api/v1/referral-registration/intents"
const referralResumePath = "/api/v1/referral-registration/resume"
const accountReferralsPath = "/api/v1/account/referrals"
const accountReferralsCompletePath = accountReferralsPath + "/complete"
const accountReferralEarningsPath = accountReferralsPath + "/earnings"
const accountReferralRulesPath = accountReferralsPath + "/rules"
const accountReferralPayoutMethodsPath = accountReferralsPath + "/payout-methods"
const accountReferralWithdrawalsPath = accountReferralsPath + "/withdrawals"
const accountReferralWithdrawalReviewQueuePath = accountReferralWithdrawalsPath + "/review-queue"
const accountReferralWithdrawalCancelPath = accountReferralWithdrawalsPath + "/:withdrawal_id/cancel"
const accountReferralWithdrawalReviewPath = accountReferralWithdrawalsPath + "/:withdrawal_id/review"
const internalReferralPaymentSettlementPath = "/api/v1/internal/referrals/payment-settlements"
const internalReferralRefundSettlementPath = "/api/v1/internal/referrals/refund-settlements"
const internalReferralChargebackSettlementPath = "/api/v1/internal/referrals/chargeback-settlements"
const internalReferralMaturityPath = "/api/v1/internal/referrals/mature"

type referralCommands interface {
	Start(context.Context, app.Request) (app.Admission, error)
	Resume(context.Context, string, string) error
	ReadSelf(context.Context) (referral.Projection, error)
	CreateSelfCode(context.Context) (string, error)
	Complete(context.Context) (referral.Receipt, error)
}

type referralEconomics interface {
	ReadEarnings(context.Context, string, string) (economics.Earnings, error)
	RequestWithdrawal(context.Context, economics.RequestWithdrawal) (economics.Withdrawal, error)
	CancelWithdrawal(context.Context, string, string, int64, string) (economics.Withdrawal, error)
	ReviewWithdrawal(context.Context, economics.ReviewWithdrawal) (economics.Withdrawal, error)
	Mature(context.Context, time.Time) error
}

type earningsLedgerReader interface {
	ListEarningsLedger(context.Context, string, string, int) ([]economics.EarningsLedgerEntry, error)
}

type withdrawalReader interface {
	ListWithdrawals(context.Context, string) ([]economics.Withdrawal, error)
	ListPendingWithdrawals(context.Context) ([]economics.Withdrawal, error)
}

type settlementWriter interface {
	RecordPaymentSettlementAndNotify(context.Context, money.PaymentSettlement, money.SettlementObserver) error
	RecordRefundSettlementAndNotify(context.Context, money.RefundSettlement, money.SettlementObserver) error
	RecordChargebackSettlementAndNotify(context.Context, money.ChargebackSettlement, money.SettlementObserver) error
}

// payoutMethodReader is intentionally separate from referral economics. A
// withdrawal may only be requested when an existing canonical account owner
// confirms a valid method for the current person; a channel enum alone is not
// payout-method persistence.
type payoutMethodReader interface {
	HasValidPayoutMethod(context.Context, string, string) (bool, error)
	ListActivePayoutMethods(context.Context, string) ([]money.PayoutMethodSummary, error)
	ReadPayoutMethodForReview(context.Context, string) (money.PayoutMethod, error)
}

type payoutMethodWriter interface {
	CreatePayoutMethodIdempotent(context.Context, money.PayoutMethod, string, string) (money.PayoutMethod, error)
}

type referralHTTPModule struct {
	commands              referralCommands
	serviceCredential     string
	economics             referralEconomics
	ledgerReader          earningsLedgerReader
	withdrawals           withdrawalReader
	payoutMethods         payoutMethodReader
	payoutMethodWriter    payoutMethodWriter
	payoutEncryptionKeys  map[string][]byte
	payoutEncryptionKeyID string
	profileReader         authidentity.SelfProfileReader
	settlements           settlementWriter
	onSlotAcquiredForTest func()
}

func (referralHTTPModule) Name() string { return "personal-referrals" }
func (m referralHTTPModule) Enabled(cfg *config.Config) bool {
	return cfg != nil && cfg.Referrals.Enabled
}
func (m referralHTTPModule) Register(reg *kernelmodule.Registry) error {
	if m.commands == nil || len(m.serviceCredential) < 64 {
		return referral.ErrUnavailable
	}
	reg.AddRoutes(m.routes()...)
	return nil
}
func (m referralHTTPModule) routes() []httproute.Descriptor {
	slots := make(chan struct{}, 8)
	// The nil production default has no behavior change. Tests supply this
	// package-private hook solely to wait until real TCP requests hold slots.
	onSlotAcquiredForTest := m.onSlotAcquiredForTest
	routes := []httproute.Descriptor{
		{Method: http.MethodPost, Path: referralIntentsPath, AuthPolicy: httproute.AuthPolicyPublic, Handler: m.start},
		{Method: http.MethodPost, Path: referralResumePath, AuthPolicy: httproute.AuthPolicyPublic, Handler: m.resume},
		{Method: http.MethodGet, Path: accountReferralsPath, AuthPolicy: httproute.AuthPolicyCurrentIdentity, Handler: m.readSelf},
		{Method: http.MethodPost, Path: accountReferralsPath, AuthPolicy: httproute.AuthPolicyCurrentIdentity, Handler: m.createSelfCode},
		{Method: http.MethodPost, Path: accountReferralsCompletePath, AuthPolicy: httproute.AuthPolicyCurrentIdentity, Handler: m.complete},
		{Method: http.MethodGet, Path: accountReferralEarningsPath, AuthPolicy: httproute.AuthPolicyCurrentIdentity, Handler: m.readEarnings},
		{Method: http.MethodGet, Path: accountReferralRulesPath, AuthPolicy: httproute.AuthPolicyCurrentIdentity, Handler: m.readRules},
		{Method: http.MethodGet, Path: accountReferralPayoutMethodsPath, AuthPolicy: httproute.AuthPolicyCurrentIdentity, Handler: m.readPayoutMethods},
		{Method: http.MethodPost, Path: accountReferralPayoutMethodsPath, AuthPolicy: httproute.AuthPolicyCurrentIdentity, Handler: m.createPayoutMethod},
		{Method: http.MethodPost, Path: accountReferralWithdrawalsPath, AuthPolicy: httproute.AuthPolicyCurrentIdentity, Handler: m.requestWithdrawal},
		{Method: http.MethodGet, Path: accountReferralWithdrawalsPath, AuthPolicy: httproute.AuthPolicyCurrentIdentity, Handler: m.readWithdrawals},
		{Method: http.MethodGet, Path: accountReferralWithdrawalReviewQueuePath, AuthPolicy: httproute.AuthPolicyCurrentIdentityWithVerifiedRoles, Permission: authz.PermissionListingKitPlatformAdm, Handler: m.readWithdrawalQueue},
		{Method: http.MethodPost, Path: accountReferralWithdrawalCancelPath, AuthPolicy: httproute.AuthPolicyCurrentIdentity, Handler: m.cancelWithdrawal},
		{Method: http.MethodPost, Path: accountReferralWithdrawalReviewPath, AuthPolicy: httproute.AuthPolicyCurrentIdentityWithVerifiedRoles, Permission: authz.PermissionListingKitPlatformAdm, Handler: m.reviewWithdrawal},
		{Method: http.MethodPost, Path: internalReferralPaymentSettlementPath, AuthPolicy: httproute.AuthPolicyPublic, Handler: m.recordPaymentSettlement},
		{Method: http.MethodPost, Path: internalReferralRefundSettlementPath, AuthPolicy: httproute.AuthPolicyPublic, Handler: m.recordRefundSettlement},
		{Method: http.MethodPost, Path: internalReferralChargebackSettlementPath, AuthPolicy: httproute.AuthPolicyPublic, Handler: m.recordChargebackSettlement},
		{Method: http.MethodPost, Path: internalReferralMaturityPath, AuthPolicy: httproute.AuthPolicyPublic, Handler: m.matureReferralEarnings},
	}
	for i := range routes {
		routes[i].Module = m.Name()
		routes[i].OrganizationAccessPolicy = httproute.OrganizationAccessPolicyNone
		routes[i].RequestTimeout = 15 * time.Second
		if i < 2 {
			routes[i].Handler = withReferralBodyDeadline(routes[i].Handler)
		} else {
			routes[i].RejectUnreadRequestBody = true
		}
		handler := routes[i].Handler
		routes[i].Handler = func(c *gin.Context) {
			defer func() {
				if recover() != nil {
					// A mutation may already have committed. Preserve UNKNOWN and
					// never let generic recovery dump credentials or panic values.
					writeReferralError(c, referral.ErrUnknown)
				}
			}()
			select {
			case slots <- struct{}{}:
				defer func() { <-slots }()
				if onSlotAcquiredForTest != nil {
					onSlotAcquiredForTest()
				}
				handler(c)
			default:
				writeReferralError(c, referral.ErrLimited)
			}
		}
	}
	return routes
}

func withReferralBodyDeadline(handler gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		deadline, ok := c.Request.Context().Deadline()
		if !ok {
			deadline = time.Now().Add(15 * time.Second)
		}
		// Closing Body can block behind net/http's active Read mutex. A socket
		// deadline interrupts that Read and releases the shared handler slot.
		err := http.NewResponseController(c.Writer).SetReadDeadline(deadline)
		if err != nil && c.Request.Context().Value(http.ServerContextKey) != nil {
			writeReferralError(c, referral.ErrUnavailable)
			return
		}
		// In-process requests have no socket. Real servers must support the
		// controller; net/http resets its read deadline for the next request.
		handler(c)
	}
}

func (m referralHTTPModule) trustedCommand(c *gin.Context) (string, bool) {
	provided := c.GetHeader("X-Referral-Service-Credential")
	credentialCount := len(c.Request.Header.Values("X-Referral-Service-Credential"))
	c.Request.Header.Del("X-Referral-Service-Credential")
	want, got := sha256.Sum256([]byte(m.serviceCredential)), sha256.Sum256([]byte(provided))
	host, _, err := net.SplitHostPort(c.Request.RemoteAddr)
	peer, parseErr := netip.ParseAddr(host)
	if len(m.serviceCredential) < 64 || credentialCount != 1 || subtle.ConstantTimeCompare(want[:], got[:]) != 1 || c.GetHeader("Authorization") != "" || err != nil || parseErr != nil || !peer.IsLoopback() {
		writeReferralError(c, referral.ErrUnauthenticated)
		return "", false
	}
	// Only the credential-bearing loopback BFF can assert this value. Public
	// Forwarded/XFF headers are never interpreted as a client identity or IP.
	ip, err := netip.ParseAddr(c.GetHeader("X-Referral-Client-IP"))
	if err != nil || ip.Zone() != "" || ip.IsUnspecified() || ip.IsMulticast() || len(c.Request.Header.Values("X-Referral-Client-IP")) != 1 || c.GetHeader("Forwarded") != "" || c.GetHeader("X-Forwarded-For") != "" || c.Request.URL.RawQuery != "" || c.Request.URL.ForceQuery {
		writeReferralError(c, referral.ErrInvalid)
		return "", false
	}
	return ip.Unmap().String(), true
}

func decodeReferralBody(c *gin.Context, out any) bool {
	media, _, err := mime.ParseMediaType(c.GetHeader("Content-Type"))
	if err != nil || media != "application/json" {
		writeReferralError(c, referral.ErrInvalid)
		return false
	}
	data, err := io.ReadAll(io.LimitReader(c.Request.Body, 4097))
	if len(data) > 4096 {
		httproute.RejectUnreadRequestBody(c)
		c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, gin.H{"error": "referral_body_too_large"})
		return false
	}
	if err != nil || len(data) == 0 || bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		writeReferralError(c, referral.ErrInvalid)
		return false
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	// Commands are flat string objects. Reject duplicate/case-variant keys and
	// nulls before Go's permissive struct decoding can collapse their meaning.
	token, tokenErr := decoder.Token()
	valid := tokenErr == nil && token == json.Delim('{')
	seen := map[string]bool{}
	allowed := map[string]bool{"code": true, "email": true, "givenName": true, "familyName": true}
	if c.Request.URL.Path == referralResumePath {
		allowed = map[string]bool{"intentID": true, "resumeSecret": true}
	}
	for valid && decoder.More() {
		k, e := decoder.Token()
		key, ok := k.(string)
		if e != nil || !ok || !allowed[key] || seen[key] {
			valid = false
			break
		}
		seen[key] = true
		v, e := decoder.Token()
		_, ok = v.(string)
		valid = e == nil && ok
	}
	if !valid {
		writeReferralError(c, referral.ErrInvalid)
		return false
	}
	decoder = json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if decoder.Decode(out) != nil || decoder.Decode(new(any)) != io.EOF {
		writeReferralError(c, referral.ErrInvalid)
		return false
	}
	return true
}

func (m referralHTTPModule) start(c *gin.Context) {
	ip, ok := m.trustedCommand(c)
	if !ok {
		return
	}
	var body struct {
		Code       string `json:"code"`
		Email      string `json:"email"`
		GivenName  string `json:"givenName"`
		FamilyName string `json:"familyName"`
	}
	if !decodeReferralBody(c, &body) {
		return
	}
	if len(c.Request.Header.Values("Idempotency-Key")) != 1 {
		writeReferralError(c, referral.ErrInvalid)
		return
	}
	out, err := m.commands.Start(c.Request.Context(), app.Request{Key: c.GetHeader("Idempotency-Key"), Code: body.Code, Email: body.Email, GivenName: body.GivenName, FamilyName: body.FamilyName, ClientIP: ip})
	if err != nil {
		writeReferralError(c, err)
		return
	}
	writeReferralJSON(c, gin.H{"intentID": out.IntentID, "resumeSecret": out.ResumeSecret, "createExpiresAt": out.CreateExpiresAt.UTC(), "completionExpiresAt": out.CompletionExpiresAt.UTC()})
}
func (m referralHTTPModule) resume(c *gin.Context) {
	if _, ok := m.trustedCommand(c); !ok {
		return
	}
	var body struct {
		IntentID     string `json:"intentID"`
		ResumeSecret string `json:"resumeSecret"`
	}
	if !decodeReferralBody(c, &body) {
		return
	}
	if err := m.commands.Resume(c.Request.Context(), body.IntentID, body.ResumeSecret); err != nil {
		writeReferralError(c, err)
		return
	}
	writeReferralJSON(c, gin.H{"status": "created"})
}
func writeReferralJSON(c *gin.Context, value any) {
	data, err := json.Marshal(value)
	if err != nil || len(data) > 16384 {
		writeReferralError(c, referral.ErrUnavailable)
		return
	}
	c.Data(http.StatusOK, "application/json; charset=utf-8", data)
}
func writeReferralError(c *gin.Context, err error) {
	status, code := http.StatusServiceUnavailable, referral.ErrUnavailable.Error()
	for _, candidate := range []struct {
		err    error
		status int
	}{
		{referral.ErrInvalid, 400}, {referral.ErrUnauthenticated, 401}, {referral.ErrMissing, 404},
		{referral.ErrConflict, 409}, {referral.ErrExpired, 410}, {referral.ErrPending, 409},
		{referral.ErrUnknown, 503}, {referral.ErrLimited, 429},
	} {
		if errors.Is(err, candidate.err) {
			status, code = candidate.status, candidate.err.Error()
			break
		}
	}
	httproute.RejectUnreadRequestBody(c)
	c.AbortWithStatusJSON(status, gin.H{"error": code})
}
