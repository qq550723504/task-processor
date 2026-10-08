package supplychainapp

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"task-processor/internal/listing/preparation"
	record "task-processor/internal/listing/record/target"
	"task-processor/internal/listing/submission"
	"task-processor/internal/marketplace/shein/goods"
	model "task-processor/internal/marketplace/shein/model"
	"task-processor/internal/product/collection"
	"task-processor/internal/storecenter"
	"time"
)

type ResolveUploadInput struct {
	RecordID  string `json:"recordId"`
	AttemptID string `json:"attemptId"`
	SPU       string `json:"spu"`
}
type UploadAttemptView struct {
	RecordID   string                     `json:"recordId"`
	AttemptID  string                     `json:"attemptId"`
	EffectKind string                     `json:"effectKind"`
	Status     submission.ExecutionStatus `json:"status"`
	Message    string                     `json:"message"`
}

func (a *Application) ReadUploadAttempt(ctx context.Context, recordID, attemptID string) (UploadAttemptView, error) {
	if a.Uploader == nil || !collection.ValidID(recordID) || !collection.ValidID(attemptID) {
		return UploadAttemptView{}, record.ErrInvalid
	}
	scope, err := a.Authorization.Authorize(ctx, preparation.PermissionRead)
	if err != nil {
		return UploadAttemptView{}, err
	}
	s := a.Uploader
	if err = s.authorize(ctx, scope); err != nil {
		return UploadAttemptView{}, err
	}
	saved, err := s.dependencies.Records.ReadTargetRecord(ctx, scope, recordID)
	if err != nil {
		return UploadAttemptView{}, err
	}
	if _, err = s.source(ctx, scope, saved); err != nil {
		return UploadAttemptView{}, err
	}
	get, ok := s.dependencies.Kernel.(interface {
		Get(context.Context, submission.ExecutionScope, string) (submission.ExecutionAttempt, error)
	})
	if !ok {
		return UploadAttemptView{}, record.ErrUnavailable
	}
	attempt, err := get.Get(ctx, submission.ExecutionScope{OrganizationID: scope.OrganizationID}, attemptID)
	if err != nil {
		return UploadAttemptView{}, err
	}
	intent, err := s.dependencies.Intents.ReadOfficialIntent(ctx, scope, attempt.IntentKey)
	if err != nil || intent.RecordID != recordID || intent.RecordHash != collection.Digest(saved) || intent.Target != attempt.Target {
		return UploadAttemptView{}, record.ErrConflict
	}
	message := "可填写原商品 SPU，从官方查询审核通过的商品核实。"
	if intent.Kind == "image" {
		message = "图片处理结果待核实；官方未提供可安全确认该图片操作的查询，原操作保留，不自动重发。"
	}
	return UploadAttemptView{RecordID: recordID, AttemptID: attemptID, EffectKind: intent.Kind, Status: attempt.Status, Message: message}, nil
}

func (a *Application) ResolveUpload(ctx context.Context, input ResolveUploadInput) (UploadResult, error) {
	if a.Uploader == nil {
		return UploadResult{}, record.ErrUnavailable
	}
	scope, err := a.Authorization.Authorize(ctx, preparation.PermissionManage)
	if err != nil {
		return UploadResult{}, err
	}
	return a.Uploader.ResolveUpload(ctx, scope, input)
}
func (s *UploadService) originalPayload(ctx context.Context, scope collection.Scope, saved record.TargetRecord) (model.PublishProduct, error) {
	raw, err := json.Marshal(saved.Result.Product)
	if err != nil {
		return model.PublishProduct{}, record.ErrUnavailable
	}
	var p model.PublishProduct
	if json.Unmarshal(raw, &p) != nil {
		return p, record.ErrUnavailable
	}
	urls := map[string]string{}
	for _, observation := range saved.ImageObservations {
		subject, err := submission.ImageSubjectID(observation.ContentHash, saved.Merchant.Site, observation.Type)
		if err != nil {
			return p, err
		}
		receipt, err := s.dependencies.Receipts.FindOfficialTarget(ctx, scope.OrganizationID, submission.ExecutionTarget{Platform: "shein", StoreID: saved.Merchant.StoreID, SubjectID: subject})
		if err != nil || !matchingImageReceipt(receipt, saved.Merchant, observation) {
			return p, record.ErrConflict
		}
		urls[strconv.Itoa(observation.Type)+"/"+observation.SourceURL] = receipt.Image.RemoteURL
	}
	valid := true
	replace := func(info *model.ImageInfo) {
		if info == nil {
			return
		}
		for i := range info.Images {
			url := urls[strconv.Itoa(info.Images[i].Type)+"/"+info.Images[i].URL]
			if url == "" || !goods.IsOfficialImageReference(url) {
				valid = false
			} else {
				info.Images[i].URL = url
			}
		}
	}
	replace(p.ImageInfo)
	for i := range p.SKCs {
		replace(&p.SKCs[i].ImageInfo)
		for j := range p.SKCs[i].SKUs {
			replace(p.SKCs[i].SKUs[j].ImageInfo)
		}
		for j := range p.SKCs[i].SiteDetailImages {
			for k := range p.SKCs[i].SiteDetailImages[j].Images {
				url := urls["7/"+p.SKCs[i].SiteDetailImages[j].Images[k].URL]
				if url == "" {
					valid = false
				} else {
					p.SKCs[i].SiteDetailImages[j].Images[k].URL = url
				}
			}
		}
	}
	if !valid {
		return p, record.ErrConflict
	}
	return p, nil
}
func matchingImageReceipt(receipt submission.OfficialReceipt, binding storecenter.ProductMerchantBinding, image goods.OfficialImageObservation) bool {
	b := receipt.Binding
	return receipt.Kind == "image" && receipt.Image != nil && b.OrganizationID == binding.OrganizationID && b.StoreID == binding.StoreID && b.Site == binding.Site && b.SupplierIdentityHash == binding.SupplierIdentityHash && b.ApplicationID == binding.ApplicationID && b.ApplicationType == binding.ApplicationType && b.ApplicationRevision == binding.ApplicationRevision && receipt.Image.ContentHash == image.ContentHash && receipt.Image.Type == image.Type && receipt.Image.SourceURL == image.SourceURL && goods.IsOfficialImageReference(receipt.Image.RemoteURL)
}
func (s *UploadService) ResolveUpload(ctx context.Context, scope collection.Scope, input ResolveUploadInput) (UploadResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 18*time.Second)
	defer cancel()
	if !collection.ValidID(input.RecordID) || !collection.ValidID(input.AttemptID) || len(input.SPU) < 1 || len(input.SPU) > 128 {
		return UploadResult{}, record.ErrInvalid
	}
	if err := s.authorize(ctx, scope); err != nil {
		return UploadResult{}, err
	}
	saved, err := s.dependencies.Records.ReadTargetRecord(ctx, scope, input.RecordID)
	if err != nil {
		return UploadResult{}, err
	}
	if _, err = s.source(ctx, scope, saved); err != nil {
		return UploadResult{}, err
	}
	get, ok := s.dependencies.Kernel.(interface {
		Get(context.Context, submission.ExecutionScope, string) (submission.ExecutionAttempt, error)
	})
	if !ok {
		return UploadResult{}, record.ErrUnavailable
	}
	attempt, err := get.Get(ctx, submission.ExecutionScope{OrganizationID: scope.OrganizationID}, input.AttemptID)
	if err != nil {
		return UploadResult{}, err
	}
	intent, err := s.dependencies.Intents.ReadOfficialIntent(ctx, scope, attempt.IntentKey)
	if err != nil || intent.Kind != "publish" || intent.RecordID != saved.ID || intent.RecordHash != collection.Digest(saved) || intent.Target != attempt.Target || intent.Binding != saved.Merchant {
		return UploadResult{}, record.ErrConflict
	}
	result := func(v submission.OfficialReceipt) (UploadResult, error) {
		if v.RecordID != saved.ID || v.Product.SPUName != input.SPU {
			return UploadResult{}, record.ErrConflict
		}
		return UploadResult{RecordID: saved.ID, PublishedRecordID: v.RecordID, AttemptID: v.ID, Status: submission.ExecutionSucceeded, CurrentRecordUploaded: true, Product: v.Product, Message: "已从官方查询核实原上传结果"}, nil
	}
	if prior, e := s.dependencies.Receipts.ReadOfficial(ctx, scope, attempt.AttemptID); e == nil {
		return result(prior)
	} else if !errors.Is(e, submission.ErrExecutionNotFound) {
		return UploadResult{}, e
	}
	if attempt.Status == submission.ExecutionClaimed && time.Now().After(attempt.LeaseExpiresAt) {
		attempt, err = s.dependencies.Kernel.Expire(ctx, submission.ExecutionScope{OrganizationID: scope.OrganizationID}, attempt.AttemptID)
		if err != nil {
			return UploadResult{}, err
		}
	}
	if attempt.Status != submission.ExecutionOutcomeUnknown {
		return unknownUpload(saved.ID, attempt.AttemptID), nil
	}
	payload, err := s.originalPayload(ctx, scope, saved)
	if err != nil {
		return UploadResult{}, err
	}
	merchant, err := s.dependencies.Stores.ExecutionMerchant(ctx, scope, saved.Merchant.StoreID, storecenter.ProductPurposePublish, &intent.Binding)
	if err != nil {
		return UploadResult{}, err
	}
	lookup, ok := merchant.(interface {
		QuerySPU(context.Context, string) (model.ProductReadback, error)
	})
	if !ok {
		return UploadResult{}, record.ErrUnavailable
	}
	observed, err := lookup.QuerySPU(ctx, input.SPU)
	if err != nil || !goods.CorrelatesProductReadback(payload, input.SPU, observed) {
		pending := unknownUpload(saved.ID, attempt.AttemptID)
		pending.Message = "官方尚无完整匹配的审核通过商品；继续保留原上传待核实状态"
		return pending, nil
	}
	proof, err := submission.NewPublishResolution(scope, intent, saved, attempt, payload, input.SPU, observed)
	if err != nil {
		return UploadResult{}, err
	}
	if err = s.authorize(ctx, scope); err != nil {
		return UploadResult{}, err
	}
	if _, err = s.dependencies.Stores.ExecutionMerchant(ctx, scope, saved.Merchant.StoreID, storecenter.ProductPurposePublish, &intent.Binding); err != nil {
		return UploadResult{}, err
	}
	repository, ok := s.dependencies.Receipts.(submission.OfficialResolutionRepository)
	if !ok {
		return UploadResult{}, record.ErrUnavailable
	}
	receipt, err := repository.ResolveOfficial(ctx, proof)
	if errors.Is(err, submission.ErrExecutionOutcomeUnknown) {
		receipt, err = s.dependencies.Receipts.ReadOfficial(ctx, scope, attempt.AttemptID)
	}
	if err != nil {
		return unknownUpload(saved.ID, attempt.AttemptID), nil
	}
	return result(receipt)
}
