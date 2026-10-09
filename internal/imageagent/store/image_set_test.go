package store

import (
	"context"
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
	plan.Set = &imageagent.ImageSetPlan{Schema: imageagent.ImageSetSchema, Source: imageagent.ImageSourceBinding{ProductID: "product-1", OperationID: "operation-1", OriginalPublicationID: "source-product", OriginalVersion: 1, EffectiveVersion: 1, CatalogHash: hash}, Target: imageagent.ImageTarget{Platform: "product"}, Configuration: agent.ConfigurationSnapshotRef{Kind: agentconfig.SnapshotKind, ID: "9e7afaa9-a9f9-48ba-a11a-b5bb377f08e9", Digest: hash}, ConfigurationEpoch: "1", ParametersDigest: hash, InputDigest: hash, MaxPoints: 20}
	plan.Slots[0].Recipe = &imageagent.ImageSlotRecipe{Purpose: "product_identity", Background: "白色", Language: "zh", Placement: imageagent.ImagePlacement{Group: "carousel", Order: 1}, PromptVersion: imageagent.ImageSetSchema, Prompt: "根据原始素材展示商品", References: []imageagent.ImageSourceObservation{{AssetID: "source-1", SHA256: hash, MediaType: "image/png", Bytes: 10, Width: 1024, Height: 1024}}, Quote: imageagent.ImageGenerationQuote{Provider: "grsai", Model: "gpt-image-2.5", Protocol: "grsai-json-sync-v1", Resolution: "1024x1024", Quality: "auto", PriceVersion: "price-1", Points: 20, RouteReference: "route-1", CredentialReference: "credential-1", ConfigurationVersion: "configuration-1"}}
	var err error
	plan.Set.QuoteDigest, err = imageagent.ImageSetQuoteDigest(plan)
	require.NoError(t, err)
	require.NoError(t, imageagent.ValidateInitialSubmittedPlan(plan))
	return plan
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
