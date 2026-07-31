package tenantdirectory

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientListsOrganizations(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/organizations/_search" || r.Method != http.MethodPost {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer token" {
			t.Fatalf("authorization = %q", r.Header.Get("Authorization"))
		}
		body, err := io.ReadAll(r.Body)
		if err != nil || string(body) != "{}" {
			t.Fatalf("body = %q, err = %v", string(body), err)
		}
		_, _ = w.Write([]byte(`{"result":[{"id":"373211199677923496","name":"Tenant A","primaryDomain":"tenant-a.example","state":"ORGANIZATION_STATE_ACTIVE"}]}`))
	}))
	defer server.Close()

	directory, err := NewClient(ClientConfig{IssuerURL: server.URL, Token: "token", HTTPClient: server.Client()})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	tenants, err := directory.List(context.Background())
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(tenants) != 1 || tenants[0].ID != "373211199677923496" || tenants[0].DisplayName != "Tenant A" {
		t.Fatalf("tenants = %#v", tenants)
	}
}
