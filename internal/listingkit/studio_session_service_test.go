package listingkit

import (
	"context"
	"slices"
	"testing"

	openaiclient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/listingkit/tenantctx"
)

type studioSessionRepoStub struct {
	sessions map[string]*SheinStudioSession
	designs  map[string][]SheinStudioDesign
}

func newStudioSessionRepoStub() *studioSessionRepoStub {
	return &studioSessionRepoStub{
		sessions: map[string]*SheinStudioSession{},
		designs:  map[string][]SheinStudioDesign{},
	}
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
	}
	return nil
}

func (r *studioSessionRepoStub) ListSessionDesigns(_ context.Context, sessionID string) ([]SheinStudioDesign, error) {
	return slices.Clone(r.designs[sessionID]), nil
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
	cloned.GroupedSelections = append(SheinStudioGroupedSelectionList(nil), session.GroupedSelections...)
	cloned.Selection = session.Selection
	return &cloned
}

func newStudioSessionTestService() *service {
	return &service{
		studioSessionRepo: newStudioSessionRepoStub(),
	}
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

func TestStudioSessionServiceEnsureAndReplaceDesigns(t *testing.T) {
	svc := newStudioSessionTestService()
	ctx := context.Background()

	detail, err := svc.EnsureStudioSession(ctx, &EnsureStudioSessionRequest{
		Selection: testStudioSelection(),
	})
	if err != nil {
		t.Fatalf("ensure session: %v", err)
	}
	if detail.Session == nil || detail.Session.ID == "" {
		t.Fatalf("session = %#v, want created session", detail.Session)
	}

	prompt := "retro cherries"
	status := SheinStudioSessionStatusReviewing
	if _, err := svc.UpdateStudioSession(ctx, detail.Session.ID, &UpdateStudioSessionRequest{
		Status:     &status,
		Prompt:     &prompt,
		StyleCount: ptr("2"),
	}); err != nil {
		t.Fatalf("update session: %v", err)
	}

	replaced, err := svc.ReplaceStudioSessionDesigns(ctx, detail.Session.ID, &ReplaceStudioSessionDesignsRequest{
		Status:            &status,
		ApprovedDesignIDs: []string{"design-1"},
		Designs: []SheinStudioDesign{
			{
				ID:                    "design-1",
				ImageURL:              "https://oss.example.com/design-1.png",
				Prompt:                "retro cherries",
				RevisedPrompt:         "rev-1",
				ImageModel:            "gpt-image-2",
				TransparentBackground: true,
				VariationIntensity:    "light",
			},
			{ID: "design-2", ImageURL: "https://oss.example.com/design-2.png", RevisedPrompt: "rev-2"},
		},
	})
	if err != nil {
		t.Fatalf("replace designs: %v", err)
	}
	if len(replaced.Designs) != 2 {
		t.Fatalf("design count = %d, want 2", len(replaced.Designs))
	}
	if !replaced.Designs[0].Approved || replaced.Designs[1].Approved {
		t.Fatalf("approved flags = %#v, want [true false]", replaced.Designs)
	}
	if replaced.Designs[0].Prompt != "retro cherries" ||
		replaced.Designs[0].ImageModel != "gpt-image-2" ||
		!replaced.Designs[0].TransparentBackground ||
		replaced.Designs[0].VariationIntensity != "light" {
		t.Fatalf("design generation metadata = %#v, want prompt/model/background/variation preserved", replaced.Designs[0])
	}

	loaded, err := svc.GetStudioSession(ctx, detail.Session.ID)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if loaded.Session.Prompt != prompt {
		t.Fatalf("prompt = %q, want %q", loaded.Session.Prompt, prompt)
	}
	if len(loaded.Session.ApprovedDesignIDs) != 1 || loaded.Session.ApprovedDesignIDs[0] != "design-1" {
		t.Fatalf("approved ids = %#v, want [design-1]", loaded.Session.ApprovedDesignIDs)
	}
}

func TestStudioSessionServiceSupportsFailedEmptyResult(t *testing.T) {
	svc := newStudioSessionTestService()
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
	if _, err := svc.ReplaceStudioSessionDesigns(ctx, detail.Session.ID, &ReplaceStudioSessionDesignsRequest{
		Status:            &status,
		ApprovedDesignIDs: []string{},
		Designs:           []SheinStudioDesign{},
	}); err != nil {
		t.Fatalf("replace empty designs: %v", err)
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

func TestStudioSessionServicePersistsGroupedSelections(t *testing.T) {
	svc := newStudioSessionTestService()
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
			BaselineStatus: "ready",
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
	svc := newStudioSessionTestService()
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
				BaselineStatus: "ready",
				SheinStoreID:   "store-9",
				Eligible:       true,
			},
		},
	})
	if err != nil {
		t.Fatalf("upsert batch: %v", err)
	}
	if detail.Session == nil || !detail.Session.SavedAsBatch {
		t.Fatalf("session = %#v, want saved batch session", detail.Session)
	}
	if detail.Session.BatchName != "retro cherries" {
		t.Fatalf("batch name = %q, want retro cherries", detail.Session.BatchName)
	}
	if len(detail.Session.GroupedSelections) != 1 {
		t.Fatalf("grouped selections = %#v, want 1 item", detail.Session.GroupedSelections)
	}

	list, err := svc.ListStudioBatches(ctx, 10)
	if err != nil {
		t.Fatalf("list batches: %v", err)
	}
	if len(list.Items) != 1 {
		t.Fatalf("batch count = %d, want 1", len(list.Items))
	}
	if list.Items[0].ID != detail.Session.ID {
		t.Fatalf("batch id = %q, want %q", list.Items[0].ID, detail.Session.ID)
	}

	loaded, err := svc.GetStudioBatch(ctx, detail.Session.ID)
	if err != nil {
		t.Fatalf("get batch: %v", err)
	}
	if loaded.Session == nil || loaded.Session.ID != detail.Session.ID {
		t.Fatalf("loaded session = %#v, want %q", loaded.Session, detail.Session.ID)
	}
	if len(loaded.Designs) != 1 || loaded.Designs[0].ID != "design-1" {
		t.Fatalf("loaded designs = %#v, want design-1", loaded.Designs)
	}
	if len(loaded.Session.GroupedSelections) != 1 || loaded.Session.GroupedSelections[0].SheinStoreID != "store-9" {
		t.Fatalf("loaded grouped selections = %#v, want store-9", loaded.Session.GroupedSelections)
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

	if err := svc.DeleteStudioBatch(ctx, detail.Session.ID); err != nil {
		t.Fatalf("delete batch: %v", err)
	}

	loaded, err := svc.GetStudioBatch(ctx, detail.Session.ID)
	if err == nil {
		t.Fatalf("get deleted batch = %#v, want error", loaded)
	}
	if err != ErrStudioSessionNotFound {
		t.Fatalf("delete batch error = %v, want ErrStudioSessionNotFound", err)
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
	if detail.Session.BatchName != "批次8" {
		t.Fatalf("batch name = %q, want 批次8", detail.Session.BatchName)
	}
}

func ptr[T any](value T) *T {
	return &value
}
