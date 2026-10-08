package zitadelprovision

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestEnterpriseSlotInventoryRequiresCompletePagination(t *testing.T) {
	for _, total := range []int{105, 0} {
		t.Run(fmt.Sprint(total), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body struct {
					Query struct {
						Offset int `json:"offset"`
					} `json:"query"`
				}
				_ = json.NewDecoder(r.Body).Decode(&body)
				roles := []ProjectRole{}
				for i := body.Query.Offset; i < 105 && i < body.Query.Offset+100; i++ {
					roles = append(roles, ProjectRole{Key: fmt.Sprintf("role-%d", i)})
				}
				writeJSON(t, w, map[string]any{"details": map[string]any{"totalResult": fmt.Sprint(total)}, "result": roles})
			}))
			defer server.Close()
			c := client{baseURL: server.URL, token: "fixture", http: server.Client()}
			roles, err := c.listEnterpriseBootstrapRoles(context.Background(), "project")
			if total == 0 {
				if err == nil {
					t.Fatal("contradictory total accepted")
				}
			} else if err != nil || len(roles) != 105 {
				t.Fatalf("incomplete inventory %d %v", len(roles), err)
			}
		})
	}
}

func TestEnterpriseBootstrapNeverReplacesExistingGrant(t *testing.T) {
	for _, same := range []bool{true, false} {
		t.Run(fmt.Sprint(same), func(t *testing.T) {
			writes := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/zitadel.project.v2.ProjectService/ListProjectGrants" {
					writes++
					t.Errorf("unexpected native mutation %s", r.URL.Path)
					return
				}
				keys := []string{"listingkit_admin", "slot"}
				if !same {
					keys = []string{"existing-paid-role"}
				}
				writeJSON(t, w, map[string]any{"pagination": map[string]any{"totalResult": "1"}, "projectGrants": []any{map[string]any{"id": "grant", "projectId": "p", "grantedOrganizationId": "org", "state": "PROJECT_GRANT_STATE_ACTIVE", "grantedRoleKeys": keys}}})
			}))
			defer server.Close()
			c := client{baseURL: server.URL, token: "fixture", http: server.Client()}
			err := c.ensureInitialEnterpriseGrant(context.Background(), "p", "org", []string{"listingkit_admin", "slot"})
			if (err == nil) != same || writes != 0 {
				t.Fatalf("err=%v writes=%d", err, writes)
			}
		})
	}
}
