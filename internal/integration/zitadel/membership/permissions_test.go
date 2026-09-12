package membership

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	domain "task-processor/internal/organization/membership"
)

func authorizedServer(handler http.Handler) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/auth/v1/permissions/zitadel/me/_search" {
			fmt.Fprint(w, `{"result":["user.grant.read","user.read"]}`)
			return
		}
		handler.ServeHTTP(w, r)
	}))
}

func TestFilteredEmptyDirectoryCannotMasqueradeAsAuthorizedEmpty(t *testing.T) {
	for _, permissions := range []string{`[]`, `["user.grant.read:other-project"]`, `["user.grant.read:project"]`, `["user.grant.read:any-grant"]`} {
		t.Run(permissions, func(t *testing.T) {
			listCalls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/auth/v1/permissions/zitadel/me/_search" {
					if r.Header.Get("x-zitadel-orgid") != "org" || r.Header.Get("Authorization") != "Bearer read" {
						t.Error("unscoped permission read")
					}
					fmt.Fprintf(w, `{"result":%s}`, permissions)
					return
				}
				listCalls++
				fmt.Fprint(w, `{"pagination":{}}`)
			}))
			defer server.Close()
			client, _ := NewClient(server.URL, "read", "project", server.Client())
			page, err := client.List(context.Background(), "org", domain.PageRequest{Limit: 20})
			if err == nil || listCalls != 0 || len(page.Items) != 0 {
				t.Fatalf("filtered-empty accepted: %+v %v listcalls=%d", page, err, listCalls)
			}
		})
	}
}

func TestDirectoryPermissionRevocationAfterReadDiscardsData(t *testing.T) {
	probes := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/auth/v1/permissions/zitadel/me/_search" {
			probes++
			if probes == 1 {
				fmt.Fprint(w, `{"result":["user.grant.read"]}`)
			} else {
				fmt.Fprint(w, `{"result":[]}`)
			}
			return
		}
		fmt.Fprint(w, `{"pagination":{}}`)
	}))
	defer server.Close()
	client, _ := NewClient(server.URL, "read", "project", server.Client())
	if _, err := client.List(context.Background(), "org", domain.PageRequest{Limit: 20}); err == nil || probes != 2 {
		t.Fatalf("revoked read accepted err=%v probes=%d", err, probes)
	}
}
