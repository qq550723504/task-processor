package listingkit

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	openaiclient "task-processor/internal/infra/clients/openai"
	studiodomain "task-processor/internal/listing/studio"
	"task-processor/internal/shared/tenantctx"
)

type studioSessionRepoStub struct {
	sessions          map[string]*SheinStudioSession
	designs           map[string][]SheinStudioDesign
	countDesignsCalls int
	listDesignsCalls  int
	timestampSeed     time.Time
	timestampTick     int64
}

func newStudioSessionRepoStub() *studioSessionRepoStub {
	return &studioSessionRepoStub{
		sessions: map[string]*SheinStudioSession{},
		designs:  map[string][]SheinStudioDesign{},
	}
}

func (r *studioSessionRepoStub) nextTimestamp() time.Time {
	if r.timestampSeed.IsZero() {
		r.timestampSeed = time.Now().UTC()
	}
	next := r.timestampSeed.Add(time.Duration(r.timestampTick) * time.Millisecond)
	r.timestampTick++
	return next
}

func (r *studioSessionRepoStub) FindLatestSessionBySelectionKey(_ context.Context, selectionKey string) (*SheinStudioSession, error) {
	for _, session := range r.sessions {
		if session.SelectionKey == selectionKey {
			return cloneSession(session), nil
		}
	}
	return nil, nil
}

func (r *studioSessionRepoStub) CreateSession(ctx context.Context, session *SheinStudioSession) error {
	if session.TenantID == "" {
		session.TenantID = tenantctx.TenantIDFromContext(ctx)
	}
	if session.UserID == "" {
		session.UserID = RequestUserIDFromContext(ctx)
	}
	now := r.nextTimestamp()
	if session.CreatedAt.IsZero() {
		session.CreatedAt = now
	}
	session.UpdatedAt = now
	r.sessions[session.ID] = cloneSession(session)
	return nil
}

func (r *studioSessionRepoStub) GetSession(_ context.Context, sessionID string) (*SheinStudioSession, error) {
	session, ok := r.sessions[sessionID]
	if !ok {
		return nil, nil
	}
	return cloneSession(session), nil
}

func (r *studioSessionRepoStub) UpdateSession(ctx context.Context, session *SheinStudioSession) error {
	if session.TenantID == "" {
		session.TenantID = tenantctx.TenantIDFromContext(ctx)
	}
	if session.UserID == "" {
		session.UserID = RequestUserIDFromContext(ctx)
	}
	session.UpdatedAt = r.nextTimestamp()
	r.sessions[session.ID] = cloneSession(session)
	return nil
}

func (r *studioSessionRepoStub) ReplaceDesigns(_ context.Context, sessionID string, approvedIDs []string, designs []SheinStudioDesign) error {
	approved := make(map[string]struct{}, len(approvedIDs))
	for _, id := range approvedIDs {
		approved[id] = struct{}{}
	}
	next := make([]SheinStudioDesign, 0, len(designs))
	for _, design := range designs {
		_, isApproved := approved[design.ID]
		next = append(next, SheinStudioDesign{
			ID:                    design.ID,
			SessionID:             sessionID,
			ImageURL:              design.ImageURL,
			ProductImageURLs:      append(SheinStudioStringList(nil), design.ProductImageURLs...),
			Prompt:                design.Prompt,
			RevisedPrompt:         design.RevisedPrompt,
			ImageModel:            design.ImageModel,
			TransparentBackground: design.TransparentBackground,
			VariationIntensity:    design.VariationIntensity,
			Role:                  design.Role,
			RoleLabel:             design.RoleLabel,
			ReviewNote:            design.ReviewNote,
			SortOrder:             design.SortOrder,
			Approved:              isApproved,
		})
	}
	r.designs[sessionID] = next
	if session, ok := r.sessions[sessionID]; ok {
		session.ApprovedDesignIDs = slices.Clone(approvedIDs)
		session.UpdatedAt = r.nextTimestamp()
	}
	return nil
}

func (r *studioSessionRepoStub) UpsertDesigns(_ context.Context, sessionID string, approvedIDs []string, designs []SheinStudioDesign) error {
	approved := make(map[string]struct{}, len(approvedIDs))
	for _, id := range approvedIDs {
		approved[id] = struct{}{}
	}
	current := append([]SheinStudioDesign(nil), r.designs[sessionID]...)
	indexByID := make(map[string]int, len(current))
	for idx, design := range current {
		indexByID[design.ID] = idx
	}
	for _, design := range designs {
		_, isApproved := approved[design.ID]
		nextDesign := SheinStudioDesign{
			ID:                    design.ID,
			SessionID:             sessionID,
			ImageURL:              design.ImageURL,
			ProductImageURLs:      append(SheinStudioStringList(nil), design.ProductImageURLs...),
			Prompt:                design.Prompt,
			RevisedPrompt:         design.RevisedPrompt,
			ImageModel:            design.ImageModel,
			TransparentBackground: design.TransparentBackground,
			VariationIntensity:    design.VariationIntensity,
			Role:                  design.Role,
			RoleLabel:             design.RoleLabel,
			ReviewNote:            design.ReviewNote,
			SortOrder:             design.SortOrder,
			Approved:              isApproved,
		}
		if idx, ok := indexByID[design.ID]; ok {
			current[idx] = nextDesign
			continue
		}
		indexByID[design.ID] = len(current)
		current = append(current, nextDesign)
	}
	r.designs[sessionID] = current
	if session, ok := r.sessions[sessionID]; ok {
		session.ApprovedDesignIDs = slices.Clone(approvedIDs)
		session.UpdatedAt = r.nextTimestamp()
	}
	return nil
}

func (r *studioSessionRepoStub) ListSessionDesigns(_ context.Context, sessionID string) ([]SheinStudioDesign, error) {
	r.listDesignsCalls++
	return slices.Clone(r.designs[sessionID]), nil
}

func (r *studioSessionRepoStub) CountSessionDesignsBySessionIDs(_ context.Context, sessionIDs []string) (map[string]int, error) {
	r.countDesignsCalls++
	counts := make(map[string]int, len(sessionIDs))
	for _, sessionID := range sessionIDs {
		counts[sessionID] = len(r.designs[sessionID])
	}
	return counts, nil
}

func (r *studioSessionRepoStub) ListGalleryItems(_ context.Context, _ int) ([]SheinStudioSessionGalleryItem, error) {
	return nil, nil
}

func (r *studioSessionRepoStub) ListBatchSessions(_ context.Context, limit int) ([]SheinStudioSession, error) {
	items := make([]SheinStudioSession, 0, len(r.sessions))
	for _, session := range r.sessions {
		if !session.SavedAsBatch {
			continue
		}
		items = append(items, *cloneSession(session))
	}
	slices.SortFunc(items, func(a, b SheinStudioSession) int {
		return b.UpdatedAt.Compare(a.UpdatedAt)
	})
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

func (r *studioSessionRepoStub) ListTenantBatchNames(ctx context.Context) ([]string, error) {
	tenantID := tenantctx.TenantIDFromContext(ctx)
	names := make([]string, 0, len(r.sessions))
	for _, session := range r.sessions {
		if !session.SavedAsBatch || !tenantctx.MatchesTenant(session.TenantID, tenantID) {
			continue
		}
		names = append(names, session.BatchName)
	}
	return names, nil
}

func (r *studioSessionRepoStub) DeleteSession(_ context.Context, sessionID string) error {
	delete(r.sessions, sessionID)
	delete(r.designs, sessionID)
	return nil
}

func cloneSession(session *SheinStudioSession) *SheinStudioSession {
	if session == nil {
		return nil
	}
	cloned := *session
	cloned.SelectedVariantIDs = append(SheinStudioInt64List(nil), session.SelectedVariantIDs...)
	cloned.ProductImagePrompts = append(SheinStudioProductImagePromptList(nil), session.ProductImagePrompts...)
	cloned.ApprovedDesignIDs = append(SheinStudioStringList(nil), session.ApprovedDesignIDs...)
	cloned.CreatedTaskIDs = append(SheinStudioStringList(nil), session.CreatedTaskIDs...)
	cloned.CreatedTasks = append(SheinStudioCreatedTaskList(nil), session.CreatedTasks...)
	cloned.GenerationJobs = append(SheinStudioGenerationJobList(nil), session.GenerationJobs...)
	cloned.GroupedSelections = append(SheinStudioGroupedSelectionList(nil), session.GroupedSelections...)
	cloned.Selection = session.Selection
	return &cloned
}

func newStudioSessionTestService() *service {
	return &service{studioDeps: studioDependencies{sessionRepo: newStudioSessionRepoStub()}}
}

func newLegacyStudioSessionTestService() *taskStudioSessionService {
	return newTaskStudioSessionService(taskStudioSessionServiceConfig{
		repo: newStudioSessionRepoStub(),
	})
}

func testStudioSelection() *SheinStudioSelection {
	return &SheinStudioSelection{
		ProductID:          124110,
		ParentProductID:    124110,
		VariantID:          124111,
		PrototypeGroupID:   18203,
		LayerID:            "787532312015200256",
		ProductName:        "Beer glass",
		VariantLabel:       "Default",
		PrintableWidth:     1000,
		PrintableHeight:    1000,
		SelectedVariantIDs: []int64{124111, 124112, 124113},
	}
}

func TestStudioSessionServiceUpdateDoesNotReloadDesignsForMetadataOnlyWrites(t *testing.T) {
	svc := newLegacyStudioSessionTestService()
	repo := svc.repo.(*studioSessionRepoStub)
	ctx := context.Background()

	detail, err := svc.EnsureStudioSession(ctx, &EnsureStudioSessionRequest{
		Selection: testStudioSelection(),
	})
	if err != nil {
		t.Fatalf("ensure session: %v", err)
	}

	reviewing := SheinStudioSessionStatusReviewing
	prompt := "retro cherries"
	updated, err := svc.UpdateStudioSession(ctx, detail.Session.ID, &UpdateStudioSessionRequest{
		Status: &reviewing,
		Prompt: &prompt,
	})
	if err != nil {
		t.Fatalf("update session: %v", err)
	}

	if repo.listDesignsCalls != 0 {
		t.Fatalf("list designs calls = %d, want 0 for metadata-only update", repo.listDesignsCalls)
	}
	if updated.Session == nil || updated.Session.Prompt != prompt {
		t.Fatalf("updated session = %#v, want prompt %q", updated.Session, prompt)
	}
	if len(updated.Designs) != 0 {
		t.Fatalf("updated designs = %#v, want empty result without reload", updated.Designs)
	}
}

func TestStudioSessionServiceSupportsFailedEmptyResult(t *testing.T) {
	svc := newLegacyStudioSessionTestService()
	ctx := context.Background()

	detail, err := svc.EnsureStudioSession(ctx, &EnsureStudioSessionRequest{
		Selection: testStudioSelection(),
	})
	if err != nil {
		t.Fatalf("ensure session: %v", err)
	}

	status := SheinStudioSessionStatusFailed
	generationError := "empty_result"
	if _, err := svc.UpdateStudioSession(ctx, detail.Session.ID, &UpdateStudioSessionRequest{
		Status:          &status,
		GenerationError: &generationError,
	}); err != nil {
		t.Fatalf("mark failed: %v", err)
	}

	loaded, err := svc.GetStudioSession(ctx, detail.Session.ID)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if loaded.Session.Status != SheinStudioSessionStatusFailed {
		t.Fatalf("status = %q, want failed", loaded.Session.Status)
	}
	if loaded.Session.GenerationError != generationError {
		t.Fatalf("generation error = %q, want %q", loaded.Session.GenerationError, generationError)
	}
	if len(loaded.Designs) != 0 {
		t.Fatalf("design count = %d, want 0", len(loaded.Designs))
	}
}

func TestStudioSessionServiceTracksMultipleGenerationJobs(t *testing.T) {
	svc := newLegacyStudioSessionTestService()
	ctx := context.Background()

	detail, err := svc.EnsureStudioSession(ctx, &EnsureStudioSessionRequest{
		Selection: testStudioSelection(),
	})
	if err != nil {
		t.Fatalf("ensure session: %v", err)
	}

	status := SheinStudioSessionStatusGenerating
	jobID := "job-primary"
	jobs := []SheinStudioGenerationJob{
		{
			JobID:            "job-primary",
			TargetGroupKey:   "primary",
			TargetGroupLabel: "当前商品",
			Status:           StudioAsyncJobStatusRunning,
		},
		{
			JobID:            "job-group-1",
			TargetGroupKey:   "group-1",
			TargetGroupLabel: "分组商品 1",
			Status:           StudioAsyncJobStatusRunning,
		},
	}
	if _, err := svc.UpdateStudioSession(ctx, detail.Session.ID, &UpdateStudioSessionRequest{
		Status:          &status,
		GenerationJobID: &jobID,
		GenerationJobs:  jobs,
	}); err != nil {
		t.Fatalf("update session: %v", err)
	}

	loaded, err := svc.GetStudioSession(ctx, detail.Session.ID)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if loaded.Session.Status != SheinStudioSessionStatusGenerating {
		t.Fatalf("status = %q, want generating", loaded.Session.Status)
	}
	if len(loaded.Session.GenerationJobs) != 2 {
		t.Fatalf("generation jobs = %#v, want 2 entries", loaded.Session.GenerationJobs)
	}
	if loaded.Session.GenerationJobs[1].TargetGroupKey != "group-1" {
		t.Fatalf("generation jobs = %#v, want grouped target metadata preserved", loaded.Session.GenerationJobs)
	}
}

func TestStudioSessionServiceCreateBatchIgnoresInFlightGenerationState(t *testing.T) {
	svc := newStudioSessionTestService()
	ctx := context.Background()

	detail, err := svc.UpsertStudioBatch(ctx, &UpsertStudioBatchRequest{
		Prompt:       "retro cherries",
		StyleCount:   "2",
		SheinStoreID: "store-1",
		Selection:    testStudioSelection(),
		BatchName:    "retro cherries copy",
		GenerationJobs: []SheinStudioGenerationJob{
			{
				JobID:            "job-primary",
				TargetGroupKey:   "primary",
				TargetGroupLabel: "当前商品",
				Status:           StudioAsyncJobStatusRunning,
			},
		},
	})
	if err != nil {
		t.Fatalf("upsert batch: %v", err)
	}
	if detail.Batch == nil {
		t.Fatalf("batch = nil, want saved batch draft")
	}
	if detail.Batch.Status != SheinStudioSessionStatusSelecting {
		t.Fatalf("status = %q, want selecting when creating a new batch copy", detail.Batch.Status)
	}
	if len(detail.Batch.GenerationJobs) != 0 {
		t.Fatalf("generation jobs = %#v, want empty on create", detail.Batch.GenerationJobs)
	}

	loaded, err := svc.GetStudioBatch(ctx, detail.Batch.ID)
	if err != nil {
		t.Fatalf("get batch: %v", err)
	}
	if len(loaded.Batch.GenerationJobs) != 0 {
		t.Fatalf("loaded generation jobs = %#v, want empty on create", loaded.Batch.GenerationJobs)
	}
}

func TestStudioSessionServicePersistsGroupedSelections(t *testing.T) {
	svc := newLegacyStudioSessionTestService()
	ctx := context.Background()

	detail, err := svc.EnsureStudioSession(ctx, &EnsureStudioSessionRequest{
		Selection: testStudioSelection(),
	})
	if err != nil {
		t.Fatalf("ensure session: %v", err)
	}

	grouped := []SheinStudioGroupedSelection{
		{
			SelectionID: "124110:18203:124200:layer-2:124200",
			Selection: SheinStudioSelection{
				ProductID:        124110,
				ParentProductID:  124110,
				VariantID:        124200,
				PrototypeGroupID: 18203,
				LayerID:          "layer-2",
				ProductName:      "Canvas tote",
				VariantLabel:     "Large / white",
			},
			BaselineStatus: SDSBaselineStatusBaselineCached,
			BaselineReason: "",
			SheinStoreID:   "store-9",
			Eligible:       true,
		},
	}
	if _, err := svc.UpdateStudioSession(ctx, detail.Session.ID, &UpdateStudioSessionRequest{
		GroupedSelections: grouped,
	}); err != nil {
		t.Fatalf("update grouped selections: %v", err)
	}

	loaded, err := svc.GetStudioSession(ctx, detail.Session.ID)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if len(loaded.Session.GroupedSelections) != 1 {
		t.Fatalf("grouped selection count = %d, want 1", len(loaded.Session.GroupedSelections))
	}
	if loaded.Session.GroupedSelections[0].Selection.ProductName != "Canvas tote" {
		t.Fatalf("grouped selection = %#v, want Canvas tote", loaded.Session.GroupedSelections[0])
	}
}

func TestStudioSessionServiceUsesContextUserIDWhenRequestOmitted(t *testing.T) {
	svc := newLegacyStudioSessionTestService()
	ctx := openaiclient.WithIdentity(context.Background(), openaiclient.Identity{TenantID: "tenant-a", UserID: "user-42"})

	detail, err := svc.EnsureStudioSession(ctx, &EnsureStudioSessionRequest{
		Selection: testStudioSelection(),
	})
	if err != nil {
		t.Fatalf("ensure session: %v", err)
	}
	if detail.Session.UserID != "user-42" {
		t.Fatalf("session user id = %q, want user-42", detail.Session.UserID)
	}
}

func TestStudioSessionServiceUpsertsAndListsBatches(t *testing.T) {
	svc := newStudioSessionTestService()
	ctx := context.Background()

	detail, err := svc.UpsertStudioBatch(ctx, &UpsertStudioBatchRequest{
		Prompt:       "retro cherries",
		StyleCount:   "2",
		SheinStoreID: "store-1",
		Selection:    testStudioSelection(),
		Designs: []SheinStudioDesign{
			{ID: "design-1", ImageURL: "https://oss.example.com/design-1.png", Prompt: "retro cherries"},
		},
		ApprovedDesignIDs: []string{"design-1"},
		BatchName:         "retro cherries",
		HotStyleReferenceImageURLs: []string{
			"https://cdn.example.com/hot-style-ref.png",
		},
		HotStyleReferenceBrief:  "embroidered cherry badge",
		HotStyleReferencePrompt: "extract cherry badge print features",
		GroupedSelections: []SheinStudioGroupedSelection{
			{
				SelectionID: "124110:18203:124200:layer-2:124200",
				Selection: SheinStudioSelection{
					ProductID:        124110,
					ParentProductID:  124110,
					VariantID:        124200,
					PrototypeGroupID: 18203,
					LayerID:          "layer-2",
					ProductName:      "Canvas tote",
					VariantLabel:     "Large / white",
				},
				BaselineStatus: SDSBaselineStatusBaselineCached,
				SheinStoreID:   "store-9",
				Eligible:       true,
			},
		},
	})
	if err != nil {
		t.Fatalf("upsert batch: %v", err)
	}
	if detail.Batch == nil || !detail.Batch.SavedAsBatch {
		t.Fatalf("batch = %#v, want saved batch draft", detail.Batch)
	}
	if detail.Batch.BatchName != "retro cherries" {
		t.Fatalf("batch name = %q, want retro cherries", detail.Batch.BatchName)
	}
	if len(detail.Batch.GroupedSelections) != 1 {
		t.Fatalf("grouped selections = %#v, want 1 item", detail.Batch.GroupedSelections)
	}

	list, err := svc.ListStudioBatches(ctx, 10)
	if err != nil {
		t.Fatalf("list batches: %v", err)
	}
	if len(list.Items) != 1 {
		t.Fatalf("batch count = %d, want 1", len(list.Items))
	}
	if list.Items[0].ID != detail.Batch.ID {
		t.Fatalf("batch id = %q, want %q", list.Items[0].ID, detail.Batch.ID)
	}
	if got, want := list.Items[0].Status, string(detail.Batch.Status); got != want {
		t.Fatalf("batch status = %q, want %q", got, want)
	}
	if got := list.Items[0].HotStyleReferenceImageURLs; len(got) != 1 || got[0] != "https://cdn.example.com/hot-style-ref.png" {
		t.Fatalf("batch list hot style reference urls = %#v", got)
	}
	if got, want := list.Items[0].HotStyleReferenceBrief, "embroidered cherry badge"; got != want {
		t.Fatalf("batch list hot style reference brief = %q, want %q", got, want)
	}
	if got, want := list.Items[0].HotStyleReferencePrompt, "extract cherry badge print features"; got != want {
		t.Fatalf("batch list hot style reference prompt = %q, want %q", got, want)
	}
	repo := svc.studioDeps.sessionRepo.(*studioSessionRepoStub)
	storedSession := repo.sessions[detail.Batch.ID]
	if storedSession == nil {
		t.Fatalf("stored session missing for %q", detail.Batch.ID)
	}
	if got, want := list.Items[0].UpdatedAt, storedSession.UpdatedAt.UTC().Format(time.RFC3339Nano); got != want {
		t.Fatalf("batch list updated_at = %q, want %q", got, want)
	}
	listDesignCallsBeforeGetBatch := repo.listDesignsCalls
	if repo.countDesignsCalls != 1 {
		t.Fatalf("count design calls = %d, want 1", repo.countDesignsCalls)
	}
	if listDesignCallsBeforeGetBatch != 1 {
		t.Fatalf("list design calls before get batch = %d, want 1", listDesignCallsBeforeGetBatch)
	}

	loaded, err := svc.GetStudioBatch(ctx, detail.Batch.ID)
	if err != nil {
		t.Fatalf("get batch: %v", err)
	}
	if loaded.Batch == nil || loaded.Batch.ID != detail.Batch.ID {
		t.Fatalf("loaded batch = %#v, want %q", loaded.Batch, detail.Batch.ID)
	}
	if len(loaded.Designs) != 1 || loaded.Designs[0].ID != "design-1" {
		t.Fatalf("loaded designs = %#v, want design-1", loaded.Designs)
	}
	if len(loaded.Batch.GroupedSelections) != 1 || loaded.Batch.GroupedSelections[0].SheinStoreID != "store-9" {
		t.Fatalf("loaded grouped selections = %#v, want store-9", loaded.Batch.GroupedSelections)
	}
	if got := loaded.Batch.HotStyleReferenceImageURLs; len(got) != 1 || got[0] != "https://cdn.example.com/hot-style-ref.png" {
		t.Fatalf("loaded hot style reference urls = %#v", got)
	}
	if got, want := loaded.Batch.HotStyleReferenceBrief, "embroidered cherry badge"; got != want {
		t.Fatalf("loaded hot style reference brief = %q, want %q", got, want)
	}
	if got, want := loaded.Batch.HotStyleReferencePrompt, "extract cherry badge print features"; got != want {
		t.Fatalf("loaded hot style reference prompt = %q, want %q", got, want)
	}
	if repo.listDesignsCalls != listDesignCallsBeforeGetBatch+1 {
		t.Fatalf(
			"list design calls after get batch = %d, want %d",
			repo.listDesignsCalls,
			listDesignCallsBeforeGetBatch+1,
		)
	}
}

func TestStudioSessionServiceDeleteBatchRemovesSession(t *testing.T) {
	svc := newStudioSessionTestService()
	ctx := context.Background()

	detail, err := svc.UpsertStudioBatch(ctx, &UpsertStudioBatchRequest{
		Prompt:       "retro cherries",
		StyleCount:   "2",
		SheinStoreID: "store-1",
		Selection:    testStudioSelection(),
		BatchName:    "retro cherries",
	})
	if err != nil {
		t.Fatalf("upsert batch: %v", err)
	}

	if err := svc.DeleteStudioBatch(ctx, detail.Batch.ID); err != nil {
		t.Fatalf("delete batch: %v", err)
	}

	loaded, err := svc.GetStudioBatch(ctx, detail.Batch.ID)
	if err == nil {
		t.Fatalf("get deleted batch = %#v, want error", loaded)
	}
	if err != ErrStudioSessionNotFound {
		t.Fatalf("delete batch error = %v, want ErrStudioSessionNotFound", err)
	}
}

func TestStudioSessionServiceDeleteBatchIsIdempotent(t *testing.T) {
	svc := newStudioSessionTestService()
	ctx := context.Background()

	detail, err := svc.UpsertStudioBatch(ctx, &UpsertStudioBatchRequest{
		Prompt:       "retro cherries",
		StyleCount:   "2",
		SheinStoreID: "store-1",
		Selection:    testStudioSelection(),
		BatchName:    "retro cherries",
	})
	if err != nil {
		t.Fatalf("upsert batch: %v", err)
	}

	if err := svc.DeleteStudioBatch(ctx, detail.Batch.ID); err != nil {
		t.Fatalf("first delete batch: %v", err)
	}
	if err := svc.DeleteStudioBatch(ctx, detail.Batch.ID); err != nil {
		t.Fatalf("second delete batch: %v", err)
	}
}

func TestStudioSessionServiceAssignsTenantScopedSequentialBatchNames(t *testing.T) {
	restoreOwnerScope := SetOwnerScopeRequiredForTesting(true)
	defer restoreOwnerScope()

	svc := newStudioSessionTestService()
	baseTenantA := WithTenantID(context.Background(), "tenant-a")
	baseTenantB := WithTenantID(context.Background(), "tenant-b")
	ctxTenantAUserA := openaiclient.WithIdentity(baseTenantA, openaiclient.Identity{TenantID: "tenant-a", UserID: "user-a"})
	ctxTenantAUserB := openaiclient.WithIdentity(baseTenantA, openaiclient.Identity{TenantID: "tenant-a", UserID: "user-b"})
	ctxTenantBUserC := openaiclient.WithIdentity(baseTenantB, openaiclient.Identity{TenantID: "tenant-b", UserID: "user-c"})

	createBatch := func(t *testing.T, ctx context.Context, name string) {
		t.Helper()
		if _, err := svc.UpsertStudioBatch(ctx, &UpsertStudioBatchRequest{
			Prompt:     "retro cherries",
			StyleCount: "1",
			Selection:  testStudioSelection(),
			BatchName:  name,
		}); err != nil {
			t.Fatalf("upsert batch %q: %v", name, err)
		}
	}

	createBatch(t, ctxTenantAUserA, "批次1")
	createBatch(t, ctxTenantAUserA, "节日专题")
	createBatch(t, ctxTenantAUserA, "批次7")
	createBatch(t, ctxTenantBUserC, "批次99")

	detail, err := svc.UpsertStudioBatch(ctxTenantAUserB, &UpsertStudioBatchRequest{
		Prompt:     "fresh batch",
		StyleCount: "1",
		Selection:  testStudioSelection(),
	})
	if err != nil {
		t.Fatalf("upsert sequential batch: %v", err)
	}
	if detail.Batch.BatchName != "批次8" {
		t.Fatalf("batch name = %q, want 批次8", detail.Batch.BatchName)
	}
}

func TestStudioSessionServiceAllowsCreatingBatchContainerWithoutPrompt(t *testing.T) {
	svc := newStudioSessionTestService()
	ctx := context.Background()

	detail, err := svc.UpsertStudioBatch(ctx, &UpsertStudioBatchRequest{
		Prompt:     "",
		StyleCount: "1",
		Selection:  testStudioSelection(),
	})
	if err != nil {
		t.Fatalf("upsert batch without prompt: %v", err)
	}
	if detail.Batch == nil || detail.Batch.Prompt != "" {
		t.Fatalf("batch prompt = %#v, want empty prompt persisted", detail.Batch)
	}
	if detail.Batch.BatchName == "" {
		t.Fatalf("batch name = %q, want generated batch name", detail.Batch.BatchName)
	}
	if detail.Batch.Status != SheinStudioSessionStatusSelecting {
		t.Fatalf("status = %q, want selecting for a prompt-less batch container", detail.Batch.Status)
	}
}

func TestStudioSessionServiceListBatchesUsesProjectedBatchStatusWhenGraphExists(t *testing.T) {
	ctx := WithTenantID(context.Background(), "tenant-a")
	sessionRepo := newStudioSessionRepoStub()
	batchRepo := NewMemStudioBatchRepository()
	svc := &service{studioDeps: studioDependencies{sessionRepo: sessionRepo, batchRepo: batchRepo}}

	detail, err := svc.UpsertStudioBatch(ctx, &UpsertStudioBatchRequest{
		Prompt:     "retro cherries",
		StyleCount: "1",
		Selection:  testStudioSelection(),
		BatchName:  "retro cherries",
	})
	if err != nil {
		t.Fatalf("upsert batch: %v", err)
	}

	now := time.Now().UTC()
	if err := batchRepo.CreateStudioBatchGraph(ctx, &StudioBatchRecord{
		ID:        detail.Batch.ID,
		Status:    StudioBatchStatusPartiallyFailed,
		CreatedAt: now,
		UpdatedAt: now,
	}, []StudioBatchItemRecord{
		{
			ID:               "item-1",
			BatchID:          detail.Batch.ID,
			TargetGroupKey:   "size:1200x1200",
			TargetGroupLabel: "1200 x 1200",
			Status:           StudioBatchItemStatusReviewReady,
			SelectionCount:   1,
			CreatedAt:        now,
			UpdatedAt:        now,
		},
		{
			ID:               "item-2",
			BatchID:          detail.Batch.ID,
			TargetGroupKey:   "size:2000x2000",
			TargetGroupLabel: "2000 x 2000",
			Status:           StudioBatchItemStatusGenerating,
			SelectionCount:   1,
			CreatedAt:        now.Add(time.Second),
			UpdatedAt:        now.Add(time.Second),
		},
	}, nil, []StudioMaterializedDesignRecord{
		{
			ID:              "design-1",
			BatchID:         detail.Batch.ID,
			ItemID:          "item-1",
			SourceAttemptID: "attempt-1",
			ImageURL:        "https://oss.example.com/design-1.png",
			ReviewStatus:    StudioMaterializedDesignReviewStatusApproved,
			CreatedAt:       now,
			UpdatedAt:       now,
		},
	}); err != nil {
		t.Fatalf("CreateStudioBatchGraph() error = %v", err)
	}

	list, err := svc.ListStudioBatches(ctx, 10)
	if err != nil {
		t.Fatalf("list batches: %v", err)
	}
	if len(list.Items) != 1 {
		t.Fatalf("batch count = %d, want 1", len(list.Items))
	}
	if got, want := list.Items[0].Status, string(StudioBatchStatusPartiallyMaterialized); got != want {
		t.Fatalf("batch status = %q, want %q", got, want)
	}
	if got, want := list.Items[0].DesignCount, 1; got != want {
		t.Fatalf("batch design count = %d, want %d", got, want)
	}
}

func TestStudioSessionServiceListBatchesUsesGraphSummaryWithoutLoadingFullDetail(t *testing.T) {
	ctx := WithTenantID(context.Background(), "tenant-a")
	batchRepo := NewMemStudioBatchRepository()
	now := time.Now().UTC()
	if err := batchRepo.CreateStudioBatchGraph(ctx, &StudioBatchRecord{
		ID:        "batch-1",
		Status:    StudioBatchStatusPartiallyMaterialized,
		CreatedAt: now,
		UpdatedAt: now,
	}, []StudioBatchItemRecord{
		{
			ID:        "item-1",
			BatchID:   "batch-1",
			Status:    StudioBatchItemStatusReviewReady,
			CreatedAt: now,
			UpdatedAt: now,
		},
	}, nil, []StudioMaterializedDesignRecord{
		{ID: "design-1", BatchID: "batch-1", ItemID: "item-1"},
		{ID: "design-2", BatchID: "batch-1", ItemID: "item-1"},
	}); err != nil {
		t.Fatalf("CreateStudioBatchGraph() error = %v", err)
	}
	svc := newTaskStudioBatchDraftService(taskStudioBatchDraftServiceConfig{
		runner: studiodomain.NewBatchDraftService(studiodomain.BatchDraftServiceConfig[
			SheinStudioSession,
			SheinStudioDesign,
			SheinStudioSessionGalleryItem,
			SheinStudioBatchListItem,
		]{
			Repo: studioBatchDraftDomainRepoStub{
				session: &SheinStudioSession{
					ID:           "batch-1",
					SavedAsBatch: true,
					BatchName:    "Batch 1",
					Status:       SheinStudioSessionStatusSelecting,
				},
			},
			IsSavedBatch: func(session *SheinStudioSession) bool { return session != nil && session.SavedAsBatch },
			SessionID: func(session *SheinStudioSession) string {
				if session == nil {
					return ""
				}
				return session.ID
			},
			MapBatchListItem: mapStudioBatchListItem,
		}),
		batchRepo: batchRepo,
		loadDetail: func(context.Context, string) (*StudioBatchDetail, error) {
			t.Fatal("ListStudioBatches must not load full batch detail")
			return nil, nil
		},
	})

	list, err := svc.ListStudioBatches(ctx, 10)
	if err != nil {
		t.Fatalf("list batches: %v", err)
	}
	if len(list.Items) != 1 {
		t.Fatalf("batch count = %d, want 1", len(list.Items))
	}
	if got, want := list.Items[0].Status, string(StudioBatchStatusReviewReady); got != want {
		t.Fatalf("batch status = %q, want %q", got, want)
	}
	if got, want := list.Items[0].DesignCount, 2; got != want {
		t.Fatalf("batch design count = %d, want %d", got, want)
	}
}

func TestTaskStudioBatchDraftServiceUsesListingStudioRunner(t *testing.T) {
	svc := newTaskStudioBatchDraftService(taskStudioBatchDraftServiceConfig{
		runner: studiodomain.NewBatchDraftService(studiodomain.BatchDraftServiceConfig[
			SheinStudioSession,
			SheinStudioDesign,
			SheinStudioSessionGalleryItem,
			SheinStudioBatchListItem,
		]{
			Repo: studioBatchDraftDomainRepoStub{
				session: &SheinStudioSession{
					ID:           "batch-1",
					SavedAsBatch: true,
					BatchName:    "Batch 1",
				},
				designs: []SheinStudioDesign{{ID: "design-1"}},
			},
			IsSavedBatch: func(session *SheinStudioSession) bool { return session != nil && session.SavedAsBatch },
			SessionID: func(session *SheinStudioSession) string {
				if session == nil {
					return ""
				}
				return session.ID
			},
			MapBatchListItem: mapStudioBatchListItem,
		}),
	})

	detail, err := svc.GetStudioBatch(context.Background(), "batch-1")
	if err != nil {
		t.Fatalf("GetStudioBatch() error = %v", err)
	}
	if detail.Batch == nil || detail.Batch.ID != "batch-1" {
		t.Fatalf("detail.Batch = %+v, want batch-1", detail.Batch)
	}
	if len(detail.Designs) != 1 || detail.Designs[0].ID != "design-1" {
		t.Fatalf("detail.Designs = %+v, want design-1", detail.Designs)
	}
}

func TestTaskStudioBatchDraftServiceMapsListingStudioNotFoundError(t *testing.T) {
	svc := newTaskStudioBatchDraftService(taskStudioBatchDraftServiceConfig{
		runner: studiodomain.NewBatchDraftService(studiodomain.BatchDraftServiceConfig[
			SheinStudioSession,
			SheinStudioDesign,
			SheinStudioSessionGalleryItem,
			SheinStudioBatchListItem,
		]{
			Repo:         studioBatchDraftDomainRepoStub{},
			IsSavedBatch: func(session *SheinStudioSession) bool { return session != nil && session.SavedAsBatch },
			SessionID: func(session *SheinStudioSession) string {
				if session == nil {
					return ""
				}
				return session.ID
			},
			MapBatchListItem: mapStudioBatchListItem,
		}),
	})

	_, err := svc.GetStudioBatch(context.Background(), "missing")
	if !errors.Is(err, ErrStudioSessionNotFound) {
		t.Fatalf("GetStudioBatch() error = %v, want ErrStudioSessionNotFound", err)
	}
}

func TestTaskStudioSessionServiceUsesListingStudioRunner(t *testing.T) {
	selectionKey := buildStudioSelectionKey(testStudioSelection())
	svc := newTaskStudioSessionService(taskStudioSessionServiceConfig{
		runner: studiodomain.NewSessionService(studiodomain.SessionServiceConfig[
			SheinStudioSession,
			SheinStudioSelection,
			SheinStudioDesign,
		]{
			Repo: studioSessionDomainRepoStub{
				sessionByKey: map[string]*SheinStudioSession{
					selectionKey: {
						ID:           "session-1",
						SelectionKey: selectionKey,
					},
				},
				designsBySessionID: map[string][]SheinStudioDesign{
					"session-1": {{ID: "design-1"}},
				},
			},
			ValidateSelection: validateStudioSessionSelection,
			BuildSelectionKey: buildStudioSelectionKey,
			NewSession:        newListingStudioSessionRecord,
			SessionID: func(session *SheinStudioSession) string {
				if session == nil {
					return ""
				}
				return session.ID
			},
			RequestUserID: func(context.Context) string { return "user-1" },
			NewSessionID:  func() string { return "session-new" },
		}),
	})

	detail, err := svc.EnsureStudioSession(context.Background(), &EnsureStudioSessionRequest{
		Selection: testStudioSelection(),
	})
	if err != nil {
		t.Fatalf("EnsureStudioSession() error = %v", err)
	}
	if detail.Session == nil || detail.Session.ID != "session-1" {
		t.Fatalf("detail.Session = %+v, want session-1", detail.Session)
	}
	if len(detail.Designs) != 1 || detail.Designs[0].ID != "design-1" {
		t.Fatalf("detail.Designs = %+v, want design-1", detail.Designs)
	}
}

func TestTaskStudioSessionServiceMapsListingStudioNotFoundError(t *testing.T) {
	svc := newTaskStudioSessionService(taskStudioSessionServiceConfig{
		runner: studiodomain.NewSessionService(studiodomain.SessionServiceConfig[
			SheinStudioSession,
			SheinStudioSelection,
			SheinStudioDesign,
		]{
			Repo:              studioSessionDomainRepoStub{},
			ValidateSelection: validateStudioSessionSelection,
			BuildSelectionKey: buildStudioSelectionKey,
			NewSession:        newListingStudioSessionRecord,
			SessionID: func(session *SheinStudioSession) string {
				if session == nil {
					return ""
				}
				return session.ID
			},
			RequestUserID: func(context.Context) string { return "user-1" },
			NewSessionID:  func() string { return "session-new" },
		}),
	})

	_, err := svc.GetStudioSession(context.Background(), "missing")
	if !errors.Is(err, ErrStudioSessionNotFound) {
		t.Fatalf("GetStudioSession() error = %v, want ErrStudioSessionNotFound", err)
	}
}

func TestTaskStudioSessionServiceSyncUsesListingStudioRunner(t *testing.T) {
	repo := newStudioSessionRepoStub()
	repo.sessions["session-1"] = &SheinStudioSession{ID: "session-1"}
	svc := newTaskStudioSessionService(taskStudioSessionServiceConfig{
		asyncJobRunner: newListingStudioSessionAsyncJobService(repo),
	})

	err := svc.SyncStudioDesignAsyncJob(context.Background(), "session-1", StudioAsyncJobStatusSucceeded, " job-1 ", " done ")
	if err != nil {
		t.Fatalf("SyncStudioDesignAsyncJob() error = %v", err)
	}
	updated := repo.sessions["session-1"]
	if updated.Status != SheinStudioSessionStatusGenerated {
		t.Fatalf("updated.Status = %q, want generated", updated.Status)
	}
	if updated.GenerationJobID != "job-1" || updated.GenerationError != "done" {
		t.Fatalf("updated = %+v, want trimmed async job fields", updated)
	}
}

func TestTaskStudioSessionServiceSyncMapsListingStudioNotFoundError(t *testing.T) {
	svc := newTaskStudioSessionService(taskStudioSessionServiceConfig{
		asyncJobRunner: newListingStudioSessionAsyncJobService(newStudioSessionRepoStub()),
	})

	err := svc.SyncStudioDesignAsyncJob(context.Background(), "missing", StudioAsyncJobStatusRunning, "job-1", "")
	if !errors.Is(err, ErrStudioSessionNotFound) {
		t.Fatalf("SyncStudioDesignAsyncJob() error = %v, want ErrStudioSessionNotFound", err)
	}
}

func TestTaskStudioSessionServiceUsesListingStudioGenerationMetadataRunner(t *testing.T) {
	repo := newStudioSessionRepoStub()
	repo.sessions["session-1"] = &SheinStudioSession{ID: "session-1"}
	svc := newTaskStudioSessionService(taskStudioSessionServiceConfig{
		repo:                     repo,
		generationMetadataRunner: newListingStudioSessionGenerationMetadataService(repo),
	})

	status := SheinStudioSessionStatusGenerating
	jobID := "job-primary"
	jobs := []SheinStudioGenerationJob{
		{JobID: "job-primary", TargetGroupKey: "primary", Status: StudioAsyncJobStatusRunning},
		{JobID: "job-group-1", TargetGroupKey: "group-1", Status: StudioAsyncJobStatusRunning},
	}
	errMessage := "pending"
	updated, err := svc.UpdateStudioSession(context.Background(), "session-1", &UpdateStudioSessionRequest{
		Status:          &status,
		GenerationJobID: &jobID,
		GenerationJobs:  jobs,
		GenerationError: &errMessage,
	})
	if err != nil {
		t.Fatalf("UpdateStudioSession() error = %v", err)
	}
	if updated.Session == nil || updated.Session.GenerationJobID != "job-primary" {
		t.Fatalf("updated.Session = %+v, want generation metadata", updated.Session)
	}
	if len(updated.Session.GenerationJobs) != 2 || updated.Session.GenerationJobs[1].TargetGroupKey != "group-1" {
		t.Fatalf("updated.Session.GenerationJobs = %+v, want preserved grouped jobs", updated.Session.GenerationJobs)
	}
	if repo.listDesignsCalls != 0 {
		t.Fatalf("list designs calls = %d, want 0 for generation metadata runner path", repo.listDesignsCalls)
	}
}

func TestTaskStudioSessionServiceUsesListingStudioReviewTaskMetadataRunner(t *testing.T) {
	repo := newStudioSessionRepoStub()
	repo.sessions["session-1"] = &SheinStudioSession{ID: "session-1"}
	svc := newTaskStudioSessionService(taskStudioSessionServiceConfig{
		repo:                     repo,
		reviewTaskMetadataRunner: newListingStudioSessionReviewTaskMetadataService(repo),
	})

	updated, err := svc.UpdateStudioSession(context.Background(), "session-1", &UpdateStudioSessionRequest{
		ApprovedDesignIDs: []string{"design-1"},
		CreatedTasks: []SheinStudioCreatedTask{
			{ID: "task-1", DesignID: "design-1"},
		},
	})
	if err != nil {
		t.Fatalf("UpdateStudioSession() error = %v", err)
	}
	if updated.Session == nil || len(updated.Session.ApprovedDesignIDs) != 1 || updated.Session.ApprovedDesignIDs[0] != "design-1" {
		t.Fatalf("updated.Session = %+v, want approved design metadata", updated.Session)
	}
	if len(updated.Session.CreatedTasks) != 1 || updated.Session.CreatedTasks[0].ID != "task-1" {
		t.Fatalf("updated.Session.CreatedTasks = %+v, want created task metadata", updated.Session.CreatedTasks)
	}
	if len(updated.Session.CreatedTaskIDs) != 1 || updated.Session.CreatedTaskIDs[0] != "task-1" {
		t.Fatalf("updated.Session.CreatedTaskIDs = %+v, want derived task ids", updated.Session.CreatedTaskIDs)
	}
	if repo.listDesignsCalls != 0 {
		t.Fatalf("list designs calls = %d, want 0 for review/task metadata runner path", repo.listDesignsCalls)
	}
}

func TestStudioSessionServiceRejectsStaleUpdateStudioSessionWrites(t *testing.T) {
	svc := newLegacyStudioSessionTestService()
	ctx := context.Background()

	detail, err := svc.EnsureStudioSession(ctx, &EnsureStudioSessionRequest{
		Selection: testStudioSelection(),
	})
	if err != nil {
		t.Fatalf("ensure session: %v", err)
	}

	firstSeenAt := detail.Session.UpdatedAt.Format(time.RFC3339Nano)
	prompt1 := "first writer"
	if _, err := svc.UpdateStudioSession(ctx, detail.Session.ID, &UpdateStudioSessionRequest{
		Prompt:            &prompt1,
		ExpectedUpdatedAt: &firstSeenAt,
	}); err != nil {
		t.Fatalf("first update: %v", err)
	}

	prompt2 := "stale writer"
	_, err = svc.UpdateStudioSession(ctx, detail.Session.ID, &UpdateStudioSessionRequest{
		Prompt:            &prompt2,
		ExpectedUpdatedAt: &firstSeenAt,
	})
	if err == nil {
		t.Fatal("stale update error = nil, want conflict")
	}
	if !errors.Is(err, ErrStudioSessionConflict) {
		t.Fatalf("stale update error = %v, want ErrStudioSessionConflict", err)
	}
}

func ptr[T any](value T) *T {
	return &value
}

type studioBatchDraftDomainRepoStub struct {
	session *SheinStudioSession
	designs []SheinStudioDesign
}

func (s studioBatchDraftDomainRepoStub) GetSession(context.Context, string) (*SheinStudioSession, error) {
	if s.session == nil {
		return nil, nil
	}
	cloned := cloneSession(s.session)
	return cloned, nil
}

func (studioBatchDraftDomainRepoStub) DeleteSession(context.Context, string) error {
	return nil
}

func (s studioBatchDraftDomainRepoStub) ListSessionDesigns(context.Context, string) ([]SheinStudioDesign, error) {
	return slices.Clone(s.designs), nil
}

func (s studioBatchDraftDomainRepoStub) CountSessionDesignsBySessionIDs(context.Context, []string) (map[string]int, error) {
	return map[string]int{"batch-1": len(s.designs)}, nil
}

func (studioBatchDraftDomainRepoStub) ListGalleryItems(context.Context, int) ([]SheinStudioSessionGalleryItem, error) {
	return nil, nil
}

func (s studioBatchDraftDomainRepoStub) ListBatchSessions(context.Context, int) ([]SheinStudioSession, error) {
	if s.session == nil {
		return nil, nil
	}
	return []SheinStudioSession{*cloneSession(s.session)}, nil
}

type studioSessionDomainRepoStub struct {
	sessionByKey       map[string]*SheinStudioSession
	sessionByID        map[string]*SheinStudioSession
	designsBySessionID map[string][]SheinStudioDesign
}

func (s studioSessionDomainRepoStub) FindLatestSessionBySelectionKey(_ context.Context, selectionKey string) (*SheinStudioSession, error) {
	session := s.sessionByKey[selectionKey]
	if session == nil {
		return nil, nil
	}
	return cloneSession(session), nil
}

func (studioSessionDomainRepoStub) CreateSession(context.Context, *SheinStudioSession) error {
	return nil
}

func (s studioSessionDomainRepoStub) GetSession(_ context.Context, sessionID string) (*SheinStudioSession, error) {
	session := s.sessionByID[sessionID]
	if session == nil {
		return nil, nil
	}
	return cloneSession(session), nil
}

func (s studioSessionDomainRepoStub) ListSessionDesigns(_ context.Context, sessionID string) ([]SheinStudioDesign, error) {
	return slices.Clone(s.designsBySessionID[sessionID]), nil
}
