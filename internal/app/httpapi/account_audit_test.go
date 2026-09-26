package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"task-processor/internal/app/accountaudit"
	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	accountprofilestore "task-processor/internal/integration/persistence/accountprofile"
	kernelmodule "task-processor/internal/kernel/module"
	"task-processor/internal/ledger/orgresource"
	registry "task-processor/internal/sourceaccountregistry"
	"task-processor/internal/workbenchcontext"
)

type auditHTTPHistory struct {
	calls       int
	unavailable bool
}

type imagePointAuditHTTPHistory struct{ calls int }

func (h *imagePointAuditHTTPHistory) ListImagePointDebits(_ context.Context, org, actor string, _ int, _ *orgresource.ImagePointAuditPosition) (orgresource.ImagePointAuditPage, error) {
	h.calls++
	if org != "B" {
		return orgresource.ImagePointAuditPage{}, registry.ErrForbidden
	}
	if actor != "" && actor != "safe-actor" {
		return orgresource.ImagePointAuditPage{}, nil
	}
	return orgresource.ImagePointAuditPage{Items: []orgresource.ImagePointDebit{{OrganizationID: org, EventID: "event-1", ActorID: "safe-actor", MemberID: "grant-1", RunID: "run-1", IntentID: "intent-1", PriceVersion: "price-1", Points: 12, CreatedAt: time.Now().UTC().Truncate(time.Microsecond)}}}, nil
}

func TestAccountAuditHTTPProjectsImagePointsUnderLiveOrgPermission(t *testing.T) {
	points := &imagePointAuditHTTPHistory{}
	query, err := accountaudit.NewWithImagePointAuditSources(&auditHTTPHistory{}, nil, nil, nil, nil, points)
	if err != nil {
		t.Fatal(err)
	}
	modules := kernelmodule.NewRegistry()
	if err := (accountAuditModule{query: query}).Register(modules); err != nil {
		t.Fatal(err)
	}
	grants := &auditHTTPGrants{role: "listingkit_viewer"}
	authorizer, _ := authz.NewListingKitAuthorizer(nil, nil)
	server := buildIsolatedApplicationHTTPServer(modules.Routes(), routeAuthDependencies{workbenchVerifier: applicationVerifier{}, organizationResolver: workbenchcontext.NewResolver(grants, "project", "v1", nil), authorizer: authorizer}, registry.Timeout)
	get := func(org string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", accountAuditPath+"?actor=safe-actor", nil)
		r.Header.Set("X-Requested-Organization-ID", org)
		r.Header.Set("Authorization", "Bearer fixture")
		w := httptest.NewRecorder()
		server.Handler.ServeHTTP(w, r)
		return w
	}
	w := get("B")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"points":{"memberId":"grant-1","quantity":"12","priceVersion":"price-1","intentId":"intent-1"}`) {
		t.Fatalf("point read %d %s", w.Code, w.Body.String())
	}
	before := points.calls
	if w = get("A"); w.Code != 403 {
		t.Fatalf("cross org %d", w.Code)
	}
	grants.revoked = true
	if w = get("B"); w.Code != 403 {
		t.Fatalf("revoked %d", w.Code)
	}
	if points.calls != before {
		t.Fatal("denied read reached point owner")
	}
}

func (h *auditHTTPHistory) List(ctx context.Context, _ registry.HistoryRequest) (registry.HistoryPage, error) {
	h.calls++
	if h.unavailable {
		return registry.HistoryPage{}, registry.ErrUnavailable
	}
	identity, _ := authidentity.AuthenticatedIdentityFromContext(ctx)
	return registry.HistoryPage{Items: []registry.CommittedOperation{{OrganizationID: identity.EffectiveOrganizationID, ActorSubject: "safe-actor", AccountID: "0198d4f0-0000-7000-8000-000000000001", Kind: registry.OperationRegister, Version: 1, OccurredAt: time.Now().UTC().Truncate(time.Microsecond)}}}, nil
}

type auditHTTPGrants struct {
	role                 string
	revoked, unavailable bool
	sources              []workbenchcontext.GrantSource
}

func (g *auditHTTPGrants) Invalidate(string, string) {}
func (g *auditHTTPGrants) Load(_ context.Context, source workbenchcontext.GrantSource, _ workbenchcontext.GrantRequest) (workbenchcontext.GrantResult, error) {
	g.sources = append(g.sources, source)
	if g.unavailable {
		return workbenchcontext.GrantResult{}, registry.ErrUnavailable
	}
	result := workbenchcontext.GrantResult{Source: source}
	if !g.revoked {
		result.Grants = []authidentity.OrganizationGrant{{OrganizationID: "B", ProjectID: "project", Roles: []string{g.role}}}
	}
	return result, nil
}
func TestAccountAuditHTTPUsesFreshOrgReadPermission(t *testing.T) {
	history := &auditHTTPHistory{}
	query, _ := accountaudit.New(history)
	module := accountAuditModule{query: query}
	modules := kernelmodule.NewRegistry()
	if err := module.Register(modules); err != nil {
		t.Fatal(err)
	}
	grants := &auditHTTPGrants{role: "listingkit_viewer"}
	authorizer, _ := authz.NewListingKitAuthorizer(nil, nil)
	server := buildIsolatedApplicationHTTPServer(modules.Routes(), routeAuthDependencies{workbenchVerifier: applicationVerifier{}, organizationResolver: workbenchcontext.NewResolver(grants, "project", "v1", nil), authorizer: authorizer}, registry.Timeout)
	get := func(org, query string, token bool) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", "/api/v1/account/audit"+query, nil)
		r.Header.Set("X-Requested-Organization-ID", org)
		if token {
			r.Header.Set("Authorization", "Bearer fixture")
		}
		w := httptest.NewRecorder()
		server.Handler.ServeHTTP(w, r)
		return w
	}
	for _, role := range []string{"listingkit_viewer", "listingkit_operator", "listingkit_admin"} {
		grants.role = role
		w := get("B", "?limit=20", true)
		if w.Code != 200 || !strings.Contains(w.Body.String(), `"effectiveOrganizationId":"B"`) {
			t.Fatalf("%s: %d %s", role, w.Code, w.Body.String())
		}
	}
	for _, source := range grants.sources {
		if source != workbenchcontext.GrantLive {
			t.Fatal("cached permission read")
		}
	}
	before := history.calls
	if w := get("A", "", true); w.Code != 403 {
		t.Fatalf("cross org %d", w.Code)
	}
	if w := get("B", "", false); w.Code != 401 {
		t.Fatalf("missing auth %d", w.Code)
	}
	grants.role = "no-role"
	if w := get("B", "", true); w.Code != 403 {
		t.Fatalf("role %d", w.Code)
	}
	grants.role = "listingkit_viewer"
	grants.revoked = true
	if w := get("B", "", true); w.Code != 403 {
		t.Fatalf("revoke %d", w.Code)
	}
	grants.revoked = false
	grants.unavailable = true
	if w := get("B", "", true); w.Code != 503 {
		t.Fatalf("dependency %d", w.Code)
	}
	grants.unavailable = false
	if history.calls != before {
		t.Fatal("denied request reached source")
	}
	for _, bad := range []string{"?limit=0", "?limit=101", "?limit=1&limit=2", "?org=A", "?cursor=", "?cursor=!"} {
		if w := get("B", bad, true); w.Code != 400 {
			t.Fatalf("query %s: %d %s", bad, w.Code, w.Body.String())
		}
	}
	history.unavailable = true
	if w := get("B", "", true); w.Code != 503 {
		t.Fatalf("source failure %d", w.Code)
	}
	if got := modules.Routes()[0]; got.Method != http.MethodGet || got.Permission != authz.PermissionWorkbenchSourceAccountRead {
		t.Fatal("wrong route contract")
	}
}

func TestProfileAuditReaderUsesFixedWidthCursorAcrossTimestampTie(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE account_business_profile_audit_events (
id INTEGER PRIMARY KEY AUTOINCREMENT, organization_id TEXT NOT NULL, actor_id TEXT NOT NULL,
user_id TEXT NOT NULL, operation TEXT NOT NULL, version INTEGER NOT NULL, created_at DATETIME NOT NULL
)`).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	for _, id := range []int{9, 10} {
		if err := db.Exec("INSERT INTO account_business_profile_audit_events (id, organization_id, actor_id, user_id, operation, version, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)", id, "org", "actor", "u"+strconv.Itoa(id), "update", id, now).Error; err != nil {
			t.Fatal(err)
		}
	}
	repository, err := accountprofilestore.New(db)
	if err != nil {
		t.Fatal(err)
	}
	reader := profileAuditReader{repository: repository}
	first, err := reader.ListRecentAudit(context.Background(), "org", 1, "", "", nil)
	if err != nil || len(first.Items) != 1 || first.Items[0].ObjectReference != "u10" || first.Next == nil {
		t.Fatalf("first page = %#v, err=%v", first, err)
	}
	second, err := reader.ListRecentAudit(context.Background(), "org", 1, "", "", first.Next)
	if err != nil || len(second.Items) != 1 || second.Items[0].ObjectReference != "u9" || second.Next != nil {
		t.Fatalf("second page = %#v, err=%v", second, err)
	}
}
