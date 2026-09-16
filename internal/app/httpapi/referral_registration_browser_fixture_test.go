//go:build integration

package httpapi

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"task-processor/internal/app/runtime/currentapplication"
	sourceSchema "task-processor/internal/app/schema/sourceaccountregistry"
	"task-processor/internal/core/config"
	referralStore "task-processor/internal/integration/persistence/referral"
	"task-processor/internal/listingsubscription"
)

// This is a controlled Go transport/provider fixture, not a browser or proof of
// official mail, authenticator setup, OIDC/Auth.js, proxy, or real IAM grants.
type referralProviderFixture struct {
	mu            sync.Mutex
	users         map[string]map[string]any
	verified      bool
	down          bool
	loseCreate    bool
	creates       int
	token         string
	api, identity *httptest.Server
}

func newReferralProviderFixture(t *testing.T) *referralProviderFixture {
	t.Helper()
	f := &referralProviderFixture{users: map[string]map[string]any{}}
	f.api = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if r.Header.Get("Authorization") != "Bearer "+f.token {
			w.WriteHeader(401)
			return
		}
		if f.down {
			w.WriteHeader(503)
			return
		}
		if r.URL.Path == "/v2/users/human" && r.Method == "POST" {
			var body map[string]any
			if json.NewDecoder(r.Body).Decode(&body) != nil {
				w.WriteHeader(400)
				return
			}
			id, _ := body["userId"].(string)
			if id == "" {
				w.WriteHeader(400)
				return
			}
			if _, ok := f.users[id]; ok {
				w.WriteHeader(409)
				return
			}
			f.users[id] = body
			f.creates++
			if f.loseCreate {
				w.WriteHeader(503)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]string{"userId": id})
			return
		}
		id := strings.TrimPrefix(r.URL.Path, "/v2/users/")
		id = strings.TrimSuffix(id, "/metadata/search")
		u, ok := f.users[id]
		if !ok {
			w.WriteHeader(404)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/metadata/search") {
			_ = json.NewEncoder(w).Encode(map[string]any{"metadata": u["metadata"]})
			return
		}
		email := u["email"].(map[string]any)["email"]
		_ = json.NewEncoder(w).Encode(map[string]any{"user": map[string]any{"userId": id, "details": map[string]string{"resourceOwner": "fixture-signup"}, "human": map[string]any{"email": map[string]any{"email": email, "isVerified": f.verified}}}})
	}))
	f.identity = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			_ = json.NewEncoder(w).Encode(map[string]string{"issuer": f.identity.URL, "authorization_endpoint": f.identity.URL + "/authorize", "token_endpoint": f.identity.URL + "/token", "userinfo_endpoint": f.identity.URL + "/oidc/v1/userinfo", "introspection_endpoint": f.identity.URL + "/introspect"})
		case "/introspect":
			_ = r.ParseForm()
			token := r.Form.Get("token")
			subject := strings.TrimPrefix(token, "fixture-user:")
			f.mu.Lock()
			_, exists := f.users[subject]
			f.mu.Unlock()
			active := strings.HasPrefix(token, "fixture-user:") && (subject == "referrer" || subject == "other" || exists)
			_ = json.NewEncoder(w).Encode(map[string]any{"active": active, "sub": subject, "exp": time.Now().Add(time.Hour).Unix(), "urn:zitadel:iam:user:resourceowner:id": "fixture-signup"})
		case "/oidc/v1/userinfo":
			subject := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer fixture-user:")
			_ = json.NewEncoder(w).Encode(map[string]any{"sub": subject, "name": "Fixture", "email_verified": true})
		case "/zitadel.authorization.v2.AuthorizationService/ListAuthorizations":
			_ = json.NewEncoder(w).Encode(map[string]any{"authorizations": []any{}, "pagination": map[string]string{"totalResult": "0"}})
		default:
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(f.api.Close)
	t.Cleanup(f.identity.Close)
	return f
}

func referralBinaryConfig(t *testing.T, owner *gorm.DB, connection *config.DatabaseConfig, f *referralProviderFixture) *currentapplication.Config {
	t.Helper()
	ctx := context.Background()
	require.NoError(t, listingsubscription.AutoMigrateRepository(owner))
	require.NoError(t, sourceSchema.Migrate(ctx, owner))
	require.NoError(t, referralStore.Install(ctx, owner))
	require.NoError(t, owner.Exec(`REVOKE ALL ON SCHEMA public FROM PUBLIC; REVOKE CREATE,TEMP ON DATABASE issue347 FROM PUBLIC; REVOKE ALL ON ALL TABLES IN SCHEMA public FROM PUBLIC`).Error)
	cfg := &currentapplication.Config{SchemaVersion: 1, Listen: currentapplication.ListenConfig{Host: "127.0.0.1", Port: 18443}, Identity: currentapplication.IdentityConfig{IssuerURL: f.identity.URL, AuthorizationAPIURL: f.identity.URL, ClientID: "fixture-client", ClientSecret: "fixture-identity", ProjectID: "fixture-project"}}
	dbConfig := func(role string) currentapplication.DatabaseConfig {
		return currentapplication.DatabaseConfig{Host: "127.0.0.1", Port: connection.Port, User: role, Password: "fixture-password", Database: connection.Database, MaxConnections: 2}
	}
	for _, role := range []string{"source_account_runtime", "commercial_reader", "referral_runtime"} {
		require.NoError(t, owner.Exec(`CREATE ROLE `+role+` LOGIN PASSWORD 'fixture-password'; GRANT CONNECT ON DATABASE issue347 TO `+role+`; GRANT USAGE ON SCHEMA public TO `+role).Error)
		for table, privileges := range run1AllowedPrivileges[role] {
			require.NoError(t, owner.Exec(`GRANT `+strings.Join(privileges, ",")+` ON public.`+table+` TO `+role).Error)
		}
	}
	require.NoError(t, owner.Exec(`GRANT SELECT,INSERT ON public.referral_codes,public.registration_intents,public.referral_relations,public.referral_receipts,public.registration_admission_buckets TO referral_runtime; GRANT UPDATE(state,ciphertext,lease_until) ON public.registration_intents TO referral_runtime; GRANT UPDATE,DELETE ON public.registration_admission_buckets TO referral_runtime; ALTER ROLE commercial_reader SET default_transaction_read_only=on`).Error)
	cfg.SourceAccountDatabase = dbConfig("source_account_runtime")
	cfg.CommercialDatabase = dbConfig("commercial_reader")
	write := func(name string, b []byte) string {
		path := filepath.Join(t.TempDir(), name)
		require.NoError(t, os.WriteFile(path, b, 0600))
		return path
	}
	secret := func(name string) string {
		b := make([]byte, 32)
		_, err := rand.Read(b)
		require.NoError(t, err)
		return write(name, []byte(hex.EncodeToString(b)))
	}
	ca := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: f.api.Certificate().Raw})
	cfg.Referrals = currentapplication.ReferralsConfig{ReferralsConfig: config.ReferralsConfig{Enabled: true, Issuer: f.identity.URL, InstanceID: "fixture-instance", SignupOrganizationID: "fixture-signup", ProviderOrigin: f.api.URL, OfficialLoginOrigin: "https://login.example", PublicAppOrigin: "https://app.example", CredentialFile: secret("machine"), ServiceCredentialFile: secret("service"), LookupKeyFile: secret("lookup"), KeyID: "k1", ProofKeyFiles: map[string]string{"k1": secret("proof")}, EncryptionKeyFiles: map[string]string{"k1": secret("encryption")}, ProviderCAFile: write("provider-ca.pem", ca)}, Database: dbConfig("referral_runtime")}
	token, err := os.ReadFile(cfg.Referrals.CredentialFile)
	require.NoError(t, err)
	f.mu.Lock()
	f.token = string(token)
	f.mu.Unlock()
	return cfg
}

func TestReferralRegistrationNormalBinaryPostgres(t *testing.T) {
	owner, connection := commercialPostgres(t)
	f := newReferralProviderFixture(t)
	cfg := referralBinaryConfig(t, owner, connection, f)
	dir := t.TempDir()
	binary := filepath.Join(dir, "current-application.exe")
	buildCtx, buildCancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer buildCancel()
	build := exec.CommandContext(buildCtx, "go", "build", "-o", binary, "./cmd/current-application")
	build.Dir = filepath.Join("..", "..", "..")
	output, err := build.CombinedOutput()
	require.NoError(t, err, string(output))
	service, err := os.ReadFile(cfg.Referrals.ServiceCredentialFile)
	require.NoError(t, err)
	client := &http.Client{Timeout: 5 * time.Second}
	var base string
	request := func(method, path, token, body, key string) (int, map[string]any) {
		r, err := http.NewRequest(method, base+path, strings.NewReader(body))
		require.NoError(t, err)
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		if strings.HasPrefix(path, "/api/v1/referral-registration/") {
			r.Header.Set("X-Referral-Service-Credential", string(service))
			r.Header.Set("X-Referral-Client-IP", "203.0.113.7")
			r.Header.Set("Content-Type", "application/json")
		}
		if key != "" {
			r.Header.Set("Idempotency-Key", key)
		}
		response, err := client.Do(r)
		require.NoError(t, err)
		defer response.Body.Close()
		data, err := io.ReadAll(response.Body)
		require.NoError(t, err)
		var out map[string]any
		_ = json.Unmarshal(data, &out)
		return response.StatusCode, out
	}
	run := func(rejected bool, exercise func()) {
		t.Helper()
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		cfg.Listen.Port = listener.Addr().(*net.TCPAddr).Port
		require.NoError(t, listener.Close())
		data, err := json.Marshal(cfg)
		require.NoError(t, err)
		manifest := filepath.Join(dir, "runtime.json")
		require.NoError(t, os.WriteFile(manifest, data, 0600))
		shutdown := filepath.Join(dir, fmt.Sprintf("stop-%d", cfg.Listen.Port))
		ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
		defer cancel()
		command := exec.CommandContext(ctx, binary, "-config", manifest, "-shutdown-file", shutdown)
		var logs bytes.Buffer
		command.Stdout = &logs
		command.Stderr = &logs
		require.NoError(t, command.Start())
		done := make(chan error, 1)
		go func() { done <- command.Wait() }()
		base = "http://" + cfg.ListenAddress()
		if rejected {
			require.Error(t, <-done)
			require.NoError(t, ctx.Err(), "startup did not reject promptly")
		} else {
			require.Eventually(t, func() bool {
				res, e := client.Get(base + "/api/v1/account/profile")
				if e != nil {
					return false
				}
				defer res.Body.Close()
				return res.StatusCode == 401
			}, 12*time.Second, 50*time.Millisecond)
			exercise()
			require.NoError(t, os.WriteFile(shutdown, []byte("stop"), 0600))
			require.NoError(t, <-done, logs.String())
		}
		require.NotContains(t, logs.String(), string(service))
		require.NotContains(t, logs.String(), "fixture-password")
		conn, e := net.DialTimeout("tcp", cfg.ListenAddress(), 100*time.Millisecond)
		if conn != nil {
			_ = conn.Close()
		}
		require.Error(t, e)
		var active int64
		require.NoError(t, owner.Raw(`SELECT count(*) FROM pg_stat_activity WHERE usename IN ('source_account_runtime','commercial_reader','referral_runtime')`).Scan(&active).Error)
		require.Zero(t, active, "binary left database sessions")
	}
	var admission map[string]any
	var payload, subject string
	key := strings.Repeat("b", 64)
	run(false, func() {
		before := run1PermissionFacts(t, owner, []string{"referral_codes", "registration_intents", "referral_relations", "referral_receipts", "registration_admission_buckets"})
		status, out := request("GET", accountReferralsPath, "fixture-user:referrer", "", "")
		require.Equal(t, 200, status, out)
		require.Equal(t, "not_created", out["codeAvailability"])
		require.Equal(t, float64(0), out["count"])
		require.Equal(t, before, run1PermissionFacts(t, owner, []string{"referral_codes", "registration_intents", "referral_relations", "referral_receipts", "registration_admission_buckets"}))
		status, out = request("POST", accountReferralsPath, "fixture-user:referrer", "", "")
		require.Equal(t, 200, status, out)
		payload = fmt.Sprintf(`{"code":%q,"email":"new@example.com","givenName":"New","familyName":"Person"}`, out["code"])
		status, admission = request("POST", referralIntentsPath, "", payload, key)
		require.Equal(t, 200, status, admission)
		status, replayed := request("POST", referralIntentsPath, "", payload, key)
		require.Equal(t, 200, status, replayed)
		require.Equal(t, admission, replayed)
		status, _ = request("POST", referralIntentsPath, "", strings.Replace(payload, "New", "Different", 1), key)
		require.Equal(t, 409, status)
		require.NoError(t, owner.Raw(`SELECT subject FROM public.registration_intents WHERE id=?`, admission["intentID"]).Scan(&subject).Error)
		f.mu.Lock()
		require.Zero(t, f.creates)
		f.loseCreate = true
		f.mu.Unlock()
	})
	// Restart loses all process memory. The durable header key returns the same
	// admission, then a lost provider response resolves by fixed-ID readback.
	run(false, func() {
		status, replayed := request("POST", referralIntentsPath, "", payload, key)
		require.Equal(t, 200, status, replayed)
		require.Equal(t, admission, replayed)
		resume, _ := json.Marshal(map[string]any{"intentID": admission["intentID"], "resumeSecret": admission["resumeSecret"]})
		status, out := request("POST", referralResumePath, "", string(resume), "")
		require.Equal(t, 200, status, out)
		status, out = request("POST", accountReferralsCompletePath, "fixture-user:"+subject, "", "")
		require.Equal(t, 409, status, out)
		status, _ = request("POST", accountReferralsCompletePath, "fixture-user:other", "", "")
		require.Equal(t, 404, status)
		f.mu.Lock()
		f.verified = true
		require.Equal(t, 1, f.creates)
		f.mu.Unlock()
		completed := make(chan error, 4)
		for i := 0; i < 4; i++ {
			go func() {
				r, _ := http.NewRequest("POST", base+accountReferralsCompletePath, nil)
				r.Header.Set("Authorization", "Bearer fixture-user:"+subject)
				res, e := client.Do(r)
				if e != nil {
					completed <- e
					return
				}
				defer res.Body.Close()
				if res.StatusCode != 200 {
					completed <- fmt.Errorf("concurrent completion status %d", res.StatusCode)
					return
				}
				completed <- nil
			}()
		}
		for i := 0; i < 4; i++ {
			require.NoError(t, <-completed)
		}
		status, out = request("POST", accountReferralsCompletePath, "fixture-user:"+subject, "", "")
		require.Equal(t, 200, status, out)
		f.mu.Lock()
		f.down = true
		f.mu.Unlock()
		status, replayed = request("POST", accountReferralsCompletePath, "fixture-user:"+subject, "", "")
		require.Equal(t, 200, status, replayed)
		require.Equal(t, out, replayed)
		status, out = request("GET", accountReferralsPath, "fixture-user:referrer", "", "")
		require.Equal(t, 200, status, out)
		require.Equal(t, float64(1), out["count"])
		status, _ = request("GET", accountReferralsPath, string(service), "", "")
		require.Equal(t, 401, status)
		var facts struct{ Relations, Receipts, Consumed int64 }
		require.NoError(t, owner.Raw(`SELECT (SELECT count(*) FROM referral_relations) AS relations,(SELECT count(*) FROM referral_receipts) AS receipts,(SELECT count(*) FROM registration_intents WHERE state='CONSUMED' AND ciphertext IS NULL) AS consumed`).Scan(&facts).Error)
		require.EqualValues(t, 1, facts.Relations)
		require.EqualValues(t, 1, facts.Receipts)
		require.EqualValues(t, 1, facts.Consumed)
	})
	cfg.Referrals.Enabled = false
	run(false, func() {
		status, _ := request("GET", accountReferralsPath, "fixture-user:referrer", "", "")
		require.Equal(t, 404, status)
		var n int64
		require.NoError(t, owner.Raw(`SELECT count(*) FROM pg_stat_activity WHERE usename='referral_runtime'`).Scan(&n).Error)
		require.Zero(t, n)
	})
	cfg.Referrals.Enabled = true
	require.NoError(t, owner.Exec(`GRANT UPDATE ON public.referral_relations TO referral_runtime`).Error)
	run(true, nil)
	require.NoError(t, owner.Exec(`REVOKE UPDATE ON public.referral_relations FROM referral_runtime`).Error)
	require.NoError(t, owner.Exec(`DROP INDEX public.registration_intents_payload_expiry_idx`).Error)
	run(true, nil)
	require.NoError(t, owner.Exec(`CREATE INDEX registration_intents_payload_expiry_idx ON public.registration_intents(completion_expires_at) WHERE ciphertext IS NOT NULL; ALTER TABLE public.referral_relations DROP CONSTRAINT referral_relations_pkey CASCADE`).Error)
	run(true, nil)
	t.Log("normal binary PASS: mounted self/admit/restart/resume/verified complete/receipt replay, immutable facts, disabled no pool, permission and missing-index rejection, every process/listener/pool closed; controlled provider only")
}
