package browsercollector_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"task-processor/internal/integration/acquisition/browsercollector"
)

const testSecret = "svc-secret-2f7c1d9a4b8e"

// A correct service credential is admitted; a wrong, absent, or empty-configured
// credential is denied. This is the resolved A1 control: without it, any
// workload on the same network could drive Chromium (design D13 / finding #14).
func TestSharedSecretAdmission(t *testing.T) {
	admit := browsercollector.SharedSecretAdmission(testSecret)
	req := func(secret string) *http.Request {
		r := httptest.NewRequest(http.MethodPost, "http://collector/acquire", strings.NewReader("{}"))
		if secret != "" {
			r.Header.Set(browsercollector.CredentialHeader, secret)
		}
		return r
	}
	require.NoError(t, admit(req(testSecret)), "correct credential must be admitted")
	require.ErrorIs(t, admit(req("wrong")), browsercollector.ErrAdmissionDenied)
	require.ErrorIs(t, admit(req(testSecret+"x")), browsercollector.ErrAdmissionDenied, "a prefix must not be admitted")
	require.ErrorIs(t, admit(req("")), browsercollector.ErrAdmissionDenied, "absent credential must be denied")

	// An unconfigured collector must fail closed rather than accept everyone.
	openAdmit := browsercollector.SharedSecretAdmission("")
	require.ErrorIs(t, openAdmit(req(testSecret)), browsercollector.ErrAdmissionDenied)
}

// The client-side attacher must fail closed when no secret is configured, so no
// unauthenticated request is ever sent.
func TestAttachSharedSecretFailsClosed(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "http://collector/acquire", nil)
	require.NoError(t, browsercollector.AttachSharedSecret(testSecret)(r))
	require.Equal(t, testSecret, r.Header.Get(browsercollector.CredentialHeader))

	r2 := httptest.NewRequest(http.MethodPost, "http://collector/acquire", nil)
	require.ErrorIs(t, browsercollector.AttachSharedSecret("  ")(r2), browsercollector.ErrAdmissionDenied)
	require.Empty(t, r2.Header.Get(browsercollector.CredentialHeader))
}

// End-to-end admission over the real handler: an unauthenticated caller is
// rejected and the provider never runs.
func TestAdmissionProtectsTheHandlerEndToEnd(t *testing.T) {
	provider := &stubProvider{ev: evidenceFor("981645030344", "ok")}
	handler, err := browsercollector.Handler(browsercollector.Options{
		Provider: provider,
		Admit:    browsercollector.SharedSecretAdmission(testSecret),
	})
	require.NoError(t, err)
	srv := httptest.NewServer(handler)
	defer srv.Close()

	// No credential.
	resp := post(t, srv.URL, `{"sourceURL":"https://detail.1688.com/offer/981645030344.html"}`)
	require.Equal(t, http.StatusForbidden, resp.StatusCode)
	resp.Body.Close()
	require.Equal(t, 0, provider.calls)

	// Correct credential.
	r, err := http.NewRequest(http.MethodPost, srv.URL+browsercollector.AcquirePath,
		strings.NewReader(`{"sourceURL":"https://detail.1688.com/offer/981645030344.html"}`))
	require.NoError(t, err)
	require.NoError(t, browsercollector.AttachSharedSecret(testSecret)(r))
	okResp, err := http.DefaultClient.Do(r)
	require.NoError(t, err)
	defer okResp.Body.Close()
	require.Equal(t, http.StatusOK, okResp.StatusCode)
	require.Equal(t, 1, provider.calls)
}

// The secret must come from the process environment and never be empty.
func TestSharedSecretFromEnv(t *testing.T) {
	secret, err := browsercollector.SharedSecretFromEnv(func(k string) string {
		if k == "COLLECTOR_SECRET" {
			return testSecret
		}
		return ""
	}, "COLLECTOR_SECRET")
	require.NoError(t, err)
	require.Equal(t, testSecret, secret)

	_, err = browsercollector.SharedSecretFromEnv(func(string) string { return "   " }, "COLLECTOR_SECRET")
	require.ErrorIs(t, err, browsercollector.ErrAdmissionDenied)

	_, err = browsercollector.SharedSecretFromEnv(nil, "DEFINITELY_UNSET_SECRET_NAME")
	require.ErrorIs(t, err, browsercollector.ErrAdmissionDenied)
}
