package httpapi

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"strings"
	"task-processor/internal/agent"
	"task-processor/internal/agentconfig"
	"task-processor/internal/authidentity"
	"task-processor/internal/httproute"
	"task-processor/internal/imageagent"
	"task-processor/internal/product/asset"
	"testing"
	"time"
)

type imageSetRequestCapabilitySource struct{ calls int }

func (s *imageSetRequestCapabilitySource) ReadImageSetSource(ctx context.Context, id imageagent.ExecutionIdentity, input imageagent.PrepareImageSetInput) (imageagent.ImageSetPreparation, error) {
	s.calls++
	capability, ok := ctx.Value(productReviewCapabilityContextKey{}).(productReviewRequestCapability)
	if !ok || capability.bearerToken != "controlled-request-token" || capability.actorID != id.UserID || capability.effectiveOrganizationID != id.TenantID || input.ContextID != id.BusinessTaskID {
		return imageagent.ImageSetPreparation{}, imageagent.ErrIdentityRequired
	}
	return imageagent.ImageSetPreparation{Source: imageagent.ImageSourceBinding{OperationID: input.ContextID}}, nil
}

func TestFullImageHTTPBindsActualRequestCredentialAndRejectsInvalidSessions(t *testing.T) {
	const sourceID = "d1abe8da-b381-4924-8d15-d79bdbfacf70"
	now := time.Now()
	for _, test := range []struct {
		name, authorization string
		expires             time.Time
		status, calls       int
	}{
		{"valid", "Bearer controlled-request-token", now.Add(time.Hour), 200, 1},
		{"missing", "", now.Add(time.Hour), 403, 0},
		{"expired", "Bearer controlled-request-token", now, 403, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			source := &imageSetRequestCapabilitySource{}
			service, err := imageagent.NewService(imageSetHTTPRepository{}, imageSetHTTPWorkflow{}, closedImageSetCatalog{}, imageagent.WithOrganizationScope())
			require.NoError(t, err)
			module := fullImageModule{application: &fullImageApplication{service: service, readSources: source}, bind: (productReviewCapabilityBinder{now: func() time.Time { return now }}).Bind}
			router := gin.New()
			for _, route := range module.routes() {
				router.Handle(route.Method, route.Path, route.Handler)
			}
			request := httptest.NewRequest(http.MethodGet, "/api/v1/workbench/sourcing/1688/acquisitions/"+sourceID+"/images/sources", nil)
			request.Header.Set("Authorization", test.authorization)
			request = request.WithContext(authidentity.WithAuthenticatedIdentity(request.Context(), authidentity.AuthenticatedIdentity{TenantID: "org", EffectiveOrganizationID: "org", UserID: "actor", EffectiveMemberID: "member", TokenExpiresAt: test.expires}))
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			require.Equal(t, test.status, response.Code, response.Body.String())
			require.Equal(t, test.calls, source.calls)
		})
	}
}

func TestFullImageTemplateReferenceUsesTheOriginalScopedSnapshot(t *testing.T) {
	snapshot := agentconfig.ImageConfigurationSnapshot{ID: "33333333-3333-4333-8333-333333333333", Digest: strings.Repeat("a", 64), Scope: agent.Scope{OrganizationID: "org", ActorID: "actor"}, MemberID: "member", RunID: "run", ContextID: "source", Template: agentconfig.TemplateRef{TemplateID: "55555555-5555-4555-8555-555555555555", Revision: "2"}}
	p := imageagent.RunProjection{Run: imageagent.Run{ID: "run", TenantID: "org", UserID: "actor", MemberID: "member", BusinessTaskID: "source"}, Plan: imageagent.Plan{Set: &imageagent.ImageSetPlan{Configuration: snapshot.Ref()}}}
	ref, err := imageSetTemplateReference(p, snapshot)
	require.NoError(t, err)
	require.Equal(t, snapshot.Template, ref)
	for _, alter := range []func(*agentconfig.ImageConfigurationSnapshot){
		func(s *agentconfig.ImageConfigurationSnapshot) { s.Scope.OrganizationID = "other" },
		func(s *agentconfig.ImageConfigurationSnapshot) { s.Scope.ActorID = "other" },
		func(s *agentconfig.ImageConfigurationSnapshot) { s.MemberID = "other" },
		func(s *agentconfig.ImageConfigurationSnapshot) { s.RunID = "other" },
		func(s *agentconfig.ImageConfigurationSnapshot) { s.ContextID = "other" },
		func(s *agentconfig.ImageConfigurationSnapshot) { s.Digest = strings.Repeat("b", 64) },
	} {
		changed := snapshot
		alter(&changed)
		_, err = imageSetTemplateReference(p, changed)
		require.ErrorIs(t, err, imageagent.ErrCommandBlocked)
	}
}

func TestFullImageRunRequiresOriginalMemberAndExplicitSourceOwner(t *testing.T) {
	identity := authidentity.AuthenticatedIdentity{TenantID: "org", EffectiveOrganizationID: "org", UserID: "actor", EffectiveMemberID: "member"}
	p := imageagent.RunProjection{Run: imageagent.Run{ScopeProtocol: imageagent.OrganizationScopeProtocol, ID: "run", TenantID: "org", UserID: "actor", MemberID: "member", BusinessTaskID: "context"}, Plan: imageagent.Plan{Set: &imageagent.ImageSetPlan{Source: imageagent.ImageSourceBinding{ContextKind: imageagent.ImageSourceAcquisition, OperationID: "context"}}}}
	require.NoError(t, validateFullImageRun(identity, imageagent.ImageSourceAcquisition, "context", "run", p))
	require.Error(t, validateFullImageRun(identity, imageagent.ImageSourceSupply, "context", "run", p))
	identity.EffectiveMemberID = "replacement"
	require.Error(t, validateFullImageRun(identity, imageagent.ImageSourceAcquisition, "context", "run", p))
}

func TestFullImageSelectionBodyRejectsProviderURLsAndDuplicateActions(t *testing.T) {
	for _, body := range []string{`{"actionId":"a","url":"https://example.com/a.png"}`, `{"actionId":"a","actionId":"b"}`, `{"actionId":"a","choices":[{"kind":"generated","url":"https://example.com/a.png"}]}`} {
		request := httptest.NewRequest("POST", "/", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		var input fullImageSelectionBody
		require.ErrorIs(t, readFullImageJSON(request, &input), imageagent.ErrValidation)
	}
}

func TestFullImageManualPermissionDenialIsKnownAndNeverAnUnknownMutation(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	writeFullImageError(c, asset.ErrSourceApprovalForbidden, true)
	require.Equal(t, 403, w.Code)
	require.JSONEq(t, `{"code":"FORBIDDEN"}`, w.Body.String())
}

type imageSetHTTPRepository struct {
	imageagent.Repository
	projection imageagent.RunProjection
}

func (r imageSetHTTPRepository) GetProjection(_ context.Context, s imageagent.RunScope) (imageagent.RunProjection, error) {
	if s != imageagent.ScopeForRun(r.projection.Run) {
		return imageagent.RunProjection{}, imageagent.ErrRunNotFound
	}
	return r.projection, nil
}

type imageSetHTTPSource struct {
	binding imageagent.ImageSourceBinding
	err     error
}

func (r *imageSetHTTPSource) ReadImageSetSource(_ context.Context, _ imageagent.ExecutionIdentity, input imageagent.PrepareImageSetInput) (imageagent.ImageSetPreparation, error) {
	if r.err != nil {
		return imageagent.ImageSetPreparation{}, r.err
	}
	return imageagent.ImageSetPreparation{Source: r.binding}, nil
}
func TestFullImageHTTPReadsOriginalApprovalWithoutASecondMutation(t *testing.T) {
	const sourceID = "d1abe8da-b381-4924-8d15-d79bdbfacf70"
	const runID = "30d26689-30b6-4358-b0f5-c310d7ab2e58"
	const actionID = "b912e7d4-df80-44a5-8510-3cdf91d5b8dd"
	binding := imageagent.ImageSourceBinding{ContextKind: imageagent.ImageSourceAcquisition, ProductID: "product", OperationID: sourceID, OriginalPublicationID: "publication", OriginalVersion: 1, EffectiveVersion: 2, ApplyReceiptID: actionID}
	projection := imageagent.RunProjection{Run: imageagent.Run{ID: runID, TenantID: "org", UserID: "actor", MemberID: "member", BusinessTaskID: sourceID, ScopeProtocol: imageagent.OrganizationScopeProtocol}, Plan: imageagent.Plan{Set: &imageagent.ImageSetPlan{Source: binding, Target: imageagent.ImageTarget{Platform: "product"}}}}
	service, err := imageagent.NewService(imageSetHTTPRepository{projection: projection}, imageSetHTTPWorkflow{}, closedImageSetCatalog{}, imageagent.WithOrganizationScope())
	require.NoError(t, err)
	sources := &imageSetHTTPSource{binding: binding}
	approvals := &acquisitionImageApprovalReaderSpy{commit: asset.ApprovalCommit{TenantID: "org", ProductKey: "product", TargetPlatform: "product", ActionID: actionID, SourceSnapshotVersion: 2, ImageSet: &asset.ImageSetSelection{Digest: strings.Repeat("a", 64), Source: asset.SourceSelectionRequest{ContextKind: "acquisition", ItemID: sourceID, OriginalPublicationID: "publication", OriginalSnapshotVersion: 1, EffectiveCatalogVersion: 2, ApplyReceiptID: actionID, TargetPlatform: "product"}}, Assets: []asset.ApprovedAsset{{ID: "selected", Role: asset.RoleMain, URL: "https://images.test/1.png"}}}}
	module := fullImageModule{application: &fullImageApplication{service: service, readSources: sources, approvals: approvals}, bind: func(ctx context.Context, _ string) (context.Context, error) { return ctx, nil }}
	router := gin.New()
	for _, route := range module.routes() {
		router.Handle(route.Method, route.Path, route.Handler)
	}
	id := authidentity.AuthenticatedIdentity{TenantID: "org", EffectiveOrganizationID: "org", UserID: "actor", EffectiveMemberID: "member"}
	call := func(identity authidentity.AuthenticatedIdentity) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodGet, "/api/v1/workbench/sourcing/1688/acquisitions/"+sourceID+"/images/runs/"+runID+"/approvals/"+actionID, nil)
		r = r.WithContext(authidentity.WithAuthenticatedIdentity(r.Context(), identity))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		return w
	}
	w := call(id)
	require.Equal(t, 200, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), actionID)
	require.Equal(t, 1, approvals.calls)
	approvals.commit.ImageSet.Source.ApplyReceiptID = "other"
	w = call(id)
	require.Equal(t, 409, w.Code)
	approvals.commit.ImageSet.Source.ApplyReceiptID = actionID
	changed := id
	changed.EffectiveMemberID = "replacement"
	w = call(changed)
	require.Equal(t, 403, w.Code)
	changed = id
	changed.EffectiveOrganizationID = "other"
	w = call(changed)
	require.Equal(t, 403, w.Code)
}
func TestFullImageRoutesHaveExactLiveOrganizationAdmission(t *testing.T) {
	routes := []httproute.Descriptor{}
	for _, r := range currentWorkbenchApplicationRoutes {
		routes = append(routes, httproute.Descriptor{Method: r.Method, Path: r.Path})
	}
	full := fullImageModule{}.routes()
	routes = append(routes, full...)
	require.NoError(t, validateCurrentApplicationRoutesInternal(routes, false, false, false, false, false, false, false, currentApplicationOptionalRoutes{FullImageSet: true}))
	for i := range full {
		changed := append([]httproute.Descriptor(nil), routes...)
		changed[len(currentWorkbenchApplicationRoutes)+i].OrganizationAccessPolicy = httproute.OrganizationAccessPolicyCachedRead
		require.Error(t, validateCurrentApplicationRoutesInternal(changed, false, false, false, false, false, false, false, currentApplicationOptionalRoutes{FullImageSet: true}))
	}
}

type imageSetHTTPWorkflow struct{ imageagent.WorkflowClient }

func TestFullImageRestartRejectsChangedPlanSourceAndMemberBeforeWorkflowStart(t *testing.T) {
	const sourceID = "d1abe8da-b381-4924-8d15-d79bdbfacf70"
	const runID = "30d26689-30b6-4358-b0f5-c310d7ab2e58"
	hash := strings.Repeat("a", 64)
	binding := imageagent.ImageSourceBinding{ContextKind: imageagent.ImageSourceAcquisition, ProductID: "product", OperationID: sourceID, OriginalPublicationID: "publication", OriginalVersion: 1, EffectiveVersion: 1, CatalogHash: "catalog-v2:" + hash}
	plan := imageagent.Plan{Revision: 1, IdempotencyKey: "original-plan", SourceAssetIDs: []string{"source-1"}, CreatedBy: "actor", Set: &imageagent.ImageSetPlan{Schema: imageagent.ImageSetSchema, Source: binding, Target: imageagent.ImageTarget{Platform: "product"}, Configuration: agent.ConfigurationSnapshotRef{Kind: "agent-configuration-v1", ID: sourceID, Digest: hash}, ConfigurationEpoch: "1", ParametersDigest: hash, InputDigest: hash, MaxPoints: 20}, Slots: []imageagent.Slot{{ID: "overview", Role: imageagent.SlotRoleDetail, Status: imageagent.SlotStatusPending, IdempotencyKey: "original-overview", SourceAssetIDs: []string{"source-1"}, Recipe: &imageagent.ImageSlotRecipe{Purpose: "product_overview", Background: "white", Language: "en", Placement: imageagent.ImagePlacement{Group: "detail", Order: 1}, PromptVersion: imageagent.ImageSetSchema, Prompt: "Use the original product image", References: []imageagent.ImageSourceObservation{{AssetID: "source-1", SHA256: hash, MediaType: "image/png", Bytes: 10, Width: 1024, Height: 1024}}, Quote: imageagent.ImageGenerationQuote{Provider: "grsai", Model: "gpt-image-2.5", Protocol: "grsai-json-sync-v1", Resolution: "1024x1024", Quality: "auto", PriceVersion: "price", Points: 20, RouteReference: "route", CredentialReference: "credential", ConfigurationVersion: "config"}}}}}
	var err error
	plan.Set.QuoteDigest, err = imageagent.ImageSetQuoteDigest(plan)
	require.NoError(t, err)
	digest, err := imageagent.ImageSetPlanDigest(plan)
	require.NoError(t, err)
	projection := imageagent.RunProjection{Run: imageagent.Run{ID: runID, TenantID: "org", UserID: "actor", MemberID: "member", BusinessTaskID: sourceID, ScopeProtocol: imageagent.OrganizationScopeProtocol, Status: imageagent.RunStatusFailed}, Plan: plan}
	service, err := imageagent.NewService(imageSetHTTPRepository{projection: projection}, imageSetHTTPWorkflow{}, closedImageSetCatalog{}, imageagent.WithOrganizationScope())
	require.NoError(t, err)
	sources := &imageSetHTTPSource{binding: binding}
	module := fullImageModule{application: &fullImageApplication{service: service, readSources: sources}, bind: func(ctx context.Context, _ string) (context.Context, error) { return ctx, nil }}
	router := gin.New()
	for _, route := range module.routes() {
		router.Handle(route.Method, route.Path, route.Handler)
	}
	for _, kind := range []string{"changed_revision", "unknown_field", "changed_source", "changed_member", "other_organization"} {
		t.Run(kind, func(t *testing.T) {
			id := authidentity.AuthenticatedIdentity{TenantID: "org", EffectiveOrganizationID: "org", UserID: "actor", EffectiveMemberID: "member"}
			sources.binding = binding
			body := `{"planRevision":2,"planDigest":"` + digest + `","quoteDigest":"` + plan.Set.QuoteDigest + `"}`
			status := http.StatusConflict
			switch kind {
			case "unknown_field":
				body = strings.TrimSuffix(body, "}") + `,"deadline":"later"}`
				status = http.StatusBadRequest
			case "changed_source":
				sources.binding.OriginalVersion++
			case "changed_member":
				id.EffectiveMemberID = "replacement"
				status = http.StatusForbidden
			case "other_organization":
				id.EffectiveOrganizationID = "other"
				status = http.StatusForbidden
			}
			r := httptest.NewRequest(http.MethodPost, "/api/v1/workbench/sourcing/1688/acquisitions/"+sourceID+"/images/runs/"+runID+"/restart", strings.NewReader(body))
			r.Header.Set("Content-Type", "application/json")
			r = r.WithContext(authidentity.WithAuthenticatedIdentity(r.Context(), id))
			w := httptest.NewRecorder()
			router.ServeHTTP(w, r)
			require.Equal(t, status, w.Code, w.Body.String())
		})
	}
}
