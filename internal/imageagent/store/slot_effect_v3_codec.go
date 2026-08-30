package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"task-processor/internal/imageagent"
)

func decodeSlotEffectV3Record(row slotExternalEffectV3Record) (imageagent.SlotEffectV3Attempt, error) {
	result := slotEffectV3FromRecord(row)
	result.CorruptionMarker = row.CorruptionMarker
	if row.Phase == string(imageagent.SlotEffectV3RecoveryBlocked) && strings.TrimSpace(row.CorruptionMarker) != "" {
		if row.BlockedCode != imageagent.SlotRecoveryBlockedCode {
			return result, fmt.Errorf("%w: corrupt effect has invalid recovery block code", imageagent.ErrInvalidPersistedPolicy)
		}
		return result, nil
	}
	if len(row.StagingManifestJSON) > 0 {
		if err := json.Unmarshal(row.StagingManifestJSON, &result.StagingManifest); err != nil {
			return result, fmt.Errorf("decode v3 staging manifest: %w", err)
		}
	}
	if len(row.FinalManifestJSON) > 0 {
		if err := json.Unmarshal(row.FinalManifestJSON, &result.FinalManifest); err != nil {
			return result, fmt.Errorf("decode v3 final manifest: %w", err)
		}
	}
	if len(row.PublishedJSON) > 0 {
		if err := json.Unmarshal(row.PublishedJSON, &result.Published); err != nil {
			return result, fmt.Errorf("decode v3 published result: %w", err)
		}
	}
	if len(row.BudgetPolicyJSON) > 0 {
		if err := json.Unmarshal(row.BudgetPolicyJSON, &result.Policy); err != nil {
			result.CorruptionMarker = persistedEffectCorruptionMarker("budget_policy_json", row.BudgetPolicyJSON)
			return result, fmt.Errorf("%w: decode v3 budget policy: %w", imageagent.ErrCorruptPersistedEffect, err)
		}
	}
	if len(row.UsageQuoteJSON) > 0 {
		if err := json.Unmarshal(row.UsageQuoteJSON, &result.Quote); err != nil {
			result.CorruptionMarker = persistedEffectCorruptionMarker("usage_quote_json", row.UsageQuoteJSON)
			return result, fmt.Errorf("%w: decode v3 usage quote: %w", imageagent.ErrCorruptPersistedEffect, err)
		}
	}
	if len(row.UsageReceiptJSON) > 0 {
		if err := json.Unmarshal(row.UsageReceiptJSON, &result.Receipt); err != nil {
			return result, fmt.Errorf("decode v3 usage receipt: %w", err)
		}
	}
	if err := imageagent.ValidateSlotEffectV3AttemptPolicy(result); err != nil {
		return imageagent.SlotEffectV3Attempt{}, err
	}
	return result, nil
}

func persistedEffectCorruptionMarker(field string, payload []byte) string {
	digest := sha256.Sum256(payload)
	return field + ":sha256:" + hex.EncodeToString(digest[:])
}

func slotEffectV3FromRecord(row slotExternalEffectV3Record) imageagent.SlotEffectV3Attempt {
	claim := imageagent.PublicationClaim{Owner: row.PublicationOwner, Fence: row.PublicationFence}
	if row.PublicationLeaseExpiresAt != nil {
		claim.LeaseExpiresAt = row.PublicationLeaseExpiresAt.UTC()
	}
	return imageagent.SlotEffectV3Attempt{Identity: imageagent.SlotExternalEffectIdentity{RunScope: imageagent.RunScope{TenantID: row.TenantID, OwnerUserID: row.OwnerUserID, RunID: row.RunID}, PlanRevision: row.PlanRevision, SlotID: row.SlotID, Attempt: row.Attempt}, IdempotencyKey: row.IdempotencyKey, InputFingerprint: row.InputFingerprint, Phase: imageagent.SlotEffectV3Phase(row.Phase), StagingManifestFingerprint: row.StagingManifestFingerprint, Publication: claim, PublicationFingerprint: row.PublicationFingerprint, ResultFingerprint: row.ResultFingerprint, BlockedCode: row.BlockedCode, RecoveryPhase: imageagent.SlotEffectV3Phase(row.RecoveryPhase), CorruptionMarker: row.CorruptionMarker, BudgetStatus: imageagent.SlotBudgetStatus(row.BudgetStatus)}
}

func decodeReservedUsage(encoded []byte) (imageagent.UsageVector, error) {
	if len(encoded) == 0 || string(encoded) == "{}" || string(encoded) == "null" {
		return imageagent.UsageVector{}, nil
	}
	var usage imageagent.UsageVector
	if err := json.Unmarshal(encoded, &usage); err != nil {
		return imageagent.UsageVector{}, fmt.Errorf("decode reserved image agent usage: %w", err)
	}
	if _, err := imageagent.CheckedAddUsage(usage, imageagent.UsageVector{}); err != nil {
		return imageagent.UsageVector{}, err
	}
	return usage, nil
}

func databaseNow(ctx context.Context, tx *gorm.DB) (time.Time, error) {
	var value string
	query := "SELECT CURRENT_TIMESTAMP"
	if tx.Dialector.Name() == "postgres" {
		query = "SELECT clock_timestamp()"
	}
	if err := tx.WithContext(ctx).Raw(query).Scan(&value).Error; err != nil {
		return time.Time{}, fmt.Errorf("read database current time: %w", err)
	}
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02 15:04:05.999999999Z07:00", "2006-01-02 15:04:05"} {
		if now, err := time.Parse(layout, value); err == nil {
			return now.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("parse database current time %q", value)
}

func cloneSlotEffectV3(effect imageagent.SlotEffectV3Attempt) imageagent.SlotEffectV3Attempt {
	effect.StagingManifest = cloneStagingManifest(effect.StagingManifest)
	effect.FinalManifest = cloneFinalManifest(effect.FinalManifest)
	effect.Published = cloneSlotEffectV3PublishedResult(effect.Published)
	effect.Quote = cloneSlotUsageQuote(effect.Quote)
	effect.Receipt = cloneSlotUsageReceipt(effect.Receipt)
	return effect
}

func cloneSlotUsageQuote(quote imageagent.SlotUsageQuote) imageagent.SlotUsageQuote {
	quote.Operations = append([]imageagent.SlotUsageOperation(nil), quote.Operations...)
	return quote
}

func cloneSlotUsageReceipt(receipt imageagent.SlotUsageReceipt) imageagent.SlotUsageReceipt {
	receipt.ProviderRequestIDs = append([]string(nil), receipt.ProviderRequestIDs...)
	return receipt
}

func cloneSlotEffectV3PublishedResult(result imageagent.SlotEffectV3PublishedResult) imageagent.SlotEffectV3PublishedResult {
	result.Candidates = append([]imageagent.SlotEffectV3AssetCandidate(nil), result.Candidates...)
	return result
}

func cloneStagingManifest(manifest imageagent.StagingManifest) imageagent.StagingManifest {
	manifest.Assets = cloneStagedAssetRefs(manifest.Assets)
	if manifest.ProviderMetadata != nil {
		manifest.ProviderMetadata = cloneMetadata(manifest.ProviderMetadata)
	}
	return manifest
}

func cloneFinalManifest(manifest imageagent.FinalManifest) imageagent.FinalManifest {
	manifest.Assets = clonePublishedAssetRefs(manifest.Assets)
	return manifest
}

func clonePublishedAssetRefs(assets []imageagent.PublishedAssetRef) []imageagent.PublishedAssetRef {
	cloned := make([]imageagent.PublishedAssetRef, len(assets))
	for index, asset := range assets {
		if asset.Operations != nil {
			asset.Operations = append([]string{}, asset.Operations...)
		}
		cloned[index] = asset
	}
	return cloned
}

func cloneStagedAssetRefs(assets []imageagent.StagedAssetRef) []imageagent.StagedAssetRef {
	cloned := make([]imageagent.StagedAssetRef, len(assets))
	for index, asset := range assets {
		if asset.Operations != nil {
			asset.Operations = append([]string{}, asset.Operations...)
		}
		cloned[index] = asset
	}
	return cloned
}

var _ imageagent.SlotExternalEffectV3Repository = (*memoryRepository)(nil)
var _ imageagent.SlotExternalEffectV3Repository = (*gormRepository)(nil)
