package httpapi

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"task-processor/internal/asset"
	"task-processor/internal/authidentity"
	"task-processor/internal/product/catalog"
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

func TestImageAgentApprovedPublisherSupersedesPriorImageAgentAssets(t *testing.T) {
	ctx, db, transactionStore := newApprovedPublisherDatabase(t)
	require.NoError(t, db.Create(approvedPublisherTask()).Error)

	previous := approvedProjection()
	previous.Run.ID = "run-previous"
	for slotIndex := range previous.Slots {
		for candidateIndex := range previous.Slots[slotIndex].Candidates {
			previous.Slots[slotIndex].Candidates[candidateIndex].AssetID = fmt.Sprintf("previous-%s-%02d", previous.Slots[slotIndex].Slot.ID, candidateIndex+1)
		}
	}
	previousDigest, err := imageagent.ResultDigestV2(previous.Plan, previous.Slots)
	require.NoError(t, err)
	previous.ResultDigest = previousDigest
	previousPublisher, err := NewImageAgentApprovedPublisher(staticProjectionSource{projection: previous}, transactionStore)
	require.NoError(t, err)
	previousInput := approvedPublicationInput(previous)
	previousInput.RunID = previous.Run.ID
	previousInput.IdempotencyKey = "approve-previous"
	previousInput.CandidateAssetIDs[0] = previous.Slots[0].Candidates[0].AssetID
	_, err = previousPublisher.PublishApproved(ctx, previousInput)
	require.NoError(t, err)

	current := approvedProjection()
	currentPublisher, err := NewImageAgentApprovedPublisher(staticProjectionSource{projection: current}, transactionStore)
	require.NoError(t, err)
	_, err = currentPublisher.PublishApproved(ctx, approvedPublicationInput(current))
	require.NoError(t, err)

	var task listingkit.Task
	require.NoError(t, db.First(&task, "id = ?", "task-1").Error)
	bundle := task.Result.StandardProductSnapshot.AssetBundle
	require.Len(t, bundle.Assets, 13, "a newer approval must replace every prior image-agent output")
	require.Equal(t, "candidate-main", bundle.Selection.MainAssetID)
	require.NotContains(t, bundle.Selection.GalleryAssetIDs, "previous-scene-01")
	for _, item := range bundle.Assets {
		require.NotContains(t, item.ID, "previous-", "superseded image-agent assets must not remain publishable")
	}
}

func TestApplyApprovedAssetRecordsUpdatesOnlyTheRunTargetBundle(t *testing.T) {
	result := &listingkit.ListingKitResult{
		StandardProductSnapshot: &listingkit.StandardProductSnapshot{AssetBundle: &asset.Bundle{Assets: []asset.Asset{{ID: "scalar-source", Kind: asset.KindSourceImage, URL: "https://cdn.example.test/scalar.png"}}}},
		AssetBundlesByTarget: map[string]*asset.Bundle{
			"shein":  {Assets: []asset.Asset{{ID: "shein-source", Kind: asset.KindSourceImage, URL: "https://cdn.example.test/shein.png"}}},
			"amazon": {Assets: []asset.Asset{{ID: "amazon-source", Kind: asset.KindSourceImage, URL: "https://cdn.example.test/amazon.png"}}},
		},
		AssetInventorySummariesByTarget: map[string]*asset.InventorySummary{},
	}

	err := applyApprovedAssetRecords(result, []asset.AssetRecord{{ID: "shein-generated", TaskID: "task-1", Kind: asset.KindMainImage, URL: "https://cdn.example.test/generated.png", Generator: "image-agent"}}, " SHEIN ")

	require.NoError(t, err)
	require.Equal(t, "scalar-source", result.StandardProductSnapshot.AssetBundle.Assets[0].ID)
	require.Equal(t, "amazon-source", result.AssetBundlesByTarget["amazon"].Assets[0].ID)
	require.Equal(t, []string{"shein-source", "shein-generated"}, []string{result.AssetBundlesByTarget["shein"].Assets[0].ID, result.AssetBundlesByTarget["shein"].Assets[1].ID})
	require.NotNil(t, result.AssetInventorySummariesByTarget["shein"])
	require.Nil(t, result.AssetInventorySummariesByTarget["amazon"])
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

func TestPublishApprovedV3RejectsZeroMainCandidates(t *testing.T) {
	projection := approvedV3Projection(t)
	projection.Plan.Slots[0].Role = imageagent.SlotRoleScene
	projection.Slots[0].Slot.Role = imageagent.SlotRoleScene
	projection.ResultDigest = mustResultDigestV3(t, projection)
	assertV3PublicationRejectedWithoutMutation(t, projection, approvedV3PublicationInput(projection))
}

func TestPublishApprovedV3RejectsMultipleMainCandidates(t *testing.T) {
	projection := approvedV3Projection(t)
	second := projection.Slots[0].Candidates[0]
	second.AssetID = "candidate-main-2"
	oldHash, newHash := second.DurableAsset.SHA256, strings.Repeat("b", 64)
	second.DurableAsset.ObjectKey = strings.Replace(second.DurableAsset.ObjectKey, "/0-"+oldHash+".png", "/1-"+newHash+".png", 1)
	second.DurableAsset.SHA256 = newHash
	projection.Slots[0].Candidates = append(projection.Slots[0].Candidates, second)
	projection.ResultDigest = mustResultDigestV3(t, projection)
	assertV3PublicationRejectedWithoutMutation(t, projection, approvedV3PublicationInput(projection))
}

func TestPublishApprovedV3CommitsExactlyOneMainAndAllGalleryAssets(t *testing.T) {
	ctx, db, transactionStore := newApprovedPublisherDatabase(t)
	require.NoError(t, db.Create(approvedPublisherTask()).Error)
	projection := approvedV3Projection(t)
	publisher, err := NewImageAgentApprovedPublisherV3(staticProjectionSource{projection: projection}, transactionStore, staticPublicURLResolver{base: "https://cdn.example"})
	require.NoError(t, err)

	ack, err := publisher.PublishApprovedV3(ctx, approvedV3PublicationInput(projection))
	require.NoError(t, err)
	require.Equal(t, projection.ResultDigest, ack.ResultDigest)

	var task listingkit.Task
	require.NoError(t, db.First(&task, "id = ?", "task-1").Error)
	bundle := task.Result.StandardProductSnapshot.AssetBundle
	require.Len(t, bundle.Assets, 13, "source + main + eleven scene assets must survive without a ten-image cap")
	require.Equal(t, "candidate-main", bundle.Selection.MainAssetID)
	require.Len(t, bundle.Selection.GalleryAssetIDs, 11)
	for _, generated := range bundle.Assets[1:] {
		require.True(t, strings.HasPrefix(generated.URL, "https://cdn.example/image-agent/public/"))
	}
}

func TestPublishApprovedV3RejectsCandidateWithoutDurableIdentity(t *testing.T) {
	projection := approvedV3Projection(t)
	projection.Slots[1].Candidates[0].DurableAsset = imageagent.DurableAssetIdentity{}
	assertV3PublicationRejectedWithoutMutation(t, projection, approvedV3PublicationInput(projection))
}

func TestPublishApprovedV3RejectsUnscopedPublishedKeysBeforeTransaction(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(string) string
	}{
		{name: "staging key", mutate: func(key string) string { return strings.Replace(key, "image-agent/public/", "image-agent/staging/", 1) }},
		{name: "foreign tenant", mutate: func(key string) string { return strings.Replace(key, "/tenant-a/", "/tenant-b/", 1) }},
		{name: "foreign run", mutate: func(key string) string { return strings.Replace(key, "/run-1/", "/run-b/", 1) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			projection := approvedV3Projection(t)
			candidate := &projection.Slots[1].Candidates[0]
			candidate.DurableAsset.ObjectKey = test.mutate(candidate.DurableAsset.ObjectKey)
			projection.ResultDigest = mustResultDigestV3(t, projection)
			assertV3PublicationRejectedWithoutMutation(t, projection, approvedV3PublicationInput(projection))
		})
	}
}

func TestPublishApprovedV3PreflightsEveryResolvedURLBeforeMutation(t *testing.T) {
	projection := approvedV3Projection(t)
	input := approvedV3PublicationInput(projection)
	ctx, db, transactionStore := newApprovedPublisherDatabase(t)
	require.NoError(t, db.Create(approvedPublisherTask()).Error)
	publisher, err := NewImageAgentApprovedPublisherV3(staticProjectionSource{projection: projection}, transactionStore, staticPublicURLResolver{base: "javascript:alert(1)"})
	require.NoError(t, err)

	_, err = publisher.PublishApprovedV3(ctx, input)
	require.Error(t, err)
	var task listingkit.Task
	require.NoError(t, db.First(&task, "id = ?", "task-1").Error)
	require.Len(t, task.Result.StandardProductSnapshot.AssetBundle.Assets, 1)
}

func TestImageAgentApprovedPublisherRejectsChangedTaskSnapshotInsideTransaction(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*listingkit.Task)
	}{
		{name: "source asset", mutate: func(task *listingkit.Task) {
			task.Result.StandardProductSnapshot.AssetBundle.Assets[0].URL = "https://source.example/changed.png"
		}},
		{name: "product context", mutate: func(task *listingkit.Task) {
			task.Result.StandardProductSnapshot.CatalogProduct.Title = "Changed title"
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx, db, transactionStore := newApprovedPublisherDatabase(t)
			task := approvedPublisherTask()
			test.mutate(task)
			require.NoError(t, db.Create(task).Error)
			projection := approvedProjection()
			publisher, err := NewImageAgentApprovedPublisher(staticProjectionSource{projection: projection}, transactionStore)
			require.NoError(t, err)

			_, err = publisher.PublishApproved(ctx, approvedPublicationInput(projection))
			require.ErrorIs(t, err, imageagent.ErrRevisionConflict)
			var stored listingkit.Task
			require.NoError(t, db.First(&stored, "id = ?", "task-1").Error)
			require.Len(t, stored.Result.StandardProductSnapshot.AssetBundle.Assets, 1)
		})
	}
}

func assertV3PublicationRejectedWithoutMutation(t *testing.T, projection imageagent.RunProjection, input imageagent.PublishApprovedV3Input) {
	t.Helper()
	ctx, db, transactionStore := newApprovedPublisherDatabase(t)
	require.NoError(t, db.Create(approvedPublisherTask()).Error)
	countingStore := &countingImageAgentPublicationRepository{delegate: transactionStore}
	publisher, err := NewImageAgentApprovedPublisherV3(staticProjectionSource{projection: projection}, countingStore, staticPublicURLResolver{base: "https://cdn.example"})
	require.NoError(t, err)
	_, err = publisher.PublishApprovedV3(ctx, input)
	require.Error(t, err)
	require.Zero(t, countingStore.calls, "invalid v3 projection must be rejected before the publication transaction")
	var task listingkit.Task
	require.NoError(t, db.First(&task, "id = ?", "task-1").Error)
	require.Len(t, task.Result.StandardProductSnapshot.AssetBundle.Assets, 1)
}

type countingImageAgentPublicationRepository struct {
	delegate listingkit.ImageAgentPublicationTransactionRepository
	calls    int
}

func (r *countingImageAgentPublicationRepository) CommitImageAgentPublication(ctx context.Context, command listingkit.ImageAgentPublicationCommit, mutate listingkit.TaskResultMutation) (listingkit.ImageAgentPublicationAcknowledgement, error) {
	r.calls++
	return r.delegate.CommitImageAgentPublication(ctx, command, mutate)
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
	return &listingkit.Task{ID: "task-1", TenantID: "tenant-a", UserID: "user-a", Result: &listingkit.ListingKitResult{Platforms: []string{"shein"}, StandardProductSnapshot: &listingkit.StandardProductSnapshot{CatalogProduct: &catalog.ProductSnapshot{Title: "Travel Bottle", CategoryPath: []string{"Outdoors", "Bottles"}, Attributes: []catalog.Attribute{{Name: "Material", Value: "Steel"}}}, AssetBundle: &asset.Bundle{Assets: []asset.Asset{{ID: "source-1", Kind: asset.KindSourceImage, URL: "https://source.example/1.png"}}}}}}
}

func approvedProjection() imageagent.RunProjection {
	sceneCandidates := make([]imageagent.AssetCandidate, 11)
	for index := range sceneCandidates {
		sceneCandidates[index] = imageagent.AssetCandidate{AssetID: fmt.Sprintf("candidate-scene-%02d", index+1), URL: fmt.Sprintf("https://cdn.example/scene-%02d.png", index+1), SourceAssetID: "source-1"}
	}
	catalogSnapshot, err := imageAgentCatalogFromTask(approvedPublisherTask())
	if err != nil {
		panic(err)
	}
	projection := imageagent.RunProjection{
		Run:          imageagent.Run{ID: "run-1", BusinessTaskID: "task-1", TenantID: "tenant-a", UserID: "user-a", Status: imageagent.RunStatusAwaitingFinalApproval, ActivePlanRevision: 1},
		AssetCatalog: catalogSnapshot,
		Plan:         imageagent.Plan{Revision: 1, Slots: []imageagent.Slot{{ID: "main", Role: imageagent.SlotRoleMain}, {ID: "scene", Role: imageagent.SlotRoleScene}}},
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

func approvedV3Projection(t *testing.T) imageagent.RunProjection {
	t.Helper()
	projection := approvedProjection()
	ownerKey, err := imageagent.ArtifactOwnerKey(projection.Run.UserID)
	require.NoError(t, err)
	for slotIndex := range projection.Slots {
		for candidateIndex := range projection.Slots[slotIndex].Candidates {
			candidate := &projection.Slots[slotIndex].Candidates[candidateIndex]
			hash := fmt.Sprintf("%064x", slotIndex*100+candidateIndex+1)
			candidate.URL = ""
			candidate.DurableAsset = imageagent.DurableAssetIdentity{ObjectKey: fmt.Sprintf("image-agent/public/tenant-a/%s/run-1/1/%s/1/%d-%s.png", ownerKey, projection.Plan.Slots[slotIndex].ID, candidateIndex, hash), SHA256: hash}
		}
	}
	projection.ResultDigest = mustResultDigestV3(t, projection)
	return projection
}

func mustResultDigestV3(t *testing.T, projection imageagent.RunProjection) string {
	t.Helper()
	digest, err := imageagent.ResultDigestV3(projection.Plan, projection.Slots)
	require.NoError(t, err)
	return digest
}

func approvedV3PublicationInput(projection imageagent.RunProjection) imageagent.PublishApprovedV3Input {
	ids := make([]string, 0)
	for _, slot := range projection.Slots {
		for _, candidate := range slot.Candidates {
			ids = append(ids, candidate.AssetID)
		}
	}
	return imageagent.PublishApprovedV3Input{RunID: projection.Run.ID, TenantID: projection.Run.TenantID, UserID: projection.Run.UserID, PlanRevision: projection.Plan.Revision, CandidateAssetIDs: ids, IdempotencyKey: "approve-v3-action-1"}
}

type staticPublicURLResolver struct{ base string }

func (r staticPublicURLResolver) PublicURL(key string) string {
	return strings.TrimRight(r.base, "/") + "/" + key
}

type staticProjectionSource struct{ projection imageagent.RunProjection }

func (s staticProjectionSource) GetProjection(context.Context, imageagent.RunScope) (imageagent.RunProjection, error) {
	return s.projection, nil
}
