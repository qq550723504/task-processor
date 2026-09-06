package zitadel

import (
	"context"
	"encoding/json"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestServiceAuthorizationRequiresExactOrganization(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		require.Equal(t, "Bearer synthetic-service", r.Header.Get("Authorization"))
		var request capturedAuthorizationListRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
		require.Len(t, request.Filters, 3)
		require.JSONEq(t, `{"id":"org-b"}`, string(request.Filters[2]["organizationId"]))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"pagination":{"totalResult":"0"},"authorizations":[]}`))
	}))
	defer server.Close()
	client := NewAuthorizationClient(server.URL, server.Client())
	_, err := client.ListServiceProjectAuthorizations(context.Background(), "synthetic-service", "actor", "project", "")
	require.Error(t, err)
	require.Zero(t, calls)
	grants, err := client.ListServiceProjectAuthorizations(context.Background(), "synthetic-service", "actor", "project", "org-b")
	require.NoError(t, err)
	require.Empty(t, grants)
	require.Equal(t, 1, calls)
}
