//go:build issue357

package httpapi

import (
	"github.com/google/uuid"
	"testing"
)

func TestIssue357ConfigRejectsExternalAndWrongResources(t *testing.T) {
	c := issue357Config{RunID: uuid.NewString(), Issuer: "http://localhost:42101", IssuerPort: 42101, WebOrigin: "http://localhost:42102", GoPort: 42103, DatabasePort: 42104, DatabaseUser: "commercial_reader", DatabaseName: "issue357", DatabaseHost: "127.0.0.1"}
	if err := c.validate(); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*issue357Config){func(x *issue357Config) { x.Issuer = "https://real.example" }, func(x *issue357Config) { x.DatabaseHost = "shared-db" }, func(x *issue357Config) { x.DatabaseName = "customer" }, func(x *issue357Config) { x.DatabaseUser = "postgres" }, func(x *issue357Config) { x.DatabasePort = x.GoPort }, func(x *issue357Config) { x.RunID = "../other" }} {
		bad := c
		mutate(&bad)
		if bad.validate() == nil {
			t.Fatal("unsafe runtime config accepted")
		}
	}
}

func TestIssue357ConfigMatchesAssignedManifestBeforeConnecting(t *testing.T) {
	c := issue357Config{RunID: uuid.NewString(), Issuer: "http://localhost:42101", IssuerPort: 42101, WebOrigin: "http://localhost:42102", GoPort: 42103, DatabasePort: 42104, DatabaseUser: "commercial_reader", DatabaseName: "issue357", DatabaseHost: "127.0.0.1"}
	m := issue357Allocation{RunID: c.RunID, Origins: map[string]string{"issuer": c.Issuer, "web": c.WebOrigin, "go": "http://127.0.0.1:42103"}, Ports: map[string]int{"issuer": 42101, "web": 42102, "go": 42103, "database": 42104}}
	if err := c.validateAssigned(m); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*issue357Config){func(x *issue357Config) { x.DatabasePort = 42105 }, func(x *issue357Config) { x.IssuerPort = 42106; x.Issuer = "http://localhost:42106" }, func(x *issue357Config) { x.ProjectID = "other-project" }, func(x *issue357Config) { x.OrganizationB = "foreign-org" }, func(x *issue357Config) { x.WebOrigin = "http://localhost:42107" }} {
		bad := c
		mutate(&bad)
		if bad.validateAssigned(m) == nil {
			t.Fatal("other assigned resource accepted")
		}
	}
}
