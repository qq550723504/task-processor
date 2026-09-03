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
	want := []string{"BillingTenantID", "GenerationUsageReservationLeaseUntil", "GenerationUsageReservationState"}
	require.Equal(t, want, hidden, "update the persistence-equivalent Task clone when durable hidden fields change")
}

func assertMemMutationTaskCloneContract(t *testing.T, task *listingkit.Task, billingTenant string, leaseUntil time.Time) {
	t.Helper()
	require.Equal(t, billingTenant, task.BillingTenantID)
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
