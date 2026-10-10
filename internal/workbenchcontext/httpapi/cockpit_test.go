package httpapi

import (
	"encoding/json"
	"github.com/stretchr/testify/require"
	"net/http"
	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	"testing"
)

func TestCockpitAvailabilityRequiresMountedModuleAndSelectedOrganization(t *testing.T) {
	h := NewHandlerWithWorkbenchAuthorizer(authz.DefaultListingKitAuthorizer())
	id := authidentity.AuthenticatedIdentity{UserID: "actor", HomeOrganizationID: "org-a", EffectiveOrganizationID: "org-a", OrganizationGrants: []authidentity.OrganizationGrant{{OrganizationID: "org-a", OrganizationName: "A", Roles: []string{"listingkit_admin"}}}}
	read := func() map[string]any {
		w := serveHandler(t, http.MethodGet, "/api/v1/workbench/context", "", id, h.GetContext)
		require.Equal(t, 200, w.Code)
		var body map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
		return body
	}
	require.NotContains(t, read(), "operationsCockpitAvailable")
	h.SetOperationsCockpitAvailable(true)
	require.Equal(t, true, read()["operationsCockpitAvailable"])
	id.EffectiveOrganizationID = ""
	id.OrganizationGrants = nil
	require.NotContains(t, read(), "operationsCockpitAvailable")
}
