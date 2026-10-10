package assetpersistence

import (
	"context"
	"encoding/json"
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

func TestEmptyImageInventorySerializesAsAnArrayAndRetainsThePreviousHead(t *testing.T) {
	db := openRepositoryTestDB(t)
	require.NoError(t, AutoMigrate(db))
	repo, err := NewRepository(db)
	require.NoError(t, err)
	reader := repo.(productasset.ImageSetInventoryReader)
	commit := setCommit("existing", productasset.ImageInventoryHead{})
	scope := productasset.InventoryScope{TenantID: commit.TenantID, ProductKey: commit.ProductKey, TargetPlatform: "product", SourceSnapshotVersion: 1}
	for _, existing := range []bool{false, true} {
		if existing {
			_, err = repo.CommitApproval(context.Background(), commit)
			require.NoError(t, err)
			scope.SourceSnapshotVersion = 2
		}
		inventory, err := reader.ReadImageSetInventory(context.Background(), scope)
		require.NoError(t, err)
		require.NotNil(t, inventory.Assets)
		encoded, err := json.Marshal(inventory)
		require.NoError(t, err)
		var decoded map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(encoded, &decoded))
		require.JSONEq(t, `[]`, string(decoded["assets"]))
		if existing {
			require.Equal(t, commit.ActionID, inventory.Head.ActionID)
			require.NotEmpty(t, inventory.Head.PayloadHash)
		} else {
			require.Empty(t, inventory.Head)
		}
	}
}

func TestImageSetInventoryReadsRequestedVersionAndRetainsGlobalHeadCAS(t *testing.T) {
	db := openRepositoryTestDB(t)
	require.NoError(t, AutoMigrate(db))
	repo, err := NewRepository(db)
	require.NoError(t, err)
	ctx := context.Background()
	reader := repo.(productasset.ImageSetInventoryReader)
	newer := setCommit("version-two", productasset.ImageInventoryHead{})
	newer.SourceSnapshotVersion = 2
	newer.ImageSet.Source.EffectiveCatalogVersion = 2
	newer.ImageSet.Digest = productasset.ImageSetSelectionDigest(newer)
	_, err = repo.CommitApproval(ctx, newer)
	require.NoError(t, err)
	newerHash, err := approvalPayloadHash(newer)
	require.NoError(t, err)
	newerHead := productasset.ImageInventoryHead{ActionID: newer.ActionID, PayloadHash: newerHash}
	restored := setCommit("restored-version-one", newerHead)
	restoredReceipt, err := repo.CommitApproval(ctx, restored)
	require.NoError(t, err)
	restoredHash, err := approvalPayloadHash(restored)
	require.NoError(t, err)
	globalHead := productasset.ImageInventoryHead{ActionID: restored.ActionID, PayloadHash: restoredHash}
	scope := productasset.InventoryScope{TenantID: newer.TenantID, ProductKey: newer.ProductKey, TargetPlatform: newer.TargetPlatform, SourceSnapshotVersion: 2}
	inventory, err := reader.ReadImageSetInventory(ctx, scope)
	require.NoError(t, err)
	require.Equal(t, newer.Assets, inventory.Assets, "restoring another version must not hide this version's selected images")
	require.Equal(t, globalHead, inventory.Head, "selection CAS still uses the global latest approval")
	encoded, err := json.Marshal(inventory)
	require.NoError(t, err)
	var wire struct {
		ApprovalActionID string `json:"approval_action_id"`
	}
	require.NoError(t, json.Unmarshal(encoded, &wire))
	require.Equal(t, newer.ActionID, wire.ApprovalActionID, "the picker needs the requested version's real approval identity")
	exact, err := repo.GetApprovedInventory(ctx, scope)
	require.NoError(t, err)
	require.Equal(t, exact.Assets, inventory.Assets, "editor and exact-version consumers read the same approved set")
	for _, version := range []uint64{0, 1, 3} {
		scope.SourceSnapshotVersion = version
		read, err := reader.ReadImageSetInventory(ctx, scope)
		require.NoError(t, err)
		require.Equal(t, globalHead, read.Head)
		if version == 3 {
			require.Empty(t, read.ApprovalActionID)
			require.NotNil(t, read.Assets)
			require.Empty(t, read.Assets, "a missing version must not fall back to the global version")
		} else {
			require.Equal(t, restored.ActionID, read.ApprovalActionID)
			require.Equal(t, restored.Assets, read.Assets)
		}
	}
	stale := productasset.CloneApprovalCommit(newer)
	stale.ActionID, stale.ImageSet.ExpectedHead = "stale-version-two", newerHead
	stale.ImageSet.Digest = productasset.ImageSetSelectionDigest(stale)
	_, err = repo.CommitApproval(ctx, stale)
	require.ErrorIs(t, err, productasset.ErrApprovalConflict)
	_, err = repo.(productasset.ApprovalCommitReader).ReadApprovalCommit(ctx, stale.TenantID, stale.ActionID)
	require.ErrorIs(t, err, productasset.ErrApprovedAssetsNotReady)
	replayed, err := repo.CommitApproval(ctx, restored)
	require.NoError(t, err)
	require.Equal(t, restoredReceipt, replayed)
	scope.SourceSnapshotVersion = 2
	read, err := reader.ReadImageSetInventory(ctx, scope)
	require.NoError(t, err)
	require.Equal(t, inventory, read)
}

func TestImageSetInventoryRejectsCorruptVersionHead(t *testing.T) {
	db := openRepositoryTestDB(t)
	require.NoError(t, AutoMigrate(db))
	repo, err := NewRepository(db)
	require.NoError(t, err)
	ctx := context.Background()
	commit := setCommit("version-one", productasset.ImageInventoryHead{})
	_, err = repo.CommitApproval(ctx, commit)
	require.NoError(t, err)
	require.NoError(t, db.Create(&ApprovedInventoryVersionHeadRecord{TenantID: commit.TenantID, ProductKey: commit.ProductKey, TargetPlatform: commit.TargetPlatform, SourceSnapshotVersion: 2, ActionID: commit.ActionID}).Error)
	_, err = repo.(productasset.ImageSetInventoryReader).ReadImageSetInventory(ctx, productasset.InventoryScope{TenantID: commit.TenantID, ProductKey: commit.ProductKey, TargetPlatform: commit.TargetPlatform, SourceSnapshotVersion: 2})
	require.ErrorIs(t, err, productasset.ErrRepositoryStateInvalid, "a mismatched version receipt must not become an empty or cross-version set")
}
