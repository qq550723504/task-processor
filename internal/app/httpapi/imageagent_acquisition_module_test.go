package httpapi

import (
	"context"
	"errors"
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
	"task-processor/internal/core/config"
	"task-processor/internal/httproute"
	"task-processor/internal/imageagent"
	imagestore "task-processor/internal/imageagent/store"
	imagetemporal "task-processor/internal/imageagent/temporal"
	imageagenttools "task-processor/internal/imageagent/tools"
	productasset "task-processor/internal/product/asset"
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
	require.Equal(t, imageagent.Budget{MaxImages: 1, EnabledLimits: imageagent.BudgetLimitImages}, input.Budget)
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
	imageRoutes := acquisitionImageRoutes(&acquisitionImageServiceSpy{}, &acquisitionImageCandidatesSpy{}, &acquisitionImageApprovalReaderSpy{}, func(ctx context.Context, _ string) (context.Context, error) { return ctx, nil }, acquisitionImagePublicURLs{})
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
	starts      []imageagent.StartRunInput
	projection  imageagent.RunProjection
	approvals   int
	mutationErr error
}

func (spy *acquisitionImageServiceSpy) Start(_ context.Context, input imageagent.StartRunInput) error {
	spy.starts = append(spy.starts, input)
	return spy.mutationErr
}
func (spy *acquisitionImageServiceSpy) Get(context.Context, string) (imageagent.RunProjection, error) {
	return spy.projection, nil
}
func (spy *acquisitionImageServiceSpy) ApproveResults(context.Context, string, int64, string, string) error {
	spy.approvals++
	return spy.mutationErr
}

type acquisitionImageCandidatesSpy struct {
	calls int
	scope imageagent.AssetCatalogScope
}

type acquisitionImageApprovalReaderSpy struct {
	commit productasset.ApprovalCommit
	calls  int
	err    error
}

func (spy *acquisitionImageApprovalReaderSpy) ReadApprovalCommit(_ context.Context, tenantID, actionID string) (productasset.ApprovalCommit, error) {
	spy.calls++
	if spy.err != nil {
		return productasset.ApprovalCommit{}, spy.err
	}
	if spy.commit.TenantID != tenantID || spy.commit.ActionID != actionID {
		return productasset.ApprovalCommit{}, productasset.ErrApprovedAssetsNotReady
	}
	return spy.commit, nil
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

type trialAcquisitionImagePublicURLs struct{ base string }

func (r trialAcquisitionImagePublicURLs) PublicURL(key string) string { return r.base + "/" + key }

func TestAcquisitionImageTrialResultDerivesOnlyExactPublishedAssetURL(t *testing.T) {
	const base = "https://localhost:19444/image-agent-assets/issue487-images"
	policy, err := imageagent.NewIsolatedTrialGeneratedURLPolicy(base, "issue487-images")
	require.NoError(t, err)
	owner, err := imageagent.ArtifactOwnerKey("actor-a")
	require.NoError(t, err)
	hash := strings.Repeat("a", 64)
	asset := imageagent.DurableAssetIdentity{ObjectKey: "image-agent/public/org-a/" + owner + "/run-a/1/main/1/0-" + hash + ".png", SHA256: hash}
	projection := imageagent.RunProjection{Run: imageagent.Run{ID: "run-a", TenantID: "org-a", UserID: "actor-a", Status: imageagent.RunStatusAwaitingFinalApproval}, Plan: imageagent.Plan{Revision: 1, Slots: []imageagent.Slot{{ID: "main", Role: imageagent.SlotRoleMain}}}, Slots: []imageagent.SlotProjection{{Slot: imageagent.Slot{ID: "main", Role: imageagent.SlotRoleMain}, Attempt: 1, Candidates: []imageagent.AssetCandidate{{AssetID: "asset-a", DurableAsset: asset}}}}}
	result, err := acquisitionImageResult(projection, trialAcquisitionImagePublicURLs{base}, policy)
	require.NoError(t, err)
	require.Equal(t, base+"/"+asset.ObjectKey, result["imageUrl"])
	_, err = acquisitionImageResult(projection, trialAcquisitionImagePublicURLs{"https://localhost:19445/image-agent-assets/issue487-images"}, policy)
	require.Error(t, err)
	projection.Slots[0].Candidates[0].URL = base + "/" + asset.ObjectKey
	_, err = acquisitionImageResult(projection, trialAcquisitionImagePublicURLs{base}, policy)
	require.ErrorIs(t, err, imageagent.ErrCommandBlocked, "even an exact-looking stored URL cannot replace durable identity resolution")
	projection.Slots[0].Candidates[0].URL = ""
	projection.Slots[0].Candidates[0].DurableAsset.ObjectKey = strings.Replace(asset.ObjectKey, "/org-a/", "/org-b/", 1)
	_, err = acquisitionImageResult(projection, trialAcquisitionImagePublicURLs{base}, policy)
	require.Error(t, err)
}

func TestAcquisitionImageRoutesRejectForgedURLCandidateOnReadAndCompletedReplay(t *testing.T) {
	const operationID = "d1abe8da-b381-4924-8d15-d79bdbfacf70"
	const runID = "30d26689-30b6-4358-b0f5-c310d7ab2e58"
	const actionID = "d05f2a61-c5cc-48ed-9dfe-3746522d4f8d"
	const forgedURL = "https://images.example.test/forged.png"
	identity := authidentity.AuthenticatedIdentity{TenantID: "org-a", EffectiveOrganizationID: "org-a", UserID: "actor-a", EffectiveMemberID: "member-a"}
	slot := imageagent.Slot{ID: "main", Role: imageagent.SlotRoleMain, SourceAssetIDs: []string{"catalog-image-1"}, IdempotencyKey: "slot-1", Status: imageagent.SlotStatusPending}
	service := &acquisitionImageServiceSpy{projection: imageagent.RunProjection{
		Run:  imageagent.Run{ScopeProtocol: imageagent.OrganizationScopeProtocol, ID: runID, TenantID: identity.TenantID, UserID: identity.UserID, MemberID: identity.EffectiveMemberID, BusinessTaskID: operationID, TargetPlatform: "product", ImagePolicyContext: imageagent.ImagePolicyContext{Country: "zz", Family: "default", SceneCategory: "general"}, Status: imageagent.RunStatusCompleted, ActivePlanRevision: 1},
		Plan: imageagent.Plan{Revision: 1, IdempotencyKey: "plan-1", SourceAssetIDs: []string{"catalog-image-1"}, Slots: []imageagent.Slot{slot}}, ResultDigest: "digest-1",
		AssetCatalog: imageagent.AssetCatalog{ProductContext: imageagent.ProductContextRef{ProductID: "product-a", SourceSnapshotVersion: 1}},
		Slots:        []imageagent.SlotProjection{{Slot: slot, Attempt: 1, Candidates: []imageagent.AssetCandidate{{AssetID: "generated-1", URL: forgedURL}}}},
	}}
	require.NoError(t, imageagent.ValidateProjectionSnapshot(imageagent.ScopeForRun(service.projection.Run), service.projection), "URL-only representation must be readable by the current projection store")
	approvalReader := &acquisitionImageApprovalReaderSpy{commit: productasset.ApprovalCommit{
		TenantID: identity.TenantID, ProductKey: "product-a", TargetPlatform: "product", SourceSnapshotVersion: 1,
		ActionID: imagetemporal.ApprovalActionPublicationKey(actionID, runID, 1),
		Assets:   []productasset.ApprovedAsset{{ID: "generated-1", RunID: runID, PlanRevision: 1, SlotID: "main", Attempt: 1, Role: productasset.RoleMain, URL: forgedURL}},
	}}
	router := gin.New()
	for _, route := range acquisitionImageRoutes(service, &acquisitionImageCandidatesSpy{}, approvalReader, func(ctx context.Context, _ string) (context.Context, error) { return ctx, nil }, acquisitionImagePublicURLs{}) {
		router.Handle(route.Method, route.Path, route.Handler)
	}
	base := "/api/v1/workbench/sourcing/1688/acquisitions/" + operationID + "/main-image/runs/" + runID
	call := func(method, path, body string) *httptest.ResponseRecorder {
		t.Helper()
		request := httptest.NewRequest(method, path, strings.NewReader(body))
		request = request.WithContext(authidentity.WithAuthenticatedIdentity(request.Context(), identity))
		request.Header.Set("Authorization", "Bearer fixture")
		if method == http.MethodPost {
			request.Header.Set("Content-Type", "application/json")
		}
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		return response
	}
	t.Run("read", func(t *testing.T) {
		response := call(http.MethodGet, base, "")
		require.Equal(t, http.StatusConflict, response.Code, "current organization read must not project a persisted arbitrary URL")
		require.NotContains(t, response.Body.String(), forgedURL)
	})
	t.Run("completed_replay", func(t *testing.T) {
		response := call(http.MethodPost, base+"/approve", `{"planRevision":1,"resultDigest":"digest-1","actionId":"`+actionID+`"}`)
		require.Equal(t, http.StatusConflict, response.Code, "completed replay must not bless a URL-only projection")
		require.Zero(t, service.approvals)
		require.Zero(t, approvalReader.calls, "forged current projection should be rejected before the immutable owner read")
	})
}

func TestAcquisitionImageNewGenerationUnavailableBeforeServiceStart(t *testing.T) {
	const operationID = "d1abe8da-b381-4924-8d15-d79bdbfacf70"
	const requestID = "30d26689-30b6-4358-b0f5-c310d7ab2e58"
	for _, tc := range []struct {
		name     string
		identity authidentity.AuthenticatedIdentity
		bindErr  error
		status   int
		code     string
	}{
		{"authorized", authidentity.AuthenticatedIdentity{TenantID: "org-a", EffectiveOrganizationID: "org-a", UserID: "actor-a", EffectiveMemberID: "member-a"}, nil, http.StatusServiceUnavailable, "IMAGE_UNAVAILABLE"},
		{"missing_identity", authidentity.AuthenticatedIdentity{}, nil, http.StatusForbidden, "FORBIDDEN"},
		{"cross_org", authidentity.AuthenticatedIdentity{TenantID: "org-a", EffectiveOrganizationID: "org-b", UserID: "actor-a", EffectiveMemberID: "member-a"}, nil, http.StatusForbidden, "FORBIDDEN"},
		{"live_authorization_rejected", authidentity.AuthenticatedIdentity{TenantID: "org-a", EffectiveOrganizationID: "org-a", UserID: "actor-a", EffectiveMemberID: "member-a"}, imageagent.ErrIdentityRequired, http.StatusForbidden, "FORBIDDEN"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			service := &acquisitionImageServiceSpy{}
			router := gin.New()
			for _, route := range acquisitionImageRoutes(service, &acquisitionImageCandidatesSpy{}, &acquisitionImageApprovalReaderSpy{}, func(ctx context.Context, _ string) (context.Context, error) { return ctx, tc.bindErr }, acquisitionImagePublicURLs{}) {
				router.Handle(route.Method, route.Path, route.Handler)
			}
			request := httptest.NewRequest(http.MethodPost, "/api/v1/workbench/sourcing/1688/acquisitions/"+operationID+"/main-image", strings.NewReader(`{"sourceImageId":"catalog-image-3"}`))
			request = request.WithContext(authidentity.WithAuthenticatedIdentity(request.Context(), tc.identity))
			request.Header.Set("Authorization", "Bearer fixture")
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Idempotency-Key", requestID)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			require.Equal(t, tc.status, response.Code, response.Body.String())
			require.JSONEq(t, `{"code":"`+tc.code+`"}`, response.Body.String())
			require.Empty(t, service.starts, "no run initialization or downstream workflow start, regardless of available member quota")
		})
	}
}

func TestAcquisitionImageConfiguredGenerationUsesExactCatalogAndStableStart(t *testing.T) {
	const operationID = "d1abe8da-b381-4924-8d15-d79bdbfacf70"
	const requestID = "30d26689-30b6-4358-b0f5-c310d7ab2e58"
	identity := authidentity.AuthenticatedIdentity{TenantID: "org-a", EffectiveOrganizationID: "org-a", UserID: "actor-a", EffectiveMemberID: "member-a"}
	service, catalog := &acquisitionImageServiceSpy{}, &acquisitionImageCandidatesSpy{}
	router := gin.New()
	for _, route := range acquisitionImageRoutes(service, catalog, &acquisitionImageApprovalReaderSpy{}, func(ctx context.Context, _ string) (context.Context, error) { return ctx, nil }, acquisitionImagePublicURLs{}, acquisitionImageRouteOptions{Price: config.ImageAgentGenerationConfig{PriceVersion: "price-1", PointsPerImage: 12}}) {
		router.Handle(route.Method, route.Path, route.Handler)
	}
	base := "/api/v1/workbench/sourcing/1688/acquisitions/" + operationID + "/main-image"
	call := func(method, path, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r = r.WithContext(authidentity.WithAuthenticatedIdentity(r.Context(), identity))
		r.Header.Set("Authorization", "Bearer fixture")
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Idempotency-Key", requestID)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		return w
	}
	w := call(http.MethodGet, base+"/candidates", "")
	require.Equal(t, 200, w.Code, w.Body.String())
	require.JSONEq(t, `{"operationId":"`+operationID+`","candidates":[{"id":"catalog-image-3","displayUrl":"https://images.example.test/3.png"}]}`, w.Body.String())
	require.Equal(t, imageagent.AssetCatalogScope{TenantID: "org-a", OwnerUserID: "actor-a", BusinessTaskID: operationID}, catalog.scope)
	for i := 0; i < 2; i++ {
		w = call(http.MethodPost, base, `{"sourceImageId":"catalog-image-3"}`)
		require.Equal(t, 202, w.Code, w.Body.String())
	}
	require.Len(t, service.starts, 2)
	require.Equal(t, service.starts[0], service.starts[1], "replay retains one stable run identity")
	require.Equal(t, []string{"catalog-image-3"}, service.starts[0].Plan.SourceAssetIDs)
	service.mutationErr = errors.New("workflow start acknowledgement lost; internal detail")
	w = call(http.MethodPost, base, `{"sourceImageId":"catalog-image-3"}`)
	require.Equal(t, 503, w.Code)
	require.JSONEq(t, `{"code":"OUTCOME_UNKNOWN"}`, w.Body.String())
	require.Equal(t, service.starts[0], service.starts[2])
}

func TestAcquisitionImageRoutesCloseNewGenerationAndPreserveHumanApproval(t *testing.T) {
	const operationID = "d1abe8da-b381-4924-8d15-d79bdbfacf70"
	const requestID = "30d26689-30b6-4358-b0f5-c310d7ab2e58"
	identity := authidentity.AuthenticatedIdentity{TenantID: "org-a", EffectiveOrganizationID: "org-a", UserID: "actor-a", EffectiveMemberID: "member-a"}
	service := &acquisitionImageServiceSpy{}
	catalog := &acquisitionImageCandidatesSpy{}
	approvalReader := &acquisitionImageApprovalReaderSpy{}
	router := gin.New()
	for _, route := range acquisitionImageRoutes(service, catalog, approvalReader, func(ctx context.Context, _ string) (context.Context, error) { return ctx, nil }, acquisitionImagePublicURLs{}) {
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
	require.Equal(t, http.StatusServiceUnavailable, response.Code, response.Body.String())
	require.JSONEq(t, `{"code":"IMAGE_UNAVAILABLE"}`, response.Body.String())
	require.Zero(t, catalog.calls, "new-generation discovery is closed, not a second receipt reader")
	response = call(http.MethodPost, base, `{"sourceImageId":"catalog-image-3","targetPlatform":"shein"}`)
	require.Equal(t, http.StatusBadRequest, response.Code)
	require.Empty(t, service.starts)
	response = call(http.MethodPost, base, `{"sourceImageId":"catalog-image-1","sourceImageId":"catalog-image-3"}`)
	require.Equal(t, http.StatusBadRequest, response.Code)
	require.Empty(t, service.starts)
	response = call(http.MethodPost, base, `{"sourceImageId":"catalog-image-3"}`)
	require.Equal(t, http.StatusServiceUnavailable, response.Code, response.Body.String())
	require.JSONEq(t, `{"code":"IMAGE_UNAVAILABLE"}`, response.Body.String())
	require.Empty(t, service.starts, "even a service ready to accept must not initialize a run or dispatch a workflow")
	// Existing durable runs remain readable and approvable independently of new-generation readiness.
	input, err := acquisitionMainRunInput(identity, operationID, requestID, "catalog-image-3")
	require.NoError(t, err)
	ownerKey, err := imageagent.ArtifactOwnerKey(identity.UserID)
	require.NoError(t, err)
	asset := imageagent.DurableAssetIdentity{ObjectKey: "image-agent/public/org-a/" + ownerKey + "/" + input.RunID + "/1/main/1/0-" + strings.Repeat("a", 64) + ".png", SHA256: strings.Repeat("a", 64)}
	service.projection = imageagent.RunProjection{Run: imageagent.Run{ScopeProtocol: imageagent.OrganizationScopeProtocol, ID: input.RunID, TenantID: identity.TenantID, UserID: identity.UserID, MemberID: identity.EffectiveMemberID, BusinessTaskID: operationID, TargetPlatform: "product", ImagePolicyContext: input.ImagePolicyContext, Status: imageagent.RunStatusAwaitingFinalApproval}, Plan: input.Plan, ResultDigest: "digest-1", AssetCatalog: imageagent.AssetCatalog{ProductContext: imageagent.ProductContextRef{ProductID: "product-a", SourceSnapshotVersion: 1}}, Slots: []imageagent.SlotProjection{{Slot: input.Plan.Slots[0], Attempt: 1, Candidates: []imageagent.AssetCandidate{{AssetID: "generated-1", DurableAsset: asset}}}}}
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
	approvalReader.commit = productasset.ApprovalCommit{TenantID: identity.TenantID, ProductKey: "product-a", TargetPlatform: "product", SourceSnapshotVersion: 1, ActionID: imagetemporal.ApprovalActionPublicationKey(requestID, input.RunID, 1), Assets: []productasset.ApprovedAsset{{ID: "generated-1", RunID: input.RunID, PlanRevision: 1, SlotID: input.Plan.Slots[0].ID, Attempt: 1, Role: productasset.RoleMain, URL: "https://images.example.test/generated.png"}}}
	response = call(http.MethodPost, base+"/runs/"+input.RunID+"/approve", `{"planRevision":1,"resultDigest":"digest-1","actionId":"`+requestID+`"}`)
	require.Equal(t, http.StatusAccepted, response.Code, "completed action must verify immutable Product Asset fact without updating closed Temporal")
	require.Equal(t, 1, approvalReader.calls)
	require.Equal(t, 1, service.approvals)
	response = call(http.MethodPost, base+"/runs/"+input.RunID+"/approve", `{"planRevision":1,"resultDigest":"digest-1","actionId":"d05f2a61-c5cc-48ed-9dfe-3746522d4f8d"}`)
	require.Equal(t, http.StatusConflict, response.Code, "new action ID cannot inherit a completed approval")
	require.Equal(t, 1, service.approvals)
	identity.EffectiveMemberID = "replaced-member"
	reads := approvalReader.calls
	response = call(http.MethodPost, base+"/runs/"+input.RunID+"/approve", `{"planRevision":1,"resultDigest":"digest-1","actionId":"`+requestID+`"}`)
	require.Equal(t, http.StatusNotFound, response.Code, "a replacement grant cannot replay an old member's completed approval")
	require.Equal(t, reads, approvalReader.calls)
	identity.EffectiveMemberID = "member-a"
	approvalReader.err = productasset.ErrRepositoryUnavailable
	response = call(http.MethodPost, base+"/runs/"+input.RunID+"/approve", `{"planRevision":1,"resultDigest":"digest-1","actionId":"`+requestID+`"}`)
	require.Equal(t, http.StatusServiceUnavailable, response.Code, "owner read failure must never be treated as successful replay")
	approvalReader.err = nil
	approvalReader.commit.Assets[0].URL = "https://images.example.test/different.png"
	response = call(http.MethodPost, base+"/runs/"+input.RunID+"/approve", `{"planRevision":1,"resultDigest":"digest-1","actionId":"`+requestID+`"}`)
	require.Equal(t, http.StatusConflict, response.Code, "same action with different approved payload cannot replay")
	service.projection.Run.Status = imageagent.RunStatusAwaitingFinalApproval
	service.mutationErr = errors.New("Temporal update acknowledgement lost")
	response = call(http.MethodPost, base+"/runs/"+input.RunID+"/approve", `{"planRevision":1,"resultDigest":"digest-1","actionId":"`+requestID+`"}`)
	require.Equal(t, http.StatusServiceUnavailable, response.Code)
	require.JSONEq(t, `{"code":"OUTCOME_UNKNOWN"}`, response.Body.String())
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
	require.EqualValues(t, 1, quote.Maximum.Images, "only the single source-based white-background edit consumes an image unit")
	reservation := imageagent.SlotEffectV3Reservation{Identity: imageagent.SlotExternalEffectIdentity{RunScope: scope, PlanRevision: 1, SlotID: "main", Attempt: 1}, IdempotencyKey: execution.IdempotencyKey, InputFingerprint: imageagent.SlotExecutionFingerprint(execution), Policy: policy, Quote: quote}
	attempt, claimed, err := repository.(imageagent.SlotExternalEffectV3Repository).ReserveSlotProviderV3(context.Background(), reservation)
	require.NoError(t, err, "the server-owned main-image budget must admit the real pre-provider quote")
	require.True(t, claimed)
	require.Equal(t, imageagent.SlotBudgetReserved, attempt.BudgetStatus)
}
