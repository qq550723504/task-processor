package httpapi

import (
	"context"
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"strings"
	"task-processor/internal/authidentity"
	e "task-processor/internal/ecoservices"
	"testing"
)

type serviceFixture struct {
	command e.Command
	query   e.Query
	writes  int
}

func (s *serviceFixture) Read(_ context.Context, q e.Query) (e.Page, error) {
	s.query = q
	return e.Page{Requests: []e.Request{}, Counts: map[string]int64{}}, nil
}
func (s *serviceFixture) Mutate(_ context.Context, c e.Command) (e.Result, error) {
	s.command = c
	s.writes++
	return e.Result{Request: &e.Request{ID: c.ID}}, nil
}
func runCommand(s *serviceFixture, body string) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	h := &Handler{service: s}
	router.POST("/requests/:id/accept", h.mutate("accept", false))
	req := httptest.NewRequest("POST", "/requests/11111111-1111-4111-8111-111111111111/accept", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "22222222-2222-4222-8222-222222222222")
	req.Header.Set("If-Match", "\"7\"")
	req = req.WithContext(authidentity.WithAuthenticatedIdentity(req.Context(), authidentity.AuthenticatedIdentity{UserID: "verified-user", EffectiveOrganizationID: "verified-org"}))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}
func TestCustomerAcceptanceUsesVerifiedScopeAndExactVersion(t *testing.T) {
	s := &serviceFixture{}
	w := runCommand(s, `{"deliveryVersion":"3"}`)
	if w.Code != 200 || s.writes != 1 || s.command.Scope.OrganizationID != "verified-org" || s.command.Scope.ActorID != "verified-user" || s.command.Version != 7 || s.command.DeliveryVersion != 3 {
		t.Fatalf("acceptance lost trusted scope/version: code=%d cmd=%+v", w.Code, s.command)
	}
}
func TestUntrustedOwnershipAndAmbiguousVersionsNeverReachDomain(t *testing.T) {
	for _, body := range []string{`{"deliveryVersion":"3","organizationId":"other"}`, `{"DeliveryVersion":"3"}`, `{"deliveryVersion":"3","DeliveryVersion":"4"}`, `{"deliveryVersion":"3","deliveryVersion":"4"}`, `{"deliveryVersion":"03"}`, `{"deliveryVersion":3}`} {
		s := &serviceFixture{}
		w := runCommand(s, body)
		if w.Code != 400 || s.writes != 0 {
			t.Fatalf("ambiguous/overposted command reached owner: code=%d body=%s", w.Code, body)
		}
	}
}
