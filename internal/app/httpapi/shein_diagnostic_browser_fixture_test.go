package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	kernelmodule "task-processor/internal/kernel/module"
	"task-processor/internal/listing/record"
	"task-processor/internal/workbenchcontext"
	contextapi "task-processor/internal/workbenchcontext/httpapi"
)

// External identity/grant substitutes exist only in this test binary. Tokens
// are generated per run; no production flag, handler or default role is added.
type browserFixtureActor struct {
	Subject string
	Role    string
	Revoked bool
}
type browserFixtureIdentity struct {
	actors map[string]browserFixtureActor
}

func (f browserFixtureIdentity) Verify(_ context.Context, token string) (authidentity.AuthenticatedIdentity, error) {
	a, ok := f.actors[token]
	if !ok {
		return authidentity.AuthenticatedIdentity{}, errors.New("unknown fixture token")
	}
	return authidentity.AuthenticatedIdentity{UserID: a.Subject, HomeOrganizationID: "200", TokenExpiresAt: time.Now().Add(time.Hour)}, nil
}

func (f browserFixtureIdentity) Load(ctx context.Context, source workbenchcontext.GrantSource, request workbenchcontext.GrantRequest) (workbenchcontext.GrantResult, error) {
	if request.Subject == "slow" {
		<-ctx.Done()
		return workbenchcontext.GrantResult{}, ctx.Err()
	}
	if request.Subject == "unavailable" {
		return workbenchcontext.GrantResult{}, errors.New("synthetic external grant outage")
	}
	// The authenticated subject selects only predeclared synthetic grants.
	var grants []authidentity.OrganizationGrant
	for _, a := range f.actors {
		if a.Subject == request.Subject && !a.Revoked {
			grants = []authidentity.OrganizationGrant{
				{OrganizationID: "200", OrganizationName: "Fixture A", ProjectID: "project", Roles: []string{a.Role}},
				{OrganizationID: "100", OrganizationName: "Fixture B", ProjectID: "project", Roles: []string{a.Role}},
			}
			break
		}
	}
	return workbenchcontext.GrantResult{Source: source, Grants: grants}, nil
}
func (browserFixtureIdentity) Invalidate(string, string) {}

// TestSheinDiagnosticBrowserFixture is a bounded standalone fixture, not a
// normal suite dependency. The launcher owns a fresh Docker PG container and
// supplies its explicit loopback DSN and an exclusive temporary control dir.
func TestSheinDiagnosticBrowserFixture(t *testing.T) {
	dir, dsn := os.Getenv("ISSUE323_FIXTURE_DIR"), os.Getenv("ISSUE323_FIXTURE_DSN")
	if dir == "" && dsn == "" {
		t.Skip("standalone fixture: use web/listingkit-ui/scripts/shein-diagnostic-fixture.mjs")
	}
	require.NotEmpty(t, dir)
	require.True(t, strings.HasPrefix(dsn, "host=127.0.0.1 ") && strings.Contains(dsn, " dbname=issue323_fixture "), "only explicit task-isolated loopback PostgreSQL")
	t.Setenv("ISSUE319_TEST_DSN", dsn)
	db := recordTestDB(t)
	publishRecordProduct(t, db, "200", "product")
	f := browserFixtureIdentity{actors: map[string]browserFixtureActor{}}
	tokens := map[string]string{}
	for name, actor := range map[string]browserFixtureActor{
		"owner":       {Subject: "owner", Role: "listingkit_operator"},
		"other":       {Subject: "other", Role: "listingkit_operator"},
		"admin":       {Subject: "admin", Role: "listingkit_admin"},
		"readonly":    {Subject: "readonly", Role: "admin"},
		"store":       {Subject: "store", Role: "store_viewer"},
		"revoked":     {Subject: "revoked", Role: "listingkit_admin", Revoked: true},
		"slow":        {Subject: "slow", Role: "listingkit_operator"},
		"unavailable": {Subject: "unavailable", Role: "listingkit_operator"},
	} {
		token := uuid.NewString()
		tokens[name], f.actors[token] = token, actor
	}
	authorizer, err := authz.NewListingKitAuthorizer(nil, nil)
	require.NoError(t, err)
	resolver := workbenchcontext.NewResolver(f, "project", "v1", nil)
	application := func() *http.Server {
		app, _, e := NewSheinRecordApplication(db, f, resolver, authorizer)
		require.NoError(t, e)
		return app
	}
	first := application()
	var current atomic.Pointer[http.Server]
	current.Store(first)
	ts := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { current.Load().Handler.ServeHTTP(w, r) }))
	ts.Config.ReadTimeout, ts.Config.WriteTimeout = first.ReadTimeout, first.WriteTimeout
	ts.Start()
	t.Cleanup(ts.Close)
	status, wire := recordPost(t, ts, tokens["owner"], "fixture-post", recordBody)
	require.Equal(t, 201, status, string(wire))
	var receipt record.Receipt
	require.NoError(t, json.Unmarshal(wire, &receipt))
	ownerRecordIDs := []string{receipt.RecordID}
	for i := 1; i < 22; i++ {
		status, wire = recordPost(t, ts, tokens["owner"], fmt.Sprintf("fixture-owner-%02d", i), recordBody)
		require.Equal(t, 201, status, string(wire))
		var created record.Receipt
		require.NoError(t, json.Unmarshal(wire, &created))
		ownerRecordIDs = append(ownerRecordIDs, created.RecordID)
	}
	otherRecordIDs := make([]string, 0, 3)
	for i := 0; i < 3; i++ {
		status, wire = recordPost(t, ts, tokens["other"], fmt.Sprintf("fixture-other-%02d", i), recordBody)
		require.Equal(t, 201, status, string(wire))
		var created record.Receipt
		require.NoError(t, json.Unmarshal(wire, &created))
		otherRecordIDs = append(otherRecordIDs, created.RecordID)
	}
	// Create a second real POST while this synthetic principal has write access,
	// then revoke only write access in the external grant fixture before GET.
	readActor := f.actors[tokens["readonly"]]
	readActor.Role = "listingkit_operator"
	f.actors[tokens["readonly"]] = readActor
	status, readWire := recordPost(t, ts, tokens["readonly"], "readonly-seed", recordBody)
	require.Equal(t, 201, status, string(readWire))
	var readReceipt record.Receipt
	require.NoError(t, json.Unmarshal(readWire, &readReceipt))
	readActor.Role = "admin"
	f.actors[tokens["readonly"]] = readActor
	before := diagnosticBusinessState(t, db)
	current.Store(application()) // new app, reader, repository and evaluator after actual POST
	registry := kernelmodule.NewRegistry()
	require.NoError(t, contextapi.NewModule(contextapi.NewHandlerWithWorkbenchAuthorizer(authorizer)).Register(registry))
	contextApp := buildHTTPServerFromRoutesAtWithAuthDependencies("127.0.0.1", 0, registry.Routes(), routeAuthDependencies{workbenchVerifier: f, organizationResolver: resolver, authorizer: authorizer, auditRecorder: workbenchcontext.NewStructuredAuditRecorder(logrus.New())})
	contextServer := httptest.NewServer(contextApp.Handler)
	t.Cleanup(contextServer.Close)
	manifest, err := json.Marshal(map[string]any{"goOrigin": ts.URL, "contextOrigin": contextServer.URL, "recordId": receipt.RecordID, "recordCount": len(ownerRecordIDs), "otherRecordCount": len(otherRecordIDs), "readonlyRecordId": readReceipt.RecordID, "tokens": tokens})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.json"), manifest, 0600))
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	deadline := time.NewTimer(30 * time.Minute)
	defer deadline.Stop()
	for {
		select {
		case <-deadline.C:
			t.Fatal("fixture maximum lifetime reached")
		case <-ticker.C:
			if _, e := os.Stat(filepath.Join(dir, "restart")); e == nil {
				current.Store(application())
				require.NoError(t, os.Remove(filepath.Join(dir, "restart")))
				require.NoError(t, os.WriteFile(filepath.Join(dir, "restarted"), []byte("ok"), 0600))
			}
			if _, e := os.Stat(filepath.Join(dir, "stop")); e == nil {
				require.Equal(t, before, diagnosticBusinessState(t, db), "BFF GET/browser run changed business tables or row versions")
				return
			}
		}
	}
}
