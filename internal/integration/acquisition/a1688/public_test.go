package a1688

import (
	"compress/gzip"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"task-processor/internal/product/sourcing"
)

type fixtureTransport struct {
	target *url.URL
	t      *testing.T
}

func (f fixtureTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	f.t.Helper()
	require.Equal(f.t, "detail.1688.com", request.URL.Host)
	require.Empty(f.t, request.Header.Get("Cookie"))
	require.Empty(f.t, request.Header.Get("Authorization"))
	require.Empty(f.t, request.URL.RawQuery)
	forward := request.Clone(request.Context())
	copyURL := *request.URL
	copyURL.Scheme, copyURL.Host = f.target.Scheme, f.target.Host
	forward.URL = &copyURL
	return http.DefaultTransport.RoundTrip(forward)
}

func publicFixtureClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	target, err := url.Parse(server.URL)
	require.NoError(t, err)
	client := New()
	if client.http == nil {
		client.http = &http.Client{}
	}
	client.http.Transport = fixtureTransport{target: target, t: t}
	return client
}

func TestPublicAcquisitionHTTPFixtureProducesExactEvidence(t *testing.T) {
	raw, err := os.ReadFile("testdata/public-product.html")
	require.NoError(t, err)
	client := publicFixtureClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(raw)
	})
	source, err := sourcing.Canonical1688Source("981645030344")
	require.NoError(t, err)
	evidence, err := client.Acquire(context.Background(), source)
	require.NoError(t, err)
	require.Equal(t, "Public fixture bottle", *evidence.Title)
	require.Equal(t, "Stainless steel bottle fixture", *evidence.Description)
	require.Equal(t, "steel", evidence.Attributes[0].Value)
	require.Equal(t, "12345678901234567890", *evidence.Variants[0].SourceID)
	require.Equal(t, "12.50", evidence.Variants[0].Price.Amount)
	require.Nil(t, evidence.Variants[0].Price.Currency, "never infer CNY")
	require.Equal(t, "2", *evidence.PriceFacts[0].MinQuantity)
	require.Len(t, evidence.ContentSHA256, 64)
	envelope, err := sourcing.MapAcquisitionEvidence(source, evidence, "public_http", "fixture-operation")
	require.NoError(t, err)
	require.Len(t, envelope.AssetCandidates, 1)
}

func TestPublicAcquisitionRejectsUntrustedHTTPAndPageShapes(t *testing.T) {
	raw, err := os.ReadFile("testdata/public-product.html")
	require.NoError(t, err)
	for name, handler := range map[string]http.HandlerFunc{
		"challenge": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = io.WriteString(w, "<title>Verify you are human</title>")
		},
		"foreign redirect": func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "https://evil.test/", 302) },
		"different offer redirect": func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "https://detail.1688.com/offer/99.html", 302)
		},
		"bad charset": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=gbk")
			_, _ = w.Write(raw)
		},
		"bad media": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(raw)
		},
		"duplicate key": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = io.WriteString(w, strings.Replace(string(raw), `"offerId":981645030344`, `"offerId":981645030344,"offerId":99`, 1))
		},
		"gzip expansion": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			w.Header().Set("Content-Encoding", "gzip")
			z := gzip.NewWriter(w)
			_, _ = io.WriteString(z, strings.Repeat("a", (2<<20)+1))
			_ = z.Close()
		},
	} {
		t.Run(name, func(t *testing.T) {
			client := publicFixtureClient(t, handler)
			source, _ := sourcing.Canonical1688Source("981645030344")
			_, err := client.Acquire(context.Background(), source)
			require.Error(t, err)
			require.NotContains(t, err.Error(), "window.context")
		})
	}
}
