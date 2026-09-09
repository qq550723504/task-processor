//go:build integration

package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	registrySchema "task-processor/internal/app/schema/sourceaccountregistry"
	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	"task-processor/internal/workbenchcontext"
)

func TestSourceAccountApplicationPostgresAcceptance(t *testing.T) {
	db, dsn := openSourceAccountApplicationPostgres(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if err := registrySchema.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	beforeTables := sourceAccountApplicationTables(t, db)
	authFixture := newSourceAccountAuthFixture()
	authorizer, err := authz.NewListingKitAuthorizer(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	resolver := workbenchcontext.NewResolver(authFixture, "project", "v1", nil)
	application, err := NewSourceAccountApplication(db, authFixture, resolver, authorizer)
	if err != nil {
		t.Fatal(err)
	}
	if afterTables := sourceAccountApplicationTables(t, db); afterTables != beforeTables {
		t.Fatalf("application construction changed schema:\nbefore=%s\nafter=%s", beforeTables, afterTables)
	}
	baseURL, stop := serveSourceAccountApplication(t, application)

	key := "0198c5c0-1111-7111-8111-111111111111"
	status, body := sourceAccountRequest(t, baseURL, "token-admin", "org-b", http.MethodPost, "/api/v1/workbench/source-accounts", key, "", `{"displayName":"Primary","platform":"1688"}`)
	if status != http.StatusCreated {
		t.Fatalf("register status=%d body=%s", status, body)
	}
	var created struct {
		Account struct {
			ID               string `json:"id"`
			Version          string `json:"version"`
			ConnectionStatus string `json:"connectionStatus"`
		} `json:"account"`
	}
	if err := json.Unmarshal(body, &created); err != nil {
		t.Fatal(err)
	}
	if created.Account.ID == "" || created.Account.Version != "1" || created.Account.ConnectionStatus != "pending_connection" {
		t.Fatalf("created response=%s", body)
	}
	var owner string
	if err := db.Raw(`SELECT organization_id FROM public.source_account_resources WHERE id = ?`, created.Account.ID).Scan(&owner).Error; err != nil || owner != "org-b" {
		t.Fatalf("persisted owner=%q err=%v", owner, err)
	}
	status, body = sourceAccountRequest(t, baseURL, "token-admin", "org-b", http.MethodGet, "/api/v1/workbench/source-accounts/"+created.Account.ID, "", "", "")
	if status != http.StatusOK || !bytes.Contains(body, []byte(`"id":"`+created.Account.ID+`"`)) || !bytes.Contains(body, []byte(`"version":"1"`)) {
		t.Fatalf("detail status=%d body=%s", status, body)
	}

	status, body = sourceAccountRequest(t, baseURL, "token-viewer", "org-b", http.MethodPost, "/api/v1/workbench/source-accounts", "0198c5c0-2222-7222-8222-222222222222", "", `{"displayName":"Denied","platform":"1688"}`)
	assertHTTPCode(t, status, body, http.StatusForbidden, "PERMISSION_DENIED")
	status, body = sourceAccountRequest(t, baseURL, "token-other", "org-b", http.MethodGet, "/api/v1/workbench/source-accounts", "", "", "")
	assertHTTPCode(t, status, body, http.StatusForbidden, "ORGANIZATION_ACCESS_DENIED")
	oversizedRequestID := strings.Repeat("r", 129*1024)
	requestWithOversizedID := sourceAccountHTTPRequest(t, baseURL, "invalid-token", "org-b", http.MethodGet, "/api/v1/workbench/source-accounts", "", "", "")
	requestWithOversizedID.Header.Set("X-Request-ID", oversizedRequestID)
	status, body = sourceAccountRoundTrip(t, requestWithOversizedID)
	assertBoundedRequestIDError(t, status, body, http.StatusUnauthorized, "AUTHENTICATION_REQUIRED", oversizedRequestID)
	requestWithOversizedID = sourceAccountHTTPRequest(t, baseURL, "token-admin", "org-b", http.MethodGet, "/api/v1/workbench/source-accounts/not-a-uuid", "", "", "")
	requestWithOversizedID.Header.Set("X-Request-ID", oversizedRequestID)
	status, body = sourceAccountRoundTrip(t, requestWithOversizedID)
	assertBoundedRequestIDError(t, status, body, http.StatusBadRequest, "INVALID_REQUEST", oversizedRequestID)

	// A committed response is deliberately discarded. A freshly reconstructed
	// application resolves the same key from PostgreSQL without a second row.
	lostKey := "0198c5c0-3333-7333-8333-333333333333"
	request := sourceAccountHTTPRequest(t, baseURL, "token-admin", "org-b", http.MethodPost, "/api/v1/workbench/source-accounts", lostKey, "", `{"displayName":"Lost response","platform":"1688"}`)
	recorder := &discardResponseWriter{header: make(http.Header)}
	application.Handler.ServeHTTP(recorder, request)
	if recorder.status != http.StatusCreated {
		t.Fatalf("discarded response status=%d", recorder.status)
	}
	stop()
	initialSQLDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err := initialSQLDB.Close(); err != nil {
		t.Fatal(err)
	}
	db = openSourceAccountApplicationDB(t, dsn)

	rebuilt, err := NewSourceAccountApplication(db, authFixture, resolver, authorizer)
	if err != nil {
		t.Fatal(err)
	}
	baseURL, stop = serveSourceAccountApplication(t, rebuilt)
	defer stop()
	status, body = sourceAccountRequest(t, baseURL, "token-admin", "org-b", http.MethodPost, "/api/v1/workbench/source-accounts", lostKey, "", `{"displayName":"Lost response","platform":"1688"}`)
	if status != http.StatusOK || !bytes.Contains(body, []byte(`"replayed":true`)) {
		t.Fatalf("response-loss replay status=%d body=%s", status, body)
	}

	status, body = sourceAccountRequest(t, baseURL, "token-admin", "org-b", http.MethodPost, "/api/v1/workbench/source-accounts/"+created.Account.ID+"/disable", "0198c5c0-4444-7444-8444-444444444444", `"1"`, "")
	if status != http.StatusOK || !bytes.Contains(body, []byte(`"managementStatus":"disabled"`)) || !bytes.Contains(body, []byte(`"version":"2"`)) {
		t.Fatalf("disable status=%d body=%s", status, body)
	}

	authFixture.setRevoked(true)
	status, body = sourceAccountRequest(t, baseURL, "token-admin", "org-b", http.MethodPost, "/api/v1/workbench/source-accounts", key, "", `{"displayName":"Primary","platform":"1688"}`)
	assertHTTPCode(t, status, body, http.StatusForbidden, "ORGANIZATION_ACCESS_DENIED")
	authFixture.setRevoked(false)
	status, body = sourceAccountRequest(t, baseURL, "token-admin", "org-b", http.MethodPost, "/api/v1/workbench/source-accounts", key, "", `{"displayName":"Primary","platform":"1688"}`)
	if status != http.StatusOK || !bytes.Contains(body, []byte(`"managementStatus":"disabled"`)) || !bytes.Contains(body, []byte(`"replayed":true`)) {
		t.Fatalf("reauthorized replay status=%d body=%s", status, body)
	}

	authFixture.setUnavailable(true)
	status, body = sourceAccountRequest(t, baseURL, "token-admin", "org-b", http.MethodPost, "/api/v1/workbench/source-accounts/"+created.Account.ID+"/enable", "0198c5c0-5555-7555-8555-555555555555", `"2"`, "")
	assertHTTPCode(t, status, body, http.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE")
	authFixture.setUnavailable(false)
	status, body = sourceAccountRequest(t, baseURL, "token-admin", "org-b", http.MethodPost, "/api/v1/workbench/source-accounts/"+created.Account.ID+"/enable", "0198c5c0-5555-7555-8555-555555555555", `"2"`, "")
	if status != http.StatusOK || !bytes.Contains(body, []byte(`"managementStatus":"enabled"`)) || !bytes.Contains(body, []byte(`"version":"3"`)) {
		t.Fatalf("enable status=%d body=%s", status, body)
	}

	status, body = sourceAccountRequest(t, baseURL, "token-admin", "org-b", http.MethodGet, "/api/v1/workbench/source-accounts?limit=1", "", "", "")
	if status != http.StatusOK || !bytes.Contains(body, []byte(`"items"`)) || bytes.Contains(body, []byte("actor-1")) {
		t.Fatalf("list status=%d body=%s", status, body)
	}
	if sourceAccountApplicationTables(t, db) != beforeTables {
		t.Fatalf("runtime changed schema: %s", sourceAccountApplicationTables(t, db))
	}
	if authFixture.liveReads < 6 || authFixture.cachedReads < 2 {
		t.Fatalf("grant reads live=%d cached=%d", authFixture.liveReads, authFixture.cachedReads)
	}
}

type sourceAccountAuthFixture struct {
	mu          sync.Mutex
	revoked     bool
	unavailable bool
	liveReads   int
	cachedReads int
}

func newSourceAccountAuthFixture() *sourceAccountAuthFixture { return &sourceAccountAuthFixture{} }

func (f *sourceAccountAuthFixture) Verify(_ context.Context, token string) (authidentity.AuthenticatedIdentity, error) {
	actors := map[string]struct{ subject, home string }{
		"token-admin":  {subject: "actor-1", home: "org-a"},
		"token-viewer": {subject: "viewer-1", home: "org-b"},
		"token-other":  {subject: "other-1", home: "org-a"},
	}
	actor, ok := actors[token]
	if !ok {
		return authidentity.AuthenticatedIdentity{}, errors.New("invalid token")
	}
	return authidentity.AuthenticatedIdentity{UserID: actor.subject, HomeOrganizationID: actor.home, TokenExpiresAt: time.Now().Add(time.Hour)}, nil
}

func (f *sourceAccountAuthFixture) Load(_ context.Context, source workbenchcontext.GrantSource, request workbenchcontext.GrantRequest) (workbenchcontext.GrantResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if source == workbenchcontext.GrantLive {
		f.liveReads++
	} else {
		f.cachedReads++
	}
	if f.unavailable {
		return workbenchcontext.GrantResult{}, errors.New("grant dependency unavailable")
	}
	grants := map[string][]authidentity.OrganizationGrant{
		"actor-1":  {{OrganizationID: "org-a", ProjectID: "project", Roles: []string{"listingkit_viewer"}}, {OrganizationID: "org-b", ProjectID: "project", Roles: []string{"listingkit_operator"}}},
		"viewer-1": {{OrganizationID: "org-b", ProjectID: "project", Roles: []string{"listingkit_viewer"}}},
		"other-1":  {{OrganizationID: "org-a", ProjectID: "project", Roles: []string{"listingkit_operator"}}},
	}[request.Subject]
	if f.revoked && request.Subject == "actor-1" {
		grants = grants[:1]
	}
	return workbenchcontext.GrantResult{Grants: grants, Source: source}, nil
}

func (f *sourceAccountAuthFixture) Invalidate(string, string) {}
func (f *sourceAccountAuthFixture) setRevoked(value bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.revoked = value
}
func (f *sourceAccountAuthFixture) setUnavailable(value bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.unavailable = value
}

func openSourceAccountApplicationPostgres(t *testing.T) (*gorm.DB, string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	container, err := tcpostgres.Run(ctx, "postgres:16-alpine", tcpostgres.WithDatabase("sourceaccountapp"), tcpostgres.WithUsername("sourceaccountapp"), tcpostgres.WithPassword("sourceaccountapp"), tcpostgres.BasicWaitStrategies())
	if err != nil {
		t.Fatalf("start PostgreSQL: %v", err)
	}
	t.Cleanup(func() { _ = container.Terminate(context.Background()) })
	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	return openSourceAccountApplicationDB(t, dsn), dsn
}

func openSourceAccountApplicationDB(t *testing.T, dsn string) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}

func serveSourceAccountApplication(t *testing.T, server *http.Server) (string, func()) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()
	stopOnce := sync.Once{}
	stop := func() {
		stopOnce.Do(func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := server.Shutdown(ctx); err != nil {
				t.Errorf("Shutdown() error = %v", err)
			}
			if err := <-done; err != nil && !errors.Is(err, http.ErrServerClosed) {
				t.Errorf("Serve() error = %v", err)
			}
		})
	}
	t.Cleanup(stop)
	return "http://" + listener.Addr().String(), stop
}

func sourceAccountRequest(t *testing.T, baseURL, token, organizationID, method, path, key, ifMatch, body string) (int, []byte) {
	t.Helper()
	request := sourceAccountHTTPRequest(t, baseURL, token, organizationID, method, path, key, ifMatch, body)
	return sourceAccountRoundTrip(t, request)
}

func sourceAccountRoundTrip(t *testing.T, request *http.Request) (int, []byte) {
	t.Helper()
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	data, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	return response.StatusCode, data
}

func sourceAccountHTTPRequest(t *testing.T, baseURL, token, organizationID, method, path, key, ifMatch, body string) *http.Request {
	t.Helper()
	request, err := http.NewRequest(method, baseURL+path, bytes.NewBufferString(body))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("X-Requested-Organization-ID", organizationID)
	request.Header.Set("X-Tenant-ID", "forged-legacy-org")
	request.Header.Set("X-User-ID", "forged-user")
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	if key != "" {
		request.Header.Set("Idempotency-Key", key)
	}
	if ifMatch != "" {
		request.Header.Set("If-Match", ifMatch)
	}
	return request
}

func assertHTTPCode(t *testing.T, status int, body []byte, wantStatus int, wantCode string) {
	t.Helper()
	if status != wantStatus || !bytes.Contains(body, []byte(fmt.Sprintf(`"code":"%s"`, wantCode))) {
		t.Fatalf("status=%d body=%s, want %d/%s", status, body, wantStatus, wantCode)
	}
}

func assertBoundedRequestIDError(t *testing.T, status int, body []byte, wantStatus int, wantCode, oversizedRequestID string) {
	t.Helper()
	assertHTTPCode(t, status, body, wantStatus, wantCode)
	if len(body) > 128*1024 || bytes.Contains(body, []byte(oversizedRequestID[:1024])) {
		t.Fatalf("unbounded request ID response length=%d body-prefix=%q", len(body), body[:min(len(body), 256)])
	}
}

func sourceAccountApplicationTables(t *testing.T, db *gorm.DB) string {
	t.Helper()
	var tables string
	if err := db.Raw(`SELECT COALESCE(string_agg(table_name, ',' ORDER BY table_name), '') FROM information_schema.tables WHERE table_schema = 'public'`).Scan(&tables).Error; err != nil {
		t.Fatal(err)
	}
	return tables
}

type discardResponseWriter struct {
	header http.Header
	status int
}

func (w *discardResponseWriter) Header() http.Header    { return w.header }
func (w *discardResponseWriter) WriteHeader(status int) { w.status = status }
func (w *discardResponseWriter) Write(data []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return len(data), nil
}
