//go:build integration

package httpapi

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
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
	invitationmail "task-processor/internal/integration/mail"
	memberstore "task-processor/internal/integration/persistence/organization/membership"
	"task-processor/internal/listingsubscription"
	platformdatabase "task-processor/internal/platform/database"
)

type membershipFixture struct {
	mu                                                sync.Mutex
	provider, application                             *httptest.Server
	roles                                             map[string]string
	users                                             map[string]map[string]any
	changed                                           int
	revoked, permissionDenied, loseUpdate, loseCreate bool
	sends                                             int
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
			allowed = map[string][]string{"organization_member_operations": {"SELECT", "INSERT", "UPDATE"}, "organization_member_invitations": {"SELECT", "INSERT", "UPDATE"}, "organization_member_audit_events": {"SELECT", "INSERT"}}
		}
		for table, privileges := range allowed {
			require.NoError(t, owner.Exec(`GRANT `+strings.Join(privileges, ",")+` ON public.`+table+` TO `+role).Error)
		}
		if role == "source_account_runtime" {
			require.NoError(t, owner.Exec(`GRANT USAGE, SELECT ON SEQUENCE public.account_business_profile_audit_events_id_seq TO `+role).Error)
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
	mailPort := membershipSMTP(t)
	server, err := NewCurrentApplicationWithMembership(ctx, pools["source_account_runtime"], pools["commercial_reader"], cfg, logger, MembershipDependencies{ReceiptDB: pools["organization_membership_runtime"], ProviderOrigin: fixture.provider.URL, ReadToken: "directory-read-sentinel", WriteToken: "membership-write-sentinel", InvitationMail: &invitationmail.Config{Host: "127.0.0.1", Port: mailPort, LocalPlaintext: true, From: "invitations@example.test", PublicOrigin: "https://localhost:22544"}})
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
	case "/auth/v1/users/me":
		emit(map[string]any{"user": map[string]any{"id": token, "details": map[string]string{"creationDate": "2026-09-01T00:00:00Z"}, "human": map[string]any{"email": map[string]any{"email": token + "@example.test", "isEmailVerified": true}}}})
		return
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
			role := map[string]string{"viewer": "listingkit_viewer", "operator": "listingkit_operator", "admin": "listingkit_admin", "other-admin": "listingkit_admin"}[token]
			if !f.revoked && role != "" {
				for _, org := range []string{"B", "C"} {
					rows = append(rows, map[string]any{"id": "actor-grant-" + org, "project": map[string]string{"id": "project"}, "organization": map[string]string{"id": org, "name": "Enterprise " + org}, "user": map[string]string{"id": token}, "state": "STATE_ACTIVE", "roles": []any{map[string]string{"key": role}}})
				}
			}
		} else {
			var filters []map[string]json.RawMessage
			_ = json.Unmarshal(payload["filters"], &filters)
			org, exact, subject := "", "", ""
			for _, filter := range filters {
				var value struct {
					ID  string   `json:"id"`
					IDs []string `json:"ids"`
				}
				if raw := filter["inUserIds"]; raw != nil {
					_ = json.Unmarshal(raw, &value)
					if len(value.IDs) == 1 {
						subject = value.IDs[0]
					}
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
				if subject == "admin" && !f.revoked {
					rows = append(rows, map[string]any{"id": "actor-grant-" + org, "project": map[string]string{"id": "project"}, "organization": map[string]string{"id": org}, "user": map[string]string{"id": subject}, "state": "STATE_ACTIVE", "roles": []any{map[string]string{"key": "listingkit_admin"}}})
				}
				for user, role := range f.roles {
					if subject != "" && subject != user || user == "recipient" && org != "B" {
						continue
					}
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
		if f.loseCreate {
			w.WriteHeader(502)
			return
		}
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

func TestMembershipPendingRecoveryKeepsUnknownAndAllowsOtherTargets(t *testing.T) {
	f := newMembershipFixture(t)
	status, detail := f.request(t, "GET", "/api/v1/account/members/grant-member", "admin", "B", "", nil)
	require.Equal(t, 200, status, detail)
	version := detail["items"].([]any)[0].(map[string]any)["observedVersion"]
	f.mu.Lock()
	f.loseUpdate = true
	f.mu.Unlock()
	key := uuid.NewString()
	status, original := f.request(t, "POST", "/api/v1/account/members/grant-member/role", "admin", "B", key, map[string]any{"role": "listingkit_operator", "expectedVersion": version})
	require.Equal(t, 200, status, original)
	require.Equal(t, "unknown", original["status"])
	status, listing := f.request(t, "GET", "/api/v1/account/member-operations?limit=1", "admin", "B", "", nil)
	require.Equal(t, 200, status, listing)
	require.Len(t, listing["items"], 1)
	status, created := f.request(t, "POST", "/api/v1/account/member-invitations", "admin", "B", uuid.NewString(), map[string]any{"email": "recipient@example.test", "role": "listingkit_viewer"})
	require.Equal(t, 200, status, created)
	status, reread := f.request(t, "GET", "/api/v1/account/member-operations/"+key, "admin", "B", "", nil)
	require.Equal(t, 200, status, reread)
	require.Equal(t, "unknown", reread["status"])
	f.mu.Lock()
	require.Equal(t, 1, f.sends)
	f.revoked = true
	f.mu.Unlock()
	status, _ = f.request(t, "GET", "/api/v1/account/member-operations", "admin", "B", "", nil)
	require.Equal(t, 403, status)
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
	raw, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	err = json.Unmarshal(raw, &result)
	if err != nil {
		require.Equal(t, 404, response.StatusCode, string(raw))
		result = map[string]any{}
	}
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
	input := map[string]any{"email": "recipient@example.test", "role": "listingkit_viewer"}
	status, _ := f.request(t, "POST", "/api/v1/account/members/invitations", "admin", "B", key, input)
	require.Equal(t, 404, status)
	status, result := f.request(t, "POST", "/api/v1/account/member-invitations", "admin", "B", key, input)
	require.Equal(t, 200, status, result)
	inv := result["invitation"].(map[string]any)
	require.Equal(t, "pending", inv["state"])
	require.Equal(t, "mail_server_accepted", inv["deliveryState"])
	status, replay := f.request(t, "POST", "/api/v1/account/member-invitations", "admin", "B", key, input)
	require.Equal(t, 200, status, replay)
	require.Equal(t, result, replay)
	f.mu.Lock()
	require.Zero(t, f.sends)
	require.Empty(t, f.users)
	f.mu.Unlock()
	status, _ = f.request(t, "GET", "/api/v1/account/member-invitations/"+key, "admin", "C", "", nil)
	require.Equal(t, 404, status)
	status, _ = f.request(t, "GET", "/api/v1/account/invitations/"+key, "foreign", "", "", nil)
	require.Equal(t, 403, status)
	status, accepted := f.request(t, "POST", "/api/v1/account/invitations/"+key+"/accept", "recipient", "", "", nil)
	require.Equal(t, 200, status, accepted)
	require.Equal(t, "accepted", accepted["invitation"].(map[string]any)["state"])
	status, acceptedReplay := f.request(t, "POST", "/api/v1/account/invitations/"+key+"/accept", "recipient", "", "", nil)
	require.Equal(t, 200, status, acceptedReplay)
	grant := accepted["invitation"].(map[string]any)["authorizationId"].(string)
	status, detail := f.request(t, "GET", "/api/v1/account/members/"+grant, "admin", "B", "", nil)
	require.Equal(t, 200, status, detail)
	version := detail["items"].([]any)[0].(map[string]any)["observedVersion"]
	status, removed := f.request(t, "POST", "/api/v1/account/members/"+grant+"/remove", "admin", "B", uuid.NewString(), map[string]any{"expectedVersion": version})
	require.Equal(t, 200, status, removed)
	require.Equal(t, "acknowledged", removed["status"])
	f.mu.Lock()
	defer f.mu.Unlock()
	require.Equal(t, 2, f.sends)
	require.Empty(t, f.users, "formal invitations never create identities")
}
func membershipSMTP(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })
	_, port, _ := net.SplitHostPort(listener.Addr().String())
	number, _ := strconv.Atoi(port)
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
				fmt.Fprint(conn, "220 local ESMTP\r\n")
				reader := bufio.NewReader(conn)
				data := false
				for {
					line, err := reader.ReadString('\n')
					if err != nil {
						return
					}
					if data {
						if line == ".\r\n" {
							data = false
							fmt.Fprint(conn, "250 queued\r\n")
						}
						continue
					}
					switch {
					case strings.HasPrefix(line, "EHLO"):
						fmt.Fprint(conn, "250 local\r\n")
					case strings.HasPrefix(line, "DATA"):
						data = true
						fmt.Fprint(conn, "354 go\r\n")
					case strings.HasPrefix(line, "QUIT"):
						fmt.Fprint(conn, "221 bye\r\n")
						return
					default:
						fmt.Fprint(conn, "250 OK\r\n")
					}
				}
			}()
		}
	}()
	return number
}
