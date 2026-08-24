package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"task-processor/internal/listingkit/memberinvite"
	"task-processor/internal/listingsubscription"
)

func TestForgedHeadersCannotGrantPlatformSubscriptionAccess(t *testing.T) {
	router := platformSubscriptionTestRouterWithOptions(t, WithPlatformSubscriptionAccess([]string{"configured-platform-admin"}, nil))
	req := httptest.NewRequest(http.MethodGet, "/platform/subscriptions/org-target", nil)
	req.Header.Set("X-User-ID", "configured-platform-admin")
	req.Header.Set("X-User-Roles", "platform_admin")
	req.Header.Set("X-Zitadel-Roles", "admin")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, req)

	assertInvitationErrorResponse(t, response, http.StatusForbidden, "zitadel_user_missing")
}

func TestAuthenticatedConfiguredSubjectGrantsPlatformSubscriptionAccessDespiteForgedHeaders(t *testing.T) {
	router := platformSubscriptionTestRouterWithOptions(t, WithPlatformSubscriptionAccess([]string{"configured-platform-admin"}, nil))
	req := httptest.NewRequest(http.MethodGet, "/platform/subscriptions/org-target", nil)
	req.Header.Set("X-User-ID", "forged-non-admin")
	req.Header.Set("X-User-Roles", "listingkit_viewer")
	req = withAuthenticatedIdentity(req, "admin-tenant", "configured-platform-admin", "listingkit_viewer")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, req)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusOK, response.Body.String())
	}
}

func TestPlatformSubscriptionAuditActorUsesAuthenticatedSubject(t *testing.T) {
	repo := listingsubscription.NewMemRepository()
	service, err := listingsubscription.NewService(repo)
	if err != nil {
		t.Fatal(err)
	}
	h, err := NewHandler(&stubHandlerCoreService{}, WithSubscriptionService(service))
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	router.PUT("/platform/subscriptions/:tenant_id/entitlements/:module_code", h.UpsertPlatformTenantSubscriptionEntitlement)
	body, err := json.Marshal(map[string]any{"status": "active"})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPut, "/platform/subscriptions/org-target/entitlements/studio", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "forged-audit-actor")
	req = withAuthenticatedIdentity(req, "admin-tenant", "zitadel-subject-101", "platform_admin")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, req)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusOK, response.Body.String())
	}
	logs, err := service.ListAuditLogs(t.Context(), "org-target", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) != 1 || logs[0].ActorID != "zitadel-subject-101" {
		t.Fatalf("audit logs = %#v", logs)
	}
}

func TestMemberInvitationMissingAuthenticatedContextStopsBeforeProviderAndAudit(t *testing.T) {
	providerCalled := false
	audit := &invitationAuditStub{}
	router := invitationRouter(t, invitationProviderStub{onInvite: func() { providerCalled = true }}, audit)
	req := httptest.NewRequest(http.MethodPost, "/platform/tenants/org-1/members/invitations", bytes.NewBufferString(`{"given_name":"Jane","family_name":"Doe","email":"jane@example.com","role":"listingkit_viewer"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "forged-admin")
	req.Header.Set("X-User-Roles", "platform_admin")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, req)

	assertInvitationErrorResponse(t, response, http.StatusForbidden, "zitadel_user_missing")
	if providerCalled {
		t.Fatal("provider was called without an authenticated identity")
	}
	if len(audit.records) != 0 {
		t.Fatalf("audit records = %#v, want none", audit.records)
	}
}

func TestMemberInvitationAuditActorUsesAuthenticatedSubject(t *testing.T) {
	audit := &invitationAuditStub{}
	router := invitationRouter(t, invitationProviderStub{
		invitation: memberinvite.Invitation{UserID: "user-1", AuthorizationID: "authorization-1"},
	}, audit)
	req := invitationRequest("org-1", "platform_admin", "forged-audit-actor")
	req = withAuthenticatedIdentity(req, "admin-tenant", "zitadel-subject-101", "platform_admin")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, req)

	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusCreated, response.Body.String())
	}
	if len(audit.records) != 1 || audit.records[0].ActorUserID != "zitadel-subject-101" {
		t.Fatalf("audit records = %#v", audit.records)
	}
}
