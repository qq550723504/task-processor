package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewRejectsNonLoopbackHTTP(t *testing.T) {
	_, err := New("http://api.example.test", "token", nil)
	require.ErrorContains(t, err, "HTTPS")
}

func TestClientClaimSendsBearerAndMapsNoJob(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer token", r.Header.Get("Authorization"))
		require.Equal(t, "/api/v1/local-agent/1688-jobs/claim", r.URL.Path)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	client, err := New(server.URL, "token", server.Client())
	require.NoError(t, err)
	claim, err := client.Claim(context.Background())
	require.NoError(t, err)
	require.Nil(t, claim)
}

func TestClientRejectsRedirects(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://elsewhere.example", http.StatusFound)
	}))
	defer server.Close()
	client, err := New(server.URL, "token", server.Client())
	require.NoError(t, err)
	_, err = client.Claim(context.Background())
	require.Error(t, err)
	require.NotContains(t, strings.ToLower(err.Error()), "elsewhere")
}
