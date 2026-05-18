package listingkit

import (
	"context"
	"testing"
	"time"

	openaiclient "task-processor/internal/infra/clients/openai"
	sheinpub "task-processor/internal/publishing/shein"
)

func TestGetTaskPreviewIncludesSheinStoreResolution(t *testing.T) {
	t.Parallel()

	repo := NewInMemoryRepositoryForTest()
	now := time.Now()
	task := &Task{
		ID:        "task-preview-store-resolution",
		TenantID:  "505",
		Status:    TaskStatusCompleted,
		CreatedAt: now.Add(-time.Minute),
		UpdatedAt: now,
		Request: &GenerateRequest{
			Text:      "demo",
			Platforms: []string{"shein"},
			Country:   "GB",
		},
		SheinStoreResolutionSnapshot: &SheinStoreResolutionSnapshot{
			StoreID:          903,
			Site:             "GB",
			Strategy:         "country",
			Reason:           "根据任务国家信息命中了对应店铺。",
			MatchedRuleKinds: []string{"country"},
			MatchedProfileID: 17,
			ResolvedAt:       now,
		},
		Result: &ListingKitResult{
			TaskID: "task-preview-store-resolution",
			Shein: &SheinPackage{
				FinalDraft: &sheinpub.FinalDraft{Confirmed: true},
				SubmissionEvents: []sheinpub.SubmissionEvent{{
					ID:        "event-1",
					Action:    "publish",
					Status:    "success",
					StartedAt: now,
				}},
			},
		},
	}
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask error = %v", err)
	}
	if err := repo.MarkCompleted(context.Background(), task.ID, task.Result); err != nil {
		t.Fatalf("MarkCompleted error = %v", err)
	}

	svc := &service{
		repo:                repo,
		storeProfileRepo:    newInMemoryStoreProfileRepository(),
		routingSettingsRepo: newInMemoryStoreRoutingSettingsRepository(),
	}
	ctx := openaiclient.WithIdentity(context.Background(), openaiclient.Identity{TenantID: "505", UserID: "user-e"})

	preview, err := svc.GetTaskPreview(ctx, task.ID, "shein")
	if err != nil {
		t.Fatalf("GetTaskPreview error = %v", err)
	}
	if preview.Shein == nil || preview.Shein.StoreResolution == nil {
		t.Fatalf("preview shein store resolution = %+v", preview.Shein)
	}
	if preview.Shein.StoreResolution.StoreID != 903 {
		t.Fatalf("store resolution store id = %d, want 903", preview.Shein.StoreResolution.StoreID)
	}
	if preview.Shein.StoreResolution.Strategy != "country" {
		t.Fatalf("store resolution strategy = %q, want country", preview.Shein.StoreResolution.Strategy)
	}
	if len(preview.Shein.StoreResolution.MatchedRuleKinds) != 1 || preview.Shein.StoreResolution.MatchedRuleKinds[0] != "country" {
		t.Fatalf("matched rule kinds = %+v, want [country]", preview.Shein.StoreResolution.MatchedRuleKinds)
	}
	if preview.Shein.StoreResolution.MatchedProfileID != 17 {
		t.Fatalf("matched profile id = %d, want 17", preview.Shein.StoreResolution.MatchedProfileID)
	}
	if preview.Shein.StoreResolution.ResolvedAt == "" {
		t.Fatalf("resolved at = %q, want populated snapshot time", preview.Shein.StoreResolution.ResolvedAt)
	}
	if preview.Shein.FinalReview == nil || preview.Shein.FinalReview.StoreID != 903 || preview.Shein.FinalReview.Site != "GB" {
		t.Fatalf("final review store context = %+v", preview.Shein.FinalReview)
	}
	if len(preview.Shein.SubmissionEvents) != 1 || preview.Shein.SubmissionEvents[0].StoreResolution == nil {
		t.Fatalf("submission events = %+v, want store resolution snapshot", preview.Shein.SubmissionEvents)
	}
	if preview.Shein.SubmissionEvents[0].StoreResolution.StoreID != 903 {
		t.Fatalf("submission event store id = %d, want 903", preview.Shein.SubmissionEvents[0].StoreResolution.StoreID)
	}
}
