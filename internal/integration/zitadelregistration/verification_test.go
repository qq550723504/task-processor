package zitadelregistration

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"task-processor/internal/referral"
	"testing"
)

func TestReadCombinesOnlyMatchingFixedUserAndProof(t *testing.T) {
	for _, tc := range []struct {
		name, user, metadata string
		want                 error
	}{
		{"valid", `{"user":{"userId":"fixed-subject","details":{"resourceOwner":"signup"},"human":{"email":{"email":"new@example.test","isVerified":true}}}}`, `{"metadata":[{"key":"referral-registration-proof","value":"cHJvb2Y="}]}`, nil},
		{"projection late", `{"user":{"userId":"fixed-subject","details":{"resourceOwner":"signup"},"human":{"email":{"email":"new@example.test","isVerified":true}}}}`, `{"metadata":[]}`, referral.ErrUnknown},
		{"wrong subject", `{"user":{"userId":"other","details":{"resourceOwner":"signup"},"human":{"email":{"email":"new@example.test","isVerified":true}}}}`, `{"metadata":[{"key":"referral-registration-proof","value":"cHJvb2Y="}]}`, referral.ErrUnknown},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.Method + " " + r.URL.Path {
				case "GET /v2/users/fixed-subject":
					_, _ = w.Write([]byte(tc.user))
				case "POST /v2/users/fixed-subject/metadata/search":
					_, _ = w.Write([]byte(tc.metadata))
				default:
					t.Error("unexpected endpoint")
					w.WriteHeader(500)
				}
			}))
			defer server.Close()
			c, _ := New(Config{Origin: server.URL, Organization: "signup", HTTPClient: server.Client(), Token: func(context.Context) (string, error) { return "controlled-token", nil }})
			u, err := c.Read(context.Background(), "fixed-subject")
			if !errors.Is(err, tc.want) {
				t.Fatalf("got %v want %v", err, tc.want)
			}
			if err == nil && (u.Proof != "proof" || !u.Verified || u.Subject != "fixed-subject") {
				t.Fatalf("bad projection %+v", u)
			}
		})
	}
}

func TestResponseBoundAndRedirectFailClosed(t *testing.T) {
	for _, status := range []int{200, 302} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			requests := 0
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				w.Header().Set("Location", "/elsewhere")
				w.WriteHeader(status)
				_, _ = w.Write([]byte(strings.Repeat("x", 65537)))
			}))
			defer server.Close()
			c, _ := New(Config{Origin: server.URL, Organization: "signup", HTTPClient: server.Client(), Token: func(context.Context) (string, error) { return "token", nil }})
			if _, err := c.Read(context.Background(), "fixed-subject"); !errors.Is(err, referral.ErrUnknown) || requests != 1 {
				t.Fatalf("bound=%v requests=%d", err, requests)
			}
		})
	}
}
