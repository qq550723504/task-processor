package effectpolicy

import (
	"time"

	"task-processor/internal/imageagent"
)

type EffectDecision struct {
	Attempt imageagent.SlotEffectV3Attempt
	Changed bool
}

type AccountingSnapshot struct {
	Policy    imageagent.BudgetPolicy
	Committed imageagent.UsageVector
	Reserved  imageagent.UsageVector
	Elapsed   time.Duration
	StartedAt time.Time
}

type AccountingDecision struct {
	EffectDecision
	Accounting        AccountingSnapshot
	AccountingChanged bool
}

type ProviderReservationDecision struct {
	AccountingDecision
	Acquired bool
}

type PublicationClaimDecision struct {
	EffectDecision
	Claim    imageagent.PublicationClaim
	Acquired bool
}

type PublicationLeaseDecision struct {
	EffectDecision
	Claim imageagent.PublicationClaim
}

func cloneSlotEffectV3Attempt(attempt imageagent.SlotEffectV3Attempt) imageagent.SlotEffectV3Attempt {
	attempt.StagingManifest.Assets = cloneStagedAssetRefs(attempt.StagingManifest.Assets)
	attempt.StagingManifest.ProviderMetadata = cloneStringMap(attempt.StagingManifest.ProviderMetadata)
	attempt.FinalManifest.Assets = clonePublishedAssetRefs(attempt.FinalManifest.Assets)
	attempt.Published.Candidates = clonePublishedCandidates(attempt.Published.Candidates)
	attempt.Quote.Operations = cloneSlice(attempt.Quote.Operations)
	attempt.Receipt.ProviderRequestIDs = cloneStrings(attempt.Receipt.ProviderRequestIDs)
	attempt.ReviewUsage = cloneReviewUsage(attempt.ReviewUsage)
	return attempt
}

func cloneReviewUsage(values []imageagent.SlotReviewUsageAttempt) []imageagent.SlotReviewUsageAttempt {
	cloned := cloneSlice(values)
	for index := range cloned {
		cloned[index].Quote.Operations = cloneSlice(cloned[index].Quote.Operations)
		cloned[index].Receipt.ProviderRequestIDs = cloneStrings(cloned[index].Receipt.ProviderRequestIDs)
	}
	return cloned
}

func cloneStagedAssetRefs(assets []imageagent.StagedAssetRef) []imageagent.StagedAssetRef {
	cloned := cloneSlice(assets)
	for index := range cloned {
		cloned[index].Operations = cloneStrings(cloned[index].Operations)
	}
	return cloned
}

func clonePublishedAssetRefs(assets []imageagent.PublishedAssetRef) []imageagent.PublishedAssetRef {
	cloned := cloneSlice(assets)
	for index := range cloned {
		cloned[index].Operations = cloneStrings(cloned[index].Operations)
	}
	return cloned
}

func clonePublishedCandidates(candidates []imageagent.SlotEffectV3AssetCandidate) []imageagent.SlotEffectV3AssetCandidate {
	return cloneSlice(candidates)
}

func cloneStringMap(values map[string]string) map[string]string {
	if values == nil {
		return nil
	}
	cloned := make(map[string]string, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}

func cloneStrings(values []string) []string {
	return cloneSlice(values)
}

func cloneSlice[T any](values []T) []T {
	if values == nil {
		return nil
	}
	cloned := make([]T, len(values))
	copy(cloned, values)
	return cloned
}
