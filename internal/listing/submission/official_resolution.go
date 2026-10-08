package submission

import (
	"context"
	"encoding/json"
	record "task-processor/internal/listing/record/target"
	"task-processor/internal/marketplace/shein/goods"
	model "task-processor/internal/marketplace/shein/model"
	"task-processor/internal/product/collection"
	"time"
)

type OfficialResolutionData struct {
	Receipt                OfficialReceipt
	FenceEpoch             int64
	IntentHash, RecordHash string
}
type OfficialResolution struct {
	data      OfficialResolutionData
	hash      string
	expiresAt time.Time
}

func (p OfficialResolution) Read(ctx context.Context) (OfficialResolutionData, error) {
	if ctx == nil || ctx.Err() != nil || !time.Now().Before(p.expiresAt) || ValidateOfficialReceipt(p.data.Receipt) != nil || p.data.FenceEpoch < 1 || collection.Digest(p.data) != p.hash {
		return OfficialResolutionData{}, ErrExecutionEvidenceRequired
	}
	v := p.data
	receipt, e := cloneOfficialReceipt(v.Receipt)
	v.Receipt = receipt
	return v, e
}
func NewPublishResolution(owner collection.Scope, intent OfficialIntent, saved record.TargetRecord, attempt ExecutionAttempt, payload model.PublishProduct, spu string, result model.ProductReadback) (OfficialResolution, error) {
	raw, err := json.Marshal(payload)
	if err != nil || ValidateOfficialIntent(intent) != nil || ValidatePersistedExecutionAttempt(attempt) != nil || attempt.Status != ExecutionOutcomeUnknown || attempt.Action != OfficialPublishAction ||
		intent.Kind != "publish" || owner != intent.Owner || saved.ID != intent.RecordID || collection.Digest(saved) != intent.RecordHash || saved.Source != intent.Source || saved.Merchant != intent.Binding ||
		attempt.OrganizationID != owner.OrganizationID || attempt.Target != intent.Target || attempt.IntentKey != intent.Key || attempt.PayloadFingerprint != intent.PayloadFingerprint ||
		digestExecutionParts(attempt.Target.Platform, attempt.Target.StoreID, attempt.Target.SubjectID, OfficialPublishAction, string(raw)) != attempt.PayloadFingerprint || !MatchesPublicationRecord(saved, raw) || !goods.CorrelatesProductReadback(payload, spu, result) || !isExecutionDigest(result.Product.ResponseHash) {
		return OfficialResolution{}, ErrExecutionEvidenceRequired
	}
	body := OfficialReceipt{ID: attempt.AttemptID, Owner: owner, Kind: "publish", RecordID: saved.ID, ProductKey: intent.Source.Source.ProductKey, Target: intent.Target, IntentKey: intent.Key, PayloadFingerprint: intent.PayloadFingerprint, Binding: intent.Binding, Product: result.Product, ResponseHash: result.Product.ResponseHash, ObservedAt: time.Now().UTC().Truncate(time.Microsecond)}
	if ValidateOfficialReceipt(body) != nil {
		return OfficialResolution{}, ErrExecutionEvidenceRequired
	}
	body, err = cloneOfficialReceipt(body)
	if err != nil {
		return OfficialResolution{}, err
	}
	data := OfficialResolutionData{Receipt: body, FenceEpoch: attempt.FenceEpoch, IntentHash: collection.Digest(intent), RecordHash: intent.RecordHash}
	return OfficialResolution{data: data, hash: collection.Digest(data), expiresAt: time.Now().Add(5 * time.Second)}, nil
}

type OfficialResolutionRepository interface {
	ResolveOfficial(context.Context, OfficialResolution) (OfficialReceipt, error)
}
