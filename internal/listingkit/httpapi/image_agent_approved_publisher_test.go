package httpapi

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"task-processor/internal/asset"
	"task-processor/internal/authidentity"
	"task-processor/internal/imageagent"
	"task-processor/internal/listingkit"
	listingkitstore "task-processor/internal/listingkit/store"
)

const approvedProjectionDigest = "9e5c8dba27d1224662e48945bf1456d7c339f541250228b068abafe8a944c0e6"

var approvedPublisherDatabaseCounter atomic.Uint64

func TestImageAgentApprovedPublisherCommitsCanonicalAssetsAndReceiptExactlyOnce(t *testing.T) {
	ctx, db, transactionStore := newApprovedPublisherDatabase(t)
	require.NoError(t, db.Create(approvedPublisherTask()).Error)
	projection := approvedProjection()
	publisher, err := NewImageAgentApprovedPublisher(staticProjectionSource{projection: projection}, transactionStore)
	require.NoError(t, err)
	input := approvedPublicationInput(projection)

	first, err := publisher.PublishApproved(ctx, input)
	require.NoError(t, err)
	second, err := publisher.PublishApproved(ctx, input)
	require.NoError(t, err)
	require.Equal(t, first, second)
	require.Equal(t, input.IdempotencyKey, first.IdempotencyKey)
	require.Equal(t, projection.ResultDigest, first.ResultDigest)

	var task listingkit.Task
	require.NoError(t, db.First(&task, "id = ?", "task-1").Error)
	require.Equal(t, []string{"shein"}, task.Result.Platforms, "unrelated task-result fields must survive")
	require.Len(t, task.Result.StandardProductSnapshot.AssetBundle.Assets, 13, "source + main + eleven scene assets must survive without a ten-image cap")
	require.Equal(t, "candidate-main", task.Result.StandardProductSnapshot.AssetBundle.Selection.MainAssetID)
	require.Len(t, task.Result.StandardProductSnapshot.AssetBundle.Selection.GalleryAssetIDs, 11)
	require.Equal(t, task.Result.StandardProductSnapshot.AssetBundle, task.Result.AssetBundle)
	require.Equal(t, task.Result.StandardProductSnapshot.AssetInventorySummary, task.Result.AssetInventorySummary)
}

func TestImageAgentApprovedPublisherRejectsStateDigestCandidateAndScopeInjection(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*imageagent.RunProjection, *imageagent.PublishApprovedInput)
	}{
		{name: "run scope", mutate: func(p *imageagent.RunProjection, _ *imageagent.PublishApprovedInput) { p.Run.ID = "foreign-run" }},
		{name: "business task", mutate: func(p *imageagent.RunProjection, _ *imageagent.PublishApprovedInput) {
			p.Run.BusinessTaskID = "foreign-task"
		}},
		{name: "not awaiting approval", mutate: func(p *imageagent.RunProjection, _ *imageagent.PublishApprovedInput) {
			p.Run.Status = imageagent.RunStatusExecuting
		}},
		{name: "digest mismatch", mutate: func(p *imageagent.RunProjection, _ *imageagent.PublishApprovedInput) { p.ResultDigest = "wrong-digest" }},
		{name: "slot not accepted", mutate: func(p *imageagent.RunProjection, _ *imageagent.PublishApprovedInput) {
			p.Slots[1].Slot.Status = imageagent.SlotStatusPending
		}},
		{name: "partial approved set", mutate: func(_ *imageagent.RunProjection, input *imageagent.PublishApprovedInput) {
			input.CandidateAssetIDs = input.CandidateAssetIDs[:1]
		}},
		{name: "foreign candidate", mutate: func(_ *imageagent.RunProjection, input *imageagent.PublishApprovedInput) {
			input.CandidateAssetIDs[1] = "foreign-candidate"
		}},
		{name: "unsafe candidate URL", mutate: func(p *imageagent.RunProjection, _ *imageagent.PublishApprovedInput) {
			p.Slots[1].Candidates[0].URL = "file:///etc/passwd"
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, db, transactionStore := newApprovedPublisherDatabase(t)
			require.NoError(t, db.Create(approvedPublisherTask()).Error)
			projection := approvedProjection()
			input := approvedPublicationInput(projection)
			tt.mutate(&projection, &input)
			publisher, err := NewImageAgentApprovedPublisher(staticProjectionSource{projection: projection}, transactionStore)
			require.NoError(t, err)

			_, err = publisher.PublishApproved(ctx, input)
			require.Error(t, err)
			var task listingkit.Task
			require.NoError(t, db.First(&task, "id = ?", "task-1").Error)
			require.Len(t, task.Result.StandardProductSnapshot.AssetBundle.Assets, 1)
		})
	}
}

func newApprovedPublisherDatabase(t *testing.T) (context.Context, *gorm.DB, listingkit.ImageAgentPublicationTransactionRepository) {
	t.Helper()
	dsn := fmt.Sprintf("file:image-agent-approved-%s-%d?mode=memory&cache=shared", t.Name(), approvedPublisherDatabaseCounter.Add(1))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&listingkit.Task{}))
	require.NoError(t, listingkitstore.AutoMigrateImageAgentPublicationReceipts(db))
	ctx := authidentity.WithAuthenticatedIdentity(context.Background(), authidentity.AuthenticatedIdentity{TenantID: "tenant-a", UserID: "user-a"})
	return ctx, db, listingkitstore.NewImageAgentPublicationTransactionRepository(db)
}

func approvedPublisherTask() *listingkit.Task {
	return &listingkit.Task{ID: "task-1", TenantID: "tenant-a", UserID: "user-a", Result: &listingkit.ListingKitResult{Platforms: []string{"shein"}, StandardProductSnapshot: &listingkit.StandardProductSnapshot{AssetBundle: &asset.Bundle{Assets: []asset.Asset{{ID: "source-1", Kind: asset.KindSourceImage, URL: "https://source.example/1.png"}}}}}}
}

func approvedProjection() imageagent.RunProjection {
	sceneCandidates := make([]imageagent.AssetCandidate, 11)
	for index := range sceneCandidates {
		sceneCandidates[index] = imageagent.AssetCandidate{AssetID: fmt.Sprintf("candidate-scene-%02d", index+1), URL: fmt.Sprintf("https://cdn.example/scene-%02d.png", index+1), SourceAssetID: "source-1"}
	}
	projection := imageagent.RunProjection{
		Run:  imageagent.Run{ID: "run-1", BusinessTaskID: "task-1", TenantID: "tenant-a", UserID: "user-a", Status: imageagent.RunStatusAwaitingFinalApproval, ActivePlanRevision: 1},
		Plan: imageagent.Plan{Revision: 1, Slots: []imageagent.Slot{{ID: "main", Role: imageagent.SlotRoleMain}, {ID: "scene", Role: imageagent.SlotRoleScene}}},
		Slots: []imageagent.SlotProjection{
			{Slot: imageagent.Slot{ID: "main", Role: imageagent.SlotRoleMain, Status: imageagent.SlotStatusAccepted}, Attempt: 1, Candidates: []imageagent.AssetCandidate{{AssetID: "candidate-main", URL: "https://cdn.example/main.png", SourceAssetID: "source-1"}}},
			{Slot: imageagent.Slot{ID: "scene", Role: imageagent.SlotRoleScene, Status: imageagent.SlotStatusAccepted}, Attempt: 1, Candidates: sceneCandidates},
		},
	}
	projection.ResultDigest = approvedProjectionDigest
	return projection
}

func approvedPublicationInput(projection imageagent.RunProjection) imageagent.PublishApprovedInput {
	ids := []string{"candidate-main"}
	for _, candidate := range projection.Slots[1].Candidates {
		ids = append(ids, candidate.AssetID)
	}
	return imageagent.PublishApprovedInput{RunID: "run-1", TenantID: "tenant-a", UserID: "user-a", PlanRevision: 1, CandidateAssetIDs: ids, IdempotencyKey: "approve-action-1"}
}

type staticProjectionSource struct{ projection imageagent.RunProjection }

func (s staticProjectionSource) GetProjection(context.Context, imageagent.RunScope) (imageagent.RunProjection, error) {
	return s.projection, nil
}
