package store

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"task-processor/internal/imageagent"
)

func TestSlotExternalEffectRepositoryContract(t *testing.T) {
	tests := []struct {
		name string
		new  func(*testing.T) imageagent.Repository
	}{
		{name: "memory", new: func(*testing.T) imageagent.Repository { return NewMemoryRepository() }},
		{name: "gorm", new: func(t *testing.T) imageagent.Repository { return NewGormRepository(newConcurrentSQLite(t)) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repository := tt.new(t)
			scope := initializeSlotEffectRun(t, repository, "run-slot-effect-"+tt.name)
			effects := repository.(imageagent.SlotExternalEffectRepository)
			identity := imageagent.SlotExternalEffectIdentity{RunScope: scope, PlanRevision: 1, SlotID: "slot-1", Attempt: 1}
			reservation := imageagent.SlotExternalEffectReservation{Identity: identity, IdempotencyKey: "slot-key-1:plan:1:attempt:1", InputFingerprint: "input-fingerprint-1"}

			started, claimed, err := effects.ReserveSlotExternalEffect(context.Background(), reservation)
			require.NoError(t, err)
			require.True(t, claimed)
			require.Equal(t, imageagent.SlotExternalEffectProviderStarted, started.Phase)
			replayed, claimed, err := effects.ReserveSlotExternalEffect(context.Background(), reservation)
			require.NoError(t, err)
			require.False(t, claimed)
			require.Equal(t, started, replayed)

			conflict := reservation
			conflict.InputFingerprint = "different-input"
			_, _, err = effects.ReserveSlotExternalEffect(context.Background(), conflict)
			require.ErrorIs(t, err, imageagent.ErrRevisionConflict)

			generated := imageagent.SlotGeneratedOutput{SlotID: "slot-1", Attempt: 1, SourceAssetID: "source-1", Assets: []imageagent.GeneratedAsset{{URL: "C:/generated/scene.png", Width: 1200, Height: 1200, Metadata: map[string]string{"local_path": "C:/generated/scene.png", "lineage": "source-1"}}}}
			generatedState, err := effects.StoreSlotGeneratedOutput(context.Background(), reservation, generated)
			require.NoError(t, err)
			require.Equal(t, imageagent.SlotExternalEffectGeneratedComplete, generatedState.Phase)
			require.Equal(t, generated, generatedState.Generated)

			result := imageagent.SlotExecutionResult{SlotID: "slot-1", Attempt: 1, Candidates: []imageagent.AssetCandidate{{AssetID: "candidate-1", URL: "https://cdn.example.test/scene.png", SourceAssetID: "source-1"}}}
			published, err := effects.CompleteSlotPublication(context.Background(), reservation, result)
			require.NoError(t, err)
			require.Equal(t, imageagent.SlotExternalEffectPublicationComplete, published.Phase)
			require.Equal(t, result, published.Published)
			replayed, err = effects.GetSlotExternalEffect(context.Background(), identity)
			require.NoError(t, err)
			require.Equal(t, published, replayed)

			otherOwner := identity
			otherOwner.OwnerUserID = "user-b"
			_, err = effects.GetSlotExternalEffect(context.Background(), otherOwner)
			require.ErrorIs(t, err, imageagent.ErrRunNotFound)
		})
	}
}

func TestGormSlotExternalEffectConcurrentExactReservationHasOneProviderOwner(t *testing.T) {
	db := newConcurrentSQLite(t)
	repository := NewGormRepository(db)
	scope := initializeSlotEffectRun(t, repository, "run-slot-effect-concurrent")
	effects := repository.(imageagent.SlotExternalEffectRepository)
	reservation := imageagent.SlotExternalEffectReservation{
		Identity:       imageagent.SlotExternalEffectIdentity{RunScope: scope, PlanRevision: 1, SlotID: "slot-1", Attempt: 1},
		IdempotencyKey: "slot-key-1:plan:1:attempt:1", InputFingerprint: "input-fingerprint-1",
	}

	const callers = 8
	start := make(chan struct{})
	type outcome struct {
		claimed bool
		err     error
	}
	outcomes := make(chan outcome, callers)
	for range callers {
		go func() {
			<-start
			_, claimed, err := effects.ReserveSlotExternalEffect(context.Background(), reservation)
			outcomes <- outcome{claimed: claimed, err: err}
		}()
	}
	close(start)
	owners := 0
	for range callers {
		got := <-outcomes
		require.NoError(t, got.err)
		if got.claimed {
			owners++
		}
	}
	require.Equal(t, 1, owners)
}

func initializeSlotEffectRun(t *testing.T, repository imageagent.Repository, runID string) imageagent.RunScope {
	t.Helper()
	run := manualRun(runID, "tenant-a")
	run.BusinessTaskID = "task-" + runID
	plan := planRevision(1)
	scope := imageagent.ScopeForRun(*run)
	_, err := repository.InitializeRun(context.Background(), imageagent.ProjectionInitialization{
		Scope: scope, Run: *run, Plan: plan,
		Catalog:  imageagent.AssetCatalog{Assets: []imageagent.AuthorizedAsset{{ID: "source-1", Type: imageagent.AuthorizedAssetSource, URL: "https://source.example/source.png"}, {ID: "style-1", Type: imageagent.AuthorizedAssetStyle}}},
		Snapshot: imageagent.RunProjection{Run: *run, Plan: plan}, CommitID: fmt.Sprintf("start:%s", runID), EventType: "run.initialized", EventPayload: []byte(`{}`),
	})
	require.NoError(t, err)
	return scope
}
