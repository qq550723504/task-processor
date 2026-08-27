package temporal

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	sdkactivity "go.temporal.io/sdk/activity"
	sdktemporal "go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"task-processor/internal/imageagent"
	"task-processor/internal/imageagent/objectstore"
)

func TestExecuteSlotV3DoesNotRegenerateAnUnownedProviderClaim(t *testing.T) {
	repository, input := initializedSlotEffectV3Activity(t, "run-v3-unowned-provider")
	effects := repository.(imageagent.SlotExternalEffectV3Repository)
	_, claimed, err := effects.ReserveSlotProviderV3(context.Background(), v3Reservation(input))
	require.NoError(t, err)
	require.True(t, claimed)
	executor := &recordingStagedExecutor{}
	activities := newV3Activities(t, repository, effects, executor, &recordingArtifactStore{})

	_, err = activities.ExecuteSlotV3(context.Background(), input)
	requireV3ApplicationErrorType(t, err, slotProviderOutcomeUnknownCode)
	require.Zero(t, executor.GenerateCalls())

	stored, err := effects.GetSlotExternalEffectV3(context.Background(), v3Reservation(input).Identity)
	require.NoError(t, err)
	require.Equal(t, imageagent.SlotEffectV3ProviderUnknown, stored.Phase)
	require.Equal(t, slotProviderOutcomeUnknownCode, stored.BlockedCode)
}

func TestExecuteSlotV3ResumesPreparedStagingByHead(t *testing.T) {
	repository, input := initializedSlotEffectV3Activity(t, "run-v3-resume-staging")
	effects := repository.(imageagent.SlotExternalEffectV3Repository)
	manifest := v3StagingManifest(input, tinyPNGBytes(t))
	seedV3StagingPrepared(t, effects, input, manifest)
	executor := &recordingStagedExecutor{}
	artifacts := &recordingArtifactStore{}
	activities := newV3Activities(t, repository, effects, executor, artifacts)

	result, err := activities.ExecuteSlotV3(context.Background(), input)
	require.NoError(t, err)
	require.Zero(t, executor.GenerateCalls())
	require.Zero(t, artifacts.PrepareCalls())
	require.Equal(t, 1, artifacts.EnsureCalls())
	require.Equal(t, manifest, artifacts.EnsuredManifest())
	require.Len(t, result.Candidates, 1)

	stored, err := effects.GetSlotExternalEffectV3(context.Background(), v3Reservation(input).Identity)
	require.NoError(t, err)
	require.Equal(t, imageagent.SlotEffectV3PublicationComplete, stored.Phase)
}

func TestExecuteSlotV3CompletesAfterOriginalLocalFilesDisappear(t *testing.T) {
	repository, input := initializedSlotEffectV3Activity(t, "run-v3-local-file-loss")
	effects := repository.(imageagent.SlotExternalEffectV3Repository)
	path := writeTinyPNG(t)
	executor := &recordingStagedExecutor{generated: generatedV3Output(input, path)}
	artifacts := &recordingArtifactStore{ensureErrors: []error{errors.New("object store temporarily unavailable"), nil}}
	activities := newV3Activities(t, repository, effects, executor, artifacts)

	_, err := activities.ExecuteSlotV3(context.Background(), input)
	require.ErrorContains(t, err, "ensure staged artifacts")
	stored, err := effects.GetSlotExternalEffectV3(context.Background(), v3Reservation(input).Identity)
	require.NoError(t, err)
	require.Equal(t, imageagent.SlotEffectV3StagingPrepared, stored.Phase)
	require.NoError(t, os.Remove(path))

	result, err := activities.ExecuteSlotV3(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, 1, executor.GenerateCalls())
	require.Equal(t, 1, artifacts.PrepareCalls())
	require.Equal(t, 2, artifacts.EnsureCalls())
	require.Len(t, result.Candidates, 1)
}

func TestExecuteSlotV3RecoversLostTransitionResponses(t *testing.T) {
	repository, input := initializedSlotEffectV3Activity(t, "run-v3-lost-transition-responses")
	baseEffects := repository.(imageagent.SlotExternalEffectV3Repository)
	effects := &lostResponseV3Repository{SlotExternalEffectV3Repository: baseEffects, loseStagedCommit: true, loseCompletion: true}
	path := writeTinyPNG(t)
	executor := &recordingStagedExecutor{generated: generatedV3Output(input, path)}
	artifacts := &recordingArtifactStore{}
	activities := newV3Activities(t, repository, effects, executor, artifacts)

	result, err := activities.ExecuteSlotV3(context.Background(), input)
	require.NoError(t, err)
	require.Len(t, result.Candidates, 1)
	require.Equal(t, 1, executor.GenerateCalls())
	require.Equal(t, 1, artifacts.EnsureCalls())
	require.Equal(t, 1, artifacts.FinalizeCalls())
	require.GreaterOrEqual(t, effects.RenewCalls(), 2)

	stored, err := baseEffects.GetSlotExternalEffectV3(context.Background(), v3Reservation(input).Identity)
	require.NoError(t, err)
	require.Equal(t, imageagent.SlotEffectV3PublicationComplete, stored.Phase)
}

func TestExecuteSlotV3RecoversLostStagingPreparationResponse(t *testing.T) {
	repository, input := initializedSlotEffectV3Activity(t, "run-v3-lost-staging-preparation")
	baseEffects := repository.(imageagent.SlotExternalEffectV3Repository)
	effects := &lostResponseV3Repository{SlotExternalEffectV3Repository: baseEffects, losePreparation: true}
	path := writeTinyPNG(t)
	executor := &recordingStagedExecutor{generated: generatedV3Output(input, path)}
	artifacts := &recordingArtifactStore{}
	activities := newV3Activities(t, repository, effects, executor, artifacts)

	result, err := activities.ExecuteSlotV3(context.Background(), input)
	require.NoError(t, err)
	require.Len(t, result.Candidates, 1)
	require.Equal(t, 1, executor.GenerateCalls())
	require.Equal(t, 1, artifacts.PrepareCalls())
	require.Equal(t, 1, artifacts.EnsureCalls())
}

func TestExecuteSlotV3RecoversLostPublicationClaimResponse(t *testing.T) {
	repository, input := initializedSlotEffectV3Activity(t, "run-v3-lost-claim-response")
	baseEffects := repository.(imageagent.SlotExternalEffectV3Repository)
	seedV3ArtifactStaged(t, baseEffects, input, v3StagingManifest(input, tinyPNGBytes(t)))
	effects := &lostResponseV3Repository{SlotExternalEffectV3Repository: baseEffects, loseClaim: true}
	artifacts := &recordingArtifactStore{}
	activities := newV3Activities(t, repository, effects, &recordingStagedExecutor{}, artifacts)
	result, err := activities.ExecuteSlotV3(context.Background(), input)
	require.NoError(t, err)
	require.Len(t, result.Candidates, 1)
	require.Equal(t, 1, artifacts.FinalizeCalls())
	stored, err := baseEffects.GetSlotExternalEffectV3(context.Background(), v3Reservation(input).Identity)
	require.NoError(t, err)
	require.EqualValues(t, 1, stored.Publication.Fence)
}

func TestExecuteSlotV3FencesLatePublicationOwner(t *testing.T) {
	repository, input := initializedSlotEffectV3Activity(t, "run-v3-stale-publication")
	effects := repository.(imageagent.SlotExternalEffectV3Repository)
	manifest := v3StagingManifest(input, tinyPNGBytes(t))
	seedV3ArtifactStaged(t, effects, input, manifest)
	final := v3FinalManifest(manifest)
	publicationFingerprint, err := imageagent.FinalManifestFingerprint(final)
	require.NoError(t, err)
	artifacts := &recordingArtifactStore{}
	artifacts.onFinalize = func() {
		time.Sleep(40 * time.Millisecond)
		_, successor, claimed, claimErr := effects.ClaimSlotPublicationV3(context.Background(), imageagent.PublicationClaimRequest{
			Reservation: v3Reservation(input), Owner: "workflow-run/activity/2", LeaseDuration: time.Minute,
			PublicationFingerprint: publicationFingerprint, FinalManifest: final,
		})
		require.NoError(t, claimErr)
		require.True(t, claimed)
		require.EqualValues(t, 2, successor.Fence)
	}
	executor := &recordingStagedExecutor{}
	activities := newV3Activities(t, repository, effects, executor, artifacts)
	activities.publicationLeaseDuration = 20 * time.Millisecond

	_, err = activities.ExecuteSlotV3(context.Background(), input)
	require.ErrorIs(t, err, imageagent.ErrRevisionConflict)
	require.Equal(t, 1, artifacts.FinalizeCalls(), "a stale owner may finish deterministic reconciliation")
	require.Zero(t, executor.BuildCalls(), "stale owner must stop before committing a result")

	stored, err := effects.GetSlotExternalEffectV3(context.Background(), v3Reservation(input).Identity)
	require.NoError(t, err)
	require.Equal(t, imageagent.SlotEffectV3PublicationClaimed, stored.Phase)
	require.Equal(t, "workflow-run/activity/2", stored.Publication.Owner)
	require.EqualValues(t, 2, stored.Publication.Fence)
	require.Empty(t, stored.Published.Candidates)
}

func TestExecuteSlotV3BlocksUnreconcilablePublication(t *testing.T) {
	repository, input := initializedSlotEffectV3Activity(t, "run-v3-publication-unknown")
	effects := repository.(imageagent.SlotExternalEffectV3Repository)
	seedV3ArtifactStaged(t, effects, input, v3StagingManifest(input, tinyPNGBytes(t)))
	artifacts := &recordingArtifactStore{finalizeError: objectstore.ErrObjectConflict}
	activities := newV3Activities(t, repository, effects, &recordingStagedExecutor{}, artifacts)

	_, err := activities.ExecuteSlotV3(context.Background(), input)
	requireV3ApplicationErrorType(t, err, slotPublicationOutcomeUnknownCode)
	stored, err := effects.GetSlotExternalEffectV3(context.Background(), v3Reservation(input).Identity)
	require.NoError(t, err)
	require.Equal(t, imageagent.SlotEffectV3PublicationUnknown, stored.Phase)
	require.Equal(t, slotPublicationOutcomeUnknownCode, stored.BlockedCode)

	_, err = activities.ExecuteSlotV3(context.Background(), input)
	requireV3ApplicationErrorType(t, err, slotPublicationOutcomeUnknownCode)
	require.Equal(t, 1, artifacts.FinalizeCalls(), "publication unknown must not expose blind activity retry")
}

func TestExecuteSlotV3BlocksMissingPreparedBytesWithoutRegeneration(t *testing.T) {
	repository, input := initializedSlotEffectV3Activity(t, "run-v3-staging-unknown")
	effects := repository.(imageagent.SlotExternalEffectV3Repository)
	seedV3StagingPrepared(t, effects, input, v3StagingManifest(input, tinyPNGBytes(t)))
	executor := &recordingStagedExecutor{}
	artifacts := &recordingArtifactStore{ensureErrors: []error{objectstore.ErrArtifactUnavailable}}
	activities := newV3Activities(t, repository, effects, executor, artifacts)

	_, err := activities.ExecuteSlotV3(context.Background(), input)
	requireV3ApplicationErrorType(t, err, slotStagingOutcomeUnknownCode)
	require.Zero(t, executor.GenerateCalls())
	stored, err := effects.GetSlotExternalEffectV3(context.Background(), v3Reservation(input).Identity)
	require.NoError(t, err)
	require.Equal(t, imageagent.SlotEffectV3StagingUnknown, stored.Phase)
}

func TestExecuteSlotV3ReturnsStoredExactResultWithoutEffects(t *testing.T) {
	repository, input := initializedSlotEffectV3Activity(t, "run-v3-exact-replay")
	effects := repository.(imageagent.SlotExternalEffectV3Repository)
	path := writeTinyPNG(t)
	executor := &recordingStagedExecutor{generated: generatedV3Output(input, path)}
	artifacts := &recordingArtifactStore{}
	activities := newV3Activities(t, repository, effects, executor, artifacts)

	first, err := activities.ExecuteSlotV3(context.Background(), input)
	require.NoError(t, err)
	second, err := activities.ExecuteSlotV3(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, first, second)
	require.Equal(t, 1, executor.GenerateCalls())
	require.Equal(t, 1, executor.BuildCalls())
	require.Equal(t, 1, artifacts.FinalizeCalls())
}

func TestExecuteSlotV3ResultPreservesDurableIdentityOnTheV3Wire(t *testing.T) {
	repository, input := initializedSlotEffectV3Activity(t, "run-v3-durable-result-wire")
	path := writeTinyPNG(t)
	activities := newV3Activities(t, repository, repository.(imageagent.SlotExternalEffectV3Repository), &recordingStagedExecutor{generated: generatedV3Output(input, path)}, &recordingArtifactStore{})

	result, err := activities.ExecuteSlotV3(context.Background(), input)
	require.NoError(t, err)
	encoded, err := json.Marshal(result)
	require.NoError(t, err)
	require.Contains(t, string(encoded), `"durable_asset"`)
	require.Contains(t, string(encoded), `"object_key"`)
	require.Contains(t, string(encoded), `"sha256"`)
}

func TestExecuteSlotV3PublicationOwnerIncludesTemporalRunActivityAndAttempt(t *testing.T) {
	owner, err := publicationOwnerFromActivityInfo(sdkactivity.Info{
		WorkflowExecution: workflow.Execution{RunID: "workflow-run-7"}, ActivityID: "activity-12", Attempt: 3,
	})
	require.NoError(t, err)
	require.Equal(t, "workflow-run-7/activity-12/3", owner)
}

func TestExecuteSlotV3FailsClosedForUnknownPersistedPhase(t *testing.T) {
	repository, input := initializedSlotEffectV3Activity(t, "run-v3-unknown-phase")
	baseEffects := repository.(imageagent.SlotExternalEffectV3Repository)
	effects := &phaseOverrideV3Repository{SlotExternalEffectV3Repository: baseEffects, phase: "future_phase"}
	activities := newV3Activities(t, repository, effects, &recordingStagedExecutor{}, &recordingArtifactStore{})

	_, err := activities.ExecuteSlotV3(context.Background(), input)
	requireV3ApplicationErrorType(t, err, slotEffectPhaseInvalidCode)
}

func initializedSlotEffectV3Activity(t *testing.T, runID string) (imageagent.Repository, ExecuteSlotV3ActivityInput) {
	t.Helper()
	repository, legacy := initializedSlotEffectActivity(t, runID)
	return repository, ExecuteSlotV3ActivityInput{
		RunID: legacy.RunID, Identity: legacy.Identity, PlanRevision: legacy.PlanRevision, Slot: legacy.Slot,
		Attempt: legacy.Attempt, IdempotencyKey: legacy.IdempotencyKey, AssetCatalog: legacy.AssetCatalog,
	}
}

func newV3Activities(t *testing.T, repository imageagent.Repository, effects imageagent.SlotExternalEffectV3Repository, executor *recordingStagedExecutor, artifacts DurableArtifactStore) *Activities {
	t.Helper()
	activities, err := NewActivities(ActivityDependencies{
		Repository: repository, SlotEffects: repository.(imageagent.SlotExternalEffectRepository), SlotExecutor: executor,
		SlotEffectsV3: effects, StagedSlotExecutor: executor, ArtifactStore: artifacts,
		Publisher: &identityCheckingPublisher{t: t}, PublicationOwner: func(context.Context) (string, error) { return "workflow-run/activity/1", nil },
	})
	require.NoError(t, err)
	return activities
}

func v3Reservation(input ExecuteSlotV3ActivityInput) imageagent.SlotEffectV3Reservation {
	execution := imageagent.SlotExecutionInput{
		RunID: input.RunID, TenantID: input.Identity.TenantID, UserID: input.Identity.UserID,
		PlanRevision: input.PlanRevision, Slot: input.Slot, Attempt: input.Attempt,
		IdempotencyKey: input.IdempotencyKey, AssetCatalog: input.AssetCatalog,
	}
	return imageagent.SlotEffectV3Reservation{
		Identity:       imageagent.SlotExternalEffectIdentity{RunScope: imageagent.RunScope{TenantID: input.Identity.TenantID, OwnerUserID: input.Identity.UserID, RunID: input.RunID}, PlanRevision: input.PlanRevision, SlotID: input.Slot.ID, Attempt: input.Attempt},
		IdempotencyKey: input.IdempotencyKey, InputFingerprint: imageagent.SlotExecutionFingerprint(execution),
	}
}

func seedV3StagingPrepared(t *testing.T, effects imageagent.SlotExternalEffectV3Repository, input ExecuteSlotV3ActivityInput, manifest imageagent.StagingManifest) {
	t.Helper()
	_, claimed, err := effects.ReserveSlotProviderV3(context.Background(), v3Reservation(input))
	require.NoError(t, err)
	require.True(t, claimed)
	_, err = effects.PrepareSlotStagingV3(context.Background(), v3Reservation(input), manifest)
	require.NoError(t, err)
}

func seedV3ArtifactStaged(t *testing.T, effects imageagent.SlotExternalEffectV3Repository, input ExecuteSlotV3ActivityInput, manifest imageagent.StagingManifest) {
	t.Helper()
	seedV3StagingPrepared(t, effects, input, manifest)
	fingerprint, err := imageagent.StagingManifestFingerprint(manifest)
	require.NoError(t, err)
	_, err = effects.CommitSlotStagedV3(context.Background(), v3Reservation(input), fingerprint)
	require.NoError(t, err)
}

func generatedV3Output(input ExecuteSlotV3ActivityInput, path string) imageagent.SlotGeneratedOutput {
	return imageagent.SlotGeneratedOutput{
		SlotID: input.Slot.ID, Attempt: input.Attempt, SourceAssetID: input.Slot.SourceAssetIDs[0],
		Assets: []imageagent.GeneratedAsset{{URL: path, Type: "gallery_image", Width: 1, Height: 1, Operations: []string{"render_scene_model"}, Metadata: map[string]string{"local_path": path, "authorization": "must-not-persist"}}},
	}
}

func writeTinyPNG(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "generated.png")
	require.NoError(t, os.WriteFile(path, tinyPNGBytes(t), 0o600))
	return path
}

func tinyPNGBytes(t *testing.T) []byte {
	t.Helper()
	var data strings.Builder
	writer := &stringWriter{builder: &data}
	require.NoError(t, png.Encode(writer, image.NewNRGBA(image.Rect(0, 0, 1, 1))))
	return []byte(data.String())
}

type stringWriter struct{ builder *strings.Builder }

func (w *stringWriter) Write(data []byte) (int, error) { return w.builder.WriteString(string(data)) }

func v3StagingManifest(input ExecuteSlotV3ActivityInput, data []byte) imageagent.StagingManifest {
	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:])
	return imageagent.StagingManifest{Assets: []imageagent.StagedAssetRef{{
		ObjectKey: fmt.Sprintf("image-agent/staging/%s/%s/%d/%s/%d/0-%s.png", input.Identity.TenantID, input.RunID, input.PlanRevision, input.Slot.ID, input.Attempt, hash),
		SHA256:    hash, SizeBytes: int64(len(data)), ContentType: "image/png", Width: 1, Height: 1,
		SourceAssetID: input.Slot.SourceAssetIDs[0], Operations: []string{"render_scene_model"},
	}}}
}

func v3FinalManifest(staging imageagent.StagingManifest) imageagent.FinalManifest {
	assets := make([]imageagent.PublishedAssetRef, len(staging.Assets))
	for index, asset := range staging.Assets {
		assets[index] = imageagent.PublishedAssetRef{
			ObjectKey: strings.Replace(asset.ObjectKey, "image-agent/staging/", "image-agent/public/", 1),
			SHA256:    asset.SHA256, SizeBytes: asset.SizeBytes, ContentType: asset.ContentType, Width: asset.Width, Height: asset.Height,
			SourceAssetID: asset.SourceAssetID, Operations: append([]string(nil), asset.Operations...), ProviderReceiptID: asset.ProviderReceiptID,
		}
	}
	return imageagent.FinalManifest{Assets: assets}
}

func requireV3ApplicationErrorType(t *testing.T, err error, want string) {
	t.Helper()
	var applicationError *sdktemporal.ApplicationError
	require.ErrorAs(t, err, &applicationError)
	require.Equal(t, want, applicationError.Type())
	require.True(t, applicationError.NonRetryable())
}

type recordingStagedExecutor struct {
	mu            sync.Mutex
	generated     imageagent.SlotGeneratedOutput
	generateCalls int
	buildCalls    int
}

func (e *recordingStagedExecutor) GenerateSlot(_ context.Context, input imageagent.SlotExecutionInput) (imageagent.SlotGeneratedOutput, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.generateCalls++
	if e.generated.SlotID == "" {
		return imageagent.SlotGeneratedOutput{}, errors.New("generated output fixture is required")
	}
	return e.generated, nil
}

func (e *recordingStagedExecutor) BuildSlotResult(_ context.Context, input imageagent.SlotExecutionInput, published imageagent.PublishedSlotOutput) (imageagent.SlotExecutionResult, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.buildCalls++
	candidates := make([]imageagent.AssetCandidate, len(published.Assets))
	for index, asset := range published.Assets {
		candidates[index] = imageagent.AssetCandidate{AssetID: fmt.Sprintf("candidate-%d", index), SourceAssetID: asset.SourceAssetID, DurableAsset: imageagent.DurableAssetIdentity{ObjectKey: asset.ObjectKey, SHA256: asset.SHA256}}
	}
	return imageagent.SlotExecutionResult{SlotID: input.Slot.ID, Attempt: input.Attempt, Candidates: candidates}, nil
}

func (e *recordingStagedExecutor) PublishSlot(context.Context, imageagent.SlotExecutionInput, imageagent.SlotGeneratedOutput) (imageagent.SlotExecutionResult, error) {
	return imageagent.SlotExecutionResult{}, errors.New("v2 publication must not be used by ExecuteSlotV3")
}

func (e *recordingStagedExecutor) GenerateCalls() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.generateCalls
}
func (e *recordingStagedExecutor) BuildCalls() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.buildCalls
}

type recordingArtifactStore struct {
	mu            sync.Mutex
	prepareCalls  int
	ensureCalls   int
	finalizeCalls int
	ensureErrors  []error
	finalizeError error
	prepared      imageagent.StagingManifest
	ensured       imageagent.StagingManifest
	onFinalize    func()
}

func (s *recordingArtifactStore) PrepareSlotArtifacts(input objectstore.PrepareSlotArtifactsInput) (objectstore.PreparedSlotArtifacts, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.prepareCalls++
	if len(input.Assets) != 1 || len(input.Assets[0].Bytes) == 0 {
		return objectstore.PreparedSlotArtifacts{}, imageagent.ErrValidation
	}
	sum := sha256.Sum256(input.Assets[0].Bytes)
	hash := hex.EncodeToString(sum[:])
	s.prepared = imageagent.StagingManifest{Assets: []imageagent.StagedAssetRef{{
		ObjectKey: fmt.Sprintf("image-agent/staging/%s/%s/%d/%s/%d/0-%s.png", input.Identity.TenantID, input.Identity.RunID, input.Identity.PlanRevision, input.Identity.SlotID, input.Identity.Attempt, hash),
		SHA256:    hash, SizeBytes: int64(len(input.Assets[0].Bytes)), ContentType: input.Assets[0].ContentType,
		Width: input.Assets[0].Width, Height: input.Assets[0].Height, SourceAssetID: input.Assets[0].SourceAssetID,
		Operations: append([]string(nil), input.Assets[0].Operations...), ProviderReceiptID: input.Assets[0].ProviderReceiptID,
	}}}
	return objectstore.PreparedSlotArtifacts{Manifest: s.prepared}, nil
}

func (s *recordingArtifactStore) EnsureStaged(_ context.Context, prepared objectstore.PreparedSlotArtifacts) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensured = prepared.Manifest
	index := s.ensureCalls
	s.ensureCalls++
	if index < len(s.ensureErrors) {
		return s.ensureErrors[index]
	}
	return nil
}

func (s *recordingArtifactStore) Finalize(_ context.Context, manifest imageagent.StagingManifest) (imageagent.FinalManifest, error) {
	s.mu.Lock()
	s.finalizeCalls++
	err := s.finalizeError
	hook := s.onFinalize
	s.mu.Unlock()
	if hook != nil {
		hook()
	}
	if err != nil {
		return imageagent.FinalManifest{}, err
	}
	return v3FinalManifest(manifest), nil
}

func (s *recordingArtifactStore) PrepareCalls() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.prepareCalls
}
func (s *recordingArtifactStore) EnsureCalls() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ensureCalls
}
func (s *recordingArtifactStore) FinalizeCalls() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.finalizeCalls
}
func (s *recordingArtifactStore) EnsuredManifest() imageagent.StagingManifest {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ensured
}

type lostResponseV3Repository struct {
	imageagent.SlotExternalEffectV3Repository
	mu               sync.Mutex
	losePreparation  bool
	loseStagedCommit bool
	loseClaim        bool
	loseCompletion   bool
	renewCalls       int
}

func (r *lostResponseV3Repository) PrepareSlotStagingV3(ctx context.Context, reservation imageagent.SlotEffectV3Reservation, manifest imageagent.StagingManifest) (imageagent.SlotEffectV3Attempt, error) {
	result, err := r.SlotExternalEffectV3Repository.PrepareSlotStagingV3(ctx, reservation, manifest)
	r.mu.Lock()
	defer r.mu.Unlock()
	if err == nil && r.losePreparation {
		r.losePreparation = false
		return imageagent.SlotEffectV3Attempt{}, errors.New("lost staging preparation response")
	}
	return result, err
}

func (r *lostResponseV3Repository) ClaimSlotPublicationV3(ctx context.Context, request imageagent.PublicationClaimRequest) (imageagent.SlotEffectV3Attempt, imageagent.PublicationClaim, bool, error) {
	result, claim, claimed, err := r.SlotExternalEffectV3Repository.ClaimSlotPublicationV3(ctx, request)
	r.mu.Lock()
	defer r.mu.Unlock()
	if err == nil && r.loseClaim {
		r.loseClaim = false
		return imageagent.SlotEffectV3Attempt{}, imageagent.PublicationClaim{}, false, errors.New("lost publication claim response")
	}
	return result, claim, claimed, err
}

type phaseOverrideV3Repository struct {
	imageagent.SlotExternalEffectV3Repository
	phase imageagent.SlotEffectV3Phase
}

func (r *phaseOverrideV3Repository) ReserveSlotProviderV3(ctx context.Context, reservation imageagent.SlotEffectV3Reservation) (imageagent.SlotEffectV3Attempt, bool, error) {
	attempt, _, err := r.SlotExternalEffectV3Repository.ReserveSlotProviderV3(ctx, reservation)
	attempt.Phase = r.phase
	return attempt, false, err
}

func (r *lostResponseV3Repository) CommitSlotStagedV3(ctx context.Context, reservation imageagent.SlotEffectV3Reservation, fingerprint string) (imageagent.SlotEffectV3Attempt, error) {
	result, err := r.SlotExternalEffectV3Repository.CommitSlotStagedV3(ctx, reservation, fingerprint)
	r.mu.Lock()
	defer r.mu.Unlock()
	if err == nil && r.loseStagedCommit {
		r.loseStagedCommit = false
		return imageagent.SlotEffectV3Attempt{}, errors.New("lost staged commit response")
	}
	return result, err
}

func (r *lostResponseV3Repository) RenewSlotPublicationV3(ctx context.Context, renewal imageagent.PublicationLeaseRenewal) (imageagent.PublicationClaim, error) {
	r.mu.Lock()
	r.renewCalls++
	r.mu.Unlock()
	return r.SlotExternalEffectV3Repository.RenewSlotPublicationV3(ctx, renewal)
}

func (r *lostResponseV3Repository) CompleteSlotPublicationV3(ctx context.Context, completion imageagent.PublicationCompletion) (imageagent.SlotEffectV3Attempt, error) {
	result, err := r.SlotExternalEffectV3Repository.CompleteSlotPublicationV3(ctx, completion)
	r.mu.Lock()
	defer r.mu.Unlock()
	if err == nil && r.loseCompletion {
		r.loseCompletion = false
		return imageagent.SlotEffectV3Attempt{}, errors.New("lost publication completion response")
	}
	return result, err
}

func (r *lostResponseV3Repository) RenewCalls() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.renewCalls
}

var _ imageagent.StagedSlotExecutor = (*recordingStagedExecutor)(nil)
var _ imageagent.RecoverableSlotExecutor = (*recordingStagedExecutor)(nil)
var _ DurableArtifactStore = (*recordingArtifactStore)(nil)
