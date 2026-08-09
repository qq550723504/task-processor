package memberinvite

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestZitadelProviderCreatesUserThenRoleAssignment(t *testing.T) {
	provider, requests := newZitadelProviderTestServer(t, http.StatusOK)
	got, err := provider.Invite(context.Background(), validInviteRequest())
	if err != nil {
		t.Fatal(err)
	}
	if got.UserID != "user-1" || got.AuthorizationID != "authorization-1" {
		t.Fatalf("invitation = %#v", got)
	}
	if len(*requests) != 2 {
		t.Fatalf("requests = %#v", *requests)
	}
	if (*requests)[0].Path != "/v2/users/human" || (*requests)[0].OrganizationID != "org-1" || !(*requests)[0].SendCode {
		t.Fatalf("create = %#v", (*requests)[0])
	}
	if (*requests)[1].Path != "/zitadel.authorization.v2.AuthorizationService/CreateAuthorization" || (*requests)[1].Role != "listingkit_viewer" {
		t.Fatalf("grant = %#v", (*requests)[1])
	}
}

func TestZitadelProviderMapsUserConflictWithoutReturningProviderBody(t *testing.T) {
	provider, _ := newZitadelProviderTestServer(t, http.StatusConflict)
	_, err := provider.Invite(context.Background(), validInviteRequest())
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("err = %v", err)
	}
}

func TestZitadelProviderPreservesUserIDWhenRoleAssignmentFails(t *testing.T) {
	provider, _ := newZitadelProviderTestServer(t, http.StatusForbidden)
	_, err := provider.Invite(context.Background(), validInviteRequest())
	var incomplete *IncompleteError
	if !errors.As(err, &incomplete) || incomplete.UserID != "user-1" {
		t.Fatalf("err = %#v", err)
	}
}

func TestNewZitadelProviderRejectsIncompleteConfiguration(t *testing.T) {
	if _, err := NewZitadelProvider(ZitadelConfig{}); err == nil {
		t.Fatal("NewZitadelProvider accepted an incomplete configuration")
	}
}

type capturedRequest struct {
	Path           string
	OrganizationID string
	Role           string
	SendCode       bool
}

func newZitadelProviderTestServer(t *testing.T, roleStatus int) (Provider, *[]capturedRequest) {
	t.Helper()
	requests := make([]capturedRequest, 0, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer token" {
			t.Fatalf("Authorization = %q", got)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Fatalf("Content-Type = %q", r.Header.Get("Content-Type"))
		}
		switch r.URL.Path {
		case "/v2/users/human":
			var body struct {
				Organization struct {
					OrganizationID string `json:"organizationId"`
				} `json:"organization"`
				Email struct {
					SendCode *struct{} `json:"sendCode"`
				} `json:"email"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode user request: %v", err)
			}
			requests = append(requests, capturedRequest{Path: r.URL.Path, OrganizationID: body.Organization.OrganizationID, SendCode: body.Email.SendCode != nil})
			writeMemberInviteJSON(t, w, http.StatusOK, map[string]string{"userId": "user-1"})
		case "/zitadel.authorization.v2.AuthorizationService/CreateAuthorization":
			var body struct {
				RoleKeys []string `json:"roleKeys"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode authorization request: %v", err)
			}
			role := ""
			if len(body.RoleKeys) == 1 {
				role = body.RoleKeys[0]
			}
			requests = append(requests, capturedRequest{Path: r.URL.Path, Role: role})
			if roleStatus < 200 || roleStatus >= 300 {
				writeMemberInviteJSON(t, w, roleStatus, map[string]string{"message": "provider-secret"})
				return
			}
			writeMemberInviteJSON(t, w, http.StatusOK, map[string]string{"id": "authorization-1"})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	t.Cleanup(server.Close)
	provider, err := NewZitadelProvider(ZitadelConfig{IssuerURL: server.URL, Token: "token", ProjectID: "project-1", HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	return provider, &requests
}

func writeMemberInviteJSON(t *testing.T, w http.ResponseWriter, status int, body any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		t.Fatalf("write response: %v", err)
	}
}
