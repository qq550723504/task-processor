package zitadelprovision

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProvisionCreatesMissingRolesOnExistingProject(t *testing.T) {
	var createdRoles []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requireAuth(t, r)
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/management/v1/projects/_search":
			writeJSON(t, w, map[string]any{
				"result": []map[string]any{
					{"id": "project-1", "name": "ListingKit"},
				},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/management/v1/projects/project-1/roles/_search":
			writeJSON(t, w, map[string]any{
				"result": []map[string]any{
					{"key": "listingkit_viewer"},
				},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/management/v1/projects/project-1/roles":
			var body map[string]string
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode create role body: %v", err)
			}
			createdRoles = append(createdRoles, body["roleKey"])
			writeJSON(t, w, map[string]any{"id": body["roleKey"]})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	result, err := Provision(context.Background(), Config{
		IssuerURL:       server.URL,
		ManagementToken: "token",
		ProjectName:     "ListingKit",
	})
	if err != nil {
		t.Fatalf("Provision returned error: %v", err)
	}

	if result.ProjectID != "project-1" {
		t.Fatalf("ProjectID = %q, want project-1", result.ProjectID)
	}
	if strings.Join(createdRoles, ",") != "listingkit_operator,listingkit_admin,platform_admin" {
		t.Fatalf("created roles = %#v", createdRoles)
	}
	if len(result.Roles) != 4 {
		t.Fatalf("roles len = %d, want 4", len(result.Roles))
	}
	if !result.Roles[0].Existed || result.Roles[1].Existed {
		t.Fatalf("unexpected role statuses: %#v", result.Roles)
	}
	if !contains(result.RecommendedScopes, "urn:zitadel:iam:org:project:role:listingkit_admin") {
		t.Fatalf("recommended scopes missing listingkit_admin: %#v", result.RecommendedScopes)
	}
}

func TestProvisionFailsWhenProjectIsMissingAndCreateProjectIsFalse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/management/v1/projects/_search":
			writeJSON(t, w, map[string]any{"result": []any{}})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	_, err := Provision(context.Background(), Config{
		IssuerURL:       server.URL,
		ManagementToken: "token",
		ProjectName:     "ListingKit",
	})
	if err == nil || !strings.Contains(err.Error(), "project ListingKit not found") {
		t.Fatalf("error = %v, want missing project error", err)
	}
}

func TestProvisionCreatesProjectWhenEnabled(t *testing.T) {
	var createProjectBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/management/v1/projects/_search":
			writeJSON(t, w, map[string]any{"result": []any{}})
		case r.Method == http.MethodPost && r.URL.Path == "/management/v1/projects":
			if err := json.NewDecoder(r.Body).Decode(&createProjectBody); err != nil {
				t.Fatalf("decode create project body: %v", err)
			}
			writeJSON(t, w, map[string]any{"id": "new-project"})
		case r.Method == http.MethodPost && r.URL.Path == "/management/v1/projects/new-project/roles/_search":
			writeJSON(t, w, map[string]any{"result": []any{}})
		case r.Method == http.MethodPost && r.URL.Path == "/management/v1/projects/new-project/roles":
			writeJSON(t, w, map[string]any{"id": "role"})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	result, err := Provision(context.Background(), Config{
		IssuerURL:       server.URL,
		ManagementToken: "token",
		ProjectName:     "ListingKit",
		CreateProject:   true,
	})
	if err != nil {
		t.Fatalf("Provision returned error: %v", err)
	}
	if result.ProjectID != "new-project" {
		t.Fatalf("ProjectID = %q, want new-project", result.ProjectID)
	}
	if createProjectBody["name"] != "ListingKit" {
		t.Fatalf("create project name = %#v", createProjectBody["name"])
	}
	if createProjectBody["projectRoleAssertion"] != true {
		t.Fatalf("projectRoleAssertion = %#v, want true", createProjectBody["projectRoleAssertion"])
	}
	if createProjectBody["projectRoleCheck"] != true {
		t.Fatalf("projectRoleCheck = %#v, want true", createProjectBody["projectRoleCheck"])
	}
}

func requireAuth(t *testing.T, r *http.Request) {
	t.Helper()
	if got := r.Header.Get("Authorization"); got != "Bearer token" {
		t.Fatalf("Authorization = %q, want Bearer token", got)
	}
}

func writeJSON(t *testing.T, w http.ResponseWriter, value any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		t.Fatalf("write json: %v", err)
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
