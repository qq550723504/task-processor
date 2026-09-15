package httpapi

import (
	"context"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	app "task-processor/internal/app/referralregistration"
	"task-processor/internal/authidentity"
	"task-processor/internal/core/config"
	"task-processor/internal/referral"
	"testing"
	"time"
)

type referralHTTPSpy struct {
	calls   []string
	subject string
	request app.Request
	err     error
}

func (s *referralHTTPSpy) Start(_ context.Context, r app.Request) (app.Admission, error) {
	s.calls = append(s.calls, "start")
	s.request = r
	return app.Admission{IntentID: "fixed-intent", Subject: "private-subject", ResumeSecret: "opaque-resume"}, s.err
}
func (s *referralHTTPSpy) Resume(context.Context, string, string) error {
	s.calls = append(s.calls, "resume")
	return s.err
}
func (s *referralHTTPSpy) identity(ctx context.Context) {
	i, _ := authidentity.AuthenticatedIdentityFromContext(ctx)
	s.subject = i.UserID
}
func (s *referralHTTPSpy) ReadSelf(ctx context.Context) (referral.Projection, error) {
	s.identity(ctx)
	s.calls = append(s.calls, "read")
	return referral.Projection{Count: 0}, s.err
}
func (s *referralHTTPSpy) CreateSelfCode(ctx context.Context) (string, error) {
	s.identity(ctx)
	s.calls = append(s.calls, "code")
	return "real-code", s.err
}
func (s *referralHTTPSpy) Complete(ctx context.Context) (referral.Receipt, error) {
	s.identity(ctx)
	s.calls = append(s.calls, "complete")
	return referral.Receipt{IntentID: "fixed", Fingerprint: "private-proof", BoundAt: time.Now()}, s.err
}

func TestAccountReferralsMountedCurrentIdentityOnly(t *testing.T) {
	f := newAccountFixture(t)
	logger := logrus.New()
	logger.SetOutput(io.Discard)
	cfg := &config.Config{Workbench: config.WorkbenchConfig{Enabled: true}, ListingKit: config.ListingKitConfig{Zitadel: config.ListingKitZitadelConfig{IssuerURL: f.provider.URL, AuthorizationAPIURL: f.provider.URL, ClientID: "fixture-client", ClientSecret: "fixture-secret", ProjectID: "project"}}}
	built, err := buildWorkbenchContextModule(cfg, logger, defaultWorkbenchContextFactories())
	require.NoError(t, err)
	for _, tc := range []struct {
		token, method, path, body string
		status                    int
		call                      string
	}{
		{"no-org", "GET", accountReferralsPath, "", 200, "read"},
		{"removed", "GET", accountReferralsPath, "", 200, "read"},
		{"admin", "GET", accountReferralsPath, "", 200, "read"},
		{"no-org", "POST", accountReferralsPath, "", 200, "code"},
		{"no-org", "POST", accountReferralsCompletePath, "", 200, "complete"},
		{"admin", "GET", accountReferralsPath + "?subject=other", "", 400, ""},
		{"no-org", "POST", accountReferralsCompletePath, `{"subject":"other"}`, 400, ""},
		{"expired", "GET", accountReferralsPath, "", 401, ""},
		{"", "GET", accountReferralsPath, "", 401, ""},
	} {
		t.Run(tc.token+tc.method+tc.path, func(t *testing.T) {
			f.revoked.Store(true)
			spy := &referralHTTPSpy{}
			server := buildCurrentApplicationHTTPServer((referralHTTPModule{commands: spy, serviceCredential: strings.Repeat("a", 64)}).routes(), *built.authDependencies)
			r := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			if tc.token != "" {
				r.Header.Set("Authorization", "Bearer "+tc.token)
			}
			r.Header.Set("X-User-ID", "other")
			r.Header.Set("X-Requested-Organization-ID", "other-org")
			w := httptest.NewRecorder()
			server.Handler.ServeHTTP(w, r)
			require.Equal(t, tc.status, w.Code, w.Body.String())
			if tc.call == "" {
				require.Empty(t, spy.calls)
			} else {
				require.Equal(t, []string{tc.call}, spy.calls)
				require.Equal(t, tc.token, spy.subject)
			}
			require.NotContains(t, w.Body.String(), "private-proof")
			require.Zero(t, f.grantReads.Load())
		})
	}
}

func TestReferralRegistrationPassesOnlyAdmittedInputAndSafeErrors(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status int
	}{{nil, 200}, {referral.ErrConflict, 409}, {referral.ErrPending, 409}, {referral.ErrUnknown, 503}, {referral.ErrExpired, 410}, {referral.ErrLimited, 429}, {io.ErrUnexpectedEOF, 503}} {
		spy := &referralHTTPSpy{err: tc.err}
		server := buildCurrentApplicationHTTPServer((referralHTTPModule{commands: spy, serviceCredential: strings.Repeat("a", 64)}).routes(), routeAuthDependencies{})
		r := httptest.NewRequest(http.MethodPost, referralIntentsPath, strings.NewReader(`{"code":"c","email":"a@example.com","givenName":"A","familyName":"B"}`))
		r.RemoteAddr = "127.0.0.1:1"
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("X-Referral-Service-Credential", strings.Repeat("a", 64))
		r.Header.Set("X-Referral-Client-IP", "203.0.113.1")
		r.Header.Set("Idempotency-Key", strings.Repeat("b", 64))
		w := httptest.NewRecorder()
		server.Handler.ServeHTTP(w, r)
		require.Equal(t, tc.status, w.Code, w.Body.String())
		require.Equal(t, "203.0.113.1", spy.request.ClientIP)
		require.Equal(t, strings.Repeat("b", 64), spy.request.Key)
		require.NotContains(t, w.Body.String(), "private-subject")
		require.NotContains(t, w.Body.String(), "a@example.com")
		require.NotContains(t, w.Body.String(), "unexpected EOF")
	}
}
