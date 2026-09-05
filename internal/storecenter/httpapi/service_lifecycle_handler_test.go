package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"task-processor/internal/authz"
	"task-processor/internal/httproute"
	kernelmodule "task-processor/internal/kernel/module"
	"task-processor/internal/storecenter"
)

const testServiceOperationID = "55555555-5555-4555-8555-555555555555"

type lifecycleServiceStub struct {
	command storecenter.ServiceCommand
	request storecenter.ServiceLifecycleApplicationRequest
	result  storecenter.ServiceOperationResult
	err     error
}

func (s *lifecycleServiceStub) Activate(_ context.Context, request storecenter.ServiceLifecycleApplicationRequest) (storecenter.ServiceOperationResult, error) {
	s.command, s.request = storecenter.ServiceCommandActivate, request
	return s.result, s.err
}

func (s *lifecycleServiceStub) Renew(_ context.Context, request storecenter.ServiceLifecycleApplicationRequest) (storecenter.ServiceOperationResult, error) {
	s.command, s.request = storecenter.ServiceCommandRenew, request
	return s.result, s.err
}

func (s *lifecycleServiceStub) Reactivate(_ context.Context, request storecenter.ServiceLifecycleApplicationRequest) (storecenter.ServiceOperationResult, error) {
	s.command, s.request = storecenter.ServiceCommandReactivate, request
	return s.result, s.err
}

func TestServiceLifecycleRoutesRemainAbsentUntilLifecycleHandlerIsExplicitlySupplied(t *testing.T) {
	baseHandler := mustHandler(t, newStoreServiceStub(t))
	baseRegistry := kernelmodule.NewRegistry()
	require.NoError(t, NewModule(baseHandler).Register(baseRegistry))
	require.Len(t, baseRegistry.Routes(), 8)

	handler, err := NewHandlerWithServiceLifecycle(newStoreServiceStub(t), &lifecycleServiceStub{})
	require.NoError(t, err)
	registry := kernelmodule.NewRegistry()
	require.NoError(t, NewModule(handler).Register(registry))
	require.Len(t, registry.Routes(), 11)

	want := []struct {
		path       string
		permission string
		access     httproute.OrganizationAccessPolicy
	}{
		{path: "/api/v1/workbench/stores/:store_id/activate", permission: authz.PermissionWorkbenchStoreLifecycle, access: httproute.OrganizationAccessPolicyLiveWrite},
		{path: "/api/v1/workbench/stores/:store_id/renew", permission: authz.PermissionWorkbenchStoreLifecycle, access: httproute.OrganizationAccessPolicyLiveWrite},
		{path: "/api/v1/workbench/stores/:store_id/reactivate", permission: authz.PermissionWorkbenchStoreLifecycle, access: httproute.OrganizationAccessPolicyLiveWrite},
	}
	for index, expected := range want {
		route := registry.Routes()[8+index]
		require.Equal(t, http.MethodPost, route.Method)
		require.Equal(t, expected.path, route.Path)
		require.Equal(t, expected.permission, route.Permission)
		require.Equal(t, httproute.AuthPolicyVerifiedIdentity, route.AuthPolicy)
		require.Equal(t, expected.access, route.OrganizationAccessPolicy)
	}
}

func TestServiceLifecycleHandlerBuildsStrictTrustedRequests(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		body     string
		command  storecenter.ServiceCommand
		quantity int64
	}{
		{name: "activate", path: "/api/v1/workbench/stores/" + testStoreID + "/activate", body: `{}`, command: storecenter.ServiceCommandActivate, quantity: 0},
		{name: "renew", path: "/api/v1/workbench/stores/" + testStoreID + "/renew", body: `{"periods":12}`, command: storecenter.ServiceCommandRenew, quantity: 12},
		{name: "reactivate", path: "/api/v1/workbench/stores/" + testStoreID + "/reactivate", body: `{"periods":2}`, command: storecenter.ServiceCommandReactivate, quantity: 2},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := &lifecycleServiceStub{result: serviceLifecycleResult(test.command, max(test.quantity, 1))}
			router := mountedServiceLifecycleRouter(t, service)
			response := serve(t, router, validIdentity(), http.MethodPost, test.path, test.body, map[string][]string{
				"Idempotency-Key": {testServiceOperationID},
				"If-Match":        {`"2"`},
			})
			require.Equal(t, http.StatusOK, response.Code, response.Body.String())
			require.Equal(t, test.command, service.command)
			require.Equal(t, storecenter.ServiceLifecycleApplicationRequest{
				OperationID: testServiceOperationID, StoreID: testStoreID, Quantity: test.quantity, ExpectedStoreVersion: 2,
			}, service.request)

			var got ServiceLifecycleResponse
			require.NoError(t, json.Unmarshal(response.Body.Bytes(), &got))
			require.Equal(t, testStoreID, got.StoreID)
			require.Equal(t, storecenter.RecordStatusActive, got.RecordStatus)
			require.Equal(t, storecenter.ServiceStatusActive, got.ServiceStatus)
			require.Equal(t, "0", got.ResourceBalanceAfter)
			require.Equal(t, int64(3), got.Version)
		})
	}
}

func TestServiceLifecycleHandlerRejectsMalformedRequestsBeforeCallingApplication(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		body    string
		headers map[string][]string
		field   string
	}{
		{name: "query", path: "/api/v1/workbench/stores/" + testStoreID + "/activate?organizationId=other", body: `{}`, headers: validLifecycleHeaders(), field: "query"},
		{name: "activate empty body", path: "/api/v1/workbench/stores/" + testStoreID + "/activate", body: "", headers: validLifecycleHeaders(), field: "body"},
		{name: "activate fields", path: "/api/v1/workbench/stores/" + testStoreID + "/activate", body: `{"periods":1}`, headers: validLifecycleHeaders(), field: "periods"},
		{name: "renew missing periods", path: "/api/v1/workbench/stores/" + testStoreID + "/renew", body: `{}`, headers: validLifecycleHeaders(), field: "periods"},
		{name: "renew zero", path: "/api/v1/workbench/stores/" + testStoreID + "/renew", body: `{"periods":0}`, headers: validLifecycleHeaders(), field: "periods"},
		{name: "renew over policy", path: "/api/v1/workbench/stores/" + testStoreID + "/renew", body: `{"periods":13}`, headers: validLifecycleHeaders(), field: "periods"},
		{name: "renew string", path: "/api/v1/workbench/stores/" + testStoreID + "/renew", body: `{"periods":"1"}`, headers: validLifecycleHeaders(), field: "periods"},
		{name: "renew fraction", path: "/api/v1/workbench/stores/" + testStoreID + "/renew", body: `{"periods":1.0}`, headers: validLifecycleHeaders(), field: "periods"},
		{name: "renew boolean", path: "/api/v1/workbench/stores/" + testStoreID + "/renew", body: `{"periods":true}`, headers: validLifecycleHeaders(), field: "periods"},
		{name: "renew duplicate", path: "/api/v1/workbench/stores/" + testStoreID + "/renew", body: `{"periods":1,"periods":1}`, headers: validLifecycleHeaders(), field: "periods"},
		{name: "renew unknown", path: "/api/v1/workbench/stores/" + testStoreID + "/renew", body: `{"periods":1,"organizationId":"other"}`, headers: validLifecycleHeaders(), field: "organizationId"},
		{name: "missing operation key", path: "/api/v1/workbench/stores/" + testStoreID + "/activate", body: `{}`, headers: map[string][]string{"If-Match": {`"2"`}}, field: "Idempotency-Key"},
		{name: "duplicate operation key", path: "/api/v1/workbench/stores/" + testStoreID + "/activate", body: `{}`, headers: map[string][]string{"Idempotency-Key": {testServiceOperationID, testServiceOperationID}, "If-Match": {`"2"`}}, field: "Idempotency-Key"},
		{name: "missing if match", path: "/api/v1/workbench/stores/" + testStoreID + "/activate", body: `{}`, headers: map[string][]string{"Idempotency-Key": {testServiceOperationID}}, field: "If-Match"},
		{name: "duplicate if match", path: "/api/v1/workbench/stores/" + testStoreID + "/activate", body: `{}`, headers: map[string][]string{"Idempotency-Key": {testServiceOperationID}, "If-Match": {`"2"`, `"2"`}}, field: "If-Match"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := &lifecycleServiceStub{}
			response := serve(t, mountedServiceLifecycleRouter(t, service), validIdentity(), http.MethodPost, test.path, test.body, test.headers)
			require.Equal(t, http.StatusBadRequest, response.Code, response.Body.String())
			assertProtocolError(t, response, "INVALID_REQUEST", []FieldError{{Field: test.field, Code: "invalid"}})
			require.Empty(t, service.command)
		})
	}
}

func TestServiceLifecycleHandlerRejectsMismatchedApplicationResult(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*storecenter.ServiceOperationSnapshot)
	}{
		{name: "organization", mutate: func(snapshot *storecenter.ServiceOperationSnapshot) { snapshot.OrganizationID = "other-org" }},
		{name: "operation", mutate: func(snapshot *storecenter.ServiceOperationSnapshot) {
			snapshot.OperationID = "77777777-7777-4777-8777-777777777777"
		}},
		{name: "store", mutate: func(snapshot *storecenter.ServiceOperationSnapshot) {
			snapshot.StoreID = "88888888-8888-4888-8888-888888888888"
		}},
		{name: "command", mutate: func(snapshot *storecenter.ServiceOperationSnapshot) {
			snapshot.Command = storecenter.ServiceCommandRenew
		}},
		{name: "quantity", mutate: func(snapshot *storecenter.ServiceOperationSnapshot) { snapshot.Quantity = "2" }},
		{name: "resource type", mutate: func(snapshot *storecenter.ServiceOperationSnapshot) { snapshot.ResourceType = "ai_point" }},
		{name: "balance", mutate: func(snapshot *storecenter.ServiceOperationSnapshot) { snapshot.BalanceAfter = "-1" }},
		{name: "version", mutate: func(snapshot *storecenter.ServiceOperationSnapshot) { snapshot.StoreVersion = 0 }},
		{name: "positive mismatched version", mutate: func(snapshot *storecenter.ServiceOperationSnapshot) { snapshot.StoreVersion = 4 }},
		{name: "state", mutate: func(snapshot *storecenter.ServiceOperationSnapshot) { snapshot.ServiceState.ExpiresAt = nil }},
		{name: "pending activation postcondition", mutate: func(snapshot *storecenter.ServiceOperationSnapshot) {
			snapshot.ServiceState = storecenter.StoreServiceState{RecordStatus: storecenter.RecordStatusActive, ServiceStatus: storecenter.ServiceStatusPendingActivation}
		}},
		{name: "provisioning postcondition", mutate: func(snapshot *storecenter.ServiceOperationSnapshot) {
			snapshot.ServiceState = storecenter.StoreServiceState{RecordStatus: storecenter.RecordStatusProvisioning}
		}},
		{name: "event", mutate: func(snapshot *storecenter.ServiceOperationSnapshot) { snapshot.EventID = "not-a-uuid" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := serviceLifecycleResult(storecenter.ServiceCommandActivate, 1)
			test.mutate(&result.Snapshot)
			service := &lifecycleServiceStub{result: result}
			response := serve(t, mountedServiceLifecycleRouter(t, service), validIdentity(), http.MethodPost,
				"/api/v1/workbench/stores/"+testStoreID+"/activate", `{}`, validLifecycleHeaders())
			require.Equal(t, http.StatusServiceUnavailable, response.Code)
			assertProtocolError(t, response, "DEPENDENCY_UNAVAILABLE", nil)
		})
	}
}

func TestServiceLifecycleResponseHasOnlyReplayStableSafeFields(t *testing.T) {
	typeOf := reflect.TypeOf(ServiceLifecycleResponse{})
	want := []string{"storeId", "recordStatus", "serviceStatus", "serviceStartedAt", "serviceExpiresAt", "version", "quantity", "resourceBalanceAfter"}
	require.Equal(t, len(want), typeOf.NumField())
	for index, key := range want {
		field := typeOf.Field(index)
		require.Equal(t, key, field.Tag.Get("json"))
	}
}

func TestNewHandlerWithServiceLifecycleRejectsMissingDependency(t *testing.T) {
	var typedNil *lifecycleServiceStub
	for _, lifecycle := range []ServiceLifecycleService{nil, typedNil} {
		_, err := NewHandlerWithServiceLifecycle(newStoreServiceStub(t), lifecycle)
		require.Error(t, err)
	}
}

func mountedServiceLifecycleRouter(t *testing.T, service ServiceLifecycleService) http.Handler {
	t.Helper()
	handler, err := NewHandlerWithServiceLifecycle(newStoreServiceStub(t), service)
	require.NoError(t, err)
	return mountedRouter(t, handler)
}

func validLifecycleHeaders() map[string][]string {
	return map[string][]string{"Idempotency-Key": {testServiceOperationID}, "If-Match": {`"2"`}}
}

func serviceLifecycleResult(command storecenter.ServiceCommand, quantity int64) storecenter.ServiceOperationResult {
	startedAt := time.Date(2026, 9, 5, 2, 0, 0, 0, time.UTC)
	expiresAt := startedAt.Add(time.Duration(quantity) * 30 * 24 * time.Hour)
	return storecenter.ServiceOperationResult{Snapshot: storecenter.ServiceOperationSnapshot{
		OrganizationID: "org-effective", OperationID: testServiceOperationID, StoreID: testStoreID,
		Command: command, Quantity: strconv.FormatInt(quantity, 10), ResourceType: "store_renewal_period", BalanceAfter: "0", StoreVersion: 3,
		ServiceState: storecenter.StoreServiceState{RecordStatus: storecenter.RecordStatusActive, ServiceStatus: storecenter.ServiceStatusActive, StartedAt: &startedAt, ExpiresAt: &expiresAt},
		EventID:      "66666666-6666-4666-8666-666666666666",
	}}
}

var _ ServiceLifecycleService = (*lifecycleServiceStub)(nil)
