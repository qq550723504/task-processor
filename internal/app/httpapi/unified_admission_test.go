package httpapi

import (
	"net/http"
	"strings"
	"testing"
)

func TestUnifiedCommercialAdmissionRetiresSubscriptionAndQuotaResume(t *testing.T) {
	catalog := false
	for _, route := range currentCommercialBillingApplicationRoutes {
		if strings.Contains(route.Path, "subscription") {
			t.Fatalf("retired subscription route admitted: %s", route.Path)
		}
		if route.Method == http.MethodGet && route.Path == "/api/v1/workbench/commercial/resource-offers" {
			catalog = true
		}
	}
	if !catalog {
		t.Fatal("resource offers absent")
	}
	for _, route := range currentStoreCenterRoutes {
		if strings.HasSuffix(route.path, "/resume") {
			t.Fatal("retired quota resume route admitted")
		}
	}
}
