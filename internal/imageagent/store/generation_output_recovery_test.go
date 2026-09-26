package store

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"task-processor/internal/imageagent"
	resourceadapter "task-processor/internal/integration/orgresource"
)

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
