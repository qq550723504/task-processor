//go:build integration

package zitadelregistration_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	pg "github.com/testcontainers/testcontainers-go/modules/postgres"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	app "task-processor/internal/app/referralregistration"
	"task-processor/internal/authidentity"
	persistence "task-processor/internal/integration/persistence/referral"
	adapter "task-processor/internal/integration/zitadelregistration"
	domain "task-processor/internal/referral"
)

// Drop a confirmed repository result at the application port. This models lost
// receipts after a real commit, not a PostgreSQL wire-level COMMIT fault.
type lostReceiptStore struct {
	domain.Store
	admission, consumption bool
}

func (s *lostReceiptStore) Admit(ctx context.Context, i domain.Intent, code string) (domain.Intent, error) {
	result, err := s.Store.Admit(ctx, i, code)
	if err == nil && s.admission {
		s.admission = false
		return domain.Intent{}, domain.ErrUnknown
	}
	return result, err
}
func (s *lostReceiptStore) Consume(ctx context.Context, i domain.Intent, now time.Time) (domain.Receipt, error) {
	result, err := s.Store.Consume(ctx, i, now)
	if err == nil && s.consumption {
		s.consumption = false
		return domain.Receipt{}, domain.ErrUnknown
	}
	return result, err
}

func TestOwnedPostgresAndControlledHTTPRecoverLostReceipts(t *testing.T) {
	ctx := context.Background()
	password := uuid.NewString()
	container, err := pg.Run(ctx, "postgres:16-alpine", pg.WithDatabase("referrals"), pg.WithUsername("owner"), pg.WithPassword(password), pg.BasicWaitStrategies())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := container.Terminate(context.Background()); err != nil {
			t.Error(err)
		}
	})
	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	if err = app.InstallSchema(ctx, dsn); err != nil {
		t.Fatalf("owned schema installation failed: %v", err)
	}
	open := func(dsn string) *gorm.DB {
		db, e := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
		if e != nil {
			t.Fatal("owned database open failed")
		}
		pool, e := db.DB()
		if e != nil {
			t.Fatal(e)
		}
		pool.SetMaxOpenConns(8)
		t.Cleanup(func() { _ = pool.Close() })
		return db
	}
	owner := open(dsn)
	if err = owner.Exec(`CREATE ROLE referral_runtime LOGIN PASSWORD '` + password + `'; REVOKE ALL ON SCHEMA public FROM PUBLIC; GRANT USAGE ON SCHEMA public TO referral_runtime; REVOKE CREATE,TEMP ON DATABASE referrals FROM PUBLIC; GRANT CONNECT ON DATABASE referrals TO referral_runtime; GRANT SELECT,INSERT ON ALL TABLES IN SCHEMA public TO referral_runtime; GRANT UPDATE(state,ciphertext,lease_until) ON public.registration_intents TO referral_runtime; GRANT UPDATE,DELETE ON public.registration_admission_buckets TO referral_runtime`).Error; err != nil {
		t.Fatal("owned runtime role setup failed")
	}
	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatal("owned URL invalid")
	}
	parsed.User = url.UserPassword("referral_runtime", password)
	repository, err := persistence.New(open(parsed.String()))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = repository.CreateCode(ctx, "issuer", "referrer", "code"); err != nil {
		t.Fatal(err)
	}
	store := &lostReceiptStore{Store: repository, admission: true, consumption: true}
	var mu sync.Mutex
	var subject, email, proof string
	creates := 0
	visible, verified := false, false
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if r.Header.Get("Authorization") != "Bearer controlled-service-token" {
			w.WriteHeader(401)
			return
		}
		switch {
		case r.Method == "POST" && r.URL.Path == "/v2/users/human":
			var body struct {
				UserID   string `json:"userId"`
				Email    struct{ Email string }
				Metadata []struct{ Key, Value string }
			}
			if json.NewDecoder(r.Body).Decode(&body) != nil || len(body.Metadata) != 1 {
				w.WriteHeader(400)
				return
			}
			creates++
			subject, email, proof = body.UserID, body.Email.Email, body.Metadata[0].Value
			w.WriteHeader(503) // Mutation happened; the create acknowledgement is unavailable.
		case r.Method == "GET":
			if subject == "" {
				w.WriteHeader(404)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"user": map[string]any{"userId": subject, "details": map[string]string{"resourceOwner": "signup"}, "human": map[string]any{"email": map[string]any{"email": email, "isVerified": verified}}}})
		case r.Method == "POST" && r.URL.Path == "/v2/users/"+subject+"/metadata/search":
			values := []map[string]string{}
			if visible {
				values = append(values, map[string]string{"key": "referral-registration-proof", "value": proof})
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"metadata": values})
		default:
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	provider, err := adapter.New(adapter.Config{Origin: server.URL, Organization: "signup", HTTPClient: server.Client(), Token: func(context.Context) (string, error) { return "controlled-service-token", nil }})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	service := func() *app.Service {
		return &app.Service{Store: store, Provider: provider, Issuer: "issuer", Instance: "instance", Organization: "signup", Now: func() time.Time { return now }, Keys: app.Keys{Active: "v1", Encryption: map[string][]byte{"v1": []byte("11111111111111111111111111111111")}, Proof: map[string][]byte{"v1": []byte("22222222222222222222222222222222")}, Lookup: []byte("33333333333333333333333333333333")}}
	}
	request := app.Request{Key: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", Code: "code", Email: "new@example.test", GivenName: "New", FamilyName: "User", ClientIP: "192.0.2.1"}
	if _, err = service().Start(ctx, request); !errors.Is(err, domain.ErrUnknown) {
		t.Fatalf("lost admission receipt=%v", err)
	}
	if creates != 0 {
		t.Fatal("unconfirmed admission called provider")
	}
	admission, err := service().Start(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	replay, err := service().Start(ctx, request)
	if err != nil || replay != admission {
		t.Fatal("restart did not recover original capability")
	}
	if err = service().Resume(ctx, admission.IntentID, admission.ResumeSecret); !errors.Is(err, domain.ErrUnknown) {
		t.Fatalf("late metadata must fail closed: %v", err)
	}
	mu.Lock()
	visible = true
	mu.Unlock()
	now = now.Add(16 * time.Second)
	if err = service().Resume(ctx, admission.IntentID, admission.ResumeSecret); err != nil {
		t.Fatal(err)
	}
	if creates != 1 || subject != admission.Subject {
		t.Fatal("UNKNOWN changed identity or recreated")
	}
	mu.Lock()
	verified = true
	mu.Unlock()
	authenticated := func() context.Context {
		return authidentity.WithAuthenticatedIdentity(ctx, authidentity.AuthenticatedIdentity{UserID: admission.Subject, TokenExpiresAt: now.Add(time.Hour)})
	}
	if _, err = service().Complete(authenticated()); !errors.Is(err, domain.ErrUnknown) {
		t.Fatalf("lost consumed receipt=%v", err)
	}
	server.Close()
	now = now.Add(25 * time.Hour)
	receipt, err := service().Complete(authenticated())
	if err != nil || receipt.Subject != admission.Subject {
		t.Fatalf("receipt replay after provider outage and expiry: %v", err)
	}
	projection, err := repository.Read(ctx, "issuer", "referrer")
	if err != nil || projection.Count != 1 {
		t.Fatalf("attribution count=%d err=%v", projection.Count, err)
	}
}
