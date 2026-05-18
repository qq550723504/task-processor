package listingkit

import (
	"context"
	"testing"
	"time"

	sheinpub "task-processor/internal/publishing/shein"
)

func TestGetSubmissionEventsIncludesStoreResolutionSnapshot(t *testing.T) {
	t.Parallel()

	repo := NewInMemoryRepositoryForTest()
	now := time.Now()
	task := &Task{
		ID:       "task-submission-events-store-resolution",
		TenantID: "808",
		Request: &GenerateRequest{
			Text:      "demo",
			Platforms: []string{"shein"},
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
			TaskID: "task-submission-events-store-resolution",
			Shein: &SheinPackage{
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

	svc := &service{repo: repo}
	page, err := svc.GetSubmissionEvents(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("GetSubmissionEvents error = %v", err)
	}
	if len(page.Items) != 1 || page.Items[0].StoreResolution == nil {
		t.Fatalf("submission events = %+v, want store resolution", page.Items)
	}
	if page.Items[0].StoreResolution.StoreID != 903 {
		t.Fatalf("store resolution store id = %d, want 903", page.Items[0].StoreResolution.StoreID)
	}
	if page.Items[0].StoreResolution.MatchedProfileID != 17 {
		t.Fatalf("store resolution profile id = %d, want 17", page.Items[0].StoreResolution.MatchedProfileID)
	}
}
