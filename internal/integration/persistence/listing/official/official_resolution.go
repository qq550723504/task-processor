package officialpersistence

import (
	"context"
	"encoding/json"
	"errors"
	submissionstore "task-processor/internal/integration/persistence/listing/submission"
	"task-processor/internal/listing/submission"
	"task-processor/internal/product/collection"
)

func (r *OfficialRepository) ResolveOfficial(ctx context.Context, proof submission.OfficialResolution) (submission.OfficialReceipt, error) {
	data, err := proof.Read(ctx)
	if err != nil {
		return submission.OfficialReceipt{}, err
	}
	body := data.Receipt
	ctx, cancel := context.WithTimeout(ctx, submission.ExecutionTimeout)
	defer cancel()
	tx := r.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return submission.OfficialReceipt{}, submission.ErrExecutionUnavailable
	}
	defer tx.Rollback()
	intent, err := readOfficialIntent(tx, body.Owner.OrganizationID, body.IntentKey, true)
	if err != nil || collection.Digest(intent) != data.IntentHash || intent.RecordHash != data.RecordHash || !receiptMatchesIntent(body, intent) {
		return submission.OfficialReceipt{}, submission.ErrExecutionEvidenceRequired
	}
	if prior, e := readOfficialReceipt(tx, body.Owner.OrganizationID, body.ID); e == nil {
		if !receiptMatchesIntent(prior, intent) || prior.Product.SPUName != body.Product.SPUName {
			return submission.OfficialReceipt{}, submission.ErrExecutionIntentConflict
		}
		return prior, nil
	} else if !errors.Is(e, submission.ErrExecutionNotFound) {
		return submission.OfficialReceipt{}, e
	}
	var original struct{ BodyHash string }
	row := tx.Raw("SELECT body_hash FROM listing_target_records WHERE organization_id=? AND actor_id=? AND member_id=? AND id=?", body.Owner.OrganizationID, body.Owner.ActorID, body.Owner.MemberID, body.RecordID).Scan(&original)
	if row.Error != nil || row.RowsAffected != 1 || original.BodyHash != data.RecordHash {
		return submission.OfficialReceipt{}, submission.ErrExecutionEvidenceRequired
	}
	bound, err := submissionstore.NewTransactionFinalizer(ctx, tx)
	if err != nil {
		return submission.OfficialReceipt{}, err
	}
	kernel, err := submission.NewExecutionKernel(bound)
	if err != nil {
		return submission.OfficialReceipt{}, err
	}
	scope := submission.ExecutionScope{OrganizationID: body.Owner.OrganizationID}
	attempt, err := kernel.Get(ctx, scope, body.ID)
	if err != nil || attempt.Target != body.Target || attempt.IntentKey != body.IntentKey || attempt.PayloadFingerprint != body.PayloadFingerprint {
		return submission.OfficialReceipt{}, submission.ErrExecutionEvidenceRequired
	}
	_, err = kernel.ResolveUnknown(ctx, scope, body.ID, data.FenceEpoch, submission.ExecutionEvidence{Kind: submission.EvidenceProviderReadBack, Outcome: submission.ExecutionSucceeded, Reference: body.ID, Fingerprint: body.ResponseHash, ObservedAt: body.ObservedAt})
	if err != nil {
		return submission.OfficialReceipt{}, err
	}
	raw, err := json.Marshal(body)
	if err != nil || len(raw) > submission.MaxExecutionPayloadBytes {
		return submission.OfficialReceipt{}, submission.ErrExecutionInvalid
	}
	created := tx.Create(&officialReceiptRow{OrganizationID: body.Owner.OrganizationID, ActorID: body.Owner.ActorID, MemberID: body.Owner.MemberID, AttemptID: body.ID, IntentKey: body.IntentKey, StoreID: body.Target.StoreID, SubjectID: body.Target.SubjectID, Kind: body.Kind, BodyHash: collection.Digest(body), BodyJSON: raw})
	if created.Error != nil || created.RowsAffected != 1 {
		return submission.OfficialReceipt{}, submission.ErrExecutionUnavailable
	}
	if _, err = proof.Read(ctx); err != nil {
		return submission.OfficialReceipt{}, err
	}
	if tx.Commit().Error != nil {
		return submission.OfficialReceipt{}, submission.ErrExecutionOutcomeUnknown
	}
	return body, nil
}
