package httpapi

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"task-processor/internal/asset"
	"task-processor/internal/authidentity"
	"task-processor/internal/imageagent"
	"task-processor/internal/listingkit"
	listingkitstore "task-processor/internal/listingkit/store"
)

func TestImageAgentApprovedPublisherWritesSelectedCandidatesCanonicallyAndIdempotently(t *testing.T) {
	ctx := authidentity.WithAuthenticatedIdentity(context.Background(), authidentity.AuthenticatedIdentity{TenantID: "tenant-a", UserID: "user-a"})
	tasks := listingkitstore.NewMemTaskRepository()
	taskResults := tasks.(listingkit.TaskResultTransactionRepository)
	require.NoError(t, tasks.CreateTask(ctx, &listingkit.Task{ID: "task-1", TenantID: "tenant-a", UserID: "user-a", Result: &listingkit.ListingKitResult{StandardProductSnapshot: &listingkit.StandardProductSnapshot{AssetBundle: &asset.Bundle{Assets: []asset.Asset{{ID: "source-1", Kind: asset.KindSourceImage, URL: "https://source.example/1.png"}}}}}}))
	projection := imageagent.RunProjection{
		Run:  imageagent.Run{ID: "run-1", BusinessTaskID: "task-1", TenantID: "tenant-a", UserID: "user-a", ActivePlanRevision: 1},
		Plan: imageagent.Plan{Revision: 1},
		Slots: []imageagent.SlotProjection{
			{Slot: imageagent.Slot{ID: "main", Role: imageagent.SlotRoleMain}, Candidates: []imageagent.AssetCandidate{{AssetID: "candidate-main", URL: "https://cdn.example/main.png", SourceAssetID: "source-1"}}},
			{Slot: imageagent.Slot{ID: "scene", Role: imageagent.SlotRoleScene}, Candidates: []imageagent.AssetCandidate{{AssetID: "candidate-scene", URL: "https://cdn.example/scene.png", SourceAssetID: "source-1"}}},
		},
	}
	publisher, err := NewImageAgentApprovedPublisher(staticProjectionSource{projection: projection}, taskResults)
	require.NoError(t, err)
	input := imageagent.PublishApprovedInput{RunID: "run-1", TenantID: "tenant-a", UserID: "user-a", PlanRevision: 1, CandidateAssetIDs: []string{"candidate-main", "candidate-scene"}, IdempotencyKey: "approve-1"}

	require.NoError(t, publisher.PublishApproved(ctx, input))
	require.NoError(t, publisher.PublishApproved(ctx, input))
	task, err := tasks.GetTask(ctx, "task-1")
	require.NoError(t, err)
	require.Len(t, task.Result.StandardProductSnapshot.AssetBundle.Assets, 3)
	require.Equal(t, "candidate-main", task.Result.StandardProductSnapshot.AssetBundle.Selection.MainAssetID)
	require.Equal(t, []string{"candidate-scene"}, task.Result.StandardProductSnapshot.AssetBundle.Selection.GalleryAssetIDs)
	require.Equal(t, task.Result.StandardProductSnapshot.AssetBundle, task.Result.AssetBundle)
}

type staticProjectionSource struct{ projection imageagent.RunProjection }

func (s staticProjectionSource) GetProjection(context.Context, imageagent.RunScope) (imageagent.RunProjection, error) {
	return s.projection, nil
}
