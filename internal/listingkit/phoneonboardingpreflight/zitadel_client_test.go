package phoneonboardingpreflight

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Request-contract provenance is pinned to the immutable ZITADEL v4.17.1
// sources. The operation tests below exercise only this version boundary:
// https://github.com/zitadel/zitadel/blob/v4.17.1/proto/zitadel/org/v2/org_service.proto
// https://github.com/zitadel/zitadel/blob/v4.17.1/proto/zitadel/user/v2/user_service.proto
// https://github.com/zitadel/zitadel/blob/v4.17.1/proto/zitadel/session/v2/session_service.proto
// https://github.com/zitadel/zitadel/blob/v4.17.1/proto/zitadel/session/v2/session.proto
// https://github.com/zitadel/zitadel/blob/v4.17.1/proto/zitadel/session/v2/challenge.proto

func TestClientUsesPinnedZITADELRequestContractsAndScopedTokens(t *testing.T) {
	t.Parallel()

	const (
		provisioningToken = "org-manager-token"
		sessionToken      = "login-client-token"
	)
	type capturedRequest struct {
		Method        string
		Path          string
		RawQuery      string
		Authorization string
		Body          []byte
	}
	var requests []capturedRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		requests = append(requests, capturedRequest{
			Method: r.Method, Path: r.URL.Path, RawQuery: r.URL.RawQuery,
			Authorization: r.Header.Get("Authorization"), Body: body,
		})
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v2/organizations":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"organizationId":"org-preflight"}`))
		case "/v2/users/new":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"user-preflight"}`))
		case "/v2/users/user-preflight/otp_sms":
			_, _ = w.Write([]byte(`{}`))
		case "/v2/sessions":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"sessionId":"session-preflight","sessionToken":"created-session-token"}`))
		case "/v2/sessions/session-preflight":
			switch r.Method {
			case http.MethodPatch:
				_, _ = w.Write([]byte(`{"sessionToken":"verified-session-token"}`))
			case http.MethodGet:
				_, _ = w.Write([]byte(`{"session":{"factors":{"user":{"id":"user-preflight","organizationId":"org-preflight","verifiedAt":"2026-08-25T01:02:03Z"},"otpSms":{"verifiedAt":"2026-08-25T01:02:04Z"}}}}`))
			case http.MethodDelete:
				w.WriteHeader(http.StatusNoContent)
			default:
				w.WriteHeader(http.StatusMethodNotAllowed)
			}
		case "/v2/organizations/org-preflight":
			if r.Method != http.MethodDelete {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"deletionDate":"2026-08-25T01:02:05Z"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, provisioningToken, sessionToken, server.Client())
	organizationID, err := client.CreateOrganization(context.Background(), "lk-phone-preflight-01JTEST")
	require.NoError(t, err)
	require.Equal(t, "org-preflight", organizationID)
	userID, err := client.CreateTechnicalUser(context.Background(), TechnicalUserInput{
		OrganizationID: "org-preflight", Username: "lkp-01JTEST",
		TechnicalEmail: "u-01JTEST@phone.invalid", Phone: "+8613712345678",
	})
	require.NoError(t, err)
	require.Equal(t, "user-preflight", userID)
	require.NoError(t, client.AddOTPSMS(context.Background(), userID))
	material, err := client.CreateSMSChallenge(context.Background(), userID, 5*time.Minute)
	require.NoError(t, err)
	require.Equal(t, SessionMaterial{ID: "session-preflight", Token: "created-session-token"}, material)
	replacementToken, err := client.VerifySMS(context.Background(), material.ID, "654321")
	require.NoError(t, err)
	require.Equal(t, "verified-session-token", replacementToken)
	proof, err := client.GetSession(context.Background(), material.ID, replacementToken)
	require.NoError(t, err)
	require.Equal(t, SessionProof{
		UserID: "user-preflight", OrganizationID: "org-preflight",
		UserVerifiedAt:   time.Date(2026, 8, 25, 1, 2, 3, 0, time.UTC),
		OTPSMSVerifiedAt: time.Date(2026, 8, 25, 1, 2, 4, 0, time.UTC),
	}, proof)
	require.NoError(t, client.DeleteSession(context.Background(), material.ID))
	require.NoError(t, client.DeleteOrganization(context.Background(), organizationID))

	require.Len(t, requests, 8)
	require.Equal(t, "/v2/organizations", requests[0].Path)
	require.Equal(t, "/v2/users/new", requests[1].Path)
	require.Equal(t, "/v2/users/user-preflight/otp_sms", requests[2].Path)
	require.Equal(t, "/v2/sessions", requests[3].Path)
	require.Equal(t, "/v2/sessions/session-preflight", requests[4].Path)
	require.Equal(t, "/v2/sessions/session-preflight", requests[5].Path)
	require.Equal(t, "/v2/sessions/session-preflight", requests[6].Path)
	require.Equal(t, http.MethodPost, requests[2].Method)
	require.Equal(t, http.MethodPatch, requests[4].Method)
	require.Equal(t, http.MethodGet, requests[5].Method)
	require.Equal(t, http.MethodDelete, requests[6].Method)
	require.Equal(t, "/v2/organizations/org-preflight", requests[7].Path)
	require.Equal(t, http.MethodDelete, requests[7].Method)
	for index, request := range requests {
		want := "Bearer " + sessionToken
		if index < 3 || index == 7 {
			want = "Bearer " + provisioningToken
		}
		require.Equalf(t, want, request.Authorization, "request %d authorization", index)
	}

	assertJSONEqual(t, map[string]any{"name": "lk-phone-preflight-01JTEST"}, requests[0].Body)
	wantCreateUser := map[string]any{
		"organizationId": "org-preflight",
		"username":       "lkp-01JTEST",
		"human": map[string]any{
			"profile": map[string]any{"givenName": "Phone", "familyName": "Preflight", "displayName": "Phone Preflight"},
			"email":   map[string]any{"email": "u-01JTEST@phone.invalid", "isVerified": true},
			"phone":   map[string]any{"phone": "+8613712345678", "isVerified": true},
		},
	}
	assertJSONEqual(t, wantCreateUser, requests[1].Body)
	require.NotContains(t, string(requests[1].Body), "password")
	require.NotContains(t, string(requests[1].Body), "hashedPassword")
	require.NotContains(t, string(requests[1].Body), "13712345678@")
	assertJSONEqual(t, map[string]any{}, requests[2].Body)
	wantCreateSession := map[string]any{
		"checks":     map[string]any{"user": map[string]any{"userId": "user-preflight"}},
		"challenges": map[string]any{"otpSms": map[string]any{"returnCode": false}},
		"lifetime":   "300s",
	}
	assertJSONEqual(t, wantCreateSession, requests[3].Body)
	wantVerifySession := map[string]any{"checks": map[string]any{"otpSms": map[string]any{"code": "654321"}}}
	assertJSONEqual(t, wantVerifySession, requests[4].Body)
	require.Equal(t, "sessionToken=verified-session-token", requests[5].RawQuery)
}

func TestClientErrorsDoNotExposeProviderBodyTokenPhoneOrCode(t *testing.T) {
	t.Parallel()
	const (
		provisioningToken = "org-manager-token-secret"
		sessionToken      = "login-client-token-secret"
		phone             = "+8613712345678"
		code              = "654321"
		providerBody      = "provider-secret-body"
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, providerBody+" "+provisioningToken+" "+sessionToken+" "+phone+" "+code, http.StatusForbidden)
	}))
	defer server.Close()
	client := newTestClient(t, server.URL, provisioningToken, sessionToken, server.Client())

	_, err := client.VerifySMS(context.Background(), "session-secret", code)
	require.Error(t, err)
	require.Contains(t, err.Error(), "verify ZITADEL SMS session: ZITADEL returned HTTP status 403")
	assertDoesNotContain(t, err.Error(), providerBody, provisioningToken, sessionToken, phone, code, "session-secret")

	transportClient, err := NewClient(ClientConfig{
		IssuerURL: "https://issuer.invalid", ProvisioningToken: provisioningToken, SessionToken: sessionToken,
		HTTPClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, errors.New("transport-secret " + providerBody + " " + phone + " " + code)
		})},
	})
	require.NoError(t, err)
	_, err = transportClient.VerifySMS(context.Background(), "session-secret", code)
	require.EqualError(t, err, "verify ZITADEL SMS session: request failed")
	assertDoesNotContain(t, err.Error(), providerBody, provisioningToken, sessionToken, phone, code, "session-secret", "transport-secret")
}

func TestClientRejectsMissingSessionMaterial(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"sessionId":"session-only"}`))
	}))
	defer server.Close()
	client := newTestClient(t, server.URL, "provisioning-token", "session-token", server.Client())

	_, err := client.CreateSMSChallenge(context.Background(), "user-secret", time.Minute)
	require.EqualError(t, err, "ZITADEL session creation returned incomplete material")
	assertDoesNotContain(t, err.Error(), "user-secret")
}

func TestClientRejectsSessionWithoutUserAndOTPSMSFactors(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"session":{"factors":{"user":{"id":"user-secret","organizationId":"org-secret","verifiedAt":"2026-08-25T01:02:03Z"}}}}`))
	}))
	defer server.Close()
	client := newTestClient(t, server.URL, "provisioning-token", "session-token", server.Client())

	_, err := client.GetSession(context.Background(), "session-secret", "replacement-token-secret")
	require.EqualError(t, err, "ZITADEL session does not contain verified user and OTP SMS factors")
	assertDoesNotContain(t, err.Error(), "user-secret", "org-secret", "session-secret", "replacement-token-secret")
}

func TestClientRejectsNonHTTPSIssuerOutsideLoopback(t *testing.T) {
	t.Parallel()
	_, err := NewClient(ClientConfig{IssuerURL: "http://issuer.example", ProvisioningToken: "provisioning-token", SessionToken: "session-token"})
	require.EqualError(t, err, "ZITADEL issuer URL must use HTTPS outside loopback")
}

func TestClientLimitsProviderResponseBody(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat("x", maxProviderResponseBytes+1)))
	}))
	defer server.Close()
	client := newTestClient(t, server.URL, "provisioning-token", "session-token", server.Client())

	_, err := client.CreateOrganization(context.Background(), "organization-secret")
	require.EqualError(t, err, "create ZITADEL organization: provider response exceeded size limit")
	assertDoesNotContain(t, err.Error(), "organization-secret")
}

func TestClientDeleteSessionAcceptsOnly200Or204(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name    string
		status  int
		wantErr string
	}{
		{name: "ok", status: http.StatusOK},
		{name: "no content", status: http.StatusNoContent},
		{name: "created", status: http.StatusCreated, wantErr: "delete ZITADEL session: ZITADEL returned HTTP status 201"},
		{name: "accepted", status: http.StatusAccepted, wantErr: "delete ZITADEL session: ZITADEL returned HTTP status 202"},
		{name: "partial content", status: http.StatusPartialContent, wantErr: "delete ZITADEL session: ZITADEL returned HTTP status 206"},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(test.status)
			}))
			defer server.Close()
			client := newTestClient(t, server.URL, "provisioning-token", "session-token", server.Client())

			err := client.DeleteSession(context.Background(), "session-preflight")
			if test.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.EqualError(t, err, test.wantErr)
		})
	}
}

func TestClientDeleteOrganizationUsesProvisioningToken(t *testing.T) {
	t.Parallel()
	var authorization string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authorization = r.Header.Get("Authorization")
		require.Equal(t, http.MethodDelete, r.Method)
		require.Equal(t, "/v2/organizations/org-preflight", r.URL.Path)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, "provisioning-token", "session-token", server.Client())
	require.NoError(t, client.DeleteOrganization(context.Background(), "org-preflight"))
	require.Equal(t, "Bearer provisioning-token", authorization)
}

func TestNewClientUsesTenSecondDefaultHTTPTimeout(t *testing.T) {
	t.Parallel()

	client, err := NewClient(ClientConfig{
		IssuerURL: "https://issuer.example", ProvisioningToken: "provisioning-token", SessionToken: "session-token",
	})
	require.NoError(t, err)
	actual, ok := client.(*zitadelClient)
	require.True(t, ok)
	require.NotSame(t, http.DefaultClient, actual.http)
	require.Equal(t, 10*time.Second, actual.http.Timeout)
}

func newTestClient(t *testing.T, issuerURL, provisioningToken, sessionToken string, httpClient *http.Client) Client {
	t.Helper()
	client, err := NewClient(ClientConfig{
		IssuerURL: issuerURL, ProvisioningToken: provisioningToken, SessionToken: sessionToken, HTTPClient: httpClient,
	})
	require.NoError(t, err)
	return client
}

func assertJSONEqual(t *testing.T, want map[string]any, body []byte) {
	t.Helper()
	var got map[string]any
	require.NoError(t, json.Unmarshal(body, &got))
	require.Equal(t, want, got)
}

func assertDoesNotContain(t *testing.T, output string, secrets ...string) {
	t.Helper()
	for _, secret := range secrets {
		require.NotContains(t, output, secret)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}
