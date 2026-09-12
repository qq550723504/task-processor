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
	closedListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	if err := closedListener.Close(); err != nil {
		t.Fatal(err)
	}
	if err := application.Serve(closedListener); err == nil {
		t.Fatal("Serve() with a closed caller-owned listener returned nil")
	}
	if err := db.Exec("SELECT 1").Error; err != nil {
		t.Fatalf("caller-owned database was not usable after listen failure: %v", err)
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
	if err := db.Exec("SELECT 1").Error; err != nil {
		t.Fatalf("caller-owned database was not usable after server shutdown: %v", err)
	}
	sameDBRebuilt, err := NewSourceAccountApplication(db, authFixture, resolver, authorizer)
	if err != nil {
		t.Fatal(err)
	}
	sameDBURL, stopSameDB := serveSourceAccountApplication(t, sameDBRebuilt)
	status, body = sourceAccountRequest(t, sameDBURL, "token-admin", "org-b", http.MethodGet, "/api/v1/workbench/source-accounts", "", "", "")
	if status != http.StatusOK || !bytes.Contains(body, []byte(created.Account.ID)) {
		t.Fatalf("same-database rebuild status=%d body=%s", status, body)
	}
	stopSameDB()
	if err := db.Exec("SELECT 1").Error; err != nil {
		t.Fatalf("caller-owned database was not usable after rebuilt server shutdown: %v", err)
	}
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

func TestSourceAccountConfiguredUserPostgresAcceptance(t *testing.T) {
	db, _ := openSourceAccountApplicationPostgres(t)
	if err := registrySchema.Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	fixture := &sourceAccountSubjectFixture{sourceAccountAuthFixture: newSourceAccountAuthFixture()}
	start := func(users []string) (string, func()) {
		authorizer, err := authz.NewListingKitAuthorizer(users, nil)
		if err != nil {
			t.Fatal(err)
		}
		resolver := workbenchcontext.NewResolver(fixture, "project", "v1", fixture)
		application, err := NewSourceAccountApplication(db, fixture, resolver, authorizer)
		if err != nil {
			t.Fatal(err)
		}
		return serveSourceAccountApplication(t, application)
	}
	baseURL, stop := start([]string{"actor-1"})
	const path = "/api/v1/workbench/source-accounts"
	const key = "0198c5c0-6666-7666-8666-666666666666"
	const input = `{"displayName":"Subject authority","platform":"1688"}`
	status, body := sourceAccountRequest(t, baseURL, "token-admin", "org-b", http.MethodPost, path, key, "", input)
	if status != http.StatusCreated {
		t.Fatalf("configured user with only custom_member role: status=%d body=%s", status, body)
	}
	var created struct{ Account struct{ ID string } }
	if err := json.Unmarshal(body, &created); err != nil || created.Account.ID == "" {
		t.Fatalf("created account: %s, %v", body, err)
	}
	item := path + "/" + created.Account.ID
	for _, readPath := range []string{path, item} {
		status, body = sourceAccountRequest(t, baseURL, "token-admin", "org-b", http.MethodGet, readPath, "", "", "")
		if status != http.StatusOK || !bytes.Contains(body, []byte(created.Account.ID)) {
			t.Fatalf("configured user read %s: %d %s", readPath, status, body)
		}
	}
	status, body = sourceAccountRequest(t, baseURL, "token-admin", "org-b", http.MethodPost, path, key, "", `{"displayName":"Different","platform":"1688"}`)
	assertHTTPCode(t, status, body, http.StatusConflict, "IDEMPOTENCY_CONFLICT")
	for index, action := range []string{"disable", "enable"} {
		actionKey := fmt.Sprintf("0198c5c0-7777-7777-8777-%012d", index+1)
		etag := fmt.Sprintf(`"%d"`, index+1)
		status, body = sourceAccountRequest(t, baseURL, "token-admin", "org-b", http.MethodPost, item+"/"+action, actionKey, etag, "")
		if status != http.StatusOK || !bytes.Contains(body, []byte(fmt.Sprintf(`"version":"%d"`, index+2))) {
			t.Fatalf("configured user %s: %d %s", action, status, body)
		}
		status, body = sourceAccountRequest(t, baseURL, "token-admin", "org-b", http.MethodPost, item+"/"+action, actionKey, etag, "")
		if status != http.StatusOK || !bytes.Contains(body, []byte(`"replayed":true`)) {
			t.Fatalf("configured user %s replay: %d %s", action, status, body)
		}
	}
	assertUnchanged := func() {
		t.Helper()
		var row struct{ Count, Version int64 }
		if err := db.Raw("SELECT count(*) AS count, max(version) AS version FROM source_account_resources").Scan(&row).Error; err != nil {
			t.Fatal(err)
		}
		var receipts int64
		if err := db.Table("source_account_operations").Count(&receipts).Error; err != nil {
			t.Fatal(err)
		}
		if row.Count != 1 || row.Version != 3 || receipts != 3 {
			t.Fatalf("unexpected persistence: resources=%d version=%d receipts=%d", row.Count, row.Version, receipts)
		}
	}
	for _, test := range []struct {
		name, token, org, method, target, requestKey, payload string
		status                                                int
		code                                                  string
	}{
		{"no grant", "token-admin", "org-c", http.MethodPost, path, key, input, 403, "ORGANIZATION_ACCESS_DENIED"},
		{"other admitted organization", "token-admin", "org-a", http.MethodGet, item, "", "", 404, "SOURCE_ACCOUNT_NOT_FOUND"},
		{"different actor", "token-viewer", "org-b", http.MethodPost, path, key, input, 403, "PERMISSION_DENIED"},
	} {
		t.Run(test.name, func(t *testing.T) {
			status, body := sourceAccountRequest(t, baseURL, test.token, test.org, test.method, test.target, test.requestKey, "", test.payload)
			assertHTTPCode(t, status, body, test.status, test.code)
			assertUnchanged()
		})
	}
	fixture.setRevoked(true)
	status, body = sourceAccountRequest(t, baseURL, "token-admin", "org-b", http.MethodPost, path, key, "", input)
	assertHTTPCode(t, status, body, 403, "ORGANIZATION_ACCESS_DENIED")
	assertUnchanged()
	fixture.setRevoked(false)
	fixture.setUnavailable(true)
	status, body = sourceAccountRequest(t, baseURL, "token-admin", "org-b", http.MethodPost, path, key, "", input)
	assertHTTPCode(t, status, body, 503, "DEPENDENCY_UNAVAILABLE")
	assertUnchanged()
	fixture.setUnavailable(false)
	fixture.setSuspended(true)
	status, body = sourceAccountRequest(t, baseURL, "token-admin", "org-b", http.MethodPost, path, key, "", input)
	assertHTTPCode(t, status, body, 403, "ORGANIZATION_SUSPENDED")
	assertUnchanged()
	fixture.setSuspended(false)
	fixture.setExpired(true)
	status, body = sourceAccountRequest(t, baseURL, "token-admin", "org-b", http.MethodPost, path, key, "", input)
	assertHTTPCode(t, status, body, 401, "AUTHENTICATION_REQUIRED")
	assertUnchanged()
	fixture.setExpired(false)
	status, body = sourceAccountRequest(t, baseURL, "token-admin", "org-b", http.MethodPost, path, key, "", input)
	if status != http.StatusOK || !bytes.Contains(body, []byte(`"replayed":true`)) || !bytes.Contains(body, []byte(`"version":"3"`)) {
		t.Fatalf("restored configured user replay: %d %s", status, body)
	}
	assertUnchanged()
	stop()
	// Removing the configured subject requires a newly constructed authorizer;
	// reducing roles alone is not revocation of the still-configured subject.
	baseURL, stop = start(nil)
	defer stop()
	for _, readPath := range []string{path, item} {
		status, body = sourceAccountRequest(t, baseURL, "token-admin", "org-b", http.MethodGet, readPath, "", "", "")
		assertHTTPCode(t, status, body, 403, "PERMISSION_DENIED")
	}
	status, body = sourceAccountRequest(t, baseURL, "token-admin", "org-b", http.MethodPost, path, key, "", input)
	assertHTTPCode(t, status, body, 403, "PERMISSION_DENIED")
	assertUnchanged()
	if fixture.liveReads < 10 || fixture.cachedReads < 4 {
		t.Fatalf("expected fresh replay admission: live=%d cached=%d", fixture.liveReads, fixture.cachedReads)
	}
}

type sourceAccountSubjectFixture struct {
	*sourceAccountAuthFixture
	expired   bool
	suspended bool
}

func (f *sourceAccountSubjectFixture) IsOrganizationSuspended(context.Context, string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.suspended, nil
}

func (f *sourceAccountSubjectFixture) setSuspended(value bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.suspended = value
}

func (f *sourceAccountSubjectFixture) Verify(ctx context.Context, token string) (authidentity.AuthenticatedIdentity, error) {
	identity, err := f.sourceAccountAuthFixture.Verify(ctx, token)
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.expired {
		identity.TokenExpiresAt = time.Now().Add(-time.Minute)
	}
	return identity, err
}

func (f *sourceAccountSubjectFixture) Load(ctx context.Context, source workbenchcontext.GrantSource, request workbenchcontext.GrantRequest) (workbenchcontext.GrantResult, error) {
	result, err := f.sourceAccountAuthFixture.Load(ctx, source, request)
	if request.Subject == "actor-1" {
		for index := range result.Grants {
			result.Grants[index].Roles = []string{"custom_member"}
		}
	}
	return result, err
}

func (f *sourceAccountSubjectFixture) setExpired(value bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.expired = value
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
