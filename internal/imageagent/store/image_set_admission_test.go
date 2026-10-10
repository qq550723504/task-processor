package store

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"task-processor/internal/agent"
	"task-processor/internal/agentconfig"
	"task-processor/internal/imageagent"
)

func TestSetConfirmationPersistsOriginalAdmissionExactlyOnce(t *testing.T) {
	for _, kind := range []string{"memory", "gorm"} {
		t.Run(kind, func(t *testing.T) {
			var repo repositoryContract = NewMemoryRepository().(repositoryContract)
			if kind == "gorm" {
				repo = NewOrganizationRepository(newConcurrentSQLite(t)).(repositoryContract)
			}
			run := manualRun("392ed0a2-0f01-4c94-9aa4-eb50271fae9c", "org")
			run.ScopeProtocol, run.MemberID, run.TargetPlatform = imageagent.OrganizationScopeProtocol, "member", "product"
			run.Status, run.CurrentNode = imageagent.RunStatusAwaitingPlanApproval, "confirm_plan"
			plan := imageSetPlanForStore(t)
			limits := agentconfig.ImageRunLimits{Images: 1, Points: 20, ElapsedSeconds: 300}
			run.Budget = imageagent.ImageSetBudget(limits)
			scope := imageagent.ScopeForRun(*run)
			ctx := context.Background()
			current, err := repo.InitializeRun(ctx, imageagent.ProjectionInitialization{Scope: scope, Run: *run, Plan: plan, Snapshot: imageagent.RunProjection{Run: *run}, Catalog: imageagent.AssetCatalog{Assets: []imageagent.AuthorizedAsset{{ID: "source-1", Type: imageagent.AuthorizedAssetSource, URL: "https://images.example.org/source.png"}}}, CommitID: "prepare", EventType: "run.prepared", EventPayload: json.RawMessage(`{}`)})
			require.NoError(t, err)
			digest, err := imageagent.ImageSetPlanDigest(plan)
			require.NoError(t, err)
			now := time.Now().UTC().Truncate(time.Microsecond)
			receipt := agentconfig.ImageRunAdmissionReceipt{ID: "74e01be8-f4e6-461f-8acb-18fe37329f1c", AdmittedAt: now, Deadline: now.Add(300 * time.Second), Command: agentconfig.ImageRunAdmissionCommand{Scope: agent.Scope{OrganizationID: scope.TenantID, ActorID: scope.OwnerUserID}, Snapshot: plan.Set.Configuration, MemberID: run.MemberID, RunID: run.ID, ConfirmActionID: "1b912e40-d50a-48f5-a12b-62c73a42e4b8", SourceDigest: imageagent.ImageSetSourceDigest(plan.Set.Source, plan), InputDigest: plan.Set.InputDigest, PlanDigest: digest, QuoteDigest: plan.Set.QuoteDigest, Limits: limits}}
			receipt.Digest, err = hashJSON(receipt)
			require.NoError(t, err)
			next := current
			next.Run.ImageAdmission, next.Run.StartedAt = &receipt, receipt.AdmittedAt
			next.Run.Status, next.Run.CurrentNode, next.Run.Version = imageagent.RunStatusExecuting, "execute_slots", current.Run.Version+1
			commit := imageagent.ProjectionCommit{Scope: scope, CommitID: "confirm:" + receipt.Command.ConfirmActionID, ExpectedProjectionVersion: current.ProjectionVersion, ExpectedRunVersion: current.Run.Version, Snapshot: next, RunMutation: &imageagent.RunMutation{ImageAdmission: &receipt, Status: next.Run.Status, CurrentNode: next.Run.CurrentNode, ActivePlanRevision: 1}, EventType: "run.confirmed", EventPayload: json.RawMessage(`{}`)}
			_, err = repo.CommitProjection(ctx, commit)
			require.NoError(t, err)
			got, err := repo.GetProjection(ctx, scope)
			require.NoError(t, err)
			require.Equal(t, &receipt, got.Run.ImageAdmission)
			require.Equal(t, now, got.Run.StartedAt)
			_, err = repo.CommitProjection(ctx, commit)
			require.NoError(t, err, "original confirm replay returns its exact immutable commit")
			changed := receipt
			changed.Digest, changed.Deadline = strings.Repeat("b", 64), now.Add(600*time.Second)
			commit.CommitID, commit.ExpectedProjectionVersion, commit.ExpectedRunVersion = "overwrite", got.ProjectionVersion, got.Run.Version
			commit.Snapshot = got
			commit.Snapshot.Run.Version++
			commit.Snapshot.Run.ImageAdmission, commit.RunMutation.ImageAdmission = &changed, &changed
			_, err = repo.CommitProjection(ctx, commit)
			require.ErrorIs(t, err, imageagent.ErrRevisionConflict, "no later mutation can replace or refresh admission")
		})
	}
}
