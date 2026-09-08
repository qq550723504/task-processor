package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"task-processor/internal/authidentity"
	"task-processor/internal/sourceaccountregistry"
)

func TestCreateProjectsStrictCurrentResource(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Date(2026, 9, 9, 1, 2, 3, 0, time.UTC)
	account := httpTestAccount(t, now)
	service := &fakeService{registerResult: sourceaccountregistry.MutationResult{Account: account}}
	handler := mustHandler(t, service)
	key := uuid.NewString()

	recorder := httptest.NewRecorder()
	ctx := testContext(recorder, http.MethodPost, "/api/v1/workbench/source-accounts", `{"displayName":"Primary","platform":"1688"}`, now, "org-b")
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Request.Header.Set("Idempotency-Key", key)
	handler.Create(ctx)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("Create() status = %d body=%s", recorder.Code, recorder.Body.String())
	}
	assertProtectedHeaders(t, recorder)
	if recorder.Header().Get("ETag") != `"1"` {
		t.Fatalf("ETag = %q", recorder.Header().Get("ETag"))
	}
	var body MutationResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.SchemaVersion != 1 || body.Replayed || body.Account.ID != account.ID || body.Account.Version != "1" || body.Account.ConnectionStatus != "pending_connection" {
		t.Fatalf("response = %#v", body)
	}
	if strings.Contains(recorder.Body.String(), "organization") || strings.Contains(recorder.Body.String(), "createdBy") || strings.Contains(recorder.Body.String(), "actor-1") {
		t.Fatalf("private fields leaked: %s", recorder.Body.String())
	}
	if service.registerKey != key || service.registerInput != (sourceaccountregistry.RegisterInput{DisplayName: "Primary", Platform: "1688"}) {
		t.Fatalf("register call = key %q input %#v", service.registerKey, service.registerInput)
	}
}

func TestCreateReplayIs200AndInputIsStrict(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Date(2026, 9, 9, 1, 2, 3, 0, time.UTC)
	service := &fakeService{registerResult: sourceaccountregistry.MutationResult{Account: httpTestAccount(t, now), Replayed: true}}
	handler := mustHandler(t, service)

	recorder := httptest.NewRecorder()
	ctx := testContext(recorder, http.MethodPost, "/api/v1/workbench/source-accounts", `{"displayName":"Primary","platform":"1688"}`, now, "org-b")
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Request.Header.Set("Idempotency-Key", uuid.NewString())
	handler.Create(ctx)
	if recorder.Code != http.StatusOK {
		t.Fatalf("replay status = %d body=%s", recorder.Code, recorder.Body.String())
	}

	tests := []struct {
		name        string
		body        string
		contentType string
		query       string
	}{
		{name: "duplicate", body: `{"displayName":"A","displayName":"B","platform":"1688"}`, contentType: "application/json"},
		{name: "unknown", body: `{"displayName":"A","platform":"1688","connected":true}`, contentType: "application/json"},
		{name: "missing", body: `{"displayName":"A"}`, contentType: "application/json"},
		{name: "wrong content type", body: `{"displayName":"A","platform":"1688"}`, contentType: "text/plain"},
		{name: "query", body: `{"displayName":"A","platform":"1688"}`, contentType: "application/json", query: "legacy=true"},
		{name: "too large", body: strings.Repeat("a", requestBodyMaxBytes+1), contentType: "application/json"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := service.registerCalls
			recorder := httptest.NewRecorder()
			path := "/api/v1/workbench/source-accounts"
			if tt.query != "" {
				path += "?" + tt.query
			}
			ctx := testContext(recorder, http.MethodPost, path, tt.body, now, "org-b")
			ctx.Request.Header.Set("Content-Type", tt.contentType)
			ctx.Request.Header.Set("Idempotency-Key", uuid.NewString())
			handler.Create(ctx)
			want := http.StatusBadRequest
			if tt.name == "too large" {
				want = http.StatusRequestEntityTooLarge
			}
			if recorder.Code != want || service.registerCalls != before {
				t.Fatalf("Create() status=%d calls=%d body=%s", recorder.Code, service.registerCalls, recorder.Body.String())
			}
			assertProtectedHeaders(t, recorder)
		})
	}
}

func TestListUsesStableOrganizationBoundCursor(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Date(2026, 9, 9, 1, 2, 3, 0, time.UTC)
	account := httpTestAccount(t, now)
	service := &fakeService{page: sourceaccountregistry.Page{Items: []sourceaccountregistry.Account{account}, Next: &sourceaccountregistry.PagePosition{CreatedAt: account.CreatedAt, ID: account.ID}}}
	handler := mustHandler(t, service)

	recorder := httptest.NewRecorder()
	ctx := testContext(recorder, http.MethodGet, "/api/v1/workbench/source-accounts?limit=1", "", now, "org-b")
	handler.List(ctx)
	if recorder.Code != http.StatusOK {
		t.Fatalf("List() status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var page PageResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || page.NextCursor == nil || *page.NextCursor == "" || service.pageRequest.Limit != 1 {
		t.Fatalf("page = %#v request=%#v", page, service.pageRequest)
	}

	service.page = sourceaccountregistry.Page{Items: []sourceaccountregistry.Account{}}
	recorder = httptest.NewRecorder()
	ctx = testContext(recorder, http.MethodGet, "/api/v1/workbench/source-accounts?limit=1&cursor="+*page.NextCursor, "", now, "org-b")
	handler.List(ctx)
	if recorder.Code != http.StatusOK || service.pageRequest.After == nil || service.pageRequest.After.ID != account.ID || !service.pageRequest.After.CreatedAt.Equal(account.CreatedAt) {
		t.Fatalf("cursor request status=%d request=%#v body=%s", recorder.Code, service.pageRequest, recorder.Body.String())
	}

	before := service.listCalls
	recorder = httptest.NewRecorder()
	ctx = testContext(recorder, http.MethodGet, "/api/v1/workbench/source-accounts?cursor="+*page.NextCursor, "", now, "org-a")
	handler.List(ctx)
	if recorder.Code != http.StatusBadRequest || service.listCalls != before {
		t.Fatalf("cross-org cursor status=%d calls=%d body=%s", recorder.Code, service.listCalls, recorder.Body.String())
	}
}

func TestDetailAndLifecycleUseVersionedCurrentContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Date(2026, 9, 9, 1, 2, 3, 0, time.UTC)
	account := httpTestAccount(t, now)
	service := &fakeService{account: account}
	handler := mustHandler(t, service)

	recorder := httptest.NewRecorder()
	ctx := testContext(recorder, http.MethodGet, "/api/v1/workbench/source-accounts/"+account.ID, "", now, "org-b")
	ctx.Params = gin.Params{{Key: "source_account_id", Value: account.ID}}
	handler.Get(ctx)
	if recorder.Code != http.StatusOK || recorder.Header().Get("ETag") != `"1"` || service.getID != account.ID {
		t.Fatalf("Get() status=%d etag=%q id=%q body=%s", recorder.Code, recorder.Header().Get("ETag"), service.getID, recorder.Body.String())
	}

	disabled := account
	disabled.ManagementStatus = sourceaccountregistry.ManagementStatusDisabled
	disabled.Version = 2
	disabled.UpdatedAt = now.Add(time.Minute)
	service.mutation = sourceaccountregistry.MutationResult{Account: disabled}
	key := uuid.NewString()
	recorder = httptest.NewRecorder()
	ctx = testContext(recorder, http.MethodPost, "/api/v1/workbench/source-accounts/"+account.ID+"/disable", "", now, "org-b")
	ctx.Params = gin.Params{{Key: "source_account_id", Value: account.ID}}
	ctx.Request.Header.Set("Idempotency-Key", key)
	ctx.Request.Header.Set("If-Match", `"1"`)
	handler.Disable(ctx)
	if recorder.Code != http.StatusOK || recorder.Header().Get("ETag") != `"2"` || service.lifecycleKind != "disable" || service.lifecycleKey != key || service.expectedVersion != 1 {
		t.Fatalf("Disable() status=%d etag=%q call=%s/%s/%d body=%s", recorder.Code, recorder.Header().Get("ETag"), service.lifecycleKind, service.lifecycleKey, service.expectedVersion, recorder.Body.String())
	}
}

func TestHandlerMapsErrorsWithoutDependencyLeakage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Date(2026, 9, 9, 1, 2, 3, 0, time.UTC)
	tests := []struct {
		err    error
		status int
		code   string
	}{
		{sourceaccountregistry.ErrInvalid, 400, "INVALID_REQUEST"},
		{sourceaccountregistry.ErrForbidden, 403, "PERMISSION_DENIED"},
		{sourceaccountregistry.ErrNotFound, 404, "SOURCE_ACCOUNT_NOT_FOUND"},
		{sourceaccountregistry.ErrIdempotencyConflict, 409, "IDEMPOTENCY_CONFLICT"},
		{sourceaccountregistry.ErrVersionConflict, 409, "VERSION_CONFLICT"},
		{sourceaccountregistry.ErrInvalidTransition, 409, "INVALID_TRANSITION"},
		{sourceaccountregistry.ErrResourceLimitReached, 409, "RESOURCE_LIMIT_REACHED"},
		{sourceaccountregistry.ErrTooLarge, 413, "INPUT_TOO_LARGE"},
		{sourceaccountregistry.ErrOutcomeUnknown, 503, "OUTCOME_UNKNOWN"},
		{context.DeadlineExceeded, 504, "DEADLINE_EXCEEDED"},
		{errors.New("database secret detail"), 503, "DEPENDENCY_UNAVAILABLE"},
	}
	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			service := &fakeService{getErr: tt.err}
			handler := mustHandler(t, service)
			recorder := httptest.NewRecorder()
			id := httpTestAccount(t, now).ID
			ctx := testContext(recorder, http.MethodGet, "/api/v1/workbench/source-accounts/"+id, "", now, "org-b")
			ctx.Params = gin.Params{{Key: "source_account_id", Value: id}}
			handler.Get(ctx)
			if recorder.Code != tt.status || !strings.Contains(recorder.Body.String(), `"code":"`+tt.code+`"`) || strings.Contains(recorder.Body.String(), "secret") {
				t.Fatalf("Get() status=%d body=%s", recorder.Code, recorder.Body.String())
			}
			assertProtectedHeaders(t, recorder)
		})
	}
}

func testContext(recorder *httptest.ResponseRecorder, method, target, body string, now time.Time, org string) *gin.Context {
	ctx, _ := gin.CreateTestContext(recorder)
	request := httptest.NewRequest(method, target, bytes.NewBufferString(body))
	identity := authidentity.AuthenticatedIdentity{TenantID: org, EffectiveOrganizationID: org, UserID: "actor-1", Roles: []string{"listingkit_operator"}, TokenExpiresAt: now.Add(time.Hour)}
	ctx.Request = request.WithContext(authidentity.WithAuthenticatedIdentity(request.Context(), identity))
	return ctx
}

func mustHandler(t *testing.T, service Service) *Handler {
	t.Helper()
	handler, err := NewHandler(service)
	if err != nil {
		t.Fatal(err)
	}
	return handler
}

func httpTestAccount(t *testing.T, now time.Time) sourceaccountregistry.Account {
	t.Helper()
	id, err := uuid.NewV7()
	if err != nil {
		t.Fatal(err)
	}
	account, err := sourceaccountregistry.NewAccount(id.String(), "org-b", "actor-1", "Primary", sourceaccountregistry.Platform1688, now)
	if err != nil {
		t.Fatal(err)
	}
	return account
}

func assertProtectedHeaders(t *testing.T, recorder *httptest.ResponseRecorder) {
	t.Helper()
	if recorder.Header().Get("Cache-Control") != "no-store" || recorder.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("protected headers = %#v", recorder.Header())
	}
}

type fakeService struct {
	registerResult  sourceaccountregistry.MutationResult
	registerErr     error
	registerCalls   int
	registerKey     string
	registerInput   sourceaccountregistry.RegisterInput
	account         sourceaccountregistry.Account
	getErr          error
	getID           string
	page            sourceaccountregistry.Page
	pageErr         error
	listCalls       int
	pageRequest     sourceaccountregistry.PageRequest
	mutation        sourceaccountregistry.MutationResult
	mutationErr     error
	lifecycleKind   string
	lifecycleKey    string
	lifecycleID     string
	expectedVersion int64
}

func (s *fakeService) Register(_ context.Context, key string, input sourceaccountregistry.RegisterInput) (sourceaccountregistry.MutationResult, error) {
	s.registerCalls++
	s.registerKey, s.registerInput = key, input
	return s.registerResult, s.registerErr
}

func (s *fakeService) Get(_ context.Context, id string) (sourceaccountregistry.Account, error) {
	s.getID = id
	return s.account, s.getErr
}

func (s *fakeService) List(_ context.Context, request sourceaccountregistry.PageRequest) (sourceaccountregistry.Page, error) {
	s.listCalls++
	s.pageRequest = request
	return s.page, s.pageErr
}

func (s *fakeService) Enable(_ context.Context, key, id string, version int64) (sourceaccountregistry.MutationResult, error) {
	s.lifecycleKind, s.lifecycleKey, s.lifecycleID, s.expectedVersion = "enable", key, id, version
	return s.mutation, s.mutationErr
}

func (s *fakeService) Disable(_ context.Context, key, id string, version int64) (sourceaccountregistry.MutationResult, error) {
	s.lifecycleKind, s.lifecycleKey, s.lifecycleID, s.expectedVersion = "disable", key, id, version
	return s.mutation, s.mutationErr
}
