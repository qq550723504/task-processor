package browsercollector_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"task-processor/internal/integration/acquisition/browsercollector"
	"task-processor/internal/product/sourcing"
)

type stubProvider struct {
	calls   int
	ev      sourcing.AcquisitionEvidence
	err     error
	lastSrc sourcing.AcquisitionSource
}

func (s *stubProvider) Acquire(_ context.Context, source sourcing.AcquisitionSource) (sourcing.AcquisitionEvidence, error) {
	s.calls++
	s.lastSrc = source
	return s.ev, s.err
}

func evidenceFor(offerID, title string) sourcing.AcquisitionEvidence {
	t := title
	return sourcing.AcquisitionEvidence{
		SchemaVersion: 1,
		SourceURL:     "https://detail.1688.com/offer/" + offerID + ".html",
		OfferID:       offerID,
		Title:         &t,
		CapturedAt:    testNow(),
	}
}

func testNow() time.Time { return time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC) }

func post(t *testing.T, url, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, url+browsercollector.AcquirePath, strings.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	return resp
}

// The collector must refuse to construct without an admission control, so a
// deployment cannot come up with caller admission silently disabled (D13/#14).
func TestHandlerRequiresAdmissionAndProvider(t *testing.T) {
	_, err := browsercollector.Handler(browsercollector.Options{Provider: &stubProvider{}})
	require.ErrorIs(t, err, sourcing.ErrAcquisitionUnavailable, "admission must be required")
	_, err = browsercollector.Handler(browsercollector.Options{Admit: func(*http.Request) error { return nil }})
	require.ErrorIs(t, err, sourcing.ErrAcquisitionUnavailable, "provider must be required")
}

// An unauthorized caller must be rejected before the provider runs, so it can
// never spend a browser slot or shared IP budget.
func TestHandlerRejectsUnauthorizedCallerBeforeProvider(t *testing.T) {
	provider := &stubProvider{ev: evidenceFor("981645030344", "ok")}
	handler, err := browsercollector.Handler(browsercollector.Options{
		Provider: provider,
		Admit:    func(*http.Request) error { return errors.New("unauthorized") },
	})
	require.NoError(t, err)
	srv := httptest.NewServer(handler)
	defer srv.Close()

	resp := post(t, srv.URL, `{"sourceURL":"https://detail.1688.com/offer/981645030344.html"}`)
	require.Equal(t, http.StatusForbidden, resp.StatusCode)
	require.Equal(t, 0, provider.calls, "provider must not run for an unauthorized caller")
}

// A canonical public source must round-trip bounded evidence, and the collector
// must not require or receive any tenant identity.
func TestHandlerReturnsBoundedEvidenceForCanonicalSource(t *testing.T) {
	provider := &stubProvider{ev: evidenceFor("981645030344", "Fixture bottle")}
	handler, err := browsercollector.Handler(browsercollector.Options{
		Provider: provider,
		Admit:    func(*http.Request) error { return nil },
	})
	require.NoError(t, err)
	srv := httptest.NewServer(handler)
	defer srv.Close()

	resp := post(t, srv.URL, `{"sourceURL":"https://detail.1688.com/offer/981645030344.html"}`)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var evidence sourcing.AcquisitionEvidence
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&evidence))
	require.Equal(t, "981645030344", evidence.OfferID)
	require.Equal(t, "Fixture bottle", *evidence.Title)
	require.Equal(t, 1, provider.calls)
	require.Equal(t, "981645030344", provider.lastSrc.OfferID)
}

// A challenge or unsupported page is a source failure, never dressed up as a
// caller error and never as success (D4).
func TestHandlerMapsProviderFailureToSourceUnavailable(t *testing.T) {
	provider := &stubProvider{err: errors.New("challenge detected")}
	handler, err := browsercollector.Handler(browsercollector.Options{
		Provider: provider,
		Admit:    func(*http.Request) error { return nil },
	})
	require.NoError(t, err)
	srv := httptest.NewServer(handler)
	defer srv.Close()

	resp := post(t, srv.URL, `{"sourceURL":"https://detail.1688.com/offer/981645030344.html"}`)
	defer resp.Body.Close()
	require.Equal(t, http.StatusBadGateway, resp.StatusCode)
	var body browsercollector.ErrorBody
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	require.Equal(t, browsercollector.CodeSourceUnavailable, body.Code)
	require.ErrorIs(t, browsercollector.SentinelFor(body.Code), sourcing.ErrAcquisitionFailed)
}

// A provider deadline must be reported as a budget failure, not a source failure.
func TestHandlerMapsProviderDeadlineToBudgetExceeded(t *testing.T) {
	provider := &stubProvider{err: context.DeadlineExceeded}
	handler, err := browsercollector.Handler(browsercollector.Options{
		Provider: provider,
		Admit:    func(*http.Request) error { return nil },
	})
	require.NoError(t, err)
	srv := httptest.NewServer(handler)
	defer srv.Close()

	resp := post(t, srv.URL, `{"sourceURL":"https://detail.1688.com/offer/981645030344.html"}`)
	defer resp.Body.Close()
	require.Equal(t, http.StatusGatewayTimeout, resp.StatusCode)
	var body browsercollector.ErrorBody
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	require.ErrorIs(t, browsercollector.SentinelFor(body.Code), context.DeadlineExceeded)
}

// Only the canonical public source is accepted; anything else is a caller error
// and must never reach the browser.
func TestHandlerRejectsNonCanonicalSources(t *testing.T) {
	for name, body := range map[string]string{
		"other host":  `{"sourceURL":"https://evil.test/offer/981645030344.html"}`,
		"unknown fld": `{"sourceURL":"https://detail.1688.com/offer/981645030344.html","organizationId":"org-1"}`,
		"not json":    `not-json`,
		"empty":       `{}`,
	} {
		provider := &stubProvider{ev: evidenceFor("981645030344", "ok")}
		handler, err := browsercollector.Handler(browsercollector.Options{
			Provider: provider,
			Admit:    func(*http.Request) error { return nil },
		})
		require.NoError(t, err)
		srv := httptest.NewServer(handler)
		resp := post(t, srv.URL, body)
		require.Equal(t, http.StatusBadRequest, resp.StatusCode, name)
		require.Equal(t, 0, provider.calls, name+": provider must not run")
		resp.Body.Close()
		srv.Close()
	}
}

// Only the single acquire method is served.
func TestHandlerRejectsOtherMethods(t *testing.T) {
	handler, err := browsercollector.Handler(browsercollector.Options{
		Provider: &stubProvider{},
		Admit:    func(*http.Request) error { return nil },
	})
	require.NoError(t, err)
	srv := httptest.NewServer(handler)
	defer srv.Close()
	resp, err := http.Get(srv.URL + browsercollector.AcquirePath)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode)
}

// The handler is installed as the whole server handler, so it must reject any
// path other than the single versioned RPC path before doing provider work.
func TestHandlerRejectsOtherPathsBeforeProviderWork(t *testing.T) {
	for _, path := range []string{"/", "/internal/v0/browser-collector/acquire", "/internal/v1/browser-collector/other", "/metrics"} {
		provider := &stubProvider{ev: evidenceFor("981645030344", "ok")}
		handler, err := browsercollector.Handler(browsercollector.Options{
			Provider: provider,
			Admit:    func(*http.Request) error { return nil },
		})
		require.NoError(t, err)
		srv := httptest.NewServer(handler)
		resp, err := http.Post(srv.URL+path, "application/json", strings.NewReader(`{"sourceURL":"https://detail.1688.com/offer/981645030344.html"}`))
		require.NoError(t, err)
		require.Equal(t, http.StatusNotFound, resp.StatusCode, path)
		require.Equal(t, 0, provider.calls, path+": provider must not run")
		resp.Body.Close()
		srv.Close()
	}
}

// An evidence body that would exceed the transport cap must be refused rather
// than streamed, because no conforming client could accept it.
func TestHandlerRefusesOversizedEvidence(t *testing.T) {
	huge := strings.Repeat("x", browsercollector.MaxResponseBytes+64)
	title := huge
	provider := &stubProvider{ev: evidenceFor("981645030344", title)}
	handler, err := browsercollector.Handler(browsercollector.Options{
		Provider: provider,
		Admit:    func(*http.Request) error { return nil },
	})
	require.NoError(t, err)
	srv := httptest.NewServer(handler)
	defer srv.Close()
	resp := post(t, srv.URL, `{"sourceURL":"https://detail.1688.com/offer/981645030344.html"}`)
	defer resp.Body.Close()
	require.Equal(t, http.StatusBadGateway, resp.StatusCode)
}
