package listingkit

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	sdstemplate "task-processor/internal/sds/template"
)

func TestBuildStudioBatchTaskGenerateRequestIncludesOwnerContext(t *testing.T) {
	t.Parallel()

	req := buildStudioBatchTaskGenerateRequest(
		nil,
		&StudioBatchRecord{
			TenantID:     "tenant-1",
			UserID:       "user-1",
			Prompt:       "prompt",
			SheinStoreID: 870,
		},
		studioBatchTaskCandidate{
			Item:        StudioBatchItemRecord{ID: "item-1"},
			SelectionID: "selection-1",
			Selection: SheinStudioGroupedSelection{
				SheinStoreID: "870",
			},
			SelectionSnapshot: SheinStudioSelection{
				ProductName:     "wallet",
				VariantID:       1,
				ParentProductID: 2,
			},
			Title: "group-a",
		},
		StudioMaterializedDesignRecord{
			ID:               "design-1",
			ImageURL:         "https://example.com/design.png",
			TargetGroupLabel: "group-a",
		},
	)

	if req == nil {
		t.Fatal("request is nil")
	}
	if req.TenantID != "tenant-1" {
		t.Fatalf("TenantID = %q, want tenant-1", req.TenantID)
	}
	if req.UserID != "user-1" {
		t.Fatalf("UserID = %q, want user-1", req.UserID)
	}
}

func TestBuildStudioBatchTaskGenerateRequestIncludesImageStrategy(t *testing.T) {
	t.Parallel()

	req := buildStudioBatchTaskGenerateRequest(
		nil,
		&StudioBatchRecord{TenantID: "tenant-1", UserID: "user-1", SheinStoreID: 870},
		studioBatchTaskCandidate{
			Item:          StudioBatchItemRecord{ID: "item-1"},
			SelectionID:   "selection-1",
			ImageStrategy: " AI_GENERATED ",
			Selection:     SheinStudioGroupedSelection{SheinStoreID: "870"},
		},
		StudioMaterializedDesignRecord{ID: "design-1", ImageURL: "https://example.com/design.png"},
	)

	if req == nil || req.Options == nil {
		t.Fatal("request options are nil")
	}
	if got, want := req.Options.ImageStrategy, sheinImageStrategyAIGenerated; got != want {
		t.Fatalf("ImageStrategy = %q, want normalized %q", got, want)
	}
}

func TestFallbackStudioBatchTaskSessionRestoresPersistedImageStrategy(t *testing.T) {
	t.Parallel()

	session := fallbackStudioBatchTaskSession("batch-1", &StudioBatchRecord{
		ImageStrategy: sheinImageStrategyAIGenerated,
	}, []string{"design-1"}, "")
	if session.ImageStrategy != sheinImageStrategyAIGenerated {
		t.Fatalf("fallback image strategy = %q, want %q", session.ImageStrategy, sheinImageStrategyAIGenerated)
	}
}

func TestBuildStudioBatchTaskProductImageRequestCarriesBatchContext(t *testing.T) {
	t.Parallel()

	session := &SheinStudioSession{
		Prompt:             "minimal geometric abstract design",
		PromptMode:         "managed",
		ProductImagePrompt: "show the approved artwork on a clean studio mockup",
		ProductImageCount:  "5",
		ProductImagePrompts: SheinStudioProductImagePromptList{
			{Role: "hero", Label: "front", Prompt: "front-facing product photo"},
		},
	}
	batch := &StudioBatchRecord{ID: "batch-1", TenantID: "tenant-1", UserID: "user-1"}
	candidate := studioBatchTaskCandidate{
		SelectionSnapshot: SheinStudioSelection{
			ProductName:            "V-neck T-shirt",
			SizeReferenceImageURLs: []string{"https://example.com/size.png"},
		},
		Title: "geometric tee",
	}
	design := StudioMaterializedDesignRecord{ID: "design-1", ImageURL: "https://example.com/design.png"}

	req := buildStudioBatchTaskProductImageRequest(session, batch, candidate, design)
	if req == nil {
		t.Fatal("product image request is nil")
	}
	if req.Prompt != "minimal geometric abstract design" || req.PromptMode != "managed" {
		t.Fatalf("prompt context = (%q, %q), want batch prompt context", req.Prompt, req.PromptMode)
	}
	if req.ProductName != "V-neck T-shirt" || req.StyleName != "geometric tee" {
		t.Fatalf("product/style = (%q, %q), want selection/style", req.ProductName, req.StyleName)
	}
	if req.SourceDesignURL != "https://example.com/design.png" {
		t.Fatalf("SourceDesignURL = %q, want materialized design URL", req.SourceDesignURL)
	}
	if req.CustomPrompt != "show the approved artwork on a clean studio mockup" || req.Count != 5 {
		t.Fatalf("custom prompt/count = (%q, %d), want persisted product-image settings", req.CustomPrompt, req.Count)
	}
	if len(req.ImagePrompts) != 1 || req.ImagePrompts[0].Role != "hero" {
		t.Fatalf("ImagePrompts = %+v, want session prompts", req.ImagePrompts)
	}
	if len(req.ProductReferenceImageURLs) != 1 || req.ProductReferenceImageURLs[0] != "https://example.com/size.png" {
		t.Fatalf("ProductReferenceImageURLs = %+v, want selection references", req.ProductReferenceImageURLs)
	}
}

func TestTaskStudioBatchServiceAttachesGeneratedProductImagesForAI(t *testing.T) {
	t.Parallel()

	var gotRequest *StudioProductImageRequest
	service := &taskStudioBatchService{
		productImageUsage: &recordingStudioProductImageUsage{},
		generateProductImages: func(_ context.Context, req *StudioProductImageRequest) (*StudioProductImageResponse, error) {
			gotRequest = req
			return &StudioProductImageResponse{Images: []StudioGeneratedImage{
				{ImageURL: "https://cdn.example.com/ai-product.png"},
			}}, nil
		},
	}
	session := &SheinStudioSession{Prompt: "retro cherries", ImageStrategy: sheinImageStrategyAIGenerated}
	batch := &StudioBatchRecord{ID: "batch-1", TenantID: "tenant-1", UserID: "user-1"}
	candidate := studioBatchTaskCandidate{
		ImageStrategy:     sheinImageStrategyAIGenerated,
		SelectionSnapshot: SheinStudioSelection{ProductName: "Canvas Tote"},
		Title:             "Style 1",
	}
	request := buildStudioBatchTaskGenerateRequest(session, batch, candidate, StudioMaterializedDesignRecord{
		ID:       "design-1",
		ImageURL: "https://cdn.example.com/design.png",
	})

	if err := service.attachStudioBatchProductImages(context.Background(), request, session, batch, candidate, StudioMaterializedDesignRecord{
		ID:       "design-1",
		ImageURL: "https://cdn.example.com/design.png",
	}); err != nil {
		t.Fatalf("attachStudioBatchProductImages() error = %v", err)
	}
	if gotRequest == nil || gotRequest.SourceDesignURL != "https://cdn.example.com/design.png" {
		t.Fatalf("generator request = %+v, want source design URL", gotRequest)
	}
	if got, want := request.Options.SheinStudio.ProductImageURLs, []string{"https://cdn.example.com/ai-product.png"}; len(got) != 1 || got[0] != want[0] {
		t.Fatalf("ProductImageURLs = %+v, want %v", got, want)
	}
}

func TestBuildStudioBatchTaskProductImageRequestFallsBackToBatchPromptMode(t *testing.T) {
	t.Parallel()

	req := buildStudioBatchTaskProductImageRequest(
		&SheinStudioSession{Prompt: "raw prompt"},
		&StudioBatchRecord{Prompt: "raw prompt", PromptMode: "raw"},
		studioBatchTaskCandidate{SelectionSnapshot: SheinStudioSelection{ProductName: "T-shirt"}, Title: "Style 1"},
		StudioMaterializedDesignRecord{ID: "design-1", ImageURL: "https://example.com/design.png"},
	)
	if req.PromptMode != "raw" {
		t.Fatalf("PromptMode = %q, want batch fallback mode raw", req.PromptMode)
	}
}

func TestStudioBatchTaskColorRepresentativesGroupsColorlessVariants(t *testing.T) {
	selection := SheinStudioSelection{Variants: []SheinStudioSelectionVariant{
		{VariantSKU: "size-s"},
		{VariantSKU: "size-m"},
	}}
	representatives := studioBatchTaskColorRepresentatives(selection)
	if len(representatives) != 1 || representatives[0].VariantSKU != "size-s" {
		t.Fatalf("colorless representatives = %+v, want one default representative", representatives)
	}
}

func TestStudioBatchTaskColorRepresentativesMergeReferencesAcrossSizes(t *testing.T) {
	selection := SheinStudioSelection{Variants: []SheinStudioSelectionVariant{
		{VariantSKU: "red-s", Color: "Red"},
		{VariantSKU: "red-m", Color: " red ", SizeReferenceImageURLs: []string{"https://example.com/red-size.png"}, MockupImageURL: "https://example.com/red-mockup.png"},
		{VariantSKU: "blue-s", Color: "Blue", MockupImageURL: "https://example.com/blue.png"},
	}}
	representatives := studioBatchTaskColorRepresentatives(selection)
	if len(representatives) != 2 {
		t.Fatalf("color representatives = %+v, want two colors", representatives)
	}
	if representatives[0].VariantSKU != "red-s" || len(representatives[0].SizeReferenceImageURLs) != 1 || representatives[0].SizeReferenceImageURLs[0] != "https://example.com/red-size.png" {
		t.Fatalf("merged red references = %+v, want references from all red sizes", representatives[0])
	}
	if representatives[0].MockupImageURL != "https://example.com/red-mockup.png" {
		t.Fatalf("merged red mockup = %q, want later size mockup", representatives[0].MockupImageURL)
	}
}

func TestBuildStudioBatchTaskProductImageRequestUsesHotReferencePrompt(t *testing.T) {
	req := buildStudioBatchTaskProductImageRequest(
		&SheinStudioSession{
			HotStyleReferenceImageURLs: []string{"https://example.com/hot-reference.png"},
			HotStyleReferenceBrief:     "embroidered cherry badge",
		},
		&StudioBatchRecord{Prompt: "", HotStyleReferenceImageURLs: SheinStudioStringList{"https://example.com/hot-reference.png"}},
		studioBatchTaskCandidate{SelectionSnapshot: SheinStudioSelection{ProductName: "T-shirt"}, Title: "Style 1"},
		StudioMaterializedDesignRecord{ID: "design-1", ImageURL: "https://example.com/design.png"},
	)
	if req == nil || req.Prompt != "embroidered cherry badge" {
		t.Fatalf("hot-reference product image prompt = %q, want persisted hot-reference brief", req.Prompt)
	}
}

func TestAppendStudioProductImageColorDirectiveAugmentsRawRolePrompt(t *testing.T) {
	request := &StudioProductImageRequest{
		PromptMode:   "raw",
		CustomPrompt: "global raw prompt",
		ImagePrompts: []StudioProductImagePrompt{{Role: "main", Prompt: "main role prompt"}},
	}
	appendStudioProductImageColorDirective(request, "Red")
	prompt := buildRawStudioProductImagePrompt(request, defaultStudioProductImageRoles[0])
	if !strings.Contains(prompt, "main role prompt") || !strings.Contains(prompt, "Red") {
		t.Fatalf("raw role prompt = %q, want role text plus color directive", prompt)
	}
}

func TestAppendStudioProductImageColorDirectiveAugmentsUnconfiguredRawRoleFallback(t *testing.T) {
	t.Parallel()

	request := &StudioProductImageRequest{
		PromptMode:   "raw",
		Prompt:       "fallback raw prompt",
		CustomPrompt: "global raw prompt",
		ImagePrompts: []StudioProductImagePrompt{{Role: "main", Prompt: "main role prompt"}},
	}
	appendStudioProductImageColorDirective(request, "Red")
	fallbackPrompt := buildRawStudioProductImagePrompt(request, studioProductImageRole{Key: "detail"})
	if !strings.Contains(fallbackPrompt, "fallback raw prompt") && !strings.Contains(fallbackPrompt, "global raw prompt") {
		t.Fatalf("fallback raw role prompt = %q, want configured fallback prompt", fallbackPrompt)
	}
	if !strings.Contains(fallbackPrompt, "Red") {
		t.Fatalf("fallback raw role prompt = %q, want Red directive", fallbackPrompt)
	}
}

func TestBuildStudioBatchTaskCandidateKeyIncludesAIGeneratedSelectionInputs(t *testing.T) {
	t.Parallel()

	candidate := studioBatchTaskCandidate{
		Item:          StudioBatchItemRecord{ID: "item-1"},
		Design:        StudioMaterializedDesignRecord{ID: "design-1"},
		SelectionID:   "selection-1",
		ImageStrategy: sheinImageStrategyAIGenerated,
		SelectionSnapshot: SheinStudioSelection{
			ProductName: "Canvas Tote",
			Variants: []SheinStudioSelectionVariant{{
				VariantSKU:             "red-s",
				Color:                  "Red",
				MockupImageURL:         "https://example.com/red.png",
				SizeReferenceImageURLs: []string{"https://example.com/red-size.png"},
			}},
		},
	}
	first := buildStudioBatchTaskCandidateKey(WithTenantID(context.Background(), "tenant-1"), &StudioBatchRecord{ID: "batch-1"}, candidate)
	candidate.SelectionSnapshot.ProductName = "Updated Canvas Tote"
	second := buildStudioBatchTaskCandidateKey(WithTenantID(context.Background(), "tenant-1"), &StudioBatchRecord{ID: "batch-1"}, candidate)
	if first == second {
		t.Fatalf("candidate keys unexpectedly match after product-image input changed: %q", first)
	}
}

func TestStudioBatchTaskCandidateKeyDiffersWhenProductImageCategoryChanges(t *testing.T) {
	t.Parallel()

	candidate := studioBatchTaskCandidate{
		ImageStrategy:            sheinImageStrategyAIGenerated,
		ProductImageCategoryPath: []string{"Apparel", "Tops"},
		SelectionSnapshot:        SheinStudioSelection{ProductName: "Canvas Tote"},
	}
	first := buildStudioBatchTaskCandidateKey(WithTenantID(context.Background(), "tenant-1"), &StudioBatchRecord{ID: "batch-1"}, candidate)
	candidate.ProductImageCategoryPath = []string{"Home", "Decor"}
	second := buildStudioBatchTaskCandidateKey(WithTenantID(context.Background(), "tenant-1"), &StudioBatchRecord{ID: "batch-1"}, candidate)
	if first == second {
		t.Fatalf("candidate keys unexpectedly match after product-image category changed: %q", first)
	}
}

func TestStudioProductImageCategoryPathUsesSDSCategoryNames(t *testing.T) {
	t.Parallel()

	path := studioProductImageCategoryPath(&sdstemplate.ProductDetail{
		ProductSummary: sdstemplate.ProductSummary{
			Categories: []sdstemplate.Category{{Name: "Apparel"}, {Name: "Tops"}},
		},
	})
	if got, want := path, []string{"Apparel", "Tops"}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("category path = %+v, want %+v", got, want)
	}
}

func TestBuildStudioBatchTaskCandidatesSnapshotsProductImageCategoryPath(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	batch := newStudioBatchRecordForTest("batch-category", now)
	batch.ImageStrategy = sheinImageStrategyAIGenerated
	batch.GroupedImageMode = "per_product"
	batch.GroupedSelections = SheinStudioGroupedSelectionList{
		studioBatchFanOutSelection("selection-1", 3001, "Red", "870", "https://cdn.example.com/template.png", "https://cdn.example.com/mask.png"),
	}
	item := StudioBatchItemRecord{ID: "item-1", BatchID: batch.ID, SelectionIDs: SheinStudioStringList{"selection-1"}, GroupMode: "per_product"}
	design := StudioMaterializedDesignRecord{ID: "design-1", BatchID: batch.ID, ItemID: item.ID, ImageURL: "https://cdn.example.com/design.png", ReviewStatus: StudioMaterializedDesignReviewStatusApproved}
	service := &taskStudioBatchService{
		sdsProductDetailProvider: stubSDSBaselineRemoteProvider{productDetail: &sdstemplate.ProductDetail{ProductSummary: sdstemplate.ProductSummary{
			Categories: []sdstemplate.Category{{Name: "Apparel"}, {Name: "Tops"}},
		}}},
	}
	candidates, _, err := service.buildStudioBatchTaskCandidates(context.Background(), &SheinStudioSession{ImageStrategy: sheinImageStrategyAIGenerated}, batch, &StudioBatchDetailGraph{
		Batch: batch,
		Items: []StudioBatchItemRecord{item},
	}, []StudioMaterializedDesignRecord{design})
	if err != nil {
		t.Fatalf("buildStudioBatchTaskCandidates() error = %v", err)
	}
	if len(candidates) != 1 {
		t.Fatalf("candidates = %+v, want one candidate", candidates)
	}
	if got, want := candidates[0].ProductImageCategoryPath, []string{"Apparel", "Tops"}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("candidate category path = %+v, want %+v", got, want)
	}
}

func TestTaskStudioBatchServiceAttachesPerColorProductImages(t *testing.T) {
	var calls []string
	var firstReferences []string
	var requests []*StudioProductImageRequest
	usage := &recordingStudioProductImageUsage{}
	service := &taskStudioBatchService{
		productImageUsage: usage,
		generateProductImages: func(_ context.Context, req *StudioProductImageRequest) (*StudioProductImageResponse, error) {
			calls = append(calls, req.StyleName)
			requests = append(requests, cloneStudioBatchProductImageRequest(req))
			if len(calls) == 1 {
				firstReferences = append([]string(nil), req.ProductReferenceImageURLs...)
			}
			return &StudioProductImageResponse{Images: []StudioGeneratedImage{{ImageURL: "https://cdn.example.com/" + req.StyleName + ".png"}}}, nil
		},
	}
	selection := SheinStudioSelection{
		ProductName: "Canvas Tote",
		Variants: []SheinStudioSelectionVariant{
			{VariantSKU: "RED", Color: "Red", MockupImageURL: "https://example.com/red.png"},
			{VariantSKU: "BLUE", Color: "Blue", MockupImageURL: "https://example.com/blue.png"},
		},
	}
	candidate := studioBatchTaskCandidate{
		ImageStrategy:     sheinImageStrategyAIGenerated,
		SelectionSnapshot: selection,
		Title:             "Style 1",
	}
	request := buildStudioBatchTaskGenerateRequest(
		&SheinStudioSession{Prompt: "retro", PromptMode: "raw", ProductImagePrompts: SheinStudioProductImagePromptList{{Role: "main", Prompt: "approved artwork"}}},
		&StudioBatchRecord{ID: "batch-1", TenantID: "tenant-a"},
		candidate,
		StudioMaterializedDesignRecord{ID: "design-1", ImageURL: "https://example.com/design.png"},
	)
	if err := service.attachStudioBatchProductImages(context.Background(), request, &SheinStudioSession{Prompt: "retro", PromptMode: "raw", ProductImagePrompts: SheinStudioProductImagePromptList{{Role: "main", Prompt: "approved artwork"}}}, &StudioBatchRecord{ID: "batch-1", TenantID: "tenant-a"}, candidate, StudioMaterializedDesignRecord{ID: "design-1", ImageURL: "https://example.com/design.png"}); err != nil {
		t.Fatalf("attachStudioBatchProductImages() error = %v", err)
	}
	if len(calls) != 2 {
		t.Fatalf("generator calls = %d (%v), want one per color representative", len(calls), calls)
	}
	if len(firstReferences) == 0 || firstReferences[0] != "https://example.com/red.png" {
		t.Fatalf("first color references = %v, want first representative references", firstReferences)
	}
	if len(requests) != 2 {
		t.Fatalf("captured product image requests = %d, want 2", len(requests))
	}
	if len(usage.authorized) != 1 || usage.authorized[0] != "tenant-a:2" || len(usage.recorded) != 0 {
		t.Fatalf("product image usage = authorized:%v recorded:%v, want authorization during attachment and no settlement before task commit", usage.authorized, usage.recorded)
	}
	firstPrompt := buildRawStudioProductImagePrompt(requests[0], defaultStudioProductImageRoles[0])
	secondPrompt := buildRawStudioProductImagePrompt(requests[1], defaultStudioProductImageRoles[0])
	if !strings.Contains(firstPrompt, "Red") || strings.Contains(firstPrompt, "Blue") {
		t.Fatalf("first raw prompt = %q, want Red directive only", firstPrompt)
	}
	if !strings.Contains(secondPrompt, "Blue") || strings.Contains(secondPrompt, "Red") {
		t.Fatalf("second raw prompt = %q, want Blue directive only", secondPrompt)
	}
	if got := len(request.Options.SheinStudio.VariantProductImages); got != 2 {
		t.Fatalf("variant product image sets = %d, want 2", got)
	}
	if got := request.Options.SheinStudio.VariantProductImages[1].Color; got != "Blue" {
		t.Fatalf("second variant color = %q, want Blue", got)
	}
}

func TestTaskStudioBatchServiceReservesProductImageUsageBeforeGeneration(t *testing.T) {
	t.Parallel()

	usage := &reservingStudioProductImageUsage{}
	service := &taskStudioBatchService{
		productImageUsage: usage,
		generateProductImages: func(_ context.Context, _ *StudioProductImageRequest) (*StudioProductImageResponse, error) {
			return &StudioProductImageResponse{Images: []StudioGeneratedImage{{ImageURL: "https://cdn.example.com/generated.png"}}}, nil
		},
	}
	candidate := studioBatchTaskCandidate{
		CandidateKey:      "candidate-reservation",
		ImageStrategy:     sheinImageStrategyAIGenerated,
		SelectionSnapshot: SheinStudioSelection{ProductName: "Canvas Tote"},
	}
	request := buildStudioBatchTaskGenerateRequest(
		&SheinStudioSession{Prompt: "retro", ImageStrategy: sheinImageStrategyAIGenerated},
		&StudioBatchRecord{ID: "batch-reservation", TenantID: "tenant-a"},
		candidate,
		StudioMaterializedDesignRecord{ID: "design-1", ImageURL: "https://example.com/design.png"},
	)
	if err := service.attachStudioBatchProductImages(context.Background(), request, nil, &StudioBatchRecord{ID: "batch-reservation", TenantID: "tenant-a"}, candidate, StudioMaterializedDesignRecord{ID: "design-1", ImageURL: "https://example.com/design.png"}); err != nil {
		t.Fatalf("attachStudioBatchProductImages() error = %v", err)
	}
	if len(usage.reserved) != 1 || usage.reserved[0] != "tenant-a:candidate-reservation:1" {
		t.Fatalf("reservations = %v, want one atomic reservation before generation", usage.reserved)
	}
}

func TestTaskStudioBatchServiceHonorsGenerationUsageRolloutGate(t *testing.T) {
	usage := &reservingStudioProductImageUsage{}
	service := &taskStudioBatchService{
		productImageUsage:        usage,
		generationUsageAdmission: generationUsageTestAdmission{tenantIDs: map[string]struct{}{}},
	}
	err := service.authorizeStudioBatchProductImageUsage(
		context.Background(),
		&StudioBatchRecord{ID: "batch-rollout-denied", TenantID: "tenant-rollout-denied"},
		studioBatchTaskCandidate{CandidateKey: "candidate-rollout-denied", ImageStrategy: sheinImageStrategyAIGenerated},
		1,
	)
	if err != nil {
		t.Fatalf("authorize error = %v, want legacy authorization outside rollout", err)
	}
	if len(usage.authorized) != 1 || usage.authorized[0] != "tenant-rollout-denied:1" || len(usage.reserved) != 0 {
		t.Fatalf("usage calls = authorized:%v reserved:%v, want legacy authorization only", usage.authorized, usage.reserved)
	}
}

func TestTaskStudioBatchServiceSettlesProductImageUsageForCommittedCandidate(t *testing.T) {
	usage := &recordingStudioProductImageUsage{}
	service := &taskStudioBatchService{productImageUsage: usage}
	candidate := studioBatchTaskCandidate{
		ImageStrategy: sheinImageStrategyAIGenerated,
		SelectionSnapshot: SheinStudioSelection{Variants: []SheinStudioSelectionVariant{
			{VariantSKU: "red-s", Color: "Red"},
			{VariantSKU: "blue-s", Color: "Blue"},
		}},
	}
	service.settleStudioBatchProductImageUsage(context.Background(), &StudioBatchRecord{TenantID: "tenant-a"}, candidate)
	if len(usage.recorded) != 1 || usage.recorded[0] != "tenant-a:2" {
		t.Fatalf("settled usage = %v, want one post-commit settlement for both colors", usage.recorded)
	}
}

func TestTaskStudioBatchServiceLegacySettlementIsIdempotentForDurableReuse(t *testing.T) {
	t.Parallel()

	ctx := WithTenantID(context.Background(), "tenant-a")
	links := NewMemStudioBatchTaskLinkRepository()
	if err := links.CreateStudioBatchTaskLink(ctx, &StudioBatchTaskLinkRecord{
		ID: "link-legacy-settle", BatchID: "batch-1", CandidateKey: "candidate-legacy-settle",
		ImageStrategy: sheinImageStrategyAIGenerated, Status: studioBatchTaskLinkStatusCreated,
	}); err != nil {
		t.Fatalf("CreateStudioBatchTaskLink() error = %v", err)
	}
	usage := &recordingStudioProductImageUsage{}
	service := &taskStudioBatchService{productImageUsage: usage, batchTaskLinkRepo: links, currentTime: time.Now}
	candidate := studioBatchTaskCandidate{
		CandidateKey:  "candidate-legacy-settle",
		ImageStrategy: sheinImageStrategyAIGenerated,
		SelectionSnapshot: SheinStudioSelection{Variants: []SheinStudioSelectionVariant{
			{VariantSKU: "red-s", Color: "Red"},
			{VariantSKU: "blue-s", Color: "Blue"},
		}},
	}
	for i := 0; i < 2; i++ {
		if err := service.settleStudioBatchProductImageUsage(ctx, &StudioBatchRecord{TenantID: "tenant-a"}, candidate); err != nil {
			t.Fatalf("settleStudioBatchProductImageUsage(%d) error = %v", i, err)
		}
	}
	if got, want := usage.recorded, []string{"tenant-a:2"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("legacy settlements = %v, want %v", got, want)
	}
	link, err := links.GetStudioBatchTaskLinkByCandidateKey(ctx, candidate.CandidateKey)
	if err != nil {
		t.Fatalf("GetStudioBatchTaskLinkByCandidateKey() error = %v", err)
	}
	if !link.ProductImageUsageSettled {
		t.Fatal("durable link usage marker = false, want true")
	}
}

func TestTaskStudioBatchServiceLegacySettlementRetriesIdempotentOperationBeforeMarker(t *testing.T) {
	t.Parallel()

	ctx := WithTenantID(context.Background(), "tenant-a")
	links := NewMemStudioBatchTaskLinkRepository()
	if err := links.CreateStudioBatchTaskLink(ctx, &StudioBatchTaskLinkRecord{
		ID: "link-idempotent-settle", BatchID: "batch-1", CandidateKey: "candidate-idempotent-settle",
		ImageStrategy: sheinImageStrategyAIGenerated, Status: studioBatchTaskLinkStatusCreated,
	}); err != nil {
		t.Fatalf("CreateStudioBatchTaskLink() error = %v", err)
	}
	usage := &idempotentRecordingStudioProductImageUsage{}
	service := &taskStudioBatchService{productImageUsage: usage, batchTaskLinkRepo: links, currentTime: time.Now}
	candidate := studioBatchTaskCandidate{
		CandidateKey:  "candidate-idempotent-settle",
		ImageStrategy: sheinImageStrategyAIGenerated,
		SelectionSnapshot: SheinStudioSelection{Variants: []SheinStudioSelectionVariant{
			{VariantSKU: "red-s", Color: "Red"},
		}},
	}
	if err := service.settleStudioBatchProductImageUsage(ctx, &StudioBatchRecord{TenantID: "tenant-a"}, candidate); err != nil {
		t.Fatalf("first settlement error = %v", err)
	}
	link, err := links.GetStudioBatchTaskLinkByCandidateKey(ctx, candidate.CandidateKey)
	if err != nil {
		t.Fatalf("GetStudioBatchTaskLinkByCandidateKey() error = %v", err)
	}
	link.ProductImageUsageSettled = false
	if err := links.UpdateStudioBatchTaskLink(ctx, link); err != nil {
		t.Fatalf("reset settlement marker error = %v", err)
	}
	if err := service.settleStudioBatchProductImageUsage(ctx, &StudioBatchRecord{TenantID: "tenant-a"}, candidate); err != nil {
		t.Fatalf("retry settlement error = %v", err)
	}
	if got := len(usage.operations); got != 1 {
		t.Fatalf("idempotent legacy operations = %d, want one", got)
	}
}

func TestTaskStudioBatchServiceCommitsDurableProductImageReservation(t *testing.T) {
	t.Parallel()

	usage := &reservingStudioProductImageUsage{}
	service := &taskStudioBatchService{productImageUsage: usage}
	candidate := studioBatchTaskCandidate{
		CandidateKey:      "candidate-commit",
		ImageStrategy:     sheinImageStrategyAIGenerated,
		SelectionSnapshot: SheinStudioSelection{ProductName: "Canvas Tote"},
	}
	if err := service.settleStudioBatchProductImageUsage(context.Background(), &StudioBatchRecord{TenantID: "tenant-a"}, candidate); err != nil {
		t.Fatalf("settleStudioBatchProductImageUsage() error = %v", err)
	}
	if len(usage.committed) != 1 || usage.committed[0] != "tenant-a:candidate-commit" {
		t.Fatalf("committed reservations = %v, want durable reservation commit", usage.committed)
	}
}

func TestTaskStudioBatchServiceSettlesLegacyUsageWhenLedgerAdmissionIsDenied(t *testing.T) {
	t.Parallel()

	usage := &reservingStudioProductImageUsage{}
	service := &taskStudioBatchService{
		productImageUsage:        usage,
		generationUsageAdmission: denyingStudioBatchGenerationUsageAdmission{},
	}
	candidate := studioBatchTaskCandidate{
		CandidateKey:      "candidate-rollout-denied-settle",
		ImageStrategy:     sheinImageStrategyAIGenerated,
		SelectionSnapshot: SheinStudioSelection{ProductName: "Canvas Tote"},
	}
	if err := service.settleStudioBatchProductImageUsage(context.Background(), &StudioBatchRecord{TenantID: "tenant-rollout-denied"}, candidate); err != nil {
		t.Fatalf("settleStudioBatchProductImageUsage() error = %v", err)
	}
	if len(usage.committed) != 0 {
		t.Fatalf("committed reservations = %v, want none for denied rollout", usage.committed)
	}
	if !reflect.DeepEqual(usage.recorded, []string{"tenant-rollout-denied:1"}) {
		t.Fatalf("legacy usage = %v, want one recorded unit", usage.recorded)
	}
}

func TestTaskStudioBatchServiceSettlesPersistedLegacyRouteAfterTenantEntersLedger(t *testing.T) {
	t.Parallel()

	ctx := WithTenantID(context.Background(), "tenant-a")
	links := NewMemStudioBatchTaskLinkRepository()
	candidate := studioBatchTaskCandidate{CandidateKey: "candidate-persisted-legacy", ImageStrategy: sheinImageStrategyAIGenerated, SelectionSnapshot: SheinStudioSelection{ProductName: "Canvas Tote"}}
	if err := links.CreateStudioBatchTaskLink(ctx, &StudioBatchTaskLinkRecord{
		ID: "link-persisted-legacy", BatchID: "batch-1", CandidateKey: candidate.CandidateKey,
		ImageStrategy: sheinImageStrategyAIGenerated, Status: studioBatchTaskLinkStatusCreated,
		ProductImageUsageRoute: studioBatchProductImageUsageRouteLegacy, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("CreateStudioBatchTaskLink() error = %v", err)
	}
	usage := &reservingStudioProductImageUsage{}
	service := &taskStudioBatchService{batchTaskLinkRepo: links, productImageUsage: usage, currentTime: time.Now}
	if err := service.settleStudioBatchProductImageUsage(ctx, &StudioBatchRecord{TenantID: "tenant-a"}, candidate); err != nil {
		t.Fatalf("settleStudioBatchProductImageUsage() error = %v", err)
	}
	if got, want := usage.recorded, []string{"tenant-a:1"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("legacy usage = %v, want %v", got, want)
	}
	if len(usage.committed) != 0 {
		t.Fatalf("committed reservations = %v, want none", usage.committed)
	}
}

func TestTaskStudioBatchServiceSettlesPersistedLedgerRouteAfterTenantLeavesLedger(t *testing.T) {
	t.Parallel()

	ctx := WithTenantID(context.Background(), "tenant-a")
	links := NewMemStudioBatchTaskLinkRepository()
	candidate := studioBatchTaskCandidate{CandidateKey: "candidate-persisted-ledger", ImageStrategy: sheinImageStrategyAIGenerated, SelectionSnapshot: SheinStudioSelection{ProductName: "Canvas Tote"}}
	if err := links.CreateStudioBatchTaskLink(ctx, &StudioBatchTaskLinkRecord{
		ID: "link-persisted-ledger", BatchID: "batch-1", CandidateKey: candidate.CandidateKey,
		ImageStrategy: sheinImageStrategyAIGenerated, Status: studioBatchTaskLinkStatusCreated,
		ProductImageUsageRoute: studioBatchProductImageUsageRouteLedger, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("CreateStudioBatchTaskLink() error = %v", err)
	}
	usage := &reservingStudioProductImageUsage{}
	service := &taskStudioBatchService{
		batchTaskLinkRepo:        links,
		productImageUsage:        usage,
		generationUsageAdmission: denyingStudioBatchGenerationUsageAdmission{},
		currentTime:              time.Now,
	}
	if err := service.settleStudioBatchProductImageUsage(ctx, &StudioBatchRecord{TenantID: "tenant-a"}, candidate); err != nil {
		t.Fatalf("settleStudioBatchProductImageUsage() error = %v", err)
	}
	if got, want := usage.committed, []string{"tenant-a:candidate-persisted-ledger"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("committed reservations = %v, want %v", got, want)
	}
	if len(usage.recorded) != 0 {
		t.Fatalf("legacy usage = %v, want none", usage.recorded)
	}
}

func TestTaskStudioBatchServiceUsesLifecycleReservationForPersistedLedgerRoute(t *testing.T) {
	t.Parallel()

	ctx := WithTenantID(context.Background(), "tenant-a")
	links := NewMemStudioBatchTaskLinkRepository()
	candidate := studioBatchTaskCandidate{CandidateKey: "candidate-persisted-ledger-reserve", ImageStrategy: sheinImageStrategyAIGenerated}
	if err := links.CreateStudioBatchTaskLink(ctx, &StudioBatchTaskLinkRecord{
		ID: "link-persisted-ledger-reserve", BatchID: "batch-1", CandidateKey: candidate.CandidateKey,
		ImageStrategy: sheinImageStrategyAIGenerated, Status: studioBatchTaskLinkStatusCreating,
		ProductImageUsageRoute: studioBatchProductImageUsageRouteLedger, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("CreateStudioBatchTaskLink() error = %v", err)
	}
	usage := &lifecycleReservingStudioProductImageUsage{}
	service := &taskStudioBatchService{
		batchTaskLinkRepo:        links,
		productImageUsage:        usage,
		generationUsageAdmission: denyingStudioBatchGenerationUsageAdmission{},
	}
	if err := service.authorizeStudioBatchProductImageUsage(ctx, &StudioBatchRecord{TenantID: "tenant-a"}, candidate, 1); err != nil {
		t.Fatalf("authorizeStudioBatchProductImageUsage() error = %v", err)
	}
	if got, want := usage.lifecycleReserved, []string{"tenant-a:candidate-persisted-ledger-reserve:1"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("lifecycle reservations = %v, want %v", got, want)
	}
	if len(usage.reserved) != 0 {
		t.Fatalf("standard reservations = %v, want none", usage.reserved)
	}
}

func TestTaskStudioBatchServicePersistsCompatibilityLedgerRoute(t *testing.T) {
	t.Parallel()

	ctx := WithTenantID(context.Background(), "tenant-a")
	links := NewMemStudioBatchTaskLinkRepository()
	candidate := studioBatchTaskCandidate{CandidateKey: "candidate-compatibility-ledger", ClaimToken: "claim-1", ImageStrategy: sheinImageStrategyAIGenerated}
	if err := links.CreateStudioBatchTaskLink(ctx, &StudioBatchTaskLinkRecord{
		ID: "link-compatibility-ledger", CandidateKey: candidate.CandidateKey, ClaimToken: candidate.ClaimToken,
		ImageStrategy: sheinImageStrategyAIGenerated, Status: studioBatchTaskLinkStatusCreated,
		CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("CreateStudioBatchTaskLink() error = %v", err)
	}
	service := &taskStudioBatchService{
		batchTaskLinkRepo: links,
		productImageUsage: &lookupReservingStudioProductImageUsage{hasReservation: true},
		currentTime:       time.Now,
	}
	route, err := service.studioBatchProductImageUsageRoute(ctx, &StudioBatchRecord{TenantID: "tenant-a"}, candidate)
	if err != nil {
		t.Fatalf("studioBatchProductImageUsageRoute() error = %v", err)
	}
	if route != studioBatchProductImageUsageRouteLedger {
		t.Fatalf("route = %q, want ledger", route)
	}
	link, err := links.GetStudioBatchTaskLinkByCandidateKey(ctx, candidate.CandidateKey)
	if err != nil {
		t.Fatalf("GetStudioBatchTaskLinkByCandidateKey() error = %v", err)
	}
	if link.ProductImageUsageRoute != studioBatchProductImageUsageRouteLedger {
		t.Fatalf("persisted route = %q, want ledger", link.ProductImageUsageRoute)
	}
}

func TestTaskStudioBatchServiceUsesLegacyRouteForPreChangeLinkWithoutReservation(t *testing.T) {
	t.Parallel()

	ctx := WithTenantID(context.Background(), "tenant-a")
	links := NewMemStudioBatchTaskLinkRepository()
	candidate := studioBatchTaskCandidate{CandidateKey: "candidate-prechange", ImageStrategy: sheinImageStrategyAIGenerated, SelectionSnapshot: SheinStudioSelection{ProductName: "Canvas Tote"}}
	if err := links.CreateStudioBatchTaskLink(ctx, &StudioBatchTaskLinkRecord{
		ID: "link-prechange", BatchID: "batch-1", CandidateKey: candidate.CandidateKey,
		ImageStrategy: sheinImageStrategyAIGenerated, Status: studioBatchTaskLinkStatusCreated,
		CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("CreateStudioBatchTaskLink() error = %v", err)
	}
	usage := &reservingStudioProductImageUsage{}
	service := &taskStudioBatchService{batchTaskLinkRepo: links, productImageUsage: usage, currentTime: time.Now}
	if err := service.settleStudioBatchProductImageUsage(ctx, &StudioBatchRecord{TenantID: "tenant-a"}, candidate); err != nil {
		t.Fatalf("settleStudioBatchProductImageUsage() error = %v", err)
	}
	if got, want := usage.recorded, []string{"tenant-a:1"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("legacy usage = %v, want %v", got, want)
	}
	if len(usage.committed) != 0 {
		t.Fatalf("committed reservations = %v, want none", usage.committed)
	}
}

func TestTaskStudioBatchServiceReleasesDurableProductImageReservation(t *testing.T) {
	t.Parallel()

	usage := &reservingStudioProductImageUsage{}
	service := &taskStudioBatchService{productImageUsage: usage}
	candidate := studioBatchTaskCandidate{
		CandidateKey:      "candidate-release",
		ImageStrategy:     sheinImageStrategyAIGenerated,
		SelectionSnapshot: SheinStudioSelection{ProductName: "Canvas Tote"},
	}
	if err := service.releaseStudioBatchProductImageUsage(context.Background(), &StudioBatchRecord{TenantID: "tenant-a"}, candidate, "generation_failed"); err != nil {
		t.Fatalf("releaseStudioBatchProductImageUsage() error = %v", err)
	}
	if len(usage.released) != 1 || usage.released[0] != "tenant-a:candidate-release:generation_failed" {
		t.Fatalf("released reservations = %v, want durable reservation release", usage.released)
	}
}

func TestTaskStudioBatchServiceKeepsGeneratedImagesWhenUsageRecordFails(t *testing.T) {
	t.Parallel()

	usage := &recordingStudioProductImageUsage{recordErr: errors.New("usage ledger unavailable")}
	service := &taskStudioBatchService{
		productImageUsage: usage,
		generateProductImages: func(context.Context, *StudioProductImageRequest) (*StudioProductImageResponse, error) {
			return &StudioProductImageResponse{Images: []StudioGeneratedImage{{ImageURL: "https://cdn.example.com/generated.png"}}}, nil
		},
	}
	request := buildStudioBatchTaskGenerateRequest(
		&SheinStudioSession{Prompt: "retro", ImageStrategy: sheinImageStrategyAIGenerated},
		&StudioBatchRecord{ID: "batch-usage-record-failure", TenantID: "tenant-a"},
		studioBatchTaskCandidate{ImageStrategy: sheinImageStrategyAIGenerated, SelectionSnapshot: SheinStudioSelection{ProductName: "Canvas Tote"}},
		StudioMaterializedDesignRecord{ID: "design-1", ImageURL: "https://example.com/design.png"},
	)
	if err := service.attachStudioBatchProductImages(context.Background(), request, nil, &StudioBatchRecord{ID: "batch-usage-record-failure", TenantID: "tenant-a"}, studioBatchTaskCandidate{ImageStrategy: sheinImageStrategyAIGenerated, SelectionSnapshot: SheinStudioSelection{ProductName: "Canvas Tote"}}, StudioMaterializedDesignRecord{ID: "design-1", ImageURL: "https://example.com/design.png"}); err != nil {
		t.Fatalf("attachStudioBatchProductImages() error = %v, want generated output retained", err)
	}
	if got := request.Options.SheinStudio.ProductImageURLs; len(got) != 1 || got[0] != "https://cdn.example.com/generated.png" {
		t.Fatalf("product image URLs = %v, want generated output despite usage record failure", got)
	}
}

func TestTaskStudioBatchServiceDefersUsageRecordingUntilAllColorImagesSucceed(t *testing.T) {
	t.Parallel()

	usage := &recordingStudioProductImageUsage{}
	callCount := 0
	service := &taskStudioBatchService{
		productImageUsage: usage,
		generateProductImages: func(context.Context, *StudioProductImageRequest) (*StudioProductImageResponse, error) {
			callCount++
			if callCount == 2 {
				return nil, errors.New("second color generation failed")
			}
			return &StudioProductImageResponse{Images: []StudioGeneratedImage{{ImageURL: fmt.Sprintf("https://cdn.example.com/generated-%d.png", callCount)}}}, nil
		},
	}
	selection := SheinStudioSelection{ProductName: "Canvas Tote", Variants: []SheinStudioSelectionVariant{
		{VariantSKU: "red-s", Color: "Red"},
		{VariantSKU: "blue-s", Color: "Blue"},
	}}
	request := buildStudioBatchTaskGenerateRequest(
		&SheinStudioSession{Prompt: "retro", ImageStrategy: sheinImageStrategyAIGenerated},
		&StudioBatchRecord{ID: "batch-usage-defer", TenantID: "tenant-a"},
		studioBatchTaskCandidate{ImageStrategy: sheinImageStrategyAIGenerated, SelectionSnapshot: selection},
		StudioMaterializedDesignRecord{ID: "design-1", ImageURL: "https://example.com/design.png"},
	)
	err := service.attachStudioBatchProductImages(context.Background(), request, nil, &StudioBatchRecord{ID: "batch-usage-defer", TenantID: "tenant-a"}, studioBatchTaskCandidate{ImageStrategy: sheinImageStrategyAIGenerated, SelectionSnapshot: selection}, StudioMaterializedDesignRecord{ID: "design-1", ImageURL: "https://example.com/design.png"})
	if err == nil || !strings.Contains(err.Error(), "second color generation failed") {
		t.Fatalf("attachStudioBatchProductImages() error = %v, want second-color failure", err)
	}
	if len(usage.recorded) != 0 {
		t.Fatalf("recorded usage = %v, want no settlement after partial generation", usage.recorded)
	}
}

func TestTaskStudioBatchServicePublicizesGeneratedUploadPaths(t *testing.T) {
	usage := &recordingStudioProductImageUsage{}
	service := &taskStudioBatchService{
		productImageUsage: usage,
		resolveUploadedImagePublicURL: func(_ context.Context, key string) (string, error) {
			if key != "upload-1" {
				t.Fatalf("resolved upload key = %q, want upload-1", key)
			}
			return "https://cdn.example.com/upload-1.png", nil
		},
		generateProductImages: func(_ context.Context, _ *StudioProductImageRequest) (*StudioProductImageResponse, error) {
			return &StudioProductImageResponse{Images: []StudioGeneratedImage{{ImageURL: "/api/v1/listing-kits/uploads/files/upload-1"}}}, nil
		},
	}
	candidate := studioBatchTaskCandidate{
		ImageStrategy:     sheinImageStrategyAIGenerated,
		SelectionSnapshot: SheinStudioSelection{ProductName: "Canvas Tote"},
		Title:             "Style 1",
	}
	request := buildStudioBatchTaskGenerateRequest(
		&SheinStudioSession{Prompt: "retro"},
		&StudioBatchRecord{ID: "batch-1"},
		candidate,
		StudioMaterializedDesignRecord{ID: "design-1", ImageURL: "https://example.com/design.png"},
	)
	if err := service.attachStudioBatchProductImages(context.Background(), request, &SheinStudioSession{Prompt: "retro"}, &StudioBatchRecord{ID: "batch-1", TenantID: "tenant-a"}, candidate, StudioMaterializedDesignRecord{ID: "design-1", ImageURL: "https://example.com/design.png"}); err != nil {
		t.Fatalf("attachStudioBatchProductImages() error = %v", err)
	}
	if got := request.Options.SheinStudio.ProductImageURLs; len(got) != 1 || got[0] != "https://cdn.example.com/upload-1.png" {
		t.Fatalf("product image URLs = %v, want public CDN URL", got)
	}
}

type recordingStudioProductImageUsage struct {
	authorized []string
	recorded   []string
	recordErr  error
}

type idempotentRecordingStudioProductImageUsage struct {
	recordingStudioProductImageUsage
	operations map[string]struct{}
}

func (u *idempotentRecordingStudioProductImageUsage) RecordProductImageUsageOnce(_ context.Context, tenantID string, quantity int, operationKey string) error {
	if u.operations == nil {
		u.operations = make(map[string]struct{})
	}
	if _, exists := u.operations[operationKey]; exists {
		return nil
	}
	u.operations[operationKey] = struct{}{}
	u.recorded = append(u.recorded, tenantID+":"+strconv.Itoa(quantity))
	return nil
}

type reservingStudioProductImageUsage struct {
	recordingStudioProductImageUsage
	reserved      []string
	committed     []string
	released      []string
	releaseErrors []error
}

type lifecycleReservingStudioProductImageUsage struct {
	reservingStudioProductImageUsage
	lifecycleReserved []string
}

type lookupReservingStudioProductImageUsage struct {
	reservingStudioProductImageUsage
	hasReservation bool
}

type disabledReservingStudioProductImageUsage struct {
	reservingStudioProductImageUsage
}

type denyingStudioBatchGenerationUsageAdmission struct{}

func (denyingStudioBatchGenerationUsageAdmission) AllowsGenerationUsage(string) bool { return false }

func (u *disabledReservingStudioProductImageUsage) StudioProductImageUsageReservationEnabled() bool {
	return false
}

func (u *reservingStudioProductImageUsage) ReserveProductImageUsage(_ context.Context, tenantID, reservationID string, quantity int) error {
	u.reserved = append(u.reserved, tenantID+":"+reservationID+":"+strconv.Itoa(quantity))
	return nil
}

func (u *lifecycleReservingStudioProductImageUsage) ReserveProductImageUsageForLifecycle(_ context.Context, tenantID, reservationID string, quantity int) error {
	u.lifecycleReserved = append(u.lifecycleReserved, tenantID+":"+reservationID+":"+strconv.Itoa(quantity))
	return nil
}

func (u *lookupReservingStudioProductImageUsage) HasProductImageUsageReservation(context.Context, string, string) (bool, error) {
	return u.hasReservation, nil
}

func (u *reservingStudioProductImageUsage) CommitProductImageUsage(_ context.Context, tenantID, reservationID string) error {
	u.committed = append(u.committed, tenantID+":"+reservationID)
	return nil
}

func (u *reservingStudioProductImageUsage) ReleaseProductImageUsage(_ context.Context, tenantID, reservationID, reason string) error {
	u.released = append(u.released, tenantID+":"+reservationID+":"+reason)
	if len(u.releaseErrors) > 0 {
		err := u.releaseErrors[0]
		u.releaseErrors = u.releaseErrors[1:]
		return err
	}
	return nil
}

func (u *recordingStudioProductImageUsage) AuthorizeProductImageUsage(_ context.Context, tenantID string, quantity int) error {
	u.authorized = append(u.authorized, tenantID+":"+strconv.Itoa(quantity))
	return nil
}

func (u *recordingStudioProductImageUsage) RecordProductImageUsage(_ context.Context, tenantID string, quantity int) error {
	u.recorded = append(u.recorded, tenantID+":"+strconv.Itoa(quantity))
	return u.recordErr
}

func TestTaskStudioBatchServiceFallsBackToLegacyProductImageUsageWithoutLedger(t *testing.T) {
	usage := &disabledReservingStudioProductImageUsage{}
	service := &taskStudioBatchService{productImageUsage: usage}
	err := service.authorizeStudioBatchProductImageUsage(
		context.Background(),
		&StudioBatchRecord{TenantID: "tenant-a"},
		studioBatchTaskCandidate{CandidateKey: "candidate-1", ImageStrategy: sheinImageStrategyAIGenerated},
		2,
	)
	if err != nil {
		t.Fatalf("authorizeStudioBatchProductImageUsage() error = %v", err)
	}
	if got, want := usage.authorized, []string{"tenant-a:2"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("legacy authorizations = %v, want %v", got, want)
	}
	if len(usage.reserved) != 0 {
		t.Fatalf("durable reservations = %v, want none", usage.reserved)
	}
}

func TestTaskStudioBatchServiceProductImageRequestLoadsSDSCategoryPath(t *testing.T) {
	var gotCategoryPath []string
	service := &taskStudioBatchService{
		productImageUsage: &recordingStudioProductImageUsage{},
		sdsProductDetailProvider: stubSDSBaselineRemoteProvider{productDetail: &sdstemplate.ProductDetail{
			ProductSummary: sdstemplate.ProductSummary{
				Categories: []sdstemplate.Category{{Name: "Apparel"}, {Name: "Tops"}},
			},
		}},
		generateProductImages: func(_ context.Context, req *StudioProductImageRequest) (*StudioProductImageResponse, error) {
			gotCategoryPath = append([]string(nil), req.CategoryPath...)
			return &StudioProductImageResponse{Images: []StudioGeneratedImage{{ImageURL: "https://example.com/product.png"}}}, nil
		},
	}
	candidate := studioBatchTaskCandidate{
		ImageStrategy:     sheinImageStrategyAIGenerated,
		SelectionSnapshot: SheinStudioSelection{ParentProductID: 42, ProductName: "T-shirt"},
		Title:             "Style 1",
	}
	request := buildStudioBatchTaskGenerateRequest(
		&SheinStudioSession{Prompt: "retro"},
		&StudioBatchRecord{ID: "batch-1"},
		candidate,
		StudioMaterializedDesignRecord{ID: "design-1", ImageURL: "https://example.com/design.png"},
	)
	if err := service.attachStudioBatchProductImages(context.Background(), request, nil, &StudioBatchRecord{ID: "batch-1"}, candidate, StudioMaterializedDesignRecord{ID: "design-1", ImageURL: "https://example.com/design.png"}); err != nil {
		t.Fatalf("attachStudioBatchProductImages() error = %v", err)
	}
	if got, want := gotCategoryPath, []string{"Apparel", "Tops"}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("category path = %+v, want %+v", got, want)
	}
}

func TestTaskStudioBatchServiceProductImageRequestUsesCandidateCategorySnapshot(t *testing.T) {
	t.Parallel()

	candidate := studioBatchTaskCandidate{
		ImageStrategy:            sheinImageStrategyAIGenerated,
		ProductImageCategoryPath: []string{"Apparel", "Tops"},
		SelectionSnapshot:        SheinStudioSelection{ProductName: "T-shirt"},
	}
	service := &taskStudioBatchService{}
	request, err := service.buildStudioBatchTaskProductImageRequest(
		context.Background(),
		&SheinStudioSession{Prompt: "retro"},
		&StudioBatchRecord{ID: "batch-1"},
		candidate,
		StudioMaterializedDesignRecord{ID: "design-1", ImageURL: "https://example.com/design.png"},
	)
	if err != nil {
		t.Fatalf("buildStudioBatchTaskProductImageRequest() error = %v", err)
	}
	if got, want := request.CategoryPath, []string{"Apparel", "Tops"}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("category path = %+v, want %+v", got, want)
	}
}

func TestTaskStudioBatchServiceRevalidatesDesignAfterProductImageGeneration(t *testing.T) {
	t.Parallel()

	ctx := WithTenantID(context.Background(), "tenant-a")
	repo := NewMemStudioBatchRepository()
	now := time.Now().UTC()
	if err := repo.CreateStudioBatchGraph(ctx, &StudioBatchRecord{ID: "batch-revalidate", CreatedAt: now, UpdatedAt: now}, []StudioBatchItemRecord{{ID: "item-revalidate", BatchID: "batch-revalidate", CreatedAt: now, UpdatedAt: now}}, nil, []StudioMaterializedDesignRecord{{
		ID: "design-revalidate", BatchID: "batch-revalidate", ItemID: "item-revalidate", ImageURL: "https://cdn.example.test/old.png", UpdatedAt: now,
	}}); err != nil {
		t.Fatalf("CreateStudioBatchGraph() error = %v", err)
	}
	service := &taskStudioBatchService{repo: repo}
	candidate := studioBatchTaskCandidate{Design: StudioMaterializedDesignRecord{ID: "design-revalidate", BatchID: "batch-revalidate", ImageURL: "https://cdn.example.test/old.png", UpdatedAt: now}}
	if err := service.revalidateStudioBatchTaskDesign(ctx, candidate); err != nil {
		t.Fatalf("revalidateStudioBatchTaskDesign() unchanged error = %v", err)
	}
	changed := candidate.Design
	changed.ImageURL = "https://cdn.example.test/new.png"
	changed.UpdatedAt = now.Add(time.Second)
	if err := repo.UpdateStudioMaterializedDesign(ctx, &changed); err != nil {
		t.Fatalf("UpdateStudioMaterializedDesign() error = %v", err)
	}
	if err := service.revalidateStudioBatchTaskDesign(ctx, candidate); err == nil {
		t.Fatal("revalidateStudioBatchTaskDesign() error = nil, want changed-design error")
	}
}

func TestTaskStudioBatchServiceReleasesStaleProductImageReservationAfterReclaim(t *testing.T) {
	t.Parallel()

	ctx := WithTenantID(context.Background(), "tenant-a")
	links := NewMemStudioBatchTaskLinkRepository()
	if err := links.CreateStudioBatchTaskLink(ctx, &StudioBatchTaskLinkRecord{
		ID: "link-stale", BatchID: "batch-1", CandidateKey: "candidate-stale",
		ClaimToken: "old-claim", ImageStrategy: sheinImageStrategyAIGenerated,
		Status: studioBatchTaskLinkStatusCreating, ProductImageUsageRoute: studioBatchProductImageUsageRouteLedger, UpdatedAt: time.Now().UTC().Add(-3 * time.Minute),
	}); err != nil {
		t.Fatalf("CreateStudioBatchTaskLink() error = %v", err)
	}
	usage := &reservingStudioProductImageUsage{}
	service := &taskStudioBatchService{
		batchTaskLinkRepo: links,
		productImageUsage: usage,
		currentTime:       time.Now,
	}
	candidate := studioBatchTaskCandidate{
		CandidateKey:  "candidate-stale",
		ImageStrategy: sheinImageStrategyAIGenerated,
		ClaimToken:    "new-claim",
		SelectionSnapshot: SheinStudioSelection{
			ProductName: "Canvas tote",
		},
	}
	claimed, previousClaimToken, err := service.claimStudioBatchTaskCandidate(ctx, &candidate)
	if err != nil || !claimed {
		t.Fatalf("claimStudioBatchTaskCandidate() = (%v, %q, %v), want claimed stale lease", claimed, previousClaimToken, err)
	}
	if previousClaimToken != "old-claim" {
		t.Fatalf("previous claim token = %q, want old-claim", previousClaimToken)
	}
	previous := candidate
	previous.ClaimToken = previousClaimToken
	if err := service.releaseStudioBatchProductImageUsage(ctx, &StudioBatchRecord{TenantID: "tenant-a"}, previous, "stale_reclaimed"); err != nil {
		t.Fatalf("releaseStudioBatchProductImageUsage() error = %v", err)
	}
	if got, want := usage.released, []string{"tenant-a:candidate-stale|old-claim:stale_reclaimed"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("released reservations = %v, want %v", got, want)
	}
}

func TestTaskStudioBatchServicePersistsPendingStaleReservationRelease(t *testing.T) {
	t.Parallel()

	ctx := WithTenantID(context.Background(), "tenant-a")
	links := NewMemStudioBatchTaskLinkRepository()
	if err := links.CreateStudioBatchTaskLink(ctx, &StudioBatchTaskLinkRecord{
		ID: "link-pending-release", BatchID: "batch-1", CandidateKey: "candidate-pending-release",
		ClaimToken: "old-claim", ImageStrategy: sheinImageStrategyAIGenerated,
		Status: studioBatchTaskLinkStatusCreating, ProductImageUsageRoute: studioBatchProductImageUsageRouteLedger, UpdatedAt: time.Now().UTC().Add(-3 * time.Minute),
	}); err != nil {
		t.Fatalf("CreateStudioBatchTaskLink() error = %v", err)
	}
	usage := &reservingStudioProductImageUsage{releaseErrors: []error{errors.New("release temporarily unavailable"), nil}}
	service := &taskStudioBatchService{batchTaskLinkRepo: links, productImageUsage: usage, currentTime: time.Now}
	candidate := studioBatchTaskCandidate{
		CandidateKey:  "candidate-pending-release",
		ClaimToken:    "new-claim",
		ImageStrategy: sheinImageStrategyAIGenerated,
	}
	claimed, previousClaimToken, err := service.claimStudioBatchTaskCandidate(ctx, &candidate)
	if err != nil || !claimed {
		t.Fatalf("claimStudioBatchTaskCandidate() = (%v, %q, %v), want stale claim", claimed, previousClaimToken, err)
	}
	claimedLink, err := links.GetStudioBatchTaskLinkByCandidateKey(ctx, candidate.CandidateKey)
	if err != nil {
		t.Fatalf("GetStudioBatchTaskLinkByCandidateKey() after reclaim error = %v", err)
	}
	if claimedLink.PendingProductImageUsageReleaseClaimToken != previousClaimToken {
		t.Fatalf("atomic pending release token = %q, want %q", claimedLink.PendingProductImageUsageReleaseClaimToken, previousClaimToken)
	}
	previous := candidate
	previous.ClaimToken = previousClaimToken
	if err := service.releaseStudioBatchProductImageUsage(ctx, &StudioBatchRecord{TenantID: "tenant-a"}, previous, "stale_reclaimed"); err == nil {
		t.Fatal("releaseStudioBatchProductImageUsage() unexpectedly succeeded")
	}
	if err := service.persistPendingStudioBatchProductImageUsageRelease(ctx, candidate, previousClaimToken); err != nil {
		t.Fatalf("persistPendingStudioBatchProductImageUsageRelease() error = %v", err)
	}
	link, err := links.GetStudioBatchTaskLinkByCandidateKey(ctx, candidate.CandidateKey)
	if err != nil {
		t.Fatalf("GetStudioBatchTaskLinkByCandidateKey() error = %v", err)
	}
	if link.PendingProductImageUsageReleaseClaimToken != previousClaimToken {
		t.Fatalf("pending release token = %q, want %q", link.PendingProductImageUsageReleaseClaimToken, previousClaimToken)
	}
	if err := service.releasePendingStudioBatchProductImageUsage(ctx, &StudioBatchRecord{TenantID: "tenant-a"}, candidate); err != nil {
		t.Fatalf("releasePendingStudioBatchProductImageUsage() error = %v", err)
	}
	link, err = links.GetStudioBatchTaskLinkByCandidateKey(ctx, candidate.CandidateKey)
	if err != nil {
		t.Fatalf("GetStudioBatchTaskLinkByCandidateKey() after retry error = %v", err)
	}
	if link.PendingProductImageUsageReleaseClaimToken != "" {
		t.Fatalf("pending release token = %q after retry, want empty", link.PendingProductImageUsageReleaseClaimToken)
	}
	if got, want := usage.released, []string{"tenant-a:candidate-pending-release|old-claim:stale_reclaimed", "tenant-a:candidate-pending-release|old-claim:pending_release_retry"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("released reservations = %v, want %v", got, want)
	}
}

func TestTaskStudioBatchServiceRecoversCreatedTaskWhenLinkPersistenceFails(t *testing.T) {
	t.Parallel()

	ctx := WithTenantID(context.Background(), "tenant-a")
	links := NewMemStudioBatchTaskLinkRepository()
	candidate := studioBatchTaskCandidate{
		CandidateKey:  "candidate-link-recovery",
		ClaimToken:    "claim-link-recovery",
		ImageStrategy: sheinImageStrategyAIGenerated,
		Design: StudioMaterializedDesignRecord{
			ID: "design-1", BatchID: "batch-1", ItemID: "item-1",
		},
		Item:        StudioBatchItemRecord{ID: "item-1", BatchID: "batch-1"},
		SelectionID: "selection-1",
	}
	if err := links.CreateStudioBatchTaskLink(ctx, &StudioBatchTaskLinkRecord{
		ID: "link-recovery", BatchID: "batch-1", ItemID: "item-1", DesignID: "design-1",
		SelectionID: "selection-1", CandidateKey: candidate.CandidateKey,
		ClaimToken: candidate.ClaimToken, ImageStrategy: sheinImageStrategyAIGenerated,
		Status: studioBatchTaskLinkStatusCreating, ProductImageUsageRoute: studioBatchProductImageUsageRouteLedger, UpdatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("CreateStudioBatchTaskLink() error = %v", err)
	}
	usage := &reservingStudioProductImageUsage{}
	markFailed := false
	service := &taskStudioBatchService{
		batchTaskLinkRepo: links,
		productImageUsage: usage,
		currentTime:       time.Now,
		markTaskFailed: func(_ context.Context, taskID, message string) error {
			if taskID != "task-created-after-link-error" || !strings.Contains(message, "link persistence") {
				t.Fatalf("MarkFailed() = (%q, %q)", taskID, message)
			}
			markFailed = true
			return nil
		},
	}

	linkErr := errors.New("link persistence temporarily unavailable")
	if err := service.recoverStudioBatchTaskAfterLinkPersistenceFailure(
		ctx,
		&StudioBatchRecord{TenantID: "tenant-a"},
		candidate,
		&Task{ID: "task-created-after-link-error"},
		linkErr,
	); err != nil {
		t.Fatalf("recoverStudioBatchTaskAfterLinkPersistenceFailure() error = %v", err)
	}
	if !markFailed {
		t.Fatal("created task was not marked failed")
	}
	link, err := links.GetStudioBatchTaskLinkByCandidateKey(ctx, candidate.CandidateKey)
	if err != nil {
		t.Fatalf("GetStudioBatchTaskLinkByCandidateKey() error = %v", err)
	}
	if link.Status != studioBatchTaskLinkStatusFailed || link.ListingKitTaskID != "task-created-after-link-error" {
		t.Fatalf("recovered link = %+v, want failed link retaining task identity", link)
	}
	if got, want := usage.released, []string{"tenant-a:candidate-link-recovery|claim-link-recovery:task_link_persistence_failed"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("released reservations = %v, want %v", got, want)
	}
}

func TestTaskStudioBatchServiceKeepsReservationWhenTaskTerminationFails(t *testing.T) {
	t.Parallel()

	ctx := WithTenantID(context.Background(), "tenant-a")
	links := NewMemStudioBatchTaskLinkRepository()
	candidate := studioBatchTaskCandidate{
		CandidateKey:  "candidate-link-recovery-pending",
		ClaimToken:    "claim-link-recovery-pending",
		ImageStrategy: sheinImageStrategyAIGenerated,
		Design:        StudioMaterializedDesignRecord{ID: "design-1", BatchID: "batch-1", ItemID: "item-1"},
		Item:          StudioBatchItemRecord{ID: "item-1", BatchID: "batch-1"}, SelectionID: "selection-1",
	}
	if err := links.CreateStudioBatchTaskLink(ctx, &StudioBatchTaskLinkRecord{
		ID: "link-recovery-pending", BatchID: "batch-1", ItemID: "item-1", DesignID: "design-1",
		SelectionID: "selection-1", CandidateKey: candidate.CandidateKey, ClaimToken: candidate.ClaimToken,
		ImageStrategy: sheinImageStrategyAIGenerated, Status: studioBatchTaskLinkStatusCreating, UpdatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("CreateStudioBatchTaskLink() error = %v", err)
	}
	usage := &reservingStudioProductImageUsage{}
	service := &taskStudioBatchService{
		batchTaskLinkRepo: links, productImageUsage: usage, currentTime: time.Now,
		markTaskFailed: func(context.Context, string, string) error { return errors.New("task store temporarily unavailable") },
	}
	linkErr := errors.New("link persistence temporarily unavailable")
	if err := service.recoverStudioBatchTaskAfterLinkPersistenceFailure(ctx, &StudioBatchRecord{TenantID: "tenant-a"}, candidate, &Task{ID: "task-created-after-link-error"}, linkErr); err == nil {
		t.Fatal("recoverStudioBatchTaskAfterLinkPersistenceFailure() error = nil, want termination failure")
	}
	link, err := links.GetStudioBatchTaskLinkByCandidateKey(ctx, candidate.CandidateKey)
	if err != nil {
		t.Fatalf("GetStudioBatchTaskLinkByCandidateKey() error = %v", err)
	}
	if link.Status != studioBatchTaskLinkStatusCreated || link.ListingKitTaskID != "task-created-after-link-error" {
		t.Fatalf("recovery link = %+v, want created link retaining task identity", link)
	}
	if len(usage.released) != 0 {
		t.Fatalf("released reservations = %v, want none", usage.released)
	}
}

func TestTaskStudioBatchServiceReturnsFailedReservationTokenAfterReclaim(t *testing.T) {
	t.Parallel()

	ctx := WithTenantID(context.Background(), "tenant-a")
	links := NewMemStudioBatchTaskLinkRepository()
	if err := links.CreateStudioBatchTaskLink(ctx, &StudioBatchTaskLinkRecord{
		ID: "link-failed", BatchID: "batch-1", CandidateKey: "candidate-failed",
		ClaimToken: "old-claim", ImageStrategy: sheinImageStrategyAIGenerated,
		Status: studioBatchTaskLinkStatusFailed,
	}); err != nil {
		t.Fatalf("CreateStudioBatchTaskLink() error = %v", err)
	}
	service := &taskStudioBatchService{batchTaskLinkRepo: links, currentTime: time.Now}
	candidate := studioBatchTaskCandidate{CandidateKey: "candidate-failed", ImageStrategy: sheinImageStrategyAIGenerated}
	claimed, previousClaimToken, err := service.claimStudioBatchTaskCandidate(ctx, &candidate)
	if err != nil || !claimed {
		t.Fatalf("claimStudioBatchTaskCandidate() = (%v, %q, %v), want claimed failed reservation", claimed, previousClaimToken, err)
	}
	if previousClaimToken != "old-claim" {
		t.Fatalf("previous claim token = %q, want old-claim", previousClaimToken)
	}
	if candidate.ClaimToken == "old-claim" || strings.TrimSpace(candidate.ClaimToken) == "" {
		t.Fatalf("new claim token = %q, want a fresh token", candidate.ClaimToken)
	}
}

func TestTaskStudioBatchServiceLinkHeartbeatRefreshesCreatingClaim(t *testing.T) {
	ctx := WithTenantID(context.Background(), "tenant-a")
	links := NewMemStudioBatchTaskLinkRepository()
	initial := time.Now().UTC().Add(-time.Minute)
	if err := links.CreateStudioBatchTaskLink(ctx, &StudioBatchTaskLinkRecord{
		ID: "link-1", BatchID: "batch-1", ItemID: "item-1", DesignID: "design-1", SelectionID: "selection-1",
		CandidateKey: "candidate-1", ClaimToken: "claim-1", Status: studioBatchTaskLinkStatusCreating, UpdatedAt: initial,
	}); err != nil {
		t.Fatalf("CreateStudioBatchTaskLink() error = %v", err)
	}
	service := &taskStudioBatchService{batchTaskLinkRepo: links, currentTime: time.Now}
	stop := service.startStudioBatchTaskLinkHeartbeat(ctx, studioBatchTaskCandidate{CandidateKey: "candidate-1", ClaimToken: "claim-1"}, time.Millisecond)
	time.Sleep(10 * time.Millisecond)
	if err := stop(); err != nil {
		t.Fatalf("heartbeat stop error = %v", err)
	}
	link, err := links.GetStudioBatchTaskLinkByCandidateKey(ctx, "candidate-1")
	if err != nil {
		t.Fatalf("GetStudioBatchTaskLinkByCandidateKey() error = %v", err)
	}
	if !link.UpdatedAt.After(initial) {
		t.Fatalf("UpdatedAt = %s, want refreshed after %s", link.UpdatedAt, initial)
	}
}

func TestStudioBatchTaskLinkHeartbeatRejectsReclaimedClaimToken(t *testing.T) {
	ctx := WithTenantID(context.Background(), "tenant-a")
	links := NewMemStudioBatchTaskLinkRepository()
	now := time.Now().UTC()
	if err := links.CreateStudioBatchTaskLink(ctx, &StudioBatchTaskLinkRecord{
		ID: "link-1", CandidateKey: "candidate-1", ClaimToken: "new-owner", Status: studioBatchTaskLinkStatusCreating, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("CreateStudioBatchTaskLink() error = %v", err)
	}
	refreshed, err := links.RefreshStudioBatchTaskLink(ctx, "candidate-1", "old-owner", now.Add(time.Second))
	if err != nil {
		t.Fatalf("RefreshStudioBatchTaskLink() error = %v", err)
	}
	if refreshed {
		t.Fatal("old claim token unexpectedly refreshed a reclaimed link")
	}
}

func TestStudioBatchTaskLeaseRevalidationRejectsReclaimedClaimToken(t *testing.T) {
	t.Parallel()

	ctx := WithTenantID(context.Background(), "tenant-a")
	links := NewMemStudioBatchTaskLinkRepository()
	if err := links.CreateStudioBatchTaskLink(ctx, &StudioBatchTaskLinkRecord{
		ID: "link-1", CandidateKey: "candidate-1", ClaimToken: "new-owner", Status: studioBatchTaskLinkStatusCreating,
	}); err != nil {
		t.Fatalf("CreateStudioBatchTaskLink() error = %v", err)
	}
	service := &taskStudioBatchService{batchTaskLinkRepo: links, currentTime: func() time.Time { return time.Unix(100, 0).UTC() }}
	err := service.revalidateStudioBatchTaskLinkLease(ctx, studioBatchTaskCandidate{
		CandidateKey: "candidate-1", ClaimToken: "old-owner",
	})
	if err == nil || !strings.Contains(err.Error(), "no longer owned") {
		t.Fatalf("revalidateStudioBatchTaskLinkLease() error = %v, want lease-loss error", err)
	}
}

func TestStudioBatchTaskLinkHeartbeatCancelsDispatchContextOnLeaseLoss(t *testing.T) {
	ctx := WithTenantID(context.Background(), "tenant-a")
	links := NewMemStudioBatchTaskLinkRepository()
	if err := links.CreateStudioBatchTaskLink(ctx, &StudioBatchTaskLinkRecord{
		ID: "link-1", CandidateKey: "candidate-1", ClaimToken: "new-owner", Status: studioBatchTaskLinkStatusCreating,
	}); err != nil {
		t.Fatalf("CreateStudioBatchTaskLink() error = %v", err)
	}
	service := &taskStudioBatchService{batchTaskLinkRepo: links, currentTime: time.Now}
	heartbeatCtx, stop := service.startStudioBatchTaskLinkHeartbeatContext(ctx, studioBatchTaskCandidate{
		CandidateKey: "candidate-1", ClaimToken: "old-owner",
	}, time.Millisecond)
	select {
	case <-heartbeatCtx.Done():
	case <-time.After(250 * time.Millisecond):
		t.Fatal("heartbeat did not cancel dispatch context after lease loss")
	}
	if err := stop(); err == nil {
		t.Fatal("heartbeat stop unexpectedly hid the active lease-loss error")
	}
}

func TestStudioBatchTaskLinkHeartbeatSurvivesCallerCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(WithTenantID(context.Background(), "tenant-a"))
	links := NewMemStudioBatchTaskLinkRepository()
	if err := links.CreateStudioBatchTaskLink(ctx, &StudioBatchTaskLinkRecord{
		ID: "link-1", CandidateKey: "candidate-1", ClaimToken: "owner-a", Status: studioBatchTaskLinkStatusCreating,
	}); err != nil {
		t.Fatalf("CreateStudioBatchTaskLink() error = %v", err)
	}
	service := &taskStudioBatchService{batchTaskLinkRepo: links, currentTime: time.Now}
	heartbeatCtx, stop := service.startStudioBatchTaskLinkHeartbeatContext(ctx, studioBatchTaskCandidate{
		CandidateKey: "candidate-1", ClaimToken: "owner-a",
	}, 50*time.Millisecond)
	cancel()
	select {
	case <-heartbeatCtx.Done():
		t.Fatal("heartbeat context canceled with caller context")
	case <-time.After(10 * time.Millisecond):
	}
	if err := stop(); err != nil {
		t.Fatalf("heartbeat stop error = %v", err)
	}
}

func TestStudioBatchTaskHeartbeatTerminalStateRequiresMatchingClaim(t *testing.T) {
	ctx := WithTenantID(context.Background(), "tenant-a")
	links := NewMemStudioBatchTaskLinkRepository()
	if err := links.CreateStudioBatchTaskLink(ctx, &StudioBatchTaskLinkRecord{
		ID: "link-1", CandidateKey: "candidate-1", ClaimToken: "owner-a", Status: studioBatchTaskLinkStatusCreated,
	}); err != nil {
		t.Fatalf("CreateStudioBatchTaskLink() error = %v", err)
	}
	service := &taskStudioBatchService{batchTaskLinkRepo: links}
	if !service.studioBatchTaskHeartbeatEndedInTerminalState(ctx, studioBatchTaskCandidate{CandidateKey: "candidate-1", ClaimToken: "owner-a"}) {
		t.Fatal("matching terminal claim was not recognized")
	}
	if service.studioBatchTaskHeartbeatEndedInTerminalState(ctx, studioBatchTaskCandidate{CandidateKey: "candidate-1", ClaimToken: "owner-b"}) {
		t.Fatal("replacement terminal claim was accepted as the original owner")
	}
}

func TestCreatedTaskFromDurableLinkRejectsHistoricalAIWithoutSettingsIdentity(t *testing.T) {
	candidate := studioBatchTaskCandidate{
		ImageStrategy:                   sheinImageStrategyAIGenerated,
		CompatibilityFingerprint:        "selection-fingerprint",
		ProductImageSettingsFingerprint: "current-settings",
	}
	service := &taskStudioBatchService{}
	legacyLink := &StudioBatchTaskLinkRecord{
		ListingKitTaskID:         "task-legacy-ai",
		ImageStrategy:            sheinImageStrategyAIGenerated,
		CompatibilityFingerprint: "selection-fingerprint",
		Status:                   studioBatchTaskLinkStatusCreated,
	}

	if _, ok := service.createdTaskFromDurableLink(context.Background(), legacyLink, candidate); ok {
		t.Fatal("createdTaskFromDurableLink() reused an AI link without the settings identity")
	}
}

func TestCreatedTaskFromDurableLinkAcceptsMatchingAISettingsIdentity(t *testing.T) {
	candidate := studioBatchTaskCandidate{
		ImageStrategy:                   sheinImageStrategyAIGenerated,
		CompatibilityFingerprint:        "selection-fingerprint",
		ProductImageSettingsFingerprint: "current-settings",
	}
	service := &taskStudioBatchService{
		getTask: func(context.Context, string) (*Task, error) {
			return &Task{Request: &GenerateRequest{Options: &GenerateOptions{
				SheinStudio: &SheinStudioOptions{ProductImageURLs: []string{"https://example.com/generated.png"}},
			}}}, nil
		},
	}
	link := &StudioBatchTaskLinkRecord{
		ListingKitTaskID:         "task-ai",
		ImageStrategy:            sheinImageStrategyAIGenerated,
		CompatibilityFingerprint: studioBatchTaskLinkCompatibilityFingerprint(candidate),
		Status:                   studioBatchTaskLinkStatusCreated,
	}

	if _, ok := service.createdTaskFromDurableLink(context.Background(), link, candidate); !ok {
		t.Fatal("createdTaskFromDurableLink() rejected a matching AI settings identity")
	}
}

func TestStudioBatchTaskImageStrategyPrefersExplicitRequest(t *testing.T) {
	t.Parallel()

	strategy := " AI_GENERATED "
	got := resolveStudioBatchTaskImageStrategy(
		&CreateStudioBatchTasksRequest{ImageStrategy: &strategy},
		&SheinStudioSession{ImageStrategy: sheinImageStrategySDSOfficial},
		nil,
	)

	if got != sheinImageStrategyAIGenerated {
		t.Fatalf("strategy = %q, want %q", got, sheinImageStrategyAIGenerated)
	}
}

func TestStudioBatchTaskImageStrategyFallsBackToPersistedBatch(t *testing.T) {
	t.Parallel()

	got := resolveStudioBatchTaskImageStrategy(
		&CreateStudioBatchTasksRequest{},
		nil,
		&StudioBatchRecord{ImageStrategy: sheinImageStrategyAIGenerated},
	)
	if got != sheinImageStrategyAIGenerated {
		t.Fatalf("strategy = %q, want persisted %q", got, sheinImageStrategyAIGenerated)
	}
}

func TestStudioBatchTaskImageStrategyMapsLegacySessionHybridToSDS(t *testing.T) {
	t.Parallel()

	got := resolveStudioBatchTaskImageStrategy(
		&CreateStudioBatchTasksRequest{},
		&SheinStudioSession{ImageStrategy: sheinImageStrategyHybrid},
		nil,
	)
	if got != sheinImageStrategySDSOfficial {
		t.Fatalf("strategy = %q, want %q", got, sheinImageStrategySDSOfficial)
	}
}

func TestStudioBatchTaskImageStrategyMapsRemovedHybridModeToSDS(t *testing.T) {
	t.Parallel()

	strategy := "hybrid"
	got := resolveStudioBatchTaskImageStrategy(
		&CreateStudioBatchTasksRequest{ImageStrategy: &strategy},
		&SheinStudioSession{ImageStrategy: sheinImageStrategyAIGenerated},
		nil,
	)

	if got != sheinImageStrategySDSOfficial {
		t.Fatalf("strategy = %q, want %q", got, sheinImageStrategySDSOfficial)
	}
}

func TestStudioBatchTaskCreationRequestCarriesStrategyAcrossResumeContext(t *testing.T) {
	t.Parallel()

	strategy := sheinImageStrategyAIGenerated
	ctx := withStudioBatchTaskImageStrategy(context.Background(), &CreateStudioBatchTasksRequest{
		ImageStrategy: &strategy,
	})
	req := studioBatchTaskCreationRequest(ctx, []string{"design-1"})
	if req == nil || req.ImageStrategy == nil {
		t.Fatal("request strategy is nil")
	}
	if got, want := *req.ImageStrategy, sheinImageStrategyAIGenerated; got != want {
		t.Fatalf("request strategy = %q, want %q", got, want)
	}
}

func TestFallbackStudioBatchTaskSessionKeepsBatchIdentityAndSelection(t *testing.T) {
	t.Parallel()

	session := fallbackStudioBatchTaskSession(
		"batch-1",
		&StudioBatchRecord{
			ID:                  "batch-1",
			SheinStoreID:        869,
			Prompt:              "batch prompt",
			PromptMode:          "raw",
			ProductImageCount:   "7",
			ProductImagePrompt:  "use a clean studio background",
			ProductImagePrompts: SheinStudioProductImagePromptList{{Role: "hero", Prompt: "front view"}},
			Selection:           SheinStudioSelectionSnapshot{ProductID: 1001, ParentProductID: 2002},
		},
		[]string{"design-1"},
		sheinImageStrategyAIGenerated,
	)
	if session.ID != "batch-1" {
		t.Fatalf("session ID = %q, want batch-1", session.ID)
	}
	if session.ImageStrategy != sheinImageStrategyAIGenerated {
		t.Fatalf("session strategy = %q, want %q", session.ImageStrategy, sheinImageStrategyAIGenerated)
	}
	if session.Selection.ProductID != 1001 || session.SheinStoreID != "869" {
		t.Fatalf("session fallback data = %+v, want batch selection/store", session)
	}
	if session.Prompt != "batch prompt" || session.PromptMode != "raw" || session.ProductImageCount != "7" || session.ProductImagePrompt != "use a clean studio background" {
		t.Fatalf("session fallback product image settings = %+v, want persisted batch settings", session)
	}
	if len(session.ProductImagePrompts) != 1 || session.ProductImagePrompts[0].Prompt != "front view" {
		t.Fatalf("session fallback product image prompts = %+v, want persisted batch prompts", session.ProductImagePrompts)
	}
}

func TestBuildStudioBatchRecordFromSessionDraftCopiesProductImageSettings(t *testing.T) {
	t.Parallel()

	batch := buildStudioBatchRecordFromSessionDraft(&SheinStudioSession{
		ID:                  "batch-1",
		PromptMode:          "raw",
		ProductImageCount:   "6",
		ProductImagePrompt:  "keep the print centered",
		ProductImagePrompts: SheinStudioProductImagePromptList{{Role: "detail", Prompt: "show stitching"}},
	}, time.Now())
	if batch.PromptMode != "raw" || batch.ProductImageCount != "6" || batch.ProductImagePrompt != "keep the print centered" {
		t.Fatalf("batch product image settings = %+v, want copied session settings", batch)
	}
	if len(batch.ProductImagePrompts) != 1 || batch.ProductImagePrompts[0].Prompt != "show stitching" {
		t.Fatalf("batch product image prompts = %+v, want copied session prompts", batch.ProductImagePrompts)
	}
}

func TestStudioBatchTaskMatchesSelectionRejectsDifferentImageStrategy(t *testing.T) {
	t.Parallel()

	task := &Task{
		Request: &GenerateRequest{
			ImageURLs: []string{"https://example.com/design.png"},
			Options: &GenerateOptions{
				ImageStrategy: sheinImageStrategySDSOfficial,
				SheinStudio:   &SheinStudioOptions{StyleID: "style-1"},
				SDS:           &SDSSyncOptions{VariantID: 1, ParentProductID: 2, PrototypeGroupID: 3, LayerID: "layer-1"},
			},
		},
	}
	candidate := studioBatchTaskCandidate{
		Design:        StudioMaterializedDesignRecord{ID: "design-1", ImageURL: "https://example.com/design.png"},
		StyleID:       "style-1",
		ImageStrategy: sheinImageStrategyAIGenerated,
		SelectionSnapshot: SheinStudioSelection{
			VariantID: 1, ParentProductID: 2, PrototypeGroupID: 3, LayerID: "layer-1",
		},
	}

	if studioBatchTaskMatchesSelection(task, candidate) {
		t.Fatal("SDS task unexpectedly matched AI candidate")
	}
}

func TestStudioBatchTaskMatchesSelectionRejectsAIWithoutGeneratedProductImages(t *testing.T) {
	t.Parallel()

	task := &Task{
		Request: &GenerateRequest{
			ImageURLs: []string{"https://example.com/design.png"},
			Options: &GenerateOptions{
				ImageStrategy: sheinImageStrategyAIGenerated,
				SheinStudio:   &SheinStudioOptions{StyleID: "style-1"},
				SDS:           &SDSSyncOptions{VariantID: 1, ParentProductID: 2, PrototypeGroupID: 3, LayerID: "layer-1"},
			},
		},
	}
	candidate := studioBatchTaskCandidate{
		Design:        StudioMaterializedDesignRecord{ID: "design-1", ImageURL: "https://example.com/design.png"},
		StyleID:       "style-1",
		ImageStrategy: sheinImageStrategyAIGenerated,
		SelectionSnapshot: SheinStudioSelection{
			VariantID: 1, ParentProductID: 2, PrototypeGroupID: 3, LayerID: "layer-1",
		},
	}
	if studioBatchTaskMatchesSelection(task, candidate) {
		t.Fatal("AI task without generated product images unexpectedly matched")
	}
}

func TestFindLegacyStudioBatchTaskDoesNotReuseAIWithoutPersistedSettingsIdentity(t *testing.T) {
	t.Parallel()

	s := &taskStudioBatchService{
		getTask: func(context.Context, string) (*Task, error) {
			t.Fatal("legacy AI task was looked up without a persisted settings identity")
			return nil, nil
		},
	}
	candidate := studioBatchTaskCandidate{
		Design:                          StudioMaterializedDesignRecord{ID: "design-1"},
		ImageStrategy:                   sheinImageStrategyAIGenerated,
		ProductImageSettingsFingerprint: "settings-fingerprint",
	}
	created, ok, err := s.findLegacyStudioBatchTask(context.Background(), SheinStudioCreatedTaskList{{ID: "legacy-ai", DesignID: "design-1"}}, candidate)
	if err != nil {
		t.Fatalf("findLegacyStudioBatchTask() error = %v", err)
	}
	if ok || created.ID != "" {
		t.Fatalf("findLegacyStudioBatchTask() = (%+v, %t), want no reuse", created, ok)
	}
}

func TestPersistStudioBatchTaskLinkRejectsReclaimedClaimToken(t *testing.T) {
	t.Parallel()

	links := NewMemStudioBatchTaskLinkRepository()
	ctx := WithTenantID(context.Background(), "tenant-1")
	if err := links.CreateStudioBatchTaskLink(ctx, &StudioBatchTaskLinkRecord{
		ID:            "link-1",
		BatchID:       "batch-1",
		ItemID:        "item-1",
		DesignID:      "design-1",
		SelectionID:   "selection-1",
		CandidateKey:  "candidate-1",
		ImageStrategy: sheinImageStrategyAIGenerated,
		Status:        studioBatchTaskLinkStatusCreating,
		ClaimToken:    "new-owner",
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}); err != nil {
		t.Fatalf("create creating link: %v", err)
	}
	s := &taskStudioBatchService{batchTaskLinkRepo: links, currentTime: time.Now}
	candidate := studioBatchTaskCandidate{
		Design:        StudioMaterializedDesignRecord{BatchID: "batch-1", ID: "design-1"},
		Item:          StudioBatchItemRecord{BatchID: "batch-1", ID: "item-1"},
		SelectionID:   "selection-1",
		CandidateKey:  "candidate-1",
		ImageStrategy: sheinImageStrategyAIGenerated,
		ClaimToken:    "old-owner",
	}
	if err := s.persistStudioBatchTaskLink(ctx, candidate, "stale-task", studioBatchTaskLinkStatusFailed, studioBatchTaskLinkSourceBatchCreated, "provider_error", "stale"); err == nil {
		t.Fatal("persistStudioBatchTaskLink() unexpectedly updated a reclaimed claim")
	}
	got, err := links.GetStudioBatchTaskLinkByCandidateKey(ctx, "candidate-1")
	if err != nil {
		t.Fatalf("get creating link: %v", err)
	}
	if got.Status != studioBatchTaskLinkStatusCreating || got.ClaimToken != "new-owner" || got.ListingKitTaskID != "" {
		t.Fatalf("reclaimed link mutated by stale worker: %+v", got)
	}
}

func TestStudioBatchTaskLinkMatchesImageStrategyRejectsHistoricalAIMismatch(t *testing.T) {
	t.Parallel()

	link := &StudioBatchTaskLinkRecord{ImageStrategy: ""}
	task := &Task{Request: &GenerateRequest{Options: &GenerateOptions{ImageStrategy: sheinImageStrategyAIGenerated}}}
	candidate := studioBatchTaskCandidate{ImageStrategy: sheinImageStrategySDSOfficial}
	if studioBatchTaskLinkMatchesImageStrategy(link, task, candidate) {
		t.Fatal("historical AI task unexpectedly matched SDS candidate")
	}
}

func TestStudioBatchTaskLinkMatchesImageStrategyRejectsUnknownHistoricalStrategy(t *testing.T) {
	t.Parallel()

	link := &StudioBatchTaskLinkRecord{Status: studioBatchTaskLinkStatusCreated, ListingKitTaskID: "historical-task"}
	candidate := studioBatchTaskCandidate{ImageStrategy: sheinImageStrategySDSOfficial}
	if studioBatchTaskLinkMatchesImageStrategy(link, nil, candidate) {
		t.Fatal("unknown historical strategy unexpectedly matched SDS candidate")
	}
}

func TestReserveStudioBatchTaskCandidateDisambiguatesHistoricalStrategyCollision(t *testing.T) {
	t.Parallel()

	links := NewMemStudioBatchTaskLinkRepository()
	ctx := WithTenantID(context.Background(), "tenant-1")
	legacy := &StudioBatchTaskLinkRecord{
		ID:               "legacy-link",
		BatchID:          "batch-1",
		ItemID:           "item-1",
		DesignID:         "design-1",
		SelectionID:      "selection-1",
		CandidateKey:     "legacy-key",
		ListingKitTaskID: "old-ai-task",
		Status:           studioBatchTaskLinkStatusCreated,
	}
	if err := links.CreateStudioBatchTaskLink(ctx, legacy); err != nil {
		t.Fatalf("create legacy link: %v", err)
	}
	s := &taskStudioBatchService{
		batchTaskLinkRepo: links,
		currentTime:       time.Now,
		getTask: func(context.Context, string) (*Task, error) {
			return &Task{Request: &GenerateRequest{Options: &GenerateOptions{ImageStrategy: sheinImageStrategyAIGenerated}}}, nil
		},
	}
	candidate := studioBatchTaskCandidate{
		Design:        StudioMaterializedDesignRecord{BatchID: "batch-1", ID: "design-1"},
		Item:          StudioBatchItemRecord{ID: "item-1", BatchID: "batch-1"},
		SelectionID:   "selection-1",
		CandidateKey:  "legacy-key",
		ImageStrategy: sheinImageStrategySDSOfficial,
	}
	if err := s.reserveStudioBatchTaskCandidate(ctx, &candidate); err != nil {
		t.Fatalf("reserve strategy-colliding candidate: %v", err)
	}
	if candidate.CandidateKey == "legacy-key" {
		t.Fatal("candidate key was not disambiguated from historical strategy collision")
	}
	if _, err := links.GetStudioBatchTaskLinkByCandidateKey(ctx, candidate.CandidateKey); err != nil {
		t.Fatalf("get disambiguated link: %v", err)
	}
	second := studioBatchTaskCandidate{
		Design:        candidate.Design,
		Item:          candidate.Item,
		SelectionID:   candidate.SelectionID,
		CandidateKey:  "legacy-key",
		ImageStrategy: sheinImageStrategySDSOfficial,
	}
	if err := s.reserveStudioBatchTaskCandidate(ctx, &second); err != nil {
		t.Fatalf("repeat reserve of disambiguated candidate: %v", err)
	}
}

func TestPersistStudioBatchTaskLinkDisambiguatesStrategyCollision(t *testing.T) {
	t.Parallel()

	links := NewMemStudioBatchTaskLinkRepository()
	ctx := WithTenantID(context.Background(), "tenant-1")
	legacy := &StudioBatchTaskLinkRecord{
		ID:               "legacy-link",
		BatchID:          "batch-1",
		ItemID:           "item-1",
		DesignID:         "design-1",
		SelectionID:      "selection-1",
		CandidateKey:     "legacy-key",
		ListingKitTaskID: "old-ai-task",
		Status:           studioBatchTaskLinkStatusCreated,
	}
	if err := links.CreateStudioBatchTaskLink(ctx, legacy); err != nil {
		t.Fatalf("create legacy link: %v", err)
	}
	s := &taskStudioBatchService{
		batchTaskLinkRepo: links,
		currentTime:       time.Now,
		getTask: func(context.Context, string) (*Task, error) {
			return &Task{Request: &GenerateRequest{Options: &GenerateOptions{ImageStrategy: sheinImageStrategyAIGenerated}}}, nil
		},
	}
	candidate := studioBatchTaskCandidate{
		Design:        StudioMaterializedDesignRecord{BatchID: "batch-1", ID: "design-1"},
		Item:          StudioBatchItemRecord{ID: "item-1", BatchID: "batch-1"},
		SelectionID:   "selection-1",
		CandidateKey:  "legacy-key",
		ImageStrategy: sheinImageStrategySDSOfficial,
	}
	if err := s.persistStudioBatchTaskLink(ctx, candidate, "new-sds-task", studioBatchTaskLinkStatusCreated, studioBatchTaskLinkSourceBatchCreated, "", ""); err != nil {
		t.Fatalf("persist strategy-colliding candidate: %v", err)
	}
	old, err := links.GetStudioBatchTaskLinkByCandidateKey(ctx, "legacy-key")
	if err != nil || old.ListingKitTaskID != "old-ai-task" {
		t.Fatalf("legacy link = %+v, err=%v; want preserved old AI task", old, err)
	}
	newLinks, err := links.ListStudioBatchTaskLinksByBatchID(ctx, "batch-1")
	if err != nil || len(newLinks) != 2 {
		t.Fatalf("links = %+v, err=%v; want disambiguated SDS link", newLinks, err)
	}
}

func TestFindDurableStudioBatchTaskChecksHistoricalCandidateKey(t *testing.T) {
	t.Parallel()

	links := NewMemStudioBatchTaskLinkRepository()
	ctx := WithTenantID(context.Background(), "tenant-1")
	if err := links.CreateStudioBatchTaskLink(ctx, &StudioBatchTaskLinkRecord{
		ID:                       "legacy-link",
		BatchID:                  "batch-1",
		ItemID:                   "item-1",
		DesignID:                 "design-1",
		SelectionID:              "selection-1",
		CandidateKey:             "legacy-key",
		ListingKitTaskID:         "old-ai-task",
		ImageStrategy:            sheinImageStrategyAIGenerated,
		CompatibilityFingerprint: "selection-fingerprint|product_image_settings=settings-fingerprint",
		Status:                   studioBatchTaskLinkStatusCreated,
	}); err != nil {
		t.Fatalf("create historical link: %v", err)
	}
	s := &taskStudioBatchService{
		batchTaskLinkRepo: links,
		currentTime:       time.Now,
		getTask: func(context.Context, string) (*Task, error) {
			return &Task{ID: "old-ai-task", Request: &GenerateRequest{Options: &GenerateOptions{
				ImageStrategy: sheinImageStrategyAIGenerated,
				SheinStudio:   &SheinStudioOptions{ProductImageURLs: []string{"https://example.com/generated.png"}},
			}}}, nil
		},
	}
	candidate := studioBatchTaskCandidate{
		Design:                          StudioMaterializedDesignRecord{ID: "design-1"},
		Item:                            StudioBatchItemRecord{ID: "item-1"},
		SelectionID:                     "selection-1",
		CandidateKey:                    "new-ai-key",
		HistoricalCandidateKey:          "legacy-key",
		ImageStrategy:                   sheinImageStrategyAIGenerated,
		CompatibilityFingerprint:        "selection-fingerprint",
		ProductImageSettingsFingerprint: "settings-fingerprint",
	}
	got, ok := s.findDurableStudioBatchTask(ctx, candidate)
	if !ok || got.ID != "old-ai-task" {
		t.Fatalf("durable lookup = (%+v, %v), want historical AI task", got, ok)
	}
}

func TestFindDurableStudioBatchTaskMatchUsesHistoricalCandidateIdentity(t *testing.T) {
	t.Parallel()

	links := NewMemStudioBatchTaskLinkRepository()
	ctx := WithTenantID(context.Background(), "tenant-1")
	if err := links.CreateStudioBatchTaskLink(ctx, &StudioBatchTaskLinkRecord{
		ID: "legacy-link-match", BatchID: "batch-1", ItemID: "item-1", DesignID: "design-1",
		SelectionID: "selection-1", CandidateKey: "legacy-key", ClaimToken: "legacy-claim",
		ListingKitTaskID: "old-ai-task", ImageStrategy: sheinImageStrategyAIGenerated,
		CompatibilityFingerprint: "selection-fingerprint|product_image_settings=settings-fingerprint",
		Status:                   studioBatchTaskLinkStatusCreated,
	}); err != nil {
		t.Fatalf("create historical link: %v", err)
	}
	s := &taskStudioBatchService{
		batchTaskLinkRepo: links,
		currentTime:       time.Now,
		getTask: func(context.Context, string) (*Task, error) {
			return &Task{ID: "old-ai-task", Request: &GenerateRequest{Options: &GenerateOptions{
				ImageStrategy: sheinImageStrategyAIGenerated,
				SheinStudio:   &SheinStudioOptions{ProductImageURLs: []string{"https://example.com/generated.png"}},
			}}}, nil
		},
	}
	candidate := studioBatchTaskCandidate{
		Design: StudioMaterializedDesignRecord{ID: "design-1"}, Item: StudioBatchItemRecord{ID: "item-1"},
		SelectionID: "selection-1", CandidateKey: "new-ai-key", HistoricalCandidateKey: "legacy-key",
		ClaimToken: "new-claim", ImageStrategy: sheinImageStrategyAIGenerated,
		CompatibilityFingerprint: "selection-fingerprint", ProductImageSettingsFingerprint: "settings-fingerprint",
	}
	matched, reusedCandidate, ok := s.findDurableStudioBatchTaskMatch(ctx, candidate)
	if !ok || matched.ID != "old-ai-task" {
		t.Fatalf("durable match = (%+v, %v), want historical AI task", matched, ok)
	}
	if reusedCandidate.CandidateKey != "legacy-key" || reusedCandidate.ClaimToken != "legacy-claim" {
		t.Fatalf("reused candidate = %+v, want historical key and claim", reusedCandidate)
	}
}

func TestFindDurableStudioBatchTaskDoesNotWaitOnMismatchedHistoricalCreatingLink(t *testing.T) {
	t.Parallel()

	links := NewMemStudioBatchTaskLinkRepository()
	ctx := WithTenantID(context.Background(), "tenant-1")
	if err := links.CreateStudioBatchTaskLink(ctx, &StudioBatchTaskLinkRecord{
		ID:            "historical-sds-link",
		CandidateKey:  "historical-key",
		ImageStrategy: sheinImageStrategySDSOfficial,
		Status:        studioBatchTaskLinkStatusCreating,
		UpdatedAt:     time.Now().UTC(),
		CreatedAt:     time.Now().UTC(),
	}); err != nil {
		t.Fatalf("create historical creating link: %v", err)
	}
	s := &taskStudioBatchService{batchTaskLinkRepo: links, currentTime: time.Now}
	started := time.Now()
	_, ok := s.findDurableStudioBatchTask(ctx, studioBatchTaskCandidate{
		CandidateKey:           "current-ai-key",
		HistoricalCandidateKey: "historical-key",
		ImageStrategy:          sheinImageStrategyAIGenerated,
	})
	if ok {
		t.Fatal("findDurableStudioBatchTask() = true, want no reuse from mismatched historical link")
	}
	if elapsed := time.Since(started); elapsed > 500*time.Millisecond {
		t.Fatalf("historical strategy mismatch waited %s, want immediate rejection", elapsed)
	}
}

func TestLoadStudioBatchRejectedTasksFromLinksFiltersInactiveStrategy(t *testing.T) {
	t.Parallel()

	links := NewMemStudioBatchTaskLinkRepository()
	ctx := WithTenantID(context.Background(), "tenant-1")
	for _, link := range []*StudioBatchTaskLinkRecord{
		{ID: "sds-rejection", BatchID: "batch-1", DesignID: "design-1", ItemID: "item-1", SelectionID: "sds", CandidateKey: "sds-rejection", ImageStrategy: sheinImageStrategySDSOfficial, Status: studioBatchTaskLinkStatusFailed, ReasonCode: "baseline_missing", Message: "sds"},
		{ID: "ai-rejection", BatchID: "batch-1", DesignID: "design-1", ItemID: "item-1", SelectionID: "ai", CandidateKey: "ai-rejection", ImageStrategy: sheinImageStrategyAIGenerated, Status: studioBatchTaskLinkStatusFailed, ReasonCode: "baseline_missing", Message: "ai"},
		{ID: "legacy-rejection", BatchID: "batch-1", DesignID: "design-1", ItemID: "item-1", SelectionID: "legacy", CandidateKey: "legacy-rejection", Status: studioBatchTaskLinkStatusFailed, ReasonCode: "baseline_missing", Message: "legacy"},
	} {
		if err := links.CreateStudioBatchTaskLink(ctx, link); err != nil {
			t.Fatalf("create rejection link %s: %v", link.ID, err)
		}
	}
	rejected, err := loadStudioBatchRejectedTasksFromLinks(ctx, links, "batch-1", sheinImageStrategyAIGenerated)
	if err != nil {
		t.Fatalf("loadStudioBatchRejectedTasksFromLinks() error = %v", err)
	}
	if len(rejected) != 2 {
		t.Fatalf("rejected = %+v, want active AI and blank-strategy legacy rejections", rejected)
	}
	seen := map[string]bool{}
	for _, task := range rejected {
		seen[task.SelectionID] = true
	}
	if !seen["ai"] || !seen["legacy"] || seen["sds"] {
		t.Fatalf("rejected selections = %#v, want ai+legacy only", seen)
	}
}

func TestCreatedTaskFromDurableLinkRejectsStoredStrategyBeforeTaskLookup(t *testing.T) {
	t.Parallel()

	lookupCalled := false
	s := &taskStudioBatchService{
		currentTime: time.Now,
		getTask: func(context.Context, string) (*Task, error) {
			lookupCalled = true
			return nil, errors.New("transient task lookup failure")
		},
	}
	link := &StudioBatchTaskLinkRecord{
		ListingKitTaskID: "sds-task",
		ImageStrategy:    sheinImageStrategySDSOfficial,
		Status:           studioBatchTaskLinkStatusCreated,
	}
	_, ok := s.createdTaskFromDurableLink(context.Background(), link, studioBatchTaskCandidate{
		ImageStrategy: sheinImageStrategyAIGenerated,
	})
	if ok {
		t.Fatal("createdTaskFromDurableLink() = true, want stored strategy mismatch rejection")
	}
	if lookupCalled {
		t.Fatal("getTask called for a stored strategy mismatch")
	}
	if link.Status != studioBatchTaskLinkStatusCreated {
		t.Fatalf("link status = %q, want unchanged created status", link.Status)
	}
}

func TestMarkStudioBatchReusedTaskClassifiesDurableTaskAsReused(t *testing.T) {
	t.Parallel()

	got := markStudioBatchReusedTask(SheinStudioCreatedTask{
		ID: "task-1",
	})
	if got.ReasonCode != studioBatchReusedTaskReasonCode {
		t.Fatalf("reason code = %q, want %q", got.ReasonCode, studioBatchReusedTaskReasonCode)
	}
}

func TestBuildStudioBatchTaskGenerateRequestIncludesSDSProductTables(t *testing.T) {
	t.Parallel()

	productSize := `[[{"content":"尺码","remark":""},{"content":"肩宽(cm/in)","remark":""}],[{"content":"S","remark":""},{"content":"52cm/20.5in","remark":""}]]`
	packagingSpecification := `[[{"content":"尺码"},{"content":"包装尺寸（cm）"}],[{"content":"S"},{"content":"40.0*30.0*1.0"}]]`
	req := buildStudioBatchTaskGenerateRequest(
		nil,
		&StudioBatchRecord{
			TenantID:     "tenant-1",
			UserID:       "user-1",
			Prompt:       "prompt",
			SheinStoreID: 870,
		},
		studioBatchTaskCandidate{
			Item:        StudioBatchItemRecord{ID: "item-1"},
			SelectionID: "selection-1",
			Selection: SheinStudioGroupedSelection{
				SheinStoreID: "870",
			},
			SelectionSnapshot: SheinStudioSelection{
				ProductName:            "dress",
				VariantID:              1,
				ParentProductID:        2,
				ProductSize:            productSize,
				PackagingSpecification: packagingSpecification,
			},
			Title: "group-a",
		},
		StudioMaterializedDesignRecord{
			ID:               "design-1",
			ImageURL:         "https://example.com/design.png",
			TargetGroupLabel: "group-a",
		},
	)

	if req == nil || req.Options == nil || req.Options.SDS == nil {
		t.Fatal("request SDS options are nil")
	}
	if req.Options.SDS.ProductSize != productSize {
		t.Fatalf("ProductSize = %q, want %q", req.Options.SDS.ProductSize, productSize)
	}
	if req.Options.SDS.PackagingSpecification != packagingSpecification {
		t.Fatalf("PackagingSpecification = %q, want %q", req.Options.SDS.PackagingSpecification, packagingSpecification)
	}
}
