package asset_test

import (
	"context"
	"errors"
	"testing"

	"task-processor/internal/product/asset"
	"task-processor/internal/product/asset/assettest"
)

func TestCommitApprovalRejectsEveryMissingIdentityField(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*asset.ApprovalCommit)
	}{
		{name: "tenant", mutate: func(c *asset.ApprovalCommit) { c.TenantID = "" }},
		{name: "product", mutate: func(c *asset.ApprovalCommit) { c.ProductKey = "" }},
		{name: "action", mutate: func(c *asset.ApprovalCommit) { c.ActionID = "" }},
		{name: "asset", mutate: func(c *asset.ApprovalCommit) { c.Assets[0].ID = "" }},
		{name: "run", mutate: func(c *asset.ApprovalCommit) { c.Assets[0].RunID = "" }},
		{name: "revision", mutate: func(c *asset.ApprovalCommit) { c.Assets[0].PlanRevision = 0 }},
		{name: "slot", mutate: func(c *asset.ApprovalCommit) { c.Assets[0].SlotID = "" }},
		{name: "attempt", mutate: func(c *asset.ApprovalCommit) { c.Assets[0].Attempt = 0 }},
		{name: "role", mutate: func(c *asset.ApprovalCommit) { c.Assets[0].Role = "" }},
		{name: "url", mutate: func(c *asset.ApprovalCommit) { c.Assets[0].URL = "" }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := assettest.NewMemoryRepository()
			commit := validCommit("tenant-a", "product-1", "approve-1", "asset-1")
			tt.mutate(&commit)

			_, err := repo.CommitApproval(context.Background(), commit)
			if !errors.Is(err, asset.ErrInvalidApproval) {
				t.Fatalf("CommitApproval() error = %v, want ErrInvalidApproval", err)
			}
		})
	}
}

func TestCommitApprovalRejectsUnsupportedRoleAndInvalidDimensions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*asset.ApprovalCommit)
	}{
		{name: "unsupported role", mutate: func(c *asset.ApprovalCommit) { c.Assets[0].Role = "thumbnail" }},
		{name: "negative width", mutate: func(c *asset.ApprovalCommit) { c.Assets[0].Width = -1 }},
		{name: "negative height", mutate: func(c *asset.ApprovalCommit) { c.Assets[0].Height = -1 }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := assettest.NewMemoryRepository()
			commit := validCommit("tenant-a", "product-1", "approve-1", "asset-1")
			tt.mutate(&commit)

			_, err := repo.CommitApproval(context.Background(), commit)
			if !errors.Is(err, asset.ErrInvalidApproval) {
				t.Fatalf("CommitApproval() error = %v, want ErrInvalidApproval", err)
			}
		})
	}
}

func TestCommitApprovalRequiresAtLeastOneApprovedAsset(t *testing.T) {
	t.Parallel()

	repo := assettest.NewMemoryRepository()
	commit := validCommit("tenant-a", "product-1", "approve-1", "asset-1")
	commit.Assets = nil

	_, err := repo.CommitApproval(context.Background(), commit)
	if !errors.Is(err, asset.ErrInvalidApproval) {
		t.Fatalf("CommitApproval() error = %v, want ErrInvalidApproval", err)
	}
}

func TestCommitApprovalRejectsDuplicateAssetIdentityWithinAction(t *testing.T) {
	t.Parallel()

	repo := assettest.NewMemoryRepository()
	commit := validCommit("tenant-a", "product-1", "approve-1", "asset-1")
	commit.Assets = append(commit.Assets, commit.Assets[0])
	commit.Assets[1].ID = "asset-2"

	_, err := repo.CommitApproval(context.Background(), commit)
	if !errors.Is(err, asset.ErrInvalidApproval) {
		t.Fatalf("CommitApproval() error = %v, want ErrInvalidApproval", err)
	}
}

func TestGetApprovedInventoryRejectsMissingScopeIdentity(t *testing.T) {
	t.Parallel()

	repo := assettest.NewMemoryRepository()
	for _, scope := range []asset.InventoryScope{
		{TenantID: "", ProductKey: "product-1"},
		{TenantID: "tenant-a", ProductKey: ""},
	} {
		_, err := repo.GetApprovedInventory(context.Background(), scope)
		if !errors.Is(err, asset.ErrInvalidInventoryScope) {
			t.Fatalf("GetApprovedInventory(%+v) error = %v, want ErrInvalidInventoryScope", scope, err)
		}
	}
}

func validCommit(tenantID, productKey, actionID, assetID string) asset.ApprovalCommit {
	return asset.ApprovalCommit{
		TenantID:   tenantID,
		ProductKey: productKey,
		ActionID:   actionID,
		Assets: []asset.ApprovedAsset{{
			ID:           assetID,
			RunID:        "run-1",
			PlanRevision: 2,
			SlotID:       "main",
			Attempt:      1,
			Role:         asset.RoleMain,
			URL:          "https://cdn.example/" + assetID + ".png",
			Width:        1200,
			Height:       1200,
			Operations:   []string{"remove_background", "approve"},
		}},
	}
}
