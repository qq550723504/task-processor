package submissionpersistence

import (
	"context"
	"encoding/json"
	"errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	recordstore "task-processor/internal/integration/persistence/listing/record"
	"task-processor/internal/listing/submission"
	"task-processor/internal/product/collection"
)

type OfficialRepository struct {
	db   *gorm.DB
	core *Repository
}

func NewOfficialRepository(ctx context.Context, db *gorm.DB) (*OfficialRepository, error) {
	if VerifyOfficialSchema(ctx, db) != nil {
		return nil, submission.ErrExecutionUnavailable
	}
	core, err := NewOfficialEffectsRepository(db)
	if err != nil {
		return nil, err
	}
	return &OfficialRepository{db: db, core: core}, nil
}

type officialIntentRow struct {
	OrganizationID, ActorID, MemberID              string
	IntentKey, RecordID, Kind, InputHash, BodyHash string
	BodyJSON                                       []byte
}

func (officialIntentRow) TableName() string { return "listing_submission_official_intents" }

type officialReceiptRow struct {
	OrganizationID, ActorID, MemberID                        string
	AttemptID, IntentKey, StoreID, SubjectID, Kind, BodyHash string
	BodyJSON                                                 []byte
}

func (officialReceiptRow) TableName() string { return "listing_submission_official_receipts" }
func readOfficialIntent(db *gorm.DB, org, key string, lock bool) (submission.OfficialIntent, error) {
	var row officialIntentRow
	query := db.Where("organization_id=? AND intent_key=?", org, key)
	if lock {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	result := query.Take(&row)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return submission.OfficialIntent{}, submission.ErrExecutionNotFound
	}
	if result.Error != nil {
		return submission.OfficialIntent{}, submission.ErrExecutionUnavailable
	}
	var body submission.OfficialIntent
	if len(row.BodyJSON) > submission.MaxExecutionPayloadBytes || json.Unmarshal(row.BodyJSON, &body) != nil || submission.ValidateOfficialIntent(body) != nil || collection.Digest(body) != row.BodyHash ||
		body.Owner.OrganizationID != org || body.Owner.ActorID != row.ActorID || body.Owner.MemberID != row.MemberID || body.Key != key || body.RecordID != row.RecordID || body.Kind != row.Kind || body.InputHash != row.InputHash {
		return body, submission.ErrExecutionUnavailable
	}
	return body, nil
}
func (r *OfficialRepository) PrepareOfficial(ctx context.Context, proof submission.OfficialIntentCommit) (submission.OfficialIntent, error) {
	body, err := proof.Read(ctx)
	if err != nil {
		return submission.OfficialIntent{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, submission.ExecutionTimeout)
	defer cancel()
	tx := r.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return submission.OfficialIntent{}, submission.ErrExecutionUnavailable
	}
	defer tx.Rollback()
	if tx.Exec("SET LOCAL synchronous_commit=on").Error != nil || advisoryLock(tx, "official-intent", body.Owner.OrganizationID, body.Key) != nil {
		return submission.OfficialIntent{}, submission.ErrExecutionUnavailable
	}
	if prior, err := readOfficialIntent(tx, body.Owner.OrganizationID, body.Key, true); err == nil {
		if prior.InputHash != body.InputHash {
			return submission.OfficialIntent{}, submission.ErrExecutionIntentConflict
		}
		return prior, nil
	} else if !errors.Is(err, submission.ErrExecutionNotFound) {
		return submission.OfficialIntent{}, err
	}
	// Match the actual Record owner fact, not a caller's similarly shaped DTO.
	records, err := recordstore.NewRepository(ctx, tx)
	if err != nil {
		return submission.OfficialIntent{}, submission.ErrExecutionUnavailable
	}
	var reference struct{ TargetID string }
	if tx.Raw("SELECT target_id FROM listing_target_records WHERE organization_id=? AND actor_id=? AND member_id=? AND id=?", body.Owner.OrganizationID, body.Owner.ActorID, body.Owner.MemberID, body.RecordID).Scan(&reference).Error != nil || reference.TargetID == "" {
		return submission.OfficialIntent{}, submission.ErrExecutionEvidenceRequired
	}
	if tx.Exec("SELECT 1 FROM listing_preparation_targets WHERE organization_id=? AND actor_id=? AND id=? FOR UPDATE", body.Owner.OrganizationID, body.Owner.ActorID, reference.TargetID).Error != nil {
		return submission.OfficialIntent{}, submission.ErrExecutionUnavailable
	}
	actual, err := records.ReadTargetHead(ctx, body.Owner, reference.TargetID)
	if err != nil || actual.ID != body.RecordID || collection.Digest(actual) != body.RecordHash || actual.Source != body.Source || actual.Merchant != body.Binding || !actual.Result.ReadyForUpload {
		return submission.OfficialIntent{}, submission.ErrExecutionIntentConflict
	}
	raw, err := json.Marshal(body)
	if err != nil || len(raw) > submission.MaxExecutionPayloadBytes {
		return submission.OfficialIntent{}, submission.ErrExecutionInvalid
	}
	row := officialIntentRow{OrganizationID: body.Owner.OrganizationID, ActorID: body.Owner.ActorID, MemberID: body.Owner.MemberID, IntentKey: body.Key, RecordID: body.RecordID, Kind: body.Kind, InputHash: body.InputHash, BodyHash: collection.Digest(body), BodyJSON: raw}
	created := tx.Create(&row)
	if created.Error != nil || created.RowsAffected != 1 {
		return submission.OfficialIntent{}, submission.ErrExecutionUnavailable
	}
	if _, err = proof.Read(ctx); err != nil {
		return submission.OfficialIntent{}, err
	}
	if tx.Commit().Error != nil {
		return submission.OfficialIntent{}, submission.ErrExecutionOutcomeUnknown
	}
	return body, nil
}
func (r *OfficialRepository) ReadOfficialIntent(ctx context.Context, scope collection.Scope, key string) (submission.OfficialIntent, error) {
	if ctx == nil || scope.Validate() != nil {
		return submission.OfficialIntent{}, submission.ErrExecutionInvalid
	}
	ctx, cancel := context.WithTimeout(ctx, submission.ExecutionTimeout)
	defer cancel()
	value, err := readOfficialIntent(r.db.WithContext(ctx), scope.OrganizationID, key, false)
	if err == nil && value.Owner != scope {
		return submission.OfficialIntent{}, submission.ErrExecutionNotFound
	}
	return value, err
}
func readOfficialReceipt(db *gorm.DB, org, attemptID string) (submission.OfficialReceipt, error) {
	var row officialReceiptRow
	result := db.Where("organization_id=? AND attempt_id=?", org, attemptID).Take(&row)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return submission.OfficialReceipt{}, submission.ErrExecutionNotFound
	}
	if result.Error != nil {
		return submission.OfficialReceipt{}, submission.ErrExecutionUnavailable
	}
	var body submission.OfficialReceipt
	if len(row.BodyJSON) > submission.MaxExecutionPayloadBytes || json.Unmarshal(row.BodyJSON, &body) != nil || submission.ValidateOfficialReceipt(body) != nil || collection.Digest(body) != row.BodyHash ||
		body.Owner.OrganizationID != org || body.Owner.ActorID != row.ActorID || body.Owner.MemberID != row.MemberID || body.ID != attemptID || body.IntentKey != row.IntentKey || body.Target.StoreID != row.StoreID || body.Target.SubjectID != row.SubjectID || body.Kind != row.Kind {
		return body, submission.ErrExecutionUnavailable
	}
	return body, nil
}
func receiptMatchesIntent(body submission.OfficialReceipt, intent submission.OfficialIntent) bool {
	if body.Owner != intent.Owner || body.Kind != intent.Kind || body.Target != intent.Target || body.IntentKey != intent.Key || body.PayloadFingerprint != intent.PayloadFingerprint || body.Binding != intent.Binding {
		return false
	}
	if body.Kind == "publish" {
		return body.RecordID == intent.RecordID && body.ProductKey == intent.Source.Source.ProductKey
	}
	if body.Image == nil || intent.Image == nil {
		return false
	}
	copy := *body.Image
	copy.RemoteURL, copy.ResponseHash = "", ""
	return collection.Digest(copy) == collection.Digest(intent.Image)
}
func (r *OfficialRepository) CompleteOfficial(ctx context.Context, proof submission.OfficialCompletion) (submission.OfficialReceipt, error) {
	claim, body, err := proof.Read(ctx)
	if err != nil {
		return submission.OfficialReceipt{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, submission.ExecutionTimeout)
	defer cancel()
	tx := r.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return submission.OfficialReceipt{}, submission.ErrExecutionUnavailable
	}
	defer tx.Rollback()
	intent, err := readOfficialIntent(tx, body.Owner.OrganizationID, body.IntentKey, true)
	if err != nil || !receiptMatchesIntent(body, intent) {
		return submission.OfficialReceipt{}, submission.ErrExecutionEvidenceRequired
	}
	if prior, err := readOfficialReceipt(tx, body.Owner.OrganizationID, body.ID); err == nil {
		if collection.Digest(prior) != collection.Digest(body) {
			return submission.OfficialReceipt{}, submission.ErrExecutionIntentConflict
		}
		return prior, nil
	} else if !errors.Is(err, submission.ErrExecutionNotFound) {
		return submission.OfficialReceipt{}, err
	}
	bound, err := NewTransactionFinalizer(ctx, tx)
	if err != nil {
		return submission.OfficialReceipt{}, err
	}
	kernel, err := submission.NewExecutionKernel(bound)
	if err != nil {
		return submission.OfficialReceipt{}, err
	}
	attempt, err := kernel.Get(ctx, claim.Scope, claim.AttemptID)
	if err != nil || attempt.Target != body.Target || attempt.IntentKey != body.IntentKey || attempt.PayloadFingerprint != body.PayloadFingerprint {
		return submission.OfficialReceipt{}, submission.ErrExecutionEvidenceRequired
	}
	_, err = kernel.Complete(ctx, claim, submission.ExecutionEvidence{Kind: submission.EvidenceProviderResponse, Outcome: submission.ExecutionSucceeded, Reference: body.ID, Fingerprint: body.ResponseHash, ObservedAt: body.ObservedAt})
	if err != nil {
		return submission.OfficialReceipt{}, err
	}
	raw, err := json.Marshal(body)
	if err != nil || len(raw) > submission.MaxExecutionPayloadBytes {
		return submission.OfficialReceipt{}, submission.ErrExecutionInvalid
	}
	row := officialReceiptRow{OrganizationID: body.Owner.OrganizationID, ActorID: body.Owner.ActorID, MemberID: body.Owner.MemberID, AttemptID: body.ID, IntentKey: body.IntentKey, StoreID: body.Target.StoreID, SubjectID: body.Target.SubjectID, Kind: body.Kind, BodyHash: collection.Digest(body), BodyJSON: raw}
	created := tx.Create(&row)
	if created.Error != nil || created.RowsAffected != 1 {
		return submission.OfficialReceipt{}, submission.ErrExecutionUnavailable
	}
	if tx.Commit().Error != nil {
		return submission.OfficialReceipt{}, submission.ErrExecutionOutcomeUnknown
	}
	return body, nil
}
func (r *OfficialRepository) validateCompleted(ctx context.Context, value submission.OfficialReceipt) error {
	attempt, err := r.core.Get(ctx, submission.ExecutionScope{OrganizationID: value.Owner.OrganizationID}, value.ID)
	if err != nil || attempt.Status != submission.ExecutionSucceeded || attempt.Target != value.Target || attempt.IntentKey != value.IntentKey || attempt.PayloadFingerprint != value.PayloadFingerprint || attempt.Evidence == nil || attempt.Evidence.Reference != value.ID || attempt.Evidence.Fingerprint != value.ResponseHash || !attempt.Evidence.ObservedAt.Equal(value.ObservedAt) {
		return submission.ErrExecutionUnavailable
	}
	return nil
}
func (r *OfficialRepository) ReadOfficial(ctx context.Context, scope collection.Scope, id string) (submission.OfficialReceipt, error) {
	if ctx == nil || scope.Validate() != nil {
		return submission.OfficialReceipt{}, submission.ErrExecutionInvalid
	}
	ctx, cancel := context.WithTimeout(ctx, submission.ExecutionTimeout)
	defer cancel()
	body, err := readOfficialReceipt(r.db.WithContext(ctx), scope.OrganizationID, id)
	if err != nil {
		return submission.OfficialReceipt{}, err
	}
	if body.Owner != scope {
		return submission.OfficialReceipt{}, submission.ErrExecutionNotFound
	}
	if err = r.validateCompleted(ctx, body); err != nil {
		return submission.OfficialReceipt{}, err
	}
	return body, nil
}

// This canonical Store result read is internal only. App requires the exact
// current source owner and merchant access before projecting provider facts.
func (r *OfficialRepository) FindOfficialTarget(ctx context.Context, org string, target submission.ExecutionTarget) (submission.OfficialReceipt, error) {
	if ctx == nil || target.Platform != "shein" || !collection.ValidID(target.StoreID) {
		return submission.OfficialReceipt{}, submission.ErrExecutionInvalid
	}
	ctx, cancel := context.WithTimeout(ctx, submission.ExecutionTimeout)
	defer cancel()
	var reference struct{ AttemptID string }
	if r.db.WithContext(ctx).Table("listing_submission_official_receipts").Select("attempt_id").Where("organization_id=? AND store_id=? AND subject_id=?", org, target.StoreID, target.SubjectID).Scan(&reference).Error != nil {
		return submission.OfficialReceipt{}, submission.ErrExecutionUnavailable
	}
	if reference.AttemptID == "" {
		return submission.OfficialReceipt{}, submission.ErrExecutionNotFound
	}
	body, err := readOfficialReceipt(r.db.WithContext(ctx), org, reference.AttemptID)
	if err != nil {
		return submission.OfficialReceipt{}, err
	}
	if body.Target != target || r.validateCompleted(ctx, body) != nil {
		return submission.OfficialReceipt{}, submission.ErrExecutionUnavailable
	}
	return body, nil
}

var _ submission.OfficialIntentRepository = (*OfficialRepository)(nil)
var _ submission.OfficialReceiptRepository = (*OfficialRepository)(nil)
