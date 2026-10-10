package temporal

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"task-processor/internal/imageagent"
	"task-processor/internal/imageagent/assetpublication"
	assetstore "task-processor/internal/integration/persistence/product/asset"
	productasset "task-processor/internal/product/asset"
)

type approvalRecoveryAuthorization struct {
	revoked     bool
	calls       int
	onAuthorize func()
}

func (a *approvalRecoveryAuthorization) AuthorizeExecution(context.Context, imageagent.ExecutionIdentity) error {
	a.calls++
	if a.revoked {
		return imageagent.ErrIdentityRequired
	}
	if a.onAuthorize != nil {
		a.onAuthorize()
	}
	return nil
}

type approvalRecoverySources struct {
	identity *approvalRecoveryAuthorization
	source   productasset.SourceSelection
	reads    int
}

func (s *approvalRecoverySources) ReadSourceSelection(context.Context, productasset.SourceSelectionRequest) (productasset.SourceSelection, error) {
	s.reads++
	if s.identity.revoked {
		return productasset.SourceSelection{}, productasset.ErrSourceApprovalForbidden
	}
	return s.source, nil
}

type approvalRecoveryTargets struct{}

func (approvalRecoveryTargets) ResolveImageSetTarget(context.Context, productasset.SourceSelection, *productasset.ImageSetTarget, []productasset.ApprovedAsset) (productasset.ImageSetTargetResolution, error) {
	return productasset.ImageSetTargetResolution{}, nil
}

type approvalRecoveryCandidates struct{}

func (approvalRecoveryCandidates) ReadImageSetCandidate(context.Context, productasset.SourceSelection, productasset.ImageSetResultBinding, productasset.ImageSetChoice) (productasset.ImageSetCandidate, error) {
	return productasset.ImageSetCandidate{}, productasset.ErrApprovalConflict
}

type lostApprovalACKSelector struct {
	service *productasset.ImageSetService
	calls   int
}

func (s *lostApprovalACKSelector) Select(ctx context.Context, command productasset.ImageSetCommand) (productasset.ApprovalReceipt, error) {
	s.calls++
	receipt, err := s.service.Select(ctx, command)
	if err != nil {
		return receipt, err
	}
	return productasset.ApprovalReceipt{}, context.DeadlineExceeded
}

type approvalRecoveryFixture struct {
	activities *Activities
	assets     productasset.Repository
	auth       *approvalRecoveryAuthorization
	sources    *approvalRecoverySources
	selector   *lostApprovalACKSelector
	publish    PublishImageSetActivityInput
	complete   PersistRunStateActivityInput
}

func newApprovalRecoveryFixture(t *testing.T) approvalRecoveryFixture {
	return newApprovalRecoveryFixtureAtPhase(t, imageagent.ImageSetApprovalPublicationStarted)
}

func newApprovalRecoveryFixtureAtPhase(t *testing.T, phase string) approvalRecoveryFixture {
	t.Helper()
	a, repo, execution, _, _, _ := persistedAcceptedImageSetFixture(t)
	auth := &approvalRecoveryAuthorization{}
	a.executionAuthorizer = auth
	current, err := repo.GetProjection(context.Background(), imageagent.RunScope{TenantID: execution.Identity.TenantID, OwnerUserID: execution.Identity.UserID, RunID: execution.RunID})
	require.NoError(t, err)
	digest, err := imageagent.ImageSetResultDigest(current.Plan, current.Slots, nil)
	require.NoError(t, err)
	result := WorkflowResult{Status: imageagent.RunStatusAwaitingFinalApproval, Plan: current.Plan, Slots: current.Slots, ResultDigest: digest}
	require.NoError(t, a.PersistRunState(context.Background(), PersistRunStateActivityInput{RunID: execution.RunID, Identity: execution.Identity, PlanRevision: 1, Projection: result, CurrentNode: "approve_results", CommitID: "awaiting-approval"}))
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "asset.db")), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, assetstore.AutoMigrate(db))
	pool, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = pool.Close() })
	assets, err := assetstore.NewRepository(db)
	require.NoError(t, err)
	source := current.Plan.Set.Source
	imageURL := "https://images.example.org/source.png"
	sources := &approvalRecoverySources{identity: auth, source: productasset.SourceSelection{ContextKind: string(source.ContextKind), TenantID: current.Run.TenantID, ActorID: current.Run.UserID, MemberID: current.Run.MemberID, ProductKey: source.ProductID, ItemID: source.OperationID, OriginalPublicationID: source.OriginalPublicationID, OriginalSnapshotVersion: source.OriginalVersion, EffectiveCatalogVersion: source.EffectiveVersion, TargetPlatform: "product", Images: []productasset.SourceImage{{ID: "source-1", URL: imageURL, ReferenceHash: productasset.ReferenceHash("source-1", imageURL), Width: 1024, Height: 1024}}}}
	service, err := productasset.NewImageSetService(sources, assets, assets.(productasset.ImageSetInventoryReader), assets.(productasset.ApprovalCommitReader), approvalRecoveryCandidates{}, approvalRecoveryTargets{})
	require.NoError(t, err)
	selection := productasset.ImageSetCommand{ActionID: "1b912e40-d50a-48f5-a12b-62c73a42e4b8", ApprovingResult: productasset.ImageSetResultBinding{RunID: current.Run.ID, PlanRevision: 1, ResultDigest: digest}, Source: productasset.SourceSelectionRequest{ContextKind: string(source.ContextKind), ItemID: source.OperationID, OriginalPublicationID: source.OriginalPublicationID, OriginalSnapshotVersion: source.OriginalVersion, EffectiveCatalogVersion: source.EffectiveVersion, TargetPlatform: "product"}, Choices: []productasset.ImageSetChoice{{Kind: "source", SourceID: "source-1", Presentation: productasset.ImagePresentation{Group: "detail", Order: 1}}}}
	preview, err := service.Preview(context.Background(), selection)
	require.NoError(t, err)
	selection.SelectionDigest = preview.Digest
	pending := &imageagent.PendingCommandReceipt{ActionID: selection.ActionID, Kind: "approve_results", Phase: phase, Status: "pending", PlanRevision: 1, Attempt: 1, SelectionDigest: selection.SelectionDigest, ResultDigest: digest}
	require.NoError(t, a.PersistPendingCommand(context.Background(), PersistPendingCommandActivityInput{RunID: current.Run.ID, Identity: execution.Identity, Receipt: pending, CommitID: "approval-publish-started"}))
	selector := &lostApprovalACKSelector{service: service}
	publisher, err := assetpublication.NewImageSetPublisher(repo, selector)
	require.NoError(t, err)
	a.imageSetPublisher = publisher
	a.imageSetApprovals = assets.(productasset.ApprovalCommitReader)
	result.Status = imageagent.RunStatusCompleted
	return approvalRecoveryFixture{activities: a, assets: assets, auth: auth, sources: sources, selector: selector, publish: PublishImageSetActivityInput{RunID: current.Run.ID, Identity: execution.Identity, PlanRevision: 1, ResultDigest: digest, Selection: selection}, complete: PersistRunStateActivityInput{RunID: current.Run.ID, Identity: execution.Identity, PlanRevision: 1, Projection: result, CurrentNode: "complete", CommitID: "complete-original-approval"}}
}

func TestImageSetFirstPublicationMarkerRequiresLiveAuthorization(t *testing.T) {
	for _, revoked := range []bool{false, true} {
		t.Run(map[bool]string{false: "authorized", true: "revoked"}[revoked], func(t *testing.T) {
			f := newApprovalRecoveryFixtureAtPhase(t, string(updatePhaseApprovalPublish))
			scope := imageagent.RunScope{TenantID: f.publish.Identity.TenantID, OwnerUserID: f.publish.Identity.UserID, RunID: f.publish.RunID}
			current, err := f.activities.repository.GetProjection(context.Background(), scope)
			require.NoError(t, err)
			pending := clonePendingReceipt(current.PendingCommand)
			pending.Phase = imageagent.ImageSetApprovalPublicationStarted
			reads := 0
			f.activities.imageSetApprovals = approvalRecoveryReceiptReader{read: func(context.Context, string, string) (productasset.ApprovalCommit, error) {
				reads++
				return productasset.ApprovalCommit{}, productasset.ErrRepositoryUnavailable
			}}
			f.auth.revoked = revoked
			authorizations := f.auth.calls
			err = f.activities.PersistPendingCommand(context.Background(), PersistPendingCommandActivityInput{RunID: f.publish.RunID, Identity: f.publish.Identity, Receipt: pending, CommandIngress: current.CommandIngress, CommitID: "first-publication-marker"})
			if revoked {
				require.ErrorIs(t, err, imageagent.ErrIdentityRequired)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, authorizations+1, f.auth.calls, "the first publication marker requires live IAM")
			require.Zero(t, reads, "before the publication marker there is no committed publication to recover")
			require.Zero(t, f.selector.calls, "persisting the boundary must not call Asset")
			retained, err := f.activities.repository.GetProjection(context.Background(), scope)
			require.NoError(t, err)
			if revoked {
				require.Equal(t, current, retained)
			} else {
				require.Equal(t, pending, retained.PendingCommand)
				require.Equal(t, current.CommandIngress, retained.CommandIngress)
				f.activities.imageSetApprovals = f.assets.(productasset.ApprovalCommitReader)
				f.loseCommittedACK(t)
				require.NoError(t, f.activities.PublishApprovedImageSet(context.Background(), f.publish))
				require.NoError(t, f.activities.PersistRunState(context.Background(), f.complete))
				require.Equal(t, 1, f.selector.calls, "the first marker and later receipt recovery form one publication")
			}
		})
	}
}

func TestImageSetFirstPublicationMarkerCannotReplacePendingApproval(t *testing.T) {
	for _, changed := range []string{"action", "selection", "ingress"} {
		t.Run(changed, func(t *testing.T) {
			f := newApprovalRecoveryFixtureAtPhase(t, string(updatePhaseApprovalPublish))
			scope := imageagent.RunScope{TenantID: f.publish.Identity.TenantID, OwnerUserID: f.publish.Identity.UserID, RunID: f.publish.RunID}
			current, err := f.activities.repository.GetProjection(context.Background(), scope)
			require.NoError(t, err)
			pending := clonePendingReceipt(current.PendingCommand)
			pending.Phase = imageagent.ImageSetApprovalPublicationStarted
			input := PersistPendingCommandActivityInput{RunID: f.publish.RunID, Identity: f.publish.Identity, Receipt: pending, CommandIngress: current.CommandIngress, CommitID: "changed-first-publication"}
			switch changed {
			case "action":
				input.Receipt.ActionID = "ca1aa25c-3661-4c80-8c85-e3fe93220f8f"
			case "selection":
				input.Receipt.SelectionDigest = strings.Repeat("c", 64)
			case "ingress":
				input.CommandIngress.Used++
			}
			require.ErrorIs(t, f.activities.PersistPendingCommand(context.Background(), input), imageagent.ErrCommandBlocked)
			retained, err := f.activities.repository.GetProjection(context.Background(), scope)
			require.NoError(t, err)
			require.Equal(t, current, retained)
			require.Zero(t, f.selector.calls)
		})
	}
}

func TestImageSetFirstPublicationMarkerUsesCheckedProjectionCAS(t *testing.T) {
	for _, changed := range []string{"attempt", "ingress"} {
		t.Run(changed, func(t *testing.T) {
			f := newApprovalRecoveryFixtureAtPhase(t, string(updatePhaseApprovalPublish))
			scope := imageagent.RunScope{TenantID: f.publish.Identity.TenantID, OwnerUserID: f.publish.Identity.UserID, RunID: f.publish.RunID}
			current, err := f.activities.repository.GetProjection(context.Background(), scope)
			require.NoError(t, err)
			pending := clonePendingReceipt(current.PendingCommand)
			pending.Phase = imageagent.ImageSetApprovalPublicationStarted
			var concurrent imageagent.RunProjection
			f.auth.onAuthorize = func() {
				f.auth.onAuthorize = nil
				updated := current
				updated.PendingCommand = clonePendingReceipt(current.PendingCommand)
				if changed == "attempt" {
					updated.PendingCommand.Attempt++
				} else {
					updated.CommandIngress.Used++
				}
				concurrent, err = f.activities.repository.CommitProjection(context.Background(), imageagent.ProjectionCommit{Scope: scope, CommitID: "concurrent-before-publication-marker", ExpectedProjectionVersion: current.ProjectionVersion, Snapshot: updated, EventType: "command.receipt.updated", EventPayload: []byte(`{}`)})
				require.NoError(t, err)
			}
			err = f.activities.PersistPendingCommand(context.Background(), PersistPendingCommandActivityInput{RunID: f.publish.RunID, Identity: f.publish.Identity, Receipt: pending, CommandIngress: current.CommandIngress, CommitID: "stale-first-publication-marker"})
			require.ErrorIs(t, err, imageagent.ErrRevisionConflict)
			retained, err := f.activities.repository.GetProjection(context.Background(), scope)
			require.NoError(t, err)
			require.Equal(t, concurrent, retained)
			require.Zero(t, f.selector.calls)
		})
	}
}

func (f approvalRecoveryFixture) loseCommittedACK(t *testing.T) productasset.ApprovalCommit {
	t.Helper()
	require.ErrorIs(t, f.activities.PublishApprovedImageSet(context.Background(), f.publish), context.DeadlineExceeded)
	commit, err := f.assets.(productasset.ApprovalCommitReader).ReadApprovalCommit(context.Background(), f.publish.Identity.TenantID, f.publish.Selection.ActionID)
	require.NoError(t, err, "the Asset transaction committed despite the lost activity acknowledgement")
	require.Equal(t, f.publish.Selection.SelectionDigest, commit.ImageSet.Digest)
	f.auth.revoked = true
	return commit
}

func TestImageSetCommittedApprovalActivityReplaysAfterRevocation(t *testing.T) {
	f := newApprovalRecoveryFixture(t)
	original := f.loseCommittedACK(t)
	reads, authorizations := f.sources.reads, f.auth.calls
	require.NoError(t, f.activities.PublishApprovedImageSet(context.Background(), f.publish))
	require.Equal(t, 1, f.selector.calls, "receipt replay must not invoke Select/CommitApproval again")
	require.Equal(t, reads, f.sources.reads, "receipt replay reads no current source/materials")
	require.Equal(t, authorizations, f.auth.calls, "already committed facts do not require a fresh grant")
	retained, err := f.assets.(productasset.ApprovalCommitReader).ReadApprovalCommit(context.Background(), f.publish.Identity.TenantID, f.publish.Selection.ActionID)
	require.NoError(t, err)
	require.Equal(t, original, retained)
}

func TestImageSetCommittedApprovalCompletionSurvivesRevocation(t *testing.T) {
	f := newApprovalRecoveryFixture(t)
	f.loseCommittedACK(t)
	require.NoError(t, f.activities.PersistRunState(context.Background(), f.complete))
	current, err := f.activities.repository.GetProjection(context.Background(), imageagent.RunScope{TenantID: f.publish.Identity.TenantID, OwnerUserID: f.publish.Identity.UserID, RunID: f.publish.RunID})
	require.NoError(t, err)
	require.Equal(t, imageagent.RunStatusCompleted, current.Run.Status)
	require.Nil(t, current.PendingCommand)
	require.Equal(t, f.complete.Projection.Plan, current.Plan)
	require.Equal(t, f.complete.Projection.Slots, current.Slots)
	require.Equal(t, 1, f.selector.calls)
	require.NoError(t, f.activities.PersistRunState(context.Background(), f.complete), "lost completion ACK also replays exactly")
}

func TestImageSetCommittedApprovalPendingRecoverySurvivesRevocation(t *testing.T) {
	f := newApprovalRecoveryFixture(t)
	f.loseCommittedACK(t)
	current, err := f.activities.repository.GetProjection(context.Background(), imageagent.RunScope{TenantID: f.publish.Identity.TenantID, OwnerUserID: f.publish.Identity.UserID, RunID: f.publish.RunID})
	require.NoError(t, err)
	pending := clonePendingReceipt(current.PendingCommand)
	pending.Attempt++
	pending.Status = "failed"
	pending.FailureCode = "publication_ack_unknown"
	require.NoError(t, f.activities.PersistPendingCommand(context.Background(), PersistPendingCommandActivityInput{RunID: f.publish.RunID, Identity: f.publish.Identity, Receipt: pending, CommandIngress: current.CommandIngress, CommitID: "original-approval-recovery"}))
	require.NoError(t, f.activities.PublishApprovedImageSet(context.Background(), f.publish))
	require.NoError(t, f.activities.PersistRunState(context.Background(), f.complete))
	require.Equal(t, 1, f.selector.calls)
}

func TestImageSetUncommittedApprovalStillRequiresLiveAuthorization(t *testing.T) {
	f := newApprovalRecoveryFixture(t)
	f.auth.revoked = true
	require.ErrorIs(t, f.activities.PublishApprovedImageSet(context.Background(), f.publish), imageagent.ErrIdentityRequired)
	require.ErrorIs(t, f.activities.PersistRunState(context.Background(), f.complete), imageagent.ErrIdentityRequired)
	require.Zero(t, f.selector.calls)
	_, err := f.assets.(productasset.ApprovalCommitReader).ReadApprovalCommit(context.Background(), f.publish.Identity.TenantID, f.publish.Selection.ActionID)
	require.ErrorIs(t, err, productasset.ErrApprovedAssetsNotReady)
}

func TestImageSetCommittedApprovalCannotAuthorizeChangedPublication(t *testing.T) {
	f := newApprovalRecoveryFixture(t)
	f.loseCommittedACK(t)
	for _, kind := range []string{"organization", "actor", "member", "business_task", "run", "revision", "result", "action", "selection", "ordered_choice", "source", "target", "head"} {
		t.Run(kind, func(t *testing.T) {
			input := f.publish
			input.Selection = *imageagent.CloneImageSetCommand(&f.publish.Selection)
			switch kind {
			case "organization":
				input.Identity.TenantID = "another-org"
			case "actor":
				input.Identity.UserID = "another-actor"
			case "member":
				input.Identity.MemberID = "another-member"
			case "business_task":
				input.Identity.BusinessTaskID = "another-operation"
			case "run":
				input.RunID = "3b9ef503-8c6c-4a76-8eb8-047213c565cf"
			case "revision":
				input.PlanRevision++
			case "result":
				input.ResultDigest = strings.Repeat("c", 64)
			case "action":
				input.Selection.ActionID = "ca1aa25c-3661-4c80-8c85-e3fe93220f8f"
			case "selection":
				input.Selection.SelectionDigest = strings.Repeat("c", 64)
			case "ordered_choice":
				input.Selection.Choices[0].SourceID = "another-source"
			case "source":
				input.Selection.Source.EffectiveCatalogVersion++
			case "target":
				input.Selection.Target = &productasset.ImageSetTarget{StoreID: "another-store"}
			case "head":
				input.Selection.ExpectedHead = productasset.ImageInventoryHead{ActionID: "newer-approval", PayloadHash: strings.Repeat("a", 64)}
			}
			require.Error(t, f.activities.PublishApprovedImageSet(context.Background(), input))
			require.Equal(t, 1, f.selector.calls)
		})
	}
}

type approvalRecoveryReceiptReader struct {
	read func(context.Context, string, string) (productasset.ApprovalCommit, error)
}

func (r approvalRecoveryReceiptReader) ReadApprovalCommit(ctx context.Context, tenant, action string) (productasset.ApprovalCommit, error) {
	return r.read(ctx, tenant, action)
}

func TestImageSetApprovalReceiptReadFailureDoesNotStartAnotherMutation(t *testing.T) {
	f := newApprovalRecoveryFixture(t)
	f.loseCommittedACK(t)
	f.auth.revoked = false // Even a live grant cannot turn a read failure into permission to retry blindly.
	f.activities.imageSetApprovals = approvalRecoveryReceiptReader{read: func(context.Context, string, string) (productasset.ApprovalCommit, error) {
		return productasset.ApprovalCommit{}, productasset.ErrRepositoryUnavailable
	}}
	require.ErrorIs(t, f.activities.PublishApprovedImageSet(context.Background(), f.publish), productasset.ErrRepositoryUnavailable)
	require.ErrorIs(t, f.activities.PersistRunState(context.Background(), f.complete), productasset.ErrRepositoryUnavailable)
	require.Equal(t, 1, f.selector.calls)
	current, err := f.activities.repository.GetProjection(context.Background(), imageagent.RunScope{TenantID: f.publish.Identity.TenantID, OwnerUserID: f.publish.Identity.UserID, RunID: f.publish.RunID})
	require.NoError(t, err)
	require.Equal(t, imageagent.RunStatusAwaitingFinalApproval, current.Run.Status)
	require.Equal(t, f.publish.Selection.ActionID, current.PendingCommand.ActionID)
}

func TestImageSetApprovalRecoveryCannotChangeCompletionOrPendingAction(t *testing.T) {
	f := newApprovalRecoveryFixture(t)
	f.loseCommittedACK(t)
	for _, kind := range []string{"plan", "slots", "result", "block", "recoverable", "pending", "member", "revision"} {
		t.Run("completion_"+kind, func(t *testing.T) {
			input := f.complete
			switch kind {
			case "plan":
				input.Projection.Plan.IdempotencyKey = "another-plan"
			case "slots":
				input.Projection.Slots = nil
			case "result":
				input.Projection.ResultDigest = strings.Repeat("c", 64)
			case "block":
				input.Projection.Block = &imageagent.Block{Code: "cancelled"}
			case "recoverable":
				input.Projection.RecoverableEffects = []imageagent.RecoverableEffect{{}}
			case "pending":
				input.Projection.PendingCommand = &imageagent.PendingCommandReceipt{ActionID: "another-action"}
			case "member":
				input.Identity.MemberID = "another-member"
			case "revision":
				input.PlanRevision++
			}
			require.Error(t, f.activities.PersistRunState(context.Background(), input))
		})
	}
	scope := imageagent.RunScope{TenantID: f.publish.Identity.TenantID, OwnerUserID: f.publish.Identity.UserID, RunID: f.publish.RunID}
	current, err := f.activities.repository.GetProjection(context.Background(), scope)
	require.NoError(t, err)
	for _, kind := range []string{"action", "selection", "result", "revision", "slot", "phase", "attempt", "ingress", "clear"} {
		t.Run("pending_"+kind, func(t *testing.T) {
			input := PersistPendingCommandActivityInput{RunID: f.publish.RunID, Identity: f.publish.Identity, Receipt: clonePendingReceipt(current.PendingCommand), CommandIngress: current.CommandIngress, CommitID: "changed-pending"}
			switch kind {
			case "action":
				input.Receipt.ActionID = "ca1aa25c-3661-4c80-8c85-e3fe93220f8f"
			case "selection":
				input.Receipt.SelectionDigest = strings.Repeat("c", 64)
			case "result":
				input.Receipt.ResultDigest = strings.Repeat("c", 64)
			case "revision":
				input.Receipt.PlanRevision++
			case "slot":
				input.Receipt.SlotID = "another-slot"
			case "phase":
				input.Receipt.Phase = string(updatePhaseApprovalPublish)
			case "attempt":
				input.Receipt.Attempt--
			case "ingress":
				input.CommandIngress.Used++
			case "clear":
				input.Receipt = nil
			}
			require.Error(t, f.activities.PersistPendingCommand(context.Background(), input))
		})
	}
	retained, err := f.activities.repository.GetProjection(context.Background(), scope)
	require.NoError(t, err)
	require.Equal(t, current, retained)
	require.Equal(t, 1, f.selector.calls)
}

func TestImageSetCommittedApprovalReplayPreservesNewerInventoryHead(t *testing.T) {
	f := newApprovalRecoveryFixture(t)
	original := f.loseCommittedACK(t)
	f.auth.revoked = false
	inventoryScope := productasset.InventoryScope{TenantID: original.TenantID, ProductKey: original.ProductKey, TargetPlatform: original.TargetPlatform, SourceSnapshotVersion: original.SourceSnapshotVersion}
	inventory, err := f.assets.(productasset.ImageSetInventoryReader).ReadImageSetInventory(context.Background(), inventoryScope)
	require.NoError(t, err)
	next := *imageagent.CloneImageSetCommand(&f.publish.Selection)
	next.ActionID, next.ExpectedHead, next.SelectionDigest = "ca1aa25c-3661-4c80-8c85-e3fe93220f8f", inventory.Head, ""
	preview, err := f.selector.service.Preview(context.Background(), next)
	require.NoError(t, err)
	next.SelectionDigest = preview.Digest
	_, err = f.selector.service.Select(context.Background(), next)
	require.NoError(t, err)
	newInventory, err := f.assets.(productasset.ImageSetInventoryReader).ReadImageSetInventory(context.Background(), inventoryScope)
	require.NoError(t, err)
	require.Equal(t, next.ActionID, newInventory.Head.ActionID)
	f.auth.revoked = true
	require.NoError(t, f.activities.PublishApprovedImageSet(context.Background(), f.publish))
	require.NoError(t, f.activities.PersistRunState(context.Background(), f.complete))
	retained, err := f.assets.(productasset.ImageSetInventoryReader).ReadImageSetInventory(context.Background(), inventoryScope)
	require.NoError(t, err)
	require.Equal(t, newInventory, retained, "finishing the original Run must not revert a newer approved inventory")
	require.Equal(t, 1, f.selector.calls)
}

func TestImageSetApprovalRecoveryUsesCheckedProjectionCAS(t *testing.T) {
	f := newApprovalRecoveryFixture(t)
	f.loseCommittedACK(t)
	scope := imageagent.RunScope{TenantID: f.publish.Identity.TenantID, OwnerUserID: f.publish.Identity.UserID, RunID: f.publish.RunID}
	f.activities.imageSetApprovals = approvalRecoveryReceiptReader{read: func(ctx context.Context, tenant, action string) (productasset.ApprovalCommit, error) {
		commit, err := f.assets.(productasset.ApprovalCommitReader).ReadApprovalCommit(ctx, tenant, action)
		require.NoError(t, err)
		current, err := f.activities.repository.GetProjection(ctx, scope)
		require.NoError(t, err)
		updated := current
		updated.PendingCommand = clonePendingReceipt(current.PendingCommand)
		updated.PendingCommand.Status = "failed"
		_, err = f.activities.repository.CommitProjection(ctx, imageagent.ProjectionCommit{Scope: scope, CommitID: "concurrent-original-metadata", ExpectedProjectionVersion: current.ProjectionVersion, Snapshot: updated, EventType: "command.receipt.updated", EventPayload: []byte(`{}`)})
		require.NoError(t, err)
		return commit, nil
	}}
	require.ErrorIs(t, f.activities.PersistRunState(context.Background(), f.complete), imageagent.ErrRevisionConflict)
	retained, err := f.activities.repository.GetProjection(context.Background(), scope)
	require.NoError(t, err)
	require.Equal(t, imageagent.RunStatusAwaitingFinalApproval, retained.Run.Status)
	require.Equal(t, "failed", retained.PendingCommand.Status)
	require.Equal(t, 1, f.selector.calls)
}
