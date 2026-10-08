package supplychainapp

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	storeapp "task-processor/internal/app/storecenter"
	"task-processor/internal/listing/preparation"
	record "task-processor/internal/listing/record/target"
	"task-processor/internal/listing/submission"
	"task-processor/internal/marketplace/shein/goods"
	model "task-processor/internal/marketplace/shein/model"
	"task-processor/internal/product/asset"
	"task-processor/internal/product/collection"
	"task-processor/internal/storecenter"
)

type ExecutionMerchant interface {
	Binding() storecenter.ProductMerchantBinding
	PublishPermission(context.Context, string) (model.PublishPermission, error)
	Publish(context.Context, model.PublishProduct) (model.PublishResult, error)
	TransformImage(context.Context, model.TransformImage) (model.TransformedImage, error)
}
type ExecutionStore interface {
	ExecutionMerchant(context.Context, collection.Scope, string, string, *storecenter.ProductMerchantBinding) (ExecutionMerchant, error)
}
type OfficialExecutionStore struct {
	Access *storeapp.OfficialProductAccess
}

func (s OfficialExecutionStore) ExecutionMerchant(ctx context.Context, scope collection.Scope, storeID, purpose string, expected *storecenter.ProductMerchantBinding) (ExecutionMerchant, error) {
	if s.Access == nil {
		return nil, record.ErrUnavailable
	}
	return s.Access.Authorize(ctx, storecenter.ProductExecutionSubject{OrganizationID: scope.OrganizationID, ActorID: scope.ActorID, MemberID: scope.MemberID, Purpose: purpose}, storeID, expected)
}

type UploadKernel interface {
	Acquire(context.Context, submission.AcquireExecutionCommand) (submission.ExecutionAcquisition, error)
	ReadIntent(context.Context, submission.ExecutionScope, string) (submission.ExecutionAttempt, error)
	MarkUnknown(context.Context, submission.ExecutionClaim, submission.UnknownReason) (submission.ExecutionAttempt, error)
	Expire(context.Context, submission.ExecutionScope, string) (submission.ExecutionAttempt, error)
}
type UploadDependencies struct {
	Sources       record.TargetExecutionSourceSelector
	Products      record.EffectiveTargetProductReader
	Assets        record.ApprovedAssetReader
	Rules         record.TargetRuleReader
	Records       record.TargetRepository
	Authorization collection.ExecutionAuthorizer
	Stores        ExecutionStore
	Images        ImageProbe
	Kernel        UploadKernel
	Intents       submission.OfficialIntentRepository
	Receipts      submission.OfficialReceiptRepository
}
type UploadService struct{ dependencies UploadDependencies }
type UploadResult struct {
	RecordID              string                     `json:"recordId"`
	PublishedRecordID     string                     `json:"publishedRecordId,omitempty"`
	AttemptID             string                     `json:"attemptId,omitempty"`
	Status                submission.ExecutionStatus `json:"status"`
	CurrentRecordUploaded bool                       `json:"currentRecordUploaded"`
	Product               model.PublishResult        `json:"product,omitempty"`
	Message               string                     `json:"message,omitempty"`
}

func NewUploadService(d UploadDependencies) (*UploadService, error) {
	if d.Sources == nil || d.Products == nil || d.Assets == nil || d.Rules == nil || d.Records == nil || d.Authorization == nil || d.Stores == nil || d.Images == nil || d.Kernel == nil || d.Intents == nil || d.Receipts == nil {
		return nil, record.ErrUnavailable
	}
	return &UploadService{d}, nil
}
func (s *UploadService) authorize(ctx context.Context, scope collection.Scope) error {
	for _, permission := range []string{collection.PermissionRead, preparation.PermissionRead, preparation.PermissionManage, preparation.PermissionSubmit} {
		if s.dependencies.Authorization.AuthorizeExecution(ctx, scope, permission) != nil {
			return record.ErrForbidden
		}
	}
	return nil
}
func (s *UploadService) source(ctx context.Context, scope collection.Scope, saved record.TargetRecord) (preparation.AuthorizedSource, error) {
	proof, err := s.dependencies.Sources.SelectForExecution(ctx, scope, saved.Source.ID)
	if err != nil {
		return preparation.AuthorizedSource{}, err
	}
	current, source, _, err := proof.Read(ctx)
	if err != nil || current != scope || source != saved.Source {
		return preparation.AuthorizedSource{}, record.ErrConflict
	}
	return proof, nil
}
func (s *UploadService) facts(ctx context.Context, scope collection.Scope, saved record.TargetRecord) (asset.ApprovedAssetInventory, goods.OfficialRuleSnapshot, error) {
	var inventory asset.ApprovedAssetInventory
	var rules goods.OfficialRuleSnapshot
	if err := s.authorize(ctx, scope); err != nil {
		return inventory, rules, err
	}
	head, err := s.dependencies.Records.ReadTargetHead(ctx, scope, saved.TargetID)
	if err != nil || head.ID != saved.ID || collection.Digest(head) != collection.Digest(saved) {
		return inventory, rules, record.ErrConflict
	}
	proof, err := s.source(ctx, scope, saved)
	if err != nil {
		return inventory, rules, err
	}
	product, err := s.dependencies.Products.ReadEffectiveTargetProduct(ctx, proof, saved.EffectiveVersion, saved.ApplyReceiptID)
	if err != nil || product.Identity.TenantID != scope.OrganizationID || product.Identity.ProductKey != saved.Source.Source.ProductKey || product.Version != saved.EffectiveVersion || collection.Digest(product.Snapshot) != saved.ProductHash {
		return inventory, rules, record.ErrConflict
	}
	assetScope := asset.InventoryScope{TenantID: scope.OrganizationID, ProductKey: product.Identity.ProductKey, TargetPlatform: "shein", SourceSnapshotVersion: product.Version}
	inventory, err = s.dependencies.Assets.GetApprovedInventory(ctx, assetScope)
	if err != nil || inventory.Scope != assetScope || collection.Digest(inventory) != saved.InventoryHash {
		return inventory, rules, record.ErrNotReady
	}
	binding, currentRules, err := s.dependencies.Rules.ReadTargetRules(ctx, scope, saved.Merchant.StoreID, saved.Input.Draft)
	if err != nil {
		return inventory, rules, err
	}
	if binding != saved.Merchant || collection.Digest(currentRules) != saved.RulesHash {
		return inventory, rules, record.ErrConflict
	}
	return inventory, currentRules, nil
}
func (s *UploadService) channel(ctx context.Context, scope collection.Scope, saved record.TargetRecord) (UploadResult, error) {
	subject, err := submission.ProductSubjectID(saved.Source.Source.ProductKey, saved.Merchant.Site)
	if err != nil {
		return UploadResult{}, err
	}
	retained, err := s.dependencies.Receipts.FindOfficialTarget(ctx, scope.OrganizationID, submission.ExecutionTarget{Platform: "shein", StoreID: saved.Merchant.StoreID, SubjectID: subject})
	if err != nil {
		return UploadResult{}, err
	}
	if retained.Kind != "publish" || retained.ProductKey != saved.Source.Source.ProductKey || retained.Binding.SupplierIdentityHash != saved.Merchant.SupplierIdentityHash {
		return UploadResult{}, record.ErrConflict
	}
	publishedRecordID := ""
	if retained.Owner == scope {
		publishedRecordID = retained.RecordID
	}
	return UploadResult{RecordID: saved.ID, PublishedRecordID: publishedRecordID, AttemptID: retained.ID, Status: submission.ExecutionSucceeded, CurrentRecordUploaded: retained.Owner == scope && retained.RecordID == saved.ID, Product: retained.Product, Message: "保留已确认的原平台商品；平台审核状态以店铺为准"}, nil
}

// Upload is an activity use case. Original membership is supplied only by the
// durable operation owner; live source and Store authorization are mandatory.
func (s *UploadService) Upload(ctx context.Context, scope collection.Scope, key, recordID string) (UploadResult, error) {
	if s == nil || ctx == nil || scope.Validate() != nil || !collection.ValidID(key) || !collection.ValidID(recordID) {
		return UploadResult{}, record.ErrInvalid
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	if err := s.authorize(ctx, scope); err != nil {
		return UploadResult{}, err
	}
	saved, err := s.dependencies.Records.ReadTargetRecord(ctx, scope, recordID)
	if err != nil {
		return UploadResult{}, err
	}
	if _, err = s.source(ctx, scope, saved); err != nil {
		return UploadResult{}, err
	}
	if _, err = s.dependencies.Stores.ExecutionMerchant(ctx, scope, saved.Merchant.StoreID, storecenter.ProductPurposePublish, &saved.Merchant); err != nil {
		return UploadResult{}, err
	}
	if existing, err := s.dependencies.Intents.ReadOfficialIntent(ctx, scope, key); err == nil {
		if existing.Kind != "publish" || existing.RecordID != saved.ID || existing.RecordHash != collection.Digest(saved) {
			return UploadResult{}, submission.ErrExecutionIntentConflict
		}
		attempt, err := s.dependencies.Kernel.ReadIntent(ctx, submission.ExecutionScope{OrganizationID: scope.OrganizationID}, key)
		if err == nil {
			return s.replay(ctx, scope, saved.ID, attempt)
		}
		if !errors.Is(err, submission.ErrExecutionNotFound) {
			return UploadResult{}, err
		}
	} else if !errors.Is(err, submission.ErrExecutionNotFound) {
		return UploadResult{}, err
	}
	if result, err := s.channel(ctx, scope, saved); err == nil {
		return result, nil
	} else if !errors.Is(err, submission.ErrExecutionNotFound) {
		return UploadResult{}, err
	}
	if !saved.Result.ReadyForUpload {
		return UploadResult{}, record.ErrNotReady
	}
	inventory, rules, err := s.facts(ctx, scope, saved)
	if err != nil {
		return UploadResult{}, err
	}
	actual, err := record.ProbeTargetImages(ctx, s.dependencies.Images, saved.Input.Draft, inventory)
	if err != nil {
		return UploadResult{}, err
	}
	if len(actual) == 0 || collection.Digest(actual) != collection.Digest(saved.ImageObservations) || !goods.BuildOfficial(saved.Input.Draft, rules, inventory, actual).ReadyForUpload {
		return UploadResult{}, record.ErrNotReady
	}
	observations := make([]goods.OfficialImageObservation, 0, len(actual))
	for _, observation := range actual {
		confirmed, pending, err := s.transform(ctx, scope, saved, observation)
		if err != nil {
			return UploadResult{}, err
		}
		if pending != nil {
			return *pending, nil
		}
		observations = append(observations, confirmed)
	}
	// Images may take time. Re-read current owners before constructing the
	// exact publication intent; drift does not silently update saved content.
	inventory, rules, err = s.facts(ctx, scope, saved)
	if err != nil {
		return UploadResult{}, err
	}
	wire := goods.BuildOfficial(saved.Input.Draft, rules, inventory, observations)
	if !wire.ReadyForUpload || len(wire.SubmissionPayload) == 0 || !submission.MatchesPublicationRecord(saved, wire.SubmissionPayload) {
		return UploadResult{}, record.ErrNotReady
	}
	merchant, err := s.dependencies.Stores.ExecutionMerchant(ctx, scope, saved.Merchant.StoreID, storecenter.ProductPurposePublish, &saved.Merchant)
	if err != nil {
		return UploadResult{}, err
	}
	permission, err := merchant.PublishPermission(ctx, wire.Product.BrandCode)
	if err != nil {
		return UploadResult{}, err
	}
	if !permission.Allowed {
		return UploadResult{}, record.ErrNotReady
	}
	proof, err := s.source(ctx, scope, saved)
	if err != nil {
		return UploadResult{}, err
	}
	intentProof, err := submission.PrepareOfficialIntent(ctx, proof, saved, key, wire.SubmissionPayload, nil)
	if err != nil {
		return UploadResult{}, err
	}
	intent, err := s.dependencies.Intents.PrepareOfficial(ctx, intentProof)
	if err != nil {
		return UploadResult{}, err
	}
	if err = s.authorize(ctx, scope); err != nil {
		return UploadResult{}, err
	}
	// Remint after preparation commit so its live Store proof is short-lived.
	merchant, err = s.dependencies.Stores.ExecutionMerchant(ctx, scope, saved.Merchant.StoreID, storecenter.ProductPurposePublish, &intent.Binding)
	if err != nil {
		return UploadResult{}, err
	}
	acquired, err := s.dependencies.Kernel.Acquire(ctx, submission.AcquireExecutionCommand{Scope: submission.ExecutionScope{OrganizationID: scope.OrganizationID}, IntentKey: key, Target: intent.Target, Action: submission.OfficialPublishAction, Payload: wire.SubmissionPayload, ClaimOwnerID: "supply-worker", Lease: time.Minute})
	if errors.Is(err, submission.ErrExecutionTargetSucceeded) {
		return s.channel(ctx, scope, saved)
	}
	if errors.Is(err, submission.ErrExecutionTargetClaimed) {
		return unknownUpload(saved.ID, ""), nil
	}
	if err != nil {
		return UploadResult{}, err
	}
	if acquired.Permit == nil {
		return s.replay(ctx, scope, saved.ID, acquired.Attempt)
	}
	claim := uploadClaim(scope, acquired.Permit)
	result, err := merchant.Publish(ctx, wire.Product)
	if err != nil {
		return s.unknown(ctx, saved.ID, claim), nil
	}
	completion, err := submission.NewPublishCompletion(scope, saved.ID, saved.Source.Source.ProductKey, intent.Binding, acquired.Attempt, claim, wire.Product, result)
	if err != nil {
		return s.unknown(ctx, saved.ID, claim), nil
	}
	retained, err := s.complete(ctx, scope, completion, acquired.Attempt.AttemptID)
	if err != nil {
		return s.unknown(ctx, saved.ID, claim), nil
	}
	return UploadResult{RecordID: saved.ID, PublishedRecordID: saved.ID, AttemptID: retained.ID, Status: submission.ExecutionSucceeded, CurrentRecordUploaded: true, Product: retained.Product, Message: "已上传，等待平台审核"}, nil
}
func uploadClaim(scope collection.Scope, permit *submission.SendPermit) submission.ExecutionClaim {
	return submission.ExecutionClaim{Scope: submission.ExecutionScope{OrganizationID: scope.OrganizationID}, AttemptID: permit.AttemptID, FenceEpoch: permit.FenceEpoch, OwnerID: permit.ClaimOwnerID, Token: permit.ClaimToken}
}
func unknownUpload(recordID, attemptID string) UploadResult {
	return UploadResult{RecordID: recordID, AttemptID: attemptID, Status: submission.ExecutionOutcomeUnknown, Message: "平台写入结果待核实；保留原操作，不自动重发"}
}
func (s *UploadService) unknown(ctx context.Context, id string, claim submission.ExecutionClaim) UploadResult {
	// Result retention is required even if the activity was cancelled or its
	// original member was revoked after dispatch. This performs no new send.
	retention, cancel := context.WithTimeout(context.WithoutCancel(ctx), submission.ExecutionTimeout)
	defer cancel()
	_, _ = s.dependencies.Kernel.MarkUnknown(retention, claim, submission.UnknownResponseLost)
	return unknownUpload(id, claim.AttemptID)
}
func (s *UploadService) complete(ctx context.Context, scope collection.Scope, proof submission.OfficialCompletion, attemptID string) (submission.OfficialReceipt, error) {
	retention, cancel := context.WithTimeout(context.WithoutCancel(ctx), submission.ExecutionTimeout)
	defer cancel()
	result, err := s.dependencies.Receipts.CompleteOfficial(retention, proof)
	if err == nil {
		return result, nil
	}
	// An uncertain database COMMIT is checked by its original receipt identity.
	// Neither this lookup nor activity retry dispatches another platform call.
	if errors.Is(err, submission.ErrExecutionOutcomeUnknown) {
		return s.dependencies.Receipts.ReadOfficial(retention, scope, attemptID)
	}
	return submission.OfficialReceipt{}, err
}
func (s *UploadService) replay(ctx context.Context, scope collection.Scope, recordID string, attempt submission.ExecutionAttempt) (UploadResult, error) {
	if attempt.Status == submission.ExecutionSucceeded {
		receipt, err := s.dependencies.Receipts.ReadOfficial(ctx, scope, attempt.AttemptID)
		if err != nil {
			return UploadResult{}, err
		}
		return UploadResult{RecordID: recordID, PublishedRecordID: receipt.RecordID, AttemptID: attempt.AttemptID, Status: attempt.Status, CurrentRecordUploaded: receipt.Kind == "publish" && receipt.RecordID == recordID, Product: receipt.Product, Message: "保留原操作结果"}, nil
	}
	if attempt.Status == submission.ExecutionFailedDefinitive || attempt.Status == submission.ExecutionCancelled {
		return UploadResult{RecordID: recordID, AttemptID: attempt.AttemptID, Status: attempt.Status, Message: "保留已核实的原操作结果"}, nil
	}
	if attempt.Status == submission.ExecutionClaimed && !time.Now().Before(attempt.LeaseExpiresAt) {
		_, err := s.dependencies.Kernel.Expire(ctx, submission.ExecutionScope{OrganizationID: scope.OrganizationID}, attempt.AttemptID)
		if err != nil {
			return UploadResult{}, err
		}
	}
	return unknownUpload(recordID, attempt.AttemptID), nil
}
func (s *UploadService) transform(ctx context.Context, scope collection.Scope, saved record.TargetRecord, observation goods.OfficialImageObservation) (goods.OfficialImageObservation, *UploadResult, error) {
	subject, err := submission.ImageSubjectID(observation.ContentHash, saved.Merchant.Site, observation.Type)
	if err != nil {
		return observation, nil, err
	}
	target := submission.ExecutionTarget{Platform: "shein", StoreID: saved.Merchant.StoreID, SubjectID: subject}
	confirmed := func(receipt submission.OfficialReceipt) (goods.OfficialImageObservation, *UploadResult, error) {
		if receipt.Kind != "image" || receipt.Image == nil || receipt.Image.ContentHash != observation.ContentHash || receipt.Image.Type != observation.Type || receipt.Binding.SupplierIdentityHash != saved.Merchant.SupplierIdentityHash || receipt.Binding.ApplicationRevision != saved.Merchant.ApplicationRevision {
			return observation, nil, record.ErrConflict
		}
		observation.RemoteURL, observation.ResponseHash = receipt.Image.RemoteURL, receipt.Image.ResponseHash
		return observation, nil, nil
	}
	if receipt, err := s.dependencies.Receipts.FindOfficialTarget(ctx, scope.OrganizationID, target); err == nil {
		return confirmed(receipt)
	} else if !errors.Is(err, submission.ErrExecutionNotFound) {
		return observation, nil, err
	}
	if err = s.authorize(ctx, scope); err != nil {
		return observation, nil, err
	}
	merchant, err := s.dependencies.Stores.ExecutionMerchant(ctx, scope, saved.Merchant.StoreID, storecenter.ProductPurposeImage, &saved.Merchant)
	if err != nil {
		return observation, nil, err
	}
	input := model.TransformImage{OriginalURL: observation.SourceURL, Type: observation.Type}
	raw, err := json.Marshal(input)
	if err != nil {
		return observation, nil, record.ErrInvalid
	}
	key := collection.StableID(scope.OrganizationID, "shein-image", target.StoreID, subject, collection.Digest(input), saved.Merchant.SupplierIdentityHash, saved.Merchant.ApplicationRevision)
	source, err := s.source(ctx, scope, saved)
	if err != nil {
		return observation, nil, err
	}
	proof, err := submission.PrepareOfficialIntent(ctx, source, saved, key, raw, &observation)
	if err != nil {
		return observation, nil, err
	}
	intent, err := s.dependencies.Intents.PrepareOfficial(ctx, proof)
	if err != nil {
		return observation, nil, err
	}
	if intent.Owner != scope {
		pending := unknownUpload(saved.ID, "")
		return observation, &pending, nil
	}
	acquired, err := s.dependencies.Kernel.Acquire(ctx, submission.AcquireExecutionCommand{Scope: submission.ExecutionScope{OrganizationID: scope.OrganizationID}, IntentKey: key, Target: target, Action: submission.OfficialImageAction, Payload: raw, ClaimOwnerID: "supply-worker", Lease: time.Minute})
	if errors.Is(err, submission.ErrExecutionTargetSucceeded) {
		receipt, err := s.dependencies.Receipts.FindOfficialTarget(ctx, scope.OrganizationID, target)
		if err != nil {
			return observation, nil, err
		}
		return confirmed(receipt)
	}
	if errors.Is(err, submission.ErrExecutionTargetClaimed) {
		pending := unknownUpload(saved.ID, "")
		return observation, &pending, nil
	}
	if err != nil {
		return observation, nil, err
	}
	if acquired.Permit == nil {
		if acquired.Attempt.Status == submission.ExecutionSucceeded {
			receipt, err := s.dependencies.Receipts.ReadOfficial(ctx, scope, acquired.Attempt.AttemptID)
			if err != nil {
				return observation, nil, err
			}
			return confirmed(receipt)
		}
		pending, err := s.replay(ctx, scope, saved.ID, acquired.Attempt)
		return observation, &pending, err
	}
	claim := uploadClaim(scope, acquired.Permit)
	result, err := merchant.TransformImage(ctx, input)
	if err != nil {
		pending := s.unknown(ctx, saved.ID, claim)
		return observation, &pending, nil
	}
	completion, err := submission.NewImageCompletion(scope, intent.Binding, acquired.Attempt, claim, input, observation, result)
	if err != nil {
		pending := s.unknown(ctx, saved.ID, claim)
		return observation, &pending, nil
	}
	receipt, err := s.complete(ctx, scope, completion, acquired.Attempt.AttemptID)
	if err != nil {
		pending := s.unknown(ctx, saved.ID, claim)
		return observation, &pending, nil
	}
	return confirmed(receipt)
}
