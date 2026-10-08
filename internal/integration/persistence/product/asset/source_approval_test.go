package assetpersistence

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	productasset "task-processor/internal/product/asset"
)

func sourceCommit(action string, version uint64, images ...string) productasset.ApprovalCommit {
	commit := productasset.ApprovalCommit{TenantID: "org-source", ProductKey: "product-source", TargetPlatform: "shein", SourceSnapshotVersion: version, ActionID: action}
	for _, image := range images {
		imageURL := "https://images.example.org/" + image + ".png"
		commit.Assets = append(commit.Assets, productasset.ApprovedAsset{ID: action + "-" + image, Role: productasset.RoleMain, URL: imageURL, SourceAssetID: image, SourceApproval: &productasset.SourceApprovalProvenance{OriginalPublicationID: "original-publication", OriginalSnapshotVersion: 1, ActorID: "original-actor", MemberID: "original-member", ReferenceHash: productasset.ReferenceHash(image, imageURL)}})
	}
	return commit
}
func TestSourceApprovalPersistsTypedIdentityAndExactInventorySet(t *testing.T) {
	db := openRepositoryTestDB(t)
	require.NoError(t, AutoMigrate(db))
	repository, err := NewRepository(db)
	require.NoError(t, err)
	commit := sourceCommit("first-selection", 2, "first", "second")
	receipt, err := repository.CommitApproval(context.Background(), commit)
	require.NoError(t, err)
	replay, err := repository.CommitApproval(context.Background(), commit)
	require.NoError(t, err)
	require.Equal(t, receipt, replay)
	reader, err := NewBoundedApprovalCommitReader(db, 2<<20)
	require.NoError(t, err)
	original, err := reader.ReadApprovalCommit(context.Background(), commit.TenantID, commit.ActionID)
	require.NoError(t, err)
	require.Equal(t, commit, original)
	original.Assets[0].SourceApproval.ActorID = "caller-mutated"
	reread, err := reader.ReadApprovalCommit(context.Background(), commit.TenantID, commit.ActionID)
	require.NoError(t, err)
	require.Equal(t, "original-actor", reread.Assets[0].SourceApproval.ActorID)
	next := sourceCommit("next-selection", 2, "first")
	agent := productasset.ApprovedAsset{ID: "selected-agent", RunID: "real-run", PlanRevision: 1, SlotID: "gallery", Attempt: 1, Role: productasset.RoleGallery, URL: "https://images.example.org/generated.png", SelectionReceipt: &productasset.SelectionReceipt{ActionID: "agent-receipt", AssetID: "original-generated"}}
	next.Assets = append(next.Assets, agent)
	_, err = repository.CommitApproval(context.Background(), next)
	require.NoError(t, err)
	inventory, err := repository.GetApprovedInventory(context.Background(), productasset.InventoryScope{TenantID: commit.TenantID, ProductKey: commit.ProductKey, TargetPlatform: commit.TargetPlatform, SourceSnapshotVersion: 2})
	require.NoError(t, err)
	require.Len(t, inventory.Assets, 2, "complete selected set replaces the old set")
	for _, a := range inventory.Assets {
		require.NotEqual(t, "first-selection-second", a.ID)
	}
	exactNext, err := reader.ReadApprovalCommit(context.Background(), next.TenantID, next.ActionID)
	require.NoError(t, err)
	require.Equal(t, next, exactNext)
	_, err = repository.GetApprovedInventory(context.Background(), productasset.InventoryScope{TenantID: commit.TenantID, ProductKey: commit.ProductKey, TargetPlatform: commit.TargetPlatform, SourceSnapshotVersion: 3})
	require.ErrorIs(t, err, productasset.ErrApprovedAssetsNotReady)
	changed := productasset.CloneApprovalCommit(commit)
	changed.Assets[0].Role = productasset.RoleGallery
	_, err = repository.CommitApproval(context.Background(), changed)
	require.ErrorIs(t, err, productasset.ErrApprovalConflict)
	var history int64
	require.NoError(t, db.Model(&ApprovedAssetRecord{}).Count(&history).Error)
	require.EqualValues(t, 4, history)
}
