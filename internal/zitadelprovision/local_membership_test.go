package zitadelprovision

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLocalMembershipWriterGrantsOnlyOrganizationAAndReadsBack(t *testing.T) {
	granted, writes, reads := false, 0, 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-pat" {
			t.Errorf("missing configured credential")
		}
		switch r.URL.Path {
		case "/v2/users/writer":
			writeJSON(t, w, map[string]any{"user": map[string]any{"userId": "writer", "username": "local-membership-write", "state": "USER_STATE_ACTIVE", "machine": map[string]any{"name": "Local membership writer"}, "details": map[string]string{"resourceOwner": "home"}}})
		case "/management/v1/orgs/me/members/_search":
			reads++
			if r.Header.Get("x-zitadel-orgid") != "org-a" {
				t.Errorf("wrong organization")
			}
			var query struct {
				Queries []struct {
					UserIDQuery struct {
						UserID string `json:"userId"`
					} `json:"userIdQuery"`
				} `json:"queries"`
			}
			if err := json.NewDecoder(r.Body).Decode(&query); err != nil || len(query.Queries) != 1 || query.Queries[0].UserIDQuery.UserID != "writer" {
				t.Errorf("missing exact user filter")
			}
			items := []any{}
			if granted {
				items = append(items, map[string]any{"userId": "writer", "userResourceOwner": "home", "details": map[string]string{"resourceOwner": "org-a"}, "roles": []string{"ORG_USER_MANAGER"}})
			}
			details := map[string]any{}
			if granted {
				details["totalResult"] = "1"
			}
			// Native v4.17.1 omits totalResult for the empty pre-grant read.
			writeJSON(t, w, map[string]any{"details": details, "result": items})
		case "/management/v1/orgs/me/members":
			writes++
			if r.Method != http.MethodPost || r.Header.Get("x-zitadel-orgid") != "org-a" {
				t.Errorf("unexpected mutation scope")
			}
			var body struct {
				UserID string   `json:"userId"`
				Roles  []string `json:"roles"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.UserID != "writer" || len(body.Roles) != 1 || body.Roles[0] != "ORG_USER_MANAGER" {
				t.Errorf("unexpected role mutation: %+v", body)
			}
			granted = true
			writeJSON(t, w, map[string]any{"details": map[string]string{"resourceOwner": "org-a"}})
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
			http.Error(w, "unexpected", 500)
		}
	}))
	defer server.Close()
	cfg := Config{IssuerURL: server.URL, ManagementToken: "test-pat", OrgID: "home", AcceptanceOrganizationIDs: []string{"org-a", "org-b"}, HTTPClient: server.Client()}
	for range 2 {
		if err := ProvisionLocalAcceptanceMembershipWriter(context.Background(), cfg, "writer"); err != nil {
			t.Fatal(err)
		}
	}
	if writes != 1 || reads < 3 {
		t.Fatalf("writes=%d reads=%d; want one grant and repeat read-back", writes, reads)
	}
}

func TestLocalMembershipWriterRejectsUnexpectedIdentityAndRoleWithoutMutation(t *testing.T) {
	for _, scenario := range []string{"wrong-user", "human", "invalid-machine", "wrong-home", "wrong-name", "inactive", "wrong-role", "wrong-org", "other-member", "other-member-home", "duplicate", "incomplete", "missing-envelope", "null-envelope", "unavailable"} {
		t.Run(scenario, func(t *testing.T) {
			writes := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/v2/users/writer":
					user := map[string]any{"userId": "writer", "username": "local-membership-write", "state": "USER_STATE_ACTIVE", "machine": map[string]any{}, "details": map[string]string{"resourceOwner": "home"}}
					switch scenario {
					case "wrong-user":
						user["userId"] = "other"
					case "human":
						delete(user, "machine")
					case "invalid-machine":
						user["machine"] = false
					case "wrong-home":
						user["details"] = map[string]string{"resourceOwner": "other"}
					case "wrong-name":
						user["username"] = "other"
					case "inactive":
						user["state"] = "USER_STATE_INACTIVE"
					}
					writeJSON(t, w, map[string]any{"user": user})
				case "/management/v1/orgs/me/members/_search":
					if scenario == "missing-envelope" {
						writeJSON(t, w, map[string]any{})
						return
					}
					if scenario == "null-envelope" {
						writeJSON(t, w, nil)
						return
					}
					member := map[string]any{"userId": "writer", "userResourceOwner": "home", "details": map[string]string{"resourceOwner": "org-a"}, "roles": []string{"ORG_USER_MANAGER"}}
					switch scenario {
					case "wrong-role":
						member["roles"] = []string{"ORG_USER_MANAGER", "ORG_OWNER"}
					case "wrong-org":
						member["details"] = map[string]string{"resourceOwner": "org-b"}
					case "other-member":
						member["userId"] = "other"
					case "other-member-home":
						member["userResourceOwner"] = "other"
					case "unavailable":
						http.Error(w, "secret provider body", http.StatusForbidden)
						return
					}
					items := []any{member}
					total := 1
					if scenario == "duplicate" {
						items = append(items, member)
						total = 2
					}
					if scenario == "incomplete" {
						total = 2
					}
					writeJSON(t, w, map[string]any{"details": map[string]int{"totalResult": total}, "result": items})
				default:
					writes++
					http.Error(w, "unexpected write", 500)
				}
			}))
			defer server.Close()
			cfg := Config{IssuerURL: server.URL, ManagementToken: "test-pat", OrgID: "home", AcceptanceOrganizationIDs: []string{"org-a", "org-b"}, HTTPClient: server.Client()}
			if err := ProvisionLocalAcceptanceMembershipWriter(context.Background(), cfg, "writer"); err == nil {
				t.Fatal("expected exact-scope validation failure")
			}
			if writes != 0 {
				t.Fatalf("unexpected mutations=%d", writes)
			}
		})
	}
}

func TestLocalMembershipWriterRejectsNonLocalAndInvalidScopeBeforeRequests(t *testing.T) {
	for _, cfg := range []Config{
		{IssuerURL: "https://example.com", ManagementToken: "pat", OrgID: "home", AcceptanceOrganizationIDs: []string{"a", "b"}},
		{IssuerURL: "http://localhost:1", ManagementToken: "pat", OrgID: "home", AcceptanceOrganizationIDs: []string{"home", "b"}},
		{IssuerURL: "http://localhost:1", ManagementToken: "pat", OrgID: "home", AcceptanceOrganizationIDs: []string{"a", "a"}},
		{IssuerURL: "http://localhost:1", ManagementToken: "pat", OrgID: "home"},
	} {
		cfg.HTTPClient = &http.Client{Transport: testRoundTripper(func(*http.Request) (*http.Response, error) {
			t.Error("invalid scope made an HTTP request")
			return nil, errors.New("unexpected request")
		})}
		if err := ProvisionLocalAcceptanceMembershipWriter(context.Background(), cfg, "writer"); err == nil {
			t.Fatal("expected local-scope rejection")
		}
	}
}
