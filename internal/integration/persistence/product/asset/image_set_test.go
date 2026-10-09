package assetpersistence

import (
	"context"
	"github.com/stretchr/testify/require"
	"strings"
	productasset "task-processor/internal/product/asset"
	"testing"
)

func setCommit(action string, head productasset.ImageInventoryHead) productasset.ApprovalCommit {
	commit := sourceCommit(action, 1, "original-b", "original-a")
	commit.TargetPlatform = "product"
	commit.ImageSet = &productasset.ImageSetSelection{Schema: productasset.ImageSetSelectionSchema, Source: productasset.SourceSelectionRequest{ItemID: "source-item", OriginalPublicationID: "original-publication", OriginalSnapshotVersion: 1, EffectiveCatalogVersion: 1, TargetPlatform: "product"}, ExpectedHead: head}
	commit.ImageSet.RequestDigest = strings.Repeat("c", 64)
	for i := range commit.Assets {
		commit.Assets[i].Role = productasset.RoleGallery
		commit.Assets[i].Presentation = &productasset.ImagePresentation{Group: "detail", Order: i + 1}
		commit.Assets[i].Width = 1024
		commit.Assets[i].Height = 1024
	}
	commit.ImageSet.Digest = productasset.ImageSetSelectionDigest(commit)
	return commit
}

func TestImageSetCommitPersistsOrderAndUsesExactHeadCAS(t *testing.T) {
	db := openRepositoryTestDB(t)
	require.NoError(t, AutoMigrate(db))
	repo, err := NewRepository(db)
	require.NoError(t, err)
	ctx := context.Background()
	first := setCommit("first", productasset.ImageInventoryHead{})
	receipt, err := repo.CommitApproval(ctx, first)
	require.NoError(t, err)
	reader := repo.(productasset.ApprovalCommitReader)
	read, err := reader.ReadApprovalCommit(ctx, first.TenantID, first.ActionID)
	require.NoError(t, err)
	require.Equal(t, first, read)
	inventory, err := repo.GetApprovedInventory(ctx, productasset.InventoryScope{TenantID: first.TenantID, ProductKey: first.ProductKey, TargetPlatform: "product", SourceSnapshotVersion: 1})
	require.NoError(t, err)
	require.Equal(t, receipt.AssetIDs, []string{inventory.Assets[0].ID, inventory.Assets[1].ID}, "consumer must receive the explicit selected order")
	hash, err := approvalPayloadHash(first)
	require.NoError(t, err)
	head := productasset.ImageInventoryHead{ActionID: first.ActionID, PayloadHash: hash}
	next := setCommit("next", head)
	_, err = repo.CommitApproval(ctx, next)
	require.NoError(t, err)
	stale := setCommit("stale", head)
	_, err = repo.CommitApproval(ctx, stale)
	require.ErrorIs(t, err, productasset.ErrApprovalConflict)
	_, err = reader.ReadApprovalCommit(ctx, first.TenantID, "stale")
	require.ErrorIs(t, err, productasset.ErrApprovedAssetsNotReady, "conflicting action must leave no receipt")
	replayed, err := repo.CommitApproval(ctx, first)
	require.NoError(t, err)
	require.Equal(t, receipt, replayed, "original successful replay precedes current head checks")
	changed := productasset.CloneApprovalCommit(first)
	changed.Assets[0].Presentation.Order = 2
	changed.Assets[1].Presentation.Order = 1
	changed.ImageSet.Digest = productasset.ImageSetSelectionDigest(changed)
	_, err = repo.CommitApproval(ctx, changed)
	require.ErrorIs(t, err, productasset.ErrApprovalConflict)
	bad := setCommit("bad", productasset.ImageInventoryHead{ActionID: "next", PayloadHash: strings.Repeat("a", 64)})
	_, err = repo.CommitApproval(ctx, bad)
	require.ErrorIs(t, err, productasset.ErrApprovalConflict)
}
