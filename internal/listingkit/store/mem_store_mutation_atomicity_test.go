package store

import (
	"context"
	"errors"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"task-processor/internal/listingkit"
	"task-processor/internal/product/catalog/canonical"
	sheinpub "task-processor/internal/publishing/shein"
)

func TestMemTaskRepositoryMutateTaskResultRollsBackNestedStateAndOwnsReturnedSnapshots(t *testing.T) {
	t.Parallel()

	ctx := listingkit.WithTenantID(context.Background(), "tenant-a")
	repo := NewMemTaskRepository().(*MemTaskRepository)
	leaseUntil := time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)
	draft := &sheinpub.RequestDraft{SpuName: "Original SPU"}
	submission := &sheinpub.SubmissionReport{LastStatus: "original"}
	finalDraft := &sheinpub.FinalDraft{MainImageURL: "https://img.example/original.jpg"}
	require.NoError(t, repo.CreateTask(ctx, &listingkit.Task{
		ID:                                   "task-atomic-mutation",
		TenantID:                             "tenant-a",
		BillingTenantID:                      "billing-a",
		SourceSnapshotVersion:                17,
		GenerationUsageReservationState:      listingkit.GenerationUsageReservationStateReserved,
		GenerationUsageReservationLeaseUntil: &leaseUntil,
		Request: &listingkit.GenerateRequest{
			Platforms: []string{"shein"},
			Source:    &listingkit.SourceReference{URL: "https://source.example/original"},
		},
		Result: &listingkit.ListingKitResult{
			ReviewReasons: []string{"original review"},
			CanonicalProduct: &canonical.Product{
				Title:      "Original title",
				Attributes: map[string]canonical.Attribute{"color": {Value: "black"}},
			},
			Shein: &sheinpub.Package{
				RequestDraft: draft, DraftPayload: draft,
				Submission: submission, SubmissionState: submission,
				FinalDraft: finalDraft, FinalSubmissionDraft: finalDraft,
			},
		},
	}))

	wantErr := errors.New("reject mutation")
	failedSnapshot, err := repo.MutateTaskResult(ctx, "task-atomic-mutation", func(task *listingkit.Task) error {
		task.Request.Platforms[0] = "amazon"
		task.Request.Source.URL = "https://source.example/rejected"
		task.Result.ReviewReasons[0] = "rejected review"
		task.Result.CanonicalProduct.Title = "Rejected title"
		task.Result.CanonicalProduct.Attributes["color"] = canonical.Attribute{Value: "red"}
		return wantErr
	})
	require.ErrorIs(t, err, wantErr)
	require.NotNil(t, failedSnapshot)
	assertMemMutationTaskState(t, failedSnapshot, "shein", "https://source.example/original", "original review", "Original title", "black")
	assertMemMutationTaskCloneContract(t, failedSnapshot, "billing-a", leaseUntil)
	assertStoredMemMutationTaskState(t, repo, ctx, "shein", "https://source.example/original", "original review", "Original title", "black")

	failedSnapshot.Request.Source.URL = "https://source.example/caller-mutated"
	failedSnapshot.Result.CanonicalProduct.Attributes["color"] = canonical.Attribute{Value: "orange"}
	assertStoredMemMutationTaskState(t, repo, ctx, "shein", "https://source.example/original", "original review", "Original title", "black")

	committedSnapshot, err := repo.MutateTaskResult(ctx, "task-atomic-mutation", func(task *listingkit.Task) error {
		task.Request.Platforms[0] = "temu"
		task.Request.Source.URL = "https://source.example/committed"
		task.Result.ReviewReasons[0] = "committed review"
		task.Result.CanonicalProduct.Title = "Committed title"
		task.Result.CanonicalProduct.Attributes["color"] = canonical.Attribute{Value: "blue"}
		return nil
	})
	require.NoError(t, err)
	assertMemMutationTaskState(t, committedSnapshot, "temu", "https://source.example/committed", "committed review", "Committed title", "blue")
	assertMemMutationTaskCloneContract(t, committedSnapshot, "billing-a", leaseUntil)

	committedSnapshot.Request.Platforms[0] = "walmart"
	committedSnapshot.Result.ReviewReasons[0] = "caller-mutated review"
	committedSnapshot.Result.CanonicalProduct.Title = "Caller-mutated title"
	assertStoredMemMutationTaskState(t, repo, ctx, "temu", "https://source.example/committed", "committed review", "Committed title", "blue")
}

func TestMemTaskCloneDurableMetadataSchemaIsExplicit(t *testing.T) {
	typeOf := reflect.TypeOf(listingkit.Task{})
	var hidden []string
	for i := 0; i < typeOf.NumField(); i++ {
		field := typeOf.Field(i)
		if field.Tag.Get("json") == "-" {
			hidden = append(hidden, field.Name)
		}
	}
	sort.Strings(hidden)
	want := []string{"BillingTenantID", "GenerationUsageReservationLeaseUntil", "GenerationUsageReservationState", "SourceSnapshotVersion"}
	require.Equal(t, want, hidden, "update the persistence-equivalent Task clone when durable hidden fields change")
}

func TestMemTaskCloneAndMutationNormalizeIndependentSemanticGraphsWithoutTouchingSource(t *testing.T) {
	ctx := listingkit.WithTenantID(context.Background(), "tenant-semantic")
	legacySDS := &listingkit.SDSSyncSummary{Status: "completed"}
	resultPod := memSemanticPodFixture(" result ")
	snapshotPod := memSemanticPodFixture(" snapshot ")
	source := &listingkit.Task{
		ID: "task-semantic-clone", TenantID: "tenant-semantic",
		Result: &listingkit.ListingKitResult{
			SDSSync:      legacySDS,
			PodExecution: resultPod,
			StandardProductSnapshot: &listingkit.StandardProductSnapshot{
				SDSSync: legacySDS, PodExecution: snapshotPod,
			},
		},
	}

	cloned, err := cloneMemTask(source)
	require.NoError(t, err)
	assertMemSemanticSourceUnchanged(t, source, legacySDS, resultPod, snapshotPod)
	assertMemSemanticCloneNormalized(t, cloned)

	repo := NewMemTaskRepository().(*MemTaskRepository)
	require.NoError(t, repo.CreateTask(ctx, source))
	wantErr := errors.New("reject before mutation")
	callbackCalled := false
	failedSnapshot, err := repo.MutateTaskResult(ctx, source.ID, func(*listingkit.Task) error {
		callbackCalled = true
		return wantErr
	})
	require.ErrorIs(t, err, wantErr)
	require.True(t, callbackCalled)
	assertMemSemanticSourceUnchanged(t, repo.tasks[source.ID], legacySDS, resultPod, snapshotPod)
	assertMemSemanticCloneNormalized(t, failedSnapshot)

	committed, err := repo.MutateTaskResult(ctx, source.ID, func(*listingkit.Task) error { return nil })
	require.NoError(t, err)
	assertMemSemanticCloneNormalized(t, repo.tasks[source.ID])
	assertMemSemanticCloneNormalized(t, committed)
	committed.Result.PodExecution.History[0].Detail = "caller mutation"
	committed.Result.StandardProductSnapshot.PodExecution.Provider = "caller"
	require.NotEqual(t, "caller mutation", repo.tasks[source.ID].Result.PodExecution.History[0].Detail)
	require.NotEqual(t, "caller", repo.tasks[source.ID].Result.StandardProductSnapshot.PodExecution.Provider)
}

func memSemanticPodFixture(detail string) *listingkit.PodExecutionSummary {
	return &listingkit.PodExecutionSummary{
		Provider: " SDS ", DependencyMode: " OPTIONAL ", Status: " SUCCEEDED ", DecisionSource: " fixture ",
		History: []listingkit.PodExecutionAuditEvent{{
			Kind: " STATUS_TRANSITION ", Code: " code ", Detail: detail, Provider: " SDS ", ToStatus: " SUCCEEDED ",
		}},
	}
}

func assertMemSemanticSourceUnchanged(t *testing.T, task *listingkit.Task, legacySDS *listingkit.SDSSyncSummary, resultPod, snapshotPod *listingkit.PodExecutionSummary) {
	t.Helper()
	require.Same(t, legacySDS, task.Result.SDSSync)
	require.Nil(t, task.Result.SDSDesignResult)
	require.Same(t, resultPod, task.Result.PodExecution)
	require.Equal(t, " SDS ", resultPod.Provider)
	require.Equal(t, " result ", resultPod.History[0].Detail)
	require.Same(t, legacySDS, task.Result.StandardProductSnapshot.SDSSync)
	require.Nil(t, task.Result.StandardProductSnapshot.SDSDesignResult)
	require.Same(t, snapshotPod, task.Result.StandardProductSnapshot.PodExecution)
	require.Equal(t, " SDS ", snapshotPod.Provider)
	require.Equal(t, " snapshot ", snapshotPod.History[0].Detail)
}

func assertMemSemanticCloneNormalized(t *testing.T, task *listingkit.Task) {
	t.Helper()
	require.Same(t, task.Result.SDSSync, task.Result.SDSDesignResult)
	require.Equal(t, "sds", task.Result.PodExecution.Provider)
	require.Equal(t, "result", task.Result.PodExecution.History[0].Detail)
	require.Same(t, task.Result.StandardProductSnapshot.SDSSync, task.Result.StandardProductSnapshot.SDSDesignResult)
	require.Equal(t, "sds", task.Result.StandardProductSnapshot.PodExecution.Provider)
	require.Equal(t, "snapshot", task.Result.StandardProductSnapshot.PodExecution.History[0].Detail)
}

func assertMemMutationTaskCloneContract(t *testing.T, task *listingkit.Task, billingTenant string, leaseUntil time.Time) {
	t.Helper()
	require.Equal(t, billingTenant, task.BillingTenantID)
	require.Equal(t, uint64(17), task.SourceSnapshotVersion)
	require.Equal(t, listingkit.GenerationUsageReservationStateReserved, task.GenerationUsageReservationState)
	require.NotNil(t, task.GenerationUsageReservationLeaseUntil)
	require.Equal(t, leaseUntil, *task.GenerationUsageReservationLeaseUntil)
	require.Same(t, task.Result.Shein.RequestDraft, task.Result.Shein.DraftPayload, "draft semantic aliases must remain identical")
	require.Same(t, task.Result.Shein.Submission, task.Result.Shein.SubmissionState, "submission semantic aliases must remain identical")
	require.Same(t, task.Result.Shein.FinalDraft, task.Result.Shein.FinalSubmissionDraft, "final-draft semantic aliases must remain identical")
}

func assertStoredMemMutationTaskState(t *testing.T, repo *MemTaskRepository, ctx context.Context, platform, sourceURL, reviewReason, title, color string) {
	t.Helper()
	stored, err := repo.GetTask(ctx, "task-atomic-mutation")
	require.NoError(t, err)
	assertMemMutationTaskState(t, stored, platform, sourceURL, reviewReason, title, color)
}

func assertMemMutationTaskState(t *testing.T, task *listingkit.Task, platform, sourceURL, reviewReason, title, color string) {
	t.Helper()
	require.Equal(t, platform, task.Request.Platforms[0])
	require.Equal(t, sourceURL, task.Request.Source.URL)
	require.Equal(t, reviewReason, task.Result.ReviewReasons[0])
	require.Equal(t, title, task.Result.CanonicalProduct.Title)
	require.Equal(t, color, task.Result.CanonicalProduct.Attributes["color"].Value)
}
