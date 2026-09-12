package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"task-processor/internal/authidentity"
	domain "task-processor/internal/organization/membership"
)

type commandStub struct{ calls int }

func (s *commandStub) Execute(_ context.Context, key string, input domain.CommandInput) (domain.Operation, error) {
	s.calls++
	return domain.Operation{Key: key, Kind: input.Kind, Phase: domain.PhaseDispatched}, nil
}
func (s *commandStub) ReadOperation(context.Context, string) (domain.Operation, error) {
	s.calls++
	return domain.Operation{}, domain.ErrNotFound
}
func (s *commandStub) Verify(context.Context, string) (domain.Operation, error) {
	s.calls++
	return domain.Operation{}, domain.ErrNotFound
}

func TestCommandHTTPRejectsAmbiguousInputBeforeFactory(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name, body, key, query string
		status                 int
	}{
		{"valid", `{"role":"listingkit_viewer","expectedVersion":"` + strings.Repeat("a", 64) + `"}`, uuid.NewString(), "", 200},
		{"duplicate field", `{"role":"listingkit_viewer","role":"listingkit_admin","expectedVersion":"` + strings.Repeat("a", 64) + `"}`, uuid.NewString(), "", 400},
		{"unknown field", `{"actor":"foreign","role":"listingkit_viewer"}`, uuid.NewString(), "", 400},
		{"bad key", `{}`, "not-uuid", "", 400},
		{"query", `{}`, uuid.NewString(), "?org=foreign", 400},
		{"large", strings.Repeat("x", 16385), uuid.NewString(), "", 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			commands := &commandStub{}
			factories := 0
			h := NewCommandHandler(nil, func(*http.Request) (CommandService, error) { factories++; return commands, nil })
			router := gin.New()
			router.POST("/api/v1/account/members/:member_id/role", h.ChangeRole)
			request := httptest.NewRequest("POST", "/api/v1/account/members/grant/role"+tc.query, strings.NewReader(tc.body))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Idempotency-Key", tc.key)
			request.Header.Set("X-Requested-Organization-ID", "org")
			request = request.WithContext(authidentity.WithAuthenticatedIdentity(request.Context(), authidentity.AuthenticatedIdentity{UserID: "actor", EffectiveOrganizationID: "org", TokenExpiresAt: time.Now().Add(time.Hour)}))
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != tc.status {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
			if tc.status != 200 && (commands.calls != 0 || factories != 0) {
				t.Fatal("invalid input reached commands")
			}
			if tc.status == 200 && !strings.Contains(response.Body.String(), `"status":"unknown"`) {
				t.Fatal(fmt.Sprintf("dispatched advertised completed: %s", response.Body.String()))
			}
		})
	}
}
