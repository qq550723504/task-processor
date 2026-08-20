package listingkit

import (
	"context"
	"errors"
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

func TestTaskStudioBatchServiceAttachesPerColorProductImages(t *testing.T) {
	var calls []string
	var firstReferences []string
	service := &taskStudioBatchService{
		generateProductImages: func(_ context.Context, req *StudioProductImageRequest) (*StudioProductImageResponse, error) {
			calls = append(calls, req.StyleName)
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
		&SheinStudioSession{Prompt: "retro"},
		&StudioBatchRecord{ID: "batch-1"},
		candidate,
		StudioMaterializedDesignRecord{ID: "design-1", ImageURL: "https://example.com/design.png"},
	)
	if err := service.attachStudioBatchProductImages(context.Background(), request, &SheinStudioSession{Prompt: "retro"}, &StudioBatchRecord{ID: "batch-1"}, candidate, StudioMaterializedDesignRecord{ID: "design-1", ImageURL: "https://example.com/design.png"}); err != nil {
		t.Fatalf("attachStudioBatchProductImages() error = %v", err)
	}
	if len(calls) != 2 {
		t.Fatalf("generator calls = %d (%v), want one per color representative", len(calls), calls)
	}
	if len(firstReferences) == 0 || firstReferences[0] != "https://example.com/red.png" {
		t.Fatalf("first color references = %v, want first representative references", firstReferences)
	}
	if got := len(request.Options.SheinStudio.VariantProductImages); got != 2 {
		t.Fatalf("variant product image sets = %d, want 2", got)
	}
	if got := request.Options.SheinStudio.VariantProductImages[1].Color; got != "Blue" {
		t.Fatalf("second variant color = %q, want Blue", got)
	}
}

func TestTaskStudioBatchServiceProductImageRequestLoadsSDSCategoryPath(t *testing.T) {
	var gotCategoryPath []string
	service := &taskStudioBatchService{
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
	)

	if got != sheinImageStrategyAIGenerated {
		t.Fatalf("strategy = %q, want %q", got, sheinImageStrategyAIGenerated)
	}
}

func TestStudioBatchTaskImageStrategyMapsLegacySessionHybridToSDS(t *testing.T) {
	t.Parallel()

	got := resolveStudioBatchTaskImageStrategy(
		&CreateStudioBatchTasksRequest{},
		&SheinStudioSession{ImageStrategy: sheinImageStrategyHybrid},
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
		ProductImageCount:   "6",
		ProductImagePrompt:  "keep the print centered",
		ProductImagePrompts: SheinStudioProductImagePromptList{{Role: "detail", Prompt: "show stitching"}},
	}, time.Now())
	if batch.ProductImageCount != "6" || batch.ProductImagePrompt != "keep the print centered" {
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
