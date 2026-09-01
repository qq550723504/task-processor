package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	"task-processor/internal/core/config"
	"task-processor/internal/httproute"
	kernelmodule "task-processor/internal/kernel/module"
	"task-processor/internal/storecenter"
)

const (
	testStoreID      = "11111111-1111-4111-8111-111111111111"
	testCreateKey    = "22222222-2222-4222-8222-222222222222"
	testDeleteKey    = "33333333-3333-4333-8333-333333333333"
	testAllocationID = "44444444-4444-4444-8444-444444444444"
)

type storeServiceStub struct {
	listResult     storecenter.ListStoresResult
	createResult   storecenter.CreateStoreResult
	getResult      storecenter.StoreProjection
	mutationResult storecenter.StoreMutationResult
	deleteResult   storecenter.DeleteStoreResult
	err            error
	calls          []string
	listRequest    storecenter.ListStoresRequest
	createRequest  storecenter.CreateStoreRequest
	getRequest     storecenter.GetStoreRequest
	updateRequest  storecenter.UpdateStoreRequest
	disableRequest storecenter.StoreLifecycleRequest
	enableRequest  storecenter.StoreLifecycleRequest
	deleteRequest  storecenter.DeleteStoreRequest
}

func (s *storeServiceStub) List(_ context.Context, request storecenter.ListStoresRequest) (storecenter.ListStoresResult, error) {
	s.calls = append(s.calls, "list")
	s.listRequest = request
	return s.listResult, s.err
}
func (s *storeServiceStub) Create(_ context.Context, request storecenter.CreateStoreRequest) (storecenter.CreateStoreResult, error) {
	s.calls = append(s.calls, "create")
	s.createRequest = request
	return s.createResult, s.err
}
func (s *storeServiceStub) Get(_ context.Context, request storecenter.GetStoreRequest) (storecenter.StoreProjection, error) {
	s.calls = append(s.calls, "get")
	s.getRequest = request
	return s.getResult, s.err
}
func (s *storeServiceStub) Update(_ context.Context, request storecenter.UpdateStoreRequest) (storecenter.StoreMutationResult, error) {
	s.calls = append(s.calls, "update")
	s.updateRequest = request
	return s.mutationResult, s.err
}
func (s *storeServiceStub) Disable(_ context.Context, request storecenter.StoreLifecycleRequest) (storecenter.StoreMutationResult, error) {
	s.calls = append(s.calls, "disable")
	s.disableRequest = request
	return s.mutationResult, s.err
}
func (s *storeServiceStub) Enable(_ context.Context, request storecenter.StoreLifecycleRequest) (storecenter.StoreMutationResult, error) {
	s.calls = append(s.calls, "enable")
	s.enableRequest = request
	return s.mutationResult, s.err
}
func (s *storeServiceStub) Delete(_ context.Context, request storecenter.DeleteStoreRequest) (storecenter.DeleteStoreResult, error) {
	s.calls = append(s.calls, "delete")
	s.deleteRequest = request
	return s.deleteResult, s.err
}

func TestModuleRegistersExactScopedStoreRoutes(t *testing.T) {
	handler := mustHandler(t, newStoreServiceStub(t))
	module := NewModule(handler)
	require.Equal(t, "store-center", module.Name())
	require.False(t, module.Enabled(nil))
	require.False(t, module.Enabled(&config.Config{}))
	require.True(t, module.Enabled(&config.Config{Workbench: config.WorkbenchConfig{Enabled: true}}))
	require.False(t, NewModule(nil).Enabled(&config.Config{Workbench: config.WorkbenchConfig{Enabled: true}}))

	registry := kernelmodule.NewRegistry()
	require.NoError(t, module.Register(registry))
	routes := registry.Routes()
	require.Len(t, routes, 7)
	want := []struct {
		method, path, permission string
		access                   httproute.OrganizationAccessPolicy
	}{
		{http.MethodGet, "/api/v1/workbench/stores", authz.PermissionWorkbenchStoreRead, httproute.OrganizationAccessPolicyCachedRead},
		{http.MethodPost, "/api/v1/workbench/stores", authz.PermissionWorkbenchStoreCreate, httproute.OrganizationAccessPolicyLiveWrite},
		{http.MethodGet, "/api/v1/workbench/stores/:store_id", authz.PermissionWorkbenchStoreRead, httproute.OrganizationAccessPolicyCachedRead},
		{http.MethodPut, "/api/v1/workbench/stores/:store_id", authz.PermissionWorkbenchStoreUpdate, httproute.OrganizationAccessPolicyLiveWrite},
		{http.MethodPost, "/api/v1/workbench/stores/:store_id/disable", authz.PermissionWorkbenchStoreLifecycle, httproute.OrganizationAccessPolicyLiveWrite},
		{http.MethodPost, "/api/v1/workbench/stores/:store_id/enable", authz.PermissionWorkbenchStoreLifecycle, httproute.OrganizationAccessPolicyLiveWrite},
		{http.MethodDelete, "/api/v1/workbench/stores/:store_id", authz.PermissionWorkbenchStoreDelete, httproute.OrganizationAccessPolicyLiveWrite},
	}
	for index, expected := range want {
		route := routes[index]
		require.Equal(t, expected.method, route.Method)
		require.Equal(t, expected.path, route.Path)
		require.Equal(t, "store-center", route.Module)
		require.Equal(t, expected.permission, route.Permission)
		require.Equal(t, httproute.AuthPolicyVerifiedIdentity, route.AuthPolicy)
		require.Equal(t, expected.access, route.OrganizationAccessPolicy)
		require.Nil(t, route.OrganizationTargetResolver)
		require.NotNil(t, route.Handler)
	}
}

func TestHandlerDerivesEveryServiceRequestOnlyFromEffectiveOrganizationIdentity(t *testing.T) {
	service := newStoreServiceStub(t)
	service.listResult.Page = 2
	service.listResult.PageSize = 5
	handler := mustHandler(t, service)
	router := mountedRouter(t, handler)
	identity := authidentity.AuthenticatedIdentity{
		UserID: " user-1 ", TenantID: "tenant-spoof", HomeOrganizationID: "org-home",
		EffectiveOrganizationID: " org-effective ", Roles: []string{"listingkit_viewer"},
		OrganizationGrants: []authidentity.OrganizationGrant{{OrganizationID: "org-a", Roles: []string{"listingkit_admin"}}},
	}
	tests := []struct {
		method, path, body string
		headers            map[string][]string
	}{
		{http.MethodGet, "/api/v1/workbench/stores?page=2&pageSize=5&platform=shein&status=active", "", nil},
		{http.MethodPost, "/api/v1/workbench/stores", `{"name":" Store ","platform":"shein","region":" SG ","externalStoreId":" ext "}`, map[string][]string{"Idempotency-Key": {testCreateKey}}},
		{http.MethodGet, "/api/v1/workbench/stores/" + testStoreID, "", nil},
		{http.MethodPut, "/api/v1/workbench/stores/" + testStoreID, `{"name":" Renamed ","region":" US "}`, map[string][]string{"If-Match": {`"2"`}}},
		{http.MethodPost, "/api/v1/workbench/stores/" + testStoreID + "/disable", "", map[string][]string{"If-Match": {`"2"`}}},
		{http.MethodPost, "/api/v1/workbench/stores/" + testStoreID + "/enable", "", map[string][]string{"If-Match": {`"3"`}}},
		{http.MethodDelete, "/api/v1/workbench/stores/" + testStoreID, "", map[string][]string{"If-Match": {`"4"`}, "Idempotency-Key": {testDeleteKey}}},
	}
	for _, tt := range tests {
		response := serve(t, router, identity, tt.method, tt.path, tt.body, tt.headers)
		require.Less(t, response.Code, 300, "%s %s: %s", tt.method, tt.path, response.Body.String())
	}
	require.Equal(t, "org-effective", service.listRequest.OrganizationID)
	require.Equal(t, storecenter.ListStoresRequest{OrganizationID: "org-effective", Page: 2, PageSize: 5, Platform: "shein", Status: storecenter.StoreStatusActive}, service.listRequest)
	require.Equal(t, "org-effective", service.createRequest.OrganizationID)
	require.Equal(t, "user-1", service.createRequest.ActorSubject)
	require.Equal(t, "Store", service.createRequest.Name)
	require.Equal(t, "SG", service.createRequest.Region)
	require.Equal(t, "ext", service.createRequest.ExternalStoreID)
	require.Equal(t, testCreateKey, service.createRequest.IdempotencyKey)
	require.Equal(t, storecenter.GetStoreRequest{OrganizationID: "org-effective", StoreID: testStoreID}, service.getRequest)
	require.Equal(t, "org-effective", service.updateRequest.OrganizationID)
	require.Equal(t, "user-1", service.updateRequest.ActorSubject)
	require.Equal(t, int64(2), service.updateRequest.ExpectedVersion)
	require.Equal(t, "org-effective", service.disableRequest.OrganizationID)
	require.Equal(t, int64(2), service.disableRequest.ExpectedVersion)
	require.Equal(t, "org-effective", service.enableRequest.OrganizationID)
	require.Equal(t, int64(3), service.enableRequest.ExpectedVersion)
	require.Equal(t, storecenter.DeleteStoreRequest{OrganizationID: "org-effective", ActorSubject: "user-1", StoreID: testStoreID, ExpectedVersion: 4, OperationKey: testDeleteKey}, service.deleteRequest)
}

func TestHandlerFailsClosedWithoutAuthoritativeIdentity(t *testing.T) {
	handler := mustHandler(t, newStoreServiceStub(t))
	router := mountedRouter(t, handler)
	tests := []struct {
		name     string
		identity authidentity.AuthenticatedIdentity
		status   int
		code     string
	}{
		{name: "missing identity", status: http.StatusUnauthorized, code: "AUTHENTICATION_REQUIRED"},
		{name: "missing organization", identity: authidentity.AuthenticatedIdentity{UserID: "user-1", TenantID: "legacy-org", HomeOrganizationID: "home-org"}, status: http.StatusConflict, code: "ORGANIZATION_SELECTION_REQUIRED"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := serve(t, router, tt.identity, http.MethodGet, "/api/v1/workbench/stores", "", nil)
			require.Equal(t, tt.status, response.Code)
			assertProtocolError(t, response, tt.code, nil)
		})
	}
}

func TestHandlerRejectsOrganizationSpoofInputsWithoutCallingService(t *testing.T) {
	identity := validIdentity()
	tests := []struct {
		name, method, path, body string
		headers                  map[string][]string
	}{
		{name: "query organization", method: http.MethodGet, path: "/api/v1/workbench/stores?organizationId=org-spoof"},
		{name: "query tenant", method: http.MethodGet, path: "/api/v1/workbench/stores?tenantId=org-spoof"},
		{name: "body organization", method: http.MethodPost, path: "/api/v1/workbench/stores", body: `{"name":"Store","platform":"shein","region":"SG","organizationId":"org-spoof"}`, headers: map[string][]string{"Idempotency-Key": {testCreateKey}}},
		{name: "body tenant", method: http.MethodPut, path: "/api/v1/workbench/stores/" + testStoreID, body: `{"name":"Store","region":"SG","tenantId":"org-spoof"}`, headers: map[string][]string{"If-Match": {`"2"`}}},
		{name: "create query", method: http.MethodPost, path: "/api/v1/workbench/stores?scope=org-spoof", body: `{"name":"Store","platform":"shein","region":"SG"}`, headers: map[string][]string{"Idempotency-Key": {testCreateKey}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := newStoreServiceStub(t)
			handler := mustHandler(t, service)
			router := mountedRouter(t, handler)
			response := serve(t, router, identity, tt.method, tt.path, tt.body, tt.headers)
			response.Header().Set("X-Tenant-ID", "org-spoof")
			require.Equal(t, http.StatusBadRequest, response.Code)
			assertProtocolError(t, response, "INVALID_REQUEST", nil)
			require.Empty(t, service.calls)
		})
	}

	service := newStoreServiceStub(t)
	handler := mustHandler(t, service)
	router := mountedRouter(t, handler)
	response := serve(t, router, identity, http.MethodGet, "/api/v1/workbench/stores", "", map[string][]string{
		"X-Tenant-ID": {"org-spoof"}, "Tenant-ID": {"org-spoof-2"}, "X-Organization-ID": {"org-spoof-3"},
	})
	require.Equal(t, http.StatusOK, response.Code)
	require.Equal(t, "org-effective", service.listRequest.OrganizationID)
}

func TestHandlerStrictlyRejectsMalformedCollectionQueriesAndPaths(t *testing.T) {
	tests := []string{
		"?page=01", "?page=0", "?page=-1", "?page=1&page=2", "?pageSize=0", "?pageSize=101", "?pageSize=20&pageSize=20",
		fmt.Sprintf("?page=%d&pageSize=100", int64(^uint(0)>>1)),
		"?platform=amazon", "?status=deleted", "?unknown=x",
	}
	for _, query := range tests {
		t.Run(query, func(t *testing.T) {
			service := newStoreServiceStub(t)
			router := mountedRouter(t, mustHandler(t, service))
			response := serve(t, router, validIdentity(), http.MethodGet, "/api/v1/workbench/stores"+query, "", nil)
			require.Equal(t, http.StatusBadRequest, response.Code)
			require.Empty(t, service.calls)
		})
	}
	for _, storeID := range []string{uuid.NewString()[0:35], "AAAAAAAA-AAAA-4AAA-8AAA-AAAAAAAAAAAA", "00000000-0000-0000-0000-000000000000", "11111111-1111-0111-8111-111111111111", testStoreID + "?page=1"} {
		service := newStoreServiceStub(t)
		router := mountedRouter(t, mustHandler(t, service))
		response := serve(t, router, validIdentity(), http.MethodGet, "/api/v1/workbench/stores/"+storeID, "", nil)
		require.Equal(t, http.StatusBadRequest, response.Code, storeID)
		require.Empty(t, service.calls)
	}
}

func TestHandlerStrictlyRejectsMalformedBodies(t *testing.T) {
	invalidUTF8 := string([]byte{'{', '"', 'n', 'a', 'm', 'e', '"', ':', '"', 0xff, '"', '}'})
	tests := []struct {
		name, method, path, body string
		headers                  map[string][]string
	}{
		{name: "create unknown", method: http.MethodPost, path: "/api/v1/workbench/stores", body: `{"name":"S","platform":"shein","region":"SG","extra":1}`, headers: map[string][]string{"Idempotency-Key": {testCreateKey}}},
		{name: "create duplicate", method: http.MethodPost, path: "/api/v1/workbench/stores", body: `{"name":"S","name":"T","platform":"shein","region":"SG"}`, headers: map[string][]string{"Idempotency-Key": {testCreateKey}}},
		{name: "create missing", method: http.MethodPost, path: "/api/v1/workbench/stores", body: `{"name":"S","platform":"shein"}`, headers: map[string][]string{"Idempotency-Key": {testCreateKey}}},
		{name: "create unsupported platform", method: http.MethodPost, path: "/api/v1/workbench/stores", body: `{"name":"S","platform":"amazon","region":"SG"}`, headers: map[string][]string{"Idempotency-Key": {testCreateKey}}},
		{name: "create platform not exact", method: http.MethodPost, path: "/api/v1/workbench/stores", body: `{"name":"S","platform":" SHEIN ","region":"SG"}`, headers: map[string][]string{"Idempotency-Key": {testCreateKey}}},
		{name: "create trailing", method: http.MethodPost, path: "/api/v1/workbench/stores", body: `{"name":"S","platform":"shein","region":"SG"}{}`, headers: map[string][]string{"Idempotency-Key": {testCreateKey}}},
		{name: "create comment", method: http.MethodPost, path: "/api/v1/workbench/stores", body: `{"name":"S",/*comment*/"platform":"shein","region":"SG"}`, headers: map[string][]string{"Idempotency-Key": {testCreateKey}}},
		{name: "create invalid utf8", method: http.MethodPost, path: "/api/v1/workbench/stores", body: invalidUTF8, headers: map[string][]string{"Idempotency-Key": {testCreateKey}}},
		{name: "create control", method: http.MethodPost, path: "/api/v1/workbench/stores", body: `{"name":"S\u0000","platform":"shein","region":"SG"}`, headers: map[string][]string{"Idempotency-Key": {testCreateKey}}},
		{name: "create trimmed control", method: http.MethodPost, path: "/api/v1/workbench/stores", body: `{"name":"\tS","platform":"shein","region":"SG"}`, headers: map[string][]string{"Idempotency-Key": {testCreateKey}}},
		{name: "create too long", method: http.MethodPost, path: "/api/v1/workbench/stores", body: `{"name":"` + strings.Repeat("界", 121) + `","platform":"shein","region":"SG"}`, headers: map[string][]string{"Idempotency-Key": {testCreateKey}}},
		{name: "create region too long", method: http.MethodPost, path: "/api/v1/workbench/stores", body: `{"name":"S","platform":"shein","region":"` + strings.Repeat("界", 65) + `"}`, headers: map[string][]string{"Idempotency-Key": {testCreateKey}}},
		{name: "create external id too long", method: http.MethodPost, path: "/api/v1/workbench/stores", body: `{"name":"S","platform":"shein","region":"SG","externalStoreId":"` + strings.Repeat("界", 129) + `"}`, headers: map[string][]string{"Idempotency-Key": {testCreateKey}}},
		{name: "update unknown", method: http.MethodPut, path: "/api/v1/workbench/stores/" + testStoreID, body: `{"name":"S","region":"SG","organizationId":"x"}`, headers: map[string][]string{"If-Match": {`"2"`}}},
		{name: "update duplicate", method: http.MethodPut, path: "/api/v1/workbench/stores/" + testStoreID, body: `{"name":"S","name":"T","region":"SG"}`, headers: map[string][]string{"If-Match": {`"2"`}}},
		{name: "disable body", method: http.MethodPost, path: "/api/v1/workbench/stores/" + testStoreID + "/disable", body: ` `, headers: map[string][]string{"If-Match": {`"2"`}}},
		{name: "delete body", method: http.MethodDelete, path: "/api/v1/workbench/stores/" + testStoreID, body: `{}`, headers: map[string][]string{"If-Match": {`"2"`}, "Idempotency-Key": {testDeleteKey}}},
		{name: "oversized", method: http.MethodPost, path: "/api/v1/workbench/stores", body: strings.Repeat("x", 16*1024+1), headers: map[string][]string{"Idempotency-Key": {testCreateKey}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.True(t, utf8.ValidString(tt.body) || tt.name == "create invalid utf8")
			service := newStoreServiceStub(t)
			router := mountedRouter(t, mustHandler(t, service))
			response := serve(t, router, validIdentity(), tt.method, tt.path, tt.body, tt.headers)
			require.Equal(t, http.StatusBadRequest, response.Code, response.Body.String())
			assertProtocolError(t, response, "INVALID_REQUEST", nil)
			require.Empty(t, service.calls)
		})
	}
}

func TestHandlerStrictlyRejectsMissingMalformedAndRepeatedHeaders(t *testing.T) {
	tests := []struct {
		name, method, path, body string
		headers                  map[string][]string
		field, fieldCode         string
	}{
		{name: "missing create key", method: http.MethodPost, path: "/api/v1/workbench/stores", body: `{"name":"S","platform":"shein","region":"SG"}`, field: "Idempotency-Key", fieldCode: "invalid"},
		{name: "nil create key", method: http.MethodPost, path: "/api/v1/workbench/stores", body: `{"name":"S","platform":"shein","region":"SG"}`, headers: map[string][]string{"Idempotency-Key": {"00000000-0000-0000-0000-000000000000"}}, field: "Idempotency-Key", fieldCode: "invalid"},
		{name: "uppercase create key", method: http.MethodPost, path: "/api/v1/workbench/stores", body: `{"name":"S","platform":"shein","region":"SG"}`, headers: map[string][]string{"Idempotency-Key": {"AAAAAAAA-AAAA-4AAA-8AAA-AAAAAAAAAAAA"}}, field: "Idempotency-Key", fieldCode: "invalid"},
		{name: "repeated create key", method: http.MethodPost, path: "/api/v1/workbench/stores", body: `{"name":"S","platform":"shein","region":"SG"}`, headers: map[string][]string{"Idempotency-Key": {testCreateKey, testCreateKey}}, field: "Idempotency-Key", fieldCode: "invalid"},
		{name: "missing if match", method: http.MethodPut, path: "/api/v1/workbench/stores/" + testStoreID, body: `{"name":"S","region":"SG"}`, field: "If-Match", fieldCode: "invalid"},
		{name: "weak if match", method: http.MethodPut, path: "/api/v1/workbench/stores/" + testStoreID, body: `{"name":"S","region":"SG"}`, headers: map[string][]string{"If-Match": {`W/"2"`}}, field: "If-Match", fieldCode: "invalid"},
		{name: "leading zero if match", method: http.MethodPut, path: "/api/v1/workbench/stores/" + testStoreID, body: `{"name":"S","region":"SG"}`, headers: map[string][]string{"If-Match": {`"02"`}}, field: "If-Match", fieldCode: "invalid"},
		{name: "zero if match", method: http.MethodPut, path: "/api/v1/workbench/stores/" + testStoreID, body: `{"name":"S","region":"SG"}`, headers: map[string][]string{"If-Match": {`"0"`}}, field: "If-Match", fieldCode: "invalid"},
		{name: "signed if match", method: http.MethodPut, path: "/api/v1/workbench/stores/" + testStoreID, body: `{"name":"S","region":"SG"}`, headers: map[string][]string{"If-Match": {`"+2"`}}, field: "If-Match", fieldCode: "invalid"},
		{name: "whitespace if match", method: http.MethodPut, path: "/api/v1/workbench/stores/" + testStoreID, body: `{"name":"S","region":"SG"}`, headers: map[string][]string{"If-Match": {` "2"`}}, field: "If-Match", fieldCode: "invalid"},
		{name: "comma if match", method: http.MethodPut, path: "/api/v1/workbench/stores/" + testStoreID, body: `{"name":"S","region":"SG"}`, headers: map[string][]string{"If-Match": {`"2","3"`}}, field: "If-Match", fieldCode: "invalid"},
		{name: "overflow if match", method: http.MethodPut, path: "/api/v1/workbench/stores/" + testStoreID, body: `{"name":"S","region":"SG"}`, headers: map[string][]string{"If-Match": {`"9223372036854775808"`}}, field: "If-Match", fieldCode: "invalid"},
		{name: "repeated if match", method: http.MethodPut, path: "/api/v1/workbench/stores/" + testStoreID, body: `{"name":"S","region":"SG"}`, headers: map[string][]string{"If-Match": {`"2"`, `"2"`}}, field: "If-Match", fieldCode: "invalid"},
		{name: "forbidden update key", method: http.MethodPut, path: "/api/v1/workbench/stores/" + testStoreID, body: `{"name":"S","region":"SG"}`, headers: map[string][]string{"If-Match": {`"2"`}, "Idempotency-Key": {testCreateKey}}, field: "Idempotency-Key", fieldCode: "not_allowed"},
		{name: "forbidden disable key", method: http.MethodPost, path: "/api/v1/workbench/stores/" + testStoreID + "/disable", headers: map[string][]string{"If-Match": {`"2"`}, "Idempotency-Key": {testCreateKey}}, field: "Idempotency-Key", fieldCode: "not_allowed"},
		{name: "forbidden enable key", method: http.MethodPost, path: "/api/v1/workbench/stores/" + testStoreID + "/enable", headers: map[string][]string{"If-Match": {`"2"`}, "Idempotency-Key": {testCreateKey}}, field: "Idempotency-Key", fieldCode: "not_allowed"},
		{name: "missing delete key", method: http.MethodDelete, path: "/api/v1/workbench/stores/" + testStoreID, headers: map[string][]string{"If-Match": {`"2"`}}, field: "Idempotency-Key", fieldCode: "invalid"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := newStoreServiceStub(t)
			router := mountedRouter(t, mustHandler(t, service))
			response := serve(t, router, validIdentity(), tt.method, tt.path, tt.body, tt.headers)
			require.Equal(t, http.StatusBadRequest, response.Code)
			assertProtocolError(t, response, "INVALID_REQUEST", []FieldError{{Field: tt.field, Code: tt.fieldCode}})
			require.Empty(t, service.calls)
		})
	}
}

func TestHandlerReturnsExactSuccessDTOsAndReplayStatuses(t *testing.T) {
	service := newStoreServiceStub(t)
	handler := mustHandler(t, service)
	router := mountedRouter(t, handler)
	identity := validIdentity()
	wantStore := `{"id":"11111111-1111-4111-8111-111111111111","name":"Store","platform":"shein","region":"SG","externalStoreId":"external-1","lifecycleStatus":"active","connectionStatus":"connected","version":2,"createdAt":"2026-08-30T01:02:03Z","updatedAt":"2026-08-30T02:03:04Z"}`
	tests := []struct {
		name, method, path, body string
		headers                  map[string][]string
		status                   int
		want                     string
	}{
		{name: "list", method: http.MethodGet, path: "/api/v1/workbench/stores", status: 200, want: `{"items":[` + wantStore + `],"quota":{"used":1,"reserved":0,"limit":3,"allowed":true,"reason":""},"pagination":{"page":1,"pageSize":20,"total":1}}`},
		{name: "create replay", method: http.MethodPost, path: "/api/v1/workbench/stores", body: `{"name":"Store","platform":"shein","region":"SG","externalStoreId":"external-1"}`, headers: map[string][]string{"Idempotency-Key": {testCreateKey}}, status: 201, want: wantStore},
		{name: "get", method: http.MethodGet, path: "/api/v1/workbench/stores/" + testStoreID, status: 200, want: wantStore},
		{name: "update", method: http.MethodPut, path: "/api/v1/workbench/stores/" + testStoreID, body: `{"name":"Store","region":"SG"}`, headers: map[string][]string{"If-Match": {`"2"`}}, status: 200, want: wantStore},
		{name: "disable", method: http.MethodPost, path: "/api/v1/workbench/stores/" + testStoreID + "/disable", headers: map[string][]string{"If-Match": {`"2"`}}, status: 200, want: wantStore},
		{name: "enable", method: http.MethodPost, path: "/api/v1/workbench/stores/" + testStoreID + "/enable", headers: map[string][]string{"If-Match": {`"2"`}}, status: 200, want: wantStore},
		{name: "delete replay", method: http.MethodDelete, path: "/api/v1/workbench/stores/" + testStoreID, headers: map[string][]string{"If-Match": {`"2"`}, "Idempotency-Key": {testDeleteKey}}, status: 200, want: `{"id":"11111111-1111-4111-8111-111111111111","deleted":true,"version":3}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := serve(t, router, identity, tt.method, tt.path, tt.body, tt.headers)
			require.Equal(t, tt.status, response.Code)
			require.JSONEq(t, tt.want, response.Body.String())
		})
	}
}

func TestHandlerSerializesStoreTimestampsAsUTC(t *testing.T) {
	location := time.FixedZone("acceptance-local", 8*60*60)
	store, err := storecenter.RehydrateStore(storecenter.StoreSnapshot{
		ID: testStoreID, OrganizationID: "org-effective", Name: "Store", Platform: storecenter.PlatformShein, Region: "SG", ExternalStoreID: "external-1",
		LifecycleStatus: storecenter.StoreStatusActive, ConnectionRef: "connection-private", QuotaAllocationID: testAllocationID, Version: 2,
		CreatedBy: "creator-private", UpdatedBy: "updater-private", CreatedAt: time.Date(2026, 8, 30, 9, 2, 3, 0, location), UpdatedAt: time.Date(2026, 8, 30, 10, 3, 4, 0, location), CreateIdempotencyKey: testCreateKey,
	})
	require.NoError(t, err)
	projection := storecenter.StoreProjection{Store: *store, ConnectionStatus: storecenter.ConnectionStatusConnected}
	service := newStoreServiceStub(t)
	service.createResult = storecenter.CreateStoreResult{Store: store, Replayed: true}
	service.getResult = projection

	response := serve(t, mountedRouter(t, mustHandler(t, service)), validIdentity(), http.MethodPost, "/api/v1/workbench/stores", `{"name":"Store","platform":"shein","region":"SG","externalStoreId":"external-1"}`, map[string][]string{"Idempotency-Key": {testCreateKey}})

	require.Equal(t, http.StatusCreated, response.Code)
	require.Contains(t, response.Body.String(), `"createdAt":"2026-08-30T01:02:03Z"`)
	require.Contains(t, response.Body.String(), `"updatedAt":"2026-08-30T02:03:04Z"`)
}

func TestHandlerEnforcesExact16KiBBodyLimit(t *testing.T) {
	base := `{"name":"Store","platform":"shein","region":"SG"}`
	require.Less(t, len(base), requestBodyMaxBytes)
	valid := base + strings.Repeat(" ", requestBodyMaxBytes-len(base))
	tooLarge := valid + " "
	for _, tt := range []struct {
		name, body string
		status     int
		called     bool
	}{
		{name: "exact limit", body: valid, status: http.StatusCreated, called: true},
		{name: "one byte over", body: tooLarge, status: http.StatusBadRequest},
	} {
		t.Run(tt.name, func(t *testing.T) {
			service := newStoreServiceStub(t)
			response := serve(t, mountedRouter(t, mustHandler(t, service)), validIdentity(), http.MethodPost, "/api/v1/workbench/stores", tt.body, map[string][]string{"Idempotency-Key": {testCreateKey}})
			require.Equal(t, tt.status, response.Code)
			require.Equal(t, tt.called, len(service.calls) > 0)
		})
	}
}

func TestHandlerDefaultsListPaginationAndReturnsNonNullItems(t *testing.T) {
	service := newStoreServiceStub(t)
	service.listResult.Items = nil
	router := mountedRouter(t, mustHandler(t, service))
	response := serve(t, router, validIdentity(), http.MethodGet, "/api/v1/workbench/stores", "", nil)
	require.Equal(t, http.StatusOK, response.Code)
	require.JSONEq(t, `{"items":[],"quota":{"used":1,"reserved":0,"limit":3,"allowed":true,"reason":""},"pagination":{"page":1,"pageSize":20,"total":1}}`, response.Body.String())
	require.Equal(t, 1, service.listRequest.Page)
	require.Equal(t, 20, service.listRequest.PageSize)
}

func TestHandlerRejectsCorruptListProducerContractWithoutLeakingReason(t *testing.T) {
	zero, negative := int64(0), int64(-1)
	tests := []struct {
		name   string
		mutate func(*storecenter.ListStoresResult)
	}{
		{name: "total is negative", mutate: func(result *storecenter.ListStoresResult) { result.Total = -1 }},
		{name: "total is below returned item count", mutate: func(result *storecenter.ListStoresResult) { result.Total = 0 }},
		{name: "returned item count exceeds page size", mutate: func(result *storecenter.ListStoresResult) {
			result.Items = append(result.Items, result.Items...)
			result.PageSize = 1
			result.Total = 2
		}},
		{name: "used is negative", mutate: func(result *storecenter.ListStoresResult) { result.Quota.Used = -1 }},
		{name: "reserved is negative", mutate: func(result *storecenter.ListStoresResult) { result.Quota.Reserved = -1 }},
		{name: "nil limit claims allowed", mutate: func(result *storecenter.ListStoresResult) {
			result.Quota.Limit = nil
			result.Quota.Allowed = true
			result.Quota.Reason = "subscription_required"
		}},
		{name: "nil limit omits subscription reason", mutate: func(result *storecenter.ListStoresResult) {
			result.Quota.Limit = nil
			result.Quota.Allowed = false
			result.Quota.Reason = ""
		}},
		{name: "nil limit has arbitrary secret reason", mutate: func(result *storecenter.ListStoresResult) {
			result.Quota.Limit = nil
			result.Quota.Allowed = false
			result.Quota.Reason = "provider-token-secret"
		}},
		{name: "zero limit", mutate: func(result *storecenter.ListStoresResult) { result.Quota.Limit = &zero }},
		{name: "negative limit", mutate: func(result *storecenter.ListStoresResult) { result.Quota.Limit = &negative }},
		{name: "available quota claims disallowed", mutate: func(result *storecenter.ListStoresResult) {
			result.Quota.Allowed = false
			result.Quota.Reason = "store_limit_reached"
		}},
		{name: "used limit claims allowed", mutate: func(result *storecenter.ListStoresResult) {
			result.Quota.Used = 3
			result.Quota.Allowed = true
			result.Quota.Reason = ""
		}},
		{name: "reserved capacity claims allowed", mutate: func(result *storecenter.ListStoresResult) {
			result.Quota.Used = 2
			result.Quota.Reserved = 1
			result.Quota.Allowed = true
			result.Quota.Reason = ""
		}},
		{name: "allowed quota has reason", mutate: func(result *storecenter.ListStoresResult) { result.Quota.Reason = "store_limit_reached" }},
		{name: "disallowed quota omits reason", mutate: func(result *storecenter.ListStoresResult) {
			result.Quota.Used = 3
			result.Quota.Allowed = false
			result.Quota.Reason = ""
		}},
		{name: "disallowed quota has arbitrary secret reason", mutate: func(result *storecenter.ListStoresResult) {
			result.Quota.Used = 3
			result.Quota.Allowed = false
			result.Quota.Reason = "sql-password-secret"
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := newStoreServiceStub(t)
			tt.mutate(&service.listResult)
			path := "/api/v1/workbench/stores"
			if service.listResult.PageSize == 1 {
				path += "?pageSize=1"
			}
			response := serve(t, mountedRouter(t, mustHandler(t, service)), validIdentity(), http.MethodGet, path, "", nil)
			require.Equal(t, http.StatusServiceUnavailable, response.Code, response.Body.String())
			assertProtocolError(t, response, "DEPENDENCY_UNAVAILABLE", []FieldError{})
			require.NotContains(t, response.Body.String(), "provider-token-secret")
			require.NotContains(t, response.Body.String(), "sql-password-secret")
		})
	}
}

func TestHandlerPreservesEveryValidQuotaProjection(t *testing.T) {
	tests := []struct {
		name  string
		quota storecenter.StoreQuotaProjection
		want  string
	}{
		{name: "subscription required", quota: storecenter.StoreQuotaProjection{Allowed: false, Reason: "subscription_required"}, want: `{"used":0,"reserved":0,"limit":null,"allowed":false,"reason":"subscription_required"}`},
		{name: "capacity available", quota: storecenter.StoreQuotaProjection{Used: 1, Reserved: 0, Limit: int64Pointer(3), Allowed: true}, want: `{"used":1,"reserved":0,"limit":3,"allowed":true,"reason":""}`},
		{name: "capacity reached", quota: storecenter.StoreQuotaProjection{Used: 2, Reserved: 1, Limit: int64Pointer(3), Allowed: false, Reason: "store_limit_reached"}, want: `{"used":2,"reserved":1,"limit":3,"allowed":false,"reason":"store_limit_reached"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := newStoreServiceStub(t)
			service.listResult.Quota = tt.quota
			response := serve(t, mountedRouter(t, mustHandler(t, service)), validIdentity(), http.MethodGet, "/api/v1/workbench/stores", "", nil)
			require.Equal(t, http.StatusOK, response.Code, response.Body.String())
			var payload struct {
				Quota json.RawMessage `json:"quota"`
			}
			require.NoError(t, json.Unmarshal(response.Body.Bytes(), &payload))
			require.JSONEq(t, tt.want, string(payload.Quota))
		})
	}
}

func int64Pointer(value int64) *int64 { return &value }

func TestHandlerFailsClosedOnCrossOrganizationOrWrongStoreProducerOutput(t *testing.T) {
	tests := []struct {
		name, path string
		mutate     func(*storeServiceStub, *testing.T)
	}{
		{name: "foreign organization list", path: "/api/v1/workbench/stores", mutate: func(service *storeServiceStub, t *testing.T) {
			projection := projectionFor(t, "org-foreign", testStoreID)
			service.listResult.Items = []storecenter.StoreProjection{projection}
		}},
		{name: "foreign organization item", path: "/api/v1/workbench/stores/" + testStoreID, mutate: func(service *storeServiceStub, t *testing.T) {
			service.getResult = projectionFor(t, "org-foreign", testStoreID)
		}},
		{name: "wrong store item", path: "/api/v1/workbench/stores/" + testStoreID, mutate: func(service *storeServiceStub, t *testing.T) {
			service.getResult = projectionFor(t, "org-effective", "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa")
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := newStoreServiceStub(t)
			tt.mutate(service, t)
			router := mountedRouter(t, mustHandler(t, service))
			response := serve(t, router, validIdentity(), http.MethodGet, tt.path, "", nil)
			require.Equal(t, http.StatusServiceUnavailable, response.Code)
			assertProtocolError(t, response, "DEPENDENCY_UNAVAILABLE", []FieldError{})
		})
	}
}

func TestHandlerMakesUnknownAndForeignStoreNotFoundIndistinguishable(t *testing.T) {
	for _, name := range []string{"unknown same organization", "foreign store id"} {
		t.Run(name, func(t *testing.T) {
			service := newStoreServiceStub(t)
			service.err = fmt.Errorf("private lookup detail: %w", storecenter.ErrNotFound)
			response := serve(t, mountedRouter(t, mustHandler(t, service)), validIdentity(), http.MethodGet, "/api/v1/workbench/stores/"+testStoreID, "", nil)
			require.Equal(t, http.StatusNotFound, response.Code)
			assertProtocolError(t, response, "STORE_NOT_FOUND", []FieldError{})
			require.NotContains(t, response.Body.String(), "private lookup detail")
		})
	}
}

func TestNewHandlerRejectsNilAndTypedNilServices(t *testing.T) {
	var typedNil *storeServiceStub
	for _, service := range []StoreService{nil, typedNil} {
		handler, err := NewHandler(service)
		require.Nil(t, handler)
		require.Error(t, err)
	}
}

func newStoreServiceStub(t *testing.T) *storeServiceStub {
	t.Helper()
	projection := testProjection(t)
	limit := int64(3)
	return &storeServiceStub{
		listResult:     storecenter.ListStoresResult{Items: []storecenter.StoreProjection{projection}, Total: 1, Page: 1, PageSize: 20, Quota: storecenter.StoreQuotaProjection{Used: 1, Limit: &limit, Allowed: true}},
		createResult:   storecenter.CreateStoreResult{Store: &projection.Store, Replayed: true},
		getResult:      projection,
		mutationResult: storecenter.StoreMutationResult{Store: projection, Replayed: true},
		deleteResult:   storecenter.DeleteStoreResult{StoreID: testStoreID, Version: 3, Replayed: true},
	}
}

func testProjection(t *testing.T) storecenter.StoreProjection {
	t.Helper()
	return projectionFor(t, "org-effective", testStoreID)
}

func projectionFor(t *testing.T, organizationID, storeID string) storecenter.StoreProjection {
	t.Helper()
	store, err := storecenter.RehydrateStore(storecenter.StoreSnapshot{
		ID: storeID, OrganizationID: organizationID, Name: "Store", Platform: storecenter.PlatformShein, Region: "SG", ExternalStoreID: "external-1",
		LifecycleStatus: storecenter.StoreStatusActive, ConnectionRef: "connection-private", QuotaAllocationID: testAllocationID, Version: 2,
		CreatedBy: "creator-private", UpdatedBy: "updater-private", CreatedAt: time.Date(2026, 8, 30, 1, 2, 3, 0, time.UTC), UpdatedAt: time.Date(2026, 8, 30, 2, 3, 4, 0, time.UTC), CreateIdempotencyKey: testCreateKey,
	})
	require.NoError(t, err)
	return storecenter.StoreProjection{Store: *store, ConnectionStatus: storecenter.ConnectionStatusConnected}
}

func mustHandler(t *testing.T, service StoreService) *Handler {
	t.Helper()
	handler, err := NewHandler(service)
	require.NoError(t, err)
	return handler
}

func mountedRouter(t *testing.T, handler *Handler) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	registry := kernelmodule.NewRegistry()
	require.NoError(t, NewModule(handler).Register(registry))
	for _, route := range registry.Routes() {
		router.Handle(route.Method, route.Path, route.Handler)
	}
	return router
}

func validIdentity() authidentity.AuthenticatedIdentity {
	return authidentity.AuthenticatedIdentity{UserID: "user-1", EffectiveOrganizationID: "org-effective"}
}

func serve(t *testing.T, router http.Handler, identity authidentity.AuthenticatedIdentity, method, path, body string, headers map[string][]string) *httptest.ResponseRecorder {
	t.Helper()
	response := httptest.NewRecorder()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Request-ID", " request-1 ")
	for key, values := range headers {
		request.Header[http.CanonicalHeaderKey(key)] = append([]string(nil), values...)
	}
	if strings.TrimSpace(identity.UserID) != "" {
		request = request.WithContext(authidentity.WithAuthenticatedIdentity(request.Context(), identity))
	}
	router.ServeHTTP(response, request)
	return response
}

func assertProtocolError(t *testing.T, response *httptest.ResponseRecorder, code string, fields []FieldError) {
	t.Helper()
	var got map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &got))
	require.Len(t, got, 4)
	for _, key := range []string{"code", "message", "requestId", "fieldErrors"} {
		require.Contains(t, got, key)
	}
	var gotCode, message, requestID string
	require.NoError(t, json.Unmarshal(got["code"], &gotCode))
	require.Equal(t, code, gotCode)
	require.NoError(t, json.Unmarshal(got["message"], &message))
	require.NotEmpty(t, message)
	require.NoError(t, json.Unmarshal(got["requestId"], &requestID))
	require.Equal(t, "request-1", requestID)
	var gotFields []FieldError
	require.NoError(t, json.Unmarshal(got["fieldErrors"], &gotFields))
	require.NotNil(t, gotFields)
	if fields != nil {
		require.Equal(t, fields, gotFields)
	}
}

func TestStoreResponseTypesHaveOnlyExactSafeJSONFields(t *testing.T) {
	tests := []struct {
		value any
		keys  []string
	}{
		{StoreResponse{}, []string{"id", "name", "platform", "region", "externalStoreId", "lifecycleStatus", "connectionStatus", "version", "createdAt", "updatedAt"}},
		{DeleteStoreResponse{}, []string{"id", "deleted", "version"}},
		{QuotaResponse{}, []string{"used", "reserved", "limit", "allowed", "reason"}},
		{PaginationResponse{}, []string{"page", "pageSize", "total"}},
	}
	for _, tt := range tests {
		typeOf := reflect.TypeOf(tt.value)
		require.Equal(t, len(tt.keys), typeOf.NumField())
		got := make([]string, 0, typeOf.NumField())
		for index := 0; index < typeOf.NumField(); index++ {
			field := typeOf.Field(index)
			got = append(got, strings.Split(field.Tag.Get("json"), ",")[0])
			lower := strings.ToLower(field.Name + " " + field.Tag.Get("json"))
			for _, forbidden := range []string{"organization", "tenant", "actor", "audit", "connectionref", "allocation", "idempotency", "operationkey", "deletedat", "credential", "token", "secret", "password", "cookie", "session", "auth"} {
				require.NotContains(t, lower, forbidden)
			}
		}
		require.ElementsMatch(t, tt.keys, got)
	}
}

var _ StoreService = (*storeServiceStub)(nil)
