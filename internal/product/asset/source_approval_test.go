package asset_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"task-processor/internal/product/asset"
	"task-processor/internal/product/asset/assettest"
)

type sourceSelectionFixture struct {
	selection asset.SourceSelection
	err       error
}

func (f *sourceSelectionFixture) ReadSourceSelection(context.Context, asset.SourceSelectionRequest) (asset.SourceSelection, error) {
	return f.selection, f.err
}

type approvalReadFixture struct{ commit asset.ApprovalCommit }

func (f approvalReadFixture) ReadApprovalCommit(context.Context, string, string) (asset.ApprovalCommit, error) {
	return f.commit, nil
}

func TestSourceApprovalDoesNotInventAgentIdentityAndBindsEffectiveVersion(t *testing.T) {
	selection := &sourceSelectionFixture{selection: asset.SourceSelection{TenantID: "org-a", ActorID: "actor-a", MemberID: "member-a", ItemID: "item-a", ProductKey: "product-a", OriginalPublicationID: "publication-a", OriginalSnapshotVersion: 1, EffectiveCatalogVersion: 2, TargetPlatform: "shein", Images: []asset.SourceImage{{ID: "image-a", URL: "https://images.example.org/original.jpg", ReferenceHash: asset.ReferenceHash("image-a", "https://images.example.org/original.jpg"), Width: 1200, Height: 1200}}}}
	repository := assettest.NewMemoryRepository()
	service, err := asset.NewSourceApprovalService(selection, repository, approvalReadFixture{})
	require.NoError(t, err)
	input := asset.SourceApprovalCommand{ActionID: "approve-original", Selection: asset.SourceSelectionRequest{ItemID: "item-a", OriginalPublicationID: "publication-a", OriginalSnapshotVersion: 1, EffectiveCatalogVersion: 2, TargetPlatform: "shein"}, Images: []asset.SourceImageChoice{{ID: "image-a", Role: asset.RoleMain}}}
	receipt, err := service.Approve(context.Background(), input)
	require.NoError(t, err)
	inventory, err := repository.GetApprovedInventory(context.Background(), asset.InventoryScope{TenantID: "org-a", ProductKey: "product-a", TargetPlatform: "shein", SourceSnapshotVersion: 2})
	require.NoError(t, err)
	require.Len(t, inventory.Assets, 1)
	approved := inventory.Assets[0]
	require.Equal(t, asset.OriginHumanSource, approved.OriginKind())
	require.Empty(t, approved.RunID)
	require.Zero(t, approved.PlanRevision)
	require.Empty(t, approved.SlotID)
	require.Zero(t, approved.Attempt)
	require.EqualValues(t, 1, approved.SourceApproval.OriginalSnapshotVersion)
	require.Equal(t, "actor-a", approved.SourceApproval.ActorID)
	replay, err := service.Approve(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, receipt, replay)
	input.Images[0].Role = asset.RoleGallery
	_, err = service.Approve(context.Background(), input)
	require.ErrorIs(t, err, asset.ErrApprovalConflict)
	input.ActionID = "changed"
	input.Images[0].ID = "client-replacement"
	_, err = service.Approve(context.Background(), input)
	require.ErrorIs(t, err, asset.ErrInvalidApproval)
	input.Images[0].ID = "image-a"
	input.Selection.EffectiveCatalogVersion = 3
	_, err = service.Approve(context.Background(), input)
	require.ErrorIs(t, err, asset.ErrApprovalConflict)
	input.Selection.EffectiveCatalogVersion = 2
	selection.err = asset.ErrSourceApprovalForbidden
	_, err = service.Approve(context.Background(), input)
	require.ErrorIs(t, err, asset.ErrSourceApprovalForbidden)
}

func TestSourceAndAgentInventorySelectionReplacesFullSet(t *testing.T) {
	repository := assettest.NewMemoryRepository()
	agent := asset.ApprovedAsset{ID: "generated", RunID: "real-run", PlanRevision: 2, SlotID: "main", Attempt: 1, Role: asset.RoleMain, URL: "https://images.example.org/generated.png", SourceAssetID: "image-a"}
	commit := asset.ApprovalCommit{TenantID: "org-a", ProductKey: "product-a", TargetPlatform: "shein", SourceSnapshotVersion: 2, ActionID: "agent-approval", Assets: []asset.ApprovedAsset{agent}}
	_, err := repository.CommitApproval(context.Background(), commit)
	require.NoError(t, err)
	selection := &sourceSelectionFixture{selection: asset.SourceSelection{TenantID: "org-a", ActorID: "actor-a", MemberID: "member-a", ItemID: "item-a", ProductKey: "product-a", OriginalPublicationID: "original", OriginalSnapshotVersion: 1, EffectiveCatalogVersion: 2, TargetPlatform: "shein", Images: []asset.SourceImage{{ID: "image-a", URL: "https://images.example.org/source.png", ReferenceHash: asset.ReferenceHash("image-a", "https://images.example.org/source.png")}}}}
	service, err := asset.NewSourceApprovalService(selection, repository, approvalReadFixture{commit})
	require.NoError(t, err)
	input := asset.SourceApprovalCommand{ActionID: "mixed", Selection: asset.SourceSelectionRequest{ItemID: "item-a", OriginalPublicationID: "original", OriginalSnapshotVersion: 1, EffectiveCatalogVersion: 2, TargetPlatform: "shein"}, Images: []asset.SourceImageChoice{{ID: "image-a", Role: asset.RoleGallery}}, Approved: []asset.ApprovedImageChoice{{ActionID: "agent-approval", AssetID: "generated"}}}
	_, err = service.Approve(context.Background(), input)
	require.NoError(t, err)
	scope := asset.InventoryScope{TenantID: "org-a", ProductKey: "product-a", TargetPlatform: "shein", SourceSnapshotVersion: 2}
	mixed, err := repository.GetApprovedInventory(context.Background(), scope)
	require.NoError(t, err)
	require.Len(t, mixed.Assets, 2)
	require.Equal(t, "real-run", mixed.Assets[1].RunID)
	require.Equal(t, "agent-approval", mixed.Assets[1].SelectionReceipt.ActionID)
	input.ActionID = "source-only"
	input.Approved = nil
	_, err = service.Approve(context.Background(), input)
	require.NoError(t, err)
	sourceOnly, err := repository.GetApprovedInventory(context.Background(), scope)
	require.NoError(t, err)
	require.Len(t, sourceOnly.Assets, 1)
	require.Equal(t, asset.OriginHumanSource, sourceOnly.Assets[0].OriginKind())
	// Unselected generated images remain in history, never silently reappear.
	input.ActionID = "wrong-agent-version"
	input.Approved = []asset.ApprovedImageChoice{{ActionID: "agent-approval", AssetID: "generated"}}
	reader := approvalReadFixture{commit}
	reader.commit.SourceSnapshotVersion = 1
	service, err = asset.NewSourceApprovalService(selection, repository, reader)
	require.NoError(t, err)
	_, err = service.Approve(context.Background(), input)
	require.ErrorIs(t, err, asset.ErrApprovalConflict)
}
