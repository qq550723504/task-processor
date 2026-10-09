package assetpersistence

import (
	"context"
	"github.com/stretchr/testify/require"
	"strings"
	imageapp "task-processor/internal/app/imageagent"
	"task-processor/internal/marketplace/shein/goods"
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

type manualSetReader struct {
	calls  int
	denied bool
}

func (r *manualSetReader) ReadManualImage(_ context.Context, ref productasset.ManualImageReference) (productasset.SourceImage, error) {
	r.calls++
	if r.denied {
		return productasset.SourceImage{}, productasset.ErrSourceApprovalForbidden
	}
	id := "source-media-" + ref.Hash
	url := "https://images.example.org/manual.png"
	return productasset.SourceImage{ID: id, URL: url, ReferenceHash: productasset.ReferenceHash(id, url), Width: 800, Height: 1000}, nil
}
func TestImageSetManualReplacementReadsExistingMediaOwnerAndRecordsItsOrigin(t *testing.T) {
	sources, input := setSelectionFixture()
	candidates := &setCandidateReader{}
	service, _ := setSelectionService(t, sources, candidates)
	manual := &manualSetReader{}
	service.WithManualImages(manual)
	input.Choices = []productasset.ImageSetChoice{{Kind: "manual", ManualMedia: &productasset.ManualImageReference{Hash: strings.Repeat("a", 64), Bytes: 100}, Presentation: productasset.ImagePresentation{Group: "carousel", Order: 1}}}
	preview, err := service.Preview(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, input.Choices[0].ManualMedia, preview.Assets[0].SourceApproval.ManualMedia)
	require.Zero(t, candidates.calls)
	require.Equal(t, 1, manual.calls)
	manual.denied = true
	_, err = service.Preview(context.Background(), input)
	require.ErrorIs(t, err, productasset.ErrSourceApprovalForbidden)
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
	service, err := productasset.NewImageSetService(sources, repo, repo.(productasset.ImageSetInventoryReader), repo.(productasset.ApprovalCommitReader), candidates, genericMaterialIdentity{})
	require.NoError(t, err)
	return service, repo
}

func TestImageSetExplicitGenericAdoptionRequiresTheExactCurrentGenericHead(t *testing.T) {
	sources, input := setSelectionFixture()
	candidates := &setCandidateReader{}
	service, repo := setSelectionService(t, sources, candidates)
	preview, err := service.Preview(context.Background(), input)
	require.NoError(t, err)
	input.SelectionDigest = preview.Digest
	receipt, err := service.Select(context.Background(), input)
	require.NoError(t, err)
	generic, err := repo.(productasset.ImageSetInventoryReader).ReadImageSetInventory(context.Background(), productasset.InventoryScope{TenantID: "org", ProductKey: "product", TargetPlatform: "product", SourceSnapshotVersion: 1})
	require.NoError(t, err)
	sources.selection.TargetPlatform = "shein"
	service, err = productasset.NewImageSetService(sources, repo, repo.(productasset.ImageSetInventoryReader), repo.(productasset.ApprovalCommitReader), candidates, selectedTargetPositions{t: t})
	require.NoError(t, err)
	input.ActionID = "adopt-1"
	input.Source.TargetPlatform = "shein"
	input.SelectionDigest = ""
	input.Target = &productasset.ImageSetTarget{RecordID: "record", StoreID: "store", Site: "shein-us", ApplicationID: "application", ApplicationMode: "self_operated", CategoryID: 1, ProductTypeID: 2, AttributesDigest: strings.Repeat("a", 64), VariantsDigest: strings.Repeat("b", 64)}
	input.Choices = []productasset.ImageSetChoice{{Kind: "approved", ApprovalActionID: receipt.ActionID, AssetID: receipt.AssetIDs[1], Presentation: productasset.ImagePresentation{Group: "detail", Order: 1}, OfficialPlacement: &productasset.ImageOfficialPlacement{Group: "skc", Type: 5, Sort: 1, Site: "shein-us"}}}
	_, err = service.Preview(context.Background(), input)
	require.ErrorIs(t, err, productasset.ErrApprovalConflict)
	input.Choices[0].GenericHead = &generic.Head
	before := candidates.calls
	preview, err = service.Preview(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, before, candidates.calls, "adoption reads immutable approved materials without generating")
	input.Choices[0].GenericHead = &productasset.ImageInventoryHead{ActionID: "stale", PayloadHash: strings.Repeat("a", 64)}
	_, err = service.Preview(context.Background(), input)
	require.ErrorIs(t, err, productasset.ErrApprovalConflict)
	input.Choices[0].GenericHead = &generic.Head
	input.SelectionDigest = preview.Digest
	_, err = service.Select(context.Background(), input)
	require.NoError(t, err)
}

type selectedTargetPositions struct {
	t    *testing.T
	deny bool
}

func (r selectedTargetPositions) ResolveImageSetTarget(_ context.Context, source productasset.SourceSelection, target *productasset.ImageSetTarget, assets []productasset.ApprovedAsset) (productasset.ImageSetTargetResolution, error) {
	if r.deny {
		return productasset.ImageSetTargetResolution{}, productasset.ErrApprovedAssetsNotReady
	}
	require.NotNil(r.t, assets[0].OfficialPlacement, "the rule owner must see the explicit position independently of the UI group")
	require.Equal(r.t, 5, assets[0].OfficialPlacement.Type)
	positions := make([]*productasset.ImageOfficialPlacement, len(assets))
	for i := range assets {
		positions[i] = assets[i].OfficialPlacement
	}
	return productasset.ImageSetTargetResolution{Target: target, RequirementDigest: strings.Repeat("c", 64), Placements: positions}, nil
}

func TestImageSetSelectionBindsExplicitPlatformPositionsToOriginalRequest(t *testing.T) {
	sources, input := setSelectionFixture()
	sources.selection.TargetPlatform = "shein"
	input.Source.TargetPlatform = "shein"
	input.Choices = input.Choices[:1]
	input.Target = &productasset.ImageSetTarget{RecordID: "record", StoreID: "store", Site: "shein-us", ApplicationID: "application", ApplicationMode: "self_operated", CategoryID: 1, ProductTypeID: 2, AttributesDigest: strings.Repeat("a", 64), VariantsDigest: strings.Repeat("b", 64)}
	input.Choices[0].OfficialPlacement = &productasset.ImageOfficialPlacement{Group: "skc", Type: 5, Sort: 3, Site: "shein-us"}
	_, repo := setSelectionService(t, sources, &setCandidateReader{})
	service, err := productasset.NewImageSetService(sources, repo, repo.(productasset.ImageSetInventoryReader), repo.(productasset.ApprovalCommitReader), &setCandidateReader{}, selectedTargetPositions{t: t})
	require.NoError(t, err)
	preview, err := service.Preview(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, input.Choices[0].OfficialPlacement, preview.Assets[0].OfficialPlacement)
	input.SelectionDigest = preview.Digest
	_, err = service.Select(context.Background(), input)
	require.NoError(t, err)
	input.Choices[0].OfficialPlacement.Type = 1
	_, err = service.Select(context.Background(), input)
	require.ErrorIs(t, err, productasset.ErrApprovalConflict, "original action cannot be replayed with another official position")
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

type genericMaterialIdentity struct{}

func (genericMaterialIdentity) ResolveImageSetTarget(context.Context, productasset.SourceSelection, *productasset.ImageSetTarget, []productasset.ApprovedAsset) (productasset.ImageSetTargetResolution, error) {
	return productasset.ImageSetTargetResolution{}, nil
}

type originalImageProbe struct{ calls int }

func (p *originalImageProbe) Probe(_ context.Context, a productasset.ApprovedAsset, typ int) (goods.OfficialImageObservation, error) {
	p.calls++
	return goods.OfficialImageObservation{AssetID: a.ID, SourceURL: a.URL, Type: typ, Width: 900, Height: 1200, ContentHash: strings.Repeat("a", 64), Bytes: 123, MediaType: "image/png"}, nil
}
func TestGenericSourceSelectionProbesRealDimensionsThroughTheMaterialResolver(t *testing.T) {
	sources, input := setSelectionFixture()
	sources.selection.Images[0].Width, sources.selection.Images[0].Height = 0, 0
	input.Choices = input.Choices[:1]
	candidates := &setCandidateReader{}
	_, repo := setSelectionService(t, sources, candidates)
	probe := &originalImageProbe{}
	service, err := productasset.NewImageSetService(sources, repo, repo.(productasset.ImageSetInventoryReader), repo.(productasset.ApprovalCommitReader), candidates, imageapp.ImageSetMaterialTargetResolver{Images: probe})
	require.NoError(t, err)
	preview, err := service.Preview(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, 900, preview.Assets[0].Width)
	require.Equal(t, 1200, preview.Assets[0].Height)
	input.SelectionDigest = preview.Digest
	receipt, err := service.Select(context.Background(), input)
	require.NoError(t, err)
	inventory, err := repo.(productasset.ImageSetInventoryReader).ReadImageSetInventory(context.Background(), productasset.InventoryScope{TenantID: "org", ProductKey: "product", TargetPlatform: "product", SourceSnapshotVersion: 1})
	require.NoError(t, err)
	require.Equal(t, receipt.AssetIDs[0], inventory.Assets[0].ID)
	require.Equal(t, 900, inventory.Assets[0].Width)
	require.Equal(t, 1200, inventory.Assets[0].Height)
	require.Zero(t, candidates.calls)
	require.Equal(t, 2, probe.calls)
}
