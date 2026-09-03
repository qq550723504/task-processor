package assetpublication

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"task-processor/internal/imageagent"
	productasset "task-processor/internal/product/asset"
	"task-processor/internal/product/asset/assettest"
)

func TestPublisherCommitsApprovedAssetsExactlyOnce(t *testing.T) {
	projection := approvedV3Projection(t)
	repository := assettest.NewMemoryRepository()
	publisher, err := NewPublisher(staticProjectionSource{projection: projection}, repository, staticPublicURLResolver{})
	require.NoError(t, err)
	input := approvedV3PublicationInput(projection)

	first, err := publisher.PublishApprovedV3(context.Background(), input)
	require.NoError(t, err)
	second, err := publisher.PublishApprovedV3(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, first, second)
	require.Equal(t, projection.AssetCatalog.ProductContext.ProductID, first.ProductKey)
	require.Equal(t, input.IdempotencyKey, first.ActionID)
	require.Equal(t, input.CandidateAssetIDs, first.AssetIDs)

	inventory, err := repository.GetApprovedInventory(context.Background(), productasset.InventoryScope{
		TenantID: projection.Run.TenantID, ProductKey: projection.AssetCatalog.ProductContext.ProductID,
	})
	require.NoError(t, err)
	require.Len(t, inventory.Assets, 2)
	require.Equal(t, productasset.RoleMain, inventory.Assets[0].Role)
	require.Equal(t, productasset.RoleGallery, inventory.Assets[1].Role)
	require.Equal(t, "https://cdn.example.test/"+projection.Slots[0].Candidates[0].DurableAsset.ObjectKey, inventory.Assets[0].URL)
	require.Equal(t, 1200, inventory.Assets[0].Width)
	require.Equal(t, 900, inventory.Assets[0].Height)
	require.Equal(t, []string{"render_scene"}, inventory.Assets[1].Operations)
}

func TestPublisherUsesProductContextIdentityInsteadOfBusinessTaskCompatibility(t *testing.T) {
	projection := approvedV3Projection(t)
	projection.Run.BusinessTaskID = "unrelated-unused-task"
	repository := assettest.NewMemoryRepository()
	publisher, err := NewPublisher(staticProjectionSource{projection: projection}, repository, staticPublicURLResolver{})
	require.NoError(t, err)

	_, err = publisher.PublishApprovedV3(context.Background(), approvedV3PublicationInput(projection))
	require.NoError(t, err)
	_, err = repository.GetApprovedInventory(context.Background(), productasset.InventoryScope{TenantID: "tenant-a", ProductKey: "product-1"})
	require.NoError(t, err)
	_, err = repository.GetApprovedInventory(context.Background(), productasset.InventoryScope{TenantID: "tenant-a", ProductKey: "unrelated-unused-task"})
	require.ErrorIs(t, err, productasset.ErrApprovedAssetsNotReady)
}

func TestV2PublisherWritesURLCandidatesToProductAssetsWithoutListingKit(t *testing.T) {
	projection := approvedV3Projection(t)
	for index := range projection.Slots {
		candidate := &projection.Slots[index].Candidates[0]
		candidate.URL = fmt.Sprintf("https://cdn.example.test/v2-%d.png", index+1)
		candidate.DurableAsset = imageagent.DurableAssetIdentity{}
	}
	var err error
	projection.ResultDigest, err = imageagent.ResultDigestV2(projection.Plan, projection.Slots)
	require.NoError(t, err)
	repository := assettest.NewMemoryRepository()
	publisher, err := NewV2Publisher(staticProjectionSource{projection: projection}, repository)
	require.NoError(t, err)

	acknowledgement, err := publisher.PublishApproved(context.Background(), imageagent.PublishApprovedInput{
		RunID: "run-1", TenantID: "tenant-a", UserID: "user-a", PlanRevision: 1,
		CandidateAssetIDs: []string{"asset-1", "asset-2"}, IdempotencyKey: "approve-v2-action-1",
	})

	require.NoError(t, err)
	require.Equal(t, "product-1", acknowledgement.ProductKey)
	inventory, err := repository.GetApprovedInventory(context.Background(), productasset.InventoryScope{TenantID: "tenant-a", ProductKey: "product-1"})
	require.NoError(t, err)
	require.Equal(t, "https://cdn.example.test/v2-1.png", inventory.Assets[0].URL)
}

func TestPublisherRejectsInvalidProjectionBeforeRepositoryCommit(t *testing.T) {
	tests := map[string]func(*imageagent.RunProjection, *imageagent.PublishApprovedV3Input){
		"wrong run status": func(projection *imageagent.RunProjection, _ *imageagent.PublishApprovedV3Input) {
			projection.Run.Status = imageagent.RunStatusExecuting
		},
		"missing product identity": func(projection *imageagent.RunProjection, _ *imageagent.PublishApprovedV3Input) {
			projection.AssetCatalog.ProductContext.ProductID = ""
		},
		"digest drift": func(projection *imageagent.RunProjection, _ *imageagent.PublishApprovedV3Input) {
			projection.ResultDigest = "drifted"
		},
		"candidate order drift": func(_ *imageagent.RunProjection, input *imageagent.PublishApprovedV3Input) {
			input.CandidateAssetIDs[0], input.CandidateAssetIDs[1] = input.CandidateAssetIDs[1], input.CandidateAssetIDs[0]
		},
		"durable object drift": func(projection *imageagent.RunProjection, _ *imageagent.PublishApprovedV3Input) {
			projection.Slots[0].Candidates[0].DurableAsset.ObjectKey = "image-agent/public/wrong"
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			projection := approvedV3Projection(t)
			input := approvedV3PublicationInput(projection)
			mutate(&projection, &input)
			repository := &countingAssetRepository{Repository: assettest.NewMemoryRepository()}
			publisher, err := NewPublisher(staticProjectionSource{projection: projection}, repository, staticPublicURLResolver{})
			require.NoError(t, err)

			_, err = publisher.PublishApprovedV3(context.Background(), input)
			require.Error(t, err)
			require.Zero(t, repository.commits)
		})
	}
}

type staticProjectionSource struct{ projection imageagent.RunProjection }

func (s staticProjectionSource) GetProjection(context.Context, imageagent.RunScope) (imageagent.RunProjection, error) {
	return s.projection, nil
}

type staticPublicURLResolver struct{}

func (staticPublicURLResolver) PublicURL(key string) string { return "https://cdn.example.test/" + key }

type countingAssetRepository struct {
	productasset.Repository
	commits int
}

func (r *countingAssetRepository) CommitApproval(ctx context.Context, commit productasset.ApprovalCommit) (productasset.ApprovalReceipt, error) {
	r.commits++
	return r.Repository.CommitApproval(ctx, commit)
}

func approvedV3Projection(t *testing.T) imageagent.RunProjection {
	t.Helper()
	ownerKey, err := imageagent.ArtifactOwnerKey("user-a")
	require.NoError(t, err)
	plan := imageagent.Plan{Revision: 1, Slots: []imageagent.Slot{
		{ID: "main-1", Role: imageagent.SlotRoleMain},
		{ID: "scene-1", Role: imageagent.SlotRoleScene},
	}}
	projection := imageagent.RunProjection{
		Run:          imageagent.Run{ID: "run-1", BusinessTaskID: "task-legacy", TenantID: "tenant-a", UserID: "user-a", Status: imageagent.RunStatusAwaitingFinalApproval, ActivePlanRevision: 1},
		Plan:         plan,
		AssetCatalog: imageagent.AssetCatalog{ProductContext: imageagent.ProductContextRef{ProductID: "product-1"}},
		Slots:        make([]imageagent.SlotProjection, len(plan.Slots)),
	}
	for index, slot := range plan.Slots {
		hash := fmt.Sprintf("%064x", index+1)
		projection.Slots[index] = imageagent.SlotProjection{
			Slot: imageagent.Slot{ID: slot.ID, Role: slot.Role, Status: imageagent.SlotStatusAccepted}, Attempt: 1,
			Candidates: []imageagent.AssetCandidate{{
				AssetID: fmt.Sprintf("asset-%d", index+1), SourceAssetID: "source-1",
				Width: 1200, Height: 900, Operations: []string{"render_scene"},
				DurableAsset: imageagent.DurableAssetIdentity{
					ObjectKey: fmt.Sprintf("image-agent/public/tenant-a/%s/run-1/1/%s/1/0-%s.png", ownerKey, slot.ID, hash), SHA256: hash,
				},
			}},
		}
	}
	projection.ResultDigest, err = imageagent.ResultDigestV3(projection.Plan, projection.Slots)
	require.NoError(t, err)
	return projection
}

func approvedV3PublicationInput(projection imageagent.RunProjection) imageagent.PublishApprovedV3Input {
	return imageagent.PublishApprovedV3Input{
		RunID: projection.Run.ID, TenantID: projection.Run.TenantID, UserID: projection.Run.UserID,
		PlanRevision: projection.Plan.Revision, CandidateAssetIDs: []string{"asset-1", "asset-2"},
		IdempotencyKey: "approve-action-1",
	}
}
