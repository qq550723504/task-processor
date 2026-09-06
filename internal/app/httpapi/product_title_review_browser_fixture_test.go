package httpapi

import (
	"context"
	"encoding/json"
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
	"task-processor/internal/authz"
	kernelmodule "task-processor/internal/kernel/module"
	"task-processor/internal/product/catalog"
	"task-processor/internal/product/review"
	"task-processor/internal/product/sourcing"
	"task-processor/internal/workbenchcontext"
	contextapi "task-processor/internal/workbenchcontext/httpapi"
)

// Test-only external identity/grants, never registered by production code.
type titleBrowserIdentity struct {
	browserFixtureIdentity
	revokeAdmin atomic.Bool
	live        atomic.Int32
	cached      atomic.Int32
}

func (f *titleBrowserIdentity) Load(ctx context.Context, source workbenchcontext.GrantSource, r workbenchcontext.GrantRequest) (workbenchcontext.GrantResult, error) {
	if source == workbenchcontext.GrantLive {
		f.live.Add(1)
		if r.Subject == "admin" && f.revokeAdmin.Load() {
			return workbenchcontext.GrantResult{Source: source}, nil
		}
	} else {
		f.cached.Add(1)
	}
	return f.browserFixtureIdentity.Load(ctx, source, r)
}

// Separate from the Listing read-only fixture: only this admitted application
// permits Product Review writes, while asserting Listing facts remain unchanged.
func TestProductTitleReviewBrowserFixture(t *testing.T) {
	dir, dsn := os.Getenv("ISSUE344_FIXTURE_DIR"), os.Getenv("ISSUE344_FIXTURE_DSN")
	if dir == "" && dsn == "" {
		t.Skip("standalone: use product-title-review-fixture.mjs")
	}
	require.NotEmpty(t, dir)
	require.True(t, strings.HasPrefix(dsn, "host=127.0.0.1 ") && strings.Contains(dsn, " dbname=issue344_fixture "), "requires task-only loopback database")
	t.Setenv("ISSUE333_TEST_DSN", dsn)
	f := newTitleFixture(t) // actual Sourcing/Catalog Publisher and exact bindings
	schema, err := os.ReadFile("../listingrecordstore/schema.sql")
	require.NoError(t, err)
	require.NoError(t, f.db.Exec(string(schema)).Error)
	identity := &titleBrowserIdentity{browserFixtureIdentity: browserFixtureIdentity{actors: map[string]browserFixtureActor{}}}
	tokens := map[string]string{}
	for name, role := range map[string]string{"owner": "listingkit_operator", "other": "listingkit_operator", "admin": "listingkit_admin", "readonly": "admin", "store": "store_viewer", "revoked": "listingkit_admin", "slow": "listingkit_operator", "unavailable": "listingkit_operator"} {
		token := uuid.NewString()
		tokens[name] = token
		identity.actors[token] = browserFixtureActor{Subject: name, Role: role, Revoked: name == "revoked"}
	}
	authorizer, err := authz.NewListingKitAuthorizer(nil, nil)
	require.NoError(t, err)
	resolver := workbenchcontext.NewResolver(identity, "project", "v1", nil)
	sourcePublisher, err := sourcing.NewPublisher(f.publisher)
	require.NoError(t, err)
	bases := []catalog.PublishedSnapshot{f.base}
	for _, org := range []string{"200", "300"} {
		for _, key := range []string{"product", "edit-product", "lost-product", "revoked-product"} {
			p, e := sourcePublisher.Publish(context.Background(), sourcing.PublishRequest{TenantID: org, ProductKey: key, PublicationID: "fixture-initial-" + key, Envelope: f.bindings[0].Source})
			require.NoError(t, e)
			bases = append(bases, p)
			f.bindings = append(f.bindings, review.Binding{Identity: p.Identity, Version: p.Version, PublicationID: p.PublicationID, Source: f.bindings[0].Source})
		}
	}
	application := func() *http.Server {
		app, e := NewProductReviewApplication(f.db, identity, resolver, authorizer, f.g, f.bindings)
		require.NoError(t, e)
		return app
	}
	listing, _, err := NewSheinRecordApplication(f.db, identity, resolver, authorizer)
	require.NoError(t, err)
	var current atomic.Pointer[http.Server]
	current.Store(application())
	var dropNext atomic.Bool
	var posts atomic.Int32
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/listing/") {
			listing.Handler.ServeHTTP(w, r)
			return
		}
		if r.Method == http.MethodPost {
			posts.Add(1)
		}
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/apply") && dropNext.Swap(false) {
			result := httptest.NewRecorder()
			current.Load().Handler.ServeHTTP(result, r) // real transaction commits
			if result.Code == http.StatusOK {
				conn, _, e := w.(http.Hijacker).Hijack()
				if e == nil {
					_ = conn.Close() // lose only transport response, never substitute result
				}
				return
			}
			for key, values := range result.Header() {
				w.Header()[key] = values
			}
			w.WriteHeader(result.Code)
			_, _ = w.Write(result.Body.Bytes())
			return
		}
		current.Load().Handler.ServeHTTP(w, r)
	}))
	server.Config.ReadTimeout = current.Load().ReadTimeout
	server.Config.WriteTimeout = current.Load().WriteTimeout
	server.Start()
	t.Cleanup(server.Close)
	status, raw := recordPostForOrganization(t, server, tokens["owner"], "listing-original", "200", recordBody)
	require.Equal(t, 201, status)
	var listingResult struct {
		RecordID string `json:"record_id"`
	}
	require.NoError(t, json.Unmarshal(raw, &listingResult))
	listingBefore := diagnosticBusinessState(t, f.db)["listing_shein_records"]
	created := map[string]string{}
	create := func(name, org, key, label string) {
		code, body, e := titleRequest(server, "POST", titleBasePath, tokens[name], org, "create-"+label, fmt.Sprintf(`{"product_key":%q,"base_version":1}`, key))
		require.NoError(t, e)
		require.Equal(t, 200, code)
		var result struct {
			ID string `json:"proposal_id"`
		}
		require.NoError(t, json.Unmarshal(body, &result))
		require.NotEmpty(t, result.ID)
		created[label] = result.ID
	}
	for i := 0; i < 22; i++ {
		create("owner", "200", "product", fmt.Sprintf("owner-%02d", i))
	}
	for _, key := range []string{"edit-product", "lost-product", "revoked-product"} {
		create("owner", "200", key, key)
	}
	create("other", "200", "product", "other")
	create("owner", "300", "product", "organization300")
	registry := kernelmodule.NewRegistry()
	require.NoError(t, contextapi.NewModule(contextapi.NewHandlerWithWorkbenchAuthorizer(authorizer)).Register(registry))
	auditFile, err := os.OpenFile(filepath.Join(dir, "audit.jsonl"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, auditFile.Close()) })
	auditLogger := logrus.New()
	auditLogger.SetOutput(auditFile)
	auditLogger.SetFormatter(&logrus.JSONFormatter{})
	audit := workbenchcontext.NewStructuredAuditRecorder(auditLogger)
	contextApplication := func(recorder workbenchcontext.AuditRecorder) *http.Server {
		return buildHTTPServerFromRoutesAtWithAuthDependencies("127.0.0.1", 0, registry.Routes(), routeAuthDependencies{workbenchVerifier: identity, organizationResolver: resolver, authorizer: authorizer, auditRecorder: recorder})
	}
	var currentContext atomic.Pointer[http.Server]
	currentContext.Store(contextApplication(audit))
	contextServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { currentContext.Load().Handler.ServeHTTP(w, r) }))
	t.Cleanup(contextServer.Close)
	writeJSON := func(name string, value any) {
		data, e := json.Marshal(value)
		require.NoError(t, e)
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), data, 0600))
	}
	writeJSON("go.json", map[string]any{"goOrigin": server.URL, "contextOrigin": contextServer.URL, "tokens": tokens, "proposals": created, "ownerCount": 25, "recordId": listingResult.RecordID})
	observe := func() {
		versions := map[string]uint64{}
		titles := map[string]string{}
		for _, base := range bases {
			p, e := f.reader.GetCurrentSnapshot(context.Background(), base.Identity)
			require.NoError(t, e)
			versions[base.Identity.TenantID+"/"+base.Identity.ProductKey] = p.Version
			titles[base.Identity.TenantID+"/"+base.Identity.ProductKey] = p.Snapshot.Title
			p.Snapshot.Title = base.Snapshot.Title
			require.Equal(t, base.Snapshot, p.Snapshot, "non-title Catalog facts changed")
		}
		require.Equal(t, listingBefore, diagnosticBusinessState(t, f.db)["listing_shein_records"], "existing Listing changed")
		writeJSON("observations.json", map[string]any{"postCount": posts.Load(), "live": identity.live.Load(), "cached": identity.cached.Load(), "versions": versions, "titles": titles, "nonTitleUnchanged": true, "listingUnchanged": true})
	}
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	deadline := time.NewTimer(30 * time.Minute)
	defer deadline.Stop()
	for {
		select {
		case <-deadline.C:
			t.Fatal("fixture maximum lifetime reached")
		case <-ticker.C:
			for name, action := range map[string]func(){
				"restart":           func() { current.Store(application()) },
				"lose-response":     func() { dropNext.Store(true) },
				"revoke":            func() { identity.revokeAdmin.Store(true) },
				"restore":           func() { identity.revokeAdmin.Store(false) },
				"observe":           observe,
				"audit-missing":     func() { currentContext.Store(contextApplication(nil)) },
				"audit-unavailable": func() { currentContext.Store(contextApplication(workbenchcontext.NewStructuredAuditRecorder(nil))) },
				"audit-restore":     func() { currentContext.Store(contextApplication(audit)) },
			} {
				if _, e := os.Stat(filepath.Join(dir, name)); e == nil {
					action()
					require.NoError(t, os.Remove(filepath.Join(dir, name)))
					require.NoError(t, os.WriteFile(filepath.Join(dir, name+"-done"), []byte("ok"), 0600))
				}
			}
			if _, e := os.Stat(filepath.Join(dir, "stop")); e == nil {
				observe()
				return
			}
		}
	}
}
