package httpapi

import (
	"net/http"
	"testing"

	"task-processor/internal/authz"
	"task-processor/internal/httproute"
	kernelmodule "task-processor/internal/kernel/module"
	"task-processor/internal/sourceaccountregistry"
)

func TestModuleRegistersExactSourceAccountRoutes(t *testing.T) {
	handler := mustHandler(t, &fakeService{})
	registry := kernelmodule.NewRegistry()
	if err := NewModule(handler).Register(registry); err != nil {
		t.Fatal(err)
	}
	want := []struct {
		method     string
		path       string
		permission string
		policy     httproute.OrganizationAccessPolicy
	}{
		{http.MethodPost, "/api/v1/workbench/source-accounts", authz.PermissionWorkbenchSourceAccountManage, httproute.OrganizationAccessPolicyLiveWrite},
		{http.MethodGet, "/api/v1/workbench/source-accounts", authz.PermissionWorkbenchSourceAccountRead, httproute.OrganizationAccessPolicyCachedRead},
		{http.MethodGet, "/api/v1/workbench/source-accounts/:source_account_id", authz.PermissionWorkbenchSourceAccountRead, httproute.OrganizationAccessPolicyCachedRead},
		{http.MethodPost, "/api/v1/workbench/source-accounts/:source_account_id/disable", authz.PermissionWorkbenchSourceAccountManage, httproute.OrganizationAccessPolicyLiveWrite},
		{http.MethodPost, "/api/v1/workbench/source-accounts/:source_account_id/enable", authz.PermissionWorkbenchSourceAccountManage, httproute.OrganizationAccessPolicyLiveWrite},
	}
	routes := registry.Routes()
	if len(routes) != len(want) {
		t.Fatalf("route count = %d, want %d", len(routes), len(want))
	}
	for index, expected := range want {
		got := routes[index]
		if got.Method != expected.method || got.Path != expected.path || got.Permission != expected.permission || got.OrganizationAccessPolicy != expected.policy || got.AuthPolicy != httproute.AuthPolicyVerifiedIdentity || got.Module != ModuleName || got.RequestTimeout != sourceaccountregistry.Timeout || got.Handler == nil {
			t.Fatalf("route[%d] = %#v, want %#v", index, got, expected)
		}
	}
}
