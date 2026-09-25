package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	imageagentpolicy "task-processor/internal/app/imageagentpolicy"
	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	"task-processor/internal/httproute"
	"task-processor/internal/imageagent"
	imagestore "task-processor/internal/imageagent/store"
	imageagenttools "task-processor/internal/imageagent/tools"
)

func TestAcquisitionMainRunInputOwnsSingleSourcePolicyAndStableIdempotency(t *testing.T) {
	identity := authidentity.AuthenticatedIdentity{TenantID: "org-a", EffectiveOrganizationID: "org-a", UserID: "actor-a", EffectiveMemberID: "member-a"}
	requestID := "30d26689-30b6-4358-b0f5-c310d7ab2e58"
	operationID := "d1abe8da-b381-4924-8d15-d79bdbfacf70"
	input, err := acquisitionMainRunInput(identity, operationID, requestID, "catalog-image-3")
	require.NoError(t, err)
	require.Equal(t, operationID, input.BusinessTaskID)
	require.Equal(t, "product", input.TargetPlatform)
	require.Equal(t, imageagent.ImagePolicyContext{Country: "zz", Family: "default", SceneCategory: "general"}, input.ImagePolicyContext)
	require.Equal(t, imageagent.RunModeManual, input.Mode)
	require.Equal(t, 1, input.MaxConcurrentSlots)
	require.Equal(t, imageagent.Budget{MaxImages: 3, EnabledLimits: imageagent.BudgetLimitImages}, input.Budget)
	require.Equal(t, []string{"catalog-image-3"}, input.Plan.SourceAssetIDs)
	require.Len(t, input.Plan.Slots, 1)
	require.Equal(t, imageagent.SlotRoleMain, input.Plan.Slots[0].Role)
	require.Equal(t, []string{"catalog-image-3"}, input.Plan.Slots[0].SourceAssetIDs)
	require.Equal(t, imageagent.SlotStatusPending, input.Plan.Slots[0].Status)
	require.NotEmpty(t, input.RunID)
	again, err := acquisitionMainRunInput(identity, operationID, requestID, "catalog-image-3")
	require.NoError(t, err)
	require.Equal(t, input, again)
	changedSource, err := acquisitionMainRunInput(identity, operationID, requestID, "catalog-image-1")
	require.NoError(t, err)
	require.Equal(t, input.RunID, changedSource.RunID, "same request must conflict at the persisted run, not fork provider work")
	require.NotEqual(t, input.Plan, changedSource.Plan)
	changedActor, err := acquisitionMainRunInput(authidentity.AuthenticatedIdentity{TenantID: "org-a", EffectiveOrganizationID: "org-a", UserID: "actor-b", EffectiveMemberID: "member-b"}, operationID, requestID, "catalog-image-3")
	require.NoError(t, err)
	require.NotEqual(t, input.RunID, changedActor.RunID)
	_, err = acquisitionMainRunInput(identity, operationID, requestID, "https://browser.example/image.png")
	require.ErrorIs(t, err, imageagent.ErrValidation)
}

func TestCurrentApplicationAdmitsOnlyLiveAuthorizedAcquisitionImageRoutes(t *testing.T) {
	routes := make([]httproute.Descriptor, 0, len(currentWorkbenchApplicationRoutes)+4)
	for _, route := range currentWorkbenchApplicationRoutes {
		routes = append(routes, httproute.Descriptor{Method: route.Method, Path: route.Path})
	}
	imageRoutes := acquisitionImageRoutes(&acquisitionImageServiceSpy{}, &acquisitionImageCandidatesSpy{}, func(ctx context.Context, _ string) (context.Context, error) { return ctx, nil }, acquisitionImagePublicURLs{})
	routes = append(routes, imageRoutes...)
	require.Error(t, validateCurrentApplicationRoutesInternal(routes, false, false, false, false, false, false, false), "disabled image module cannot leak routes")
	require.NoError(t, validateCurrentApplicationRoutesInternal(routes, false, false, false, false, false, false, false, true))
	for i := range imageRoutes {
		for _, mutate := range []func(*httproute.Descriptor){
			func(route *httproute.Descriptor) { route.Module = "other" },
			func(route *httproute.Descriptor) { route.Permission = authz.PermissionWorkbenchSourceAccountRead },
			func(route *httproute.Descriptor) {
				route.OrganizationAccessPolicy = httproute.OrganizationAccessPolicyCachedRead
			},
		} {
			changed := append([]httproute.Descriptor(nil), routes...)
			mutate(&changed[len(currentWorkbenchApplicationRoutes)+i])
			require.Error(t, validateCurrentApplicationRoutesInternal(changed, false, false, false, false, false, false, false, true))
		}
	}
}

type acquisitionImageServiceSpy struct {
	starts     []imageagent.StartRunInput
	projection imageagent.RunProjection
	approvals  int
}

func (spy *acquisitionImageServiceSpy) Start(_ context.Context, input imageagent.StartRunInput) error {
	spy.starts = append(spy.starts, input)
	return nil
}
func (spy *acquisitionImageServiceSpy) Get(context.Context, string) (imageagent.RunProjection, error) {
	return spy.projection, nil
}
func (spy *acquisitionImageServiceSpy) ApproveResults(context.Context, string, int64, string, string) error {
	spy.approvals++
	return nil
}

type acquisitionImageCandidatesSpy struct {
	calls int
	scope imageagent.AssetCatalogScope
}

func (spy *acquisitionImageCandidatesSpy) Candidates(_ context.Context, scope imageagent.AssetCatalogScope) ([]imageagent.AuthorizedAsset, error) {
	spy.calls++
	spy.scope = scope
	return []imageagent.AuthorizedAsset{{ID: "catalog-image-3", DisplayURL: "https://images.example.test/3.png"}}, nil
}

type acquisitionImagePublicURLs struct{}

func (acquisitionImagePublicURLs) PublicURL(string) string {
	return "https://images.example.test/generated.png"
}

func TestAcquisitionImageRoutesAcceptOnlyNarrowServerOwnedStartAndHumanApproval(t *testing.T) {
	const operationID = "d1abe8da-b381-4924-8d15-d79bdbfacf70"
	const requestID = "30d26689-30b6-4358-b0f5-c310d7ab2e58"
	identity := authidentity.AuthenticatedIdentity{TenantID: "org-a", EffectiveOrganizationID: "org-a", UserID: "actor-a", EffectiveMemberID: "member-a"}
	service := &acquisitionImageServiceSpy{}
	catalog := &acquisitionImageCandidatesSpy{}
	router := gin.New()
	for _, route := range acquisitionImageRoutes(service, catalog, func(ctx context.Context, _ string) (context.Context, error) { return ctx, nil }, acquisitionImagePublicURLs{}) {
		router.Handle(route.Method, route.Path, route.Handler)
	}
	call := func(method, path, body string) *httptest.ResponseRecorder {
		t.Helper()
		request := httptest.NewRequest(method, path, strings.NewReader(body))
		request = request.WithContext(authidentity.WithAuthenticatedIdentity(request.Context(), identity))
		request.Header.Set("Authorization", "Bearer fixture")
		if method == http.MethodPost {
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Idempotency-Key", requestID)
		}
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		return response
	}
	base := "/api/v1/workbench/sourcing/1688/acquisitions/" + operationID + "/main-image"
	response := call(http.MethodGet, base+"/candidates", "")
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	require.JSONEq(t, `{"operationId":"`+operationID+`","candidates":[{"id":"catalog-image-3","displayUrl":"https://images.example.test/3.png"}]}`, response.Body.String())
	require.Equal(t, operationID, catalog.scope.BusinessTaskID)
	response = call(http.MethodPost, base, `{"sourceImageId":"catalog-image-3","targetPlatform":"shein"}`)
	require.Equal(t, http.StatusBadRequest, response.Code)
	require.Empty(t, service.starts)
	response = call(http.MethodPost, base, `{"sourceImageId":"catalog-image-1","sourceImageId":"catalog-image-3"}`)
	require.Equal(t, http.StatusBadRequest, response.Code)
	require.Empty(t, service.starts)
	response = call(http.MethodPost, base, `{"sourceImageId":"catalog-image-3"}`)
	require.Equal(t, http.StatusAccepted, response.Code, response.Body.String())
	require.Len(t, service.starts, 1)
	input := service.starts[0]
	service.projection = imageagent.RunProjection{Run: imageagent.Run{ScopeProtocol: imageagent.OrganizationScopeProtocol, ID: input.RunID, TenantID: identity.TenantID, UserID: identity.UserID, MemberID: identity.EffectiveMemberID, BusinessTaskID: operationID, TargetPlatform: "product", ImagePolicyContext: input.ImagePolicyContext, Status: imageagent.RunStatusAwaitingFinalApproval}, Plan: input.Plan, ResultDigest: "digest-1"}
	identity.EffectiveMemberID = "replaced-member"
	response = call(http.MethodGet, base+"/runs/"+input.RunID, "")
	require.Equal(t, http.StatusNotFound, response.Code, "a replacement grant must not read the old member's image or digest")
	response = call(http.MethodPost, base+"/runs/"+input.RunID+"/approve", `{"planRevision":1,"resultDigest":"digest-1","actionId":"`+requestID+`"}`)
	require.Equal(t, http.StatusNotFound, response.Code, "a replacement grant must not approve the old member's image")
	require.Zero(t, service.approvals)
	identity.EffectiveMemberID = "member-a"
	response = call(http.MethodGet, base+"/runs/"+input.RunID, "")
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	require.Contains(t, response.Body.String(), `"approvalAvailable":true`)
	response = call(http.MethodPost, base+"/runs/"+input.RunID+"/approve", `{"planRevision":1,"resultDigest":"wrong","actionId":"`+requestID+`"}`)
	require.Equal(t, http.StatusConflict, response.Code)
	require.Zero(t, service.approvals)
	response = call(http.MethodPost, base+"/runs/"+input.RunID+"/approve", `{"planRevision":1,"resultDigest":"digest-1","actionId":"`+requestID+`"}`)
	require.Equal(t, http.StatusAccepted, response.Code, response.Body.String())
	require.Equal(t, 1, service.approvals)
	service.projection.Run.Status = imageagent.RunStatusCompleted
	response = call(http.MethodPost, base+"/runs/"+input.RunID+"/approve", `{"planRevision":1,"resultDigest":"digest-1","actionId":"`+requestID+`"}`)
	require.Equal(t, http.StatusAccepted, response.Code, "lost approval response must reach the workflow's idempotent action owner")
	require.Equal(t, 2, service.approvals)
}

func TestAcquisitionMainImageBudgetAdmitsRealMainQuoteBeforeProvider(t *testing.T) {
	identity := authidentity.AuthenticatedIdentity{TenantID: "org-a", EffectiveOrganizationID: "org-a", UserID: "actor-a", EffectiveMemberID: "member-a"}
	input, err := acquisitionMainRunInput(identity, "d1abe8da-b381-4924-8d15-d79bdbfacf70", "30d26689-30b6-4358-b0f5-c310d7ab2e58", "catalog-image-3")
	require.NoError(t, err)
	policyResolver, err := imageagentpolicy.LoadEmbeddedResolver()
	require.NoError(t, err)
	executor := imageagenttools.NewProductImageSlotExecutor(imageagenttools.Dependencies{UsageQuoter: &chain1ImagePorts{}, ProfileResolver: policyResolver})
	catalog, err := imageagent.NormalizeAssetCatalog(imageagent.AssetCatalog{
		Assets:         []imageagent.AuthorizedAsset{{ID: "catalog-image-3", Type: imageagent.AuthorizedAssetSource, URL: "https://images.example.test/3.png", SourceURL: "https://images.example.test/3.png", Width: 800, Height: 800}},
		ProductContext: imageagent.ProductContextRef{ProductID: "crawler:1688:123", Title: "Product", SourceSnapshotVersion: 1},
	})
	require.NoError(t, err)
	policy, err := input.Budget.Policy()
	require.NoError(t, err)
	run := imageagent.Run{ScopeProtocol: imageagent.OrganizationScopeProtocol, ID: input.RunID, BusinessTaskID: input.BusinessTaskID, TargetPlatform: input.TargetPlatform, ImagePolicyContext: input.ImagePolicyContext, TenantID: identity.TenantID, UserID: identity.UserID, MemberID: identity.EffectiveMemberID, Mode: input.Mode, IdempotencyKey: input.IdempotencyKey, Status: imageagent.RunStatusExecuting, ActivePlanRevision: 1, Version: 1, Budget: input.Budget, StartedAt: time.Now().UTC()}
	plan := input.Plan
	plan.CreatedBy = identity.UserID
	repository := imagestore.NewMemoryRepository()
	scope := imageagent.ScopeForRun(run)
	_, err = repository.InitializeRun(context.Background(), imageagent.ProjectionInitialization{Scope: scope, Run: run, Plan: plan, Catalog: catalog, Snapshot: imageagent.RunProjection{Run: run, Plan: plan}, CommitID: "start:" + input.IdempotencyKey, EventType: "run.initialized", EventPayload: []byte(`{}`)})
	require.NoError(t, err)
	execution := imageagent.SlotExecutionInput{RunID: input.RunID, TenantID: identity.TenantID, UserID: identity.UserID, TargetPlatform: input.TargetPlatform, ImagePolicyContext: &input.ImagePolicyContext, PlanRevision: 1, Slot: plan.Slots[0], Attempt: 1, IdempotencyKey: plan.Slots[0].IdempotencyKey + ":plan:1:attempt:1", AssetCatalog: catalog, ProductContext: catalog.ProductContext}
	quote, err := executor.QuoteSlot(context.Background(), execution, policy)
	require.NoError(t, err)
	require.EqualValues(t, 3, quote.Maximum.Images, "Extract, RenderWhiteBackground, and Review each consume one quoted image unit")
	reservation := imageagent.SlotEffectV3Reservation{Identity: imageagent.SlotExternalEffectIdentity{RunScope: scope, PlanRevision: 1, SlotID: "main", Attempt: 1}, IdempotencyKey: execution.IdempotencyKey, InputFingerprint: imageagent.SlotExecutionFingerprint(execution), Policy: policy, Quote: quote}
	attempt, claimed, err := repository.(imageagent.SlotExternalEffectV3Repository).ReserveSlotProviderV3(context.Background(), reservation)
	require.NoError(t, err, "the server-owned main-image budget must admit the real pre-provider quote")
	require.True(t, claimed)
	require.Equal(t, imageagent.SlotBudgetReserved, attempt.BudgetStatus)
}
