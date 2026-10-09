package store

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"task-processor/internal/agent"
	"task-processor/internal/agentconfig"
	"task-processor/internal/imageagent"
)

func imageSetPlanForStore(t *testing.T) imageagent.Plan {
	t.Helper()
	plan := planRevision(1)
	plan.StyleReferenceIDs = nil
	plan.Slots[0].StyleReferenceIDs = nil
	hash := strings.Repeat("a", 64)
	plan.Set = &imageagent.ImageSetPlan{Schema: imageagent.ImageSetSchema, Source: imageagent.ImageSourceBinding{ProductID: "product-1", OperationID: "operation-1", OriginalPublicationID: "source-product", OriginalVersion: 1, EffectiveVersion: 1, CatalogHash: "catalog-v2:" + hash}, Target: imageagent.ImageTarget{Platform: "product"}, Configuration: agent.ConfigurationSnapshotRef{Kind: agentconfig.SnapshotKind, ID: "9e7afaa9-a9f9-48ba-a11a-b5bb377f08e9", Digest: hash}, ConfigurationEpoch: "1", ParametersDigest: hash, InputDigest: hash, MaxPoints: 20}
	plan.Slots[0].Recipe = &imageagent.ImageSlotRecipe{Purpose: "product_identity", Background: "白色", Language: "zh", Placement: imageagent.ImagePlacement{Group: "carousel", Order: 1}, PromptVersion: imageagent.ImageSetSchema, Prompt: "根据原始素材展示商品", References: []imageagent.ImageSourceObservation{{AssetID: "source-1", SHA256: hash, MediaType: "image/png", Bytes: 10, Width: 1024, Height: 1024}}, Quote: imageagent.ImageGenerationQuote{Provider: "grsai", Model: "gpt-image-2.5", Protocol: "grsai-json-sync-v1", Resolution: "1024x1024", Quality: "auto", PriceVersion: "price-1", Points: 20, RouteReference: "route-1", CredentialReference: "credential-1", ConfigurationVersion: "configuration-1"}}
	var err error
	plan.Set.QuoteDigest, err = imageagent.ImageSetQuoteDigest(plan)
	require.NoError(t, err)
	require.NoError(t, imageagent.ValidateInitialSubmittedPlan(plan))
	return plan
}

func TestSetSlotClosurePreservesUnstartedWorkInNormalizedAndProjectedState(t *testing.T) {
	for _, kind := range []string{"memory", "gorm"} {
		t.Run(kind, func(t *testing.T) {
			var repo repositoryContract = NewMemoryRepository().(repositoryContract)
			var sqlRepo *gormRepository
			if kind == "gorm" {
				sqlRepo = NewGormRepository(newConcurrentSQLite(t)).(*gormRepository)
				repo = sqlRepo
			}
			ctx := context.Background()
			run := manualRun("set-unstarted", "tenant-a")
			plan := imageSetPlanForStore(t)
			scope := imageagent.ScopeForRun(*run)
			current, err := repo.InitializeRun(ctx, imageagent.ProjectionInitialization{Scope: scope, Run: *run, Plan: plan, Catalog: imageagent.AssetCatalog{Assets: []imageagent.AuthorizedAsset{{ID: "source-1", Type: imageagent.AuthorizedAssetSource, URL: "https://images.example.org/source.png"}}}, Snapshot: imageagent.RunProjection{Run: *run, Plan: plan}, CommitID: "start", EventType: "run.initialized", EventPayload: json.RawMessage(`{}`)})
			require.NoError(t, err)
			unverified := current
			unverified.Slots = append([]imageagent.SlotProjection(nil), current.Slots...)
			unverified.Slots[0].Slot.Status = imageagent.SlotStatusAccepted
			unverified.Slots[0].Attempt = 1
			unverified.Slots[0].Candidates = []imageagent.AssetCandidate{{AssetID: "unverified", URL: "https://images.example.org/unverified.png", SourceAssetID: "source-1", Width: 1024, Height: 1024, Operations: []string{"render_source_edit"}}}
			badMutation := &imageagent.SlotProjectionMutation{PlanRevision: 1, Result: imageagent.SlotResult{SlotID: plan.Slots[0].ID, Attempt: 1, Status: imageagent.SlotStatusAccepted, CandidateAssetIDs: []string{"unverified"}}, Projection: unverified.Slots[0], Attempt: imageagent.StepAttempt{TenantID: scope.TenantID, OwnerUserID: scope.OwnerUserID, RunID: scope.RunID, PlanRevision: 1, SlotID: plan.Slots[0].ID, Attempt: 1, Node: "execute_slot_v3", IdempotencyKey: "unverified", Outcome: "accepted"}}
			_, err = repo.CommitProjection(ctx, imageagent.ProjectionCommit{Scope: scope, CommitID: "unverified", ExpectedProjectionVersion: current.ProjectionVersion, Snapshot: unverified, EventType: "slot.result.persisted", EventPayload: json.RawMessage(`{}`), SlotMutation: badMutation})
			require.ErrorIs(t, err, imageagent.ErrRevisionConflict, "a set cannot persist accepted candidates without original settlement evidence")
			closure := &imageagent.ImageSlotClosure{Kind: "not_dispatched"}
			updated := current
			updated.Slots = append([]imageagent.SlotProjection(nil), current.Slots...)
			updated.Slots[0].Slot.Status = imageagent.SlotStatusBlocked
			updated.Slots[0].ErrorCode = imageagent.BudgetElapsedCode
			updated.Slots[0].Closure = closure
			mutation := &imageagent.SlotProjectionMutation{PlanRevision: 1, Result: imageagent.SlotResult{SlotID: plan.Slots[0].ID, Status: imageagent.SlotStatusBlocked, ErrorCode: imageagent.BudgetElapsedCode, Closure: closure}, Projection: updated.Slots[0], Attempt: imageagent.StepAttempt{TenantID: scope.TenantID, OwnerUserID: scope.OwnerUserID, RunID: scope.RunID, PlanRevision: 1, SlotID: plan.Slots[0].ID, Node: "execute_slot_v3", IdempotencyKey: "unstarted", Outcome: "blocked", ErrorCategory: imageagent.BudgetElapsedCode}}
			got, err := repo.CommitProjection(ctx, imageagent.ProjectionCommit{Scope: scope, CommitID: "unstarted", ExpectedProjectionVersion: current.ProjectionVersion, Snapshot: updated, EventType: "slot.result.persisted", EventPayload: json.RawMessage(`{}`), SlotMutation: mutation})
			require.NoError(t, err)
			require.Zero(t, got.Slots[0].Attempt)
			require.Equal(t, closure, got.Slots[0].Closure)
			if sqlRepo != nil {
				var row slotRecord
				require.NoError(t, sqlRepo.db.Where("run_id = ?", scope.RunID).Take(&row).Error)
				result, err := slotResultFromRecord(row)
				require.NoError(t, err)
				require.Equal(t, closure, result.Closure)
			}
		})
	}
}

func TestSetPlanReplayBindsNormalizedRecipeAndConfiguration(t *testing.T) {
	for _, kind := range []string{"memory", "gorm"} {
		t.Run(kind, func(t *testing.T) {
			var repository repositoryContract = NewMemoryRepository().(repositoryContract)
			if kind == "gorm" {
				repository = NewGormRepository(newConcurrentSQLite(t)).(repositoryContract)
			}
			ctx := context.Background()
			scope := imageagent.RunScope{TenantID: "tenant-a", OwnerUserID: "user-1", RunID: "run-1"}
			require.NoError(t, repository.CreateRun(ctx, manualRun(scope.RunID, scope.TenantID)))
			original := imageSetPlanForStore(t)
			require.NoError(t, repository.AppendPlan(ctx, scope, 0, original))
			require.NoError(t, repository.AppendPlan(ctx, scope, 0, imageSetPlanForStore(t)))
			changed := imageSetPlanForStore(t)
			changed.Slots[0].Recipe.Prompt = "改变构图和背景"
			require.ErrorIs(t, repository.AppendPlan(ctx, scope, 0, changed), imageagent.ErrRevisionConflict, "same plan identity cannot conceal a changed recipe")
			changed = imageSetPlanForStore(t)
			changed.Set.InputDigest = strings.Repeat("b", 64)
			require.ErrorIs(t, repository.AppendPlan(ctx, scope, 0, changed), imageagent.ErrRevisionConflict, "same identity cannot conceal a changed configuration/input")
			// Mutation of a caller-owned recipe must not alter the retained plan.
			original.Slots[0].Recipe.Background = "red"
			require.NoError(t, repository.AppendPlan(ctx, scope, 0, imageSetPlanForStore(t)))
		})
	}
}
