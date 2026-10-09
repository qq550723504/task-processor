package assetpersistence

import (
	"context"
	"github.com/stretchr/testify/require"
	"strings"
	productasset "task-processor/internal/product/asset"
	"testing"
)

type setSourceReader struct {
	selection productasset.SourceSelection
	calls     int
}

func (r *setSourceReader) ReadSourceSelection(context.Context, productasset.SourceSelectionRequest) (productasset.SourceSelection, error) {
	r.calls++
	return r.selection, nil
}

type setCandidateReader struct {
	calls           int
	deny            error
	differentResult bool
}

func (r *setCandidateReader) ReadImageSetCandidate(_ context.Context, source productasset.SourceSelection, choice productasset.ImageSetChoice) (productasset.ImageSetCandidate, error) {
	r.calls++
	if r.deny != nil {
		return productasset.ImageSetCandidate{}, r.deny
	}
	hash := strings.Repeat("a", 64)
	digest := hash
	if r.differentResult && choice.SlotID == "second" {
		digest = strings.Repeat("b", 64)
	}
	return productasset.ImageSetCandidate{Asset: productasset.ApprovedAsset{ID: choice.AssetID, RunID: choice.RunID, PlanRevision: choice.PlanRevision, SlotID: choice.SlotID, Attempt: choice.Attempt, Role: productasset.RoleGallery, URL: "https://images.example.org/" + choice.AssetID + ".png", SourceAssetID: "source-1", Width: 1024, Height: 1024, Operations: []string{"render_source_edit"}, GenerationEvidence: &productasset.GenerationEvidence{IntentID: hash, Fingerprint: hash, SettlementProofDigest: hash, ArtifactHash: hash}}, Result: productasset.ImageSetResultBinding{RunID: choice.RunID, PlanRevision: choice.PlanRevision, ResultDigest: digest}}, nil
}

func setSelectionFixture() (*setSourceReader, productasset.ImageSetCommand) {
	imageURL := "https://images.example.org/source.png"
	source := &setSourceReader{selection: productasset.SourceSelection{TenantID: "org", ActorID: "actor", MemberID: "member", ItemID: "item", ProductKey: "product", OriginalPublicationID: "original", TargetPlatform: "product", OriginalSnapshotVersion: 1, EffectiveCatalogVersion: 1, Images: []productasset.SourceImage{{ID: "source-1", URL: imageURL, ReferenceHash: productasset.ReferenceHash("source-1", imageURL), Width: 1024, Height: 1024}}}}
	input := productasset.ImageSetCommand{ActionID: "select-1", Source: productasset.SourceSelectionRequest{ItemID: "item", OriginalPublicationID: "original", OriginalSnapshotVersion: 1, EffectiveCatalogVersion: 1, TargetPlatform: "product"}, Choices: []productasset.ImageSetChoice{{Kind: "source", SourceID: "source-1", Presentation: productasset.ImagePresentation{Group: "carousel", Order: 1}}, {Kind: "generated", AssetID: "candidate-1", RunID: "run", PlanRevision: 1, SlotID: "first", Attempt: 1, ResultDigest: strings.Repeat("a", 64), Presentation: productasset.ImagePresentation{Group: "detail", Order: 1}}}}
	return source, input
}

func setSelectionService(t *testing.T, sources *setSourceReader, candidates *setCandidateReader) (*productasset.ImageSetService, productasset.Repository) {
	t.Helper()
	db := openRepositoryTestDB(t)
	require.NoError(t, AutoMigrate(db))
	repo, err := NewRepository(db)
	require.NoError(t, err)
	service, err := productasset.NewImageSetService(sources, repo, repo.(productasset.ImageSetInventoryReader), repo.(productasset.ApprovalCommitReader), candidates, nil)
	require.NoError(t, err)
	return service, repo
}

func TestImageSetSelectionResolvesSourcesAndCandidatesThenReplaysOriginalReceipt(t *testing.T) {
	sources, input := setSelectionFixture()
	candidates := &setCandidateReader{}
	service, repo := setSelectionService(t, sources, candidates)
	ctx := context.Background()
	preview, err := service.Preview(ctx, input)
	require.NoError(t, err)
	require.Len(t, preview.Assets, 2)
	require.NotEqual(t, "candidate-1", preview.Assets[1].ID)
	require.Equal(t, "run", preview.Assets[1].RunID)
	input.SelectionDigest = preview.Digest
	receipt, err := service.Select(ctx, input)
	require.NoError(t, err)
	require.Len(t, receipt.AssetIDs, 2)
	inventory, err := repo.(productasset.ImageSetInventoryReader).ReadImageSetInventory(ctx, productasset.InventoryScope{TenantID: "org", ProductKey: "product", TargetPlatform: "product", SourceSnapshotVersion: 1})
	require.NoError(t, err)
	next := input
	next.ActionID = "select-2"
	next.ExpectedHead = inventory.Head
	next.SelectionDigest = ""
	next.Choices = []productasset.ImageSetChoice{{Kind: "approved", ApprovalActionID: input.ActionID, AssetID: receipt.AssetIDs[1], Presentation: productasset.ImagePresentation{Group: "detail", Order: 1}}}
	preview, err = service.Preview(ctx, next)
	require.NoError(t, err)
	require.NotEqual(t, receipt.AssetIDs[1], preview.Assets[0].ID)
	require.Equal(t, receipt.AssetIDs[1], preview.Assets[0].SelectionReceipt.AssetID)
	require.Equal(t, "run", preview.Assets[0].RunID)
	next.SelectionDigest = preview.Digest
	_, err = service.Select(ctx, next)
	require.NoError(t, err)
	calls := candidates.calls
	candidates.deny = productasset.ErrApprovedAssetsNotReady
	replay, err := service.Select(ctx, input)
	require.NoError(t, err)
	require.Equal(t, receipt, replay)
	require.Equal(t, calls, candidates.calls, "receipt replay must not re-evaluate mutable candidates or current head")
	changed := input
	changed.Choices = append([]productasset.ImageSetChoice(nil), input.Choices...)
	changed.Choices[0].Presentation.Group = "detail"
	_, err = service.Select(ctx, changed)
	require.ErrorIs(t, err, productasset.ErrApprovalConflict)
}

func TestImageSetSelectionRejectsUnknownCandidateAndConflictingResultDigests(t *testing.T) {
	for _, mode := range []string{"unknown", "mixed_digests", "duplicate", "foreign_source"} {
		t.Run(mode, func(t *testing.T) {
			sources, input := setSelectionFixture()
			candidates := &setCandidateReader{}
			switch mode {
			case "unknown":
				candidates.deny = productasset.ErrApprovedAssetsNotReady
			case "mixed_digests":
				second := input.Choices[1]
				second.AssetID = "candidate-2"
				second.SlotID = "second"
				second.ResultDigest = strings.Repeat("b", 64)
				second.Presentation.Order = 2
				input.Choices = append(input.Choices, second)
				candidates.differentResult = true
			case "duplicate":
				second := input.Choices[1]
				second.Presentation.Order = 2
				input.Choices = append(input.Choices, second)
			case "foreign_source":
				input.Source.OriginalPublicationID = "foreign"
			}
			service, repo := setSelectionService(t, sources, candidates)
			_, err := service.Preview(context.Background(), input)
			require.Error(t, err)
			_, err = repo.(productasset.ApprovalCommitReader).ReadApprovalCommit(context.Background(), "org", input.ActionID)
			require.ErrorIs(t, err, productasset.ErrApprovedAssetsNotReady)
		})
	}
}
