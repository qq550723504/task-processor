package zitadelregistration

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	app "task-processor/internal/app/referralregistration"
	"task-processor/internal/referral"
	"testing"
	"time"
)

func creation() app.Creation {
	return app.Creation{Subject: "fixed-subject", Organization: "signup", Email: "new@example.test", GivenName: "New", FamilyName: "User", Proof: "v1:immutable-proof"}
}

type adapterIntentStore struct {
	referral.Store
	intent referral.Intent
}

func (s *adapterIntentStore) AllowIP(context.Context, string, time.Time) error { return nil }
func (s *adapterIntentStore) Cleanup(context.Context, time.Time) error         { return nil }
func (s *adapterIntentStore) Admit(_ context.Context, i referral.Intent, _ string) (referral.Intent, error) {
	i.Referrer = "referrer"
	s.intent = i
	return i, nil
}
func (s *adapterIntentStore) Find(context.Context, string, string, string) (referral.Intent, error) {
	return s.intent, nil
}
func (s *adapterIntentStore) Claim(context.Context, string) (time.Time, error) {
	return time.Now().Add(15 * time.Second), nil
}
func (s *adapterIntentStore) PermitCreate(context.Context, string, time.Time) (time.Duration, error) {
	return 15 * time.Second, nil
}

func TestPinnedFixedIDConflictReachesApplicationAfterFixedReadback(t *testing.T) {
	var mu sync.Mutex
	var paths []string
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		paths = append(paths, r.Method+" "+r.URL.Path)
		mu.Unlock()
		if r.Method == http.MethodPost {
			w.WriteHeader(400)
			_, _ = w.Write([]byte(`{"code":9,"details":[{"@type":"type.googleapis.com/zitadel.v1.ErrorDetail","id":"COMMAND-7yiox1isql"}]}`))
			return
		}
		w.WriteHeader(404)
	}))
	defer server.Close()
	provider, err := New(Config{Origin: server.URL, LoginOrigin: "https://login.example.test", Organization: "signup", HTTPClient: server.Client(), Token: func(context.Context) (string, error) { return "credential", nil }})
	if err != nil {
		t.Fatal(err)
	}
	store := &adapterIntentStore{}
	service := &app.Service{Store: store, Provider: provider, Issuer: "issuer", Instance: "instance", Organization: "signup", Now: time.Now, Keys: app.Keys{Active: "v1", Encryption: map[string][]byte{"v1": []byte("11111111111111111111111111111111")}, Proof: map[string][]byte{"v1": []byte("22222222222222222222222222222222")}, Lookup: []byte("33333333333333333333333333333333")}}
	admission, err := service.Start(context.Background(), app.Request{Key: strings.Repeat("a", 64), Code: "code", Email: "new@example.test", GivenName: "New", FamilyName: "User", ClientIP: "192.0.2.1"})
	if err != nil {
		t.Fatal(err)
	}
	if err = service.Resume(context.Background(), admission.IntentID, admission.ResumeSecret); !errors.Is(err, referral.ErrConflict) {
		t.Fatalf("wire conflict lost in application: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(paths) != 3 || paths[0] != "GET /v2/users/"+admission.Subject || paths[1] != "POST /v2/users/human" || paths[2] != paths[0] || store.intent.ID != admission.IntentID {
		t.Fatalf("unexpected recovery path: %v", paths)
	}
}
func TestCreateUsesPinnedCreateOnlyWire(t *testing.T) {
	calls := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != "POST" || r.URL.Path != "/v2/users/human" || r.Header.Get("Authorization") != "Bearer controlled-service-token" {
			t.Error("unexpected endpoint or credential")
		}
		var body map[string]json.RawMessage
		if json.NewDecoder(r.Body).Decode(&body) != nil {
			t.Fatal("invalid JSON")
		}
		for _, key := range []string{"userId", "organization", "profile", "email", "metadata"} {
			if body[key] == nil {
				t.Errorf("missing %s", key)
			}
		}
		if len(body) != 5 {
			t.Error("extra create fields, possibly local auth material")
		}
		var id string
		_ = json.Unmarshal(body["userId"], &id)
		if id != creation().Subject {
			t.Error("subject changed")
		}
		var email struct {
			Email    string
			SendCode *struct {
				URLTemplate string `json:"urlTemplate"`
			}
			IsVerified *bool
			ReturnCode *struct{}
		}
		_ = json.Unmarshal(body["email"], &email)
		if email.Email != creation().Email || email.SendCode == nil || email.IsVerified != nil || email.ReturnCode != nil {
			t.Error("must send official verification")
		}
		if email.SendCode == nil || email.SendCode.URLTemplate != "https://login.example.test/ui/v2/login/verify?code={{.Code}}&userId={{.UserID}}&organization={{.OrgID}}" {
			t.Error("must use exact server-configured official verification template")
		}
		var metadata []struct{ Key, Value string }
		_ = json.Unmarshal(body["metadata"], &metadata)
		if len(metadata) != 1 || metadata[0].Value != base64.StdEncoding.EncodeToString([]byte(creation().Proof)) {
			t.Error("missing atomic proof")
		}
		_, _ = w.Write([]byte(`{"userId":"fixed-subject"}`))
	}))
	defer server.Close()
	c, err := New(Config{LoginOrigin: "https://login.example.test", Origin: server.URL, Organization: "signup", HTTPClient: server.Client(), Token: func(context.Context) (string, error) { return "controlled-service-token", nil }})
	if err != nil {
		t.Fatal(err)
	}
	if err = c.Create(context.Background(), creation()); err != nil || calls != 1 {
		t.Fatalf("create=%v calls=%d", err, calls)
	}
}
func TestMissingCredentialDoesNotDispatch(t *testing.T) {
	calls := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { calls++ }))
	defer server.Close()
	c, _ := New(Config{LoginOrigin: "https://login.example.test", Origin: server.URL, Organization: "signup", HTTPClient: server.Client(), Token: func(context.Context) (string, error) { return "", errors.New("secret") }})
	if err := c.Create(context.Background(), creation()); !errors.Is(err, referral.ErrUnavailable) || strings.Contains(err.Error(), "secret") || calls != 0 {
		t.Fatalf("credential handling=%v calls=%d", err, calls)
	}
}
func TestInsecureOriginRejected(t *testing.T) {
	if _, err := New(Config{LoginOrigin: "https://login.example.test", Origin: "http://provider.test", Organization: "signup", Token: func(context.Context) (string, error) { return "credential", nil }}); !errors.Is(err, referral.ErrInvalid) {
		t.Fatalf("insecure config=%v", err)
	}
}

func TestOfficialLoginOriginRejectsRedirectsAndOversizeTemplate(t *testing.T) {
	for _, origin := range []string{"", "http://login.example.test", "https://login.example.test/redirect", "https://login.example.test?returnTo=elsewhere", "https://login.example.test#", "https://name:password@login.example.test", "https://login.example.test/%2F", "https://" + strings.Repeat("a", 63) + "." + strings.Repeat("b", 63) + ".test"} {
		if _, err := New(Config{Origin: "https://api.example.test", LoginOrigin: origin, Organization: "signup", Token: func(context.Context) (string, error) { return "credential", nil }}); !errors.Is(err, referral.ErrInvalid) {
			t.Errorf("accepted invalid official Login origin: %v", err)
		}
	}
}

func TestPinnedAlreadyExistsStatusIsConflict(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"code":6,"message":"controlled existing account"}`))
	}))
	defer server.Close()
	c, err := New(Config{Origin: server.URL, LoginOrigin: "https://login.example.test", Organization: "signup", HTTPClient: server.Client(), Token: func(context.Context) (string, error) { return "credential", nil }})
	if err != nil {
		t.Fatal(err)
	}
	if err = c.Create(context.Background(), creation()); !errors.Is(err, referral.ErrConflict) {
		t.Fatalf("pinned AlreadyExists mapping=%v", err)
	}
}

func TestPinnedFixedIDPreconditionUsesStructuredErrorIdentity(t *testing.T) {
	const fixed = `{"code":9,"message":"localized text is not an identity","details":[{"@type":"type.googleapis.com/zitadel.v1.ErrorDetail","id":"COMMAND-7yiox1isql","message":"arbitrary localized message"}]}`
	for _, tc := range []struct {
		name, body string
		status     int
		want       error
	}{
		{"pinned fixed ID", fixed, 400, referral.ErrConflict},
		{"other precondition", strings.ReplaceAll(fixed, "COMMAND-7yiox1isql", "COMMAND-other"), 400, referral.ErrUnknown},
		{"wrong detail type", strings.ReplaceAll(fixed, "zitadel.v1.ErrorDetail", "other.ErrorDetail"), 400, referral.ErrUnknown},
		{"wrong code", strings.ReplaceAll(fixed, `"code":9`, `"code":3`), 400, referral.ErrUnknown},
		{"wrong status", fixed, 500, referral.ErrUnknown},
		{"message only", `{"code":9,"message":"COMMAND-7yiox1isql Errors.User.AlreadyExisting"}`, 400, referral.ErrUnknown},
		{"malformed", `{"code":9,"details":`, 400, referral.ErrUnknown},
		{"wrong shape", `{"code":9,"details":{"id":"COMMAND-7yiox1isql"}}`, 400, referral.ErrUnknown},
		{"oversize", strings.ReplaceAll(fixed, "localized text is not an identity", strings.Repeat("x", 65536)), 400, referral.ErrUnknown},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()
			c, err := New(Config{Origin: server.URL, LoginOrigin: "https://login.example.test", Organization: "signup", HTTPClient: server.Client(), Token: func(context.Context) (string, error) { return "credential", nil }})
			if err != nil {
				t.Fatal(err)
			}
			if err = c.Create(context.Background(), creation()); !errors.Is(err, tc.want) {
				t.Fatalf("classification=%v want=%v", err, tc.want)
			}
			if _, err = c.Read(context.Background(), "fixed-subject"); !errors.Is(err, referral.ErrUnknown) {
				t.Fatalf("non-create error reclassified=%v", err)
			}
		})
	}
}
