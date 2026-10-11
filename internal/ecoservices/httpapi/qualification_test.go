package httpapi

import (
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"strings"
	"task-processor/internal/authidentity"
	"testing"
)

type qualificationFiles struct{ FilePort }

func TestQualificationRoutesCloseAllFinancialConsumers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := &serviceFixture{}
	h, err := NewQualificationHandler(s, &qualificationFiles{})
	if err != nil {
		t.Fatal(err)
	}
	allowed := map[string]bool{
		"GET " + Base + "/catalog": true, "GET " + Base + "/catalog/:id": true,
		"GET " + Base + "/applications": true, "POST " + Base + "/applications": true,
		"POST " + Base + "/applications/:id/agreement": true,
		"POST " + Base + "/applications/files":         true, "POST " + Base + "/applications/:id/files": true,
		"GET " + Base + "/applications/files/:id": true,
		"GET " + Base + "/provider/listings":      true,
		"GET " + Base + "/requests":               true, "GET " + Base + "/requests/:id": true,
		"GET " + AdminBase + "/applications": true, "GET " + AdminBase + "/files/:id": true,
		"POST " + AdminBase + "/applications/:id/approve": true, "POST " + AdminBase + "/applications/:id/reject": true,
	}
	for _, route := range Routes(h) {
		if allowed[route.Method+" "+route.Path] {
			continue
		}
		router := gin.New()
		router.Handle(route.Method, route.Path, route.Handler)
		path := strings.ReplaceAll(route.Path, ":id", "11111111-1111-4111-8111-111111111111")
		request := httptest.NewRequest(route.Method, path, strings.NewReader("{}"))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, request)
		if w.Code != 503 {
			t.Fatalf("closed consumer %s %s returned %d", route.Method, path, w.Code)
		}
	}
	if s.writes != 0 || s.query.Kind != "" {
		t.Fatal("closed endpoint reached domain")
	}
}

func TestCheckoutWithoutPaymentPortReturnsUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	h := &Handler{}
	router.POST("/orders/:id/checkout", h.checkout)
	r := httptest.NewRequest("POST", "/orders/11111111-1111-4111-8111-111111111111/checkout", strings.NewReader("{}"))
	r.Header.Set("Content-Type", "application/json")
	r = r.WithContext(authidentity.WithAuthenticatedIdentity(r.Context(), authidentity.AuthenticatedIdentity{UserID: "user", EffectiveOrganizationID: "org"}))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)
	if w.Code != 503 {
		t.Fatalf("unconfigured checkout returned %d", w.Code)
	}
}
