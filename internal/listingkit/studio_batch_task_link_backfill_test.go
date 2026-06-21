package listingkit

import (
	"context"
	"errors"
	"testing"
	"time"

	"gorm.io/gorm"
)

func TestStudioBatchTaskLinkBackfillCreatesDurableLinksFromLegacyCreatedTasks(t *testing.T) {
	ctx := WithTenantID(context.Background(), "tenant-a")
	fixture := newStudioBatchTaskLinkBackfillFixture(t, ctx)

	summary, err := BackfillLegacyStudioBatchTaskLinks(ctx, StudioBatchTaskLinkBackfillConfig{
		SessionRepository: fixture.sessions,
		BatchRepository:   fixture.batches,
		TaskGetter:        fixture.tasks,
		LinkRepository:    fixture.links,
		Limit:             100,
	})
	if err != nil {
		t.Fatalf("BackfillLegacyStudioBatchTaskLinks() error = %v", err)
	}
	if summary.SessionsScanned != 1 || summary.LinksCreated != 1 || summary.LinksAlreadyPresent != 0 {
		t.Fatalf("summary = %+v, want scanned=1 created=1 already=0", summary)
	}
	link := fixture.mustGetLink(t, ctx, fixture.candidateKey)
	if link.ListingKitTaskID != "task-1" || link.BatchID != "batch-1" || link.ItemID != "item-1" || link.DesignID != "design-1" || link.SelectionID != "selection-1" {
		t.Fatalf("link = %+v, want legacy task ownership persisted", link)
	}

	second, err := BackfillLegacyStudioBatchTaskLinks(ctx, StudioBatchTaskLinkBackfillConfig{
		SessionRepository: fixture.sessions,
		BatchRepository:   fixture.batches,
		TaskGetter:        fixture.tasks,
		LinkRepository:    fixture.links,
	})
	if err != nil {
		t.Fatalf("BackfillLegacyStudioBatchTaskLinks(second) error = %v", err)
	}
	if second.LinksCreated != 0 || second.LinksAlreadyPresent != 1 {
		t.Fatalf("second summary = %+v, want idempotent already-present result", second)
	}
}

func TestStudioBatchTaskLinkBackfillRecordsMissingTasksWithoutInvalidDuplicate(t *testing.T) {
	ctx := WithTenantID(context.Background(), "tenant-a")
	fixture := newStudioBatchTaskLinkBackfillFixture(t, ctx)
	fixture.tasks.tasks = map[string]*Task{}

	summary, err := BackfillLegacyStudioBatchTaskLinks(ctx, StudioBatchTaskLinkBackfillConfig{
		SessionRepository: fixture.sessions,
		BatchRepository:   fixture.batches,
		TaskGetter:        fixture.tasks,
		LinkRepository:    fixture.links,
	})
	if err != nil {
		t.Fatalf("BackfillLegacyStudioBatchTaskLinks() error = %v", err)
	}
	if summary.LinksCreated != 0 || len(summary.MissingTasks) != 1 || summary.MissingTasks[0].TaskID != "task-1" {
		t.Fatalf("summary = %+v, want one missing task and no link", summary)
	}
	if _, err := fixture.links.GetStudioBatchTaskLinkByCandidateKey(ctx, fixture.candidateKey); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("GetStudioBatchTaskLinkByCandidateKey() error = %v, want record not found", err)
	}
}

func TestStudioBatchTaskLinkBackfillRecordsExistingLinkAlreadyPresent(t *testing.T) {
	ctx := WithTenantID(context.Background(), "tenant-a")
	fixture := newStudioBatchTaskLinkBackfillFixture(t, ctx)
	mustCreateStudioBatchTaskLinkForTest(t, fixture.links, ctx, &StudioBatchTaskLinkRecord{
		ID:                       buildStudioBatchTaskLinkID(fixture.candidate),
		BatchID:                  "batch-1",
		ItemID:                   "item-1",
		DesignID:                 "design-1",
		SelectionID:              "selection-1",
		CompatibilityFingerprint: fixture.candidate.CompatibilityFingerprint,
		SheinStoreID:             fixture.candidate.SheinStoreID,
		ListingKitTaskID:         "task-1",
		CandidateKey:             fixture.candidateKey,
		Status:                   studioBatchTaskLinkStatusCreated,
		CreatedAt:                time.Now().UTC(),
		UpdatedAt:                time.Now().UTC(),
	})

	summary, err := BackfillLegacyStudioBatchTaskLinks(ctx, StudioBatchTaskLinkBackfillConfig{
		SessionRepository: fixture.sessions,
		BatchRepository:   fixture.batches,
		TaskGetter:        fixture.tasks,
		LinkRepository:    fixture.links,
	})
	if err != nil {
		t.Fatalf("BackfillLegacyStudioBatchTaskLinks() error = %v", err)
	}
	if summary.LinksCreated != 0 || summary.LinksAlreadyPresent != 1 {
		t.Fatalf("summary = %+v, want already-present without duplicate", summary)
	}
	links, err := fixture.links.ListStudioBatchTaskLinksByBatchID(ctx, "batch-1")
	if err != nil {
		t.Fatalf("ListStudioBatchTaskLinksByBatchID() error = %v", err)
	}
	if len(links) != 1 {
		t.Fatalf("len(links) = %d, want 1", len(links))
	}
}

func TestStudioBatchTaskLinkBackfillRejectsCrossTenantTask(t *testing.T) {
	ctx := WithTenantID(context.Background(), "tenant-a")
	fixture := newStudioBatchTaskLinkBackfillFixture(t, ctx)
	fixture.tasks.tasks["task-1"].TenantID = "tenant-b"

	summary, err := BackfillLegacyStudioBatchTaskLinks(ctx, StudioBatchTaskLinkBackfillConfig{
		SessionRepository: fixture.sessions,
		BatchRepository:   fixture.batches,
		TaskGetter:        fixture.tasks,
		LinkRepository:    fixture.links,
	})
	if err != nil {
		t.Fatalf("BackfillLegacyStudioBatchTaskLinks() error = %v", err)
	}
	if summary.LinksCreated != 0 || len(summary.MissingTasks) != 1 {
		t.Fatalf("summary = %+v, want cross-tenant task treated unresolved without link", summary)
	}
	if _, err := fixture.links.GetStudioBatchTaskLinkByCandidateKey(ctx, fixture.candidateKey); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("GetStudioBatchTaskLinkByCandidateKey() error = %v, want record not found", err)
	}
}

func TestStudioBatchTaskLinkBackfillResolvesLegacyStyleIDBySelectionMetadata(t *testing.T) {
	ctx := WithTenantID(context.Background(), "tenant-a")
	fixture := newStudioBatchTaskLinkBackfillFixture(t, ctx)
	fixture.sessions.sessions[0].CreatedTasks = []SheinStudioCreatedTask{{ID: "task-1", DesignID: "design-1"}}

	summary, err := BackfillLegacyStudioBatchTaskLinks(ctx, StudioBatchTaskLinkBackfillConfig{
		SessionRepository: fixture.sessions,
		BatchRepository:   fixture.batches,
		TaskGetter:        fixture.tasks,
		LinkRepository:    fixture.links,
	})
	if err != nil {
		t.Fatalf("BackfillLegacyStudioBatchTaskLinks() error = %v", err)
	}
	if summary.LinksCreated != 1 || len(summary.UnresolvedSelectionOwnership) != 0 {
		t.Fatalf("summary = %+v, want legacy style id resolved from task metadata", summary)
	}
	link := fixture.mustGetLink(t, ctx, fixture.candidateKey)
	if link.SelectionID != "selection-1" || link.ItemID != "item-1" {
		t.Fatalf("link = %+v, want selection/item resolved by task metadata", link)
	}
}

type studioBatchTaskLinkBackfillFixture struct {
	sessions     *studioBatchTaskLinkBackfillSessionRepo
	batches      *MemStudioBatchRepository
	tasks        *studioBatchTaskLinkBackfillTaskRepo
	links        *MemStudioBatchTaskLinkRepository
	candidate    studioBatchTaskCandidate
	candidateKey string
}

func newStudioBatchTaskLinkBackfillFixture(t *testing.T, ctx context.Context) *studioBatchTaskLinkBackfillFixture {
	t.Helper()
	now := time.Now().UTC()
	selection := SheinStudioSelection{
		ParentProductID:  100,
		VariantID:        200,
		PrototypeGroupID: 300,
		LayerID:          "layer-1",
		DesignType:       "front",
		ProductName:      "Product",
		VariantLabel:     "Red / M",
		PrintableWidth:   1000,
		PrintableHeight:  1200,
		TemplateImageURL: "https://example.com/template.png",
		MaskImageURL:     "https://example.com/mask.png",
	}
	batch := &StudioBatchRecord{
		ID:                "batch-1",
		Status:            StudioBatchStatusTasksCreated,
		GroupedImageMode:  "per_product",
		GroupedSelections: []SheinStudioGroupedSelection{{SelectionID: "selection-1", Selection: selection, SheinStoreID: "1001", Eligible: true}},
		SheinStoreID:      1001,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	item := StudioBatchItemRecord{
		ID:             "item-1",
		BatchID:        "batch-1",
		SelectionIDs:   []string{"selection-1"},
		GroupMode:      "per_product",
		Status:         StudioBatchItemStatusReviewReady,
		SelectionCount: 1,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	design := StudioMaterializedDesignRecord{
		ID:           "design-1",
		BatchID:      "batch-1",
		ItemID:       "item-1",
		ImageURL:     "https://example.com/design.png",
		ReviewStatus: StudioMaterializedDesignReviewStatusApproved,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	batchRepo := NewMemStudioBatchRepository()
	if err := batchRepo.CreateStudioBatchGraph(ctx, batch, []StudioBatchItemRecord{item}, nil, []StudioMaterializedDesignRecord{design}); err != nil {
		t.Fatalf("CreateStudioBatchGraph() error = %v", err)
	}
	candidate := studioBatchTaskCandidate{
		Design:                   design,
		Item:                     item,
		Selection:                batch.GroupedSelections[0],
		SelectionSnapshot:        selection,
		SelectionID:              "selection-1",
		CompatibilityFingerprint: buildStudioBatchCompatibilityFingerprint(selection),
		SheinStoreID:             1001,
		StyleID:                  buildStudioBatchTaskScopedStyleID("batch-1", "item-1", "design-1", "selection-1"),
		Title:                    "Red / M",
	}
	candidate.CandidateKey = buildStudioBatchTaskCandidateKey(ctx, batch, candidate)
	task := &Task{
		ID:       "task-1",
		TenantID: "tenant-a",
		Status:   TaskStatusPending,
		Request: &GenerateRequest{
			TenantID:  "tenant-a",
			ImageURLs: []string{"https://example.com/design.png"},
			Options: &GenerateOptions{
				SheinStudio: &SheinStudioOptions{StyleID: buildStudioBatchTaskStyleID("design-1")},
				SDS: &SDSSyncOptions{
					ParentProductID:  selection.ParentProductID,
					VariantID:        selection.VariantID,
					PrototypeGroupID: selection.PrototypeGroupID,
					LayerID:          selection.LayerID,
				},
			},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
	return &studioBatchTaskLinkBackfillFixture{
		sessions: &studioBatchTaskLinkBackfillSessionRepo{sessions: []SheinStudioSession{{
			ID:           "batch-1",
			TenantID:     "tenant-a",
			SavedAsBatch: true,
			CreatedTasks: []SheinStudioCreatedTask{{
				ID:                       "task-1",
				DesignID:                 "design-1",
				ItemID:                   "item-1",
				SelectionID:              "selection-1",
				CompatibilityFingerprint: candidate.CompatibilityFingerprint,
				Status:                   studioBatchCreatedTaskStatus,
			}},
			CreatedAt: now,
			UpdatedAt: now,
		}}},
		batches:      batchRepo,
		tasks:        &studioBatchTaskLinkBackfillTaskRepo{tasks: map[string]*Task{"task-1": task}},
		links:        NewMemStudioBatchTaskLinkRepository(),
		candidate:    candidate,
		candidateKey: candidate.CandidateKey,
	}
}

func (f *studioBatchTaskLinkBackfillFixture) mustGetLink(t *testing.T, ctx context.Context, candidateKey string) *StudioBatchTaskLinkRecord {
	t.Helper()
	link, err := f.links.GetStudioBatchTaskLinkByCandidateKey(ctx, candidateKey)
	if err != nil {
		t.Fatalf("GetStudioBatchTaskLinkByCandidateKey() error = %v", err)
	}
	return link
}

type studioBatchTaskLinkBackfillSessionRepo struct {
	sessions []SheinStudioSession
}

func (r *studioBatchTaskLinkBackfillSessionRepo) ListBatchSessions(context.Context, int) ([]SheinStudioSession, error) {
	return append([]SheinStudioSession(nil), r.sessions...), nil
}

type studioBatchTaskLinkBackfillTaskRepo struct {
	tasks map[string]*Task
}

func (r *studioBatchTaskLinkBackfillTaskRepo) GetTask(_ context.Context, taskID string) (*Task, error) {
	task := r.tasks[taskID]
	if task == nil {
		return nil, ErrTaskNotFound
	}
	cloned := *task
	return &cloned, nil
}
