package zitadelregistration

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	app "task-processor/internal/app/referralregistration"
	"task-processor/internal/referral"
	"testing"
)

func creation() app.Creation {
	return app.Creation{Subject: "fixed-subject", Organization: "signup", Email: "new@example.test", GivenName: "New", FamilyName: "User", Proof: "v1:immutable-proof"}
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
