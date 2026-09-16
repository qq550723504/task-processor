//go:build integration

package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	registrySchema "task-processor/internal/app/schema/sourceaccountregistry"
	"task-processor/internal/core/config"
	memberstore "task-processor/internal/integration/persistence/organization/membership"
	"task-processor/internal/listingsubscription"
	platformdatabase "task-processor/internal/platform/database"
)

type membershipFixture struct {
	mu                                    sync.Mutex
	provider, application                 *httptest.Server
	roles                                 map[string]string
	users                                 map[string]map[string]any
	changed                               int
	revoked, permissionDenied, loseUpdate bool
	sends                                 int
}

func newMembershipFixture(t *testing.T) *membershipFixture {
	t.Helper()
	ctx := context.Background()
	owner, connection := commercialPostgres(t)
	require.NoError(t, listingsubscription.AutoMigrateRepository(owner))
	require.NoError(t, registrySchema.Migrate(ctx, owner))
	sqlDB, err := owner.DB()
	require.NoError(t, err)
	tx, err := sqlDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	require.NoError(t, memberstore.InstallSchemaTx(ctx, tx))
	require.NoError(t, tx.Commit())
	require.NoError(t, owner.Exec(`REVOKE CREATE,TEMPORARY ON DATABASE issue347 FROM PUBLIC; REVOKE CREATE ON SCHEMA public FROM PUBLIC; REVOKE ALL ON ALL TABLES IN SCHEMA public FROM PUBLIC`).Error)
	pools := map[string]*gorm.DB{}
	for _, role := range []string{"source_account_runtime", "commercial_reader", "organization_membership_runtime"} {
		require.NoError(t, owner.Exec(`CREATE ROLE `+role+` LOGIN PASSWORD 'membership-fixture-password'; GRANT CONNECT ON DATABASE issue347 TO `+role+`; GRANT USAGE ON SCHEMA public TO `+role).Error)
		allowed := run1AllowedPrivileges[role]
		if role == "organization_membership_runtime" {
			allowed = map[string][]string{"organization_member_operations": {"SELECT", "INSERT", "UPDATE"}}
		}
		for table, privileges := range allowed {
			require.NoError(t, owner.Exec(`GRANT `+strings.Join(privileges, ",")+` ON public.`+table+` TO `+role).Error)
		}
		cfg := &platformdatabase.Config{Host: "127.0.0.1", Port: connection.Port, User: role, Password: "membership-fixture-password", Database: connection.Database, MaxConnections: 3, MaxIdleConnections: 1}
		var pool *gorm.DB
		if role == "commercial_reader" {
			pool, err = platformdatabase.OpenExistingReadOnlyContext(ctx, cfg)
		} else {
			pool, err = platformdatabase.OpenExistingWritableContext(ctx, cfg)
		}
		require.NoError(t, err)
		pools[role] = pool
		t.Cleanup(func() { require.NoError(t, platformdatabase.Close(pool)) })
	}
	fixture := &membershipFixture{roles: map[string]string{"member": "listingkit_viewer"}, users: map[string]map[string]any{}}
	fixture.provider = httptest.NewServer(http.HandlerFunc(fixture.handleProvider))
	t.Cleanup(fixture.provider.Close)
	cfg := &config.Config{Workbench: config.WorkbenchConfig{Enabled: true}, ListingKit: config.ListingKitConfig{Zitadel: config.ListingKitZitadelConfig{IssuerURL: fixture.provider.URL, AuthorizationAPIURL: fixture.provider.URL, ClientID: "membership-fixture", ClientSecret: "synthetic-client-secret", ProjectID: "project", AuthorizationRequired: true}}}
	logger := logrus.New()
	logger.SetOutput(io.Discard)
	server, err := NewCurrentApplicationWithMembership(ctx, pools["source_account_runtime"], pools["commercial_reader"], cfg, logger, MembershipDependencies{ReceiptDB: pools["organization_membership_runtime"], ProviderOrigin: fixture.provider.URL, ReadToken: "directory-read-sentinel", WriteToken: "membership-write-sentinel"})
	require.NoError(t, err)
	fixture.application = httptest.NewServer(server.Handler)
	t.Cleanup(fixture.application.Close)
	return fixture
}

func (f *membershipFixture) handleProvider(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	emit := func(value any) { _ = json.NewEncoder(w).Encode(value) }
	switch r.URL.Path {
	case "/.well-known/openid-configuration":
		emit(map[string]string{"introspection_endpoint": f.provider.URL + "/oauth/v2/introspect"})
		return
	case "/oauth/v2/introspect":
		_ = r.ParseForm()
		subject := r.Form.Get("token")
		emit(map[string]any{"active": subject != "expired", "sub": subject, "exp": time.Now().Add(time.Hour).Unix(), "urn:zitadel:iam:user:resourceowner:id": "A"})
		return
	case "/auth/v1/permissions/zitadel/me/_search":
		if token != "directory-read-sentinel" {
			w.WriteHeader(403)
			return
		}
		permissions := []string{}
		if !f.permissionDenied && (r.Header.Get("x-zitadel-orgid") == "B" || r.Header.Get("x-zitadel-orgid") == "C") {
			permissions = []string{"user.grant.read", "user.read"}
		}
		emit(map[string]any{"result": permissions})
		return
	}
	if strings.HasPrefix(r.URL.Path, "/v2/users/") && r.Method == "GET" {
		if token != "directory-read-sentinel" {
			w.WriteHeader(403)
			return
		}
		user, ok := f.users[strings.TrimPrefix(r.URL.Path, "/v2/users/")]
		if !ok {
			w.WriteHeader(404)
			return
		}
		emit(map[string]any{"user": user})
		return
	}
	var payload map[string]json.RawMessage
	if json.NewDecoder(r.Body).Decode(&payload) != nil {
		w.WriteHeader(400)
		return
	}
	field := func(key string) string { var value string; _ = json.Unmarshal(payload[key], &value); return value }
	if r.URL.Path == "/zitadel.authorization.v2.AuthorizationService/ListAuthorizations" {
		rows := []any{}
		if token != "directory-read-sentinel" {
			role := map[string]string{"viewer": "listingkit_viewer", "operator": "listingkit_operator", "admin": "listingkit_admin"}[token]
			if !f.revoked && role != "" {
				for _, org := range []string{"B", "C"} {
					rows = append(rows, map[string]any{"id": "actor-grant-" + org, "project": map[string]string{"id": "project"}, "organization": map[string]string{"id": org, "name": "Enterprise " + org}, "user": map[string]string{"id": token}, "state": "STATE_ACTIVE", "roles": []any{map[string]string{"key": role}}})
				}
			}
		} else {
			var filters []map[string]json.RawMessage
			_ = json.Unmarshal(payload["filters"], &filters)
			org, exact := "", ""
			for _, filter := range filters {
				var value struct {
					ID  string   `json:"id"`
					IDs []string `json:"ids"`
				}
				if raw := filter["organizationId"]; raw != nil {
					_ = json.Unmarshal(raw, &value)
					org = value.ID
				}
				if raw := filter["authorizationIds"]; raw != nil {
					_ = json.Unmarshal(raw, &value)
					if len(value.IDs) == 1 {
						exact = value.IDs[0]
					}
				}
			}
			if !f.permissionDenied {
				for user, role := range f.roles {
					id := "grant-" + user
					if exact != "" && exact != id {
						continue
					}
					rows = append(rows, map[string]any{"id": id, "creationDate": "2026-09-12T00:00:00Z", "changeDate": time.Date(2026, 9, 12, 0, 0, f.changed, 0, time.UTC).Format(time.RFC3339Nano), "project": map[string]string{"id": "project"}, "organization": map[string]string{"id": org}, "user": map[string]string{"id": user, "displayName": "成员 " + user, "preferredLoginName": user + "@example.com"}, "state": "STATE_ACTIVE", "roles": []any{map[string]string{"key": role}}})
				}
			}
		}
		emit(map[string]any{"pagination": map[string]string{"totalResult": fmt.Sprint(len(rows))}, "authorizations": rows})
		return
	}
	if token != "membership-write-sentinel" {
		w.WriteHeader(403)
		return
	}
	f.sends++
	f.changed++
	at := time.Now().UTC().Format(time.RFC3339Nano)
	switch r.URL.Path {
	case "/v2/users/new":
		id := field("userId")
		var human map[string]any
		_ = json.Unmarshal(payload["human"], &human)
		f.users[id] = map[string]any{"userId": id, "username": field("username"), "details": map[string]string{"resourceOwner": field("organizationId")}, "human": human}
		emit(map[string]string{"id": id, "creationDate": at})
	case "/zitadel.authorization.v2.AuthorizationService/CreateAuthorization":
		var roles []string
		_ = json.Unmarshal(payload["roleKeys"], &roles)
		if len(roles) != 1 {
			w.WriteHeader(400)
			return
		}
		id := field("userId")
		f.roles[id] = roles[0]
		emit(map[string]string{"id": "grant-" + id, "creationDate": at})
	case "/zitadel.authorization.v2.AuthorizationService/UpdateAuthorization":
		var roles []string
		_ = json.Unmarshal(payload["roleKeys"], &roles)
		if len(roles) != 1 {
			w.WriteHeader(400)
			return
		}
		f.roles[strings.TrimPrefix(field("id"), "grant-")] = roles[0]
		if f.loseUpdate {
			conn, _, err := w.(http.Hijacker).Hijack()
			if err == nil {
				_ = conn.Close()
			}
			return
		}
		emit(map[string]string{"changeDate": at})
	case "/zitadel.authorization.v2.AuthorizationService/DeleteAuthorization":
		delete(f.roles, strings.TrimPrefix(field("id"), "grant-"))
		emit(map[string]string{"deletionDate": at})
	default:
		w.WriteHeader(404)
	}
}

func (f *membershipFixture) request(t *testing.T, method, path, actor, org, key string, payload any) (int, map[string]any) {
	t.Helper()
	var body []byte
	if payload != nil {
		var err error
		body, err = json.Marshal(payload)
		require.NoError(t, err)
	}
	request, err := http.NewRequest(method, f.application.URL+path, bytes.NewReader(body))
	require.NoError(t, err)
	request.Header.Set("Authorization", "Bearer "+actor)
	request.Header.Set("X-Requested-Organization-ID", org)
	if key != "" {
		request.Header.Set("Idempotency-Key", key)
	}
	if payload != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := http.DefaultClient.Do(request)
	require.NoError(t, err)
	defer response.Body.Close()
	var result map[string]any
	require.NoError(t, json.NewDecoder(response.Body).Decode(&result))
	return response.StatusCode, result
}

func TestMembershipMountedPostgresProviderChain(t *testing.T) {
	f := newMembershipFixture(t)
	for _, actor := range []string{"viewer", "operator", "admin"} {
		status, result := f.request(t, "GET", "/api/v1/account/members", actor, "B", "", nil)
		require.Equal(t, 200, status, result)
		require.Equal(t, actor == "admin", result["canManage"])
	}
	status, detail := f.request(t, "GET", "/api/v1/account/members/grant-member", "admin", "B", "", nil)
	require.Equal(t, 200, status, detail)
	version := detail["items"].([]any)[0].(map[string]any)["observedVersion"]
	key := uuid.NewString()
	payload := map[string]any{"role": "listingkit_operator", "expectedVersion": version}
	status, result := f.request(t, "POST", "/api/v1/account/members/grant-member/role", "viewer", "B", key, payload)
	require.Equal(t, 403, status, result)
	status, result = f.request(t, "POST", "/api/v1/account/members/grant-member/role", "admin", "B", key, payload)
	require.Equal(t, 200, status, result)
	require.Equal(t, "acknowledged", result["status"])
	status, result = f.request(t, "POST", "/api/v1/account/members/grant-member/role", "admin", "B", key, payload)
	require.Equal(t, 200, status, result)
	f.mu.Lock()
	require.Equal(t, 1, f.sends)
	f.loseUpdate = true
	f.mu.Unlock()
	status, detail = f.request(t, "GET", "/api/v1/account/members/grant-member", "admin", "B", "", nil)
	require.Equal(t, 200, status, detail)
	version = detail["items"].([]any)[0].(map[string]any)["observedVersion"]
	key = uuid.NewString()
	payload = map[string]any{"role": "listingkit_viewer", "expectedVersion": version}
	status, result = f.request(t, "POST", "/api/v1/account/members/grant-member/role", "admin", "B", key, payload)
	require.Equal(t, 200, status, result)
	require.Equal(t, "unknown", result["status"])
	status, result = f.request(t, "POST", "/api/v1/account/member-operations/"+key+"/verify", "admin", "B", "", nil)
	require.Equal(t, 200, status, result)
	require.Equal(t, "unknown", result["status"])
	f.mu.Lock()
	require.Equal(t, 2, f.sends)
	f.revoked = true
	f.mu.Unlock()
	status, _ = f.request(t, "GET", "/api/v1/account/members", "admin", "B", "", nil)
	require.Equal(t, 403, status)
}

func TestMembershipMountedInviteRemoveAndScopedReceipts(t *testing.T) {
	f := newMembershipFixture(t)
	key := uuid.NewString()
	input := map[string]any{"email": "fixture@example.com", "firstName": "Test", "lastName": "Member", "role": "listingkit_viewer"}
	status, result := f.request(t, "POST", "/api/v1/account/members/invitations", "admin", "B", key, input)
	require.Equal(t, 200, status, result)
	require.Equal(t, "acknowledged", result["status"])
	require.NotNil(t, result["userAcknowledgment"])
	require.NotNil(t, result["acknowledgment"])
	grant := result["authorizationId"].(string)
	status, replay := f.request(t, "POST", "/api/v1/account/members/invitations", "admin", "B", key, input)
	require.Equal(t, 200, status, replay)
	require.Equal(t, result["targetUserId"], replay["targetUserId"])
	status, _ = f.request(t, "GET", "/api/v1/account/member-operations/"+key, "admin", "C", "", nil)
	require.Equal(t, 404, status)
	status, _ = f.request(t, "GET", "/api/v1/account/member-operations/"+key, "viewer", "B", "", nil)
	require.Equal(t, 403, status)
	status, _ = f.request(t, "GET", "/api/v1/account/members", "admin", "A", "", nil)
	require.Equal(t, 403, status)
	status, detail := f.request(t, "GET", "/api/v1/account/members/"+grant, "admin", "B", "", nil)
	require.Equal(t, 200, status, detail)
	version := detail["items"].([]any)[0].(map[string]any)["observedVersion"]
	status, result = f.request(t, "POST", "/api/v1/account/members/"+grant+"/remove", "admin", "B", uuid.NewString(), map[string]any{"expectedVersion": version})
	require.Equal(t, 200, status, result)
	require.Equal(t, "acknowledged", result["status"])
	require.Equal(t, "not_visible", result["observation"])
	f.mu.Lock()
	defer f.mu.Unlock()
	require.Equal(t, 3, f.sends)
	require.Len(t, f.users, 1, "removal must retain the identity")
}
