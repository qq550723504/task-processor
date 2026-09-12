//go:build integration

package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"task-processor/internal/app/accountaudit"
	schema "task-processor/internal/app/schema/sourceaccountregistry"
	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	"task-processor/internal/core/config"
	store "task-processor/internal/integration/persistence/sourceaccountregistry"
	"task-processor/internal/listingsubscription"
	registry "task-processor/internal/sourceaccountregistry"
)

// VERIFICATION_ONLY combination of #411 and frozen #417. Only external identity
// responses are fixtures; the default application, HistoryService and PG are real.
func TestIssue411ConfiguredUserAccountAuditPostgres(t *testing.T) {
	owner, connection := commercialPostgres(t)
	ctx := context.Background()
	require.NoError(t, listingsubscription.AutoMigrateRepository(owner))
	require.NoError(t, schema.Migrate(ctx, owner))
	auth, err := authz.NewListingKitAuthorizer(nil, nil)
	require.NoError(t, err)
	repo, err := store.NewRepository(ctx, owner)
	require.NoError(t, err)
	sourceService, err := registry.NewService(repo, auth)
	require.NoError(t, err)
	ids := map[string]string{}
	for _, org := range []string{"A", "B", "C"} {
		identity := authidentity.AuthenticatedIdentity{UserID: "different-receipt-author", TenantID: org, EffectiveOrganizationID: org, Roles: []string{"listingkit_operator"}, TokenExpiresAt: time.Now().Add(time.Hour)}
		created, err := sourceService.Register(authidentity.WithAuthenticatedIdentity(ctx, identity), uuid.NewString(), registry.RegisterInput{DisplayName: "Owned " + org, Platform: "1688"})
		require.NoError(t, err)
		ids[org] = created.Account.ID
	}
	// Exercise the same minimal database roles as the default current application.
	require.NoError(t, owner.Exec("REVOKE CREATE ON SCHEMA public FROM PUBLIC").Error)
	require.NoError(t, owner.Exec("REVOKE ALL ON ALL TABLES IN SCHEMA public FROM PUBLIC").Error)
	for _, role := range []string{"source_account_runtime", "commercial_reader"} {
		require.NoError(t, owner.Exec(`CREATE ROLE `+role+` LOGIN PASSWORD 'owned-combination'; GRANT CONNECT ON DATABASE issue347 TO `+role+`; GRANT USAGE ON SCHEMA public TO `+role).Error)
		for table, privileges := range run1AllowedPrivileges[role] {
			require.NoError(t, owner.Exec("GRANT "+strings.Join(privileges, ",")+" ON public."+table+" TO "+role).Error)
		}
	}
	require.NoError(t, owner.Exec("ALTER ROLE commercial_reader SET default_transaction_read_only=on").Error)
	source := openSourceAccountApplicationDB(t, fmt.Sprintf("host=127.0.0.1 port=%d dbname=issue347 user=source_account_runtime password=owned-combination sslmode=disable", connection.Port))
	commercial := openSourceAccountApplicationDB(t, fmt.Sprintf("host=127.0.0.1 port=%d dbname=issue347 user=commercial_reader password=owned-combination sslmode=disable", connection.Port))
	external := newAccountFixture(t)
	// Reuse the external fixture's exact subject/project filter assertions, but
	// give this test only an unprivileged Organization role. Do not alter the
	// shared fixture's defaults or inject a synthetic platform_admin role.
	grants := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/zitadel.authorization.v2.AuthorizationService/ListAuthorizations", r.URL.Path)
		recorded := httptest.NewRecorder()
		external.provider.Config.Handler.ServeHTTP(recorded, r)
		if recorded.Code != http.StatusOK {
			w.WriteHeader(recorded.Code)
			_, _ = w.Write(recorded.Body.Bytes())
			return
		}
		var response map[string]any
		require.NoError(t, json.Unmarshal(recorded.Body.Bytes(), &response))
		for _, row := range response["authorizations"].([]any) {
			row.(map[string]any)["roles"] = []any{map[string]string{"key": "custom_member"}}
		}
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(response))
	}))
	defer grants.Close()
	start := func(users []string) *httptest.Server {
		cfg := &config.Config{Workbench: config.WorkbenchConfig{Enabled: true}, ListingKit: config.ListingKitConfig{PlatformAdminUsers: users, Zitadel: config.ListingKitZitadelConfig{IssuerURL: external.provider.URL, ClientID: "fixture-client", ClientSecret: "fixture-secret", ProjectID: "project", AuthorizationAPIURL: grants.URL}}}
		log := logrus.New()
		log.SetOutput(io.Discard)
		app, err := NewCurrentApplication(ctx, source, commercial, cfg, log)
		require.NoError(t, err)
		return httptest.NewServer(app.Handler)
	}
	running := start([]string{"u1", "no-org"})
	t.Cleanup(running.Close)
	tables := []string{"source_account_resources", "source_account_operations"}
	before := run1PermissionFacts(t, owner, tables)
	get := func(token, org string) (int, []byte) {
		r, err := http.NewRequest(http.MethodGet, running.URL+accountAuditPath+"?limit=20", nil)
		require.NoError(t, err)
		r.Header.Set("Authorization", "Bearer "+token)
		r.Header.Set("X-Requested-Organization-ID", org)
		r.Header.Set("X-User-ID", "forged-user")
		r.Header.Set("X-User-Roles", "platform_admin")
		response, err := http.DefaultClient.Do(r)
		require.NoError(t, err)
		defer response.Body.Close()
		body, err := io.ReadAll(response.Body)
		require.NoError(t, err)
		require.Equal(t, before, run1PermissionFacts(t, owner, tables))
		return response.StatusCode, body
	}
	for _, org := range []string{"B", "C"} {
		status, body := get("u1", org)
		require.Equal(t, 200, status, string(body))
		var page accountaudit.Page
		require.NoError(t, json.Unmarshal(body, &page))
		require.Len(t, page.Items, 1)
		require.Contains(t, string(body), ids[org])
		require.Contains(t, string(body), "different-receipt-author")
		require.NotContains(t, string(body), ids["A"])
		other := "B"
		if org == "B" {
			other = "C"
		}
		require.NotContains(t, string(body), ids[other])
	}
	for _, test := range []struct {
		token, org string
		status     int
		code       string
	}{
		{"u1", "A", 403, "ORGANIZATION_ACCESS_DENIED"},
		{"no-org", "B", 403, "ORGANIZATION_ACCESS_REVOKED"},
		{"expired", "B", 401, "AUTHENTICATION_REQUIRED"},
	} {
		status, body := get(test.token, test.org)
		assertHTTPCode(t, status, body, test.status, test.code)
	}
	external.revoked.Store(true)
	status, body := get("u1", "B")
	assertHTTPCode(t, status, body, 403, "ORGANIZATION_ACCESS_REVOKED")
	external.revoked.Store(false)
	external.unavailable.Store(true)
	status, body = get("u1", "B")
	assertHTTPCode(t, status, body, 503, "DEPENDENCY_UNAVAILABLE")
	external.unavailable.Store(false)
	running.Close()
	running = start(nil) // Actual new authorizer after configured-user removal.
	defer running.Close()
	status, body = get("u1", "B")
	assertHTTPCode(t, status, body, 403, "PERMISSION_DENIED")
	require.Equal(t, before, run1PermissionFacts(t, owner, tables))
	t.Log("configured subject B/C scoped committed history PASS; all reads left source tables unchanged")
}
