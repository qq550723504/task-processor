package membership

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/require"
	"net/http"
	"strings"
	"testing"
)

func TestSummaryUsesProviderTotalsWithScopedFilters(t *testing.T) {
	server := authorizedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Filters []map[string]any `json:"filters"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		raw, _ := json.Marshal(body.Filters)
		require.Contains(t, string(raw), `"organizationId":{"id":"effective-b"}`)
		require.Contains(t, string(raw), `"projectId":{"id":"project"}`)
		total := 250
		response := validResponse
		if strings.Contains(string(raw), `"state":{"state":"STATE_ACTIVE"}`) {
			total = 245
		}
		if strings.Contains(string(raw), `"state":{"state":"STATE_INACTIVE"}`) {
			total = 5
			response = strings.Replace(response, "STATE_ACTIVE", "STATE_INACTIVE", 1)
		}
		if strings.Contains(string(raw), `"roleKey":{"key":"listingkit_admin"}`) {
			total = 3
			response = strings.Replace(response, "listingkit_viewer", "listingkit_admin", 1)
		}
		response = strings.Replace(response, `"totalResult":"1"`, fmt.Sprintf(`"totalResult":"%d"`, total), 1)
		_, _ = w.Write([]byte(response))
	}))
	defer server.Close()
	client, err := NewClient(server.URL, "synthetic-read-token", "project", server.Client())
	require.NoError(t, err)
	summary, err := client.Counts(context.Background(), "effective-b")
	require.NoError(t, err)
	require.Equal(t, 250, summary.Total)
	require.Equal(t, 245, summary.Active)
	require.Equal(t, 3, summary.Administrators)
	require.Equal(t, 5, summary.Inactive)
}
