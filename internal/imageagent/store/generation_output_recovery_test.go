package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"task-processor/internal/agent"
	"task-processor/internal/imageagent"
	resourceadapter "task-processor/internal/integration/orgresource"
)

func TestImageSetSuccessRecoversStagingForFrozenSlotShapes(t *testing.T) {
	for _, shape := range []struct {
		purpose, group string
		references     int
	}{{"product_identity", "carousel", 2}, {"product_overview", "detail", 1}, {"use_scenario", "detail", 2}, {"key_benefits", "detail", 2}} {
		t.Run(shape.purpose, func(t *testing.T) {
			owner, commercial, limits, intent := generationPointDatabases(t, true)
			ctx := context.Background()
			recipe := &imageagent.ImageSlotRecipe{Purpose: shape.purpose, Background: "white", Language: "en", Placement: imageagent.ImagePlacement{Group: shape.group, Order: 1}, PromptVersion: imageagent.ImageSetSchema, Prompt: "Use the exact source product", Quote: imageagent.ImageGenerationQuote{Provider: intent.Provider, Model: intent.Model, Protocol: intent.Protocol, Resolution: intent.Resolution, Quality: intent.Quality, PriceVersion: intent.PriceVersion, Points: intent.Points, RouteReference: intent.RouteReference, CredentialReference: intent.CredentialReference, ConfigurationVersion: intent.ConfigurationVersion}}
			sources := []string{}
			for index := 0; index < shape.references; index++ {
				id := fmt.Sprintf("source-%d", index+1)
				sources = append(sources, id)
				recipe.References = append(recipe.References, imageagent.ImageSourceObservation{AssetID: id, SHA256: strings.Repeat("b", 64), MediaType: "image/png", Bytes: 10, Width: 1024, Height: 1024})
			}
			set := &imageagent.ImageSetPlan{Schema: imageagent.ImageSetSchema, Source: imageagent.ImageSourceBinding{ContextKind: imageagent.ImageSourceAcquisition, ProductID: "product", OperationID: "operation", OriginalPublicationID: "publication", OriginalVersion: 1, EffectiveVersion: 1, CatalogHash: intent.CatalogHash}, Target: imageagent.ImageTarget{Platform: "product"}, Configuration: agent.ConfigurationSnapshotRef{Kind: "agent-configuration-v1", ID: "9e7afaa9-a9f9-48ba-a11a-b5bb377f08e9", Digest: strings.Repeat("a", 64)}, ConfigurationEpoch: "1", ParametersDigest: strings.Repeat("a", 64), InputDigest: strings.Repeat("a", 64), MaxPoints: intent.Points}
			slot := imageagent.Slot{ID: intent.Identity.SlotID, Role: imageagent.SlotRoleForImagePurpose(shape.purpose), SourceAssetIDs: sources, IdempotencyKey: "slot", Recipe: recipe}
			var err error
			set.QuoteDigest, err = imageagent.ImageSetQuoteDigest(imageagent.Plan{Set: set, Slots: []imageagent.Slot{slot}})
			require.NoError(t, err)
			input := imageagent.SlotExecutionInput{RunID: intent.Identity.RunID, TenantID: intent.Identity.TenantID, UserID: intent.Identity.OwnerUserID, PlanRevision: intent.Identity.PlanRevision, Attempt: intent.Identity.Attempt, Slot: slot, ImageSet: set, TargetPlatform: "product"}
			require.NoError(t, imageagent.ValidateImageSetExecution(input))
			intent.InputProtocol, intent.PromptVersion = imageagent.ImageSetSchema, imageagent.ImageSetSchema
			intent.InputDigest = imageagent.ImageGenerationInputDigestFromFingerprint(imageagent.SlotExecutionFingerprint(input))
			intent.SourceDigest = imageagent.ImageSourceBundleDigest(recipe.References)
			encodedSet, _ := json.Marshal(set)
			encodedRecipe, _ := json.Marshal(recipe)
			encodedSources, _ := json.Marshal(sources)
			require.NoError(t, owner.db.Model(&planRecord{}).Where("run_id = ? AND revision = ?", intent.Identity.RunID, intent.Identity.PlanRevision).Update("set_json", encodedSet).Error)
			require.NoError(t, owner.db.Model(&slotRecord{}).Where("run_id = ? AND id = ?", intent.Identity.RunID, intent.Identity.SlotID).Updates(map[string]any{"role": string(slot.Role), "source_asset_ids": encodedSources, "style_reference_ids": []byte("[]"), "recipe_json": encodedRecipe}).Error)
			require.NoError(t, slotEffectV3IdentityWhere(owner.db.Model(&slotExternalEffectV3Record{}), intent.Identity).Update("input_fingerprint", imageagent.SlotExecutionFingerprint(input)).Error)
			_, err = owner.PrepareGenerationIntent(ctx, intent)
			require.NoError(t, err)
			resources, err := resourceadapter.NewGormImageGenerationRepository(commercial, resourceadapter.TransactionConfig{}, owner, generationPointTestAuth{})
			require.NoError(t, err)
			receipt, err := resources.ReserveImageGeneration(ctx, intent.Identity)
			require.NoError(t, err)
			_, err = owner.BindGenerationReservation(ctx, intent, receipt)
			require.NoError(t, err)
			_, won, err := owner.BeginGenerationDispatch(ctx, intent)
			require.NoError(t, err)
			require.True(t, won)
			_, err = owner.RecordGenerationSuccess(ctx, intent, imageagent.GenerationSuccess{ResponseID: "result", ResultDigest: strings.Repeat("c", 64)}.WithResultLocator("https://example.com/generated.png", ""))
			require.NoError(t, err)
			settled, err := resources.FinalizeImageGeneration(ctx, intent.Identity)
			require.NoError(t, err)
			_, err = owner.BindGenerationSettlement(ctx, intent, settled)
			require.NoError(t, err)
			fact, err := owner.ReadGenerationFact(ctx, intent.Identity)
			require.NoError(t, err)
			require.Equal(t, intent, fact.Intent)
			effect, err := owner.GetSlotExternalEffectV3(ctx, intent.Identity)
			require.NoError(t, err)
			reservation := v3Reservation("points")
			reservation.Policy, reservation.Quote = effect.Policy, effect.Quote
			reservation.InputFingerprint = effect.InputFingerprint
			manifest := v3StagingManifest()
			ownerKey, err := imageagent.ArtifactOwnerKey(intent.Identity.OwnerUserID)
			require.NoError(t, err)
			manifest.Assets[0].ObjectKey = fmt.Sprintf("image-agent/staging/%s/%s/%s/%d/%s/%d/0-%s.png", intent.Identity.TenantID, ownerKey, intent.Identity.RunID, intent.Identity.PlanRevision, intent.Identity.SlotID, intent.Identity.Attempt, manifest.Assets[0].SHA256)
			manifest.Assets[0].SourceAssetID = sources[0]
			for _, corruption := range []string{"primary", "recipe"} {
				bad := manifest
				bad.Assets = append([]imageagent.StagedAssetRef(nil), manifest.Assets...)
				if corruption == "primary" {
					bad.Assets[0].SourceAssetID = "wrong"
				} else {
					changed := imageagent.CloneImageSlotRecipe(recipe)
					changed.References[0].SHA256 = strings.Repeat("d", 64)
					encoded, _ := json.Marshal(changed)
					require.NoError(t, owner.db.Model(&slotRecord{}).Where("run_id = ? AND id = ?", intent.Identity.RunID, intent.Identity.SlotID).Update("recipe_json", encoded).Error)
				}
				_, err = owner.RecoverGenerationStaging(ctx, reservation, intent, fact.TerminalProofDigest(), bad)
				require.Error(t, err)
				require.NoError(t, owner.db.Model(&slotRecord{}).Where("run_id = ? AND id = ?", intent.Identity.RunID, intent.Identity.SlotID).Update("recipe_json", encodedRecipe).Error)
			}
			for replay := 0; replay < 2; replay++ {
				recovered, err := owner.RecoverGenerationStaging(ctx, reservation, intent, fact.TerminalProofDigest(), manifest)
				require.NoError(t, err)
				require.Equal(t, imageagent.SlotEffectV3StagingPrepared, recovered.Phase)
			}
			after, err := owner.ReadGenerationFact(ctx, intent.Identity)
			require.NoError(t, err)
			require.Equal(t, fact, after)
			_, won, err = owner.BeginGenerationDispatch(ctx, intent)
			require.NoError(t, err)
			require.False(t, won)
			limit, err := limits.ReadMonthlyLimit(ctx, intent.Identity.TenantID, intent.MemberID)
			require.NoError(t, err)
			require.Equal(t, intent.Points, limit.Consumed)
		})
	}
}

func TestGenerationSuccessCanRecoverStagingWithoutDispatch(t *testing.T) {
	for _, phase := range []imageagent.SlotEffectV3Phase{imageagent.SlotEffectV3ProviderClaimed, imageagent.SlotEffectV3ProviderUnknown, imageagent.SlotEffectV3StagingUnknown} {
		t.Run(string(phase), func(t *testing.T) {
			owner, commercial, _, intent := generationPointDatabases(t)
			ctx := context.Background()
			resources, err := resourceadapter.NewGormImageGenerationRepository(commercial, resourceadapter.TransactionConfig{}, owner, generationPointTestAuth{})
			require.NoError(t, err)
			receipt, err := resources.ReserveImageGeneration(ctx, intent.Identity)
			require.NoError(t, err)
			_, err = owner.BindGenerationReservation(ctx, intent, receipt)
			require.NoError(t, err)
			_, won, err := owner.BeginGenerationDispatch(ctx, intent)
			require.NoError(t, err)
			require.True(t, won)
			proof := imageagent.GenerationSuccess{ResponseID: "result-1", ResultDigest: strings.Repeat("c", 64)}.WithResultLocator("https://example.com/generated.png", "")
			_, err = owner.RecordGenerationSuccess(ctx, intent, proof)
			require.NoError(t, err)
			effect, err := owner.GetSlotExternalEffectV3(ctx, intent.Identity)
			require.NoError(t, err)
			reservation := v3Reservation("points")
			reservation.Policy, reservation.Quote = effect.Policy, effect.Quote
			manifest := v3StagingManifest()
			ownerKey, err := imageagent.ArtifactOwnerKey(intent.Identity.OwnerUserID)
			require.NoError(t, err)
			manifest.Assets[0].ObjectKey = fmt.Sprintf("image-agent/staging/%s/%s/%s/%d/%s/%d/0-%s.png", intent.Identity.TenantID, ownerKey, intent.Identity.RunID, intent.Identity.PlanRevision, intent.Identity.SlotID, intent.Identity.Attempt, manifest.Assets[0].SHA256)
			code := ""
			if phase == imageagent.SlotEffectV3ProviderUnknown {
				code = imageagent.SlotProviderOutcomeUnknownCode
			}
			if phase == imageagent.SlotEffectV3StagingUnknown {
				code = imageagent.SlotStagingOutcomeUnknownCode
			}
			require.NoError(t, slotEffectV3IdentityWhere(owner.db.Model(&slotExternalEffectV3Record{}), intent.Identity).Updates(map[string]any{"phase": string(phase), "blocked_code": code}).Error)
			recovery, ok := any(owner).(interface {
				RecoverGenerationStaging(context.Context, imageagent.SlotEffectV3Reservation, imageagent.GenerationIntent, string, imageagent.StagingManifest) (imageagent.SlotEffectV3Attempt, error)
			})
			require.True(t, ok, "V3 owner lacks success-proof staging recovery")
			fact, err := owner.ReadGenerationFact(ctx, intent.Identity)
			require.NoError(t, err)
			var wg sync.WaitGroup
			start := make(chan struct{})
			failures := make(chan error, 4)
			for n := 0; n < 4; n++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					<-start
					_, e := recovery.RecoverGenerationStaging(ctx, reservation, intent, fact.TerminalProofDigest(), manifest)
					failures <- e
				}()
			}
			close(start)
			wg.Wait()
			close(failures)
			for e := range failures {
				require.NoError(t, e)
			}
			got, err := recovery.RecoverGenerationStaging(ctx, reservation, intent, fact.TerminalProofDigest(), manifest)
			require.NoError(t, err)
			require.Equal(t, imageagent.SlotEffectV3StagingPrepared, got.Phase)
			changed := proof.WithResultLocator("https://example.com/different.png", "")
			_, err = owner.RecordGenerationSuccess(ctx, intent, changed)
			require.ErrorIs(t, err, imageagent.ErrRevisionConflict)
			_, won, err = owner.BeginGenerationDispatch(ctx, intent)
			require.NoError(t, err)
			require.False(t, won)
			_, err = recovery.RecoverGenerationStaging(ctx, reservation, intent, "changed", manifest)
			require.Error(t, err)
			for _, field := range []string{"member", "catalog", "attempt", "org", "quote", "manifest"} {
				t.Run(field, func(t *testing.T) {
					changedIntent, changedReservation, changedManifest := intent, reservation, manifest
					switch field {
					case "member":
						changedIntent.MemberID = "other"
					case "catalog":
						changedIntent.CatalogHash = "catalog-v1:" + strings.Repeat("d", 64)
					case "attempt":
						changedIntent.Identity.Attempt++
					case "org":
						changedIntent.Identity.TenantID = "other"
					case "quote":
						changedReservation.Quote.Fingerprint = "other"
					case "manifest":
						changedManifest.Assets = append([]imageagent.StagedAssetRef(nil), manifest.Assets...)
						changedManifest.Assets[0].SourceAssetID = "other"
					}
					_, e := recovery.RecoverGenerationStaging(ctx, changedReservation, changedIntent, fact.TerminalProofDigest(), changedManifest)
					require.Error(t, e)
				})
			}
		})
	}
}
