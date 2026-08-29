package imageagentacceptance

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"task-processor/internal/asset"
	"task-processor/internal/authidentity"
	"task-processor/internal/authruntime/zitadel"
	"task-processor/internal/imageagent"
	"task-processor/internal/listingkit"
	"task-processor/internal/listingkit/core"
)

const (
	seedNamespace        = "image-agent-acceptance:v1"
	seedSourceAssetID    = "image-agent-acceptance-source"
	seedStyleAssetID     = "image-agent-acceptance-style"
	seedTargetPlatform   = "shein"
	seedWorkspaceBaseURL = "/listing-kits/"
)

type SeedRequest struct {
	Runtime   RuntimeConfig
	Token     string
	SourceURL string
	StyleURL  string
}

type SeedResult struct {
	TaskID       string
	TenantID     string
	UserID       string
	WorkspaceURL string
}

// Seed creates the one deterministic, owned acceptance task. The caller must
// invoke the supplied EnvironmentGuard with the loaded RuntimeConfig before
// calling Seed; the fixed Seed signature intentionally carries no DSN.
func Seed(ctx context.Context, guard EnvironmentGuard, verifier zitadel.Verifier, repo listingkit.Repository, request SeedRequest) (SeedResult, error) {
	if guard == nil {
		return SeedResult{}, errors.New("acceptance environment guard is required")
	}
	if verifier == nil {
		return SeedResult{}, errors.New("ZITADEL verifier is required")
	}
	if repo == nil {
		return SeedResult{}, errors.New("ListingKit repository is required")
	}
	if strings.TrimSpace(request.Token) == "" {
		return SeedResult{}, errors.New("acceptance bearer token is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	db, err := guard.Verify(ctx, request.Runtime)
	if err != nil {
		return SeedResult{}, fmt.Errorf("verify acceptance environment: %w", err)
	}
	if db != nil {
		defer closeDatabase(db)
	}

	identity, err := verifier.Verify(ctx, request.Token)
	if err != nil {
		return SeedResult{}, fmt.Errorf("verify acceptance bearer token: %w", err)
	}
	identity.TenantID = strings.TrimSpace(identity.TenantID)
	identity.UserID = strings.TrimSpace(identity.UserID)
	if identity.TenantID == "" || identity.UserID == "" {
		return SeedResult{}, errors.New("verified acceptance identity requires tenant and user")
	}
	if !hasSeedRole(identity.Roles) {
		return SeedResult{}, errors.New("verified acceptance identity lacks a required ListingKit role")
	}

	sourceURL, err := imageagent.ValidateSafeImageURL(request.SourceURL)
	if err != nil {
		return SeedResult{}, fmt.Errorf("validate acceptance source URL: %w", err)
	}
	styleURL := ""
	if strings.TrimSpace(request.StyleURL) != "" {
		styleURL, err = imageagent.ValidateSafeImageURL(request.StyleURL)
		if err != nil {
			return SeedResult{}, fmt.Errorf("validate acceptance style URL: %w", err)
		}
	}

	taskID := seedTaskID(identity.TenantID, identity.UserID)
	identityContext := authidentity.WithAuthenticatedIdentity(ctx, identity)
	existing, err := repo.GetTask(identityContext, taskID)
	if err == nil {
		if !equivalentSeedTask(existing, identity, sourceURL, styleURL) {
			return SeedResult{}, fmt.Errorf("acceptance task %q refuses to overwrite an existing task with changed owner, target, or URL", taskID)
		}
		return seedResult(existing), nil
	}
	if !errors.Is(err, core.ErrTaskNotFound) {
		return SeedResult{}, fmt.Errorf("read acceptance task: %w", err)
	}
	// The normal read is owner-scoped. A not-found result can therefore also
	// mean that a conflicting record already occupies the deterministic ID.
	// Check the repository's unscoped read path before CreateTask so a seed can
	// never overwrite that record through an idempotent store implementation.
	if conflicting, conflictErr := repo.GetTask(context.Background(), taskID); conflictErr == nil {
		return SeedResult{}, fmt.Errorf("acceptance task %q refuses to overwrite an existing task owned by %q/%q", taskID, strings.TrimSpace(conflicting.TenantID), strings.TrimSpace(conflicting.UserID))
	} else if !errors.Is(conflictErr, core.ErrTaskNotFound) {
		return SeedResult{}, fmt.Errorf("check existing acceptance task: %w", conflictErr)
	}

	task := &listingkit.Task{
		ID:       taskID,
		TenantID: identity.TenantID,
		UserID:   identity.UserID,
		Status:   core.TaskStatusPending,
		Result: &listingkit.ListingKitResult{
			TaskID: taskID,
			AssetBundlesByTarget: map[string]*asset.Bundle{
				seedTargetPlatform: {Assets: acceptanceAssets(sourceURL, styleURL)},
			},
			StandardProductSnapshot: &listingkit.StandardProductSnapshot{},
		},
	}
	if err := repo.CreateTask(identityContext, task); err != nil {
		return SeedResult{}, fmt.Errorf("create acceptance task: %w", err)
	}
	return seedResult(task), nil
}

func hasSeedRole(roles []string) bool {
	for _, role := range roles {
		switch strings.TrimSpace(role) {
		case "listingkit_operator", "listingkit_admin", "platform_admin":
			return true
		}
	}
	return false
}

func seedTaskID(tenantID, userID string) string {
	digest := sha256.Sum256([]byte(seedNamespace + "\x00" + tenantID + "\x00" + userID))
	return hex.EncodeToString(digest[:])[:36]
}

func acceptanceAssets(sourceURL, styleURL string) []asset.Asset {
	assets := []asset.Asset{{ID: seedSourceAssetID, Kind: asset.KindSourceImage, URL: sourceURL}}
	if styleURL != "" {
		assets = append(assets, asset.Asset{ID: seedStyleAssetID, Kind: asset.KindGalleryImage, URL: styleURL})
	}
	return assets
}

func equivalentSeedTask(task *listingkit.Task, identity authidentity.AuthenticatedIdentity, sourceURL, styleURL string) bool {
	if task == nil || strings.TrimSpace(task.TenantID) != identity.TenantID || strings.TrimSpace(task.UserID) != identity.UserID || task.Result == nil || task.Result.StandardProductSnapshot == nil {
		return false
	}
	if len(task.Result.AssetBundlesByTarget) != 1 {
		return false
	}
	bundle := task.Result.AssetBundlesByTarget[seedTargetPlatform]
	if bundle == nil {
		return false
	}
	want := acceptanceAssets(sourceURL, styleURL)
	if len(bundle.Assets) != len(want) {
		return false
	}
	for i := range want {
		if !reflect.DeepEqual(bundle.Assets[i], want[i]) {
			return false
		}
	}
	return true
}

func seedResult(task *listingkit.Task) SeedResult {
	return SeedResult{
		TaskID:       task.ID,
		TenantID:     task.TenantID,
		UserID:       task.UserID,
		WorkspaceURL: seedWorkspaceBaseURL + task.ID + "/workspace",
	}
}
