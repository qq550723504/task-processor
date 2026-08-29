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
	attempt.Quote.Operations = append([]imageagent.SlotUsageOperation(nil), attempt.Quote.Operations...)
	attempt.Receipt.ProviderRequestIDs = cloneStrings(attempt.Receipt.ProviderRequestIDs)
	return attempt
}

func cloneStagedAssetRefs(assets []imageagent.StagedAssetRef) []imageagent.StagedAssetRef {
	cloned := append([]imageagent.StagedAssetRef(nil), assets...)
	for index := range cloned {
		cloned[index].Operations = cloneStrings(cloned[index].Operations)
	}
	return cloned
}

func clonePublishedAssetRefs(assets []imageagent.PublishedAssetRef) []imageagent.PublishedAssetRef {
	cloned := append([]imageagent.PublishedAssetRef(nil), assets...)
	for index := range cloned {
		cloned[index].Operations = cloneStrings(cloned[index].Operations)
	}
	return cloned
}

func clonePublishedCandidates(candidates []imageagent.SlotEffectV3AssetCandidate) []imageagent.SlotEffectV3AssetCandidate {
	return append([]imageagent.SlotEffectV3AssetCandidate(nil), candidates...)
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
	return append([]string(nil), values...)
}
