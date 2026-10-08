package submission

import (
	"context"
	"encoding/json"
	"time"

	"task-processor/internal/authidentity"
	"task-processor/internal/marketplace/shein/goods"
	model "task-processor/internal/marketplace/shein/model"
	"task-processor/internal/product/collection"
	"task-processor/internal/storecenter"
)

const OfficialPublishAction = "publish_product"
const OfficialImageAction = "transform_image"

func ProductSubjectID(productKey, site string) (string, error) {
	if !authidentity.IsBoundedIdentifier(productKey) || site != "shein-us" {
		return "", ErrExecutionInvalid
	}
	return "product-us-" + digestExecutionParts(productKey, site), nil
}
func ImageSubjectID(contentHash, site string, imageType int) (string, error) {
	if !isExecutionDigest(contentHash) || site != "shein-us" || imageType != 1 && imageType != 2 && imageType != 5 && imageType != 6 && imageType != 7 {
		return "", ErrExecutionInvalid
	}
	raw, _ := json.Marshal(imageType)
	return "image-us-" + digestExecutionParts(contentHash, site, OfficialImageAction, string(raw)), nil
}

// OfficialReceipt stores the bounded correlated provider facts. The immutable
// Record remains the payload owner, and the existing execution attempt remains
// the sole owner of success / UNKNOWN / the stable target fence.
type OfficialReceipt struct {
	ID                 string                             `json:"id"`
	Owner              collection.Scope                   `json:"owner"`
	Kind               string                             `json:"kind"`
	RecordID           string                             `json:"record_id,omitempty"`
	ProductKey         string                             `json:"product_key,omitempty"`
	Target             ExecutionTarget                    `json:"target"`
	IntentKey          string                             `json:"intent_key"`
	PayloadFingerprint string                             `json:"payload_fingerprint"`
	ResponseHash       string                             `json:"response_hash"`
	Binding            storecenter.ProductMerchantBinding `json:"binding"`
	Product            model.PublishResult                `json:"product,omitempty"`
	Image              *goods.OfficialImageObservation    `json:"image,omitempty"`
	ObservedAt         time.Time                          `json:"observed_at"`
}
type OfficialCompletion struct {
	claim   ExecutionClaim
	receipt OfficialReceipt
	hash    string
}

func (p OfficialCompletion) Read(ctx context.Context) (ExecutionClaim, OfficialReceipt, error) {
	if ctx == nil || ctx.Err() != nil || !validExecutionClaim(p.claim) || ValidateOfficialReceipt(p.receipt) != nil || collection.Digest(p.receipt) != p.hash {
		return ExecutionClaim{}, OfficialReceipt{}, ErrExecutionEvidenceRequired
	}
	copy, err := cloneOfficialReceipt(p.receipt)
	if err != nil {
		return ExecutionClaim{}, OfficialReceipt{}, err
	}
	return p.claim, copy, nil
}
func cloneOfficialReceipt(value OfficialReceipt) (OfficialReceipt, error) {
	raw, err := json.Marshal(value)
	if err != nil || len(raw) > MaxExecutionPayloadBytes {
		return OfficialReceipt{}, ErrExecutionEvidenceRequired
	}
	var copied OfficialReceipt
	if json.Unmarshal(raw, &copied) != nil {
		return OfficialReceipt{}, ErrExecutionEvidenceRequired
	}
	return copied, nil
}
func completionBase(owner collection.Scope, binding storecenter.ProductMerchantBinding, attempt ExecutionAttempt, claim ExecutionClaim, payload []byte, action string) (OfficialReceipt, error) {
	if owner.Validate() != nil || ValidatePersistedExecutionAttempt(attempt) != nil || attempt.Status != ExecutionClaimed || !validExecutionClaim(claim) ||
		claim.Scope.OrganizationID != owner.OrganizationID || attempt.OrganizationID != owner.OrganizationID || claim.AttemptID != attempt.AttemptID || claim.OwnerID != attempt.ClaimOwnerID || claim.FenceEpoch != attempt.FenceEpoch ||
		attempt.Target.Platform != "shein" || attempt.Target.StoreID != binding.StoreID || binding.OrganizationID != owner.OrganizationID || binding.Site != "shein-us" || attempt.Action != action ||
		attempt.PayloadFingerprint != digestExecutionParts(attempt.Target.Platform, attempt.Target.StoreID, attempt.Target.SubjectID, action, string(payload)) {
		return OfficialReceipt{}, ErrExecutionEvidenceRequired
	}
	return OfficialReceipt{ID: attempt.AttemptID, Owner: owner, Target: attempt.Target, IntentKey: attempt.IntentKey, PayloadFingerprint: attempt.PayloadFingerprint, Binding: binding, ObservedAt: time.Now().UTC().Truncate(time.Microsecond)}, nil
}
func NewPublishCompletion(owner collection.Scope, recordID, productKey string, binding storecenter.ProductMerchantBinding, attempt ExecutionAttempt, claim ExecutionClaim, payload model.PublishProduct, result model.PublishResult) (OfficialCompletion, error) {
	raw, err := json.Marshal(payload)
	if err != nil || len(raw) > MaxExecutionPayloadBytes || !goods.CorrelatesPublishedMembership(payload, result) || !isExecutionDigest(result.ResponseHash) {
		return OfficialCompletion{}, ErrExecutionEvidenceRequired
	}
	body, err := completionBase(owner, binding, attempt, claim, raw, OfficialPublishAction)
	if err != nil {
		return OfficialCompletion{}, err
	}
	subject, err := ProductSubjectID(productKey, binding.Site)
	if err != nil || subject != attempt.Target.SubjectID || !collection.ValidID(recordID) {
		return OfficialCompletion{}, ErrExecutionEvidenceRequired
	}
	body.Kind, body.RecordID, body.ProductKey, body.Product, body.ResponseHash = "publish", recordID, productKey, result, result.ResponseHash
	return sealOfficialCompletion(claim, body)
}
func NewImageCompletion(owner collection.Scope, binding storecenter.ProductMerchantBinding, attempt ExecutionAttempt, claim ExecutionClaim, input model.TransformImage, observation goods.OfficialImageObservation, result model.TransformedImage) (OfficialCompletion, error) {
	if !goods.ValidImageObservation(observation) || observation.SourceURL != input.OriginalURL || observation.Type != input.Type || observation.RemoteURL != "" || result.Original != input.OriginalURL || result.FailureReason != "" || !goods.IsOfficialImageReference(result.Transformed) || !isExecutionDigest(result.ResponseHash) {
		return OfficialCompletion{}, ErrExecutionEvidenceRequired
	}
	raw, err := json.Marshal(input)
	if err != nil {
		return OfficialCompletion{}, ErrExecutionEvidenceRequired
	}
	body, err := completionBase(owner, binding, attempt, claim, raw, OfficialImageAction)
	if err != nil {
		return OfficialCompletion{}, err
	}
	subject, err := ImageSubjectID(observation.ContentHash, binding.Site, input.Type)
	if err != nil || subject != attempt.Target.SubjectID {
		return OfficialCompletion{}, ErrExecutionEvidenceRequired
	}
	observation.RemoteURL, observation.ResponseHash = result.Transformed, result.ResponseHash
	body.Kind, body.Image, body.ResponseHash = "image", &observation, result.ResponseHash
	return sealOfficialCompletion(claim, body)
}
func sealOfficialCompletion(claim ExecutionClaim, body OfficialReceipt) (OfficialCompletion, error) {
	copied, err := cloneOfficialReceipt(body)
	if err != nil || ValidateOfficialReceipt(copied) != nil {
		return OfficialCompletion{}, ErrExecutionEvidenceRequired
	}
	return OfficialCompletion{claim: claim, receipt: copied, hash: collection.Digest(copied)}, nil
}
func ValidateOfficialReceipt(value OfficialReceipt) error {
	if !validExecutionAttemptID(value.ID) || value.Owner.Validate() != nil || value.Target.Platform != "shein" || !collection.ValidID(value.Target.StoreID) || !authidentity.IsBoundedIdentifier(value.IntentKey) ||
		!isExecutionDigest(value.PayloadFingerprint) || !isExecutionDigest(value.ResponseHash) || value.Binding.OrganizationID != value.Owner.OrganizationID || value.Binding.StoreID != value.Target.StoreID || value.Binding.Site != "shein-us" ||
		value.Binding.StoreVersion < 1 || value.Binding.ConnectionRevision < 1 || !authidentity.IsBoundedIdentifier(value.Binding.ApplicationRevision) || !isExecutionDigest(value.Binding.SupplierIdentityHash) || value.Binding.ServiceExpiresAt.IsZero() || value.ObservedAt.IsZero() {
		return ErrExecutionEvidenceRequired
	}
	switch value.Kind {
	case "publish":
		subject, err := ProductSubjectID(value.ProductKey, value.Binding.Site)
		if err != nil || subject != value.Target.SubjectID || !collection.ValidID(value.RecordID) || value.Image != nil || !authidentity.IsBoundedIdentifier(value.Product.SPUName) || len(value.Product.SKCs) < 1 || len(value.Product.SKCs) > 40 {
			return ErrExecutionEvidenceRequired
		}
		// Reconstruct membership from the retained result to validate platform ID
		// uniqueness. Exact sent-payload correlation was sealed before persistence.
		input := model.PublishProduct{}
		for _, skc := range value.Product.SKCs {
			group := model.ProductSKC{}
			for _, sku := range skc.SKUs {
				group.SKUs = append(group.SKUs, model.ProductSKU{SupplierSKU: sku.SupplierSKU})
			}
			input.SKCs = append(input.SKCs, group)
		}
		if !goods.CorrelatesPublishedMembership(input, value.Product) {
			return ErrExecutionEvidenceRequired
		}
	case "image":
		if value.Image == nil || !goods.ValidImageObservation(*value.Image) || value.RecordID != "" || value.ProductKey != "" || value.Product.SPUName != "" || len(value.Product.SKCs) != 0 || !goods.IsOfficialImageReference(value.Image.RemoteURL) || value.Image.ResponseHash != value.ResponseHash {
			return ErrExecutionEvidenceRequired
		}
		subject, err := ImageSubjectID(value.Image.ContentHash, value.Binding.Site, value.Image.Type)
		if err != nil || subject != value.Target.SubjectID {
			return ErrExecutionEvidenceRequired
		}
	default:
		return ErrExecutionEvidenceRequired
	}
	return nil
}

type OfficialReceiptRepository interface {
	CompleteOfficial(context.Context, OfficialCompletion) (OfficialReceipt, error)
	ReadOfficial(context.Context, collection.Scope, string) (OfficialReceipt, error)
	FindOfficialTarget(context.Context, string, ExecutionTarget) (OfficialReceipt, error)
}
