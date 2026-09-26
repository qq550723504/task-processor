package store

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"time"

	"gorm.io/gorm"

	"task-processor/internal/imageagent"
	"task-processor/internal/imageagent/effectpolicy"
)

// RecoverGenerationStaging is restricted to the fixed generation owner. It
// never resets the dispatch fence or creates a provider reservation.
func (r *gormRepository) RecoverGenerationStaging(ctx context.Context, reservation imageagent.SlotEffectV3Reservation, intent imageagent.GenerationIntent, proof string, manifest imageagent.StagingManifest) (imageagent.SlotEffectV3Attempt, error) {
	var result imageagent.SlotEffectV3Attempt
	if r.scopeProtocol != imageagent.OrganizationScopeProtocol || intent.Identity != reservation.Identity {
		return result, imageagent.ErrIdentityRequired
	}
	normalized, fingerprint, err := effectpolicy.PreflightStagingManifest(manifest)
	if err != nil || len(normalized.Assets) != 1 {
		return result, imageagent.ErrValidation
	}
	ref := normalized.Assets[0]
	if !strings.HasPrefix(ref.ObjectKey, "image-agent/staging/") {
		return result, imageagent.ErrValidation
	}
	input := imageagent.SlotExecutionInput{TenantID: intent.Identity.TenantID, UserID: intent.Identity.OwnerUserID, RunID: intent.Identity.RunID, PlanRevision: intent.Identity.PlanRevision, Slot: imageagent.Slot{ID: intent.Identity.SlotID}, Attempt: intent.Identity.Attempt}
	public := imageagent.PublishedAssetRef{ObjectKey: strings.Replace(ref.ObjectKey, "image-agent/staging/", imageagent.PublishedArtifactPrefix+"/", 1), SHA256: ref.SHA256, SizeBytes: ref.SizeBytes, ContentType: ref.ContentType, Width: ref.Width, Height: ref.Height, SourceAssetID: ref.SourceAssetID, Operations: ref.Operations, ProviderReceiptID: ref.ProviderReceiptID}
	if imageagent.ValidatePublishedAssetRefForSlot(input, public, 0) != nil {
		return result, imageagent.ErrValidation
	}
	err = withProjectionTransaction(ctx, r.db, func(tx *gorm.DB) error {
		run, err := r.findRunForUpdate(ctx, tx, intent.Identity.RunScope)
		if err != nil {
			return err
		}
		if run.ScopeProtocol != imageagent.OrganizationScopeProtocol || run.MemberID != intent.MemberID {
			return imageagent.ErrIdentityRequired
		}
		row, err := r.findSlotEffectV3ForUpdate(ctx, tx, intent.Identity)
		if err != nil {
			return err
		}
		fact, err := decodeGenerationFact(row.GenerationFactJSON)
		if err != nil {
			return err
		}
		if fact.Intent != intent || fact.State != imageagent.GenerationSucceeded || fact.TerminalProofDigest() != proof {
			return imageagent.ErrRevisionConflict
		}
		var catalog assetCatalogManifestRecord
		if err := tx.Where("tenant_id = ? AND owner_user_id = ? AND run_id = ?", intent.Identity.TenantID, intent.Identity.OwnerUserID, intent.Identity.RunID).Take(&catalog).Error; err != nil {
			return err
		}
		if catalog.Hash != intent.CatalogHash {
			return imageagent.ErrRevisionConflict
		}
		var slot slotRecord
		if err := tx.Where("tenant_id = ? AND owner_user_id = ? AND run_id = ? AND plan_revision = ? AND id = ?", intent.Identity.TenantID, intent.Identity.OwnerUserID, intent.Identity.RunID, intent.Identity.PlanRevision, intent.Identity.SlotID).Take(&slot).Error; err != nil {
			return err
		}
		var sources []string
		if json.Unmarshal(slot.SourceAssetIDs, &sources) != nil || slot.Role != string(imageagent.SlotRoleMain) || len(sources) != 1 || ref.SourceAssetID != sources[0] {
			return imageagent.ErrRevisionConflict
		}
		current, err := decodeSlotEffectV3Record(row)
		if err != nil {
			return err
		}
		if current.IdempotencyKey != reservation.IdempotencyKey || current.InputFingerprint != reservation.InputFingerprint || !reflect.DeepEqual(current.Policy, reservation.Policy) || !reflect.DeepEqual(current.Quote, reservation.Quote) {
			return imageagent.ErrRevisionConflict
		}
		if current.StagingManifestFingerprint != "" && current.StagingManifestFingerprint != fingerprint {
			return imageagent.ErrRevisionConflict
		}
		switch current.Phase {
		case imageagent.SlotEffectV3ProviderClaimed, imageagent.SlotEffectV3ProviderUnknown, imageagent.SlotEffectV3StagingUnknown:
		case imageagent.SlotEffectV3StagingPrepared, imageagent.SlotEffectV3ArtifactStaged, imageagent.SlotEffectV3PublicationClaimed, imageagent.SlotEffectV3PublicationComplete:
			if current.StagingManifestFingerprint != fingerprint {
				return imageagent.ErrRevisionConflict
			}
			result = current
			return nil
		default:
			return imageagent.ErrRevisionConflict
		}
		// The fixed single-edit quote is persisted, not recomputed from current
		// price/model credentials. Unknown holds become committed only on this
		// persisted success proof; ordinary UNKNOWN remains untouched.
		updates := map[string]any{}
		if current.Quote.Fingerprint != "" && current.BudgetStatus != imageagent.SlotBudgetCommitted {
			if current.BudgetStatus != imageagent.SlotBudgetReserved && current.BudgetStatus != imageagent.SlotBudgetUnknown {
				return imageagent.ErrRevisionConflict
			}
			accounting, err := providerAccountingSnapshotFromRecord(run)
			if err != nil {
				return err
			}
			current.BudgetStatus = imageagent.SlotBudgetReserved
			receipt := imageagent.SlotUsageReceipt{Actual: current.Quote.Maximum, CostBasis: imageagent.UsageCostUnavailable}
			if fact.Success.RequestID != "" {
				receipt.ProviderRequestIDs = []string{fact.Success.RequestID}
			}
			if current.Quote.Maximum.Images != 1 || current.Quote.Maximum.ModelCalls != 1 || current.Quote.Maximum.CostMicros != 0 {
				return imageagent.ErrRevisionConflict
			}
			decision, err := effectpolicy.SettleProvider(current, reservation, receipt, accounting, time.Now().UTC())
			if err != nil {
				return err
			}
			if err := persistGormProviderAccounting(tx, intent.Identity.RunScope, decision); err != nil {
				return err
			}
			current = decision.Attempt
			encoded, err := json.Marshal(receipt)
			if err != nil {
				return err
			}
			updates["usage_receipt_json"] = encoded
			updates["budget_status"] = string(current.BudgetStatus)
			updates["budget_settled_at"] = time.Now().UTC()
		}
		current.Phase = imageagent.SlotEffectV3StagingPrepared
		current.StagingManifest = normalized
		current.StagingManifestFingerprint = fingerprint
		current.BlockedCode = ""
		encoded, err := json.Marshal(normalized)
		if err != nil {
			return err
		}
		updates["phase"] = string(current.Phase)
		updates["staging_manifest_json"] = encoded
		updates["staging_manifest_fingerprint"] = fingerprint
		updates["staging_prepared_at"] = time.Now().UTC()
		updates["blocked_code"] = ""
		if err := slotEffectV3IdentityWhere(tx.Model(&slotExternalEffectV3Record{}), intent.Identity).Updates(updates).Error; err != nil {
			return err
		}
		result = current
		return nil
	})
	return result, err
}
