package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"task-processor/internal/authidentity"
	"task-processor/internal/ledger/money"
	economics "task-processor/internal/referraleconomics"
)

type withdrawalKYCReplayStub struct {
	result economics.Withdrawal
	found  bool
	err    error
	calls  int
	input  economics.WithdrawalReplayRequest
}

func (s *withdrawalKYCReplayStub) ReplayWithdrawal(_ context.Context, input economics.WithdrawalReplayRequest) (economics.Withdrawal, bool, error) {
	s.calls++
	s.input = input
	return s.result, s.found, s.err
}

type withdrawalKYCReaderStub struct {
	verified bool
	err      error
	calls    int
	subject  string
}

func (s *withdrawalKYCReaderStub) IsPersonalVerified(_ context.Context, subject string) (bool, error) {
	s.calls++
	s.subject = subject
	return s.verified, s.err
}

type withdrawalKYCProfileStub struct {
	err   error
	calls int
}

func (s *withdrawalKYCProfileStub) ReadSelf(_ context.Context, _ string, subject string) (authidentity.SelfProfile, error) {
	s.calls++
	if s.err != nil {
		return authidentity.SelfProfile{}, s.err
	}
	yes := true
	return authidentity.SelfProfile{UserID: subject, EmailVerified: &yes, PhoneNumberVerified: &yes}, nil
}

type withdrawalKYCPayoutStub struct {
	calls int
	err   error
}

func (s *withdrawalKYCPayoutStub) HasValidPayoutMethod(context.Context, string, string) (bool, error) {
	return true, nil
}
func (s *withdrawalKYCPayoutStub) ListActivePayoutMethods(_ context.Context, subject string) ([]money.PayoutMethodSummary, error) {
	s.calls++
	if s.err != nil {
		return nil, s.err
	}
	return []money.PayoutMethodSummary{{
		MethodID: "method-1", SubjectUserID: subject, Type: money.PayoutAlipay,
		DisplayName: "支付宝", MaskedDestination: "***1234", Status: money.PayoutMethodActive, Version: 1,
	}}, nil
}
func (s *withdrawalKYCPayoutStub) ReadPayoutMethodForReview(context.Context, string) (money.PayoutMethod, error) {
	return money.PayoutMethod{}, nil
}

type withdrawalKYCEconomicsStub struct {
	requestCalls int
	cancelCalls  int
	requestInput economics.RequestWithdrawal
	result       economics.Withdrawal
}

func (s *withdrawalKYCEconomicsStub) ReadEarnings(context.Context, string, string) (economics.Earnings, error) {
	return economics.Earnings{}, nil
}
func (s *withdrawalKYCEconomicsStub) ReadEarningsSnapshot(context.Context, string, string, int) (economics.EarningsSnapshot, error) {
	return economics.EarningsSnapshot{}, nil
}
func (s *withdrawalKYCEconomicsStub) RequestWithdrawal(_ context.Context, input economics.RequestWithdrawal) (economics.Withdrawal, error) {
	s.requestCalls++
	s.requestInput = input
	return s.result, nil
}
func (s *withdrawalKYCEconomicsStub) CancelWithdrawal(_ context.Context, id, referrer string, version int64, key string) (economics.Withdrawal, error) {
	s.cancelCalls++
	out := s.result
	out.ID = id
	out.Referrer = referrer
	out.Status = economics.WithdrawalCanceled
	out.Version = version + 1
	return out, nil
}
func (s *withdrawalKYCEconomicsStub) ReviewWithdrawal(context.Context, economics.ReviewWithdrawal) (economics.Withdrawal, error) {
	return economics.Withdrawal{}, nil
}
func (s *withdrawalKYCEconomicsStub) Mature(context.Context, time.Time) error { return nil }

func withdrawalKYCContext(t *testing.T, path, body, key string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	request.Header.Set("Authorization", "Bearer current-token")
	request.Header.Set("Idempotency-Key", key)
	request = request.WithContext(authidentity.WithAuthenticatedIdentity(request.Context(), authidentity.AuthenticatedIdentity{UserID: "person-1"}))
	c.Request = request
	return c, recorder
}

func withdrawalKYCResult() economics.Withdrawal {
	now := time.Date(2026, 9, 27, 1, 2, 3, 0, time.UTC)
	return economics.Withdrawal{
		ID: "withdrawal-1", Referrer: "person-1", Currency: economics.CurrencyCNY,
		Method: economics.MethodAlipay, PayoutMethodID: "method-1",
		AmountMinor: economics.MinimumWithdrawalMinor, Status: economics.WithdrawalRequested,
		Version: 1, CreatedAt: now, UpdatedAt: now,
	}
}

func TestWithdrawalAlwaysRequiresPersonalKYC(t *testing.T) {
	result := withdrawalKYCResult()
	replay := &withdrawalKYCReplayStub{}
	profile := &withdrawalKYCProfileStub{}
	payout := &withdrawalKYCPayoutStub{}
	economicsStub := &withdrawalKYCEconomicsStub{result: result}
	c, recorder := withdrawalKYCContext(t, accountReferralWithdrawalsPath, `{"amountMinor":"10000","payoutMethodId":"method-1","expectedVersion":"1"}`, "request-key")

	module := referralHTTPModule{
		economics: economicsStub, withdrawalReplay: replay,
		profileReader: profile, payoutMethods: payout,
		// Stage B is active: an absent KYC reader must fail closed rather than
		// silently reverting to the pre-KYC withdrawal path.
	}
	module.requestWithdrawal(c)

	if recorder.Code != http.StatusServiceUnavailable || !strings.Contains(recorder.Body.String(), "DEPENDENCY_UNAVAILABLE") {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if replay.calls != 1 || profile.calls != 1 || payout.calls != 0 || economicsStub.requestCalls != 0 {
		t.Fatalf("replay=%d profile=%d payout=%d mutation=%d", replay.calls, profile.calls, payout.calls, economicsStub.requestCalls)
	}
}

func TestWithdrawalCommittedReplayPrecedesMutableEligibilityDependencies(t *testing.T) {
	result := withdrawalKYCResult()
	replay := &withdrawalKYCReplayStub{result: result, found: true}
	economicsStub := &withdrawalKYCEconomicsStub{result: result}
	c, recorder := withdrawalKYCContext(t, accountReferralWithdrawalsPath, `{"amountMinor":"10000","payoutMethodId":"method-1","expectedVersion":"7"}`, "request-key")

	module := referralHTTPModule{
		economics:        economicsStub,
		withdrawalReplay: replay,
		// A committed replay must not need profile, KYC, or payout-method dependencies.
	}
	module.requestWithdrawal(c)

	if recorder.Code != http.StatusOK || replay.calls != 1 || economicsStub.requestCalls != 0 {
		t.Fatalf("replay status=%d replayCalls=%d requestCalls=%d body=%s", recorder.Code, replay.calls, economicsStub.requestCalls, recorder.Body.String())
	}
	if replay.input.Referrer != "person-1" || replay.input.IdempotencyKey != "request-key" || replay.input.PayoutMethodID != "method-1" || replay.input.ExpectedVersion != 7 {
		t.Fatalf("replay input=%+v", replay.input)
	}
}

func TestWithdrawalNeverSeenKeyRequiresPersonalKYCBeforePayoutAndMutation(t *testing.T) {
	for _, tc := range []struct {
		name       string
		verified   bool
		kycErr     error
		missingKYC bool
		wantStatus int
		wantCode   string
	}{
		{name: "not verified", verified: false, wantStatus: http.StatusConflict, wantCode: "PAYOUT_ELIGIBILITY_UNMET"},
		{name: "KYC dependency unavailable", kycErr: errors.New("kyc unavailable"), wantStatus: http.StatusServiceUnavailable, wantCode: "DEPENDENCY_UNAVAILABLE"},
		{name: "KYC capability not configured", missingKYC: true, wantStatus: http.StatusServiceUnavailable, wantCode: "DEPENDENCY_UNAVAILABLE"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := withdrawalKYCResult()
			replay := &withdrawalKYCReplayStub{}
			kyc := &withdrawalKYCReaderStub{verified: tc.verified, err: tc.kycErr}
			profile := &withdrawalKYCProfileStub{}
			payout := &withdrawalKYCPayoutStub{}
			economicsStub := &withdrawalKYCEconomicsStub{result: result}
			c, recorder := withdrawalKYCContext(t, accountReferralWithdrawalsPath, `{"amountMinor":"10000","payoutMethodId":"method-1","expectedVersion":"1"}`, "request-key")

			var kycReader personalKYCReader = kyc
			if tc.missingKYC {
				kycReader = nil
			}
			module := referralHTTPModule{economics: economicsStub, withdrawalReplay: replay, personalKYC: kycReader, profileReader: profile, payoutMethods: payout}
			module.requestWithdrawal(c)

			if recorder.Code != tc.wantStatus || !strings.Contains(recorder.Body.String(), tc.wantCode) {
				t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
			}
			wantKYCCalls := 1
			if tc.missingKYC {
				wantKYCCalls = 0
			}
			if replay.calls != 1 || profile.calls != 1 || kyc.calls != wantKYCCalls || (!tc.missingKYC && kyc.subject != "person-1") || payout.calls != 0 || economicsStub.requestCalls != 0 {
				t.Fatalf("calls replay=%d profile=%d kyc=%d subject=%q payout=%d mutation=%d", replay.calls, profile.calls, kyc.calls, kyc.subject, payout.calls, economicsStub.requestCalls)
			}
		})
	}
}

func TestWithdrawalVerifiedPersonalKYCContinuesExistingAdmission(t *testing.T) {
	result := withdrawalKYCResult()
	replay := &withdrawalKYCReplayStub{}
	kyc := &withdrawalKYCReaderStub{verified: true}
	profile := &withdrawalKYCProfileStub{}
	payout := &withdrawalKYCPayoutStub{}
	economicsStub := &withdrawalKYCEconomicsStub{result: result}
	c, recorder := withdrawalKYCContext(t, accountReferralWithdrawalsPath, `{"amountMinor":"10000","payoutMethodId":"method-1","expectedVersion":"1"}`, "request-key")

	module := referralHTTPModule{economics: economicsStub, withdrawalReplay: replay, personalKYC: kyc, profileReader: profile, payoutMethods: payout}
	module.requestWithdrawal(c)

	if recorder.Code != http.StatusOK || replay.calls != 1 || profile.calls != 1 || kyc.calls != 1 || payout.calls != 1 || economicsStub.requestCalls != 1 {
		t.Fatalf("status=%d replay=%d profile=%d kyc=%d payout=%d mutation=%d body=%s", recorder.Code, replay.calls, profile.calls, kyc.calls, payout.calls, economicsStub.requestCalls, recorder.Body.String())
	}
	if economicsStub.requestInput.Referrer != "person-1" || economicsStub.requestInput.Method != economics.MethodAlipay || economicsStub.requestInput.IdempotencyKey != "request-key" {
		t.Fatalf("request input=%+v", economicsStub.requestInput)
	}
}

func TestCancelExistingWithdrawalDoesNotAcquirePersonalKYCDependency(t *testing.T) {
	result := withdrawalKYCResult()
	kyc := &withdrawalKYCReaderStub{err: errors.New("must not be called")}
	profile := &withdrawalKYCProfileStub{}
	economicsStub := &withdrawalKYCEconomicsStub{result: result}
	c, recorder := withdrawalKYCContext(t, accountReferralWithdrawalsPath+"/withdrawal-1/cancel", `{"expectedVersion":"1"}`, "cancel-key")
	c.Params = gin.Params{{Key: "withdrawal_id", Value: "withdrawal-1"}}

	module := referralHTTPModule{economics: economicsStub, personalKYC: kyc, profileReader: profile}
	module.cancelWithdrawal(c)

	if recorder.Code != http.StatusOK || economicsStub.cancelCalls != 1 || kyc.calls != 0 {
		t.Fatalf("status=%d cancel=%d kyc=%d body=%s", recorder.Code, economicsStub.cancelCalls, kyc.calls, recorder.Body.String())
	}
}
