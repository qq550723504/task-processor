package imageagent

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func generationTestIntent() GenerationIntent {
	return GenerationIntent{Identity: SlotExternalEffectIdentity{RunScope: RunScope{TenantID: "org-1", OwnerUserID: "actor-1", RunID: "run-1"}, PlanRevision: 1, SlotID: "main", Attempt: 1}, MemberID: "grant-1", CatalogHash: "catalog-v1:" + strings.Repeat("a", 64), SourceDigest: strings.Repeat("b", 64), PromptVersion: "white-v1", RouteReference: "route-1", CredentialReference: "credential-1", ConfigurationVersion: "config-1", Provider: "grsai", Model: "gpt-image-2.5", Protocol: "grsai-json-sync-v1", Resolution: "1024x1024", Quality: "auto", PriceVersion: "price-1", Points: 12, LimitVersion: 1, MonthStart: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)}
}

func generationTestReceipt(f GenerationFact) GenerationReservationReceipt {
	return GenerationReservationReceipt{IntentID: f.IntentID, Fingerprint: f.Fingerprint, OrganizationID: f.Intent.Identity.TenantID, MemberID: f.Intent.MemberID, OperationID: "image-reserve:" + f.IntentID, ReservationID: "reservation-1", ResourceType: "ai_point", Points: f.Intent.Points, PriceVersion: f.Intent.PriceVersion, LimitVersion: f.Intent.LimitVersion, MonthStart: f.Intent.MonthStart}
}

func TestGenerationFactFenceAndImmutableProviderProof(t *testing.T) {
	f, err := NewGenerationFact(generationTestIntent())
	require.NoError(t, err)
	require.Equal(t, GenerationPrepared, f.State)
	_, won, err := f.BeginDispatch()
	require.Error(t, err)
	require.False(t, won)
	f, err = f.BindReservation(generationTestReceipt(f))
	require.NoError(t, err)
	started, won, err := f.BeginDispatch()
	require.NoError(t, err)
	require.True(t, won)
	replay, won, err := started.BeginDispatch()
	require.NoError(t, err)
	require.False(t, won)
	require.Equal(t, started, replay)
	_, err = started.RecordNoGeneration()
	require.Error(t, err)
	unknown, err := started.MarkUnknown()
	require.NoError(t, err)
	_, won, err = unknown.BeginDispatch()
	require.NoError(t, err)
	require.False(t, won)
	proof := GenerationSuccess{ResponseID: "response-1", ResultDigest: strings.Repeat("c", 64)}
	succeeded, err := unknown.RecordSuccess(proof)
	require.NoError(t, err)
	replayed, err := succeeded.RecordSuccess(proof)
	require.NoError(t, err)
	require.Equal(t, succeeded, replayed)
	changed := proof
	changed.ResponseID = "different"
	_, err = succeeded.RecordSuccess(changed)
	require.Error(t, err)
	kept, err := succeeded.MarkUnknown()
	require.NoError(t, err)
	require.Equal(t, succeeded, kept)
	closed, err := f.RecordNoGeneration()
	require.NoError(t, err)
	_, won, err = closed.BeginDispatch()
	require.NoError(t, err)
	require.False(t, won)
	_, err = closed.RecordSuccess(proof)
	require.Error(t, err)
}

func TestGenerationIntentStableIdentityRejectsDifferentEconomics(t *testing.T) {
	intent := generationTestIntent()
	f, err := NewGenerationFact(intent)
	require.NoError(t, err)
	for _, field := range []string{"price", "points", "member", "month", "route"} {
		t.Run(field, func(t *testing.T) {
			changed := intent
			switch field {
			case "price":
				changed.PriceVersion = "price-2"
			case "points":
				changed.Points++
			case "member":
				changed.MemberID = "grant-2"
			case "month":
				changed.MonthStart = changed.MonthStart.AddDate(0, 1, 0)
			case "route":
				changed.RouteReference = "route-2"
			}
			other, err := NewGenerationFact(changed)
			require.NoError(t, err)
			require.Equal(t, f.IntentID, other.IntentID)
			require.NotEqual(t, f.Fingerprint, other.Fingerprint)
			_, err = f.BindReservation(generationTestReceipt(other))
			require.Error(t, err)
		})
	}
}

func TestGenerationSuccessNeverInventsUnknownUsage(t *testing.T) {
	f, err := NewGenerationFact(generationTestIntent())
	require.NoError(t, err)
	f, err = f.BindReservation(generationTestReceipt(f))
	require.NoError(t, err)
	f, _, err = f.BeginDispatch()
	require.NoError(t, err)
	proof := GenerationSuccess{ResponseID: "response-1", ResultDigest: strings.Repeat("c", 64), UsageKnown: true, InputTokens: 1, OutputTokens: 5, TotalTokens: 6}
	_, err = f.RecordSuccess(proof)
	require.NoError(t, err)
	proof.UsageKnown = false
	_, err = f.RecordSuccess(proof)
	require.Error(t, err)
	proof.UsageKnown = true
	proof.TotalTokens = 5
	_, err = f.RecordSuccess(proof)
	require.Error(t, err)
}
