package pipeline

import (
	"errors"
	"testing"

	"task-processor/internal/model"
	shein "task-processor/internal/shein"
)

func TestTaskErrorRouterRouteCookieError(t *testing.T) {
	router := NewTaskErrorRouter()

	decision := router.Route(model.Task{TenantID: 1, StoreID: 2}, shein.NewCookieLoadError(1, 2, "cookie missing"))
	if decision.route != taskErrorRouteFailure {
		t.Fatalf("route = %v, want failure", decision.route)
	}
	if decision.err == nil {
		t.Fatal("cookie error should be preserved")
	}
}

func TestTaskErrorRouterRouteSemanticAuthError(t *testing.T) {
	router := NewTaskErrorRouter()

	decision := router.Route(model.Task{TenantID: 11, StoreID: 22}, errors.New("20302 子系统登录重定向"))
	if decision.route != taskErrorRouteAuthenticationExpired {
		t.Fatalf("route = %v, want authentication expired", decision.route)
	}
	if decision.authErr == nil {
		t.Fatal("auth error should be materialized")
	}
	if decision.authErr.TenantID != 11 || decision.authErr.ShopID != 22 {
		t.Fatal("auth error should inherit tenant/store from task")
	}
}
