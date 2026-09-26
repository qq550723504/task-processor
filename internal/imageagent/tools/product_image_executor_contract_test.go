package tools

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"task-processor/internal/imageagent"
	imagepolicy "task-processor/internal/marketplace/imagepolicy"
	productimage "task-processor/internal/product/image"
)

func TestExecutorResolvesOnlyExplicitRunPolicyKeyAndCarriesDefaults(t *testing.T) {
	resolver := &recordingImageProfileResolver{profile: testImageProfile()}
	renderer := &recordingProductSceneRenderer{candidates: []productimage.Candidate{testSceneCandidate(t, "https://source.example/item.png")}}
	executor := NewProductImageSlotExecutor(Dependencies{
		SubjectExtractor: testProductSubjectExtractor{}, WhiteBackgroundRenderer: testProductWhiteRenderer{},
		SceneRenderer: renderer, Reviewer: testProductReviewer{}, UsageQuoter: testProductUsageQuoter{}, ProfileResolver: resolver,
	})
	input := testProductImageExecutionInput()
	input.ProductContext.ProductType = "misleading-category"
	input.ProductContext.Title = "must not select policy"

	generated, err := executor.GenerateSlot(context.Background(), input)

	require.NoError(t, err)
	require.Equal(t, imagepolicy.ProfileInput{
		Marketplace: "shein", Country: "us", Family: "default", SceneCategory: "shoes",
	}, resolver.input)
	require.Equal(t, "shoes", renderer.request.Options.SceneCategory)
	require.Equal(t, "lifestyle", renderer.request.Options.SceneStyle)
	require.Equal(t, string(imageagent.SlotRoleScene), renderer.request.Options.SlotRole)
	require.Equal(t, "show the product in use", renderer.request.Options.SlotBrief)
	require.Equal(t, "misleading-category", renderer.request.Product.ProductType, "product facts are rendering input, not policy input")
	require.Equal(t, testTinyPNG(t), generated.Assets[0].Bytes)
	require.Equal(t, "image/png", generated.Assets[0].ContentType)
}

func TestGenericMainQuotesOnlyOneSourceEditWithoutExtractOrReview(t *testing.T) {
	input := testProductImageExecutionInput()
	input.TargetPlatform = "product"
	input.ImagePolicyContext = &imageagent.ImagePolicyContext{Country: "zz", Family: "default", SceneCategory: "general"}
	input.Slot.Role = imageagent.SlotRoleMain
	profile := testImageProfile()
	profile.Key = imagepolicy.PolicyKey{Marketplace: "product", Country: "zz", Family: "default", SceneCategory: "general"}
	executor := NewProductImageSlotExecutor(Dependencies{UsageQuoter: testProductUsageQuoter{}, ProfileResolver: &recordingImageProfileResolver{profile: profile}})
	quote, err := executor.QuoteSlot(context.Background(), input, imageagent.BudgetPolicy{})
	require.NoError(t, err)
	require.Len(t, quote.Operations, 1)
	require.Equal(t, "render_source_white_background", quote.Operations[0].Name)
	require.EqualValues(t, 1, quote.Maximum.Images)
	require.EqualValues(t, 1, quote.Maximum.ModelCalls)
}

func TestGenericMainGeneratesOneSourceEditWithoutReviewerOrExtractor(t *testing.T) {
	input := testProductImageExecutionInput()
	input.TargetPlatform = "product"
	input.ImagePolicyContext = &imageagent.ImagePolicyContext{Country: "zz", Family: "default", SceneCategory: "general"}
	input.Slot.Role = imageagent.SlotRoleMain
	profile := testImageProfile()
	profile.Key = imagepolicy.PolicyKey{Marketplace: "product", Country: "zz", Family: "default", SceneCategory: "general"}
	profile.Thresholds.WhiteBackgroundReview = 1
	white := &recordingProductWhiteRenderer{candidate: testWhiteCandidate(t, "https://source.example/item.png")}
	white.candidate.Asset.Operations = []string{productimage.SourceWhiteBackgroundOperation}
	executor := NewProductImageSlotExecutor(Dependencies{WhiteBackgroundRenderer: white, UsageQuoter: testProductUsageQuoter{}, ProfileResolver: &recordingImageProfileResolver{profile: profile}})
	quote, err := executor.QuoteSlot(context.Background(), input, imageagent.BudgetPolicy{})
	require.NoError(t, err)
	output, err := executor.GenerateQuotedSlot(context.Background(), input, quote)
	require.NoError(t, err)
	require.True(t, white.request.SourceOnly)
	require.Equal(t, productimage.Candidate{}, white.request.Subject)
	require.Equal(t, "source-1", white.request.Source.SourceAssetID)
	require.Len(t, output.Assets, 1)
	require.Equal(t, []string{productimage.SourceWhiteBackgroundOperation}, output.Assets[0].Operations)
	require.EqualValues(t, 1, output.UsageReceipt.Actual.Images)
	require.EqualValues(t, 1, output.UsageReceipt.Actual.ModelCalls)
	require.Equal(t, 1, white.calls)
	oldQuote := quote
	oldQuote.Fingerprint = "prior-extract-render-review-quote"
	_, err = executor.GenerateQuotedSlot(context.Background(), input, oldQuote)
	require.ErrorIs(t, err, imageagent.ErrRevisionConflict)
	require.Equal(t, 1, white.calls, "changed flow quote must not dispatch again")
	_, err = executor.QuoteStagedReview(context.Background(), input, imageagent.BudgetPolicy{})
	require.Error(t, err, "this flow must not quote a hidden Review on recovery")
}

func TestExecutorReviewsGeneratedCandidatesBeforeAcceptance(t *testing.T) {
	profile := testImageProfile()
	profile.Thresholds.MainReview = 0.80
	reviewer := &recordingProductReviewer{review: productimage.Review{Score: 0.55, Reasons: []string{"product mismatch"}}}
	executor := NewProductImageSlotExecutor(Dependencies{
		SubjectExtractor: testProductSubjectExtractor{}, WhiteBackgroundRenderer: testProductWhiteRenderer{},
		SceneRenderer: &recordingProductSceneRenderer{candidates: []productimage.Candidate{testSceneCandidate(t, "https://source.example/item.png")}},
		Reviewer:      reviewer, UsageQuoter: testProductUsageQuoter{},
		ProfileResolver: &recordingImageProfileResolver{profile: profile},
	})

	generated, err := executor.GenerateSlot(context.Background(), testProductImageExecutionInput())

	require.ErrorIs(t, err, imageagent.ErrValidation)
	require.Len(t, generated.Assets, 1, "quality review must retain generated output for operator inspection")
	require.Len(t, reviewer.request.Candidates, 1)
	require.Equal(t, "product-1", reviewer.request.Product.ProductKey)
}

func TestExecutorReviewRetryPinsAndAuthorizesTheReviewRoute(t *testing.T) {
	reviewer := &recordingProductReviewer{review: productimage.Review{Score: 1}}
	quoter := &recordingProductUsageQuoter{quote: productimage.UsageQuote{
		Provider: "openai", RouteReference: "review-route", Model: "review-model",
		CredentialReference: "review-credential", ConfigurationVersion: "review-config", PricingVersion: "review-pricing",
		Fingerprint: "review-provider-quote", MaximumOutputs: 1, MaximumModelCalls: 1,
		MaximumCostMicros: 7, CostUpperBoundKnown: true,
	}}
	executor := NewProductImageSlotExecutor(Dependencies{
		Reviewer: reviewer, UsageQuoter: quoter, ProfileResolver: &recordingImageProfileResolver{profile: testImageProfile()},
	})
	input := testProductImageExecutionInput()
	quote, err := executor.QuoteStagedReview(context.Background(), input, imageagent.BudgetPolicy{})
	require.NoError(t, err)
	receipt, err := executor.ReviewStagedSlotQuoted(context.Background(), input, imageagent.SlotGeneratedOutput{
		SlotID: input.Slot.ID, Attempt: input.Attempt, SourceAssetID: "source-1",
		Assets: []imageagent.GeneratedAsset{{URL: "https://staging.example/review.png", SourceURL: "https://source.example/item.png", Width: 1, Height: 1, Operations: []string{"render_scene_model"}}},
	}, quote)
	require.NoError(t, err)
	require.Equal(t, quote.Maximum, receipt.Actual)
	require.NotNil(t, reviewer.request.Authorization)
	require.Equal(t, "review-route", reviewer.request.Authorization.RouteReference)
	require.Equal(t, "review-model", reviewer.request.Authorization.Model)
}

func TestTrialStagedReviewRequiresExactRecoveredIdentityBeforeProvider(t *testing.T) {
	base := "https://localhost:19444/image-agent-assets/issue487-images"
	policy, err := imageagent.NewIsolatedTrialGeneratedURLPolicy(base, "issue487-images")
	require.NoError(t, err)
	reviewer := &recordingProductReviewer{review: productimage.Review{Score: 1}}
	executor := NewProductImageSlotExecutor(Dependencies{
		Reviewer: reviewer, UsageQuoter: testProductUsageQuoter{}, ProfileResolver: &recordingImageProfileResolver{profile: testImageProfile()}, GeneratedURLTrial: policy,
	})
	input := testProductImageExecutionInput()
	owner, err := imageagent.ArtifactOwnerKey(input.UserID)
	require.NoError(t, err)
	hash := strings.Repeat("a", 64)
	ref := imageagent.StagedAssetRef{ObjectKey: "image-agent/staging/" + input.TenantID + "/" + owner + "/" + input.RunID + "/1/scene-1/1/0-" + hash + ".png", SHA256: hash, SizeBytes: 8, ContentType: "image/png", Width: 1, Height: 1, SourceAssetID: "source-1", Operations: []string{"render_scene_model"}}
	staged := imageagent.SlotGeneratedOutput{SlotID: input.Slot.ID, Attempt: input.Attempt, SourceAssetID: "source-1", Assets: []imageagent.GeneratedAsset{{URL: base + "/" + ref.ObjectKey, ContentType: ref.ContentType, SourceURL: "https://source.example/item.png", Width: 1, Height: 1, Operations: ref.Operations, StagedRef: &ref}}}
	quote, err := executor.QuoteStagedReview(context.Background(), input, imageagent.BudgetPolicy{})
	require.NoError(t, err)
	_, err = executor.ReviewStagedSlotQuoted(context.Background(), input, staged, quote)
	require.NoError(t, err)
	require.NotEmpty(t, reviewer.request.Candidates)

	staged.Assets[0].StagedRef = nil
	_, err = executor.ReviewStagedSlotQuoted(context.Background(), input, staged, quote)
	require.Error(t, err)
	staged.Assets[0].StagedRef = &ref
	staged.Assets[0].SourceURL = "https://localhost:19444/private-source.png"
	_, err = executor.ReviewStagedSlotQuoted(context.Background(), input, staged, quote)
	require.Error(t, err)
}

func TestStagedReviewOnlyReleasesBudgetForConfirmedNoDispatch(t *testing.T) {
	for _, tc := range []struct {
		name      string
		reviewErr error
		want      imageagent.ProviderDispatchState
	}{
		{"confirmed_preflight", productimage.ErrReviewConfirmedNotDispatched, imageagent.ProviderRejectedBeforeEffect},
		{"unknown_transport", productimage.ErrExternalCapabilityUnavailable, imageagent.ProviderDispatchedUnknown},
	} {
		t.Run(tc.name, func(t *testing.T) {
			executor := NewProductImageSlotExecutor(Dependencies{
				Reviewer: &failingProductReviewer{err: tc.reviewErr}, UsageQuoter: testProductUsageQuoter{},
				ProfileResolver: &recordingImageProfileResolver{profile: testImageProfile()},
			})
			input := testProductImageExecutionInput()
			quote, err := executor.QuoteStagedReview(context.Background(), input, imageagent.BudgetPolicy{})
			require.NoError(t, err)
			receipt, err := executor.ReviewStagedSlotQuoted(context.Background(), input, imageagent.SlotGeneratedOutput{
				SlotID: input.Slot.ID, Attempt: input.Attempt, SourceAssetID: "source-1",
				Assets: []imageagent.GeneratedAsset{{URL: "https://staging.example/review.png", SourceURL: "https://source.example/item.png", Width: 1, Height: 1, Operations: []string{"render_scene_model"}}},
			}, quote)
			require.Equal(t, tc.want, imageagent.ProviderDispatchStateOf(err))
			if tc.want == imageagent.ProviderRejectedBeforeEffect {
				require.Equal(t, imageagent.SlotUsageReceipt{}, receipt)
			} else {
				require.Equal(t, quote.Maximum, receipt.Actual)
			}
		})
	}
}

func TestGenerateQuotedSlotPreservesPriorImageEffectWhenReviewNotDispatched(t *testing.T) {
	executor := NewProductImageSlotExecutor(Dependencies{
		SubjectExtractor: testProductSubjectExtractor{}, WhiteBackgroundRenderer: testProductWhiteRenderer{},
		Reviewer: &failingProductReviewer{err: productimage.ErrReviewConfirmedNotDispatched}, UsageQuoter: testProductUsageQuoter{},
		ProfileResolver: &recordingImageProfileResolver{profile: testImageProfile()},
	})
	input := testProductImageExecutionInput()
	input.Slot.Role = imageagent.SlotRoleMain
	quote, err := executor.QuoteSlot(context.Background(), input, imageagent.BudgetPolicy{})
	require.NoError(t, err)
	_, err = executor.GenerateQuotedSlot(context.Background(), input, quote)
	require.Equal(t, imageagent.ProviderDispatchedUnknown, imageagent.ProviderDispatchStateOf(err), "image generation has already crossed its provider effect")
}

func TestExecutorClassifiesReviewerTransportFailureSeparately(t *testing.T) {
	reviewer := &failingProductReviewer{err: context.DeadlineExceeded}
	executor := NewProductImageSlotExecutor(Dependencies{
		SceneRenderer: &recordingProductSceneRenderer{candidates: []productimage.Candidate{testSceneCandidate(t, "https://source.example/item.png")}},
		Reviewer:      reviewer, UsageQuoter: testProductUsageQuoter{},
		ProfileResolver: &recordingImageProfileResolver{profile: testImageProfile()},
	})

	generated, err := executor.GenerateSlot(context.Background(), testProductImageExecutionInput())

	require.Error(t, err)
	require.Len(t, generated.Assets, 1)
	transportOutput, ok := imageagent.ReviewTransportOutput(err)
	require.True(t, ok, "reviewer transport failures must preserve staged output as a distinct error")
	require.Len(t, transportOutput.Assets, 1)
	_, qualityReview := imageagent.ReviewRequiredOutput(err)
	require.False(t, qualityReview, "transport failure must not be classified as a low-score decision")
}

func TestFrozenV2ExecutorAcceptsHistoricalInputWithoutV3PolicyFields(t *testing.T) {
	executor := NewFrozenV2ProductImageSlotExecutor(Dependencies{
		SceneRenderer: &recordingProductSceneRenderer{candidates: []productimage.Candidate{testSceneCandidate(t, "https://source.example/item.png")}},
		UsageQuoter:   testProductUsageQuoter{},
	})
	input := testProductImageExecutionInput()
	input.TargetPlatform = ""
	input.ImagePolicyContext = nil

	generated, err := executor.GenerateSlot(context.Background(), input)

	require.NoError(t, err)
	require.Len(t, generated.Assets, 1)
}

func TestFrozenV2ExecutorPublishesInlineProviderArtifactThroughCompatibilityMaterializer(t *testing.T) {
	materializer := &recordingLegacyAssetMaterializer{url: "https://cdn.example.test/v2/scene.png"}
	executor := NewFrozenV2ProductImageSlotExecutor(Dependencies{
		SceneRenderer: &recordingProductSceneRenderer{candidates: []productimage.Candidate{testSceneCandidate(t, "https://source.example/item.png")}},
		UsageQuoter:   testProductUsageQuoter{}, LegacyAssetMaterializer: materializer,
	})
	input := testProductImageExecutionInput()
	input.TargetPlatform = ""
	input.ImagePolicyContext = nil

	generated, err := executor.GenerateSlot(context.Background(), input)
	require.NoError(t, err)

	result, err := executor.PublishSlot(context.Background(), input, generated)
	require.NoError(t, err)
	require.Equal(t, materializer.url, result.Candidates[0].URL)
	require.Equal(t, 1, materializer.calls)
}

func TestExecutorAcceptsHistoricalV3InputWithoutPolicyFields(t *testing.T) {
	executor := NewProductImageSlotExecutor(Dependencies{
		SceneRenderer: &recordingProductSceneRenderer{candidates: []productimage.Candidate{testSceneCandidate(t, "https://source.example/item.png")}},
		UsageQuoter:   testProductUsageQuoter{},
	})
	input := testProductImageExecutionInput()
	input.TargetPlatform = ""
	input.ImagePolicyContext = nil

	generated, err := executor.GenerateSlot(context.Background(), input)

	require.NoError(t, err)
	require.Len(t, generated.Assets, 1)
}

func TestExecutorFailsClosedWhenExactPolicyIsMissingBeforeProviderDispatch(t *testing.T) {
	resolver := &recordingImageProfileResolver{err: imagepolicy.ErrPolicyNotFound}
	renderer := &recordingProductSceneRenderer{}
	executor := NewProductImageSlotExecutor(Dependencies{
		SubjectExtractor: testProductSubjectExtractor{}, WhiteBackgroundRenderer: testProductWhiteRenderer{},
		SceneRenderer: renderer, Reviewer: testProductReviewer{}, UsageQuoter: testProductUsageQuoter{}, ProfileResolver: resolver,
	})

	_, err := executor.GenerateSlot(context.Background(), testProductImageExecutionInput())

	require.ErrorIs(t, err, imagepolicy.ErrPolicyNotFound)
	require.Zero(t, renderer.calls)
}

func TestExecutorRejectsProviderSourcePassThroughEvenWithoutAppWrapper(t *testing.T) {
	renderer := &recordingProductSceneRenderer{candidates: []productimage.Candidate{{Asset: productimage.Asset{
		URL: "https://source.example/item.png", SourceURL: "https://source.example/item.png", SourceAssetID: "source-1",
		Role: productimage.RoleScene, Width: 1200, Height: 1200, Operations: []string{"render_scene"},
	}}}}
	executor := NewProductImageSlotExecutor(Dependencies{
		SubjectExtractor: testProductSubjectExtractor{}, WhiteBackgroundRenderer: testProductWhiteRenderer{},
		SceneRenderer: renderer, Reviewer: testProductReviewer{}, UsageQuoter: testProductUsageQuoter{}, ProfileResolver: &recordingImageProfileResolver{profile: testImageProfile()},
	})

	_, err := executor.GenerateSlot(context.Background(), testProductImageExecutionInput())

	require.ErrorIs(t, err, imageagent.ErrValidation)
}

func TestExecutorChainsSubjectIntoWhiteBackgroundAndRetainsOriginalProvenance(t *testing.T) {
	subject := &recordingProductSubjectExtractor{candidate: testSubjectCandidate(t, "https://source.example/item.png")}
	white := &recordingProductWhiteRenderer{candidate: testWhiteCandidate(t, "https://source.example/item.png")}
	executor := NewProductImageSlotExecutor(Dependencies{
		SubjectExtractor: subject, WhiteBackgroundRenderer: white, SceneRenderer: &recordingProductSceneRenderer{},
		Reviewer: testProductReviewer{}, UsageQuoter: testProductUsageQuoter{}, ProfileResolver: &recordingImageProfileResolver{profile: testImageProfile()},
	})
	input := testProductImageExecutionInput()
	input.Slot.Role = imageagent.SlotRoleMain

	generated, err := executor.GenerateSlot(context.Background(), input)

	require.NoError(t, err)
	require.Equal(t, subject.candidate, white.request.Subject)
	require.Equal(t, "source-1", generated.SourceAssetID)
	require.Equal(t, "https://source.example/item.png", generated.Assets[0].SourceURL)
}

func TestExecutorQuotesExactOperationWithExecutingProviderIdentity(t *testing.T) {
	quoter := &recordingProductUsageQuoter{quote: productimage.UsageQuote{
		Operation: "render_scene", Provider: "openai", RouteReference: "image-route", Model: "gpt-image-1",
		CredentialReference: "image-credential", ConfigurationVersion: "config-v1", PricingVersion: "pricing-v1",
		Fingerprint: "provider-quote-v1", MaximumOutputs: 1, MaximumModelCalls: 1,
		MaximumCostMicros: 10, CostUpperBoundKnown: true,
	}}
	renderer := &recordingProductSceneRenderer{candidates: []productimage.Candidate{testSceneCandidate(t, "https://source.example/item.png")}}
	executor := NewProductImageSlotExecutor(Dependencies{
		SubjectExtractor: testProductSubjectExtractor{}, WhiteBackgroundRenderer: testProductWhiteRenderer{},
		SceneRenderer: renderer, Reviewer: testProductReviewer{}, UsageQuoter: quoter,
		ProfileResolver: &recordingImageProfileResolver{profile: testImageProfile()},
	})

	quote, err := executor.QuoteSlot(context.Background(), testProductImageExecutionInput(), imageagent.BudgetPolicy{})

	require.NoError(t, err)
	require.Len(t, quoter.requests, 2)
	require.Equal(t, "render_scene", quoter.requests[0].Operation)
	require.Equal(t, "review", quoter.requests[1].Operation)
	require.Equal(t, int64(1), quoter.requests[0].MaximumOutputs)
	require.Len(t, quote.Operations, 2)
	require.Equal(t, "openai", quote.Operations[0].Provider)
	require.Equal(t, "gpt-image-1", quote.Operations[0].Model)
	_, err = executor.GenerateQuotedSlot(context.Background(), testProductImageExecutionInput(), quote)
	require.NoError(t, err)
	require.NotNil(t, renderer.request.Authorization)
	require.Equal(t, quoter.quote, *renderer.request.Authorization)
}

func testProductImageExecutionInput() imageagent.SlotExecutionInput {
	return imageagent.SlotExecutionInput{
		RunID: "run-1", TenantID: "tenant-1", UserID: "user-1", TargetPlatform: "shein",
		ImagePolicyContext: &imageagent.ImagePolicyContext{Country: "us", Family: "default", SceneCategory: "shoes"},
		PlanRevision:       1, Attempt: 1, IdempotencyKey: "attempt-1",
		Slot: imageagent.Slot{
			ID: "scene-1", Role: imageagent.SlotRoleScene, Brief: "show the product in use",
			SourceAssetIDs: []string{"source-1"}, IdempotencyKey: "slot-1",
		},
		AssetCatalog: imageagent.AssetCatalog{Assets: []imageagent.AuthorizedAsset{{
			ID: "source-1", Type: imageagent.AuthorizedAssetSource, URL: "https://source.example/item.png",
			SourceURL: "https://source.example/item.png", Width: 1200, Height: 1200,
		}}},
		ProductContext: imageagent.ProductContextRef{
			ProductID: "product-1", Title: "Product", ProductType: "shoe", Attributes: map[string]string{"color": "blue"},
		},
	}
}

func testImageProfile() imagepolicy.ProductImageProfile {
	return imagepolicy.ProductImageProfile{
		Key:           imagepolicy.PolicyKey{Marketplace: "shein", Country: "us", Family: "default", SceneCategory: "shoes"},
		PolicyVersion: "policy-v1",
		SceneDefaults: productimage.SceneOptions{
			SceneCategory: "shoes", SceneStyle: "lifestyle", BackgroundTone: "warm",
			Composition: "centered", PropsLevel: "light", AudienceHint: "general",
		},
	}
}

func testSceneCandidate(t *testing.T, sourceURL string) productimage.Candidate {
	t.Helper()
	return productimage.Candidate{Asset: productimage.Asset{
		Bytes: testTinyPNG(t), MediaType: "image/png", SourceURL: sourceURL, SourceAssetID: "source-1",
		Role: productimage.RoleScene, Width: 1, Height: 1, Operations: []string{"render_scene"},
	}}
}

func testSubjectCandidate(t *testing.T, sourceURL string) productimage.Candidate {
	t.Helper()
	return productimage.Candidate{Asset: productimage.Asset{
		Bytes: testTinyPNG(t), MediaType: "image/png", SourceURL: sourceURL, SourceAssetID: "source-1",
		Role: productimage.RoleSubject, Width: 1, Height: 1, Operations: []string{"extract_subject"},
	}}
}

func testWhiteCandidate(t *testing.T, sourceURL string) productimage.Candidate {
	t.Helper()
	return productimage.Candidate{Asset: productimage.Asset{
		Bytes: testTinyPNG(t), MediaType: "image/png", SourceURL: sourceURL, SourceAssetID: "source-1",
		Role: productimage.RoleWhiteBackground, Width: 1, Height: 1, Operations: []string{"render_white_background"},
	}}
}

func testTinyPNG(t *testing.T) []byte {
	t.Helper()
	canvas := image.NewRGBA(image.Rect(0, 0, 1, 1))
	canvas.Set(0, 0, color.RGBA{R: 10, G: 20, B: 30, A: 255})
	var encoded bytes.Buffer
	require.NoError(t, png.Encode(&encoded, canvas))
	return encoded.Bytes()
}

type recordingImageProfileResolver struct {
	input   imagepolicy.ProfileInput
	profile imagepolicy.ProductImageProfile
	err     error
}

type recordingLegacyAssetMaterializer struct {
	url   string
	calls int
}

func (m *recordingLegacyAssetMaterializer) Materialize(context.Context, imageagent.SlotExecutionInput, int, imageagent.GeneratedAsset) (string, error) {
	m.calls++
	return m.url, nil
}

func (r *recordingImageProfileResolver) Resolve(input imagepolicy.ProfileInput) (imagepolicy.ProductImageProfile, error) {
	r.input = input
	return r.profile, r.err
}

type recordingProductSubjectExtractor struct {
	candidate productimage.Candidate
	request   productimage.ExtractRequest
}

func (r *recordingProductSubjectExtractor) Extract(_ context.Context, request productimage.ExtractRequest) (productimage.Candidate, error) {
	r.request = request
	return r.candidate, nil
}

type testProductSubjectExtractor struct{}

func (testProductSubjectExtractor) Extract(context.Context, productimage.ExtractRequest) (productimage.Candidate, error) {
	return productimage.Candidate{}, errors.New("not called")
}

type recordingProductWhiteRenderer struct {
	calls     int
	candidate productimage.Candidate
	request   productimage.RenderRequest
}

func (r *recordingProductWhiteRenderer) RenderWhiteBackground(_ context.Context, request productimage.RenderRequest) (productimage.Candidate, error) {
	r.calls++
	r.request = request
	return r.candidate, nil
}

type testProductWhiteRenderer struct{}

func (testProductWhiteRenderer) RenderWhiteBackground(context.Context, productimage.RenderRequest) (productimage.Candidate, error) {
	return productimage.Candidate{}, errors.New("not called")
}

type recordingProductSceneRenderer struct {
	candidates []productimage.Candidate
	request    productimage.SceneRequest
	calls      int
}

type recordingProductReviewer struct {
	review  productimage.Review
	request productimage.ReviewRequest
}

type failingProductReviewer struct{ err error }

func (r *failingProductReviewer) Review(context.Context, productimage.ReviewRequest) (productimage.Review, error) {
	return productimage.Review{}, r.err
}

func (r *recordingProductReviewer) Review(_ context.Context, request productimage.ReviewRequest) (productimage.Review, error) {
	r.request = request
	return r.review, nil
}

type testProductReviewer struct{}

func (testProductReviewer) Review(context.Context, productimage.ReviewRequest) (productimage.Review, error) {
	return productimage.Review{Score: 1}, nil
}

func (r *recordingProductSceneRenderer) RenderScene(_ context.Context, request productimage.SceneRequest) ([]productimage.Candidate, error) {
	r.calls++
	r.request = request
	return append([]productimage.Candidate(nil), r.candidates...), nil
}

type testProductUsageQuoter struct{}

func (testProductUsageQuoter) QuoteUsage(_ context.Context, request productimage.UsageQuoteRequest) (productimage.UsageQuote, error) {
	return productimage.UsageQuote{
		Operation: request.Operation, Provider: "test", RouteReference: "test-route", Model: "test-model",
		CredentialReference: "test-credential", ConfigurationVersion: "test-config", PricingVersion: "test-pricing",
		Fingerprint: request.Operation + "-quote", MaximumOutputs: request.MaximumOutputs,
		MaximumModelCalls: 1, CostUpperBoundKnown: true,
	}, nil
}

type recordingProductUsageQuoter struct {
	request  productimage.UsageQuoteRequest
	requests []productimage.UsageQuoteRequest
	quote    productimage.UsageQuote
}

func (q *recordingProductUsageQuoter) QuoteUsage(_ context.Context, request productimage.UsageQuoteRequest) (productimage.UsageQuote, error) {
	q.request = request
	q.requests = append(q.requests, request)
	quote := q.quote
	quote.Operation = request.Operation
	return quote, nil
}
