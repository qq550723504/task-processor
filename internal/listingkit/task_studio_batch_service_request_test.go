package listingkit

import (
	"context"
	"errors"
	"testing"
	"time"
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
			ID:           "batch-1",
			SheinStoreID: 869,
			Selection:    SheinStudioSelectionSnapshot{ProductID: 1001, ParentProductID: 2002},
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
		ID:               "legacy-link",
		BatchID:          "batch-1",
		ItemID:           "item-1",
		DesignID:         "design-1",
		SelectionID:      "selection-1",
		CandidateKey:     "legacy-key",
		ListingKitTaskID: "old-ai-task",
		Status:           studioBatchTaskLinkStatusCreated,
	}); err != nil {
		t.Fatalf("create historical link: %v", err)
	}
	s := &taskStudioBatchService{
		batchTaskLinkRepo: links,
		currentTime:       time.Now,
		getTask: func(context.Context, string) (*Task, error) {
			return &Task{ID: "old-ai-task", Request: &GenerateRequest{Options: &GenerateOptions{ImageStrategy: sheinImageStrategyAIGenerated}}}, nil
		},
	}
	candidate := studioBatchTaskCandidate{
		Design:                   StudioMaterializedDesignRecord{ID: "design-1"},
		Item:                     StudioBatchItemRecord{ID: "item-1"},
		SelectionID:              "selection-1",
		CandidateKey:             "new-ai-key",
		HistoricalCandidateKey:   "legacy-key",
		ImageStrategy:            sheinImageStrategyAIGenerated,
		CompatibilityFingerprint: "",
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
