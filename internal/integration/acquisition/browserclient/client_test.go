package browserclient_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"task-processor/internal/integration/acquisition/browserclient"
	"task-processor/internal/integration/acquisition/browsercollector"
	"task-processor/internal/product/sourcing"
)

func fixedTime() time.Time { return time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC) }

func mustSource(t *testing.T) sourcing.AcquisitionSource {
	t.Helper()
	source, err := sourcing.Canonical1688Source("981645030344")
	require.NoError(t, err)
	return source
}

// A well-formed collector response must arrive as untrusted evidence, and the
// client must attach its admission credential to the request.
func TestClientAcquiresEvidenceAndSendsAdmission(t *testing.T) {
	title := "Fixture bottle"
	var sawPath, sawCredential string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawPath = r.URL.Path
		sawCredential = r.Header.Get("X-Collector-Credential")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"schemaVersion":1,"sourceURL":"https://detail.1688.com/offer/981645030344.html","offerID":"981645030344","title":"` + title + `","capturedAt":"2026-09-26T12:00:00Z","contentSHA256":"abc","parserVersion":"1688-browser-context-v1"}`))
	}))
	defer srv.Close()

	client, err := browserclient.New(browserclient.Options{
		Endpoint: srv.URL,
		Admission: func(r *http.Request) error {
			r.Header.Set("X-Collector-Credential", "service-token")
			return nil
		},
	})
	require.NoError(t, err)
	evidence, err := client.Acquire(context.Background(), mustSource(t))
	require.NoError(t, err)
	require.Equal(t, "981645030344", evidence.OfferID)
	require.Equal(t, title, *evidence.Title)
	require.Equal(t, browsercollector.AcquirePath, sawPath)
	require.Equal(t, "service-token", sawCredential)
}

// A non-canonical source must be rejected locally, before any RPC is made, so a
// malformed request never reaches the browser.
func TestClientRejectsNonCanonicalSourceWithoutCalling(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	client, err := browserclient.New(browserclient.Options{Endpoint: srv.URL})
	require.NoError(t, err)
	_, err = client.Acquire(context.Background(), sourcing.AcquisitionSource{URL: "https://evil.test/offer/1.html", OfferID: "1"})
	require.ErrorIs(t, err, sourcing.ErrInvalidAcquisition)
	require.False(t, called, "must not call the collector for a non-canonical source")
}

// Collector error codes must map back onto the domain sentinels the current
// application already renders, and never onto success.
func TestClientMapsCollectorErrorCodes(t *testing.T) {
	for name, tc := range map[string]struct {
		status int
		body   string
		want   error
	}{
		"invalid request": {http.StatusBadRequest, `{"code":"invalid_request"}`, sourcing.ErrInvalidAcquisition},
		"source failure":  {http.StatusBadGateway, `{"code":"source_unavailable"}`, sourcing.ErrAcquisitionFailed},
		"budget exceeded": {http.StatusGatewayTimeout, `{"code":"budget_exceeded"}`, context.DeadlineExceeded},
		"forbidden":       {http.StatusForbidden, `{"code":"invalid_request"}`, sourcing.ErrAcquisitionUnavailable},
		"unparsable body": {http.StatusBadGateway, `not-json`, sourcing.ErrAcquisitionFailed},
		"unknown code":    {http.StatusBadGateway, `{"code":"future_code"}`, sourcing.ErrAcquisitionFailed},
		"status only 504": {http.StatusGatewayTimeout, ``, context.DeadlineExceeded},
	} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(tc.status)
			_, _ = w.Write([]byte(tc.body))
		}))
		client, err := browserclient.New(browserclient.Options{Endpoint: srv.URL})
		require.NoError(t, err)
		_, gotErr := client.Acquire(context.Background(), mustSource(t))
		require.ErrorIs(t, gotErr, tc.want, name)
		srv.Close()
	}
}

// An oversized collector response must be rejected before it is decoded, so a
// broken or hostile peer cannot force an unbounded allocation.
func TestClientRejectsOversizedResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Pad far beyond the transport cap.
		_, _ = w.Write([]byte(`{"title":"` + strings.Repeat("x", browsercollector.MaxResponseBytes+16) + `"}`))
	}))
	defer srv.Close()
	client, err := browserclient.New(browserclient.Options{Endpoint: srv.URL})
	require.NoError(t, err)
	_, err = client.Acquire(context.Background(), mustSource(t))
	require.ErrorIs(t, err, sourcing.ErrAcquisitionUnavailable)
}

// An unreachable collector is an availability failure, not a caller error and
// not a fabricated success.
func TestClientMapsTransportFailureToUnavailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	endpoint := srv.URL
	srv.Close() // nothing is listening now
	client, err := browserclient.New(browserclient.Options{Endpoint: endpoint})
	require.NoError(t, err)
	_, err = client.Acquire(context.Background(), mustSource(t))
	require.ErrorIs(t, err, sourcing.ErrAcquisitionUnavailable)
}

// New must reject a missing endpoint rather than build an unusable client.
func TestNewRequiresEndpoint(t *testing.T) {
	_, err := browserclient.New(browserclient.Options{})
	require.ErrorIs(t, err, sourcing.ErrAcquisitionUnavailable)
}

// End-to-end over the real contract: the client and the collector handler must
// interoperate, and the result must map through the current owner.
func TestClientAndCollectorRoundTrip(t *testing.T) {
	provider := &evidenceProvider{}
	handler, err := browsercollector.Handler(browsercollector.Options{
		Provider: provider,
		Admit:    func(r *http.Request) error { return nil },
	})
	require.NoError(t, err)
	srv := httptest.NewServer(handler)
	defer srv.Close()

	client, err := browserclient.New(browserclient.Options{Endpoint: srv.URL})
	require.NoError(t, err)
	source := mustSource(t)
	evidence, err := client.Acquire(context.Background(), source)
	require.NoError(t, err)
	require.Equal(t, source.OfferID, evidence.OfferID)

	// The evidence remains untrusted until the current owner maps it.
	envelope, err := sourcing.MapAcquisitionEvidence(source, evidence, sourcing.AcquisitionChannelPublicBrowser, "op-collector-rt")
	require.NoError(t, err)
	key, _, err := sourcing.PublicationIdentity(envelope)
	require.NoError(t, err)
	require.Equal(t, "crawler:1688:981645030344", key)
}

type evidenceProvider struct{}

func (evidenceProvider) Acquire(_ context.Context, source sourcing.AcquisitionSource) (sourcing.AcquisitionEvidence, error) {
	title := "Fixture bottle"
	amount := "12.50"
	return sourcing.AcquisitionEvidence{
		SchemaVersion: 1,
		SourceURL:     source.URL,
		OfferID:       source.OfferID,
		Title:         &title,
		PriceFacts:    []sourcing.AcquisitionPrice{{Amount: amount}},
		CapturedAt:    fixedTime(),
		ContentSHA256: strings.Repeat("ab", 32),
		ParserVersion: "1688-browser-context-v1",
	}, nil
}
