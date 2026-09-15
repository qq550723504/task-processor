//go:build integration

package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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

// Only external OIDC/UserInfo/grant HTTP and Auth.js session issuance are
// synthetic. NewCurrentApplication, BFF, registry writes/history and PG are real.
func TestAccountAuditBrowserFixture(t *testing.T) {
	owner, connection := commercialPostgres(t) // Newly created task-owned PostgreSQL.
	ctx := context.Background()
	require.NoError(t, listingsubscription.AutoMigrateRepository(owner))
	require.NoError(t, schema.Migrate(ctx, owner))
	auth, _ := authz.NewListingKitAuthorizer(nil, nil)
	repo, err := store.NewRepository(ctx, owner)
	require.NoError(t, err)
	now := time.Now().UTC().Truncate(time.Microsecond)
	service, err := registry.NewService(repo, auth, registry.WithClock(func() time.Time { return now }))
	require.NoError(t, err)
	scoped := authidentity.WithAuthenticatedIdentity(ctx, authidentity.AuthenticatedIdentity{UserID: "u1", TenantID: "B", EffectiveOrganizationID: "B", Roles: []string{"listingkit_operator"}, TokenExpiresAt: now.Add(time.Hour)})
	for i := 0; i < 25; i++ {
		_, err = service.Register(scoped, uuid.NewString(), registry.RegisterInput{DisplayName: fmt.Sprintf("owned fixture %d", i), Platform: "1688"})
		require.NoError(t, err)
	}
	require.NoError(t, owner.Exec("REVOKE CREATE ON SCHEMA public FROM PUBLIC").Error)
	require.NoError(t, owner.Exec("REVOKE ALL ON ALL TABLES IN SCHEMA public FROM PUBLIC").Error)
	for _, role := range []string{"source_account_runtime", "commercial_reader"} {
		require.NoError(t, owner.Exec(`CREATE ROLE `+role+` LOGIN PASSWORD 'synthetic-audit-password'; GRANT CONNECT ON DATABASE issue347 TO `+role+`; GRANT USAGE ON SCHEMA public TO `+role).Error)
		for table, privileges := range run1AllowedPrivileges[role] {
			require.NoError(t, owner.Exec("GRANT "+strings.Join(privileges, ",")+" ON public."+table+" TO "+role).Error)
		}
	}
	require.NoError(t, owner.Exec("ALTER ROLE commercial_reader SET default_transaction_read_only=on").Error)
	source := openSourceAccountApplicationDB(t, fmt.Sprintf("host=127.0.0.1 port=%d dbname=issue347 user=source_account_runtime password=synthetic-audit-password sslmode=disable", connection.Port))
	commercial := openSourceAccountApplicationDB(t, fmt.Sprintf("host=127.0.0.1 port=%d dbname=issue347 user=commercial_reader password=synthetic-audit-password sslmode=disable", connection.Port))
	external := newAccountFixture(t)
	cfg := &config.Config{Workbench: config.WorkbenchConfig{Enabled: true}, ListingKit: config.ListingKitConfig{Zitadel: config.ListingKitZitadelConfig{IssuerURL: external.provider.URL, ClientID: "fixture-client", ClientSecret: "fixture-secret", ProjectID: "project", AuthorizationAPIURL: external.provider.URL}}}
	log := logrus.New()
	log.SetOutput(io.Discard)
	server, err := NewCurrentApplication(ctx, source, commercial, cfg, log)
	require.NoError(t, err)
	running := httptest.NewServer(server.Handler)
	defer running.Close()
	tables := []string{"source_account_resources", "source_account_operations"}
	before := run1PermissionFacts(t, owner, tables)
	get := func(query string) (int, accountaudit.Page) {
		r, _ := http.NewRequest("GET", running.URL+accountAuditPath+query, nil)
		r.Header.Set("Authorization", "Bearer u1")
		r.Header.Set("X-Requested-Organization-ID", "B")
		response, err := http.DefaultClient.Do(r)
		require.NoError(t, err)
		defer response.Body.Close()
		var page accountaudit.Page
		if response.StatusCode == 200 {
			require.NoError(t, json.NewDecoder(response.Body).Decode(&page))
		}
		return response.StatusCode, page
	}
	status, page := get("?limit=20")
	require.Equal(t, 200, status)
	require.Len(t, page.Items, 20)
	require.NotNil(t, page.NextCursor)
	status, last := get("?limit=20&cursor=" + *page.NextCursor)
	require.Equal(t, 200, status)
	require.Len(t, last.Items, 5)
	require.Nil(t, last.NextCursor)
	require.Equal(t, before, run1PermissionFacts(t, owner, tables))
	dir := os.Getenv("ISSUE412_FIXTURE_DIR")
	if dir == "" {
		return
	}
	require.True(t, filepath.IsAbs(dir))
	control := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			w.WriteHeader(405)
			return
		}
		switch r.URL.Path {
		case "/revoke":
			external.revoked.Store(true)
		case "/restore":
			external.revoked.Store(false)
			external.unavailable.Store(false)
		case "/unavailable":
			external.unavailable.Store(true)
		default:
			w.WriteHeader(404)
			return
		}
		w.WriteHeader(204)
	}))
	defer control.Close()
	seed, _ := json.Marshal(map[string]any{"goOrigin": running.URL, "issuerURL": external.provider.URL, "controlOrigin": control.URL, "tokens": []string{"u1", "expired", "grant-down", "no-org"}, "fixtureRecords": 25})
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.json"), seed, 0600))
	timer := time.NewTimer(29 * time.Minute)
	defer timer.Stop()
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-timer.C:
			t.Fatal("audit fixture expired")
		case <-ticker.C:
			if _, err = os.Stat(filepath.Join(dir, "stop-go")); err == nil {
				require.Equal(t, before, run1PermissionFacts(t, owner, tables))
				t.Log("real default application audit reads left source facts unchanged")
				return
			}
		}
	}
}
