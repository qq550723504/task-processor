package listingkit

import (
	"context"
	"slices"
	"testing"
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

func (r *studioSessionRepoStub) CreateSession(_ context.Context, session *SheinStudioSession) error {
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

func (r *studioSessionRepoStub) UpdateSession(_ context.Context, session *SheinStudioSession) error {
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

func ptr[T any](value T) *T {
	return &value
}
