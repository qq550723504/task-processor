package imageagent_test

import (
	"context"
	"encoding/json"
	"testing"

	"task-processor/internal/imageagent"
	"task-processor/internal/imageagent/store"

	"github.com/stretchr/testify/require"
)

func TestServiceRejectsInvalidImagePolicyKeyBeforeResolvingAssets(t *testing.T) {
	t.Parallel()

	tests := map[string]func(*imageagent.StartRunInput){
		"missing marketplace":   func(input *imageagent.StartRunInput) { input.TargetPlatform = "" },
		"uppercase marketplace": func(input *imageagent.StartRunInput) { input.TargetPlatform = "SHEIN" },
		"uppercase country":     func(input *imageagent.StartRunInput) { input.ImagePolicyContext.Country = "US" },
		"trimmed family":        func(input *imageagent.StartRunInput) { input.ImagePolicyContext.Family = " default" },
		"missing category":      func(input *imageagent.StartRunInput) { input.ImagePolicyContext.SceneCategory = "" },
	}
	for name, mutate := range tests {
		name, mutate := name, mutate
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			catalog := &countingPolicyCatalog{catalog: authorizedCatalog()}
			service, err := imageagent.NewService(store.NewMemoryRepository(), &recordingWorkflowClient{}, catalog)
			require.NoError(t, err)
			input := validPolicyStartInput("run-invalid-policy")
			mutate(&input)

			err = service.Start(verifiedContext("tenant-a", "user-a"), input)

			require.ErrorIs(t, err, imageagent.ErrValidation)
			require.Zero(t, catalog.calls)
		})
	}
}

func TestServicePersistsImmutableImagePolicyContextAndUsesItForStartIdempotency(t *testing.T) {
	t.Parallel()

	repository := store.NewMemoryRepository()
	workflows := &recordingWorkflowClient{}
	catalog := &countingPolicyCatalog{catalog: authorizedCatalog()}
	service, err := imageagent.NewService(repository, workflows, catalog)
	require.NoError(t, err)
	input := validPolicyStartInput("run-policy")
	ctx := verifiedContext("tenant-a", "user-a")

	require.NoError(t, service.Start(ctx, input))
	projection, err := repository.GetProjection(ctx, imageagent.RunScope{TenantID: "tenant-a", OwnerUserID: "user-a", RunID: input.RunID})
	require.NoError(t, err)
	require.Equal(t, input.TargetPlatform, projection.Run.TargetPlatform)
	require.Equal(t, input.ImagePolicyContext, projection.Run.ImagePolicyContext)
	require.Len(t, workflows.starts, 1)
	require.Equal(t, input.ImagePolicyContext, workflows.starts[0].Run.ImagePolicyContext)

	conflict := input
	conflict.ImagePolicyContext.Family = "family-b"
	require.ErrorIs(t, service.Start(ctx, conflict), imageagent.ErrRevisionConflict)
	require.Equal(t, 1, catalog.calls, "conflicting replay must not re-read mutable assets")
}

func TestSlotExecutionFingerprintBindsStructuredPolicyKey(t *testing.T) {
	t.Parallel()

	context := &imageagent.ImagePolicyContext{Country: "us", Family: "default", SceneCategory: "shoes"}
	base := imageagent.SlotExecutionInput{
		RunID: "run-1", TargetPlatform: "shein", ImagePolicyContext: context,
		PlanRevision: 1, Slot: imageagent.Slot{ID: "slot-1"}, Attempt: 1, IdempotencyKey: "attempt-1",
	}
	wantDifferent := []imageagent.SlotExecutionInput{base, base, base, base}
	wantDifferent[0].TargetPlatform = "temu"
	wantDifferent[1].ImagePolicyContext = &imageagent.ImagePolicyContext{Country: "ca", Family: "default", SceneCategory: "shoes"}
	wantDifferent[2].ImagePolicyContext = &imageagent.ImagePolicyContext{Country: "us", Family: "family-b", SceneCategory: "shoes"}
	wantDifferent[3].ImagePolicyContext = &imageagent.ImagePolicyContext{Country: "us", Family: "default", SceneCategory: "bags"}

	fingerprint := imageagent.SlotExecutionFingerprint(base)
	for _, changed := range wantDifferent {
		require.NotEqual(t, fingerprint, imageagent.SlotExecutionFingerprint(changed))
	}
}

func TestHistoricalSlotExecutionFingerprintPayloadOmitsAbsentPolicyFields(t *testing.T) {
	t.Parallel()

	payload, err := json.Marshal(imageagent.SlotExecutionInput{RunID: "historical-run"})
	require.NoError(t, err)
	require.NotContains(t, string(payload), "TargetPlatform")
	require.NotContains(t, string(payload), "ImagePolicyContext")
}

type countingPolicyCatalog struct {
	catalog imageagent.AssetCatalog
	calls   int
}

func (c *countingPolicyCatalog) Resolve(context.Context, imageagent.AssetCatalogScope) (imageagent.AssetCatalog, error) {
	c.calls++
	return c.catalog, nil
}

func validPolicyStartInput(runID string) imageagent.StartRunInput {
	return imageagent.StartRunInput{
		RunID: runID, BusinessTaskID: "task-1", TargetPlatform: "shein",
		ImagePolicyContext: imageagent.ImagePolicyContext{Country: "us", Family: "default", SceneCategory: "shoes"},
		Mode:               imageagent.RunModeManual, IdempotencyKey: runID + "-key", Plan: commandPlan(1),
	}
}

func withValidPolicy(input imageagent.StartRunInput) imageagent.StartRunInput {
	input.TargetPlatform = "shein"
	input.ImagePolicyContext = imageagent.ImagePolicyContext{Country: "us", Family: "default", SceneCategory: "shoes"}
	return input
}
