package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"task-processor/internal/listingkit"
	"task-processor/internal/listingkit/memberinvite"
	"task-processor/internal/listingkit/tenantdirectory"
	"task-processor/internal/listingsubscription"
)

func TestPlatformSubscriptionRequiresPlatformRole(t *testing.T) {
	router := platformSubscriptionTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/platform/subscriptions/org-target", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%s", resp.Code, http.StatusForbidden, resp.Body.String())
	}
}

func TestPlatformSubscriptionCanOpenModuleForTenant(t *testing.T) {
	router := platformSubscriptionTestRouter(t)

	body, err := json.Marshal(map[string]any{
		"status": "active",
		"limits": map[string]int{"design_jobs": 12},
	})
	if err != nil {
		t.Fatal(err)
	}
	putReq := httptest.NewRequest(http.MethodPut, "/platform/subscriptions/org-target/entitlements/studio", bytes.NewReader(body))
	putReq.Header.Set("Content-Type", "application/json")
	putReq = withAuthenticatedIdentity(putReq, "admin-tenant", "admin-1", "platform_admin")
	putResp := httptest.NewRecorder()
	router.ServeHTTP(putResp, putReq)

	if putResp.Code != http.StatusOK {
		t.Fatalf("put status = %d, want %d; body=%s", putResp.Code, http.StatusOK, putResp.Body.String())
	}

	getReq := httptest.NewRequest(http.MethodGet, "/platform/subscriptions/org-target", nil)
	getReq = withAuthenticatedIdentity(getReq, "admin-tenant", "admin-1", "platform_admin")
	getResp := httptest.NewRecorder()
	router.ServeHTTP(getResp, getReq)

	if getResp.Code != http.StatusOK {
		t.Fatalf("get status = %d, want %d; body=%s", getResp.Code, http.StatusOK, getResp.Body.String())
	}
	var summary listingsubscription.Summary
	if err := json.Unmarshal(getResp.Body.Bytes(), &summary); err != nil {
		t.Fatalf("decode summary: %v", err)
	}
	if summary.TenantID != "org-target" {
		t.Fatalf("tenant id = %q, want org-target", summary.TenantID)
	}
	var studio *listingsubscription.EntitlementView
	for i := range summary.Entitlements {
		if summary.Entitlements[i].Module.Code == listingsubscription.ModuleStudio {
			studio = &summary.Entitlements[i]
			break
		}
	}
	if studio == nil || !studio.Allowed || studio.Limits["design_jobs"] != 12 {
		t.Fatalf("studio entitlement = %#v", studio)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/platform/subscriptions", nil)
	listReq = withAuthenticatedIdentity(listReq, "admin-tenant", "admin-1", "platform_admin")
	listResp := httptest.NewRecorder()
	router.ServeHTTP(listResp, listReq)
	if listResp.Code != http.StatusOK {
		t.Fatalf("list status = %d, want %d; body=%s", listResp.Code, http.StatusOK, listResp.Body.String())
	}
	var listBody struct {
		Items []listingsubscription.TenantOverview `json:"items"`
	}
	if err := json.Unmarshal(listResp.Body.Bytes(), &listBody); err != nil {
		t.Fatalf("decode tenant list: %v", err)
	}
	if len(listBody.Items) != 1 || listBody.Items[0].TenantID != "org-target" || listBody.Items[0].ActiveCount != 1 {
		t.Fatalf("tenant list = %#v", listBody.Items)
	}
	if listBody.Items[0].TenantDisplayName != "org-target" {
		t.Fatalf("tenant display name = %q, want tenant id fallback", listBody.Items[0].TenantDisplayName)
	}
}

func TestPlatformSubscriptionListReturnsResolvedTenantDisplayName(t *testing.T) {
	repo := listingsubscription.NewMemRepository()
	service, err := listingsubscription.NewService(repo)
	if err != nil {
		t.Fatalf("create subscription service: %v", err)
	}
	service.SetTenantDisplayNameResolver(subscriptionDisplayNameResolver{
		"org-target": "目标租户",
	})
	h, err := NewHandler(&stubHandlerCoreService{}, WithSubscriptionService(service))
	if err != nil {
		t.Fatalf("create handler: %v", err)
	}
	body, err := json.Marshal(map[string]any{
		"status": "active",
		"limits": map[string]int{"design_jobs": 12},
	})
	if err != nil {
		t.Fatal(err)
	}
	putReq := httptest.NewRequest(http.MethodPut, "/platform/subscriptions/org-target/entitlements/studio", bytes.NewReader(body))
	putReq.Header.Set("Content-Type", "application/json")
	putReq = withAuthenticatedIdentity(putReq, "admin-tenant", "admin-1", "platform_admin")
	putResp := httptest.NewRecorder()
	fullRouter := gin.New()
	fullRouter.GET("/platform/subscriptions", h.ListPlatformTenantSubscriptions)
	fullRouter.PUT("/platform/subscriptions/:tenant_id/entitlements/:module_code", h.UpsertPlatformTenantSubscriptionEntitlement)
	fullRouter.ServeHTTP(putResp, putReq)
	if putResp.Code != http.StatusOK {
		t.Fatalf("put status = %d, want %d; body=%s", putResp.Code, http.StatusOK, putResp.Body.String())
	}

	req := httptest.NewRequest(http.MethodGet, "/platform/subscriptions", nil)
	req = withAuthenticatedIdentity(req, "admin-tenant", "admin-1", "platform_admin")
	resp := httptest.NewRecorder()
	fullRouter.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("list status = %d, want %d; body=%s", resp.Code, http.StatusOK, resp.Body.String())
	}
	var listBody struct {
		Items []listingsubscription.TenantOverview `json:"items"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &listBody); err != nil {
		t.Fatalf("decode tenant list: %v", err)
	}
	if len(listBody.Items) != 1 || listBody.Items[0].TenantDisplayName != "目标租户" {
		t.Fatalf("tenant list = %#v", listBody.Items)
	}
}

func TestPlatformTenantDirectoryRequiresPlatformRole(t *testing.T) {
	service, err := listingsubscription.NewService(listingsubscription.NewMemRepository())
	if err != nil {
		t.Fatalf("create subscription service: %v", err)
	}
	h, err := NewHandler(
		&stubHandlerCoreService{},
		WithSubscriptionService(service),
		WithTenantDirectory(tenantDirectoryStub{{ID: "org-target", DisplayName: "Target tenant"}}),
	)
	if err != nil {
		t.Fatalf("create handler: %v", err)
	}
	router := gin.New()
	router.GET("/platform/tenant-directory", h.ListPlatformTenantDirectory)

	forbiddenReq := httptest.NewRequest(http.MethodGet, "/platform/tenant-directory", nil)
	forbiddenResp := httptest.NewRecorder()
	router.ServeHTTP(forbiddenResp, forbiddenReq)
	if forbiddenResp.Code != http.StatusForbidden {
		t.Fatalf("forbidden status = %d, want %d", forbiddenResp.Code, http.StatusForbidden)
	}

	req := httptest.NewRequest(http.MethodGet, "/platform/tenant-directory", nil)
	req = withAuthenticatedIdentity(req, "admin-tenant", "admin-1", "platform_admin")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", resp.Code, http.StatusOK, resp.Body.String())
	}
	var body struct {
		Items []tenantdirectory.Tenant `json:"items"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode tenant directory: %v", err)
	}
	if len(body.Items) != 1 || body.Items[0].ID != "org-target" {
		t.Fatalf("items = %#v", body.Items)
	}
}

type tenantDirectoryStub []tenantdirectory.Tenant

func (s tenantDirectoryStub) List(context.Context) ([]tenantdirectory.Tenant, error) {
	return append([]tenantdirectory.Tenant(nil), s...), nil
}

func TestInviteTenantMemberRequiresPlatformAdminAndKnownTenant(t *testing.T) {
	audit := &invitationAuditStub{}
	router := invitationRouter(t, invitationProviderStub{
		invitation: memberinvite.Invitation{UserID: "user-1", AuthorizationID: "authorization-1"},
		onInvite:   func() { t.Fatal("provider called before authorization or tenant validation") },
	}, audit)

	unauthorized := httptest.NewRecorder()
	router.ServeHTTP(unauthorized, invitationRequest("org-1", "", "admin-1"))
	if unauthorized.Code != http.StatusForbidden {
		t.Fatalf("unauthorized status = %d; body=%s", unauthorized.Code, unauthorized.Body.String())
	}
	assertInvitationErrorResponse(t, unauthorized, http.StatusForbidden, "listingkit_permission_denied")
	if len(audit.records) != 0 {
		t.Fatalf("unauthorized request wrote audit records: %#v", audit.records)
	}

	missingTenant := httptest.NewRecorder()
	router.ServeHTTP(missingTenant, invitationRequest("org-missing", "platform_admin", "admin-1"))
	assertInvitationErrorResponse(t, missingTenant, http.StatusNotFound, "tenant_not_found")
	if len(audit.records) != 1 || audit.records[0].TenantID != "org-missing" || audit.records[0].Outcome != memberinvite.OutcomeFailed || audit.records[0].ErrorCode != "tenant_not_found" {
		t.Fatalf("audit records = %#v", audit.records)
	}
}

func TestInviteTenantMemberReturnsCreatedInvitationAndRecordsActor(t *testing.T) {
	audit := &invitationAuditStub{}
	response := invokeInvitation(t, invitationProviderStub{
		invitation: memberinvite.Invitation{UserID: "user-1", AuthorizationID: "authorization-1"},
	}, audit)
	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d; body=%s", response.Code, response.Body.String())
	}
	var body struct {
		TenantID            string `json:"tenant_id"`
		UserID              string `json:"user_id"`
		AuthorizationID     string `json:"authorization_id"`
		InvitationEmailSent bool   `json:"invitation_email_sent"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.TenantID != "org-1" || body.UserID != "user-1" || body.AuthorizationID != "authorization-1" || !body.InvitationEmailSent {
		t.Fatalf("body = %#v", body)
	}
	if len(audit.records) != 1 || audit.records[0].ActorUserID != "admin-1" || audit.records[0].Outcome != memberinvite.OutcomeSucceeded {
		t.Fatalf("audit records = %#v", audit.records)
	}
}

func TestInviteTenantMemberMapsProviderConflictTo409(t *testing.T) {
	audit := &invitationAuditStub{}
	response := invokeInvitation(t, invitationProviderStub{err: memberinvite.ErrConflict}, audit)
	assertInvitationErrorResponse(t, response, http.StatusConflict, "member_invitation_conflict")
	assertInvitationAudit(t, audit, memberinvite.OutcomeFailed, "member_invitation_conflict", "")
}

func TestInviteTenantMemberRecordsIncompleteProviderState(t *testing.T) {
	audit := &invitationAuditStub{}
	response := invokeInvitation(t, invitationProviderStub{err: &memberinvite.IncompleteError{UserID: "user-1", Err: errors.New("provider-secret")}}, audit)
	assertInvitationErrorResponse(t, response, http.StatusBadGateway, "zitadel_member_invitation_incomplete")
	var body struct {
		UserID string `json:"user_id"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.UserID != "user-1" {
		t.Fatalf("user_id = %q", body.UserID)
	}
	if bytes.Contains(response.Body.Bytes(), []byte("provider-secret")) {
		t.Fatalf("response leaked provider error: %s", response.Body.String())
	}
	assertInvitationAudit(t, audit, memberinvite.OutcomeIncomplete, "zitadel_member_invitation_incomplete", "user-1")
}

func TestInviteTenantMemberRecordsIncompleteProviderConflict(t *testing.T) {
	audit := &invitationAuditStub{}
	response := invokeInvitation(t, invitationProviderStub{err: &memberinvite.IncompleteError{UserID: "user-1", Err: memberinvite.ErrConflict}}, audit)
	assertInvitationErrorResponse(t, response, http.StatusBadGateway, "zitadel_member_invitation_incomplete")
	var body struct {
		UserID string `json:"user_id"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.UserID != "user-1" {
		t.Fatalf("user_id = %q", body.UserID)
	}
	assertInvitationAudit(t, audit, memberinvite.OutcomeIncomplete, "zitadel_member_invitation_incomplete", "user-1")
}

func TestInviteTenantMemberReturns503WhenProviderIsNotConfigured(t *testing.T) {
	audit := &invitationAuditStub{}
	router := invitationRouter(t, nil, audit)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, invitationRequest("org-1", "platform_admin", "admin-1"))
	assertInvitationErrorResponse(t, response, http.StatusServiceUnavailable, "member_invitation_unavailable")
	assertInvitationAudit(t, audit, memberinvite.OutcomeFailed, "member_invitation_unavailable", "")
}

func TestInviteTenantMemberReturnsUnavailableWhenAuditCannotBePersisted(t *testing.T) {
	audit := &invitationAuditStub{err: errors.New("database-secret")}
	response := invokeInvitation(t, invitationProviderStub{
		invitation: memberinvite.Invitation{UserID: "user-1", AuthorizationID: "authorization-1"},
	}, audit)
	assertInvitationErrorResponse(t, response, http.StatusServiceUnavailable, "member_invitation_unavailable")
	if strings.Contains(response.Body.String(), "database-secret") {
		t.Fatalf("response leaked audit error: %s", response.Body.String())
	}
}

func TestInviteTenantMemberMapsTenantDirectoryFailuresToStableInvitationErrors(t *testing.T) {
	provider := invitationProviderStub{onInvite: func() { t.Fatal("provider called before tenant validation") }}

	t.Run("directory missing", func(t *testing.T) {
		audit := &invitationAuditStub{}
		response := httptest.NewRecorder()
		invitationRouterWithDirectory(t, provider, audit, nil).ServeHTTP(response, invitationRequest("org-1", "platform_admin", "admin-1"))
		assertInvitationErrorResponse(t, response, http.StatusServiceUnavailable, "member_invitation_unavailable")
		assertInvitationAudit(t, audit, memberinvite.OutcomeFailed, "member_invitation_unavailable", "")
	})

	t.Run("directory lookup fails", func(t *testing.T) {
		audit := &invitationAuditStub{}
		response := httptest.NewRecorder()
		invitationRouterWithDirectory(t, provider, audit, tenantDirectoryErrorStub{}).ServeHTTP(response, invitationRequest("org-1", "platform_admin", "admin-1"))
		assertInvitationErrorResponse(t, response, http.StatusBadGateway, "zitadel_member_invitation_failed")
		assertInvitationAudit(t, audit, memberinvite.OutcomeFailed, "zitadel_member_invitation_failed", "")
	})
}

func TestInviteTenantMemberMapsInvalidAndProviderFailures(t *testing.T) {
	t.Run("invalid request", func(t *testing.T) {
		audit := &invitationAuditStub{}
		provider := invitationProviderStub{onInvite: func() { t.Fatal("provider called for invalid input") }}
		response := httptest.NewRecorder()
		request := invitationRequest("org-1", "platform_admin", "admin-1")
		request.Body = io.NopCloser(bytes.NewBufferString(`{"given_name":"Jane","family_name":"Doe","email":"bad","role":"listingkit_viewer"}`))
		invitationRouter(t, provider, audit).ServeHTTP(response, request)
		assertInvitationErrorResponse(t, response, http.StatusBadRequest, "invalid_member_invitation")
		if len(audit.records) != 1 || audit.records[0].Email != "bad" || audit.records[0].Outcome != memberinvite.OutcomeFailed || audit.records[0].ErrorCode != "invalid_member_invitation" {
			t.Fatalf("audit records = %#v", audit.records)
		}
	})

	t.Run("provider failure", func(t *testing.T) {
		audit := &invitationAuditStub{}
		response := invokeInvitation(t, invitationProviderStub{err: errors.New("provider-secret")}, audit)
		assertInvitationErrorResponse(t, response, http.StatusBadGateway, "zitadel_member_invitation_failed")
		if strings.Contains(response.Body.String(), "provider-secret") {
			t.Fatalf("response leaked provider error: %s", response.Body.String())
		}
		assertInvitationAudit(t, audit, memberinvite.OutcomeFailed, "zitadel_member_invitation_failed", "")
	})
}

type invitationProviderStub struct {
	invitation memberinvite.Invitation
	err        error
	onInvite   func()
}

func (p invitationProviderStub) Invite(_ context.Context, _ memberinvite.InviteRequest) (memberinvite.Invitation, error) {
	if p.onInvite != nil {
		p.onInvite()
	}
	return p.invitation, p.err
}

type invitationAuditStub struct {
	records []memberinvite.AuditRecord
	err     error
}

func (s *invitationAuditStub) Record(_ context.Context, record memberinvite.AuditRecord) error {
	s.records = append(s.records, record)
	return s.err
}

func invitationRouter(t *testing.T, provider memberinvite.Provider, audit memberinvite.AuditRepository) *gin.Engine {
	return invitationRouterWithDirectory(t, provider, audit, tenantDirectoryStub{{ID: "org-1", DisplayName: "Tenant One"}})
}

func invitationRouterWithDirectory(t *testing.T, provider memberinvite.Provider, audit memberinvite.AuditRepository, directory tenantdirectory.Directory) *gin.Engine {
	t.Helper()
	service, err := listingsubscription.NewService(listingsubscription.NewMemRepository())
	if err != nil {
		t.Fatal(err)
	}
	h, err := NewHandler(&stubHandlerCoreService{}, WithDependencies(HandlerDependencies{
		Subscription: SubscriptionDependencies{
			Service:                         service,
			TenantDirectory:                 directory,
			MemberInvitationProvider:        provider,
			MemberInvitationAuditRepository: audit,
		},
	}))
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	router.POST("/platform/tenants/:tenant_id/members/invitations", h.InviteTenantMember)
	return router
}

type tenantDirectoryErrorStub struct{}

func (tenantDirectoryErrorStub) List(context.Context) ([]tenantdirectory.Tenant, error) {
	return nil, errors.New("directory-secret")
}

func invitationRequest(tenantID, role, actorID string) *http.Request {
	body := bytes.NewBufferString(`{"given_name":"Jane","family_name":"Doe","email":"jane@example.com","role":"listingkit_viewer"}`)
	request := httptest.NewRequest(http.MethodPost, "/platform/tenants/"+tenantID+"/members/invitations", body)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-User-Roles", role)
	request.Header.Set("X-User-ID", actorID)
	return withAuthenticatedIdentity(request, "admin-tenant", actorID, role)
}

func withAuthenticatedIdentity(request *http.Request, tenantID, userID string, roles ...string) *http.Request {
	return request.WithContext(listingkit.WithAuthenticatedIdentity(request.Context(), listingkit.AuthenticatedIdentity{
		TenantID: tenantID,
		UserID:   userID,
		Roles:    roles,
	}))
}

func invokeInvitation(t *testing.T, provider memberinvite.Provider, audit memberinvite.AuditRepository) *httptest.ResponseRecorder {
	t.Helper()
	response := httptest.NewRecorder()
	invitationRouter(t, provider, audit).ServeHTTP(response, invitationRequest("org-1", "platform_admin", "admin-1"))
	return response
}

func assertInvitationErrorResponse(t *testing.T, response *httptest.ResponseRecorder, status int, errorCode string) {
	t.Helper()
	if response.Code != status {
		t.Fatalf("status = %d, want %d; body=%s", response.Code, status, response.Body.String())
	}
	var body struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Error != errorCode {
		t.Fatalf("error = %q, want %q; body=%s", body.Error, errorCode, response.Body.String())
	}
}

func assertInvitationAudit(t *testing.T, audit *invitationAuditStub, outcome memberinvite.Outcome, errorCode, userID string) {
	t.Helper()
	if len(audit.records) != 1 {
		t.Fatalf("audit records = %#v", audit.records)
	}
	record := audit.records[0]
	if record.ActorUserID != "admin-1" || record.TenantID != "org-1" || record.Email != "jane@example.com" || record.Role != "listingkit_viewer" || record.Outcome != outcome || record.ErrorCode != errorCode || record.UserID != userID {
		t.Fatalf("audit record = %#v", record)
	}
}

func TestPlatformSubscriptionCanApplyPlanForTenant(t *testing.T) {
	router := platformSubscriptionTestRouter(t)

	listReq := httptest.NewRequest(http.MethodGet, "/platform/subscription-plans", nil)
	listReq = withAuthenticatedIdentity(listReq, "admin-tenant", "admin-1", "platform_admin")
	listResp := httptest.NewRecorder()
	router.ServeHTTP(listResp, listReq)
	if listResp.Code != http.StatusOK {
		t.Fatalf("list plans status = %d, want %d; body=%s", listResp.Code, http.StatusOK, listResp.Body.String())
	}
	var listBody struct {
		Items []listingsubscription.PlanBundle `json:"items"`
	}
	if err := json.Unmarshal(listResp.Body.Bytes(), &listBody); err != nil {
		t.Fatalf("decode plans: %v", err)
	}
	if len(listBody.Items) == 0 {
		t.Fatal("plans response is empty")
	}

	body, err := json.Marshal(map[string]any{
		"plan_code": listingsubscription.PlanProfessional,
		"status":    listingsubscription.StatusActive,
	})
	if err != nil {
		t.Fatal(err)
	}
	applyReq := httptest.NewRequest(http.MethodPut, "/platform/subscriptions/org-target/plan", bytes.NewReader(body))
	applyReq.Header.Set("Content-Type", "application/json")
	applyReq = withAuthenticatedIdentity(applyReq, "admin-tenant", "admin-1", "platform_admin")
	applyResp := httptest.NewRecorder()
	router.ServeHTTP(applyResp, applyReq)
	if applyResp.Code != http.StatusOK {
		t.Fatalf("apply plan status = %d, want %d; body=%s", applyResp.Code, http.StatusOK, applyResp.Body.String())
	}

	getReq := httptest.NewRequest(http.MethodGet, "/platform/subscriptions/org-target", nil)
	getReq = withAuthenticatedIdentity(getReq, "admin-tenant", "admin-1", "platform_admin")
	getResp := httptest.NewRecorder()
	router.ServeHTTP(getResp, getReq)
	if getResp.Code != http.StatusOK {
		t.Fatalf("get status = %d, want %d; body=%s", getResp.Code, http.StatusOK, getResp.Body.String())
	}
	var summary listingsubscription.Summary
	if err := json.Unmarshal(getResp.Body.Bytes(), &summary); err != nil {
		t.Fatalf("decode summary: %v", err)
	}
	if summary.Subscription == nil || summary.Subscription.PlanCode != listingsubscription.PlanProfessional {
		t.Fatalf("summary subscription = %#v", summary.Subscription)
	}
	if summary.CurrentPlan == nil || summary.CurrentPlan.Plan.Code != listingsubscription.PlanProfessional {
		t.Fatalf("summary current plan = %#v", summary.CurrentPlan)
	}
}

func TestPlatformSubscriptionCanManagePlans(t *testing.T) {
	router := platformSubscriptionTestRouter(t)

	createBody, err := json.Marshal(map[string]any{
		"code":        "growth",
		"name":        "增长版",
		"description": "面向增长期租户",
		"sort_order":  25,
		"active":      true,
		"modules": []map[string]any{
			{"module_code": listingsubscription.ModuleStudio, "limits": map[string]int{"design_jobs": 50}, "sort_order": 10},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	createReq := httptest.NewRequest(http.MethodPost, "/platform/subscription-plans", bytes.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createReq = withAuthenticatedIdentity(createReq, "admin-tenant", "admin-1", "platform_admin")
	createResp := httptest.NewRecorder()
	router.ServeHTTP(createResp, createReq)
	if createResp.Code != http.StatusOK {
		t.Fatalf("create plan status = %d, want %d; body=%s", createResp.Code, http.StatusOK, createResp.Body.String())
	}

	moduleBody, err := json.Marshal(map[string]any{
		"limits":     map[string]int{"storage_bytes": 5368709120},
		"sort_order": 20,
	})
	if err != nil {
		t.Fatal(err)
	}
	moduleReq := httptest.NewRequest(http.MethodPut, "/platform/subscription-plans/growth/modules/oss_storage", bytes.NewReader(moduleBody))
	moduleReq.Header.Set("Content-Type", "application/json")
	moduleReq = withAuthenticatedIdentity(moduleReq, "admin-tenant", "admin-1", "platform_admin")
	moduleResp := httptest.NewRecorder()
	router.ServeHTTP(moduleResp, moduleReq)
	if moduleResp.Code != http.StatusOK {
		t.Fatalf("module status = %d, want %d; body=%s", moduleResp.Code, http.StatusOK, moduleResp.Body.String())
	}

	statusBody, err := json.Marshal(map[string]any{"active": false})
	if err != nil {
		t.Fatal(err)
	}
	statusReq := httptest.NewRequest(http.MethodPut, "/platform/subscription-plans/growth/status", bytes.NewReader(statusBody))
	statusReq.Header.Set("Content-Type", "application/json")
	statusReq = withAuthenticatedIdentity(statusReq, "admin-tenant", "admin-1", "platform_admin")
	statusResp := httptest.NewRecorder()
	router.ServeHTTP(statusResp, statusReq)
	if statusResp.Code != http.StatusOK {
		t.Fatalf("status update = %d, want %d; body=%s", statusResp.Code, http.StatusOK, statusResp.Body.String())
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/platform/subscription-plans/growth/modules/studio", nil)
	deleteReq = withAuthenticatedIdentity(deleteReq, "admin-tenant", "admin-1", "platform_admin")
	deleteResp := httptest.NewRecorder()
	router.ServeHTTP(deleteResp, deleteReq)
	if deleteResp.Code != http.StatusOK {
		t.Fatalf("delete module status = %d, want %d; body=%s", deleteResp.Code, http.StatusOK, deleteResp.Body.String())
	}

	var bundle listingsubscription.PlanBundle
	if err := json.Unmarshal(deleteResp.Body.Bytes(), &bundle); err != nil {
		t.Fatalf("decode bundle: %v", err)
	}
	if bundle.Plan.Code != "growth" || bundle.Plan.Active {
		t.Fatalf("bundle plan = %#v", bundle.Plan)
	}
	if len(bundle.Modules) != 1 || bundle.Modules[0].ModuleCode != listingsubscription.ModuleOSSStorage {
		t.Fatalf("bundle modules = %#v", bundle.Modules)
	}
}

func TestPlatformSubscriptionPlanTenantsAndAuditLogs(t *testing.T) {
	router := platformSubscriptionTestRouter(t)

	applyBody, err := json.Marshal(map[string]any{
		"plan_code": listingsubscription.PlanProfessional,
		"status":    listingsubscription.StatusActive,
	})
	if err != nil {
		t.Fatal(err)
	}
	applyReq := httptest.NewRequest(http.MethodPut, "/platform/subscriptions/org-alpha/plan", bytes.NewReader(applyBody))
	applyReq.Header.Set("Content-Type", "application/json")
	applyReq = withAuthenticatedIdentity(applyReq, "admin-tenant", "admin-1", "platform_admin")
	applyResp := httptest.NewRecorder()
	router.ServeHTTP(applyResp, applyReq)
	if applyResp.Code != http.StatusOK {
		t.Fatalf("apply status = %d, want %d; body=%s", applyResp.Code, http.StatusOK, applyResp.Body.String())
	}

	tenantsReq := httptest.NewRequest(http.MethodGet, "/platform/subscription-plans/professional/tenants", nil)
	tenantsReq = withAuthenticatedIdentity(tenantsReq, "admin-tenant", "admin-1", "platform_admin")
	tenantsResp := httptest.NewRecorder()
	router.ServeHTTP(tenantsResp, tenantsReq)
	if tenantsResp.Code != http.StatusOK {
		t.Fatalf("tenants status = %d, want %d; body=%s", tenantsResp.Code, http.StatusOK, tenantsResp.Body.String())
	}
	var tenantsBody struct {
		Items []listingsubscription.TenantSubscription `json:"items"`
	}
	if err := json.Unmarshal(tenantsResp.Body.Bytes(), &tenantsBody); err != nil {
		t.Fatalf("decode tenants: %v", err)
	}
	if len(tenantsBody.Items) != 1 || tenantsBody.Items[0].TenantID != "org-alpha" {
		t.Fatalf("tenants body = %#v", tenantsBody.Items)
	}

	auditReq := httptest.NewRequest(http.MethodGet, "/platform/subscription-plans/professional/audit-logs", nil)
	auditReq = withAuthenticatedIdentity(auditReq, "admin-tenant", "admin-1", "platform_admin")
	auditResp := httptest.NewRecorder()
	router.ServeHTTP(auditResp, auditReq)
	if auditResp.Code != http.StatusOK {
		t.Fatalf("audit status = %d, want %d; body=%s", auditResp.Code, http.StatusOK, auditResp.Body.String())
	}
	var auditBody struct {
		Items []listingsubscription.AuditLog `json:"items"`
	}
	if err := json.Unmarshal(auditResp.Body.Bytes(), &auditBody); err != nil {
		t.Fatalf("decode audit: %v", err)
	}
	if len(auditBody.Items) == 0 || auditBody.Items[0].Action != "plan_apply" {
		t.Fatalf("audit body = %#v", auditBody.Items)
	}
}

func TestPlatformSubscriptionCanUseConfiguredAdminUser(t *testing.T) {
	router := platformSubscriptionTestRouterWithOptions(t, WithPlatformSubscriptionAccess([]string{"admin-user"}, nil))

	req := httptest.NewRequest(http.MethodGet, "/platform/subscriptions/org-target", nil)
	req.Header.Set("X-User-ID", "admin-user")
	req = withAuthenticatedIdentity(req, "admin-tenant", "admin-user")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", resp.Code, http.StatusOK, resp.Body.String())
	}
}

func platformSubscriptionTestRouter(t *testing.T) *gin.Engine {
	return platformSubscriptionTestRouterWithOptions(t)
}

func platformSubscriptionTestRouterWithOptions(t *testing.T, opts ...HandlerOption) *gin.Engine {
	t.Helper()
	repo := listingsubscription.NewMemRepository()
	service, err := listingsubscription.NewService(repo)
	if err != nil {
		t.Fatalf("create subscription service: %v", err)
	}
	baseOpts := []HandlerOption{WithSubscriptionService(service)}
	baseOpts = append(baseOpts, opts...)
	h, err := NewHandler(&stubHandlerCoreService{}, baseOpts...)
	if err != nil {
		t.Fatalf("create handler: %v", err)
	}
	router := gin.New()
	router.GET("/platform/subscriptions", h.ListPlatformTenantSubscriptions)
	router.GET("/platform/subscription-plans", h.ListPlatformSubscriptionPlans)
	router.POST("/platform/subscription-plans", h.UpsertPlatformSubscriptionPlan)
	router.PUT("/platform/subscription-plans/:plan_code", h.UpsertPlatformSubscriptionPlan)
	router.PUT("/platform/subscription-plans/:plan_code/modules/:module_code", h.UpsertPlatformSubscriptionPlanModule)
	router.DELETE("/platform/subscription-plans/:plan_code/modules/:module_code", h.DeletePlatformSubscriptionPlanModule)
	router.PUT("/platform/subscription-plans/:plan_code/status", h.SetPlatformSubscriptionPlanStatus)
	router.GET("/platform/subscription-plans/:plan_code/tenants", h.ListPlatformSubscriptionPlanTenants)
	router.GET("/platform/subscription-plans/:plan_code/audit-logs", h.ListPlatformSubscriptionPlanAuditLogs)
	router.GET("/platform/subscriptions/:tenant_id", h.GetPlatformTenantSubscription)
	router.PUT("/platform/subscriptions/:tenant_id/plan", h.ApplyPlatformTenantSubscriptionPlan)
	router.PUT("/platform/subscriptions/:tenant_id/entitlements/:module_code", h.UpsertPlatformTenantSubscriptionEntitlement)
	return router
}

type subscriptionDisplayNameResolver map[string]string

func (r subscriptionDisplayNameResolver) ResolveTenantDisplayName(_ context.Context, tenantID string) (string, error) {
	return r[tenantID], nil
}
