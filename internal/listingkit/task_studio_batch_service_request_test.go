package listingkit

import (
	"context"
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
